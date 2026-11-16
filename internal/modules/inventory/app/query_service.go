package app

import (
	"context"

	"github.com/mlf-erp/mlf-erp/internal/modules/inventory/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/mlf-erp/mlf-erp/internal/platform/tenant"
)

type QueryService struct {
	repo domain.QueryRepository
}

func NewQueryService(repo domain.QueryRepository) *QueryService {
	return &QueryService{repo: repo}
}

func (s *QueryService) ListBalances(ctx context.Context, warehouseID, materialID, keyword string, page, pageSize int) ([]*domain.Balance, int, error) {
	tenantID := tenantIDFrom(ctx)
	page, pageSize = normalizePage(page, pageSize)
	filter := scope.FromActor(ctx, tenantID)
	applyInventoryScope(&filter, ctx)
	return s.repo.ListBalances(ctx, filter, tenantID, warehouseID, materialID, keyword, (page-1)*pageSize, pageSize)
}

func (s *QueryService) ListTransactions(ctx context.Context, warehouseID, materialID, keyword string, page, pageSize int) ([]*domain.Transaction, int, error) {
	tenantID := tenantIDFrom(ctx)
	page, pageSize = normalizePage(page, pageSize)
	filter := scope.FromActor(ctx, tenantID)
	applyInventoryScope(&filter, ctx)
	return s.repo.ListTransactions(ctx, filter, tenantID, warehouseID, materialID, keyword, (page-1)*pageSize, pageSize)
}

func tenantIDFrom(ctx context.Context) string {
	info, _ := tenant.From(ctx)
	return info.ID
}

func applyInventoryScope(filter *scope.Filter, ctx context.Context) {
	actor, _ := auth.ActorFrom(ctx)
	if filter.DataScope == "self" {
		filter.OrgPath = actor.OrgPath
	}
	if filter.DataScope == "company" {
		filter.CompanyPath = actor.CompanyPath
	}
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
