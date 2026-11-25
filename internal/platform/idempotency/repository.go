package idempotency

import (
	"context"
	"time"
)

type Repository interface {
	Acquire(ctx context.Context, in AcquireInput) (AcquireResult, error)
	Complete(ctx context.Context, id string, code int, body []byte, contentType string, truncated bool) error
	Fail(ctx context.Context, id string, code int, body []byte, contentType string) error
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}
