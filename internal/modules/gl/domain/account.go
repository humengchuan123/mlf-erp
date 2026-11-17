package domain

import "time"

const (
	AccountTypeAsset     = "asset"
	AccountTypeLiability = "liability"
	AccountTypeEquity    = "equity"
	AccountTypeRevenue   = "revenue"
	AccountTypeExpense   = "expense"

	DirectionDebit  = "debit"
	DirectionCredit = "credit"

	StatusActive   = "active"
	StatusDisabled = "disabled"
)

type Account struct {
	ID        string
	TenantID  string
	Code      string
	Name      string
	Type      string
	Direction string
	ParentID  string
	Level     int
	IsLeaf    bool
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}
