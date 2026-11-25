package idempotency

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
)

var errBodyTooLarge = errors.New("request body exceeds idempotency limit")

func Middleware(repo Repository, log *slog.Logger, cfg Config) gin.HandlerFunc {
	cfg = cfg.withDefaults()
	if log == nil {
		log = slog.Default()
	}

	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		if _, protected := cfg.ProtectedPaths[c.FullPath()]; !protected {
			c.Next()
			return
		}

		key := NormalizeKey(c.GetHeader(HeaderIdempotencyKey))
		if key == "" {
			if cfg.Required {
				httpx.Error(c, http.StatusBadRequest, httpx.CodeIdempotencyKeyRequired, "Idempotency-Key header is required")
				c.Abort()
				return
			}
			log.Debug("idempotency key missing; passing through",
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
			)
			c.Next()
			return
		}
		if err := ValidateKey(key); err != nil {
			httpx.Error(c, http.StatusBadRequest, httpx.CodeIdempotencyKeyInvalid, "Idempotency-Key must be 8-128 characters using A-Z, a-z, 0-9, '.', '_', ':' or '-'")
			c.Abort()
			return
		}

		actor, ok := auth.ActorFrom(c.Request.Context())
		if !ok || actor.TenantID == "" || actor.UserID == "" {
			httpx.Error(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		body, err := readRequestBody(c.Request, cfg.MaxBodyBytes)
		if errors.Is(err, errBodyTooLarge) {
			httpx.Error(c, http.StatusRequestEntityTooLarge, httpx.CodeLimitExceeded, "request body exceeds idempotency limit")
			c.Abort()
			return
		}
		if err != nil {
			httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "failed to read request body")
			c.Abort()
			return
		}

		path := c.Request.URL.Path
		now := time.Now()
		result, err := repo.Acquire(c.Request.Context(), AcquireInput{
			TenantID:    actor.TenantID,
			UserID:      actor.UserID,
			Key:         key,
			Method:      c.Request.Method,
			Path:        path,
			RequestHash: RequestHash(c.Request.Method, path, body),
			Now:         now,
			ExpiresAt:   now.Add(cfg.TTL),
		})
		if err != nil {
			log.Error("idempotency acquire failed",
				"error", err,
				"method", c.Request.Method,
				"path", path,
			)
			httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "idempotency store unavailable")
			c.Abort()
			return
		}

		switch result.State {
		case StateReplay:
			writeReplay(c, result.Record)
			return
		case StateConflict:
			httpx.Error(c, http.StatusConflict, httpx.CodeIdempotencyConflict, "idempotency key was used with a different request")
			c.Abort()
			return
		case StateInProgress:
			httpx.Error(c, http.StatusConflict, httpx.CodeIdempotencyInProgress, "request with this idempotency key is still in progress")
			c.Abort()
			return
		case StateResponseUnavailable:
			httpx.Error(c, http.StatusConflict, httpx.CodeIdempotencyResponseUnavailable, "the original response was too large to replay")
			c.Abort()
			return
		case StateAcquired:
			// Continue to the handler with a capturing writer.
		default:
			log.Error("idempotency acquire returned unknown state", "state", int(result.State))
			httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "idempotency store unavailable")
			c.Abort()
			return
		}

		original := c.Writer
		capture := newCaptureWriter(original)
		c.Writer = capture
		defer func() {
			// If a handler panics, restore the original writer so Gin's
			// recovery middleware can write its response to the client.
			if c.Writer == capture {
				c.Writer = original
			}
		}()

		c.Next()

		status := capture.Status()
		responseBody := capture.Body()
		contentType := original.Header().Get("Content-Type")
		c.Writer = original

		// The client may have disconnected after the handler committed. Persist
		// the outcome with a context independent of the request cancellation;
		// otherwise the record would remain in_progress and block safe replays.
		persistCtx, cancelPersist := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)

		retryable := status >= http.StatusInternalServerError ||
			status == http.StatusUnauthorized ||
			status == http.StatusForbidden ||
			status == http.StatusTooManyRequests
		if retryable {
			if err := repo.Fail(persistCtx, result.Record.ID, status, storedBody(responseBody, cfg.MaxResponseBytes), contentType); err != nil {
				log.Error("idempotency fail record update failed", "error", err, "id", result.Record.ID)
			}
		} else {
			body, truncated := storedBody(responseBody, cfg.MaxResponseBytes), false
			if int64(len(responseBody)) > cfg.MaxResponseBytes {
				body = nil
				truncated = true
			}
			if err := repo.Complete(persistCtx, result.Record.ID, status, body, contentType, truncated); err != nil {
				log.Error("idempotency complete record update failed", "error", err, "id", result.Record.ID)
			}
		}
		cancelPersist()

		original.WriteHeader(status)
		original.WriteHeaderNow()
		if status != http.StatusNoContent && status != http.StatusNotModified && len(responseBody) > 0 {
			_, _ = original.Write(responseBody)
		}
	}
}

func writeReplay(c *gin.Context, record *Record) {
	if record == nil {
		httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "idempotency store unavailable")
		c.Abort()
		return
	}

	c.Header(HeaderIdempotencyReplayed, "true")
	if record.ResponseContentType != "" {
		c.Header("Content-Type", record.ResponseContentType)
	}
	status := record.ResponseCode
	if status <= 0 {
		status = http.StatusOK
	}
	c.Writer.WriteHeader(status)
	c.Writer.WriteHeaderNow()
	if status != http.StatusNoContent && status != http.StatusNotModified && len(record.ResponseBody) > 0 {
		_, _ = c.Writer.Write(record.ResponseBody)
	}
	c.Abort()
}

func readRequestBody(request *http.Request, maxBytes int64) ([]byte, error) {
	if request.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, errBodyTooLarge
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func storedBody(body []byte, maxBytes int64) []byte {
	if int64(len(body)) > maxBytes {
		return nil
	}
	return body
}

type captureWriter struct {
	gin.ResponseWriter
	status int
	size   int
	body   bytes.Buffer
}

func newCaptureWriter(writer gin.ResponseWriter) *captureWriter {
	return &captureWriter{
		ResponseWriter: writer,
		status:         http.StatusOK,
		size:           -1,
	}
}

func (w *captureWriter) WriteHeader(code int) {
	if code > 0 && w.status != code {
		if w.Written() {
			return
		}
		w.status = code
	}
}

func (w *captureWriter) WriteHeaderNow() {
	if !w.Written() {
		w.size = 0
	}
}

func (w *captureWriter) Write(body []byte) (int, error) {
	w.WriteHeaderNow()
	n, _ := w.body.Write(body)
	w.size += n
	return n, nil
}

func (w *captureWriter) WriteString(body string) (int, error) {
	return w.Write([]byte(body))
}

func (w *captureWriter) Status() int {
	return w.status
}

func (w *captureWriter) Size() int {
	return w.size
}

func (w *captureWriter) Written() bool {
	return w.size != -1
}

func (w *captureWriter) Flush() {
	// Buffering the response means flushing would expose a partial body.
}

func (w *captureWriter) Body() []byte {
	return w.body.Bytes()
}

var _ gin.ResponseWriter = (*captureWriter)(nil)
