package storage

import (
	"context"
	"io"
	"time"
)

type Object struct {
	Key         string
	Size        int64
	ContentType string
	UpdatedAt   time.Time
}

type Store interface {
	Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}
