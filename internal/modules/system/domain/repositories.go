package domain

import "context"

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	FindByID(ctx context.Context, tenantID, id string) (*User, error)
	FindByUsername(ctx context.Context, tenantID, username string) (*User, error)
	List(ctx context.Context, tenantID, keyword string, offset, limit int) ([]*User, int, error)
	SetRoles(ctx context.Context, tenantID, userID string, roleIDs []string) error
	GetRoleIDs(ctx context.Context, tenantID, userID string) ([]string, error)
	DataScopeByUserID(ctx context.Context, tenantID, userID, orgID string) (string, error)
	TouchLastLogin(ctx context.Context, tenantID, userID string) error
	CountActiveSuperAdmins(ctx context.Context, tenantID string) (int, error)
}

type RoleRepository interface {
	Create(ctx context.Context, role *Role) error
	Update(ctx context.Context, role *Role) error
	Delete(ctx context.Context, tenantID, id string) error
	FindByID(ctx context.Context, tenantID, id string) (*Role, error)
	List(ctx context.Context, tenantID string) ([]*Role, error)
	SetPermissions(ctx context.Context, tenantID, roleID string, permissionCodes []string) error
	PermissionCodesByUserID(ctx context.Context, tenantID, userID string) ([]string, error)
	RoleCodesByUserID(ctx context.Context, tenantID, userID string) ([]string, error)
}

type PermissionRepository interface {
	List(ctx context.Context) ([]*Permission, error)
}

type OrgRepository interface {
	Create(ctx context.Context, org *Org) error
	Update(ctx context.Context, org *Org) error
	Delete(ctx context.Context, tenantID, id string) error
	FindByID(ctx context.Context, tenantID, id string) (*Org, error)
	AncestorByType(ctx context.Context, tenantID, orgID, orgType string) (*Org, error)
	List(ctx context.Context, tenantID string) ([]*Org, error)
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *RefreshToken) error
	FindByHash(ctx context.Context, hash string) (*RefreshToken, error)
	ConsumeByHash(ctx context.Context, hash string) (bool, error)
	RevokeByHash(ctx context.Context, hash string) error
	RevokeAllForUser(ctx context.Context, tenantID, userID string) error
	DeleteExpired(ctx context.Context) error
}

type AuditRepository interface {
	Create(ctx context.Context, log *AuditLog) error
	List(ctx context.Context, tenantID string, offset, limit int) ([]*AuditLog, int, error)
}

type LoginLogRepository interface {
	Create(ctx context.Context, log *LoginLog) error
}

type CodeRuleRepository interface {
	List(ctx context.Context, tenantID string) ([]*CodeRule, error)
	Create(ctx context.Context, rule *CodeRule) error
	Update(ctx context.Context, rule *CodeRule) error
	Next(ctx context.Context, tenantID, entity string) (string, error)
}
