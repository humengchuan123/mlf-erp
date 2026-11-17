package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

type ReportService struct {
	reports domain.ReportRepository
}

func NewReportService(reports domain.ReportRepository) *ReportService {
	return &ReportService{reports: reports}
}

func (s *ReportService) TrialBalance(ctx context.Context, startDate, endDate time.Time) (*domain.TrialBalance, error) {
	tenantID, _, _ := tenantActor(ctx)
	startDate, endDate = normalizeRange(startDate, endDate)
	if startDate.After(endDate) {
		return nil, domain.ErrInvalidInput
	}
	return s.reports.TrialBalance(ctx, scope.FromActor(ctx, tenantID), tenantID, startDate, endDate)
}

func (s *ReportService) GeneralLedger(
	ctx context.Context,
	accountID string,
	startDate, endDate time.Time,
) (*domain.GeneralLedger, error) {
	tenantID, _, _ := tenantActor(ctx)
	accountID = trim(accountID)
	if _, err := uuid.Parse(accountID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	startDate, endDate = normalizeRange(startDate, endDate)
	if startDate.After(endDate) {
		return nil, domain.ErrInvalidInput
	}
	return s.reports.GeneralLedger(ctx, scope.FromActor(ctx, tenantID), tenantID, accountID, startDate, endDate)
}

func (s *ReportService) BalanceSheet(ctx context.Context, asOf time.Time) (*domain.BalanceSheet, error) {
	tenantID, _, _ := tenantActor(ctx)
	if asOf.IsZero() {
		asOf = time.Now()
	}
	return s.reports.BalanceSheet(ctx, scope.FromActor(ctx, tenantID), tenantID, asOf)
}

func (s *ReportService) AuxiliaryBalance(
	ctx context.Context,
	dimension, accountID string,
	startDate, endDate time.Time,
) (*domain.AuxiliaryBalance, error) {
	tenantID, _, _ := tenantActor(ctx)
	switch dimension {
	case "partner", "department", "project":
	default:
		return nil, domain.ErrInvalidInput
	}
	startDate, endDate = normalizeRange(startDate, endDate)
	if startDate.After(endDate) {
		return nil, domain.ErrInvalidInput
	}
	accountID = trim(accountID)
	if accountID != "" {
		if _, err := uuid.Parse(accountID); err != nil {
			return nil, domain.ErrInvalidInput
		}
	}
	return s.reports.AuxiliaryBalance(ctx, scope.FromActor(ctx, tenantID), tenantID, dimension, accountID, startDate, endDate)
}

func (s *ReportService) VATSummary(ctx context.Context, startDate, endDate time.Time) (*domain.VATSummary, error) {
	tenantID, _, _ := tenantActor(ctx)
	startDate, endDate = normalizeRange(startDate, endDate)
	if startDate.After(endDate) {
		return nil, domain.ErrInvalidInput
	}
	return s.reports.VATSummary(ctx, scope.FromActor(ctx, tenantID), tenantID, startDate, endDate)
}

func (s *ReportService) IncomeStatement(ctx context.Context, startDate, endDate time.Time) (*domain.IncomeStatement, error) {
	tenantID, _, _ := tenantActor(ctx)
	startDate, endDate = normalizeRange(startDate, endDate)
	if startDate.After(endDate) {
		return nil, domain.ErrInvalidInput
	}
	return s.reports.IncomeStatement(ctx, scope.FromActor(ctx, tenantID), tenantID, startDate, endDate)
}

func normalizeRange(startDate, endDate time.Time) (time.Time, time.Time) {
	now := time.Now()
	if startDate.IsZero() {
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}
	if endDate.IsZero() {
		endDate = now
	}
	return startDate, endDate
}
