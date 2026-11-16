package domain

import "time"

const (
	LoginStatusSuccess = "success"
	LoginStatusFailure = "failure"
)

type LoginLog struct {
	ID            string
	TenantID      string
	UserID        string
	Username      string
	Status        string
	FailureReason string
	IP            string
	UserAgent     string
	CreatedAt     time.Time
}
