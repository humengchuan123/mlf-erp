-- Stage 3I: purchase returns and supplier credit notes.

ALTER TABLE erp.purchase_receipt_lines
    ADD COLUMN IF NOT EXISTS returned_qty numeric(19,6) NOT NULL DEFAULT 0;
ALTER TABLE erp.purchase_receipt_lines
    ADD CONSTRAINT purchase_receipt_lines_returned_qty_check
    CHECK (returned_qty >= 0 AND returned_qty <= quantity);
ALTER TABLE erp.purchase_receipt_lines
    ADD CONSTRAINT purchase_receipt_lines_tenant_id_key UNIQUE (tenant_id, id);

CREATE TABLE IF NOT EXISTS erp.purchase_returns (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    receipt_id uuid NOT NULL,
    receipt_code text NOT NULL DEFAULT '',
    order_id uuid NOT NULL,
    order_code text NOT NULL DEFAULT '',
    supplier_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    return_date date NOT NULL,
    status text NOT NULL DEFAULT 'posted',
    currency text NOT NULL DEFAULT 'CNY',
    exchange_rate numeric(19,8) NOT NULL CHECK (exchange_rate > 0),
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
    CONSTRAINT purchase_returns_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT purchase_returns_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT purchase_returns_receipt_fk FOREIGN KEY (tenant_id, receipt_id)
        REFERENCES erp.purchase_receipts (tenant_id, id),
    CONSTRAINT purchase_returns_credit_note_fk FOREIGN KEY (tenant_id, credit_note_id)
        REFERENCES erp.finance_credit_notes (tenant_id, id)
);

CREATE INDEX IF NOT EXISTS purchase_returns_receipt_idx
    ON erp.purchase_returns (tenant_id, receipt_id, return_date DESC);
CREATE INDEX IF NOT EXISTS purchase_returns_supplier_idx
    ON erp.purchase_returns (tenant_id, supplier_id, return_date DESC);
CREATE INDEX IF NOT EXISTS purchase_returns_date_idx
    ON erp.purchase_returns (tenant_id, return_date DESC);

CREATE TABLE IF NOT EXISTS erp.purchase_return_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    return_id uuid NOT NULL,
    line_no integer NOT NULL,
    receipt_line_id uuid NOT NULL,
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
    CONSTRAINT purchase_return_lines_return_fk FOREIGN KEY (tenant_id, return_id)
        REFERENCES erp.purchase_returns (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT purchase_return_lines_receipt_line_fk FOREIGN KEY (tenant_id, receipt_line_id)
        REFERENCES erp.purchase_receipt_lines (tenant_id, id),
    CONSTRAINT purchase_return_lines_line_key UNIQUE (tenant_id, return_id, receipt_line_id),
    CONSTRAINT purchase_return_lines_quantity_check CHECK (quantity > 0)
);

CREATE INDEX IF NOT EXISTS purchase_return_lines_return_idx
    ON erp.purchase_return_lines (tenant_id, return_id);

INSERT INTO erp.code_rules (
    id, tenant_id, entity, prefix, date_pattern, separator, sequence_length, reset_policy, status
)
SELECT gen_random_uuid(), t.id, 'purchase_return', 'PR', 'yyyyMMdd', '-', 4, 'day', 'active'
FROM erp.tenants t
ON CONFLICT (tenant_id, entity) DO NOTHING;

INSERT INTO erp.permissions (code, name, module, type)
VALUES
    ('purchase.return.read', '查看采购退货', 'purchase', 'api'),
    ('purchase.return.write', '管理采购退货', 'purchase', 'api')
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name, module = EXCLUDED.module, type = EXCLUDED.type;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, p.code
FROM erp.roles r
CROSS JOIN (VALUES ('purchase.return.read'), ('purchase.return.write')) AS p(code)
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;
