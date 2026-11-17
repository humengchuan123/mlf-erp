package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type CodeFunc func(ctx context.Context, tenantID, entity string) (string, error)

type BackfillResult struct {
	SalesShipments   int
	PurchaseReceipts int
	Payments         int
}

func Backfill(ctx context.Context, db *pgxpool.Pool, next CodeFunc) (*BackfillResult, error) {
	result := &BackfillResult{}

	type salesShipment struct {
		tenantID, orgID, id, code, customerID, createdBy string
		shipmentDate                                     time.Time
		netAmount, taxAmount, grossAmount, costAmount    decimal.Decimal
	}
	rows, err := db.Query(ctx, `
		SELECT s.tenant_id::text, s.org_id::text, s.id::text, s.code, s.shipment_date,
		       o.customer_id::text, s.created_by::text,
		       COALESCE(SUM(sl.base_amount), 0)::text, COALESCE(SUM(sl.base_tax_amount), 0)::text,
		       COALESCE(SUM(sl.base_gross_amount), 0)::text, COALESCE(SUM(sl.cost_amount), 0)::text
		FROM erp.sales_shipments s
		JOIN erp.sales_orders o ON o.tenant_id = s.tenant_id AND o.id = s.order_id
		JOIN erp.sales_shipment_lines sl ON sl.tenant_id = s.tenant_id AND sl.shipment_id = s.id
		WHERE NOT EXISTS (
		    SELECT 1 FROM erp.gl_vouchers v
		    WHERE v.tenant_id = s.tenant_id AND v.source_type = 'sales_shipment' AND v.source_id = s.id
		)
		GROUP BY s.id, s.tenant_id, s.org_id, s.code, s.shipment_date, o.customer_id, s.created_by
		ORDER BY s.shipment_date, s.code`)
	if err != nil {
		return result, fmt.Errorf("list sales shipments for backfill: %w", err)
	}
	salesShipments := make([]salesShipment, 0)
	for rows.Next() {
		var item salesShipment
		var netRaw, taxRaw, grossRaw, costRaw string
		if err := rows.Scan(
			&item.tenantID, &item.orgID, &item.id, &item.code, &item.shipmentDate,
			&item.customerID, &item.createdBy, &netRaw, &taxRaw, &grossRaw, &costRaw,
		); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan sales shipment for backfill: %w", err)
		}
		item.netAmount = decimal.RequireFromString(netRaw)
		item.taxAmount = decimal.RequireFromString(taxRaw)
		item.grossAmount = decimal.RequireFromString(grossRaw)
		item.costAmount = decimal.RequireFromString(costRaw)
		salesShipments = append(salesShipments, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return result, err
	}

	for _, item := range salesShipments {
		code, err := next(ctx, item.tenantID, "gl_voucher")
		if err != nil {
			return result, err
		}
		tx, err := db.Begin(ctx)
		if err != nil {
			return result, fmt.Errorf("begin sales backfill: %w", err)
		}
		if err := PostSalesShipment(ctx, tx, SalesShipmentPosting{
			TenantID: item.tenantID, OrgID: item.orgID, VoucherCode: code,
			SourceID: item.id, SourceCode: item.code, CustomerID: item.customerID,
			CreatedBy: item.createdBy, VoucherDate: item.shipmentDate,
			BaseNetAmount: item.netAmount, BaseTaxAmount: item.taxAmount,
			BaseGrossAmount: item.grossAmount, CostAmount: item.costAmount,
		}); err != nil {
			tx.Rollback(ctx)
			return result, fmt.Errorf("backfill sales shipment %s: %w", item.code, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return result, fmt.Errorf("commit sales backfill %s: %w", item.code, err)
		}
		result.SalesShipments++
	}

	type purchaseReceipt struct {
		tenantID, orgID, id, code, supplierID, createdBy string
		receiptDate                                      time.Time
		netAmount, taxAmount, grossAmount                decimal.Decimal
	}
	rows, err = db.Query(ctx, `
		SELECT r.tenant_id::text, r.org_id::text, r.id::text, r.code, r.receipt_date,
		       o.supplier_id::text, r.created_by::text,
		       COALESCE(SUM(rl.base_amount), 0)::text, COALESCE(SUM(rl.base_tax_amount), 0)::text,
		       COALESCE(SUM(rl.base_gross_amount), 0)::text
		FROM erp.purchase_receipts r
		JOIN erp.purchase_orders o ON o.tenant_id = r.tenant_id AND o.id = r.order_id
		JOIN erp.purchase_receipt_lines rl ON rl.tenant_id = r.tenant_id AND rl.receipt_id = r.id
		WHERE NOT EXISTS (
		    SELECT 1 FROM erp.gl_vouchers v
		    WHERE v.tenant_id = r.tenant_id AND v.source_type = 'purchase_receipt' AND v.source_id = r.id
		)
		GROUP BY r.id, r.tenant_id, r.org_id, r.code, r.receipt_date, o.supplier_id, r.created_by
		ORDER BY r.receipt_date, r.code`)
	if err != nil {
		return result, fmt.Errorf("list purchase receipts for backfill: %w", err)
	}
	purchaseReceipts := make([]purchaseReceipt, 0)
	for rows.Next() {
		var item purchaseReceipt
		var netRaw, taxRaw, grossRaw string
		if err := rows.Scan(
			&item.tenantID, &item.orgID, &item.id, &item.code, &item.receiptDate,
			&item.supplierID, &item.createdBy, &netRaw, &taxRaw, &grossRaw,
		); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan purchase receipt for backfill: %w", err)
		}
		item.netAmount = decimal.RequireFromString(netRaw)
		item.taxAmount = decimal.RequireFromString(taxRaw)
		item.grossAmount = decimal.RequireFromString(grossRaw)
		purchaseReceipts = append(purchaseReceipts, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return result, err
	}

	for _, item := range purchaseReceipts {
		code, err := next(ctx, item.tenantID, "gl_voucher")
		if err != nil {
			return result, err
		}
		tx, err := db.Begin(ctx)
		if err != nil {
			return result, fmt.Errorf("begin purchase backfill: %w", err)
		}
		if err := PostPurchaseReceipt(ctx, tx, PurchaseReceiptPosting{
			TenantID: item.tenantID, OrgID: item.orgID, VoucherCode: code,
			SourceID: item.id, SourceCode: item.code, SupplierID: item.supplierID,
			CreatedBy: item.createdBy, VoucherDate: item.receiptDate,
			BaseNetAmount: item.netAmount, BaseTaxAmount: item.taxAmount, BaseGrossAmount: item.grossAmount,
		}); err != nil {
			tx.Rollback(ctx)
			return result, fmt.Errorf("backfill purchase receipt %s: %w", item.code, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return result, fmt.Errorf("commit purchase backfill %s: %w", item.code, err)
		}
		result.PurchaseReceipts++
	}

	type payment struct {
		tenantID, orgID, id, code, partnerID, createdBy, paymentType, method string
		paymentDate                                                          time.Time
		amount                                                               decimal.Decimal
	}
	rows, err = db.Query(ctx, `
		SELECT p.tenant_id::text, p.org_id::text, p.id::text, p.code, p.payment_date,
		       p.partner_id::text, p.created_by::text, p.base_amount::text, p.payment_type, p.method
		FROM erp.finance_payments p
		WHERE NOT EXISTS (
		    SELECT 1 FROM erp.gl_vouchers v
		    WHERE v.tenant_id = p.tenant_id
		      AND v.source_type IN ('receipt_payment', 'supplier_payment')
		      AND v.source_id = p.id
		)
		ORDER BY p.payment_date, p.code`)
	if err != nil {
		return result, fmt.Errorf("list payments for backfill: %w", err)
	}
	payments := make([]payment, 0)
	for rows.Next() {
		var item payment
		var amountRaw string
		if err := rows.Scan(
			&item.tenantID, &item.orgID, &item.id, &item.code, &item.paymentDate,
			&item.partnerID, &item.createdBy, &amountRaw, &item.paymentType, &item.method,
		); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan payment for backfill: %w", err)
		}
		item.amount = decimal.RequireFromString(amountRaw)
		payments = append(payments, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return result, err
	}

	for _, item := range payments {
		code, err := next(ctx, item.tenantID, "gl_voucher")
		if err != nil {
			return result, err
		}
		tx, err := db.Begin(ctx)
		if err != nil {
			return result, fmt.Errorf("begin payment backfill: %w", err)
		}
		if err := PostPayment(ctx, tx, PaymentPosting{
			TenantID: item.tenantID, OrgID: item.orgID, VoucherCode: code,
			SourceID: item.id, SourceCode: item.code, PartnerID: item.partnerID,
			CreatedBy: item.createdBy, VoucherDate: item.paymentDate, BaseAmount: item.amount,
			PaymentType: item.paymentType, PaymentMethod: item.method,
		}); err != nil {
			tx.Rollback(ctx)
			return result, fmt.Errorf("backfill payment %s: %w", item.code, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return result, fmt.Errorf("commit payment backfill %s: %w", item.code, err)
		}
		result.Payments++
	}

	return result, nil
}
