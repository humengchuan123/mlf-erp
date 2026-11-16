package security

import (
	"testing"
	"time"
)

func TestTokenGenerateAndParse(t *testing.T) {
	service := NewTokenService("test-secret", "mlf-erp-test", 15*time.Minute)

	raw, expiresAt, err := service.GenerateAccessToken("user-1", "tenant-1", "org-1", "admin")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	if raw == "" {
		t.Fatal("GenerateAccessToken() returned empty token")
	}
	if !expiresAt.After(time.Now()) {
		t.Fatal("expiresAt must be in the future")
	}

	claims, err := service.ParseAccessToken(raw)
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}
	if claims.UserID != "user-1" || claims.TenantID != "tenant-1" || claims.OrgID != "org-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestTokenRejectsExpiredAndWrongSignature(t *testing.T) {
	expiredService := NewTokenService("test-secret", "mlf-erp-test", -time.Minute)
	raw, _, err := expiredService.GenerateAccessToken("user-1", "tenant-1", "org-1", "admin")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	if _, err := NewTokenService("test-secret", "mlf-erp-test", time.Minute).ParseAccessToken(raw); err == nil {
		t.Fatal("ParseAccessToken() accepted expired token")
	}

	validService := NewTokenService("test-secret", "mlf-erp-test", time.Minute)
	raw, _, err = validService.GenerateAccessToken("user-1", "tenant-1", "org-1", "admin")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	if _, err := NewTokenService("other-secret", "mlf-erp-test", time.Minute).ParseAccessToken(raw); err == nil {
		t.Fatal("ParseAccessToken() accepted token with wrong signature")
	}
}

func TestRefreshTokenHashIsStable(t *testing.T) {
	raw, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}
	if raw == "" || hash == "" {
		t.Fatal("GenerateRefreshToken() returned empty values")
	}
	if HashRefreshToken(raw) != hash {
		t.Fatal("HashRefreshToken() is not stable")
	}
}
