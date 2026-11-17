-- Backfill payable invoices for purchase receipts created before Stage 3B.
-- Receipts that already have a payable invoice are left untouched.
INSERT INTO erp.finance_invoices (
    id, tenant_id, org_id, code, invoice_type, source_type, source_id, source_code,
    partner_id, invoice_date, due_date, currency, status, total_amount, paid_amount,
    remark, created_by, updated_by
)
SELECT
    gen_random_uuid(),
    r.tenant_id,
    r.org_id,
    'AP-LEGACY-' || r.code,
    'payable',
    'purchase_receipt',
    r.id,
    r.code,
    o.supplier_id,
    r.receipt_date,
    r.receipt_date + 30,
    o.currency,
    CASE WHEN SUM(rl.amount) > 0 THEN 'open' ELSE 'paid' END,
    SUM(rl.amount),
    0,
    'legacy payable backfill',
    r.created_by,
    r.created_by
FROM erp.purchase_receipts r
JOIN erp.purchase_orders o
  ON o.tenant_id = r.tenant_id AND o.id = r.order_id
JOIN erp.purchase_receipt_lines rl
  ON rl.tenant_id = r.tenant_id AND rl.receipt_id = r.id
WHERE NOT EXISTS (
    SELECT 1
    FROM erp.finance_invoices i
    WHERE i.tenant_id = r.tenant_id
      AND i.invoice_type = 'payable'
      AND i.source_type = 'purchase_receipt'
      AND i.source_id = r.id
)
GROUP BY
    r.id, r.tenant_id, r.org_id, r.code, r.receipt_date, r.created_by,
    o.supplier_id, o.currency
ON CONFLICT DO NOTHING;
