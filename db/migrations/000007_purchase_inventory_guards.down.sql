ALTER TABLE erp.purchase_receipt_lines
    DROP CONSTRAINT IF EXISTS purchase_receipt_lines_receipt_line_key;

ALTER TABLE erp.purchase_order_lines
    DROP CONSTRAINT IF EXISTS purchase_order_lines_received_check;
