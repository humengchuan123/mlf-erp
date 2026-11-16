package domain

import "time"

const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
)

type User struct {
	ID           string
	TenantID     string
	OrgID        string
	Username     string
	DisplayName  string
	Email        string
	PasswordHash string
	Status       string
	IsSuperAdmin bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}
