DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'purchase_receipt_lines_receipt_line_key'
    ) THEN
        ALTER TABLE erp.purchase_receipt_lines
            ADD CONSTRAINT purchase_receipt_lines_receipt_line_key
            UNIQUE (receipt_id, order_line_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'purchase_order_lines_received_check'
    ) THEN
        ALTER TABLE erp.purchase_order_lines
            ADD CONSTRAINT purchase_order_lines_received_check
            CHECK (received_qty >= 0 AND received_qty <= quantity);
    END IF;
END $$;
