package domain

import (
	"context"
	"time"

	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type AccountRepository interface {
	List(ctx context.Context, tenantID string) ([]*Account, error)
	Get(ctx context.Context, tenantID, id string) (*Account, error)
	Create(ctx context.Context, account *Account) error
	Update(ctx context.Context, account *Account) error
	InUse(ctx context.Context, tenantID, id string) (bool, error)
}

type PeriodRepository interface {
	List(ctx context.Context, tenantID string, year int) ([]*Period, error)
	Get(ctx context.Context, tenantID, id string) (*Period, error)
	Ensure(ctx context.Context, tenantID string, date time.Time) (*Period, error)
	EnsureYear(ctx context.Context, tenantID string, year int) error
	Close(ctx context.Context, tenantID, orgID, id, userID, voucherCode, fxVoucherCode, fxReversalCode string) error
	Reopen(ctx context.Context, tenantID, orgID, id, userID, reversalCode, fxReopenCode, fxReversalReopenCode string) error
	CloseYear(ctx context.Context, tenantID, orgID string, year int, userID, voucherCode string) error
	YearCloseStatus(ctx context.Context, tenantID string, year int) (*YearClose, error)
}

type DimensionRepository interface {
	List(ctx context.Context, tenantID string) (*Dimensions, error)
}

type VoucherRepository interface {
	Create(ctx context.Context, filter scope.Filter, voucher *Voucher, post bool) error
	Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*Voucher, error)
	List(ctx context.Context, filter scope.Filter, tenantID, status, voucherType, keyword string, startDate, endDate time.Time, offset, limit int) ([]*Voucher, int, error)
	UpdateDraft(ctx context.Context, filter scope.Filter, voucher *Voucher) error
	Post(ctx context.Context, filter scope.Filter, tenantID, id, userID string) error
	Reverse(ctx context.Context, filter scope.Filter, tenantID, id, reversalCode, userID string) (*Voucher, error)
}

type ReportRepository interface {
	TrialBalance(ctx context.Context, filter scope.Filter, tenantID string, startDate, endDate time.Time) (*TrialBalance, error)
	GeneralLedger(ctx context.Context, filter scope.Filter, tenantID, accountID string, startDate, endDate time.Time) (*GeneralLedger, error)
	BalanceSheet(ctx context.Context, filter scope.Filter, tenantID string, asOf time.Time) (*BalanceSheet, error)
	IncomeStatement(ctx context.Context, filter scope.Filter, tenantID string, startDate, endDate time.Time) (*IncomeStatement, error)
	AuxiliaryBalance(ctx context.Context, filter scope.Filter, tenantID, dimension, accountID string, startDate, endDate time.Time) (*AuxiliaryBalance, error)
	VATSummary(ctx context.Context, filter scope.Filter, tenantID string, startDate, endDate time.Time) (*VATSummary, error)
	CashFlowStatement(ctx context.Context, filter scope.Filter, tenantID string, startDate, endDate time.Time) (*CashFlowStatement, error)
}
