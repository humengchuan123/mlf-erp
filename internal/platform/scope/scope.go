package scope

import (
	"context"

	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
)

type Filter struct {
	TenantID    string
	OrgID       string
	DataScope   string
	UserID      string
	OrgPath     string
	CompanyPath string
}

func FromActor(ctx context.Context, tenantID string) Filter {
	actor, _ := auth.ActorFrom(ctx)
	dataScope := actor.DataScope
	if dataScope == "" {
		dataScope = "self"
	}

	return Filter{
		TenantID:    tenantID,
		OrgID:       actor.OrgID,
		DataScope:   dataScope,
		UserID:      actor.UserID,
		OrgPath:     actor.OrgPath,
		CompanyPath: actor.CompanyPath,
	}
}
