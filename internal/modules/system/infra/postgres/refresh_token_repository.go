package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

type RefreshTokenRepository struct {
	db *pgxpool.Pool
}

func NewRefreshTokenRepository(db *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token *domain.RefreshToken) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO erp.refresh_tokens (
			id, tenant_id, user_id, token_hash, expires_at, user_agent, ip, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, now())`,
		token.ID, token.TenantID, token.UserID, token.TokenHash, token.ExpiresAt, token.UserAgent, token.IP,
	)
	if err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, user_id::text, token_hash,
		       expires_at, revoked_at, user_agent, ip, created_at
		FROM erp.refresh_tokens
		WHERE token_hash = $1`, hash)

	var token domain.RefreshToken
	var revokedAt *time.Time
	err := row.Scan(
		&token.ID, &token.TenantID, &token.UserID, &token.TokenHash,
		&token.ExpiresAt, &revokedAt, &token.UserAgent, &token.IP, &token.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan refresh token: %w", err)
	}
	token.RevokedAt = revokedAt
	return &token, nil
}

func (r *RefreshTokenRepository) RevokeByHash(ctx context.Context, hash string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE erp.refresh_tokens
		SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, tenantID, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE erp.refresh_tokens
		SET revoked_at = now()
		WHERE tenant_id = $1 AND user_id = $2 AND revoked_at IS NULL`, tenantID, userID)
	if err != nil {
		return fmt.Errorf("revoke user refresh tokens: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM erp.refresh_tokens WHERE expires_at < now()`)
	if err != nil {
		return fmt.Errorf("delete expired refresh tokens: %w", err)
	}
	return nil
}
