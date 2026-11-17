package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/finance/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/fx"
	"github.com/shopspring/decimal"
)

type ExchangeRateRepository struct {
	db *pgxpool.Pool
}

func NewExchangeRateRepository(db *pgxpool.Pool) *ExchangeRateRepository {
	return &ExchangeRateRepository{db: db}
}

func (r *ExchangeRateRepository) List(
	ctx context.Context,
	tenantID, currency string,
	startDate, endDate time.Time,
	offset, limit int,
) ([]*domain.ExchangeRate, int, error) {
	args := []any{tenantID}
	where := "tenant_id = $1"
	if currency != "" {
		args = append(args, currency)
		where += fmt.Sprintf(" AND currency_code = $%d", len(args))
	}
	if !startDate.IsZero() {
		args = append(args, startDate.Format("2006-01-02"))
		where += fmt.Sprintf(" AND rate_date >= $%d::date", len(args))
	}
	if !endDate.IsZero() {
		args = append(args, endDate.Format("2006-01-02"))
		where += fmt.Sprintf(" AND rate_date <= $%d::date", len(args))
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM erp.finance_exchange_rates WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count exchange rates: %w", err)
	}

	dataArgs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.Query(ctx, `
		SELECT id::text, tenant_id::text, currency_code, rate_date, rate::text,
		       created_by::text, created_at, updated_at
		FROM erp.finance_exchange_rates
		WHERE `+where+fmt.Sprintf(`
		ORDER BY rate_date DESC, currency_code
		LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2), dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list exchange rates: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.ExchangeRate, 0)
	for rows.Next() {
		item, scanErr := scanExchangeRate(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *ExchangeRateRepository) Get(ctx context.Context, tenantID, id string) (*domain.ExchangeRate, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, currency_code, rate_date, rate::text,
		       created_by::text, created_at, updated_at
		FROM erp.finance_exchange_rates
		WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return scanExchangeRate(row)
}

func (r *ExchangeRateRepository) Upsert(
	ctx context.Context,
	tenantID, userID string,
	input domain.ExchangeRateInput,
) (*domain.ExchangeRate, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO erp.finance_exchange_rates (
			id, tenant_id, currency_code, rate_date, rate, created_by
		) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, currency_code, rate_date) DO UPDATE
		SET rate = EXCLUDED.rate, created_by = EXCLUDED.created_by, updated_at = now()
		RETURNING id::text, tenant_id::text, currency_code, rate_date, rate::text,
		          created_by::text, created_at, updated_at`,
		tenantID, input.CurrencyCode, input.RateDate.Format("2006-01-02"),
		input.Rate.String(), userID)
	item, err := scanExchangeRate(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	return item, nil
}

func (r *ExchangeRateRepository) Update(
	ctx context.Context,
	tenantID, id, userID string,
	input domain.ExchangeRateInput,
) (*domain.ExchangeRate, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE erp.finance_exchange_rates
		SET currency_code = $3, rate_date = $4, rate = $5, created_by = $6, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING id::text, tenant_id::text, currency_code, rate_date, rate::text,
		          created_by::text, created_at, updated_at`,
		tenantID, id, input.CurrencyCode, input.RateDate.Format("2006-01-02"),
		input.Rate.String(), userID)
	item, err := scanExchangeRate(row)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, mapDBError(err)
	}
	return item, nil
}

func (r *ExchangeRateRepository) Latest(
	ctx context.Context,
	tenantID, currency string,
	date time.Time,
) (decimal.Decimal, error) {
	var raw string
	err := r.db.QueryRow(ctx, `
		SELECT rate::text
		FROM erp.finance_exchange_rates
		WHERE tenant_id = $1 AND currency_code = $2 AND rate_date <= $3::date
		ORDER BY rate_date DESC
		LIMIT 1`, tenantID, currency, date.Format("2006-01-02")).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return decimal.Zero, fx.ErrRateNotFound
	}
	if err != nil {
		return decimal.Zero, fmt.Errorf("load exchange rate: %w", err)
	}
	return decimal.RequireFromString(raw), nil
}

func scanExchangeRate(row rowScanner) (*domain.ExchangeRate, error) {
	var item domain.ExchangeRate
	var rate string
	err := row.Scan(
		&item.ID, &item.TenantID, &item.CurrencyCode, &item.RateDate, &rate,
		&item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan exchange rate: %w", err)
	}
	item.Rate = decimal.RequireFromString(rate)
	return &item, nil
}
