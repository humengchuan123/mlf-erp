package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/security"
)

type UserService struct {
	users domain.UserRepository
	roles domain.RoleRepository
}

func NewUserService(users domain.UserRepository, roles domain.RoleRepository) *UserService {
	return &UserService{users: users, roles: roles}
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

func (s *UserService) Create(ctx context.Context, input CreateUserInput) (*domain.User, error) {
	username := strings.TrimSpace(input.Username)
	displayName := strings.TrimSpace(input.DisplayName)
	if username == "" || displayName == "" {
		return nil, domain.ErrConflict
	}
	if len(input.Password) < 8 {
		return nil, domain.ErrWeakPassword
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

	if strings.TrimSpace(input.DisplayName) != "" {
		user.DisplayName = strings.TrimSpace(input.DisplayName)
	}
	user.Email = strings.TrimSpace(input.Email)
	user.OrgID = input.OrgID
	if input.Status != "" {
		user.Status = input.Status
	}
	user.IsSuperAdmin = input.IsSuperAdmin

	if input.Password != "" {
		if len(input.Password) < 8 {
			return nil, domain.ErrWeakPassword
		}
		hash, hashErr := security.HashPassword(input.Password)
		if hashErr != nil {
			return nil, hashErr
		}
		user.PasswordHash = hash
	}

	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) Disable(ctx context.Context, tenantID, id string) error {
	user, err := s.users.FindByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	user.Status = domain.UserStatusDisabled
	return s.users.Update(ctx, user)
}

func (s *UserService) SetRoles(ctx context.Context, tenantID, userID string, roleIDs []string) error {
	if _, err := s.users.FindByID(ctx, tenantID, userID); err != nil {
		return err
	}
	return s.users.SetRoles(ctx, tenantID, userID, roleIDs)
}

func (s *UserService) GetRoleIDs(ctx context.Context, tenantID, userID string) ([]string, error) {
	return s.users.GetRoleIDs(ctx, tenantID, userID)
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
