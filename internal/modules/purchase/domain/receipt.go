package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type Receipt struct {
	ID             string
	TenantID       string
	OrgID          string
	Code           string
	OrderID        string
	OrderCode      string
	SupplierName   string
	WarehouseID    string
	WarehouseName  string
	ReceiptDate    time.Time
	Status         string
	Remark         string
	CreatedBy      string
	CreatedAt      time.Time
	PayableID      string
	PayableCode    string
	PayableDueDate time.Time
	Lines          []*ReceiptLine
}

type ReceiptLine struct {
	ID           string
	TenantID     string
	ReceiptID    string
	OrderLineID  string
	MaterialID   string
	MaterialName string
	Quantity     decimal.Decimal
	UnitPrice    decimal.Decimal
	Amount       decimal.Decimal
}
