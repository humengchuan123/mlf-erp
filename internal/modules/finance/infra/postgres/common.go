package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mlf-erp/mlf-erp/internal/modules/finance/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

func appendScope(where *string, args *[]any, alias string, filter scope.Filter) {
	if filter.DataScope == "all" {
		return
	}
	if filter.DataScope == "" {
		filter.DataScope = "self"
	}

	switch filter.DataScope {
	case "self":
		if filter.UserID == "" {
			*where += " AND 1 = 0"
			return
		}
		*args = append(*args, filter.UserID)
		*where += fmt.Sprintf(" AND %s.created_by = $%d", alias, len(*args))
	case "dept":
		if filter.OrgPath == "" {
			*where += " AND 1 = 0"
			return
		}
		*args = append(*args, filter.OrgPath)
		*where += fmt.Sprintf(
			" AND EXISTS (SELECT 1 FROM erp.organizations so WHERE so.tenant_id = %s.tenant_id AND so.id = %s.org_id AND (so.path = $%d OR so.path LIKE $%d || '/%%'))",
			alias, alias, len(*args), len(*args),
		)
	case "company":
		if filter.CompanyPath == "" {
			*where += " AND 1 = 0"
			return
		}
		*args = append(*args, filter.CompanyPath)
		*where += fmt.Sprintf(
			" AND EXISTS (SELECT 1 FROM erp.organizations so WHERE so.tenant_id = %s.tenant_id AND so.id = %s.org_id AND (so.path = $%d OR so.path LIKE $%d || '/%%'))",
			alias, alias, len(*args), len(*args),
		)
	default:
		if filter.UserID == "" {
			*where += " AND 1 = 0"
			return
		}
		*args = append(*args, filter.UserID)
		*where += fmt.Sprintf(" AND %s.created_by = $%d", alias, len(*args))
	}
}

func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23503":
			return domain.ErrConflict
		case "22P02", "23502", "22003":
			return domain.ErrInvalidInput
		}
	}
	return err
}
