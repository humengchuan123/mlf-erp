package domain

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrInvalidInput        = errors.New("invalid input")
	ErrConflict            = errors.New("conflict")
	ErrTypeMismatch        = errors.New("invoice and payment type mismatch")
	ErrPartnerMismatch     = errors.New("invoice and payment partner mismatch")
	ErrCurrencyMismatch    = errors.New("invoice and payment currency mismatch")
	ErrOverAllocate        = errors.New("allocation exceeds outstanding amount")
	ErrInvoicePaid         = errors.New("invoice is already paid")
	ErrPaymentAllocated    = errors.New("payment is already fully allocated")
	ErrInvoiceStatus       = errors.New("invoice status does not allow this operation")
	ErrVatInvoiceDuplicate = errors.New("VAT invoice number is already used")
)
