package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestOrderRecalculate(t *testing.T) {
	order := &Order{
		Lines: []*OrderLine{
			{Quantity: decimal.RequireFromString("2"), UnitPrice: decimal.RequireFromString("120.5")},
			{Quantity: decimal.RequireFromString("3"), UnitPrice: decimal.RequireFromString("80")},
		},
	}
	order.Recalculate()

	if !order.Lines[0].Amount.Equal(decimal.RequireFromString("241")) {
		t.Fatalf("line 1 amount = %s, want 241", order.Lines[0].Amount)
	}
	if !order.Lines[1].Amount.Equal(decimal.RequireFromString("240")) {
		t.Fatalf("line 2 amount = %s, want 240", order.Lines[1].Amount)
	}
	if !order.TotalAmount.Equal(decimal.RequireFromString("481")) {
		t.Fatalf("total = %s, want 481", order.TotalAmount)
	}
}

func TestOrderLineRemaining(t *testing.T) {
	line := &OrderLine{
		Quantity:   decimal.RequireFromString("10"),
		ShippedQty: decimal.RequireFromString("4"),
	}
	if !line.Remaining().Equal(decimal.RequireFromString("6")) {
		t.Fatalf("remaining = %s, want 6", line.Remaining())
	}
}

func TestOrderStatusTransitions(t *testing.T) {
	if !StatusDraft.CanEdit() || !StatusDraft.CanSubmit() || !StatusDraft.CanCancel() {
		t.Fatal("draft should be editable/submittable/cancellable")
	}
	if !StatusSubmitted.CanApprove() {
		t.Fatal("submitted should be approvable")
	}
	if StatusDraft.CanShip() || StatusSubmitted.CanShip() {
		t.Fatal("draft/submitted must not be shippable")
	}
	if !StatusApproved.CanShip() || !StatusPartiallyShipped.CanShip() {
		t.Fatal("approved/partially shipped should be shippable")
	}
	if StatusApproved.CanCancel() != true {
		t.Fatal("approved order without shipment should be cancellable")
	}
	if StatusPartiallyShipped.CanCancel() {
		t.Fatal("partially shipped order must not be cancellable")
	}
}

func TestShipmentTotals(t *testing.T) {
	shipment := &Shipment{
		Lines: []*ShipmentLine{
			{
				Quantity:   decimal.RequireFromString("2"),
				Amount:     decimal.RequireFromString("240"),
				CostAmount: decimal.RequireFromString("200"),
			},
			{
				Quantity:   decimal.RequireFromString("1"),
				Amount:     decimal.RequireFromString("80"),
				CostAmount: decimal.RequireFromString("50"),
			},
		},
	}
	if !shipment.TotalAmount().Equal(decimal.RequireFromString("320")) {
		t.Fatalf("total amount = %s, want 320", shipment.TotalAmount())
	}
	if !shipment.TotalCost().Equal(decimal.RequireFromString("250")) {
		t.Fatalf("total cost = %s, want 250", shipment.TotalCost())
	}
}
