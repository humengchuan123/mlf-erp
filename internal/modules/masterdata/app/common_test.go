package app

import (
	"context"
	"testing"

	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/tenant"
)

func TestTenantActorUsesTenantOrgBeforeActorOrg(t *testing.T) {
	ctx := tenant.With(context.Background(), tenant.Info{ID: "tenant-1", OrgID: "org-tenant"})
	ctx = auth.WithActor(ctx, auth.Actor{UserID: "user-1", TenantID: "tenant-1", OrgID: "org-actor"})

	tenantID, orgID, userID := tenantActor(ctx)
	if tenantID != "tenant-1" || orgID != "org-tenant" || userID != "user-1" {
		t.Fatalf("tenantActor() = (%q, %q, %q), want (tenant-1, org-tenant, user-1)", tenantID, orgID, userID)
	}
}

func TestTenantActorFallsBackToActorOrg(t *testing.T) {
	ctx := tenant.With(context.Background(), tenant.Info{ID: "tenant-1"})
	ctx = auth.WithActor(ctx, auth.Actor{UserID: "user-1", TenantID: "tenant-1", OrgID: "org-actor"})

	_, orgID, _ := tenantActor(ctx)
	if orgID != "org-actor" {
		t.Fatalf("tenantActor() org = %q, want org-actor", orgID)
	}
}
