package domain

import "time"

type Role struct {
	ID          string
	TenantID    string
	Code        string
	Name        string
	IsSystem    bool
	Permissions []string
	CreatedAt   time.Time
}
