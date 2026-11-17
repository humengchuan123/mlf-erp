package app

import (
	"context"

	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
)

type PeriodService struct {
	periods domain.PeriodRepository
	codes   CodeGenerator
}

func NewPeriodService(periods domain.PeriodRepository, codes CodeGenerator) *PeriodService {
	return &PeriodService{periods: periods, codes: codes}
}

func (s *PeriodService) List(ctx context.Context, year int) ([]*domain.Period, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.periods.List(ctx, tenantID, year)
}

func (s *PeriodService) EnsureYear(ctx context.Context, year int) error {
	tenantID, _, _ := tenantActor(ctx)
	return s.periods.EnsureYear(ctx, tenantID, year)
}

func (s *PeriodService) Close(ctx context.Context, id string) error {
	tenantID, orgID, userID := tenantActor(ctx)
	code, err := s.codes.Next(ctx, tenantID, "gl_voucher")
	if err != nil {
		return err
	}
	return s.periods.Close(ctx, tenantID, orgID, id, userID, code)
}

func (s *PeriodService) Reopen(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	code, err := s.codes.Next(ctx, tenantID, "gl_voucher")
	if err != nil {
		return err
	}
	return s.periods.Reopen(ctx, tenantID, id, userID, code)
}

func (s *PeriodService) CloseYear(ctx context.Context, year int) error {
	tenantID, orgID, userID := tenantActor(ctx)
	code, err := s.codes.Next(ctx, tenantID, "gl_voucher")
	if err != nil {
		return err
	}
	return s.periods.CloseYear(ctx, tenantID, orgID, year, userID, code)
}

func (s *PeriodService) YearCloseStatus(ctx context.Context, year int) (*domain.YearClose, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.periods.YearCloseStatus(ctx, tenantID, year)
}
