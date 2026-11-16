package domain

import "time"

type Currency struct {
	ID            string    `db:"id" json:"id"`
	TenantID      string    `db:"tenant_id" json:"tenant_id"`
	Code          string    `db:"code" json:"code"`
	Name          string    `db:"name" json:"name"`
	Symbol        string    `db:"symbol" json:"symbol"`
	DecimalPlaces int       `db:"decimal_places" json:"decimal_places"`
	Status        string    `db:"status" json:"status"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
}

func (c Currency) TableName() string { return "erp.currencies" }

func (c Currency) SelectColumns() []string {
	return []string{
		"id::text AS id", "tenant_id::text AS tenant_id", "code", "name",
		"symbol", "decimal_places", "status", "created_at", "updated_at",
	}
}

func (c Currency) InsertColumns() []string {
	return []string{"id", "tenant_id", "code", "name", "symbol", "decimal_places", "status"}
}

func (c Currency) InsertValues() []any {
	return []any{c.ID, c.TenantID, c.Code, c.Name, c.Symbol, c.DecimalPlaces, c.Status}
}

func (c Currency) UpdateColumns() []string {
	return []string{"code", "name", "symbol", "decimal_places", "status"}
}

func (c Currency) UpdateValues() []any {
	return []any{c.Code, c.Name, c.Symbol, c.DecimalPlaces, c.Status}
}

func (c Currency) SearchColumns() []string { return []string{"code", "name"} }
func (c Currency) Scoped() bool            { return false }
func (c Currency) GetID() string           { return c.ID }
func (c Currency) GetTenantID() string     { return c.TenantID }
func (c Currency) GetOrgID() string        { return "" }
func (c Currency) GetCreatedBy() string    { return "" }
