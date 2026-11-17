package domain

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrInvalidInput    = errors.New("invalid input")
	ErrConflict        = errors.New("conflict")
	ErrUnbalanced      = errors.New("voucher is not balanced")
	ErrPeriodClosed    = errors.New("accounting period is closed")
	ErrInvalidStatus   = errors.New("invalid voucher status")
	ErrAccountNotLeaf  = errors.New("account is not a leaf")
	ErrAccountDisabled = errors.New("account is disabled")
	ErrDuplicateSource = errors.New("voucher source already posted")
)
