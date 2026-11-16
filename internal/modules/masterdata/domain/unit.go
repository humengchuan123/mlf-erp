package domain

import "time"

type Unit struct {
	ID        string    `db:"id" json:"id"`
	TenantID  string    `db:"tenant_id" json:"tenant_id"`
	Code      string    `db:"code" json:"code"`
	Name      string    `db:"name" json:"name"`
	Precision int       `db:"precision" json:"precision"`
	Status    string    `db:"status" json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func (u Unit) TableName() string { return "erp.units" }

func (u Unit) SelectColumns() []string {
	return []string{
		"id::text AS id", "tenant_id::text AS tenant_id", "code", "name",
		"precision", "status", "created_at", "updated_at",
	}
}

func (u Unit) InsertColumns() []string {
	return []string{"id", "tenant_id", "code", "name", "precision", "status"}
}

func (u Unit) InsertValues() []any {
	return []any{u.ID, u.TenantID, u.Code, u.Name, u.Precision, u.Status}
}

func (u Unit) UpdateColumns() []string {
	return []string{"code", "name", "precision", "status"}
}

func (u Unit) UpdateValues() []any {
	return []any{u.Code, u.Name, u.Precision, u.Status}
}

func (u Unit) SearchColumns() []string { return []string{"code", "name"} }
func (u Unit) Scoped() bool            { return false }
func (u Unit) GetID() string           { return u.ID }
func (u Unit) GetTenantID() string     { return u.TenantID }
func (u Unit) GetOrgID() string        { return "" }
func (u Unit) GetCreatedBy() string    { return "" }
