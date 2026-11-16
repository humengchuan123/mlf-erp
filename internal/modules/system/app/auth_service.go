package app

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/security"
)

var dummyPasswordHash = func() string {
	hash, err := security.HashPassword("mlf-erp-dummy-password")
	if err != nil {
		return ""
	}
	return hash
}()

type AuthService struct {
	users        domain.UserRepository
	roles        domain.RoleRepository
	tokens       domain.RefreshTokenRepository
	loginLogs    domain.LoginLogRepository
	tokenService *security.TokenService
	refreshTTL   time.Duration
}

func NewAuthService(
	users domain.UserRepository,
	roles domain.RoleRepository,
	tokens domain.RefreshTokenRepository,
	loginLogs domain.LoginLogRepository,
	tokenService *security.TokenService,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		users:        users,
		roles:        roles,
		tokens:       tokens,
		loginLogs:    loginLogs,
		tokenService: tokenService,
		refreshTTL:   refreshTTL,
	}
}

type LoginInput struct {
	TenantID  string
	Username  string
	Password  string
	IP        string
	UserAgent string
}

type LoginResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	User         *domain.User
	Roles        []string
	Permissions  []string
}

type MeResult struct {
	User        *domain.User
	Roles       []string
	Permissions []string
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	user, err := s.users.FindByUsername(ctx, input.TenantID, input.Username)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			if dummyPasswordHash != "" {
				security.CheckPassword(dummyPasswordHash, input.Password)
			}
			s.writeLoginLog(ctx, input, "", domain.LoginStatusFailure, "user not found")
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}

	if !user.IsActive() {
		security.CheckPassword(user.PasswordHash, input.Password)
		s.writeLoginLog(ctx, input, user.ID, domain.LoginStatusFailure, "user disabled")
		return nil, domain.ErrInvalidCredentials
	}
	if !security.CheckPassword(user.PasswordHash, input.Password) {
		s.writeLoginLog(ctx, input, user.ID, domain.LoginStatusFailure, "invalid password")
		return nil, domain.ErrInvalidCredentials
	}

	if err := s.users.TouchLastLogin(ctx, input.TenantID, user.ID); err != nil {
		return nil, err
	}

	result, err := s.issueTokens(ctx, user, input.IP, input.UserAgent)
	if err != nil {
		return nil, err
	}
	s.writeLoginLog(ctx, input, user.ID, domain.LoginStatusSuccess, "")
	return result, nil
}

func (s *AuthService) Refresh(ctx context.Context, rawToken, ip, userAgent string) (*LoginResult, error) {
	hash := security.HashRefreshToken(rawToken)
	token, err := s.tokens.FindByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidToken
		}
		return nil, err
	}
	if token.RevokedAt != nil {
		_ = s.tokens.RevokeAllForUser(ctx, token.TenantID, token.UserID)
		return nil, domain.ErrInvalidToken
	}
	if !token.ExpiresAt.After(time.Now()) {
		return nil, domain.ErrInvalidToken
	}

	consumed, err := s.tokens.ConsumeByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if !consumed {
		_ = s.tokens.RevokeAllForUser(ctx, token.TenantID, token.UserID)
		return nil, domain.ErrInvalidToken
	}

	user, err := s.users.FindByID(ctx, token.TenantID, token.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidToken
		}
		return nil, err
	}
	if !user.IsActive() {
		return nil, domain.ErrInvalidToken
	}

	return s.issueTokens(ctx, user, ip, userAgent)
}

func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	return s.tokens.RevokeByHash(ctx, security.HashRefreshToken(rawToken))
}

func (s *AuthService) Me(ctx context.Context, tenantID, userID string) (*MeResult, error) {
	user, err := s.users.FindByID(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}

	roles, err := s.roles.RoleCodesByUserID(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	permissions, err := s.roles.PermissionCodesByUserID(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}

	return &MeResult{User: user, Roles: roles, Permissions: permissions}, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, tenantID, userID, oldPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return domain.ErrWeakPassword
	}

	user, err := s.users.FindByID(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if !security.CheckPassword(user.PasswordHash, oldPassword) {
		return domain.ErrInvalidCredentials
	}

	hash, err := security.HashPassword(newPassword)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	if err := s.users.Update(ctx, user); err != nil {
		return err
	}
	return s.tokens.RevokeAllForUser(ctx, tenantID, userID)
}

func (s *AuthService) issueTokens(ctx context.Context, user *domain.User, ip, userAgent string) (*LoginResult, error) {
	accessToken, expiresAt, err := s.tokenService.GenerateAccessToken(user.ID, user.TenantID, user.OrgID, user.Username)
	if err != nil {
		return nil, err
	}

	rawRefresh, refreshHash, err := security.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	if err := s.tokens.Create(ctx, &domain.RefreshToken{
		ID:        uuid.NewString(),
		TenantID:  user.TenantID,
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: time.Now().Add(s.refreshTTL),
		UserAgent: userAgent,
		IP:        ip,
	}); err != nil {
		return nil, err
	}

	roles, err := s.roles.RoleCodesByUserID(ctx, user.TenantID, user.ID)
	if err != nil {
		return nil, err
	}
	permissions, err := s.roles.PermissionCodesByUserID(ctx, user.TenantID, user.ID)
	if err != nil {
		return nil, err
	}

	expiresIn := int64(time.Until(expiresAt).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    expiresIn,
		User:         user,
		Roles:        roles,
		Permissions:  permissions,
	}, nil
}

func (s *AuthService) writeLoginLog(ctx context.Context, input LoginInput, userID, status, reason string) {
	_ = s.loginLogs.Create(ctx, &domain.LoginLog{
		ID:            uuid.NewString(),
		TenantID:      input.TenantID,
		UserID:        userID,
		Username:      input.Username,
		Status:        status,
		FailureReason: reason,
		IP:            input.IP,
		UserAgent:     input.UserAgent,
	})
}
