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

	ErrCreditNoteClosed        = errors.New("credit note is not open")
	ErrOverApply               = errors.New("credit note allocation exceeds remaining amount")
	ErrCreditNoteOverInvoice   = errors.New("credit note allocation exceeds invoice outstanding")
	ErrOverRefund              = errors.New("credit note refund exceeds remaining amount")
	ErrInsufficientInvoiceBase = errors.New("insufficient invoice base amount")
	ErrInvoiceTypeMismatch     = errors.New("credit note and invoice type mismatch")
	ErrOrgMismatch             = errors.New("organization does not match")
	ErrRefundAmountTooSmall    = errors.New("refund cash base amount rounds to zero")
	ErrCrossRateAllocation     = errors.New("cross-rate credit note allocation is not supported")
)
