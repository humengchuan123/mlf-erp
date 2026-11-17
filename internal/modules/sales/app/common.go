package app

import (
	"context"

	masterdomain "github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/mlf-erp/mlf-erp/internal/platform/tenant"
)

type CodeGenerator interface {
	Next(ctx context.Context, tenantID, entity string) (string, error)
}

type PartnerReader interface {
	FindByID(ctx context.Context, filter scope.Filter, tenantID, id string) (masterdomain.Partner, error)
}

type MaterialReader interface {
	FindByID(ctx context.Context, filter scope.Filter, tenantID, id string) (masterdomain.Material, error)
}

type WarehouseReader interface {
	FindByID(ctx context.Context, filter scope.Filter, tenantID, id string) (masterdomain.Warehouse, error)
}

type TaxRateReader interface {
	FindByID(ctx context.Context, filter scope.Filter, tenantID, id string) (masterdomain.TaxRate, error)
}

func tenantActor(ctx context.Context) (tenantID, orgID, userID string) {
	if info, ok := tenant.From(ctx); ok {
		tenantID = info.ID
		orgID = info.OrgID
	}
	if actor, ok := auth.ActorFrom(ctx); ok {
		userID = actor.UserID
		if orgID == "" {
			orgID = actor.OrgID
		}
	}
	return tenantID, orgID, userID
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}
