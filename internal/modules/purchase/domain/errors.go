package domain

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrConflict            = errors.New("conflict")
	ErrInvalidInput        = errors.New("invalid input")
	ErrInvalidStatus       = errors.New("invalid order status")
	ErrOverReceive         = errors.New("received quantity exceeds remaining quantity")
	ErrOverReturn          = errors.New("return quantity exceeds remaining receipt quantity")
	ErrInsufficientStock   = errors.New("insufficient inventory")
	ErrReceiptLineNotFound = errors.New("receipt line not found")
)
