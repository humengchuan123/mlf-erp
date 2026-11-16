package domain

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func TestNewOrder(t *testing.T) {
	amount := decimal.RequireFromString("1234.5600")

	order, err := NewOrder("order-1", "tenant-1", "org-1", "PO-001", "SUP-001", amount, "")
	if err != nil {
		t.Fatalf("NewOrder() error = %v", err)
	}

	if order.Currency != "CNY" {
		t.Fatalf("Currency = %q, want CNY", order.Currency)
	}
	if order.Status != OrderStatusDraft {
		t.Fatalf("Status = %q, want %q", order.Status, OrderStatusDraft)
	}
	if !order.Amount.Equal(amount) {
		t.Fatalf("Amount = %s, want %s", order.Amount, amount)
	}
}

func TestNewOrderValidation(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		code     string
		amount   string
		wantErr  error
	}{
		{name: "missing tenant", tenantID: "", code: "PO-001", amount: "1.00", wantErr: ErrTenantRequired},
		{name: "missing code", tenantID: "tenant-1", code: "", amount: "1.00", wantErr: ErrCodeRequired},
		{name: "negative amount", tenantID: "tenant-1", code: "PO-001", amount: "-1.00", wantErr: ErrAmountNegative},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOrder(
				"order-1",
				tt.tenantID,
				"org-1",
				tt.code,
				"SUP-001",
				decimal.RequireFromString(tt.amount),
				"CNY",
			)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewOrder() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
