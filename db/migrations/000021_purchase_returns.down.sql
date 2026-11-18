-- Rollback Stage 3I. Refuse when purchase return data exists because dropping
-- the tables would leave supplier credit GL/inventory/AP state inconsistent.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM erp.purchase_returns LIMIT 1) THEN
        RAISE EXCEPTION 'cannot roll back 000021_purchase_returns while purchase return data exists; archive/reverse returns first';
    END IF;
END $$;

DELETE FROM erp.role_permissions WHERE permission_code IN ('purchase.return.read', 'purchase.return.write');
DELETE FROM erp.permissions WHERE code IN ('purchase.return.read', 'purchase.return.write');
DELETE FROM erp.code_rules WHERE entity = 'purchase_return';

DROP TABLE IF EXISTS erp.purchase_return_lines;
DROP TABLE IF EXISTS erp.purchase_returns;

ALTER TABLE erp.purchase_receipt_lines
    DROP CONSTRAINT IF EXISTS purchase_receipt_lines_returned_qty_check;
ALTER TABLE erp.purchase_receipt_lines
    DROP CONSTRAINT IF EXISTS purchase_receipt_lines_tenant_id_key;
ALTER TABLE erp.purchase_receipt_lines
    DROP COLUMN IF EXISTS returned_qty;
