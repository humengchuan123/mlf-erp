package domain

import "time"

type RefreshToken struct {
	ID        string
	TenantID  string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	UserAgent string
	IP        string
	CreatedAt time.Time
}

func (t *RefreshToken) IsValid(now time.Time) bool {
	return t.RevokedAt == nil && t.ExpiresAt.After(now)
}
