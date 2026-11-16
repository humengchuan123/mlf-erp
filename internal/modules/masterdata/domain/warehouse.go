package domain

import "time"

const (
	WarehouseTypeNormal      = "normal"
	WarehouseTypeConsignment = "consignment"
	WarehouseTypeOther       = "other"
)

type Warehouse struct {
	ID        string    `db:"id" json:"id"`
	TenantID  string    `db:"tenant_id" json:"tenant_id"`
	OrgID     string    `db:"org_id" json:"org_id"`
	Code      string    `db:"code" json:"code"`
	Name      string    `db:"name" json:"name"`
	Type      string    `db:"type" json:"type"`
	Address   string    `db:"address" json:"address"`
	Status    string    `db:"status" json:"status"`
	CreatedBy string    `db:"created_by" json:"created_by"`
	UpdatedBy string    `db:"updated_by" json:"updated_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func (w Warehouse) TableName() string { return "erp.warehouses" }

func (w Warehouse) SelectColumns() []string {
	return []string{
		"id::text AS id", "tenant_id::text AS tenant_id", "org_id::text AS org_id",
		"code", "name", "type", "address", "status",
		"created_by::text AS created_by", "updated_by::text AS updated_by",
		"created_at", "updated_at",
	}
}

func (w Warehouse) InsertColumns() []string {
	return []string{"id", "tenant_id", "org_id", "code", "name", "type", "address", "status", "created_by", "updated_by"}
}

func (w Warehouse) InsertValues() []any {
	return []any{w.ID, w.TenantID, w.OrgID, w.Code, w.Name, w.Type, w.Address, w.Status, w.CreatedBy, w.UpdatedBy}
}

func (w Warehouse) UpdateColumns() []string {
	return []string{"org_id", "code", "name", "type", "address", "status", "updated_by"}
}

func (w Warehouse) UpdateValues() []any {
	return []any{w.OrgID, w.Code, w.Name, w.Type, w.Address, w.Status, w.UpdatedBy}
}

func (w Warehouse) SearchColumns() []string { return []string{"code", "name", "address"} }
func (w Warehouse) Scoped() bool            { return true }
func (w Warehouse) GetID() string           { return w.ID }
func (w Warehouse) GetTenantID() string     { return w.TenantID }
func (w Warehouse) GetOrgID() string        { return w.OrgID }
func (w Warehouse) GetCreatedBy() string    { return w.CreatedBy }
