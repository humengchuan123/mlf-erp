package domain

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidInput      = errors.New("invalid input")
	ErrConflict          = errors.New("conflict")
	ErrInvalidStatus     = errors.New("invalid order status")
	ErrOverShipment      = errors.New("shipped quantity exceeds remaining quantity")
	ErrInsufficientStock = errors.New("insufficient inventory")
)
