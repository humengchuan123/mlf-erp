package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	HeaderIdempotencyKey      = "Idempotency-Key"
	HeaderIdempotencyReplayed = "Idempotency-Replayed"
)

const (
	statusInProgress = "in_progress"
	statusCompleted  = "completed"
	statusFailed     = "failed"
)

var (
	ErrInvalidKey = errors.New("invalid idempotency key")

	keyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)
)

type State int

const (
	StateAcquired State = iota
	StateReplay
	StateConflict
	StateInProgress
	StateResponseUnavailable
)

type Record struct {
	ID                  string
	RequestHash         string
	Status              string
	ResponseCode        int
	ResponseBody        []byte
	ResponseContentType string
	ResponseTruncated   bool
	ExpiresAt           time.Time
}

type AcquireInput struct {
	TenantID    string
	UserID      string
	Key         string
	Method      string
	Path        string
	RequestHash string
	Now         time.Time
	ExpiresAt   time.Time
}

type AcquireResult struct {
	State  State
	Record *Record
}

func NormalizeKey(key string) string {
	return strings.TrimSpace(key)
}

func ValidateKey(key string) error {
	if !keyPattern.MatchString(NormalizeKey(key)) {
		return ErrInvalidKey
	}
	return nil
}

func RequestHash(method, path string, body []byte) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(method))
	_, _ = digest.Write([]byte("\n"))
	_, _ = digest.Write([]byte(path))
	_, _ = digest.Write([]byte("\n"))
	_, _ = digest.Write(body)
	return hex.EncodeToString(digest.Sum(nil))
}
