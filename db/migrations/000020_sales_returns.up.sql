-- Stage 3H: sales returns, credit notes and invoice credited amounts.

ALTER TABLE erp.sales_shipment_lines
    ADD COLUMN IF NOT EXISTS returned_qty numeric(19,6) NOT NULL DEFAULT 0;
ALTER TABLE erp.sales_shipment_lines
    ADD CONSTRAINT sales_shipment_lines_returned_qty_check
    CHECK (returned_qty >= 0 AND returned_qty <= quantity);
ALTER TABLE erp.sales_shipment_lines
    ADD CONSTRAINT sales_shipment_lines_tenant_id_key UNIQUE (tenant_id, id);

ALTER TABLE erp.finance_invoices
    ADD COLUMN IF NOT EXISTS credited_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_credited_amount numeric(19,4) NOT NULL DEFAULT 0;
ALTER TABLE erp.finance_invoices
    ADD CONSTRAINT finance_invoices_credited_check
    CHECK (credited_amount >= 0 AND base_credited_amount >= 0 AND paid_amount + credited_amount <= total_amount);

CREATE TABLE IF NOT EXISTS erp.finance_credit_notes (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    credit_type text NOT NULL DEFAULT 'sales_return',
    source_type text NOT NULL,
    source_id uuid NOT NULL,
    source_code text NOT NULL DEFAULT '',
    invoice_id uuid NOT NULL,
    partner_id uuid NOT NULL,
    note_date date NOT NULL,
    currency text NOT NULL DEFAULT 'CNY',
    exchange_rate numeric(19,8) NOT NULL,
    amount numeric(19,4) NOT NULL,
    base_amount numeric(19,4) NOT NULL,
    applied_amount numeric(19,4) NOT NULL DEFAULT 0,
    applied_base_amount numeric(19,4) NOT NULL DEFAULT 0,
    remaining_amount numeric(19,4) NOT NULL,
    remaining_base_amount numeric(19,4) NOT NULL,
    status text NOT NULL,
    reason text NOT NULL DEFAULT '',
    remark text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_credit_notes_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT finance_credit_notes_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT finance_credit_notes_source_key UNIQUE (tenant_id, source_type, source_id),
    CONSTRAINT finance_credit_notes_invoice_fk FOREIGN KEY (tenant_id, invoice_id)
        REFERENCES erp.finance_invoices (tenant_id, id),
    CONSTRAINT finance_credit_notes_amount_check CHECK (
        amount > 0 AND base_amount > 0 AND applied_amount >= 0 AND applied_base_amount >= 0
        AND remaining_amount >= 0 AND remaining_base_amount >= 0
        AND applied_amount + remaining_amount = amount
        AND applied_base_amount + remaining_base_amount = base_amount
    ),
    CONSTRAINT finance_credit_notes_exchange_rate_check CHECK (exchange_rate > 0),
    CONSTRAINT finance_credit_notes_status_check CHECK (status IN ('open', 'applied'))
);

CREATE INDEX IF NOT EXISTS finance_credit_notes_list_idx
    ON erp.finance_credit_notes (tenant_id, org_id, status, note_date DESC);
CREATE INDEX IF NOT EXISTS finance_credit_notes_partner_idx
    ON erp.finance_credit_notes (tenant_id, partner_id, status);
CREATE INDEX IF NOT EXISTS finance_credit_notes_invoice_idx
    ON erp.finance_credit_notes (tenant_id, invoice_id);

CREATE TABLE IF NOT EXISTS erp.sales_returns (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    shipment_id uuid NOT NULL,
    shipment_code text NOT NULL DEFAULT '',
    order_id uuid NOT NULL,
    order_code text NOT NULL DEFAULT '',
    customer_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    return_date date NOT NULL,
    status text NOT NULL DEFAULT 'posted',
    currency text NOT NULL DEFAULT 'CNY',
    exchange_rate numeric(19,8) NOT NULL,
    price_includes_tax boolean NOT NULL DEFAULT true,
    net_amount numeric(19,4) NOT NULL DEFAULT 0,
    tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    total_amount numeric(19,4) NOT NULL DEFAULT 0,
    base_net_amount numeric(19,4) NOT NULL DEFAULT 0,
    base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    base_total_amount numeric(19,4) NOT NULL DEFAULT 0,
    cost_amount numeric(19,4) NOT NULL DEFAULT 0,
    credit_note_id uuid,
    voucher_code text NOT NULL DEFAULT '',
    reason text NOT NULL DEFAULT '',
    remark text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sales_returns_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT sales_returns_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT sales_returns_shipment_fk FOREIGN KEY (tenant_id, shipment_id)
        REFERENCES erp.sales_shipments (tenant_id, id),
    CONSTRAINT sales_returns_credit_note_fk FOREIGN KEY (tenant_id, credit_note_id)
        REFERENCES erp.finance_credit_notes (tenant_id, id),
    CONSTRAINT sales_returns_exchange_rate_check CHECK (exchange_rate > 0)
);

CREATE INDEX IF NOT EXISTS sales_returns_shipment_idx
    ON erp.sales_returns (tenant_id, shipment_id, return_date DESC);
CREATE INDEX IF NOT EXISTS sales_returns_customer_idx
    ON erp.sales_returns (tenant_id, customer_id, return_date DESC);
CREATE INDEX IF NOT EXISTS sales_returns_list_idx
    ON erp.sales_returns (tenant_id, return_date DESC);

CREATE TABLE IF NOT EXISTS erp.sales_return_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    return_id uuid NOT NULL,
    line_no integer NOT NULL,
    shipment_line_id uuid NOT NULL,
    order_line_id uuid NOT NULL,
    material_id uuid NOT NULL,
    quantity numeric(19,6) NOT NULL,
    unit_price numeric(19,6) NOT NULL,
    tax_rate_id uuid,
    tax_rate numeric(9,6) NOT NULL DEFAULT 0,
    amount numeric(19,4) NOT NULL DEFAULT 0,
    tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    gross_amount numeric(19,4) NOT NULL DEFAULT 0,
    base_amount numeric(19,4) NOT NULL DEFAULT 0,
    base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    base_gross_amount numeric(19,4) NOT NULL DEFAULT 0,
    unit_cost numeric(19,6) NOT NULL DEFAULT 0,
    cost_amount numeric(19,4) NOT NULL DEFAULT 0,
    CONSTRAINT sales_return_lines_return_fk FOREIGN KEY (tenant_id, return_id)
        REFERENCES erp.sales_returns (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT sales_return_lines_shipment_line_fk FOREIGN KEY (tenant_id, shipment_line_id)
        REFERENCES erp.sales_shipment_lines (tenant_id, id),
    CONSTRAINT sales_return_lines_line_key UNIQUE (tenant_id, return_id, shipment_line_id),
    CONSTRAINT sales_return_lines_quantity_check CHECK (quantity > 0)
);

CREATE INDEX IF NOT EXISTS sales_return_lines_return_idx
    ON erp.sales_return_lines (tenant_id, return_id);

INSERT INTO erp.code_rules (
    id, tenant_id, entity, prefix, date_pattern, separator, sequence_length, reset_policy, status
)
SELECT gen_random_uuid(), t.id, r.entity, r.prefix, 'yyyyMMdd', '-', 4, 'day', 'active'
FROM erp.tenants t
CROSS JOIN (
    VALUES
        ('sales_return', 'SR'),
        ('finance_credit_note', 'CN')
) AS r(entity, prefix)
ON CONFLICT (tenant_id, entity) DO NOTHING;

INSERT INTO erp.permissions (code, name, module, type)
VALUES
    ('sales.return.read', '查看销售退货', 'sales', 'api'),
    ('sales.return.write', '管理销售退货', 'sales', 'api')
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name, module = EXCLUDED.module, type = EXCLUDED.type;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, p.code
FROM erp.roles r
CROSS JOIN (VALUES ('sales.return.read'), ('sales.return.write')) AS p(code)
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;
