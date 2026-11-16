package domain

import "time"

const (
	TaxRateTypeOutput = "vat_output"
	TaxRateTypeInput  = "vat_input"
	TaxRateTypeOther  = "other"
)

type TaxRate struct {
	ID        string    `db:"id" json:"id"`
	TenantID  string    `db:"tenant_id" json:"tenant_id"`
	Code      string    `db:"code" json:"code"`
	Name      string    `db:"name" json:"name"`
	Rate      string    `db:"rate" json:"rate"`
	Type      string    `db:"type" json:"type"`
	Status    string    `db:"status" json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func (t TaxRate) TableName() string { return "erp.tax_rates" }

func (t TaxRate) SelectColumns() []string {
	return []string{
		"id::text AS id", "tenant_id::text AS tenant_id", "code", "name",
		"rate::text AS rate", "type", "status", "created_at", "updated_at",
	}
}

func (t TaxRate) InsertColumns() []string {
	return []string{"id", "tenant_id", "code", "name", "rate", "type", "status"}
}

func (t TaxRate) InsertValues() []any {
	return []any{t.ID, t.TenantID, t.Code, t.Name, t.Rate, t.Type, t.Status}
}

func (t TaxRate) UpdateColumns() []string {
	return []string{"code", "name", "rate", "type", "status"}
}

func (t TaxRate) UpdateValues() []any {
	return []any{t.Code, t.Name, t.Rate, t.Type, t.Status}
}

func (t TaxRate) SearchColumns() []string { return []string{"code", "name"} }
func (t TaxRate) Scoped() bool            { return false }
func (t TaxRate) GetID() string           { return t.ID }
func (t TaxRate) GetTenantID() string     { return t.TenantID }
func (t TaxRate) GetOrgID() string        { return "" }
func (t TaxRate) GetCreatedBy() string    { return "" }
