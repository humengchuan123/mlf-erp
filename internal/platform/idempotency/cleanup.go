package idempotency

import (
	"context"
	"log/slog"
	"time"
)

func RunCleanup(ctx context.Context, repo Repository, log *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	if log == nil {
		log = slog.Default()
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := repo.DeleteExpired(ctx, time.Now()); err != nil {
				log.Warn("idempotency cleanup failed", "error", err)
			} else if n > 0 {
				log.Info("idempotency cleanup", "deleted", n)
			}
		}
	}
}
