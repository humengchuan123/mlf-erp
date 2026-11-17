package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type VoucherService struct {
	vouchers domain.VoucherRepository
	codes    CodeGenerator
}

func NewVoucherService(vouchers domain.VoucherRepository, codes CodeGenerator) *VoucherService {
	return &VoucherService{vouchers: vouchers, codes: codes}
}

type VoucherLineInput struct {
	AccountID    string
	Summary      string
	Debit        decimal.Decimal
	Credit       decimal.Decimal
	PartnerID    string
	DepartmentID string
}

type VoucherInput struct {
	VoucherDate time.Time
	Memo        string
	Lines       []VoucherLineInput
}

func (s *VoucherService) List(
	ctx context.Context,
	status, voucherType, keyword string,
	startDate, endDate time.Time,
	page, pageSize int,
) ([]*domain.Voucher, int, error) {
	tenantID, _, _ := tenantActor(ctx)
	page, pageSize = normalizePage(page, pageSize)
	return s.vouchers.List(
		ctx,
		scope.FromActor(ctx, tenantID),
		tenantID,
		trim(status),
		trim(voucherType),
		trim(keyword),
		startDate,
		endDate,
		(page-1)*pageSize,
		pageSize,
	)
}

func (s *VoucherService) Get(ctx context.Context, id string) (*domain.Voucher, error) {
	tenantID, _, _ := tenantActor(ctx)
	return s.vouchers.Get(ctx, scope.FromActor(ctx, tenantID), tenantID, id)
}

func (s *VoucherService) Create(ctx context.Context, input VoucherInput) (*domain.Voucher, error) {
	tenantID, orgID, userID := tenantActor(ctx)
	date := input.VoucherDate
	if date.IsZero() {
		date = time.Now()
	}
	lines := make([]domain.VoucherLineInput, 0, len(input.Lines))
	for _, line := range input.Lines {
		lines = append(lines, domain.VoucherLineInput{
			AccountID:    line.AccountID,
			Summary:      trim(line.Summary),
			Debit:        line.Debit.Round(4),
			Credit:       line.Credit.Round(4),
			PartnerID:    trim(line.PartnerID),
			DepartmentID: trim(line.DepartmentID),
		})
	}
	code, err := s.codes.Next(ctx, tenantID, "gl_voucher")
	if err != nil {
		return nil, err
	}
	voucher := &domain.Voucher{
		ID:          uuid.NewString(),
		TenantID:    tenantID,
		OrgID:       orgID,
		Code:        code,
		VoucherDate: date,
		VoucherType: domain.VoucherTypeManual,
		SourceType:  domain.SourceTypeManual,
		Status:      domain.VoucherStatusDraft,
		Memo:        trim(input.Memo),
		CreatedBy:   userID,
		InputLines:  lines,
	}
	if err := s.vouchers.Create(ctx, scope.FromActor(ctx, tenantID), voucher, false); err != nil {
		return nil, err
	}
	return voucher, nil
}

func (s *VoucherService) UpdateDraft(ctx context.Context, id string, input VoucherInput) (*domain.Voucher, error) {
	tenantID, _, _ := tenantActor(ctx)
	filter := scope.FromActor(ctx, tenantID)
	voucher, err := s.vouchers.Get(ctx, filter, tenantID, id)
	if err != nil {
		return nil, err
	}
	if voucher.Status != domain.VoucherStatusDraft {
		return nil, domain.ErrInvalidStatus
	}
	date := input.VoucherDate
	if date.IsZero() {
		date = voucher.VoucherDate
	}
	lines := make([]domain.VoucherLineInput, 0, len(input.Lines))
	for _, line := range input.Lines {
		lines = append(lines, domain.VoucherLineInput{
			AccountID:    line.AccountID,
			Summary:      trim(line.Summary),
			Debit:        line.Debit.Round(4),
			Credit:       line.Credit.Round(4),
			PartnerID:    trim(line.PartnerID),
			DepartmentID: trim(line.DepartmentID),
		})
	}
	voucher.VoucherDate = date
	voucher.Memo = trim(input.Memo)
	voucher.InputLines = lines
	voucher.UpdatedAt = time.Now()
	if err := s.vouchers.UpdateDraft(ctx, filter, voucher); err != nil {
		return nil, err
	}
	return voucher, nil
}

func (s *VoucherService) Post(ctx context.Context, id string) error {
	tenantID, _, userID := tenantActor(ctx)
	return s.vouchers.Post(ctx, scope.FromActor(ctx, tenantID), tenantID, id, userID)
}

func (s *VoucherService) Reverse(ctx context.Context, id string) (*domain.Voucher, error) {
	tenantID, _, userID := tenantActor(ctx)
	code, err := s.codes.Next(ctx, tenantID, "gl_voucher")
	if err != nil {
		return nil, err
	}
	return s.vouchers.Reverse(ctx, scope.FromActor(ctx, tenantID), tenantID, id, code, userID)
}
