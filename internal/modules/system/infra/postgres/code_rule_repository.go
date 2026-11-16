package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

type CodeRuleRepository struct {
	db *pgxpool.Pool
}

func NewCodeRuleRepository(db *pgxpool.Pool) *CodeRuleRepository {
	return &CodeRuleRepository{db: db}
}

func (r *CodeRuleRepository) List(ctx context.Context, tenantID string) ([]*domain.CodeRule, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, tenant_id::text, entity, prefix, date_pattern, separator,
		       sequence_length, current_value, reset_policy, last_reset_date,
		       status, created_at, updated_at
		FROM erp.code_rules
		WHERE tenant_id = $1
		ORDER BY entity`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list code rules: %w", err)
	}
	defer rows.Close()

	result := make([]*domain.CodeRule, 0)
	for rows.Next() {
		rule, scanErr := scanCodeRule(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, rule)
	}
	return result, rows.Err()
}

func (r *CodeRuleRepository) Create(ctx context.Context, rule *domain.CodeRule) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO erp.code_rules (
			id, tenant_id, entity, prefix, date_pattern, separator,
			sequence_length, current_value, reset_policy, last_reset_date, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		rule.ID, rule.TenantID, rule.Entity, rule.Prefix, rule.DatePattern, rule.Separator,
		rule.SequenceLength, rule.CurrentValue, rule.ResetPolicy, rule.LastResetDate, rule.Status,
	)
	if err != nil {
		return fmt.Errorf("create code rule: %w", mapDBError(err))
	}
	return nil
}

func (r *CodeRuleRepository) Update(ctx context.Context, rule *domain.CodeRule) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE erp.code_rules
		SET prefix = $3, date_pattern = $4, separator = $5,
		    sequence_length = $6, reset_policy = $7, status = $8, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		rule.TenantID, rule.ID, rule.Prefix, rule.DatePattern, rule.Separator,
		rule.SequenceLength, rule.ResetPolicy, rule.Status,
	)
	if err != nil {
		return fmt.Errorf("update code rule: %w", mapDBError(err))
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *CodeRuleRepository) Next(ctx context.Context, tenantID, entity string) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin code generation: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, entity, prefix, date_pattern, separator,
		       sequence_length, current_value, reset_policy, last_reset_date,
		       status, created_at, updated_at
		FROM erp.code_rules
		WHERE tenant_id = $1 AND entity = $2 AND status = 'active'
		FOR UPDATE`, tenantID, entity)

	rule, err := scanCodeRule(row)
	if err != nil {
		return "", err
	}

	now := time.Now()
	currentValue := rule.CurrentValue
	if shouldReset(rule, now) {
		currentValue = 0
	}
	currentValue++

	lastResetDate := rule.LastResetDate
	if shouldReset(rule, now) {
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		lastResetDate = &today
	}

	if _, err := tx.Exec(ctx, `
		UPDATE erp.code_rules
		SET current_value = $3, last_reset_date = $4, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, rule.ID, currentValue, lastResetDate,
	); err != nil {
		return "", fmt.Errorf("update code sequence: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit code generation: %w", err)
	}

	return formatCode(rule, now, currentValue), nil
}

func scanCodeRule(row rowScanner) (*domain.CodeRule, error) {
	var rule domain.CodeRule
	var lastResetDate *time.Time

	err := row.Scan(
		&rule.ID, &rule.TenantID, &rule.Entity, &rule.Prefix, &rule.DatePattern, &rule.Separator,
		&rule.SequenceLength, &rule.CurrentValue, &rule.ResetPolicy, &lastResetDate,
		&rule.Status, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan code rule: %w", err)
	}
	rule.LastResetDate = lastResetDate
	return &rule, nil
}

func shouldReset(rule *domain.CodeRule, now time.Time) bool {
	if rule.LastResetDate == nil {
		return true
	}

	last := *rule.LastResetDate
	switch rule.ResetPolicy {
	case domain.ResetPolicyDay:
		return last.Year() != now.Year() || last.Month() != now.Month() || last.Day() != now.Day()
	case domain.ResetPolicyMonth:
		return last.Year() != now.Year() || last.Month() != now.Month()
	case domain.ResetPolicyYear:
		return last.Year() != now.Year()
	default:
		return false
	}
}

func formatCode(rule *domain.CodeRule, now time.Time, sequence int64) string {
	datePart := ""
	switch rule.DatePattern {
	case domain.DatePatternYear:
		datePart = now.Format("2006")
	case domain.DatePatternMonth:
		datePart = now.Format("200601")
	case domain.DatePatternDay:
		datePart = now.Format("20060102")
	}

	length := rule.SequenceLength
	if length <= 0 {
		length = 4
	}
	sequencePart := fmt.Sprintf("%0*d", length, sequence)

	parts := make([]string, 0, 3)
	if rule.Prefix != "" {
		parts = append(parts, rule.Prefix)
	}
	if datePart != "" {
		parts = append(parts, datePart)
	}
	parts = append(parts, sequencePart)

	separator := rule.Separator
	if separator == "" {
		separator = "-"
	}

	result := parts[0]
	for _, part := range parts[1:] {
		result += separator + part
	}
	return result
}
