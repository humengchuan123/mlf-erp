package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

type LoginLogRepository struct {
	db *pgxpool.Pool
}

func NewLoginLogRepository(db *pgxpool.Pool) *LoginLogRepository {
	return &LoginLogRepository{db: db}
}

func (r *LoginLogRepository) Create(ctx context.Context, log *domain.LoginLog) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO erp.login_logs (
			id, tenant_id, user_id, username, status, failure_reason, ip, user_agent, created_at
		) VALUES (
			$1, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid,
			$4, $5, $6, $7, $8, now()
		)`,
		log.ID, log.TenantID, log.UserID, log.Username,
		log.Status, log.FailureReason, log.IP, log.UserAgent,
	)
	if err != nil {
		return fmt.Errorf("create login log: %w", err)
	}
	return nil
}
