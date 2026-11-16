package domain

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserDisabled       = errors.New("user is disabled")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidToken       = errors.New("invalid token")
	ErrOrgHasChildren     = errors.New("organization has children")
	ErrSystemRole         = errors.New("system role cannot be deleted")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
)
