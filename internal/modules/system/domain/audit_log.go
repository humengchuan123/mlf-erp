package domain

import "time"

type AuditLog struct {
	ID           string
	TenantID     string
	OrgID        string
	UserID       string
	Action       string
	ResourceType string
	ResourceID   string
	RequestID    string
	IP           string
	UserAgent    string
	BeforeData   []byte
	AfterData    []byte
	CreatedAt    time.Time
}
