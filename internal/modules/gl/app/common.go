package app

import (
	"context"
	"strings"

	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/tenant"
)

type CodeGenerator interface {
	Next(ctx context.Context, tenantID, entity string) (string, error)
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

func trim(value string) string {
	return strings.TrimSpace(value)
}
