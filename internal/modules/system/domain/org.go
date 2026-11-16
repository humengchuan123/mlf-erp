package domain

import "time"

const (
	OrgTypeGroup   = "group"
	OrgTypeCompany = "company"
	OrgTypeFactory = "factory"
	OrgTypeDept    = "dept"
	OrgTypeOther   = "other"

	OrgStatusActive   = "active"
	OrgStatusDisabled = "disabled"
)

type Org struct {
	ID        string
	TenantID  string
	ParentID  string
	Type      string
	Code      string
	Name      string
	Path      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
	Children  []*Org
}
