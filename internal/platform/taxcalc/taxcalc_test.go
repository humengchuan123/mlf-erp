package taxcalc_test

import (
	"testing"

	"github.com/mlf-erp/mlf-erp/internal/platform/taxcalc"
	"github.com/shopspring/decimal"
)

func TestCalculateInclusive(t *testing.T) {
	got, err := taxcalc.Calculate(
		decimal.NewFromInt(1),
		decimal.NewFromInt(100),
		decimal.RequireFromString("0.13"),
		true,
	)
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if !got.Net.Equal(decimal.RequireFromString("88.4956")) {
		t.Fatalf("net = %s, want 88.4956", got.Net)
	}
	if !got.Tax.Equal(decimal.RequireFromString("11.5044")) {
		t.Fatalf("tax = %s, want 11.5044", got.Tax)
	}
	if !got.Gross.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("gross = %s, want 100", got.Gross)
	}
}

func TestCalculateExclusive(t *testing.T) {
	got, err := taxcalc.Calculate(
		decimal.NewFromInt(2),
		decimal.NewFromInt(50),
		decimal.RequireFromString("0.13"),
		false,
	)
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if !got.Net.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("net = %s, want 100", got.Net)
	}
	if !got.Tax.Equal(decimal.RequireFromString("13")) {
		t.Fatalf("tax = %s, want 13", got.Tax)
	}
	if !got.Gross.Equal(decimal.RequireFromString("113")) {
		t.Fatalf("gross = %s, want 113", got.Gross)
	}
}

func TestCalculateZeroRate(t *testing.T) {
	got, err := taxcalc.Calculate(
		decimal.NewFromInt(3),
		decimal.RequireFromString("19.99"),
		decimal.Zero,
		true,
	)
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if !got.Net.Equal(decimal.RequireFromString("59.97")) {
		t.Fatalf("net = %s, want 59.97", got.Net)
	}
	if !got.Tax.IsZero() {
		t.Fatalf("tax = %s, want 0", got.Tax)
	}
	if !got.Gross.Equal(decimal.RequireFromString("59.97")) {
		t.Fatalf("gross = %s, want 59.97", got.Gross)
	}
}

func TestCalculateRoundingKeepsNetPlusTaxEqualGross(t *testing.T) {
	got, err := taxcalc.Calculate(
		decimal.NewFromInt(3),
		decimal.RequireFromString("33.3333"),
		decimal.RequireFromString("0.13"),
		true,
	)
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if !got.Net.Add(got.Tax).Equal(got.Gross) {
		t.Fatalf("net %s + tax %s != gross %s", got.Net, got.Tax, got.Gross)
	}
}

func TestCalculateRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name     string
		quantity decimal.Decimal
		price    decimal.Decimal
		rate     decimal.Decimal
	}{
		{"negative quantity", decimal.NewFromInt(-1), decimal.NewFromInt(1), decimal.Zero},
		{"negative price", decimal.NewFromInt(1), decimal.NewFromInt(-1), decimal.Zero},
		{"negative rate", decimal.NewFromInt(1), decimal.NewFromInt(1), decimal.RequireFromString("-0.1")},
		{"rate one", decimal.NewFromInt(1), decimal.NewFromInt(1), decimal.NewFromInt(1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := taxcalc.Calculate(tc.quantity, tc.price, tc.rate, true); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
