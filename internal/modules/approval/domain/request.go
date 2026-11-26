package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type RequestStatus string

const (
	RequestPending   RequestStatus = "pending"
	RequestApproved  RequestStatus = "approved"
	RequestRejected  RequestStatus = "rejected"
	RequestCancelled RequestStatus = "cancelled"
)

type ActionType string

const (
	ActionApprove ActionType = "approve"
	ActionReject  ActionType = "reject"
	ActionCancel  ActionType = "cancel"
	ActionSkip    ActionType = "skip"
)

type Request struct {
	ID           string
	TenantID     string
	OrgID        string
	DocumentType DocumentType
	DocumentID   string
	DocumentCode string
	FlowID       string
	Status       RequestStatus
	CurrentStep  int
	RequestedBy  string
	RequestedAt  time.Time
	CompletedAt  *time.Time
	Amount       decimal.Decimal
	BaseAmount   decimal.Decimal
	Currency     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Steps        []FlowStep
	Actions      []Action
}

type Action struct {
	ID        string
	TenantID  string
	RequestID string
	StepNo    int
	Action    ActionType
	ActorID   string
	Comment   string
	CreatedAt time.Time
}

type Actor struct {
	UserID       string
	IsSuperAdmin bool
}

type StartInput struct {
	TenantID     string
	OrgID        string
	DocumentType DocumentType
	DocumentID   string
	DocumentCode string
	RequestedBy  string
	Amount       decimal.Decimal
	BaseAmount   decimal.Decimal
	Currency     string
}
