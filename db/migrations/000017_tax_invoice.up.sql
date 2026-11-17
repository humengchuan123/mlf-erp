-- Stage 3E: tax-inclusive/exclusive document amounts, VAT invoice fields,
-- input/output VAT accounts and mappings.

ALTER TABLE erp.sales_orders
    ADD COLUMN IF NOT EXISTS price_includes_tax boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.sales_order_lines
    ADD COLUMN IF NOT EXISTS tax_rate_id uuid,
    ADD COLUMN IF NOT EXISTS tax_rate numeric(9,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gross_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.purchase_orders
    ADD COLUMN IF NOT EXISTS price_includes_tax boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.purchase_order_lines
    ADD COLUMN IF NOT EXISTS tax_rate_id uuid,
    ADD COLUMN IF NOT EXISTS tax_rate numeric(9,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gross_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.sales_shipments
    ADD COLUMN IF NOT EXISTS price_includes_tax boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.sales_shipment_lines
    ADD COLUMN IF NOT EXISTS tax_rate_id uuid,
    ADD COLUMN IF NOT EXISTS tax_rate numeric(9,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gross_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.purchase_receipts
    ADD COLUMN IF NOT EXISTS price_includes_tax boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.purchase_receipt_lines
    ADD COLUMN IF NOT EXISTS tax_rate_id uuid,
    ADD COLUMN IF NOT EXISTS tax_rate numeric(9,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gross_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.finance_invoices
    ADD COLUMN IF NOT EXISTS net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS vat_invoice_no text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS vat_invoice_type text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS vat_invoice_date date,
    ADD COLUMN IF NOT EXISTS vat_invoice_status text NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS buyer_tax_no text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS seller_tax_no text NOT NULL DEFAULT '';

UPDATE erp.sales_orders SET net_amount = total_amount WHERE net_amount = 0 AND total_amount <> 0;
UPDATE erp.sales_order_lines SET gross_amount = amount WHERE gross_amount = 0 AND amount <> 0;
UPDATE erp.purchase_orders SET net_amount = total_amount WHERE net_amount = 0 AND total_amount <> 0;
UPDATE erp.purchase_order_lines SET gross_amount = amount WHERE gross_amount = 0 AND amount <> 0;
UPDATE erp.sales_shipments SET net_amount = total_amount WHERE net_amount = 0 AND total_amount <> 0;
UPDATE erp.sales_shipment_lines SET gross_amount = amount WHERE gross_amount = 0 AND amount <> 0;
UPDATE erp.purchase_receipts SET net_amount = total_amount WHERE net_amount = 0 AND total_amount <> 0;
UPDATE erp.purchase_receipt_lines SET gross_amount = amount WHERE gross_amount = 0 AND amount <> 0;
UPDATE erp.finance_invoices SET net_amount = total_amount WHERE net_amount = 0 AND total_amount <> 0;

ALTER TABLE erp.sales_order_lines
    ADD CONSTRAINT sales_order_lines_tax_rate_fk
        FOREIGN KEY (tenant_id, tax_rate_id) REFERENCES erp.tax_rates (tenant_id, id) NOT VALID;

ALTER TABLE erp.purchase_order_lines
    ADD CONSTRAINT purchase_order_lines_tax_rate_fk
        FOREIGN KEY (tenant_id, tax_rate_id) REFERENCES erp.tax_rates (tenant_id, id) NOT VALID;

ALTER TABLE erp.sales_shipment_lines
    ADD CONSTRAINT sales_shipment_lines_tax_rate_fk
        FOREIGN KEY (tenant_id, tax_rate_id) REFERENCES erp.tax_rates (tenant_id, id) NOT VALID;

ALTER TABLE erp.purchase_receipt_lines
    ADD CONSTRAINT purchase_receipt_lines_tax_rate_fk
        FOREIGN KEY (tenant_id, tax_rate_id) REFERENCES erp.tax_rates (tenant_id, id) NOT VALID;

ALTER TABLE erp.finance_invoices
    ADD CONSTRAINT finance_invoices_vat_status_check
        CHECK (vat_invoice_status IN ('pending', 'issued', 'void')),
    ADD CONSTRAINT finance_invoices_vat_type_check
        CHECK (vat_invoice_type IN ('', 'special', 'general'));

CREATE UNIQUE INDEX IF NOT EXISTS finance_invoices_vat_no_key
    ON erp.finance_invoices (tenant_id, vat_invoice_no)
    WHERE vat_invoice_no <> '';

INSERT INTO erp.gl_accounts (id, tenant_id, code, name, type, direction, level, is_leaf, status)
SELECT gen_random_uuid(), t.id, a.code, a.name, 'liability', a.direction, 3, true, 'active'
FROM erp.tenants t
CROSS JOIN (
    VALUES
        ('222101', '应交增值税-进项税额', 'debit'),
        ('222102', '应交增值税-销项税额', 'credit')
) AS a(code, name, direction)
ON CONFLICT (tenant_id, code) DO NOTHING;

UPDATE erp.gl_accounts SET is_leaf = false WHERE code = '2221';

UPDATE erp.gl_accounts child
SET parent_id = parent.id
FROM erp.gl_accounts parent
WHERE child.tenant_id = parent.tenant_id
  AND parent.code = '2221'
  AND child.code IN ('222101', '222102');

INSERT INTO erp.gl_account_mappings (tenant_id, key, account_code)
SELECT t.id, mapping.key, mapping.account_code
FROM erp.tenants t
CROSS JOIN (
    VALUES
        ('input_vat', '222101'),
        ('output_vat', '222102')
) AS mapping(key, account_code)
ON CONFLICT DO NOTHING;

INSERT INTO erp.permissions (code, name, module, type)
VALUES ('finance.invoice.write', '开具/作废发票', 'finance', 'api')
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name, module = EXCLUDED.module, type = EXCLUDED.type;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, 'finance.invoice.write'
FROM erp.roles r
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;
