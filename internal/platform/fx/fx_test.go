package fx_test

import (
	"context"
	"testing"
	"time"

	"github.com/mlf-erp/mlf-erp/internal/platform/fx"
	"github.com/shopspring/decimal"
)

type stubReader struct {
	rate decimal.Decimal
	err  error
}

func (s stubReader) Latest(context.Context, string, string, time.Time) (decimal.Decimal, error) {
	return s.rate, s.err
}

func TestConvert(t *testing.T) {
	got, err := fx.Convert(decimal.NewFromInt(100), decimal.RequireFromString("7.2"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if !got.Equal(decimal.RequireFromString("720")) {
		t.Fatalf("converted = %s, want 720", got)
	}
}

func TestConvertRejectsInvalidRate(t *testing.T) {
	if _, err := fx.Convert(decimal.NewFromInt(100), decimal.Zero); err == nil {
		t.Fatal("expected invalid rate error")
	}
}

func TestDiff(t *testing.T) {
	got := fx.Diff(
		decimal.NewFromInt(100),
		decimal.RequireFromString("7.4"),
		decimal.RequireFromString("7.3"),
	)
	if !got.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("diff = %s, want 10", got)
	}
}

func TestResolveBaseAndOverride(t *testing.T) {
	rate, err := fx.Resolve(context.Background(), stubReader{}, "tenant", "CNY", time.Now(), decimal.Zero)
	if err != nil || !rate.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("CNY rate = %s, err = %v", rate, err)
	}
	rate, err = fx.Resolve(context.Background(), stubReader{}, "tenant", "USD", time.Now(), decimal.RequireFromString("7.1"))
	if err != nil || !rate.Equal(decimal.RequireFromString("7.1")) {
		t.Fatalf("override rate = %s, err = %v", rate, err)
	}
}

func TestResolveUsesReader(t *testing.T) {
	rate, err := fx.Resolve(
		context.Background(),
		stubReader{rate: decimal.RequireFromString("7.25")},
		"tenant", "USD", time.Now(), decimal.Zero,
	)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !rate.Equal(decimal.RequireFromString("7.25")) {
		t.Fatalf("rate = %s, want 7.25", rate)
	}
}

func TestResolveMissingReader(t *testing.T) {
	_, err := fx.Resolve(context.Background(), nil, "tenant", "USD", time.Now(), decimal.Zero)
	if err == nil {
		t.Fatal("expected rate not found error")
	}
}
