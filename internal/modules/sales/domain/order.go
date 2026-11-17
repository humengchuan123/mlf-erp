package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type Order struct {
	ID            string
	TenantID      string
	OrgID         string
	Code          string
	CustomerID    string
	CustomerName  string
	WarehouseID   string
	WarehouseName string
	OrderDate     time.Time
	Currency      string
	Status        Status
	TotalAmount   decimal.Decimal
	Remark        string
	CreatedBy     string
	UpdatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Lines         []*OrderLine
}

type OrderLine struct {
	ID           string
	TenantID     string
	OrderID      string
	LineNo       int
	MaterialID   string
	MaterialName string
	Quantity     decimal.Decimal
	UnitPrice    decimal.Decimal
	Amount       decimal.Decimal
	ShippedQty   decimal.Decimal
}

func (l *OrderLine) Remaining() decimal.Decimal {
	return l.Quantity.Sub(l.ShippedQty)
}

func (o *Order) Recalculate() {
	total := decimal.Zero
	for _, line := range o.Lines {
		line.Amount = line.Quantity.Mul(line.UnitPrice).Round(4)
		total = total.Add(line.Amount)
	}
	o.TotalAmount = total.Round(4)
}
