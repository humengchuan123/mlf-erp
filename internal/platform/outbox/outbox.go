package outbox

import (
	"context"
	"time"
)

type Event struct {
	ID            string
	TenantID      string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	AvailableAt   time.Time
}

type Store interface {
	Append(ctx context.Context, event Event) error
	FetchPending(ctx context.Context, limit int) ([]Event, error)
	MarkPublished(ctx context.Context, id string) error
}
