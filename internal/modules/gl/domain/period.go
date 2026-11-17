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

type PeriodClose struct {
	ID                string
	TenantID          string
	PeriodID          string
	VoucherID         string
	ReversalVoucherID string
	Status            string
	CreatedBy         string
	CreatedAt         time.Time
	ReopenedBy        string
	ReopenedAt        *time.Time
}

type YearClose struct {
	ID        string
	TenantID  string
	Year      int
	VoucherID string
	Status    string
	CreatedBy string
	CreatedAt time.Time
}

type Dimension struct {
	ID   string
	Code string
	Name string
}

type Dimensions struct {
	Partners    []*Dimension
	Departments []*Dimension
}
