package domain

import "time"

const (
	PeriodStatusOpen   = "open"
	PeriodStatusClosed = "closed"
)

type Period struct {
	ID        string
	TenantID  string
	Year      int
	Month     int
	StartDate time.Time
	EndDate   time.Time
	Status    string
	ClosedAt  *time.Time
	ClosedBy  string
	CreatedAt time.Time
}
