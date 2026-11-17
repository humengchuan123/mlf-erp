DROP INDEX IF EXISTS erp.finance_invoices_vat_no_key;

ALTER TABLE erp.finance_invoices
    DROP CONSTRAINT IF EXISTS finance_invoices_vat_type_check,
    DROP CONSTRAINT IF EXISTS finance_invoices_vat_status_check;

ALTER TABLE erp.purchase_receipt_lines DROP CONSTRAINT IF EXISTS purchase_receipt_lines_tax_rate_fk;
ALTER TABLE erp.sales_shipment_lines DROP CONSTRAINT IF EXISTS sales_shipment_lines_tax_rate_fk;
ALTER TABLE erp.purchase_order_lines DROP CONSTRAINT IF EXISTS purchase_order_lines_tax_rate_fk;
ALTER TABLE erp.sales_order_lines DROP CONSTRAINT IF EXISTS sales_order_lines_tax_rate_fk;

ALTER TABLE erp.finance_invoices
    DROP COLUMN IF EXISTS seller_tax_no,
    DROP COLUMN IF EXISTS buyer_tax_no,
    DROP COLUMN IF EXISTS vat_invoice_status,
    DROP COLUMN IF EXISTS vat_invoice_date,
    DROP COLUMN IF EXISTS vat_invoice_type,
    DROP COLUMN IF EXISTS vat_invoice_no,
    DROP COLUMN IF EXISTS tax_amount,
    DROP COLUMN IF EXISTS net_amount;

-- Restore the pre-3E amount semantics: old code treats line amount as gross.
UPDATE erp.sales_order_lines SET amount = gross_amount WHERE gross_amount <> 0;
UPDATE erp.sales_shipment_lines SET amount = gross_amount WHERE gross_amount <> 0;
UPDATE erp.purchase_order_lines SET amount = gross_amount WHERE gross_amount <> 0;
UPDATE erp.purchase_receipt_lines SET amount = gross_amount WHERE gross_amount <> 0;

ALTER TABLE erp.purchase_receipt_lines
    DROP COLUMN IF EXISTS gross_amount,
    DROP COLUMN IF EXISTS tax_amount,
    DROP COLUMN IF EXISTS tax_rate,
    DROP COLUMN IF EXISTS tax_rate_id;

ALTER TABLE erp.purchase_receipts
    DROP COLUMN IF EXISTS total_amount,
    DROP COLUMN IF EXISTS tax_amount,
    DROP COLUMN IF EXISTS net_amount,
    DROP COLUMN IF EXISTS price_includes_tax;

ALTER TABLE erp.sales_shipment_lines
    DROP COLUMN IF EXISTS gross_amount,
    DROP COLUMN IF EXISTS tax_amount,
    DROP COLUMN IF EXISTS tax_rate,
    DROP COLUMN IF EXISTS tax_rate_id;

ALTER TABLE erp.sales_shipments
    DROP COLUMN IF EXISTS total_amount,
    DROP COLUMN IF EXISTS tax_amount,
    DROP COLUMN IF EXISTS net_amount,
    DROP COLUMN IF EXISTS price_includes_tax;

ALTER TABLE erp.purchase_order_lines
    DROP COLUMN IF EXISTS gross_amount,
    DROP COLUMN IF EXISTS tax_amount,
    DROP COLUMN IF EXISTS tax_rate,
    DROP COLUMN IF EXISTS tax_rate_id;

ALTER TABLE erp.purchase_orders
    DROP COLUMN IF EXISTS tax_amount,
    DROP COLUMN IF EXISTS net_amount,
    DROP COLUMN IF EXISTS price_includes_tax;

ALTER TABLE erp.sales_order_lines
    DROP COLUMN IF EXISTS gross_amount,
    DROP COLUMN IF EXISTS tax_amount,
    DROP COLUMN IF EXISTS tax_rate,
    DROP COLUMN IF EXISTS tax_rate_id;

ALTER TABLE erp.sales_orders
    DROP COLUMN IF EXISTS tax_amount,
    DROP COLUMN IF EXISTS net_amount,
    DROP COLUMN IF EXISTS price_includes_tax;

DELETE FROM erp.role_permissions WHERE permission_code = 'finance.invoice.write';
DELETE FROM erp.permissions WHERE code = 'finance.invoice.write';
DELETE FROM erp.gl_account_mappings WHERE key IN ('input_vat', 'output_vat');

DELETE FROM erp.gl_accounts a
WHERE a.code IN ('222101', '222102')
  AND NOT EXISTS (
      SELECT 1 FROM erp.gl_voucher_lines l
      WHERE l.tenant_id = a.tenant_id AND l.account_id = a.id
  );

UPDATE erp.gl_accounts
SET is_leaf = true
WHERE code = '2221'
  AND NOT EXISTS (
      SELECT 1 FROM erp.gl_accounts child
      WHERE child.tenant_id = erp.gl_accounts.tenant_id
        AND child.parent_id = erp.gl_accounts.id
  );
