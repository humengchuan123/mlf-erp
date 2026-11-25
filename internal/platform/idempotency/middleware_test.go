package idempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
)

func TestMiddlewareExecutesThenReplays(t *testing.T) {
	repo := newFakeRepository()
	handlerCalls := 0
	router := newTestEngine(repo, testConfig("/test"), func(c *gin.Context) {
		handlerCalls++
		c.JSON(http.StatusCreated, gin.H{"id": "payment-1"})
	})

	first := performRequest(t, router, http.MethodPost, "/test", "key-12345678", `{"amount":"10"}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want 201; body=%s", first.Code, first.Body.String())
	}
	if handlerCalls != 1 {
		t.Fatalf("handler calls = %d, want 1", handlerCalls)
	}
	if repo.completeCalls != 1 {
		t.Fatalf("complete calls = %d, want 1", repo.completeCalls)
	}
	if got := first.Header().Get(HeaderIdempotencyReplayed); got != "" {
		t.Fatalf("first response %s = %q, want empty", HeaderIdempotencyReplayed, got)
	}

	second := performRequest(t, router, http.MethodPost, "/test", "key-12345678", `{"amount":"10"}`)
	if second.Code != http.StatusCreated {
		t.Fatalf("replay status = %d, want 201; body=%s", second.Code, second.Body.String())
	}
	if handlerCalls != 1 {
		t.Fatalf("handler calls after replay = %d, want 1", handlerCalls)
	}
	if repo.completeCalls != 1 {
		t.Fatalf("complete calls after replay = %d, want 1", repo.completeCalls)
	}
	if got := second.Header().Get(HeaderIdempotencyReplayed); got != "true" {
		t.Fatalf("replay header = %q, want true", got)
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("replay body = %q, want %q", second.Body.String(), first.Body.String())
	}
	if !strings.Contains(second.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("replay content type = %q, want application/json", second.Header().Get("Content-Type"))
	}
}

func TestMiddlewarePersistsAfterRequestCancellation(t *testing.T) {
	repo := newFakeRepository()
	var cancelRequest context.CancelFunc
	router := newTestEngine(repo, testConfig("/test"), func(c *gin.Context) {
		cancelRequest()
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancelRequest = cancel
	request := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"amount":"10"}`))
	request = request.WithContext(ctx)
	request.Header.Set(HeaderIdempotencyKey, "key-12345678")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if repo.completeCalls != 1 {
		t.Fatalf("complete calls = %d, want 1", repo.completeCalls)
	}
	if repo.lastCompleteCtxErr != nil {
		t.Fatalf("complete context error = %v, want nil", repo.lastCompleteCtxErr)
	}
}

func TestMiddlewareRejectsDifferentBody(t *testing.T) {
	repo := newFakeRepository()
	handlerCalls := 0
	router := newTestEngine(repo, testConfig("/test"), func(c *gin.Context) {
		handlerCalls++
		c.JSON(http.StatusCreated, gin.H{"ok": true})
	})

	first := performRequest(t, router, http.MethodPost, "/test", "key-12345678", `{"amount":"10"}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want 201", first.Code)
	}

	second := performRequest(t, router, http.MethodPost, "/test", "key-12345678", `{"amount":"11"}`)
	requireErrorCode(t, second, http.StatusConflict, "IDEMPOTENCY_CONFLICT")
	if handlerCalls != 1 {
		t.Fatalf("handler calls = %d, want 1", handlerCalls)
	}
}

func TestMiddlewareInProgress(t *testing.T) {
	repo := newFakeRepository()
	const body = `{"amount":"10"}`
	in := AcquireInput{
		TenantID: "tenant-1",
		UserID:   "user-1",
		Key:      "key-12345678",
		Method:   http.MethodPost,
		Path:     "/test",
		Now:      time.Now(),
	}
	in.RequestHash = RequestHash(in.Method, in.Path, []byte(body))
	repo.seed(in, statusInProgress, Record{RequestHash: in.RequestHash, ExpiresAt: time.Now().Add(time.Hour)})

	handlerCalls := 0
	router := newTestEngine(repo, testConfig("/test"), func(c *gin.Context) {
		handlerCalls++
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	response := performRequest(t, router, http.MethodPost, "/test", in.Key, body)
	requireErrorCode(t, response, http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS")
	if handlerCalls != 0 {
		t.Fatalf("handler calls = %d, want 0", handlerCalls)
	}
}

func TestMiddlewareInProgressDifferentPayload(t *testing.T) {
	repo := newFakeRepository()
	const seededBody = `{"amount":"10"}`
	const requestBody = `{"amount":"11"}`
	in := AcquireInput{
		TenantID: "tenant-1",
		UserID:   "user-1",
		Key:      "key-12345678",
		Method:   http.MethodPost,
		Path:     "/test",
		Now:      time.Now(),
	}
	in.RequestHash = RequestHash(in.Method, in.Path, []byte(seededBody))
	repo.seed(in, statusInProgress, Record{RequestHash: in.RequestHash, ExpiresAt: time.Now().Add(time.Hour)})

	handlerCalls := 0
	router := newTestEngine(repo, testConfig("/test"), func(c *gin.Context) {
		handlerCalls++
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	response := performRequest(t, router, http.MethodPost, "/test", in.Key, requestBody)
	requireErrorCode(t, response, http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS")
	if handlerCalls != 0 {
		t.Fatalf("handler calls = %d, want 0", handlerCalls)
	}
}

func TestMiddlewareResponseUnavailable(t *testing.T) {
	repo := newFakeRepository()
	const body = `{"amount":"10"}`
	in := AcquireInput{
		TenantID: "tenant-1",
		UserID:   "user-1",
		Key:      "key-12345678",
		Method:   http.MethodPost,
		Path:     "/test",
		Now:      time.Now(),
	}
	in.RequestHash = RequestHash(in.Method, in.Path, []byte(body))
	repo.seed(in, statusCompleted, Record{
		RequestHash:       in.RequestHash,
		ResponseCode:      http.StatusOK,
		ResponseTruncated: true,
		ExpiresAt:         time.Now().Add(time.Hour),
	})

	handlerCalls := 0
	router := newTestEngine(repo, testConfig("/test"), func(c *gin.Context) {
		handlerCalls++
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	response := performRequest(t, router, http.MethodPost, "/test", in.Key, body)
	requireErrorCode(t, response, http.StatusConflict, "IDEMPOTENCY_RESPONSE_UNAVAILABLE")
	if handlerCalls != 0 {
		t.Fatalf("handler calls = %d, want 0", handlerCalls)
	}
}

func TestMiddlewareMissingKey(t *testing.T) {
	t.Run("optional", func(t *testing.T) {
		repo := newFakeRepository()
		handlerCalls := 0
		router := newTestEngine(repo, testConfig("/test"), func(c *gin.Context) {
			handlerCalls++
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})

		response := performRequest(t, router, http.MethodPost, "/test", "", `{"amount":"10"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		if handlerCalls != 1 {
			t.Fatalf("handler calls = %d, want 1", handlerCalls)
		}
		if repo.acquireCalls != 0 {
			t.Fatalf("acquire calls = %d, want 0", repo.acquireCalls)
		}
	})

	t.Run("required", func(t *testing.T) {
		repo := newFakeRepository()
		handlerCalls := 0
		cfg := testConfig("/test")
		cfg.Required = true
		router := newTestEngine(repo, cfg, func(c *gin.Context) {
			handlerCalls++
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})

		response := performRequest(t, router, http.MethodPost, "/test", "", `{"amount":"10"}`)
		requireErrorCode(t, response, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
		if handlerCalls != 0 {
			t.Fatalf("handler calls = %d, want 0", handlerCalls)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		repo := newFakeRepository()
		handlerCalls := 0
		router := newTestEngine(repo, testConfig("/test"), func(c *gin.Context) {
			handlerCalls++
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})

		response := performRequest(t, router, http.MethodPost, "/test", "bad key", `{"amount":"10"}`)
		requireErrorCode(t, response, http.StatusBadRequest, "IDEMPOTENCY_KEY_INVALID")
		if handlerCalls != 0 {
			t.Fatalf("handler calls = %d, want 0", handlerCalls)
		}
	})
}

func TestMiddlewareSkipsUnprotectedRoutes(t *testing.T) {
	repo := newFakeRepository()
	handlerCalls := 0
	handler := func(c *gin.Context) {
		handlerCalls++
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(testActorMiddleware())
	middleware := Middleware(repo, testLogger(), testConfig("/protected"))
	router.POST("/protected", middleware, handler)
	router.POST("/other", middleware, handler)
	router.GET("/protected", middleware, handler)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/other"},
		{http.MethodGet, "/protected"},
	}
	for _, tc := range cases {
		response := performRequest(t, router, tc.method, tc.path, "key-12345678", "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d, want 200", tc.method, tc.path, response.Code)
		}
	}
	if handlerCalls != len(cases) {
		t.Fatalf("handler calls = %d, want %d", handlerCalls, len(cases))
	}
	if repo.acquireCalls != 0 {
		t.Fatalf("acquire calls = %d, want 0", repo.acquireCalls)
	}
}

func TestMiddlewareFailAllowsRetry(t *testing.T) {
	repo := newFakeRepository()
	handlerCalls := 0
	router := newTestEngine(repo, testConfig("/test"), func(c *gin.Context) {
		handlerCalls++
		if handlerCalls == 1 {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	first := performRequest(t, router, http.MethodPost, "/test", "key-12345678", `{"amount":"10"}`)
	if first.Code != http.StatusInternalServerError {
		t.Fatalf("first status = %d, want 500", first.Code)
	}
	if repo.failCalls != 1 || repo.completeCalls != 0 {
		t.Fatalf("fail calls = %d, complete calls = %d; want 1 and 0", repo.failCalls, repo.completeCalls)
	}

	second := performRequest(t, router, http.MethodPost, "/test", "key-12345678", `{"amount":"10"}`)
	if second.Code != http.StatusOK {
		t.Fatalf("retry status = %d, want 200; body=%s", second.Code, second.Body.String())
	}
	if handlerCalls != 2 {
		t.Fatalf("handler calls = %d, want 2", handlerCalls)
	}
	if repo.failCalls != 1 || repo.completeCalls != 1 {
		t.Fatalf("after retry fail calls = %d, complete calls = %d; want 1 and 1", repo.failCalls, repo.completeCalls)
	}
}

func TestMiddlewareRetryableStatuses(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			repo := newFakeRepository()
			router := newTestEngine(repo, testConfig("/test"), func(c *gin.Context) {
				c.JSON(status, gin.H{"ok": false})
			})

			response := performRequest(t, router, http.MethodPost, "/test", "key-12345678", `{}`)
			if response.Code != status {
				t.Fatalf("status = %d, want %d", response.Code, status)
			}
			if repo.failCalls != 1 || repo.completeCalls != 0 {
				t.Fatalf("fail calls = %d, complete calls = %d; want 1 and 0", repo.failCalls, repo.completeCalls)
			}
		})
	}
}

func TestMiddlewareTruncatesOversizedResponse(t *testing.T) {
	repo := newFakeRepository()
	cfg := testConfig("/test")
	cfg.MaxResponseBytes = 4
	router := newTestEngine(repo, cfg, func(c *gin.Context) {
		c.String(http.StatusOK, "0123456789")
	})

	first := performRequest(t, router, http.MethodPost, "/test", "key-12345678", `{}`)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", first.Code)
	}
	if got := first.Body.String(); got != "0123456789" {
		t.Fatalf("first body = %q, want full response", got)
	}
	if !repo.lastCompletedTruncated {
		t.Fatal("completed record truncated = false, want true")
	}
	if len(repo.lastCompletedBody) != 0 {
		t.Fatalf("completed record body length = %d, want 0", len(repo.lastCompletedBody))
	}

	second := performRequest(t, router, http.MethodPost, "/test", "key-12345678", `{}`)
	requireErrorCode(t, second, http.StatusConflict, "IDEMPOTENCY_RESPONSE_UNAVAILABLE")
}

func TestMiddlewareStoresEmptyNoContent(t *testing.T) {
	repo := newFakeRepository()
	router := newTestEngine(repo, testConfig("/test"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	first := performRequest(t, router, http.MethodPost, "/test", "key-12345678", `{}`)
	if first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d, want 204", first.Code)
	}
	if repo.lastCompletedTruncated {
		t.Fatal("completed record truncated = true, want false for 204")
	}
	if len(repo.lastCompletedBody) != 0 {
		t.Fatalf("completed record body length = %d, want 0", len(repo.lastCompletedBody))
	}

	second := performRequest(t, router, http.MethodPost, "/test", "key-12345678", `{}`)
	if second.Code != http.StatusNoContent {
		t.Fatalf("replay status = %d, want 204", second.Code)
	}
	if second.Body.Len() != 0 {
		t.Fatalf("replay body = %q, want empty", second.Body.String())
	}
	if got := second.Header().Get(HeaderIdempotencyReplayed); got != "true" {
		t.Fatalf("replay header = %q, want true", got)
	}
}

func TestCaptureWriter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/capture", func(c *gin.Context) {
		original := c.Writer
		capture := newCaptureWriter(original)
		c.Writer = capture

		c.Header("Content-Type", "text/plain")
		c.Writer.WriteHeader(http.StatusTeapot)
		if capture.Status() != http.StatusTeapot {
			t.Errorf("captured status = %d, want 418", capture.Status())
		}
		if capture.Written() {
			t.Error("capture written = true before WriteHeaderNow")
		}
		capture.WriteHeaderNow()
		if !capture.Written() {
			t.Error("capture written = false after WriteHeaderNow")
		}
		if capture.Size() != 0 {
			t.Errorf("captured size = %d, want 0 after WriteHeaderNow", capture.Size())
		}

		n, err := c.Writer.WriteString("hello")
		if err != nil || n != 5 {
			t.Errorf("WriteString() = %d, %v; want 5, nil", n, err)
		}
		if capture.Size() != 5 {
			t.Errorf("captured size = %d, want 5", capture.Size())
		}
		if string(capture.Body()) != "hello" {
			t.Errorf("captured body = %q, want hello", capture.Body())
		}

		c.Writer = original
		original.WriteHeader(capture.Status())
		_, _ = original.Write(capture.Body())
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/capture", nil))
	if response.Code != http.StatusTeapot {
		t.Fatalf("response status = %d, want 418", response.Code)
	}
	if response.Body.String() != "hello" {
		t.Fatalf("response body = %q, want hello", response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("response content type = %q, want text/plain", got)
	}
}

func newTestEngine(repo Repository, cfg Config, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(testActorMiddleware())
	router.POST("/test", Middleware(repo, testLogger(), cfg), handler)
	return router
}

func testActorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := auth.Actor{TenantID: "tenant-1", UserID: "user-1"}
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
		c.Next()
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testConfig(paths ...string) Config {
	protected := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		protected[path] = struct{}{}
	}
	return Config{
		TTL:              time.Hour,
		MaxBodyBytes:     1 << 20,
		MaxResponseBytes: 1 << 20,
		ProtectedPaths:   protected,
	}
}

func performRequest(t *testing.T, router http.Handler, method, path, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if key != "" {
		request.Header.Set(HeaderIdempotencyKey, key)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func requireErrorCode(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, status, response.Body.String())
	}
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v; body=%s", err, response.Body.String())
	}
	if payload.Code != code {
		t.Fatalf("error code = %q, want %q; body=%s", payload.Code, code, response.Body.String())
	}
}

type fakeScope struct {
	tenantID string
	userID   string
	key      string
	method   string
	path     string
}

type fakeEntry struct {
	id                  string
	requestHash         string
	status              string
	responseCode        int
	responseBody        []byte
	responseContentType string
	responseTruncated   bool
	expiresAt           time.Time
}

type fakeRepository struct {
	mu sync.Mutex

	entries map[fakeScope]*fakeEntry
	nextID  int

	acquireCalls           int
	completeCalls          int
	failCalls              int
	lastCompletedCode      int
	lastCompletedBody      []byte
	lastCompletedType      string
	lastCompletedTruncated bool
	lastCompleteCtxErr     error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{entries: make(map[fakeScope]*fakeEntry)}
}

func (r *fakeRepository) Acquire(_ context.Context, in AcquireInput) (AcquireResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.acquireCalls++
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	scope := scopeOf(in)
	entry := r.entries[scope]
	if entry == nil {
		r.nextID++
		entry = &fakeEntry{
			id:          fmt.Sprintf("id-%d", r.nextID),
			requestHash: in.RequestHash,
			status:      statusInProgress,
			expiresAt:   in.ExpiresAt,
		}
		r.entries[scope] = entry
		return AcquireResult{State: StateAcquired, Record: recordOf(entry)}, nil
	}

	if !entry.expiresAt.After(now) || (entry.status == statusFailed && entry.requestHash == in.RequestHash) {
		resetFakeEntry(entry, in)
		return AcquireResult{State: StateAcquired, Record: recordOf(entry)}, nil
	}

	switch entry.status {
	case statusFailed:
		return AcquireResult{State: StateConflict, Record: recordOf(entry)}, nil
	case statusCompleted:
		switch {
		case entry.requestHash != in.RequestHash:
			return AcquireResult{State: StateConflict, Record: recordOf(entry)}, nil
		case entry.responseTruncated:
			return AcquireResult{State: StateResponseUnavailable, Record: recordOf(entry)}, nil
		default:
			return AcquireResult{State: StateReplay, Record: recordOf(entry)}, nil
		}
	case statusInProgress:
		// Match the Postgres repository: an in-flight request keeps the key
		// reserved even if the retry payload differs.
		return AcquireResult{State: StateInProgress, Record: recordOf(entry)}, nil
	default:
		return AcquireResult{}, fmt.Errorf("unexpected fake status %q", entry.status)
	}
}

func (r *fakeRepository) Complete(
	ctx context.Context,
	id string,
	code int,
	body []byte,
	contentType string,
	truncated bool,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.completeCalls++
	r.lastCompleteCtxErr = ctx.Err()
	entry := r.entryByID(id)
	if entry == nil {
		return fmt.Errorf("fake record %s not found", id)
	}
	entry.status = statusCompleted
	entry.responseCode = code
	entry.responseBody = append([]byte(nil), body...)
	entry.responseContentType = contentType
	entry.responseTruncated = truncated
	r.lastCompletedCode = code
	r.lastCompletedBody = append([]byte(nil), body...)
	r.lastCompletedType = contentType
	r.lastCompletedTruncated = truncated
	return nil
}

func (r *fakeRepository) Fail(
	_ context.Context,
	id string,
	code int,
	body []byte,
	contentType string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.failCalls++
	entry := r.entryByID(id)
	if entry == nil {
		return fmt.Errorf("fake record %s not found", id)
	}
	entry.status = statusFailed
	entry.responseCode = code
	entry.responseBody = append([]byte(nil), body...)
	entry.responseContentType = contentType
	entry.responseTruncated = false
	return nil
}

func (r *fakeRepository) DeleteExpired(_ context.Context, now time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var deleted int64
	for scope, entry := range r.entries {
		if entry.expiresAt.Before(now) {
			delete(r.entries, scope)
			deleted++
		}
	}
	return deleted, nil
}

func (r *fakeRepository) seed(in AcquireInput, status string, record Record) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if record.ID == "" {
		r.nextID++
		record.ID = fmt.Sprintf("id-%d", r.nextID)
	}
	expiresAt := record.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(time.Hour)
	}
	r.entries[scopeOf(in)] = &fakeEntry{
		id:                  record.ID,
		requestHash:         record.RequestHash,
		status:              status,
		responseCode:        record.ResponseCode,
		responseBody:        append([]byte(nil), record.ResponseBody...),
		responseContentType: record.ResponseContentType,
		responseTruncated:   record.ResponseTruncated,
		expiresAt:           expiresAt,
	}
}

func (r *fakeRepository) entryByID(id string) *fakeEntry {
	for _, entry := range r.entries {
		if entry.id == id {
			return entry
		}
	}
	return nil
}

func scopeOf(in AcquireInput) fakeScope {
	return fakeScope{
		tenantID: in.TenantID,
		userID:   in.UserID,
		key:      in.Key,
		method:   in.Method,
		path:     in.Path,
	}
}

func recordOf(entry *fakeEntry) *Record {
	return &Record{
		ID:                  entry.id,
		RequestHash:         entry.requestHash,
		Status:              entry.status,
		ResponseCode:        entry.responseCode,
		ResponseBody:        append([]byte(nil), entry.responseBody...),
		ResponseContentType: entry.responseContentType,
		ResponseTruncated:   entry.responseTruncated,
		ExpiresAt:           entry.expiresAt,
	}
}

func resetFakeEntry(entry *fakeEntry, in AcquireInput) {
	entry.requestHash = in.RequestHash
	entry.status = statusInProgress
	entry.responseCode = 0
	entry.responseBody = nil
	entry.responseContentType = ""
	entry.responseTruncated = false
	entry.expiresAt = in.ExpiresAt
}
