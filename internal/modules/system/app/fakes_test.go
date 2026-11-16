package app

import (
	"context"
	"time"

	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

func testKey(tenantID, id string) string {
	return tenantID + "/" + id
}

type fakeUserRepo struct {
	users     map[string]*domain.User
	userRoles map[string][]string
	dataScope string
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		users:     make(map[string]*domain.User),
		userRoles: make(map[string][]string),
		dataScope: "self",
	}
}

func (r *fakeUserRepo) Create(_ context.Context, user *domain.User) error {
	copied := *user
	r.users[testKey(user.TenantID, user.ID)] = &copied
	return nil
}

func (r *fakeUserRepo) Update(_ context.Context, user *domain.User) error {
	copied := *user
	r.users[testKey(user.TenantID, user.ID)] = &copied
	return nil
}

func (r *fakeUserRepo) FindByID(_ context.Context, tenantID, id string) (*domain.User, error) {
	user, ok := r.users[testKey(tenantID, id)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copied := *user
	return &copied, nil
}

func (r *fakeUserRepo) FindByUsername(_ context.Context, tenantID, username string) (*domain.User, error) {
	for _, user := range r.users {
		if user.TenantID == tenantID && user.Username == username {
			copied := *user
			return &copied, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *fakeUserRepo) List(_ context.Context, tenantID, _ string, _, _ int) ([]*domain.User, int, error) {
	result := make([]*domain.User, 0)
	for _, user := range r.users {
		if user.TenantID == tenantID {
			copied := *user
			result = append(result, &copied)
		}
	}
	return result, len(result), nil
}

func (r *fakeUserRepo) SetRoles(_ context.Context, tenantID, userID string, roleIDs []string) error {
	r.userRoles[testKey(tenantID, userID)] = append([]string(nil), roleIDs...)
	return nil
}

func (r *fakeUserRepo) GetRoleIDs(_ context.Context, tenantID, userID string) ([]string, error) {
	return r.userRoles[testKey(tenantID, userID)], nil
}

func (r *fakeUserRepo) DataScopeByUserID(_ context.Context, _, _, _ string) (string, error) {
	return r.dataScope, nil
}

func (r *fakeUserRepo) TouchLastLogin(_ context.Context, _, _ string) error {
	return nil
}

func (r *fakeUserRepo) CountActiveSuperAdmins(_ context.Context, tenantID string) (int, error) {
	count := 0
	for _, user := range r.users {
		if user.TenantID == tenantID && user.IsSuperAdmin && user.Status == domain.UserStatusActive {
			count++
		}
	}
	return count, nil
}

type fakeRoleRepo struct {
	roles           map[string]*domain.Role
	userRoles       map[string][]string
	rolePermissions map[string][]string
}

func newFakeRoleRepo() *fakeRoleRepo {
	return &fakeRoleRepo{
		roles:           make(map[string]*domain.Role),
		userRoles:       make(map[string][]string),
		rolePermissions: make(map[string][]string),
	}
}

func (r *fakeRoleRepo) Create(_ context.Context, role *domain.Role) error {
	copied := *role
	r.roles[role.ID] = &copied
	return nil
}

func (r *fakeRoleRepo) Update(_ context.Context, role *domain.Role) error {
	copied := *role
	r.roles[role.ID] = &copied
	return nil
}

func (r *fakeRoleRepo) Delete(_ context.Context, _, id string) error {
	delete(r.roles, id)
	return nil
}

func (r *fakeRoleRepo) FindByID(_ context.Context, _, id string) (*domain.Role, error) {
	role, ok := r.roles[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copied := *role
	copied.Permissions = append([]string(nil), r.rolePermissions[id]...)
	return &copied, nil
}

func (r *fakeRoleRepo) List(_ context.Context, _ string) ([]*domain.Role, error) {
	result := make([]*domain.Role, 0, len(r.roles))
	for _, role := range r.roles {
		copied := *role
		result = append(result, &copied)
	}
	return result, nil
}

func (r *fakeRoleRepo) SetPermissions(_ context.Context, _, roleID string, codes []string) error {
	r.rolePermissions[roleID] = append([]string(nil), codes...)
	return nil
}

func (r *fakeRoleRepo) PermissionCodesByUserID(_ context.Context, tenantID, userID string) ([]string, error) {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, roleID := range r.userRoles[testKey(tenantID, userID)] {
		for _, code := range r.rolePermissions[roleID] {
			if _, ok := seen[code]; ok {
				continue
			}
			seen[code] = struct{}{}
			result = append(result, code)
		}
	}
	return result, nil
}

func (r *fakeRoleRepo) RoleCodesByUserID(_ context.Context, tenantID, userID string) ([]string, error) {
	result := make([]string, 0)
	for _, roleID := range r.userRoles[testKey(tenantID, userID)] {
		if role, ok := r.roles[roleID]; ok {
			result = append(result, role.Code)
		}
	}
	return result, nil
}

type fakeRefreshTokenRepo struct {
	tokens map[string]*domain.RefreshToken
}

func newFakeRefreshTokenRepo() *fakeRefreshTokenRepo {
	return &fakeRefreshTokenRepo{tokens: make(map[string]*domain.RefreshToken)}
}

func (r *fakeRefreshTokenRepo) Create(_ context.Context, token *domain.RefreshToken) error {
	copied := *token
	r.tokens[token.TokenHash] = &copied
	return nil
}

func (r *fakeRefreshTokenRepo) FindByHash(_ context.Context, hash string) (*domain.RefreshToken, error) {
	token, ok := r.tokens[hash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copied := *token
	return &copied, nil
}

func (r *fakeRefreshTokenRepo) ConsumeByHash(_ context.Context, hash string) (bool, error) {
	token, ok := r.tokens[hash]
	if !ok || token.RevokedAt != nil || !token.ExpiresAt.After(time.Now()) {
		return false, nil
	}
	now := time.Now()
	token.RevokedAt = &now
	return true, nil
}

func (r *fakeRefreshTokenRepo) RevokeByHash(_ context.Context, hash string) error {
	if token, ok := r.tokens[hash]; ok {
		now := time.Now()
		token.RevokedAt = &now
	}
	return nil
}

func (r *fakeRefreshTokenRepo) RevokeAllForUser(_ context.Context, tenantID, userID string) error {
	now := time.Now()
	for _, token := range r.tokens {
		if token.TenantID == tenantID && token.UserID == userID {
			token.RevokedAt = &now
		}
	}
	return nil
}

func (r *fakeRefreshTokenRepo) DeleteExpired(_ context.Context) error {
	return nil
}

type fakeLoginLogRepo struct {
	logs []*domain.LoginLog
}

func (r *fakeLoginLogRepo) Create(_ context.Context, log *domain.LoginLog) error {
	copied := *log
	r.logs = append(r.logs, &copied)
	return nil
}
