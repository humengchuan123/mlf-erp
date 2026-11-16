package postgres

import (
	"testing"
	"time"

	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

func TestFormatCode(t *testing.T) {
	now := time.Date(2026, 11, 16, 10, 30, 0, 0, time.Local)

	tests := []struct {
		name string
		rule domain.CodeRule
		want string
	}{
		{
			name: "day pattern",
			rule: domain.CodeRule{Prefix: "CUS", DatePattern: domain.DatePatternDay, Separator: "-", SequenceLength: 4},
			want: "CUS-20261116-0001",
		},
		{
			name: "month pattern",
			rule: domain.CodeRule{Prefix: "MAT", DatePattern: domain.DatePatternMonth, Separator: "-", SequenceLength: 3},
			want: "MAT-202611-001",
		},
		{
			name: "no date",
			rule: domain.CodeRule{Prefix: "WH", DatePattern: domain.DatePatternNone, Separator: "-", SequenceLength: 2},
			want: "WH-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatCode(&tt.rule, now, 1); got != tt.want {
				t.Fatalf("formatCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShouldReset(t *testing.T) {
	now := time.Date(2026, 11, 16, 10, 30, 0, 0, time.Local)
	yesterday := time.Date(2026, 11, 15, 9, 0, 0, 0, time.Local)
	lastMonth := time.Date(2026, 10, 16, 9, 0, 0, 0, time.Local)
	lastYear := time.Date(2025, 11, 16, 9, 0, 0, 0, time.Local)

	tests := []struct {
		name string
		rule domain.CodeRule
		want bool
	}{
		{"never", domain.CodeRule{ResetPolicy: domain.ResetPolicyNever, LastResetDate: &lastYear}, false},
		{"day same day", domain.CodeRule{ResetPolicy: domain.ResetPolicyDay, LastResetDate: &now}, false},
		{"day next day", domain.CodeRule{ResetPolicy: domain.ResetPolicyDay, LastResetDate: &yesterday}, true},
		{"month same month", domain.CodeRule{ResetPolicy: domain.ResetPolicyMonth, LastResetDate: &now}, false},
		{"month next month", domain.CodeRule{ResetPolicy: domain.ResetPolicyMonth, LastResetDate: &lastMonth}, true},
		{"year same year", domain.CodeRule{ResetPolicy: domain.ResetPolicyYear, LastResetDate: &now}, false},
		{"year next year", domain.CodeRule{ResetPolicy: domain.ResetPolicyYear, LastResetDate: &lastYear}, true},
		{"missing date", domain.CodeRule{ResetPolicy: domain.ResetPolicyDay, LastResetDate: nil}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldReset(&tt.rule, now); got != tt.want {
				t.Fatalf("shouldReset() = %v, want %v", got, tt.want)
			}
		})
	}
}
