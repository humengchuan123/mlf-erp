package domain

import "time"

const (
	PartnerTypeCustomer = "customer"
	PartnerTypeSupplier = "supplier"
	PartnerTypeBoth     = "both"
)

type Partner struct {
	ID        string    `db:"id" json:"id"`
	TenantID  string    `db:"tenant_id" json:"tenant_id"`
	OrgID     string    `db:"org_id" json:"org_id"`
	Code      string    `db:"code" json:"code"`
	Name      string    `db:"name" json:"name"`
	Type      string    `db:"type" json:"type"`
	Category  string    `db:"category" json:"category"`
	Contact   string    `db:"contact" json:"contact"`
	Phone     string    `db:"phone" json:"phone"`
	Email     string    `db:"email" json:"email"`
	Address   string    `db:"address" json:"address"`
	TaxNo     string    `db:"tax_no" json:"tax_no"`
	Status    string    `db:"status" json:"status"`
	CreatedBy string    `db:"created_by" json:"created_by"`
	UpdatedBy string    `db:"updated_by" json:"updated_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func (p Partner) TableName() string { return "erp.partners" }

func (p Partner) SelectColumns() []string {
	return []string{
		"id::text AS id", "tenant_id::text AS tenant_id", "org_id::text AS org_id",
		"code", "name", "type", "category", "contact", "phone", "email", "address", "tax_no",
		"status", "created_by::text AS created_by", "updated_by::text AS updated_by",
		"created_at", "updated_at",
	}
}

func (p Partner) InsertColumns() []string {
	return []string{
		"id", "tenant_id", "org_id", "code", "name", "type", "category", "contact",
		"phone", "email", "address", "tax_no", "status", "created_by", "updated_by",
	}
}

func (p Partner) InsertValues() []any {
	return []any{
		p.ID, p.TenantID, p.OrgID, p.Code, p.Name, p.Type, p.Category, p.Contact,
		p.Phone, p.Email, p.Address, p.TaxNo, p.Status, p.CreatedBy, p.UpdatedBy,
	}
}

func (p Partner) UpdateColumns() []string {
	return []string{
		"org_id", "code", "name", "type", "category", "contact",
		"phone", "email", "address", "tax_no", "status", "updated_by",
	}
}

func (p Partner) UpdateValues() []any {
	return []any{
		p.OrgID, p.Code, p.Name, p.Type, p.Category, p.Contact,
		p.Phone, p.Email, p.Address, p.TaxNo, p.Status, p.UpdatedBy,
	}
}

func (p Partner) SearchColumns() []string { return []string{"code", "name", "contact", "phone"} }
func (p Partner) Scoped() bool            { return true }
func (p Partner) GetID() string           { return p.ID }
func (p Partner) GetTenantID() string     { return p.TenantID }
func (p Partner) GetOrgID() string        { return p.OrgID }
func (p Partner) GetCreatedBy() string    { return p.CreatedBy }
