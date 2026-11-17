package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
	"github.com/shopspring/decimal"
)

type SalesShipmentPosting struct {
	TenantID    string
	OrgID       string
	VoucherCode string
	SourceID    string
	SourceCode  string
	CustomerID  string
	CreatedBy   string
	VoucherDate time.Time
	SalesAmount decimal.Decimal
	CostAmount  decimal.Decimal
}

type PurchaseReceiptPosting struct {
	TenantID    string
	OrgID       string
	VoucherCode string
	SourceID    string
	SourceCode  string
	SupplierID  string
	CreatedBy   string
	VoucherDate time.Time
	Amount      decimal.Decimal
}

type PaymentPosting struct {
	TenantID      string
	OrgID         string
	VoucherCode   string
	SourceID      string
	SourceCode    string
	PartnerID     string
	CreatedBy     string
	VoucherDate   time.Time
	Amount        decimal.Decimal
	PaymentType   string
	PaymentMethod string
}

type postingLine struct {
	AccountKey string
	Summary    string
	Debit      decimal.Decimal
	Credit     decimal.Decimal
	PartnerID  string
}

func PostSalesShipment(ctx context.Context, tx pgx.Tx, input SalesShipmentPosting) error {
	lines := make([]postingLine, 0, 4)
	if input.SalesAmount.IsPositive() {
		lines = append(lines,
			postingLine{AccountKey: "ar", Summary: "销售应收", Debit: input.SalesAmount, PartnerID: input.CustomerID},
			postingLine{AccountKey: "revenue", Summary: "销售收入", Credit: input.SalesAmount, PartnerID: input.CustomerID},
		)
	}
	if input.CostAmount.IsPositive() {
		lines = append(lines,
			postingLine{AccountKey: "cogs", Summary: "销售成本", Debit: input.CostAmount},
			postingLine{AccountKey: "inventory", Summary: "库存出库", Credit: input.CostAmount},
		)
	}
	return postVoucherTx(ctx, tx, voucherPostingInput{
		TenantID:    input.TenantID,
		OrgID:       input.OrgID,
		VoucherCode: input.VoucherCode,
		VoucherDate: input.VoucherDate,
		VoucherType: domain.VoucherTypeSalesShipment,
		SourceType:  domain.SourceTypeSalesShipment,
		SourceID:    input.SourceID,
		SourceCode:  input.SourceCode,
		Memo:        "销售发货 " + input.SourceCode,
		CreatedBy:   input.CreatedBy,
		Lines:       lines,
	})
}

func PostPurchaseReceipt(ctx context.Context, tx pgx.Tx, input PurchaseReceiptPosting) error {
	if !input.Amount.IsPositive() {
		return nil
	}
	return postVoucherTx(ctx, tx, voucherPostingInput{
		TenantID:    input.TenantID,
		OrgID:       input.OrgID,
		VoucherCode: input.VoucherCode,
		VoucherDate: input.VoucherDate,
		VoucherType: domain.VoucherTypePurchaseReceipt,
		SourceType:  domain.SourceTypePurchaseReceipt,
		SourceID:    input.SourceID,
		SourceCode:  input.SourceCode,
		Memo:        "采购收货 " + input.SourceCode,
		CreatedBy:   input.CreatedBy,
		Lines: []postingLine{
			{AccountKey: "inventory", Summary: "采购入库", Debit: input.Amount},
			{AccountKey: "ap", Summary: "采购应付", Credit: input.Amount, PartnerID: input.SupplierID},
		},
	})
}

func PostPayment(ctx context.Context, tx pgx.Tx, input PaymentPosting) error {
	if !input.Amount.IsPositive() {
		return nil
	}
	cashKey := "bank"
	if input.PaymentMethod == "cash" {
		cashKey = "cash"
	}

	var lines []postingLine
	voucherType := domain.VoucherTypeReceiptPayment
	sourceType := domain.SourceTypeReceiptPayment
	memo := "收款 " + input.SourceCode
	if input.PaymentType == "receipt" {
		lines = []postingLine{
			{AccountKey: cashKey, Summary: "客户收款", Debit: input.Amount},
			{AccountKey: "ar", Summary: "核销应收", Credit: input.Amount, PartnerID: input.PartnerID},
		}
	} else {
		voucherType = domain.VoucherTypeSupplierPayment
		sourceType = domain.SourceTypeSupplierPayment
		memo = "付款 " + input.SourceCode
		lines = []postingLine{
			{AccountKey: "ap", Summary: "核销应付", Debit: input.Amount, PartnerID: input.PartnerID},
			{AccountKey: cashKey, Summary: "供应商付款", Credit: input.Amount},
		}
	}

	return postVoucherTx(ctx, tx, voucherPostingInput{
		TenantID:    input.TenantID,
		OrgID:       input.OrgID,
		VoucherCode: input.VoucherCode,
		VoucherDate: input.VoucherDate,
		VoucherType: voucherType,
		SourceType:  sourceType,
		SourceID:    input.SourceID,
		SourceCode:  input.SourceCode,
		Memo:        memo,
		CreatedBy:   input.CreatedBy,
		Lines:       lines,
	})
}

type voucherPostingInput struct {
	TenantID    string
	OrgID       string
	VoucherCode string
	VoucherDate time.Time
	VoucherType string
	SourceType  string
	SourceID    string
	SourceCode  string
	Memo        string
	CreatedBy   string
	Lines       []postingLine
}

func postVoucherTx(ctx context.Context, tx pgx.Tx, input voucherPostingInput) error {
	if len(input.Lines) == 0 {
		return nil
	}
	if input.VoucherCode == "" || input.SourceID == "" || input.TenantID == "" {
		return domain.ErrInvalidInput
	}

	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM erp.gl_vouchers
			WHERE tenant_id = $1 AND source_type = $2 AND source_id = $3
		)`, input.TenantID, input.SourceType, input.SourceID).Scan(&exists); err != nil {
		return fmt.Errorf("check existing voucher: %w", err)
	}
	if exists {
		return nil
	}

	date := input.VoucherDate
	if date.IsZero() {
		date = time.Now()
	}
	period, err := ensurePeriodTx(ctx, tx, input.TenantID, date)
	if err != nil {
		return err
	}
	if period.Status == domain.PeriodStatusClosed {
		return domain.ErrPeriodClosed
	}

	accountIDs, err := loadMappedAccountsTx(ctx, tx, input.TenantID, input.Lines)
	if err != nil {
		return err
	}

	voucherID := uuid.NewString()
	lines := make([]*domain.VoucherLine, 0, len(input.Lines))
	totalDebit := decimal.Zero
	totalCredit := decimal.Zero
	for i, line := range input.Lines {
		accountID := accountIDs[line.AccountKey]
		debit := line.Debit.Round(4)
		credit := line.Credit.Round(4)
		totalDebit = totalDebit.Add(debit)
		totalCredit = totalCredit.Add(credit)
		lines = append(lines, &domain.VoucherLine{
			ID:        uuid.NewString(),
			TenantID:  input.TenantID,
			VoucherID: voucherID,
			LineNo:    i + 1,
			AccountID: accountID,
			Summary:   line.Summary,
			Debit:     debit,
			Credit:    credit,
			PartnerID: line.PartnerID,
		})
	}
	if !totalDebit.Equal(totalCredit) || !totalDebit.IsPositive() {
		return domain.ErrUnbalanced
	}

	now := time.Now()
	voucher := &domain.Voucher{
		ID:          voucherID,
		TenantID:    input.TenantID,
		OrgID:       input.OrgID,
		Code:        input.VoucherCode,
		VoucherDate: date,
		PeriodID:    period.ID,
		VoucherType: input.VoucherType,
		SourceType:  input.SourceType,
		SourceID:    input.SourceID,
		SourceCode:  input.SourceCode,
		Status:      domain.VoucherStatusPosted,
		Memo:        input.Memo,
		CreatedBy:   input.CreatedBy,
		PostedBy:    input.CreatedBy,
		PostedAt:    &now,
		CreatedAt:   now,
		UpdatedAt:   now,
		TotalAmount: totalDebit.Round(4),
		Lines:       lines,
	}
	return insertVoucherTx(ctx, tx, voucher)
}

func loadMappedAccountsTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	lines []postingLine,
) (map[string]string, error) {
	keys := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		if _, exists := seen[line.AccountKey]; exists {
			continue
		}
		seen[line.AccountKey] = struct{}{}
		keys = append(keys, line.AccountKey)
	}

	accounts := make(map[string]string, len(keys))
	for _, key := range keys {
		var accountID string
		var isLeaf bool
		var status string
		err := tx.QueryRow(ctx, `
			SELECT a.id::text, a.is_leaf, a.status
			FROM erp.gl_account_mappings m
			JOIN erp.gl_accounts a ON a.tenant_id = m.tenant_id AND a.code = m.account_code
			WHERE m.tenant_id = $1 AND m.key = $2`, tenantID, key).Scan(&accountID, &isLeaf, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("missing GL account mapping %q: %w", key, domain.ErrInvalidInput)
		}
		if err != nil {
			return nil, fmt.Errorf("load GL account mapping %q: %w", key, err)
		}
		if !isLeaf {
			return nil, domain.ErrAccountNotLeaf
		}
		if status != domain.StatusActive {
			return nil, domain.ErrAccountDisabled
		}
		accounts[key] = accountID
	}
	return accounts, nil
}
