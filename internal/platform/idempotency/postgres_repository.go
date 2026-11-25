package idempotency

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

var _ Repository = (*PostgresRepository)(nil)

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Acquire(ctx context.Context, in AcquireInput) (AcquireResult, error) {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	if in.ExpiresAt.IsZero() {
		in.ExpiresAt = now.Add(24 * time.Hour)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return AcquireResult{}, fmt.Errorf("begin idempotency acquire: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO erp.idempotency_keys (
			id, tenant_id, user_id, idempotency_key, method, path, request_hash,
			status, created_at, updated_at, expires_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6,
			'in_progress', $7, $7, $8
		)
		ON CONFLICT (tenant_id, user_id, idempotency_key, method, path) DO NOTHING
		RETURNING id::text`,
		in.TenantID, in.UserID, in.Key, in.Method, in.Path, in.RequestHash, now, in.ExpiresAt,
	).Scan(&id)
	switch {
	case err == nil:
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return AcquireResult{}, fmt.Errorf("commit idempotency insert: %w", commitErr)
		}
		return AcquireResult{
			State: StateAcquired,
			Record: &Record{
				ID:          id,
				RequestHash: in.RequestHash,
				Status:      statusInProgress,
				ExpiresAt:   in.ExpiresAt,
			},
		}, nil
	case errors.Is(err, pgx.ErrNoRows):
		// The scope already exists; lock it below.
	default:
		return AcquireResult{}, fmt.Errorf("insert idempotency key: %w", err)
	}

	record, err := scanRecord(tx.QueryRow(ctx, `
		SELECT id::text, request_hash, status,
		       COALESCE(response_code, 0),
		       COALESCE(response_body, ''),
		       COALESCE(response_content_type, ''),
		       response_truncated,
		       expires_at
		FROM erp.idempotency_keys
		WHERE tenant_id = $1 AND user_id = $2 AND idempotency_key = $3
		  AND method = $4 AND path = $5
		FOR UPDATE`,
		in.TenantID, in.UserID, in.Key, in.Method, in.Path,
	))
	if err != nil {
		return AcquireResult{}, fmt.Errorf("lock idempotency key: %w", err)
	}

	result := AcquireResult{Record: record}
	switch {
	case !record.ExpiresAt.After(now):
		if err := resetRecord(ctx, tx, record, in, now); err != nil {
			return AcquireResult{}, err
		}
		result.State = StateAcquired
	case record.Status == statusFailed:
		if record.RequestHash != in.RequestHash {
			result.State = StateConflict
			break
		}
		if err := resetRecord(ctx, tx, record, in, now); err != nil {
			return AcquireResult{}, err
		}
		result.State = StateAcquired
	case record.Status == statusCompleted:
		switch {
		case record.RequestHash != in.RequestHash:
			result.State = StateConflict
		case record.ResponseTruncated:
			result.State = StateResponseUnavailable
		default:
			result.State = StateReplay
		}
	case record.Status == statusInProgress:
		// The original request may still be committing. Even with a different
		// payload, do not invite the client to rotate the key and double-execute;
		// keep it on IDEMPOTENCY_IN_PROGRESS until the first request finishes.
		result.State = StateInProgress
	default:
		return AcquireResult{}, fmt.Errorf("idempotency record %s has unknown status %q", record.ID, record.Status)
	}

	if err := tx.Commit(ctx); err != nil {
		return AcquireResult{}, fmt.Errorf("commit idempotency acquire: %w", err)
	}
	return result, nil
}

func (r *PostgresRepository) Complete(
	ctx context.Context,
	id string,
	code int,
	body []byte,
	contentType string,
	truncated bool,
) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE erp.idempotency_keys
		SET status = 'completed',
		    response_code = $2,
		    response_body = $3,
		    response_content_type = $4,
		    response_truncated = $5,
		    updated_at = now()
		WHERE id = $1`,
		id, code, textBody(body), contentType, truncated,
	)
	if err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("complete idempotency key: record %s not found", id)
	}
	return nil
}

func (r *PostgresRepository) Fail(
	ctx context.Context,
	id string,
	code int,
	body []byte,
	contentType string,
) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE erp.idempotency_keys
		SET status = 'failed',
		    response_code = $2,
		    response_body = $3,
		    response_content_type = $4,
		    response_truncated = false,
		    updated_at = now()
		WHERE id = $1`,
		id, code, textBody(body), contentType,
	)
	if err != nil {
		return fmt.Errorf("fail idempotency key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("fail idempotency key: record %s not found", id)
	}
	return nil
}

func (r *PostgresRepository) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM erp.idempotency_keys WHERE expires_at < $1`, now)
	if err != nil {
		return 0, fmt.Errorf("delete expired idempotency keys: %w", err)
	}
	return tag.RowsAffected(), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(row rowScanner) (*Record, error) {
	var (
		record       Record
		responseBody string
	)
	if err := row.Scan(
		&record.ID,
		&record.RequestHash,
		&record.Status,
		&record.ResponseCode,
		&responseBody,
		&record.ResponseContentType,
		&record.ResponseTruncated,
		&record.ExpiresAt,
	); err != nil {
		return nil, err
	}
	record.ResponseBody = []byte(responseBody)
	return &record, nil
}

func resetRecord(ctx context.Context, tx pgx.Tx, record *Record, in AcquireInput, now time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE erp.idempotency_keys
		SET request_hash = $2,
		    status = 'in_progress',
		    response_code = NULL,
		    response_body = NULL,
		    response_content_type = NULL,
		    response_truncated = false,
		    updated_at = $3,
		    expires_at = $4
		WHERE id = $1`,
		record.ID, in.RequestHash, now, in.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("reset idempotency key: %w", err)
	}
	record.RequestHash = in.RequestHash
	record.Status = statusInProgress
	record.ResponseCode = 0
	record.ResponseBody = nil
	record.ResponseContentType = ""
	record.ResponseTruncated = false
	record.ExpiresAt = in.ExpiresAt
	return nil
}

func textBody(body []byte) any {
	if body == nil {
		return nil
	}
	return string(body)
}
