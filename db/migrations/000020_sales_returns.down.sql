-- Rollback Stage 3H. Business data should be archived before running this
-- migration; dropping the tables loses returns, credit notes and credited
-- invoice amounts.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM erp.sales_returns LIMIT 1) THEN
        RAISE EXCEPTION 'cannot roll back 000020_sales_returns while sales return data exists; archive/reverse returns first';
    END IF;
END $$;

DELETE FROM erp.role_permissions WHERE permission_code IN ('sales.return.read', 'sales.return.write');
DELETE FROM erp.permissions WHERE code IN ('sales.return.read', 'sales.return.write');
DELETE FROM erp.code_rules WHERE entity IN ('sales_return', 'finance_credit_note');

DROP TABLE IF EXISTS erp.sales_return_lines;
DROP TABLE IF EXISTS erp.sales_returns;
DROP TABLE IF EXISTS erp.finance_credit_notes;

ALTER TABLE erp.finance_invoices
    DROP CONSTRAINT IF EXISTS finance_invoices_credited_check;
ALTER TABLE erp.finance_invoices
    DROP COLUMN IF EXISTS base_credited_amount,
    DROP COLUMN IF EXISTS credited_amount;

ALTER TABLE erp.sales_shipment_lines
    DROP CONSTRAINT IF EXISTS sales_shipment_lines_returned_qty_check;
ALTER TABLE erp.sales_shipment_lines
    DROP CONSTRAINT IF EXISTS sales_shipment_lines_tenant_id_key;
ALTER TABLE erp.sales_shipment_lines
    DROP COLUMN IF EXISTS returned_qty;
