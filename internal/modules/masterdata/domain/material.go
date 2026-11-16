package domain

import "time"

type Material struct {
	ID        string    `db:"id" json:"id"`
	TenantID  string    `db:"tenant_id" json:"tenant_id"`
	OrgID     string    `db:"org_id" json:"org_id"`
	Code      string    `db:"code" json:"code"`
	Name      string    `db:"name" json:"name"`
	Spec      string    `db:"spec" json:"spec"`
	Category  string    `db:"category" json:"category"`
	UnitID    string    `db:"unit_id" json:"unit_id"`
	TaxRateID string    `db:"tax_rate_id" json:"tax_rate_id"`
	Barcode   string    `db:"barcode" json:"barcode"`
	Status    string    `db:"status" json:"status"`
	CreatedBy string    `db:"created_by" json:"created_by"`
	UpdatedBy string    `db:"updated_by" json:"updated_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func (m Material) TableName() string { return "erp.materials" }

func (m Material) SelectColumns() []string {
	return []string{
		"id::text AS id", "tenant_id::text AS tenant_id", "org_id::text AS org_id",
		"code", "name", "spec", "category",
		"COALESCE(unit_id::text, '') AS unit_id", "COALESCE(tax_rate_id::text, '') AS tax_rate_id",
		"barcode", "status", "created_by::text AS created_by", "updated_by::text AS updated_by",
		"created_at", "updated_at",
	}
}

func (m Material) InsertColumns() []string {
	return []string{
		"id", "tenant_id", "org_id", "code", "name", "spec", "category",
		"unit_id", "tax_rate_id", "barcode", "status", "created_by", "updated_by",
	}
}

func (m Material) InsertValues() []any {
	return []any{
		m.ID, m.TenantID, m.OrgID, m.Code, m.Name, m.Spec, m.Category,
		nullableUUID(m.UnitID), nullableUUID(m.TaxRateID), m.Barcode, m.Status, m.CreatedBy, m.UpdatedBy,
	}
}

func (m Material) UpdateColumns() []string {
	return []string{
		"org_id", "code", "name", "spec", "category",
		"unit_id", "tax_rate_id", "barcode", "status", "updated_by",
	}
}

func (m Material) UpdateValues() []any {
	return []any{
		m.OrgID, m.Code, m.Name, m.Spec, m.Category,
		nullableUUID(m.UnitID), nullableUUID(m.TaxRateID), m.Barcode, m.Status, m.UpdatedBy,
	}
}

func (m Material) SearchColumns() []string { return []string{"code", "name", "spec", "barcode"} }
func (m Material) Scoped() bool            { return true }
func (m Material) GetID() string           { return m.ID }
func (m Material) GetTenantID() string     { return m.TenantID }
func (m Material) GetOrgID() string        { return m.OrgID }
func (m Material) GetCreatedBy() string    { return m.CreatedBy }
