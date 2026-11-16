package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type Balance struct {
	TenantID      string          `json:"tenant_id"`
	OrgID         string          `json:"org_id"`
	WarehouseID   string          `json:"warehouse_id"`
	WarehouseName string          `json:"warehouse_name"`
	MaterialID    string          `json:"material_id"`
	MaterialName  string          `json:"material_name"`
	OnHand        decimal.Decimal `json:"on_hand"`
	Reserved      decimal.Decimal `json:"reserved"`
	Available     decimal.Decimal `json:"available"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type Transaction struct {
	ID            string          `json:"id"`
	TenantID      string          `json:"tenant_id"`
	OrgID         string          `json:"org_id"`
	WarehouseID   string          `json:"warehouse_id"`
	WarehouseName string          `json:"warehouse_name"`
	MaterialID    string          `json:"material_id"`
	MaterialName  string          `json:"material_name"`
	Direction     string          `json:"direction"`
	BizType       string          `json:"biz_type"`
	BizID         string          `json:"biz_id"`
	BizCode       string          `json:"biz_code"`
	Quantity      decimal.Decimal `json:"quantity"`
	UnitCost      decimal.Decimal `json:"unit_cost"`
	Amount        decimal.Decimal `json:"amount"`
	OccurredAt    time.Time       `json:"occurred_at"`
	CreatedBy     string          `json:"created_by"`
}
