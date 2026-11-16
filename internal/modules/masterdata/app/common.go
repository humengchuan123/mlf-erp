package app

import (
	"context"
	"errors"

	"github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	systemdomain "github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/tenant"
)

type CodeGenerator interface {
	Next(ctx context.Context, tenantID, entity string) (string, error)
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

func resolveCode(ctx context.Context, generator CodeGenerator, tenantID, entity, code string) (string, error) {
	if code != "" {
		return code, nil
	}
	if generator == nil {
		return "", domain.ErrInvalidInput
	}
	code, err := generator.Next(ctx, tenantID, entity)
	if err != nil {
		if errors.Is(err, systemdomain.ErrNotFound) {
			return "", domain.ErrInvalidInput
		}
		return "", err
	}
	return code, nil
}

func validStatus(status string) bool {
	return status == domain.StatusActive || status == domain.StatusDisabled
}
