package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestOrderRecalculate(t *testing.T) {
	order := &Order{
		Lines: []*OrderLine{
			{Quantity: decimal.RequireFromString("2"), UnitPrice: decimal.RequireFromString("10.5")},
			{Quantity: decimal.RequireFromString("3"), UnitPrice: decimal.RequireFromString("4.2")},
		},
	}
	if err := order.Recalculate(); err != nil {
		t.Fatalf("recalculate: %v", err)
	}

	if !order.Lines[0].Amount.Equal(decimal.RequireFromString("21")) {
		t.Fatalf("line 1 amount = %s, want 21", order.Lines[0].Amount)
	}
	if !order.Lines[1].Amount.Equal(decimal.RequireFromString("12.6")) {
		t.Fatalf("line 2 amount = %s, want 12.6", order.Lines[1].Amount)
	}
	if !order.TotalAmount.Equal(decimal.RequireFromString("33.6")) {
		t.Fatalf("total = %s, want 33.6", order.TotalAmount)
	}
}

func TestOrderLineRemaining(t *testing.T) {
	line := &OrderLine{
		Quantity:    decimal.RequireFromString("10"),
		ReceivedQty: decimal.RequireFromString("3.5"),
	}
	if !line.Remaining().Equal(decimal.RequireFromString("6.5")) {
		t.Fatalf("remaining = %s, want 6.5", line.Remaining())
	}
}

func TestOrderStatusTransitions(t *testing.T) {
	if !StatusDraft.CanEdit() || !StatusDraft.CanSubmit() || !StatusDraft.CanCancel() {
		t.Fatal("draft should be editable/submittable/cancellable")
	}
	if !StatusSubmitted.CanApprove() {
		t.Fatal("submitted should be approvable")
	}
	if StatusDraft.CanReceive() || StatusSubmitted.CanReceive() {
		t.Fatal("draft/submitted must not be receivable")
	}
	if !StatusApproved.CanReceive() || !StatusPartiallyReceived.CanReceive() {
		t.Fatal("approved/partially received should be receivable")
	}
	if StatusReceived.CanCancel() {
		t.Fatal("received order must not be cancellable")
	}
}
