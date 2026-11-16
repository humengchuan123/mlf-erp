package domain

const (
	PermissionTypeAPI    = "api"
	PermissionTypeMenu   = "menu"
	PermissionTypeButton = "button"
	PermissionTypeField  = "field"
)

type Permission struct {
	Code        string
	Name        string
	Module      string
	Type        string
	Description string
}
