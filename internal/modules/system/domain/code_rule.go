package domain

import "time"

const (
	CodeRuleStatusActive   = "active"
	CodeRuleStatusDisabled = "disabled"

	DatePatternNone  = "none"
	DatePatternYear  = "yyyy"
	DatePatternMonth = "yyyyMM"
	DatePatternDay   = "yyyyMMdd"

	ResetPolicyNever = "never"
	ResetPolicyYear  = "year"
	ResetPolicyMonth = "month"
	ResetPolicyDay   = "day"
)

type CodeRule struct {
	ID             string
	TenantID       string
	Entity         string
	Prefix         string
	DatePattern    string
	Separator      string
	SequenceLength int
	CurrentValue   int64
	ResetPolicy    string
	LastResetDate  *time.Time
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
