package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/approval/domain"
	"github.com/shopspring/decimal"
)

type FlowRepository struct {
	db *pgxpool.Pool
}

func NewFlowRepository(db *pgxpool.Pool) *FlowRepository {
	return &FlowRepository{db: db}
}

func (r *FlowRepository) List(ctx context.Context, tenantID string, documentType domain.DocumentType, active *bool) ([]*domain.Flow, error) {
	args := []any{tenantID}
	where := "f.tenant_id = $1"
	if documentType != "" {
		args = append(args, string(documentType))
		where += fmt.Sprintf(" AND f.document_type = $%d", len(args))
	}
	if active != nil {
		args = append(args, *active)
		where += fmt.Sprintf(" AND f.active = $%d", len(args))
	}

	rows, err := r.db.Query(ctx, flowSelectSQL+` WHERE `+where+`
		ORDER BY f.priority DESC, f.created_at DESC, f.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list approval flows: %w", err)
	}
	defer rows.Close()

	flows := make([]*domain.Flow, 0)
	for rows.Next() {
		flow, scanErr := scanFlow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		flows = append(flows, flow)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate approval flows: %w", err)
	}
	rows.Close()

	for _, flow := range flows {
		steps, stepErr := loadFlowSteps(ctx, r.db, tenantID, flow.ID)
		if stepErr != nil {
			return nil, stepErr
		}
		flow.Steps = steps
	}
	return flows, nil
}

func (r *FlowRepository) Get(ctx context.Context, tenantID, id string) (*domain.Flow, error) {
	flow, err := scanFlow(r.db.QueryRow(ctx, flowSelectSQL+`
		WHERE f.tenant_id = $1 AND f.id = $2`, tenantID, id))
	if err != nil {
		return nil, err
	}

	steps, err := loadFlowSteps(ctx, r.db, tenantID, flow.ID)
	if err != nil {
		return nil, err
	}
	flow.Steps = steps
	return flow, nil
}

func (r *FlowRepository) Create(ctx context.Context, flow *domain.Flow) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create approval flow: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.approval_flows (
			id, tenant_id, org_id, document_type, name, condition_type,
			threshold_amount, priority, active, created_by, updated_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		flow.ID, flow.TenantID, nullableUUID(flow.OrgID), string(flow.DocumentType),
		flow.Name, string(flow.ConditionType), flow.ThresholdAmount.String(),
		flow.Priority, flow.Active, flow.CreatedBy, flow.UpdatedBy,
	); err != nil {
		return fmt.Errorf("insert approval flow: %w", mapFlowWriteError(err))
	}

	if err := insertFlowSteps(ctx, tx, flow); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create approval flow: %w", err)
	}
	return nil
}

func (r *FlowRepository) Update(ctx context.Context, flow *domain.Flow) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update approval flow: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE erp.approval_flows
		SET org_id = $3, document_type = $4, name = $5, condition_type = $6,
		    threshold_amount = $7, priority = $8, active = $9,
		    updated_by = $10, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		flow.TenantID, flow.ID, nullableUUID(flow.OrgID), string(flow.DocumentType),
		flow.Name, string(flow.ConditionType), flow.ThresholdAmount.String(),
		flow.Priority, flow.Active, flow.UpdatedBy,
	)
	if err != nil {
		return fmt.Errorf("update approval flow: %w", mapFlowWriteError(err))
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM erp.approval_flow_steps
		WHERE tenant_id = $1 AND flow_id = $2`, flow.TenantID, flow.ID); err != nil {
		return fmt.Errorf("delete approval flow steps: %w", err)
	}
	if err := insertFlowSteps(ctx, tx, flow); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit update approval flow: %w", err)
	}
	return nil
}

func (r *FlowRepository) SetActive(ctx context.Context, tenantID, id string, active bool, updatedBy string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE erp.approval_flows
		SET active = $3, updated_by = $4, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, active, updatedBy,
	)
	if err != nil {
		return fmt.Errorf("set approval flow active: %w", mapFlowWriteError(err))
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *FlowRepository) Delete(ctx context.Context, tenantID, id string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete approval flow: %w", err)
	}
	defer tx.Rollback(ctx)

	var found int
	err = tx.QueryRow(ctx, `
		SELECT 1
		FROM erp.approval_flows
		WHERE tenant_id = $1 AND id = $2
		FOR UPDATE`, tenantID, id).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock approval flow: %w", mapFlowWriteError(err))
	}

	var inUse bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM erp.approval_requests
			WHERE tenant_id = $1 AND flow_id = $2
		)`, tenantID, id).Scan(&inUse); err != nil {
		return fmt.Errorf("check approval flow requests: %w", err)
	}
	if inUse {
		return domain.ErrFlowInUse
	}

	tag, err := tx.Exec(ctx, `
		DELETE FROM erp.approval_flows
		WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete approval flow: %w", mapFlowDeleteError(err))
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete approval flow: %w", err)
	}
	return nil
}

func (r *FlowRepository) FindMatching(
	ctx context.Context,
	tenantID, orgID string,
	documentType domain.DocumentType,
	baseAmount decimal.Decimal,
) (*domain.Flow, error) {
	flow, err := scanFlow(r.db.QueryRow(ctx, flowSelectSQL+`
		WHERE f.tenant_id = $1
		  AND f.document_type = $2
		  AND f.active = true
		  AND (f.org_id IS NULL OR f.org_id = $3)
		  AND (
			f.condition_type = 'always'
			OR (f.condition_type = 'amount_gte' AND f.threshold_amount <= $4)
		  )
		ORDER BY (f.org_id IS NOT NULL) DESC, f.priority DESC, f.created_at DESC, f.id
		LIMIT 1`,
		tenantID, string(documentType), nullableUUID(orgID), baseAmount.String(),
	))
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	steps, err := loadFlowSteps(ctx, r.db, tenantID, flow.ID)
	if err != nil {
		return nil, err
	}
	flow.Steps = steps
	return flow, nil
}

func (r *FlowRepository) HasRequests(ctx context.Context, tenantID, flowID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM erp.approval_requests
			WHERE tenant_id = $1 AND flow_id = $2
		)`, tenantID, flowID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check approval flow requests: %w", err)
	}
	return exists, nil
}

func insertFlowSteps(ctx context.Context, tx pgx.Tx, flow *domain.Flow) error {
	for _, step := range flow.Steps {
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.approval_flow_steps (
				id, tenant_id, flow_id, step_no, name, approver_type, approver_id
			) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			step.ID, flow.TenantID, flow.ID, step.StepNo, step.Name,
			string(step.ApproverType), step.ApproverID,
		); err != nil {
			return fmt.Errorf("insert approval flow step: %w", mapFlowWriteError(err))
		}
	}
	return nil
}

func loadFlowSteps(ctx context.Context, q querier, tenantID, flowID string) ([]domain.FlowStep, error) {
	rows, err := q.Query(ctx, `
		SELECT id::text, tenant_id::text, flow_id::text, step_no, name,
		       approver_type, approver_id::text, created_at
		FROM erp.approval_flow_steps
		WHERE tenant_id = $1 AND flow_id = $2
		ORDER BY step_no, id`, tenantID, flowID)
	if err != nil {
		return nil, fmt.Errorf("list approval flow steps: %w", err)
	}
	defer rows.Close()

	steps := make([]domain.FlowStep, 0)
	for rows.Next() {
		var step domain.FlowStep
		var approverType string
		if err := rows.Scan(
			&step.ID, &step.TenantID, &step.FlowID, &step.StepNo, &step.Name,
			&approverType, &step.ApproverID, &step.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan approval flow step: %w", err)
		}
		step.ApproverType = domain.ApproverType(approverType)
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate approval flow steps: %w", err)
	}
	return steps, nil
}

const flowSelectSQL = `
	SELECT f.id::text, f.tenant_id::text, COALESCE(f.org_id::text, ''),
	       f.document_type, f.name, f.condition_type, f.threshold_amount::text,
	       f.priority, f.active, f.created_by::text, f.updated_by::text,
	       f.created_at, f.updated_at
	FROM erp.approval_flows f`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFlow(row rowScanner) (*domain.Flow, error) {
	var flow domain.Flow
	var documentType, conditionType, thresholdAmount string

	err := row.Scan(
		&flow.ID, &flow.TenantID, &flow.OrgID, &documentType, &flow.Name,
		&conditionType, &thresholdAmount, &flow.Priority, &flow.Active,
		&flow.CreatedBy, &flow.UpdatedBy, &flow.CreatedAt, &flow.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan approval flow: %w", err)
	}
	flow.DocumentType = domain.DocumentType(documentType)
	flow.ConditionType = domain.ConditionType(conditionType)
	flow.ThresholdAmount = decimal.RequireFromString(thresholdAmount)
	return &flow, nil
}
