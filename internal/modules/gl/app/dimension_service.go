package app

import (
	"context"

	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
)

type DimensionService struct {
	dimensions domain.DimensionRepository
}

func NewDimensionService(dimensions domain.DimensionRepository) *DimensionService {
	return &DimensionService{dimensions: dimensions}
}

func (s *DimensionService) List(ctx context.Context) (*domain.Dimensions, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.dimensions.List(ctx, tenantID)
}
