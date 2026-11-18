package domain

import "errors"

var (
	ErrNotFound             = errors.New("not found")
	ErrInvalidInput         = errors.New("invalid input")
	ErrConflict             = errors.New("conflict")
	ErrInvalidStatus        = errors.New("invalid order status")
	ErrOverShipment         = errors.New("shipped quantity exceeds remaining quantity")
	ErrOverReturn           = errors.New("return quantity exceeds remaining shipment quantity")
	ErrInsufficientStock    = errors.New("insufficient inventory")
	ErrShipmentLineNotFound = errors.New("shipment line not found")
)
