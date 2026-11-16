package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

type AuditRepository struct {
	db *pgxpool.Pool
}

func NewAuditRepository(db *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(ctx context.Context, log *domain.AuditLog) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO erp.audit_logs (
			id, tenant_id, org_id, user_id, action, resource_type, resource_id,
			request_id, ip, user_agent, before_data, after_data, created_at
		) VALUES (
			$1, $2, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid,
			$5, $6, $7, $8, $9, $10, $11, $12, now()
		)`,
		log.ID, log.TenantID, log.OrgID, log.UserID, log.Action, log.ResourceType,
		log.ResourceID, log.RequestID, log.IP, log.UserAgent, log.BeforeData, log.AfterData,
	)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

func (r *AuditRepository) List(ctx context.Context, tenantID string, offset, limit int) ([]*domain.AuditLog, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM erp.audit_logs WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT id::text, tenant_id::text, COALESCE(org_id::text, ''), COALESCE(user_id::text, ''),
		       action, resource_type, resource_id, request_id, ip, user_agent,
		       before_data, after_data, created_at
		FROM erp.audit_logs
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, tenantID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	result := make([]*domain.AuditLog, 0)
	for rows.Next() {
		log := &domain.AuditLog{}
		if err := rows.Scan(
			&log.ID, &log.TenantID, &log.OrgID, &log.UserID,
			&log.Action, &log.ResourceType, &log.ResourceID, &log.RequestID,
			&log.IP, &log.UserAgent, &log.BeforeData, &log.AfterData, &log.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		result = append(result, log)
	}
	return result, total, rows.Err()
}
