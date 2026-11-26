package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mlf-erp/mlf-erp/internal/modules/approval/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// appendApprovalScope applies the actor's data scope to an approval request
// query. Self scope must include requests the actor is assigned to approve,
// not just requests the actor created.
func appendApprovalScope(where *string, args *[]any, alias string, filter scope.Filter) {
	if filter.DataScope == "all" {
		return
	}
	if filter.DataScope == "" {
		filter.DataScope = "self"
	}

	switch filter.DataScope {
	case "dept":
		appendApprovalOrgPath(where, args, alias, filter.OrgPath)
	case "company":
		appendApprovalOrgPath(where, args, alias, filter.CompanyPath)
	case "self":
		appendApprovalSelfScope(where, args, alias, filter.UserID)
	default:
		appendApprovalSelfScope(where, args, alias, filter.UserID)
	}
}

// appendApprovalOrgScope applies only department/company organization scope.
// It is used for the pending list because self scope there is expressed by the
// current-step approver condition instead of created_by.
func appendApprovalOrgScope(where *string, args *[]any, alias string, filter scope.Filter) {
	switch filter.DataScope {
	case "dept":
		appendApprovalOrgPath(where, args, alias, filter.OrgPath)
	case "company":
		appendApprovalOrgPath(where, args, alias, filter.CompanyPath)
	}
}

func appendApprovalSelfScope(where *string, args *[]any, alias, userID string) {
	if userID == "" {
		*where += " AND 1 = 0"
		return
	}

	*args = append(*args, userID)
	index := len(*args)
	*where += fmt.Sprintf(
		` AND (%[1]s.requested_by = $%[2]d
			OR EXISTS (
				SELECT 1
				FROM erp.approval_flow_steps s
				WHERE s.tenant_id = %[1]s.tenant_id
				  AND s.flow_id = %[1]s.flow_id
				  AND (
					(s.approver_type = 'user' AND s.approver_id = $%[2]d)
					OR (s.approver_type = 'role' AND EXISTS (
						SELECT 1
						FROM erp.user_roles ur
						WHERE ur.tenant_id = %[1]s.tenant_id
						  AND ur.user_id = $%[2]d
						  AND ur.role_id = s.approver_id
					))
				  )
			))`,
		alias, index,
	)
}

func appendApprovalOrgPath(where *string, args *[]any, alias, path string) {
	if path == "" {
		*where += " AND 1 = 0"
		return
	}

	*args = append(*args, path)
	index := len(*args)
	*where += fmt.Sprintf(
		" AND EXISTS (SELECT 1 FROM erp.organizations so WHERE so.tenant_id = %s.tenant_id AND so.id = %s.org_id AND (so.path = $%d OR so.path LIKE $%d || '/%%'))",
		alias, alias, index, index,
	)
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func hasPgCode(err error, codes ...string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	for _, code := range codes {
		if pgErr.Code == code {
			return true
		}
	}
	return false
}

func mapFlowWriteError(err error) error {
	if hasPgCode(err, "23505", "23503", "23514", "23502", "22P02", "22003") {
		return domain.ErrInvalidFlow
	}
	return err
}

func mapFlowDeleteError(err error) error {
	if hasPgCode(err, "23503") {
		return domain.ErrFlowInUse
	}
	return mapFlowWriteError(err)
}

func mapRequestCreateError(err error) error {
	if hasPgCode(err, "23505") {
		return domain.ErrPendingExists
	}
	if hasPgCode(err, "23503", "23514", "23502", "22P02", "22003") {
		return domain.ErrInvalidFlow
	}
	return err
}

func mapRequestWriteError(err error) error {
	if hasPgCode(err, "23503", "23514", "23502", "22P02", "22003") {
		return domain.ErrInvalidFlow
	}
	return err
}

func mapActionInsertError(err error) error {
	if hasPgCode(err, "23505") {
		return domain.ErrAlreadyActed
	}
	if hasPgCode(err, "23503") {
		return domain.ErrNotFound
	}
	if hasPgCode(err, "23514", "23502", "22P02", "22003") {
		return domain.ErrInvalidFlow
	}
	return err
}
