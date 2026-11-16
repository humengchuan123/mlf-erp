package domain

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

type Entity interface {
	TableName() string
	SelectColumns() []string
	InsertColumns() []string
	InsertValues() []any
	UpdateColumns() []string
	UpdateValues() []any
	SearchColumns() []string
	Scoped() bool
	GetID() string
	GetTenantID() string
	GetOrgID() string
	GetCreatedBy() string
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}
