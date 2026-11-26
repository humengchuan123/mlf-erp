package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/approval/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type RequestRepository struct {
	db *pgxpool.Pool
}

func NewRequestRepository(db *pgxpool.Pool) *RequestRepository {
	return &RequestRepository{db: db}
}

func (r *RequestRepository) Create(ctx context.Context, request *domain.Request) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create approval request: %w", err)
	}
	defer tx.Rollback(ctx)

	// Share-lock the flow so its steps cannot change between resolution and
	// insert. Flow updates and deletes take FOR UPDATE and therefore wait.
	var found int
	err = tx.QueryRow(ctx, `
		SELECT 1
		FROM erp.approval_flows
		WHERE tenant_id = $1 AND id = $2
		FOR SHARE`, request.TenantID, request.FlowID).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrInvalidFlow
	}
	if err != nil {
		return fmt.Errorf("lock approval flow: %w", err)
	}

	steps, err := loadFlowSteps(ctx, tx, request.TenantID, request.FlowID)
	if err != nil {
		return err
	}
	resolved, err := domain.ResolveApprovalSteps(steps, request.RequestedBy)
	if err != nil {
		return err
	}
	request.CurrentStep = resolved.CurrentStep

	_, err = tx.Exec(ctx, `
		INSERT INTO erp.approval_requests (
			id, tenant_id, org_id, document_type, document_id, document_code,
			flow_id, status, current_step, requested_by, requested_at,
			amount, currency, base_amount
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		request.ID, request.TenantID, request.OrgID, string(request.DocumentType),
		request.DocumentID, request.DocumentCode, request.FlowID,
		string(request.Status), request.CurrentStep, request.RequestedBy,
		request.RequestedAt, request.Amount.String(), request.Currency,
		request.BaseAmount.String(),
	)
	if err != nil {
		return fmt.Errorf("insert approval request: %w", mapRequestCreateError(err))
	}
	if err := insertSkipActions(ctx, tx, request.TenantID, request.ID, request.RequestedBy, resolved.Skipped); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create approval request: %w", err)
	}
	return nil
}

func (r *RequestRepository) Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*domain.Request, error) {
	args := []any{tenantID, id}
	where := "r.tenant_id = $1 AND r.id = $2"
	appendApprovalScope(&where, &args, "r", filter)

	request, err := scanRequest(r.db.QueryRow(ctx, requestSelectSQL+` WHERE `+where, args...))
	if err != nil {
		return nil, err
	}

	steps, err := loadFlowSteps(ctx, r.db, tenantID, request.FlowID)
	if err != nil {
		return nil, err
	}
	request.Steps = steps

	actions, err := loadApprovalActions(ctx, r.db, tenantID, request.ID)
	if err != nil {
		return nil, err
	}
	request.Actions = actions
	return request, nil
}

func (r *RequestRepository) List(
	ctx context.Context,
	filter scope.Filter,
	tenantID string,
	documentType domain.DocumentType,
	documentID, status string,
	page, pageSize int,
) ([]*domain.Request, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	args := []any{tenantID}
	where := "r.tenant_id = $1"
	if documentType != "" {
		args = append(args, string(documentType))
		where += fmt.Sprintf(" AND r.document_type = $%d", len(args))
	}
	if documentID != "" {
		args = append(args, documentID)
		where += fmt.Sprintf(" AND r.document_id = $%d", len(args))
	}
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND r.status = $%d", len(args))
	}
	appendApprovalScope(&where, &args, "r", filter)

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM erp.approval_requests r WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count approval requests: %w", err)
	}

	query := requestSelectSQL + ` WHERE ` + where +
		fmt.Sprintf(` ORDER BY r.requested_at DESC, r.created_at DESC, r.id
			LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list approval requests: %w", err)
	}
	defer rows.Close()

	requests := make([]*domain.Request, 0)
	for rows.Next() {
		request, scanErr := scanRequest(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate approval requests: %w", err)
	}
	rows.Close()

	for _, request := range requests {
		steps, stepErr := loadFlowSteps(ctx, r.db, tenantID, request.FlowID)
		if stepErr != nil {
			return nil, 0, stepErr
		}
		request.Steps = steps
		actions, actionErr := loadApprovalActions(ctx, r.db, tenantID, request.ID)
		if actionErr != nil {
			return nil, 0, actionErr
		}
		request.Actions = actions
	}
	return requests, total, nil
}

func (r *RequestRepository) ListPending(
	ctx context.Context,
	filter scope.Filter,
	tenantID string,
	actor domain.Actor,
) ([]*domain.Request, error) {
	if actor.UserID == "" {
		return []*domain.Request{}, nil
	}

	args := []any{tenantID, actor.UserID}
	where := "r.tenant_id = $1 AND r.status = 'pending' AND r.requested_by <> $2"
	if !actor.IsSuperAdmin {
		where += `
			AND EXISTS (
				SELECT 1
				FROM erp.approval_flow_steps s
				WHERE s.tenant_id = r.tenant_id
				  AND s.flow_id = r.flow_id
				  AND s.step_no = r.current_step
				  AND (
					(s.approver_type = 'user' AND s.approver_id = $2)
					OR (s.approver_type = 'role' AND EXISTS (
						SELECT 1
						FROM erp.user_roles ur
						WHERE ur.tenant_id = r.tenant_id
						  AND ur.user_id = $2
						  AND ur.role_id = s.approver_id
					))
				  )
			)`
	}
	appendApprovalOrgScope(&where, &args, "r", filter)

	rows, err := r.db.Query(ctx, requestSelectSQL+` WHERE `+where+`
		ORDER BY r.requested_at ASC, r.created_at ASC, r.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list pending approval requests: %w", err)
	}
	defer rows.Close()

	requests := make([]*domain.Request, 0)
	for rows.Next() {
		request, scanErr := scanRequest(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending approval requests: %w", err)
	}
	rows.Close()

	for _, request := range requests {
		steps, stepErr := loadFlowSteps(ctx, r.db, tenantID, request.FlowID)
		if stepErr != nil {
			return nil, stepErr
		}
		request.Steps = steps
	}
	return requests, nil
}

func (r *RequestRepository) GetPendingByDocument(
	ctx context.Context,
	tenantID string,
	documentType domain.DocumentType,
	documentID string,
) (*domain.Request, error) {
	request, err := scanRequest(r.db.QueryRow(ctx, requestSelectSQL+`
		WHERE r.tenant_id = $1
		  AND r.document_type = $2
		  AND r.document_id = $3
		  AND r.status = 'pending'
		ORDER BY r.requested_at DESC, r.created_at DESC
		LIMIT 1`, tenantID, string(documentType), documentID))
	if err != nil {
		return nil, err
	}

	steps, err := loadFlowSteps(ctx, r.db, tenantID, request.FlowID)
	if err != nil {
		return nil, err
	}
	request.Steps = steps
	return request, nil
}

func (r *RequestRepository) Approve(
	ctx context.Context,
	tenantID, requestID string,
	actor domain.Actor,
	comment string,
) (bool, error) {
	return r.act(ctx, tenantID, requestID, actor, comment, domain.ActionApprove)
}

func (r *RequestRepository) Reject(
	ctx context.Context,
	tenantID, requestID string,
	actor domain.Actor,
	comment string,
) error {
	_, err := r.act(ctx, tenantID, requestID, actor, comment, domain.ActionReject)
	return err
}

func (r *RequestRepository) act(
	ctx context.Context,
	tenantID, requestID string,
	actor domain.Actor,
	comment string,
	action domain.ActionType,
) (bool, error) {
	if actor.UserID == "" {
		return false, domain.ErrNotApprover
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin approval action: %w", err)
	}
	defer tx.Rollback(ctx)

	request, steps, err := r.lockPendingRequest(ctx, tx, tenantID, requestID)
	if err != nil {
		return false, err
	}

	step, ok := findFlowStep(steps, request.CurrentStep)
	if !ok {
		return false, domain.ErrInvalidFlow
	}

	if request.RequestedBy == actor.UserID {
		return false, domain.ErrSelfApproval
	}
	if !actor.IsSuperAdmin {
		switch step.ApproverType {
		case domain.ApproverUser:
			if step.ApproverID != actor.UserID {
				return false, domain.ErrNotApprover
			}
		case domain.ApproverRole:
			member, err := userHasRole(ctx, tx, tenantID, actor.UserID, step.ApproverID)
			if err != nil {
				return false, err
			}
			if !member {
				return false, domain.ErrNotApprover
			}
		default:
			return false, domain.ErrInvalidFlow
		}
	}

	if err := insertApprovalAction(ctx, tx, tenantID, request.ID, request.CurrentStep, action, actor.UserID, comment); err != nil {
		return false, err
	}

	final := false
	newStatus := domain.RequestPending
	newStep := request.CurrentStep
	var completedAt *time.Time
	var skipped []domain.FlowStep
	if action == domain.ActionApprove {
		next, skippedSteps, hasNext := nextStepAndSkipped(steps, request.CurrentStep, request.RequestedBy)
		skipped = skippedSteps
		if hasNext {
			newStep = next.StepNo
		} else {
			newStatus = domain.RequestApproved
			now := time.Now()
			completedAt = &now
			final = true
		}
	} else {
		newStatus = domain.RequestRejected
		now := time.Now()
		completedAt = &now
	}

	if err := insertSkipActions(ctx, tx, tenantID, request.ID, request.RequestedBy, skipped); err != nil {
		return false, err
	}

	updated, err := updateRequestState(ctx, tx, tenantID, request.ID, request.CurrentStep, newStatus, newStep, completedAt)
	if err != nil {
		return false, err
	}
	if !updated {
		return false, domain.ErrAlreadyActed
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit approval action: %w", err)
	}
	return final, nil
}

func (r *RequestRepository) CancelByDocument(
	ctx context.Context,
	tenantID string,
	documentType domain.DocumentType,
	documentID, actorID, reason string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin cancel approval request: %w", err)
	}
	defer tx.Rollback(ctx)

	request, err := scanRequest(tx.QueryRow(ctx, requestSelectSQL+`
		WHERE r.tenant_id = $1
		  AND r.document_type = $2
		  AND r.document_id = $3
		  AND r.status = 'pending'
		ORDER BY r.requested_at DESC, r.created_at DESC
		LIMIT 1
		FOR UPDATE`, tenantID, string(documentType), documentID))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ErrNoPending
	}
	if err != nil {
		return err
	}

	if err := insertApprovalAction(ctx, tx, tenantID, request.ID, request.CurrentStep, domain.ActionCancel, actorID, reason); err != nil {
		return err
	}

	now := time.Now()
	updated, err := updateRequestState(ctx, tx, tenantID, request.ID, request.CurrentStep, domain.RequestCancelled, request.CurrentStep, &now)
	if err != nil {
		return err
	}
	if !updated {
		return domain.ErrInvalidStatus
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit cancel approval request: %w", err)
	}
	return nil
}

func (r *RequestRepository) lockPendingRequest(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, requestID string,
) (*domain.Request, []domain.FlowStep, error) {
	request, err := scanRequest(tx.QueryRow(ctx, requestSelectSQL+`
		WHERE r.tenant_id = $1 AND r.id = $2
		FOR UPDATE`, tenantID, requestID))
	if err != nil {
		return nil, nil, err
	}
	if request.Status != domain.RequestPending {
		return nil, nil, domain.ErrInvalidStatus
	}

	steps, err := loadFlowSteps(ctx, tx, tenantID, request.FlowID)
	if err != nil {
		return nil, nil, err
	}
	return request, steps, nil
}

const skippedStepComment = "applicant step skipped"

func insertApprovalAction(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, requestID string,
	stepNo int,
	action domain.ActionType,
	actorID, comment string,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO erp.approval_actions (
			id, tenant_id, request_id, step_no, action, actor_id, comment
		) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		uuid.NewString(), tenantID, requestID, stepNo, string(action), actorID, comment,
	)
	if err != nil {
		return fmt.Errorf("insert approval action: %w", mapActionInsertError(err))
	}
	return nil
}

// insertSkipActions records steps skipped because they were assigned to the
// requester. ON CONFLICT DO NOTHING keeps repeated resolutions idempotent
// against the approval_actions_skip_idx unique index.
func insertSkipActions(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, requestID, requestedBy string,
	steps []domain.FlowStep,
) error {
	for _, step := range steps {
		_, err := tx.Exec(ctx, `
			INSERT INTO erp.approval_actions (
				id, tenant_id, request_id, step_no, action, actor_id, comment
			) VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT DO NOTHING`,
			uuid.NewString(), tenantID, requestID, step.StepNo,
			string(domain.ActionSkip), requestedBy, skippedStepComment,
		)
		if err != nil {
			return fmt.Errorf("insert approval skip action: %w", mapActionInsertError(err))
		}
	}
	return nil
}

func updateRequestState(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, requestID string,
	oldStep int,
	status domain.RequestStatus,
	currentStep int,
	completedAt *time.Time,
) (bool, error) {
	var completed any
	if completedAt != nil {
		completed = *completedAt
	}

	tag, err := tx.Exec(ctx, `
		UPDATE erp.approval_requests
		SET status = $3, current_step = $4, completed_at = $5, updated_at = now()
		WHERE tenant_id = $1
		  AND id = $2
		  AND status = 'pending'
		  AND current_step = $6`,
		tenantID, requestID, string(status), currentStep, completed, oldStep,
	)
	if err != nil {
		return false, fmt.Errorf("update approval request state: %w", mapRequestWriteError(err))
	}
	return tag.RowsAffected() == 1, nil
}

func userHasRole(ctx context.Context, q querier, tenantID, userID, roleID string) (bool, error) {
	var exists bool
	if err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM erp.user_roles
			WHERE tenant_id = $1 AND user_id = $2 AND role_id = $3
		)`, tenantID, userID, roleID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check approval role member: %w", err)
	}
	return exists, nil
}

func findFlowStep(steps []domain.FlowStep, stepNo int) (domain.FlowStep, bool) {
	for _, step := range steps {
		if step.StepNo == stepNo {
			return step, true
		}
	}
	return domain.FlowStep{}, false
}

// nextStepAndSkipped returns the lowest eligible step after currentStep and
// every user step after currentStep assigned to the requester. Role steps are
// retained even when the requester holds the role. When no eligible step
// remains the request is complete.
func nextStepAndSkipped(steps []domain.FlowStep, currentStep int, requestedBy string) (domain.FlowStep, []domain.FlowStep, bool) {
	var selected domain.FlowStep
	found := false
	skipped := make([]domain.FlowStep, 0)

	for _, step := range steps {
		if step.StepNo <= currentStep {
			continue
		}
		if step.ApproverType == domain.ApproverUser && step.ApproverID == requestedBy {
			skipped = append(skipped, step)
			continue
		}
		if !found || step.StepNo < selected.StepNo {
			selected = step
			found = true
		}
	}
	sort.Slice(skipped, func(i, j int) bool { return skipped[i].StepNo < skipped[j].StepNo })
	return selected, skipped, found
}

func loadApprovalActions(ctx context.Context, q querier, tenantID, requestID string) ([]domain.Action, error) {
	rows, err := q.Query(ctx, `
		SELECT id::text, tenant_id::text, request_id::text, step_no, action,
		       actor_id::text, comment, created_at
		FROM erp.approval_actions
		WHERE tenant_id = $1 AND request_id = $2
		ORDER BY created_at, id`, tenantID, requestID)
	if err != nil {
		return nil, fmt.Errorf("list approval actions: %w", err)
	}
	defer rows.Close()

	actions := make([]domain.Action, 0)
	for rows.Next() {
		var action domain.Action
		var actionType string
		if err := rows.Scan(
			&action.ID, &action.TenantID, &action.RequestID, &action.StepNo,
			&actionType, &action.ActorID, &action.Comment, &action.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan approval action: %w", err)
		}
		action.Action = domain.ActionType(actionType)
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate approval actions: %w", err)
	}
	return actions, nil
}

const requestSelectSQL = `
	SELECT r.id::text, r.tenant_id::text, r.org_id::text, r.document_type,
	       r.document_id::text, r.document_code, r.flow_id::text, r.status,
	       r.current_step, r.requested_by::text, r.requested_at, r.completed_at,
	       r.amount::text, r.currency, r.base_amount::text, r.created_at, r.updated_at
	FROM erp.approval_requests r`

func scanRequest(row rowScanner) (*domain.Request, error) {
	var request domain.Request
	var documentType, status string
	var amount, baseAmount string
	var completedAt *time.Time

	err := row.Scan(
		&request.ID, &request.TenantID, &request.OrgID, &documentType,
		&request.DocumentID, &request.DocumentCode, &request.FlowID, &status,
		&request.CurrentStep, &request.RequestedBy, &request.RequestedAt,
		&completedAt, &amount, &request.Currency, &baseAmount,
		&request.CreatedAt, &request.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan approval request: %w", err)
	}
	request.DocumentType = domain.DocumentType(documentType)
	request.Status = domain.RequestStatus(status)
	request.CompletedAt = completedAt
	request.Amount = decimal.RequireFromString(amount)
	request.BaseAmount = decimal.RequireFromString(baseAmount)
	return &request, nil
}
