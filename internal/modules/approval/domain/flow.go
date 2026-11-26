package domain

import (
	"sort"
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

// ResolvedSteps is the outcome of resolving a flow against the requester.
// Skipped holds user steps assigned to the requester, which cannot be acted on
// by them, while CurrentStep is the first remaining step in step order.
type ResolvedSteps struct {
	CurrentStep int
	Skipped     []FlowStep
}

// ResolveApprovalSteps sorts the flow steps and skips user steps assigned to
// the requester. Role steps are always retained, even when the requester holds
// the role. It returns ErrNoApprover when every step is a requester user step.
func ResolveApprovalSteps(steps []FlowStep, requestedBy string) (ResolvedSteps, error) {
	ordered := append([]FlowStep(nil), steps...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].StepNo == ordered[j].StepNo {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].StepNo < ordered[j].StepNo
	})

	resolved := ResolvedSteps{Skipped: make([]FlowStep, 0)}
	currentFound := false
	for _, step := range ordered {
		if step.ApproverType == ApproverUser && step.ApproverID == requestedBy {
			resolved.Skipped = append(resolved.Skipped, step)
			continue
		}
		if !currentFound {
			resolved.CurrentStep = step.StepNo
			currentFound = true
		}
	}
	if !currentFound {
		return ResolvedSteps{}, ErrNoApprover
	}
	return resolved, nil
}
