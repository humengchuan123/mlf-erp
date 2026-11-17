package app

import (
	"context"

	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
)

type PeriodService struct {
	periods domain.PeriodRepository
}

func NewPeriodService(periods domain.PeriodRepository) *PeriodService {
	return &PeriodService{periods: periods}
}

func (s *PeriodService) List(ctx context.Context, year int) ([]*domain.Period, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.periods.List(ctx, tenantID, year)
}

func (s *PeriodService) Close(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	return s.periods.Close(ctx, tenantID, id, userID)
}

func (s *PeriodService) Reopen(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	return s.periods.Reopen(ctx, tenantID, id, userID)
}
