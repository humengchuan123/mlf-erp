-- Stage 3F: multi-currency amounts, exchange rates, realized FX and
-- period-end FX revaluation.

CREATE TABLE IF NOT EXISTS erp.finance_exchange_rates (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    currency_code text NOT NULL,
    rate_date date NOT NULL,
    rate numeric(19,8) NOT NULL,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_exchange_rates_tenant_rate_key
        UNIQUE (tenant_id, currency_code, rate_date),
    CONSTRAINT finance_exchange_rates_rate_check CHECK (rate > 0),
    CONSTRAINT finance_exchange_rates_currency_fk
        FOREIGN KEY (tenant_id, currency_code)
        REFERENCES erp.currencies (tenant_id, code) NOT VALID
);

CREATE INDEX IF NOT EXISTS finance_exchange_rates_lookup_idx
    ON erp.finance_exchange_rates (tenant_id, currency_code, rate_date DESC);

ALTER TABLE erp.sales_orders
    ADD COLUMN IF NOT EXISTS exchange_rate numeric(19,8) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS base_net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_total_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.sales_order_lines
    ADD COLUMN IF NOT EXISTS base_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_gross_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.purchase_orders
    ADD COLUMN IF NOT EXISTS exchange_rate numeric(19,8) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS base_net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_total_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.purchase_order_lines
    ADD COLUMN IF NOT EXISTS base_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_gross_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.sales_shipments
    ADD COLUMN IF NOT EXISTS exchange_rate numeric(19,8) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS base_net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_total_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.sales_shipment_lines
    ADD COLUMN IF NOT EXISTS base_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_gross_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.purchase_receipts
    ADD COLUMN IF NOT EXISTS exchange_rate numeric(19,8) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS base_net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_total_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.purchase_receipt_lines
    ADD COLUMN IF NOT EXISTS base_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_gross_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.finance_invoices
    ADD COLUMN IF NOT EXISTS exchange_rate numeric(19,8) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS base_net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_total_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_paid_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_revalued_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.finance_payments
    ADD COLUMN IF NOT EXISTS exchange_rate numeric(19,8) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS base_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_allocated_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.finance_payment_allocations
    ADD COLUMN IF NOT EXISTS base_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS invoice_base_amount numeric(19,4) NOT NULL DEFAULT 0;

UPDATE erp.sales_orders
SET base_net_amount = net_amount, base_tax_amount = tax_amount, base_total_amount = total_amount
WHERE base_total_amount = 0 AND total_amount <> 0;
UPDATE erp.sales_order_lines
SET base_amount = amount, base_tax_amount = tax_amount, base_gross_amount = gross_amount
WHERE base_gross_amount = 0 AND gross_amount <> 0;
UPDATE erp.purchase_orders
SET base_net_amount = net_amount, base_tax_amount = tax_amount, base_total_amount = total_amount
WHERE base_total_amount = 0 AND total_amount <> 0;
UPDATE erp.purchase_order_lines
SET base_amount = amount, base_tax_amount = tax_amount, base_gross_amount = gross_amount
WHERE base_gross_amount = 0 AND gross_amount <> 0;
UPDATE erp.sales_shipments
SET base_net_amount = net_amount, base_tax_amount = tax_amount, base_total_amount = total_amount
WHERE base_total_amount = 0 AND total_amount <> 0;
UPDATE erp.sales_shipment_lines
SET base_amount = amount, base_tax_amount = tax_amount, base_gross_amount = gross_amount
WHERE base_gross_amount = 0 AND gross_amount <> 0;
UPDATE erp.purchase_receipts
SET base_net_amount = net_amount, base_tax_amount = tax_amount, base_total_amount = total_amount
WHERE base_total_amount = 0 AND total_amount <> 0;
UPDATE erp.purchase_receipt_lines
SET base_amount = amount, base_tax_amount = tax_amount, base_gross_amount = gross_amount
WHERE base_gross_amount = 0 AND gross_amount <> 0;
UPDATE erp.finance_invoices
SET base_net_amount = net_amount, base_tax_amount = tax_amount,
    base_total_amount = total_amount, base_paid_amount = paid_amount
WHERE base_total_amount = 0 AND total_amount <> 0;
UPDATE erp.finance_payments
SET base_amount = amount, base_allocated_amount = allocated_amount
WHERE base_amount = 0 AND amount <> 0;
UPDATE erp.finance_payment_allocations
SET base_amount = amount, invoice_base_amount = amount
WHERE base_amount = 0 AND amount <> 0;

ALTER TABLE erp.sales_orders ADD CONSTRAINT sales_orders_exchange_rate_check CHECK (exchange_rate > 0);
ALTER TABLE erp.purchase_orders ADD CONSTRAINT purchase_orders_exchange_rate_check CHECK (exchange_rate > 0);
ALTER TABLE erp.sales_shipments ADD CONSTRAINT sales_shipments_exchange_rate_check CHECK (exchange_rate > 0);
ALTER TABLE erp.purchase_receipts ADD CONSTRAINT purchase_receipts_exchange_rate_check CHECK (exchange_rate > 0);
ALTER TABLE erp.finance_invoices ADD CONSTRAINT finance_invoices_exchange_rate_check CHECK (exchange_rate > 0);
ALTER TABLE erp.finance_payments ADD CONSTRAINT finance_payments_exchange_rate_check CHECK (exchange_rate > 0);

CREATE TABLE IF NOT EXISTS erp.finance_fx_settlements (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    payment_id uuid NOT NULL,
    invoice_id uuid NOT NULL,
    amount numeric(19,4) NOT NULL,
    payment_rate numeric(19,8) NOT NULL,
    invoice_rate numeric(19,8) NOT NULL,
    base_adjustment numeric(19,4) NOT NULL,
    voucher_id uuid,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_fx_settlements_payment_fk
        FOREIGN KEY (tenant_id, payment_id) REFERENCES erp.finance_payments (tenant_id, id),
    CONSTRAINT finance_fx_settlements_invoice_fk
        FOREIGN KEY (tenant_id, invoice_id) REFERENCES erp.finance_invoices (tenant_id, id),
    CONSTRAINT finance_fx_settlements_amount_check CHECK (amount > 0)
);
CREATE INDEX IF NOT EXISTS finance_fx_settlements_payment_idx
    ON erp.finance_fx_settlements (tenant_id, payment_id);
CREATE INDEX IF NOT EXISTS finance_fx_settlements_invoice_idx
    ON erp.finance_fx_settlements (tenant_id, invoice_id);

CREATE TABLE IF NOT EXISTS erp.gl_fx_revaluations (
    id uuid PRIMARY KEY,
    batch_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    period_id uuid NOT NULL,
    invoice_id uuid NOT NULL,
    invoice_type text NOT NULL,
    partner_id uuid NOT NULL,
    currency text NOT NULL,
    foreign_balance numeric(19,4) NOT NULL,
    old_rate numeric(19,8) NOT NULL,
    new_rate numeric(19,8) NOT NULL,
    base_adjustment numeric(19,4) NOT NULL,
    status text NOT NULL DEFAULT 'posted',
    voucher_id uuid,
    reversal_voucher_id uuid,
    reopen_voucher_id uuid,
    reopen_reversal_voucher_id uuid,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    reversed_at timestamptz,
    CONSTRAINT gl_fx_revaluations_status_check CHECK (status IN ('posted', 'reopened')),
    CONSTRAINT gl_fx_revaluations_period_fk
        FOREIGN KEY (tenant_id, period_id) REFERENCES erp.gl_periods (tenant_id, id),
    CONSTRAINT gl_fx_revaluations_invoice_fk
        FOREIGN KEY (tenant_id, invoice_id) REFERENCES erp.finance_invoices (tenant_id, id),
    CONSTRAINT gl_fx_revaluations_voucher_fk
        FOREIGN KEY (tenant_id, voucher_id) REFERENCES erp.gl_vouchers (tenant_id, id),
    CONSTRAINT gl_fx_revaluations_reversal_fk
        FOREIGN KEY (tenant_id, reversal_voucher_id) REFERENCES erp.gl_vouchers (tenant_id, id)
);
CREATE UNIQUE INDEX IF NOT EXISTS gl_fx_revaluations_posted_key
    ON erp.gl_fx_revaluations (tenant_id, period_id, invoice_id)
    WHERE status = 'posted';
CREATE INDEX IF NOT EXISTS gl_fx_revaluations_period_idx
    ON erp.gl_fx_revaluations (tenant_id, period_id, status);

INSERT INTO erp.gl_accounts (id, tenant_id, code, name, type, direction, level, is_leaf, status)
SELECT gen_random_uuid(), t.id, a.code, a.name, 'expense', a.direction, 3, true, 'active'
FROM erp.tenants t
CROSS JOIN (
    VALUES
        ('660301', '汇兑损失', 'debit'),
        ('660302', '汇兑收益', 'credit')
) AS a(code, name, direction)
ON CONFLICT (tenant_id, code) DO NOTHING;

UPDATE erp.gl_accounts SET is_leaf = false WHERE code = '6603';

UPDATE erp.gl_accounts child
SET parent_id = parent.id
FROM erp.gl_accounts parent
WHERE child.tenant_id = parent.tenant_id
  AND parent.code = '6603'
  AND child.code IN ('660301', '660302');

INSERT INTO erp.gl_account_mappings (tenant_id, key, account_code)
SELECT t.id, m.key, m.account_code
FROM erp.tenants t
CROSS JOIN (
    VALUES
        ('fx_loss', '660301'),
        ('fx_gain', '660302')
) AS m(key, account_code)
ON CONFLICT DO NOTHING;

INSERT INTO erp.permissions (code, name, module, type)
VALUES
    ('finance.exchange_rate.read', '查看汇率', 'finance', 'api'),
    ('finance.exchange_rate.write', '管理汇率', 'finance', 'api')
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name, module = EXCLUDED.module, type = EXCLUDED.type;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, p.code
FROM erp.roles r
CROSS JOIN (VALUES ('finance.exchange_rate.read'), ('finance.exchange_rate.write')) AS p(code)
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;
