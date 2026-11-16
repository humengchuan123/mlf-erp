package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/security"
)

type UserService struct {
	users  domain.UserRepository
	roles  domain.RoleRepository
	tokens domain.RefreshTokenRepository
}

func NewUserService(users domain.UserRepository, roles domain.RoleRepository, tokens domain.RefreshTokenRepository) *UserService {
	return &UserService{users: users, roles: roles, tokens: tokens}
}

type CreateUserInput struct {
	TenantID     string
	OrgID        string
	Username     string
	DisplayName  string
	Email        string
	Password     string
	IsSuperAdmin bool
	RoleIDs      []string
}

type UpdateUserInput struct {
	OrgID        string
	DisplayName  string
	Email        string
	Status       string
	IsSuperAdmin bool
	Password     string
}

func (s *UserService) List(ctx context.Context, tenantID, keyword string, page, pageSize int) ([]*domain.User, int, error) {
	page, pageSize = normalizePage(page, pageSize)
	return s.users.List(ctx, tenantID, strings.TrimSpace(keyword), (page-1)*pageSize, pageSize)
}

func (s *UserService) Get(ctx context.Context, tenantID, id string) (*domain.User, error) {
	return s.users.FindByID(ctx, tenantID, id)
}

func (s *UserService) Create(ctx context.Context, input CreateUserInput) (*domain.User, error) {
	actor, _ := auth.ActorFrom(ctx)
	if input.IsSuperAdmin && !actor.IsSuperAdmin {
		return nil, domain.ErrForbidden
	}

	username := strings.TrimSpace(input.Username)
	displayName := strings.TrimSpace(input.DisplayName)
	if username == "" || displayName == "" {
		return nil, domain.ErrInvalidInput
	}
	if len(input.Password) < 8 {
		return nil, domain.ErrWeakPassword
	}
	if err := s.validateRoleAssignment(ctx, actor, input.TenantID, input.RoleIDs); err != nil {
		return nil, err
	}

	hash, err := security.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:           uuid.NewString(),
		TenantID:     input.TenantID,
		OrgID:        input.OrgID,
		Username:     username,
		DisplayName:  displayName,
		Email:        strings.TrimSpace(input.Email),
		PasswordHash: hash,
		Status:       domain.UserStatusActive,
		IsSuperAdmin: input.IsSuperAdmin,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	if len(input.RoleIDs) > 0 {
		if err := s.users.SetRoles(ctx, user.TenantID, user.ID, input.RoleIDs); err != nil {
			return nil, err
		}
	}
	return user, nil
}

func (s *UserService) Update(ctx context.Context, tenantID, id string, input UpdateUserInput) (*domain.User, error) {
	user, err := s.users.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	actor, _ := auth.ActorFrom(ctx)
	if user.IsSuperAdmin && !actor.IsSuperAdmin {
		return nil, domain.ErrForbidden
	}
	if input.IsSuperAdmin && !actor.IsSuperAdmin {
		return nil, domain.ErrForbidden
	}
	if user.ID == actor.UserID {
		if input.Status == domain.UserStatusDisabled {
			return nil, domain.ErrForbidden
		}
		if !input.IsSuperAdmin {
			return nil, domain.ErrForbidden
		}
	}
	if user.IsSuperAdmin && !input.IsSuperAdmin {
		activeSuperAdmins, countErr := s.users.CountActiveSuperAdmins(ctx, tenantID)
		if countErr != nil {
			return nil, countErr
		}
		if activeSuperAdmins <= 1 {
			return nil, domain.ErrForbidden
		}
	}
	if input.Status != "" && input.Status != domain.UserStatusActive && input.Status != domain.UserStatusDisabled {
		return nil, domain.ErrInvalidInput
	}

	if strings.TrimSpace(input.DisplayName) != "" {
		user.DisplayName = strings.TrimSpace(input.DisplayName)
	}
	user.Email = strings.TrimSpace(input.Email)
	if input.OrgID != "" {
		user.OrgID = input.OrgID
	}
	if input.Status != "" {
		user.Status = input.Status
	}
	user.IsSuperAdmin = input.IsSuperAdmin

	passwordChanged := false
	if input.Password != "" {
		if len(input.Password) < 8 {
			return nil, domain.ErrWeakPassword
		}
		hash, hashErr := security.HashPassword(input.Password)
		if hashErr != nil {
			return nil, hashErr
		}
		user.PasswordHash = hash
		passwordChanged = true
	}

	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	if passwordChanged {
		if err := s.tokens.RevokeAllForUser(ctx, tenantID, user.ID); err != nil {
			return nil, err
		}
	}
	return user, nil
}

func (s *UserService) Disable(ctx context.Context, tenantID, id string) error {
	user, err := s.users.FindByID(ctx, tenantID, id)
	if err != nil {
		return err
	}

	actor, _ := auth.ActorFrom(ctx)
	if user.IsSuperAdmin && !actor.IsSuperAdmin {
		return domain.ErrForbidden
	}
	if user.ID == actor.UserID {
		return domain.ErrForbidden
	}
	if user.IsSuperAdmin {
		activeSuperAdmins, countErr := s.users.CountActiveSuperAdmins(ctx, tenantID)
		if countErr != nil {
			return countErr
		}
		if activeSuperAdmins <= 1 {
			return domain.ErrForbidden
		}
	}

	user.Status = domain.UserStatusDisabled
	return s.users.Update(ctx, user)
}

func (s *UserService) SetRoles(ctx context.Context, tenantID, userID string, roleIDs []string) error {
	if _, err := s.users.FindByID(ctx, tenantID, userID); err != nil {
		return err
	}

	actor, _ := auth.ActorFrom(ctx)
	if userID == actor.UserID && !actor.IsSuperAdmin {
		return domain.ErrForbidden
	}
	if err := s.validateRoleAssignment(ctx, actor, tenantID, roleIDs); err != nil {
		return err
	}
	return s.users.SetRoles(ctx, tenantID, userID, roleIDs)
}

func (s *UserService) GetRoleIDs(ctx context.Context, tenantID, userID string) ([]string, error) {
	return s.users.GetRoleIDs(ctx, tenantID, userID)
}

func (s *UserService) validateRoleAssignment(ctx context.Context, actor auth.Actor, tenantID string, roleIDs []string) error {
	for _, roleID := range roleIDs {
		role, err := s.roles.FindByID(ctx, tenantID, roleID)
		if err != nil {
			return err
		}
		if role.IsSystem && !actor.IsSuperAdmin {
			return domain.ErrForbidden
		}
		if !actor.IsSuperAdmin {
			for _, permission := range role.Permissions {
				if !actor.HasPermission(permission) {
					return domain.ErrForbidden
				}
			}
		}
	}
	return nil
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
