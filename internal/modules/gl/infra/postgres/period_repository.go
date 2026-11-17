package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
)

type PeriodRepository struct {
	db *pgxpool.Pool
}

func NewPeriodRepository(db *pgxpool.Pool) *PeriodRepository {
	return &PeriodRepository{db: db}
}

func (r *PeriodRepository) List(ctx context.Context, tenantID string, year int) ([]*domain.Period, error) {
	if year == 0 {
		year = time.Now().Year()
	}
	rows, err := r.db.Query(ctx, periodSelectSQL+`
		WHERE tenant_id = $1 AND year = $2 ORDER BY month`, tenantID, year)
	if err != nil {
		return nil, fmt.Errorf("list periods: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Period, 0)
	for rows.Next() {
		item, scanErr := scanPeriod(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PeriodRepository) Get(ctx context.Context, tenantID, id string) (*domain.Period, error) {
	row := r.db.QueryRow(ctx, periodSelectSQL+` WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return scanPeriod(row)
}

func (r *PeriodRepository) Ensure(ctx context.Context, tenantID string, date time.Time) (*domain.Period, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin ensure period: %w", err)
	}
	defer tx.Rollback(ctx)

	period, err := ensurePeriodTx(ctx, tx, tenantID, date)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit ensure period: %w", err)
	}
	return period, nil
}

func (r *PeriodRepository) Close(ctx context.Context, tenantID, id, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin close period: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	err = tx.QueryRow(ctx, `
		SELECT status FROM erp.gl_periods
		WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock period: %w", err)
	}
	if status == domain.PeriodStatusClosed {
		return domain.ErrConflict
	}

	var draftCount int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM erp.gl_vouchers
		WHERE tenant_id = $1 AND period_id = $2 AND status = 'draft'`, tenantID, id).Scan(&draftCount); err != nil {
		return fmt.Errorf("count draft vouchers: %w", err)
	}
	if draftCount > 0 {
		return domain.ErrConflict
	}

	if _, err := tx.Exec(ctx, `
		UPDATE erp.gl_periods
		SET status = 'closed', closed_at = now(), closed_by = $3
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, userID); err != nil {
		return fmt.Errorf("close period: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit close period: %w", err)
	}
	return nil
}

func (r *PeriodRepository) Reopen(ctx context.Context, tenantID, id, _ string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE erp.gl_periods
		SET status = 'open', closed_at = NULL, closed_by = NULL
		WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("reopen period: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func ensurePeriodTx(ctx context.Context, tx pgx.Tx, tenantID string, date time.Time) (*domain.Period, error) {
	year, month := date.Year(), int(date.Month())
	start := time.Date(year, date.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.gl_periods (id, tenant_id, year, month, start_date, end_date, status)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, 'open')
		ON CONFLICT (tenant_id, year, month) DO NOTHING`,
		tenantID, year, month, start, end,
	); err != nil {
		return nil, fmt.Errorf("ensure period: %w", err)
	}

	row := tx.QueryRow(ctx, periodSelectSQL+`
		WHERE tenant_id = $1 AND year = $2 AND month = $3`, tenantID, year, month)
	return scanPeriod(row)
}

const periodSelectSQL = `
	SELECT id::text, tenant_id::text, year, month, start_date, end_date,
	       status, closed_at, COALESCE(closed_by::text, ''), created_at
	FROM erp.gl_periods`

func scanPeriod(row rowScanner) (*domain.Period, error) {
	var item domain.Period
	var closedAt sql.NullTime
	err := row.Scan(
		&item.ID, &item.TenantID, &item.Year, &item.Month, &item.StartDate, &item.EndDate,
		&item.Status, &closedAt, &item.ClosedBy, &item.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan period: %w", err)
	}
	if closedAt.Valid {
		item.ClosedAt = &closedAt.Time
	}
	return &item, nil
}
