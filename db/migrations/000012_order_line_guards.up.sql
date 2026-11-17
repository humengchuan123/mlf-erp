DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sales_order_lines_quantity_check') THEN
        ALTER TABLE erp.sales_order_lines
            ADD CONSTRAINT sales_order_lines_quantity_check
            CHECK (quantity > 0 AND unit_price >= 0);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sales_shipment_lines_quantity_check') THEN
        ALTER TABLE erp.sales_shipment_lines
            ADD CONSTRAINT sales_shipment_lines_quantity_check
            CHECK (quantity > 0 AND unit_price >= 0 AND unit_cost >= 0 AND cost_amount >= 0);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'purchase_order_lines_quantity_check') THEN
        ALTER TABLE erp.purchase_order_lines
            ADD CONSTRAINT purchase_order_lines_quantity_check
            CHECK (quantity > 0 AND unit_price >= 0);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'purchase_receipt_lines_quantity_check') THEN
        ALTER TABLE erp.purchase_receipt_lines
            ADD CONSTRAINT purchase_receipt_lines_quantity_check
            CHECK (quantity > 0 AND unit_price >= 0);
    END IF;
END $$;
