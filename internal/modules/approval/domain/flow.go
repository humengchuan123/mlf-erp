package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type DocumentType string

const (
	DocumentSalesOrder    DocumentType = "sales_order"
	DocumentPurchaseOrder DocumentType = "purchase_order"
)

type ConditionType string

const (
	ConditionAlways    ConditionType = "always"
	ConditionAmountGTE ConditionType = "amount_gte"
)

type ApproverType string

const (
	ApproverRole ApproverType = "role"
	ApproverUser ApproverType = "user"
)

type Flow struct {
	ID              string
	TenantID        string
	OrgID           string
	Name            string
	DocumentType    DocumentType
	ConditionType   ConditionType
	ThresholdAmount decimal.Decimal
	Priority        int
	Active          bool
	CreatedBy       string
	UpdatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Steps           []FlowStep
}

type FlowStep struct {
	ID           string
	TenantID     string
	FlowID       string
	StepNo       int
	Name         string
	ApproverType ApproverType
	ApproverID   string
	CreatedAt    time.Time
}
