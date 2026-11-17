DROP INDEX IF EXISTS erp.gl_fx_revaluations_period_idx;
DROP INDEX IF EXISTS erp.gl_fx_revaluations_posted_key;
DROP TABLE IF EXISTS erp.gl_fx_revaluations;

DROP INDEX IF EXISTS erp.finance_fx_settlements_invoice_idx;
DROP INDEX IF EXISTS erp.finance_fx_settlements_payment_idx;
DROP TABLE IF EXISTS erp.finance_fx_settlements;

ALTER TABLE erp.finance_payments DROP CONSTRAINT IF EXISTS finance_payments_exchange_rate_check;
ALTER TABLE erp.finance_invoices DROP CONSTRAINT IF EXISTS finance_invoices_exchange_rate_check;
ALTER TABLE erp.purchase_receipts DROP CONSTRAINT IF EXISTS purchase_receipts_exchange_rate_check;
ALTER TABLE erp.sales_shipments DROP CONSTRAINT IF EXISTS sales_shipments_exchange_rate_check;
ALTER TABLE erp.purchase_orders DROP CONSTRAINT IF EXISTS purchase_orders_exchange_rate_check;
ALTER TABLE erp.sales_orders DROP CONSTRAINT IF EXISTS sales_orders_exchange_rate_check;

ALTER TABLE erp.finance_payment_allocations
    DROP COLUMN IF EXISTS invoice_base_amount,
    DROP COLUMN IF EXISTS base_amount;

ALTER TABLE erp.finance_payments
    DROP COLUMN IF EXISTS base_allocated_amount,
    DROP COLUMN IF EXISTS base_amount,
    DROP COLUMN IF EXISTS exchange_rate;

ALTER TABLE erp.finance_invoices
    DROP COLUMN IF EXISTS base_revalued_amount,
    DROP COLUMN IF EXISTS base_paid_amount,
    DROP COLUMN IF EXISTS base_total_amount,
    DROP COLUMN IF EXISTS base_tax_amount,
    DROP COLUMN IF EXISTS base_net_amount,
    DROP COLUMN IF EXISTS exchange_rate;

ALTER TABLE erp.purchase_receipt_lines
    DROP COLUMN IF EXISTS base_gross_amount,
    DROP COLUMN IF EXISTS base_tax_amount,
    DROP COLUMN IF EXISTS base_amount;

ALTER TABLE erp.purchase_receipts
    DROP COLUMN IF EXISTS base_total_amount,
    DROP COLUMN IF EXISTS base_tax_amount,
    DROP COLUMN IF EXISTS base_net_amount,
    DROP COLUMN IF EXISTS exchange_rate;

ALTER TABLE erp.sales_shipment_lines
    DROP COLUMN IF EXISTS base_gross_amount,
    DROP COLUMN IF EXISTS base_tax_amount,
    DROP COLUMN IF EXISTS base_amount;

ALTER TABLE erp.sales_shipments
    DROP COLUMN IF EXISTS base_total_amount,
    DROP COLUMN IF EXISTS base_tax_amount,
    DROP COLUMN IF EXISTS base_net_amount,
    DROP COLUMN IF EXISTS exchange_rate;

ALTER TABLE erp.purchase_order_lines
    DROP COLUMN IF EXISTS base_gross_amount,
    DROP COLUMN IF EXISTS base_tax_amount,
    DROP COLUMN IF EXISTS base_amount;

ALTER TABLE erp.purchase_orders
    DROP COLUMN IF EXISTS base_total_amount,
    DROP COLUMN IF EXISTS base_tax_amount,
    DROP COLUMN IF EXISTS base_net_amount,
    DROP COLUMN IF EXISTS exchange_rate;

ALTER TABLE erp.sales_order_lines
    DROP COLUMN IF EXISTS base_gross_amount,
    DROP COLUMN IF EXISTS base_tax_amount,
    DROP COLUMN IF EXISTS base_amount;

ALTER TABLE erp.sales_orders
    DROP COLUMN IF EXISTS base_total_amount,
    DROP COLUMN IF EXISTS base_tax_amount,
    DROP COLUMN IF EXISTS base_net_amount,
    DROP COLUMN IF EXISTS exchange_rate;

DROP INDEX IF EXISTS erp.finance_exchange_rates_lookup_idx;
DROP TABLE IF EXISTS erp.finance_exchange_rates;

DELETE FROM erp.role_permissions WHERE permission_code IN ('finance.exchange_rate.read', 'finance.exchange_rate.write');
DELETE FROM erp.permissions WHERE code IN ('finance.exchange_rate.read', 'finance.exchange_rate.write');
DELETE FROM erp.gl_account_mappings WHERE key IN ('fx_loss', 'fx_gain');

DELETE FROM erp.gl_accounts a
WHERE a.code IN ('660301', '660302')
  AND NOT EXISTS (
      SELECT 1 FROM erp.gl_voucher_lines l
      WHERE l.tenant_id = a.tenant_id AND l.account_id = a.id
  );

UPDATE erp.gl_accounts
SET is_leaf = true
WHERE code = '6603'
  AND NOT EXISTS (
      SELECT 1 FROM erp.gl_accounts child
      WHERE child.tenant_id = erp.gl_accounts.tenant_id
        AND child.parent_id = erp.gl_accounts.id
  );
