package app

import (
	"errors"
	"testing"

	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

func TestNormalizeCodeRuleRejectsCollidingResetPatterns(t *testing.T) {
	tests := []struct {
		name        string
		datePattern string
		resetPolicy string
		wantErr     bool
	}{
		{name: "none with daily reset", datePattern: domain.DatePatternNone, resetPolicy: domain.ResetPolicyDay, wantErr: true},
		{name: "year with monthly reset", datePattern: domain.DatePatternYear, resetPolicy: domain.ResetPolicyMonth, wantErr: true},
		{name: "month with daily reset", datePattern: domain.DatePatternMonth, resetPolicy: domain.ResetPolicyDay, wantErr: true},
		{name: "day with daily reset", datePattern: domain.DatePatternDay, resetPolicy: domain.ResetPolicyDay},
		{name: "month with monthly reset", datePattern: domain.DatePatternMonth, resetPolicy: domain.ResetPolicyMonth},
		{name: "year with yearly reset", datePattern: domain.DatePatternYear, resetPolicy: domain.ResetPolicyYear},
		{name: "none with never reset", datePattern: domain.DatePatternNone, resetPolicy: domain.ResetPolicyNever},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeCodeRule(CodeRuleInput{
				Entity:         "partner",
				Prefix:         "CUS",
				DatePattern:    tt.datePattern,
				SequenceLength: 4,
				ResetPolicy:    tt.resetPolicy,
			}, "t1", "rule-1")
			if tt.wantErr && !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("normalizeCodeRule() error = %v, want ErrInvalidInput", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("normalizeCodeRule() error = %v", err)
			}
		})
	}
}
