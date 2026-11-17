ALTER TABLE erp.sales_order_lines
    DROP CONSTRAINT IF EXISTS sales_order_lines_quantity_check;
ALTER TABLE erp.sales_shipment_lines
    DROP CONSTRAINT IF EXISTS sales_shipment_lines_quantity_check;
ALTER TABLE erp.purchase_order_lines
    DROP CONSTRAINT IF EXISTS purchase_order_lines_quantity_check;
ALTER TABLE erp.purchase_receipt_lines
    DROP CONSTRAINT IF EXISTS purchase_receipt_lines_quantity_check;
