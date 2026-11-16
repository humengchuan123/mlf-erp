package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/security"
)

func newTestAuthService(t *testing.T, password string) (*AuthService, *fakeUserRepo, *fakeRoleRepo, *fakeRefreshTokenRepo, *fakeLoginLogRepo) {
	t.Helper()

	passwordHash, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	users := newFakeUserRepo()
	users.users[testKey("t1", "u1")] = &domain.User{
		ID:           "u1",
		TenantID:     "t1",
		Username:     "admin",
		DisplayName:  "Admin",
		PasswordHash: passwordHash,
		Status:       domain.UserStatusActive,
	}

	roles := newFakeRoleRepo()
	roles.roles["r1"] = &domain.Role{
		ID:       "r1",
		TenantID: "t1",
		Code:     "admin",
		Name:     "Administrator",
		IsSystem: true,
	}
	roles.userRoles[testKey("t1", "u1")] = []string{"r1"}
	roles.rolePermissions["r1"] = []string{"purchase.read", "system.user.read"}

	tokens := newFakeRefreshTokenRepo()
	logs := &fakeLoginLogRepo{}
	tokenService := security.NewTokenService("test-secret", "test-issuer", time.Minute)
	service := NewAuthService(users, roles, tokens, logs, tokenService, time.Hour)

	return service, users, roles, tokens, logs
}

func TestAuthServiceLoginSuccess(t *testing.T) {
	service, _, _, _, logs := newTestAuthService(t, "Admin@123456")

	result, err := service.Login(context.Background(), LoginInput{
		TenantID: "t1",
		Username: "admin",
		Password: "Admin@123456",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		t.Fatal("Login() returned empty tokens")
	}
	if len(result.Permissions) != 2 {
		t.Fatalf("Permissions = %v, want 2 entries", result.Permissions)
	}
	if len(logs.logs) != 1 || logs.logs[0].Status != domain.LoginStatusSuccess {
		t.Fatalf("login logs = %+v, want one success entry", logs.logs)
	}
}

func TestAuthServiceLoginFailure(t *testing.T) {
	service, _, _, _, logs := newTestAuthService(t, "Admin@123456")

	_, err := service.Login(context.Background(), LoginInput{
		TenantID: "t1",
		Username: "admin",
		Password: "wrong-password",
	})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
	if len(logs.logs) != 1 || logs.logs[0].Status != domain.LoginStatusFailure {
		t.Fatalf("login logs = %+v, want one failure entry", logs.logs)
	}
}

func TestAuthServiceDisabledUserUsesGenericFailure(t *testing.T) {
	service, users, _, _, logs := newTestAuthService(t, "Admin@123456")
	users.users[testKey("t1", "u1")].Status = domain.UserStatusDisabled

	_, err := service.Login(context.Background(), LoginInput{
		TenantID: "t1",
		Username: "admin",
		Password: "Admin@123456",
	})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
	if len(logs.logs) != 1 || logs.logs[0].FailureReason != "user disabled" {
		t.Fatalf("login logs = %+v, want disabled reason", logs.logs)
	}
}

func TestAuthServiceUnknownUserUsesGenericFailure(t *testing.T) {
	service, _, _, _, logs := newTestAuthService(t, "Admin@123456")

	_, err := service.Login(context.Background(), LoginInput{
		TenantID: "t1",
		Username: "missing-user",
		Password: "whatever",
	})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
	if len(logs.logs) != 1 || logs.logs[0].FailureReason != "user not found" {
		t.Fatalf("login logs = %+v, want not-found reason", logs.logs)
	}
}

func TestAuthServiceRefreshRotation(t *testing.T) {
	service, _, _, _, _ := newTestAuthService(t, "Admin@123456")

	result, err := service.Login(context.Background(), LoginInput{
		TenantID: "t1",
		Username: "admin",
		Password: "Admin@123456",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	refreshed, err := service.Refresh(context.Background(), result.RefreshToken, "", "")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed.RefreshToken == result.RefreshToken {
		t.Fatal("Refresh() did not rotate refresh token")
	}

	again, err := service.Refresh(context.Background(), refreshed.RefreshToken, "", "")
	if err != nil {
		t.Fatalf("Refresh() with rotated token error = %v", err)
	}

	// Reuse of an already rotated token must invalidate the whole token family.
	if _, err := service.Refresh(context.Background(), result.RefreshToken, "", ""); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("Refresh() with old token error = %v, want ErrInvalidToken", err)
	}
	if _, err := service.Refresh(context.Background(), again.RefreshToken, "", ""); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("Refresh() after reuse detection error = %v, want ErrInvalidToken", err)
	}
}

func TestAuthServiceChangePasswordRevokesTokens(t *testing.T) {
	service, _, _, _, _ := newTestAuthService(t, "Admin@123456")

	result, err := service.Login(context.Background(), LoginInput{
		TenantID: "t1",
		Username: "admin",
		Password: "Admin@123456",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if err := service.ChangePassword(context.Background(), "t1", "u1", "Admin@123456", "NewPass@123"); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if _, err := service.Refresh(context.Background(), result.RefreshToken, "", ""); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("Refresh() after password change error = %v, want ErrInvalidToken", err)
	}
	if _, err := service.Login(context.Background(), LoginInput{
		TenantID: "t1",
		Username: "admin",
		Password: "NewPass@123",
	}); err != nil {
		t.Fatalf("Login() with new password error = %v", err)
	}
}
