package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type OrderStatus string

const (
	OrderStatusDraft    OrderStatus = "draft"
	OrderStatusApproved OrderStatus = "approved"
	OrderStatusClosed   OrderStatus = "closed"
)

var (
	ErrTenantRequired = errors.New("tenant is required")
	ErrCodeRequired   = errors.New("order code is required")
	ErrAmountNegative = errors.New("order amount cannot be negative")
)

type Order struct {
	ID         string
	TenantID   string
	OrgID      string
	Code       string
	SupplierID string
	Amount     decimal.Decimal
	Currency   string
	Status     OrderStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewOrder(
	id string,
	tenantID string,
	orgID string,
	code string,
	supplierID string,
	amount decimal.Decimal,
	currency string,
) (*Order, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, ErrTenantRequired
	}
	if strings.TrimSpace(code) == "" {
		return nil, ErrCodeRequired
	}
	if amount.IsNegative() {
		return nil, ErrAmountNegative
	}

	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = "CNY"
	}

	now := time.Now()
	return &Order{
		ID:         id,
		TenantID:   tenantID,
		OrgID:      orgID,
		Code:       strings.TrimSpace(code),
		SupplierID: strings.TrimSpace(supplierID),
		Amount:     amount,
		Currency:   currency,
		Status:     OrderStatusDraft,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}
