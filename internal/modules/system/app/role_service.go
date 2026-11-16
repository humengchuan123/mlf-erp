package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
)

type RoleService struct {
	roles domain.RoleRepository
}

func NewRoleService(roles domain.RoleRepository) *RoleService {
	return &RoleService{roles: roles}
}

type CreateRoleInput struct {
	TenantID string
	Code     string
	Name     string
}

func (s *RoleService) List(ctx context.Context, tenantID string) ([]*domain.Role, error) {
	return s.roles.List(ctx, tenantID)
}

func (s *RoleService) Create(ctx context.Context, input CreateRoleInput) (*domain.Role, error) {
	code := strings.TrimSpace(input.Code)
	name := strings.TrimSpace(input.Name)
	if code == "" || name == "" {
		return nil, domain.ErrInvalidInput
	}

	role := &domain.Role{
		ID:       uuid.NewString(),
		TenantID: input.TenantID,
		Code:     code,
		Name:     name,
	}
	if err := s.roles.Create(ctx, role); err != nil {
		return nil, err
	}
	return role, nil
}

func (s *RoleService) Update(ctx context.Context, tenantID, id, code, name string) (*domain.Role, error) {
	role, err := s.roles.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	actor, _ := auth.ActorFrom(ctx)
	if role.IsSystem && !actor.IsSuperAdmin {
		return nil, domain.ErrForbidden
	}

	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return nil, domain.ErrInvalidInput
	}
	if role.IsSystem && code != role.Code {
		return nil, domain.ErrForbidden
	}
	role.Code = code
	role.Name = name

	if err := s.roles.Update(ctx, role); err != nil {
		return nil, err
	}
	return role, nil
}

func (s *RoleService) Delete(ctx context.Context, tenantID, id string) error {
	role, err := s.roles.FindByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if role.IsSystem {
		return domain.ErrSystemRole
	}
	return s.roles.Delete(ctx, tenantID, id)
}

func (s *RoleService) SetPermissions(ctx context.Context, tenantID, id string, permissionCodes []string) error {
	role, err := s.roles.FindByID(ctx, tenantID, id)
	if err != nil {
		return err
	}

	actor, _ := auth.ActorFrom(ctx)
	if role.IsSystem && !actor.IsSuperAdmin {
		return domain.ErrForbidden
	}
	if !actor.IsSuperAdmin {
		for _, existing := range role.Permissions {
			if !actor.HasPermission(existing) {
				return domain.ErrForbidden
			}
		}
		for _, requested := range permissionCodes {
			if !actor.HasPermission(requested) {
				return domain.ErrForbidden
			}
		}
	}

	return s.roles.SetPermissions(ctx, tenantID, id, permissionCodes)
}

type PermissionService struct {
	permissions domain.PermissionRepository
}

func NewPermissionService(permissions domain.PermissionRepository) *PermissionService {
	return &PermissionService{permissions: permissions}
}

func (s *PermissionService) List(ctx context.Context) ([]*domain.Permission, error) {
	return s.permissions.List(ctx)
}
