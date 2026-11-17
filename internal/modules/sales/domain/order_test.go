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
	if err := order.Recalculate(); err != nil {
		t.Fatalf("recalculate: %v", err)
	}

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

func TestOrderRecalculateMixedTax(t *testing.T) {
	order := &Order{
		PriceIncludesTax: true,
		Lines: []*OrderLine{
			{Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(113), TaxRate: decimal.RequireFromString("0.13")},
			{Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(50), TaxRate: decimal.Zero},
		},
	}
	if err := order.Recalculate(); err != nil {
		t.Fatalf("recalculate: %v", err)
	}
	if !order.NetAmount.Equal(decimal.RequireFromString("150")) {
		t.Fatalf("net = %s, want 150", order.NetAmount)
	}
	if !order.TaxAmount.Equal(decimal.RequireFromString("13")) {
		t.Fatalf("tax = %s, want 13", order.TaxAmount)
	}
	if !order.TotalAmount.Equal(decimal.RequireFromString("163")) {
		t.Fatalf("gross = %s, want 163", order.TotalAmount)
	}
	if !order.Lines[0].GrossAmount.Equal(decimal.NewFromInt(113)) {
		t.Fatalf("line 1 gross = %s, want 113", order.Lines[0].GrossAmount)
	}
}

func TestOrderRecalculateExclusive(t *testing.T) {
	order := &Order{
		PriceIncludesTax: false,
		Lines: []*OrderLine{
			{Quantity: decimal.NewFromInt(2), UnitPrice: decimal.NewFromInt(50), TaxRate: decimal.RequireFromString("0.13")},
		},
	}
	if err := order.Recalculate(); err != nil {
		t.Fatalf("recalculate: %v", err)
	}
	if !order.NetAmount.Equal(decimal.NewFromInt(100)) || !order.TaxAmount.Equal(decimal.NewFromInt(13)) || !order.TotalAmount.Equal(decimal.NewFromInt(113)) {
		t.Fatalf("net/tax/gross = %s/%s/%s, want 100/13/113", order.NetAmount, order.TaxAmount, order.TotalAmount)
	}
}

func TestOrderRecalculateBaseAmounts(t *testing.T) {
	order := &Order{
		ExchangeRate:     decimal.RequireFromString("7.2"),
		PriceIncludesTax: true,
		Lines: []*OrderLine{
			{Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(113), TaxRate: decimal.RequireFromString("0.13")},
			{Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(50), TaxRate: decimal.Zero},
		},
	}
	if err := order.Recalculate(); err != nil {
		t.Fatalf("recalculate: %v", err)
	}
	if !order.BaseNetAmount.Equal(decimal.RequireFromString("1080")) {
		t.Fatalf("base net = %s, want 1080", order.BaseNetAmount)
	}
	if !order.BaseTaxAmount.Equal(decimal.RequireFromString("93.6")) {
		t.Fatalf("base tax = %s, want 93.6", order.BaseTaxAmount)
	}
	if !order.BaseTotalAmount.Equal(decimal.RequireFromString("1173.6")) {
		t.Fatalf("base gross = %s, want 1173.6", order.BaseTotalAmount)
	}
	if !order.Lines[0].BaseAmount.Equal(decimal.RequireFromString("720")) ||
		!order.Lines[0].BaseTaxAmount.Equal(decimal.RequireFromString("93.6")) ||
		!order.Lines[0].BaseGrossAmount.Equal(decimal.RequireFromString("813.6")) {
		t.Fatalf("line base net/tax/gross = %s/%s/%s, want 720/93.6/813.6",
			order.Lines[0].BaseAmount, order.Lines[0].BaseTaxAmount, order.Lines[0].BaseGrossAmount)
	}
	if !order.Lines[0].BaseAmount.Add(order.Lines[0].BaseTaxAmount).Equal(order.Lines[0].BaseGrossAmount) {
		t.Fatal("line base net + base tax must equal base gross")
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
				Quantity:    decimal.RequireFromString("2"),
				Amount:      decimal.RequireFromString("240"),
				GrossAmount: decimal.RequireFromString("240"),
				CostAmount:  decimal.RequireFromString("200"),
			},
			{
				Quantity:    decimal.RequireFromString("1"),
				Amount:      decimal.RequireFromString("80"),
				GrossAmount: decimal.RequireFromString("80"),
				CostAmount:  decimal.RequireFromString("50"),
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
