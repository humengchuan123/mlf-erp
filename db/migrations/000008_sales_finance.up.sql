ALTER TABLE erp.inventory_balances
    ADD COLUMN IF NOT EXISTS avg_cost numeric(19,6) NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'inventory_balances_non_negative'
    ) THEN
        ALTER TABLE erp.inventory_balances
            ADD CONSTRAINT inventory_balances_non_negative
            CHECK (on_hand >= 0 AND reserved >= 0 AND available >= 0);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS erp.sales_orders (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    org_id uuid NOT NULL,
    code text NOT NULL,
    customer_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    order_date date NOT NULL,
    currency text NOT NULL DEFAULT 'CNY',
    status text NOT NULL DEFAULT 'draft',
    total_amount numeric(19,4) NOT NULL DEFAULT 0,
    remark text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sales_orders_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT sales_orders_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.sales_order_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    order_id uuid NOT NULL,
    line_no integer NOT NULL,
    material_id uuid NOT NULL,
    quantity numeric(19,6) NOT NULL,
    unit_price numeric(19,6) NOT NULL,
    amount numeric(19,4) NOT NULL,
    shipped_qty numeric(19,6) NOT NULL DEFAULT 0,
    CONSTRAINT sales_order_lines_order_fk FOREIGN KEY (tenant_id, order_id)
        REFERENCES erp.sales_orders (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT sales_order_lines_order_line_key UNIQUE (order_id, line_no),
    CONSTRAINT sales_order_lines_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT sales_order_lines_shipped_check CHECK (shipped_qty >= 0 AND shipped_qty <= quantity)
);

CREATE TABLE IF NOT EXISTS erp.sales_shipments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    order_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    shipment_date date NOT NULL,
    status text NOT NULL DEFAULT 'posted',
    remark text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sales_shipments_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT sales_shipments_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT sales_shipments_order_fk FOREIGN KEY (tenant_id, order_id)
        REFERENCES erp.sales_orders (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.sales_shipment_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    order_line_id uuid NOT NULL,
    material_id uuid NOT NULL,
    quantity numeric(19,6) NOT NULL,
    unit_price numeric(19,6) NOT NULL,
    amount numeric(19,4) NOT NULL,
    unit_cost numeric(19,6) NOT NULL DEFAULT 0,
    cost_amount numeric(19,4) NOT NULL DEFAULT 0,
    CONSTRAINT sales_shipment_lines_shipment_fk FOREIGN KEY (tenant_id, shipment_id)
        REFERENCES erp.sales_shipments (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT sales_shipment_lines_order_line_fk FOREIGN KEY (tenant_id, order_line_id)
        REFERENCES erp.sales_order_lines (tenant_id, id),
    CONSTRAINT sales_shipment_lines_shipment_line_key UNIQUE (shipment_id, order_line_id)
);

CREATE TABLE IF NOT EXISTS erp.finance_invoices (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    invoice_type text NOT NULL,
    source_type text NOT NULL,
    source_id uuid NOT NULL,
    source_code text NOT NULL DEFAULT '',
    partner_id uuid NOT NULL,
    invoice_date date NOT NULL,
    due_date date NOT NULL,
    currency text NOT NULL DEFAULT 'CNY',
    status text NOT NULL DEFAULT 'open',
    total_amount numeric(19,4) NOT NULL,
    paid_amount numeric(19,4) NOT NULL DEFAULT 0,
    remark text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_invoices_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT finance_invoices_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT finance_invoices_source_key UNIQUE (tenant_id, invoice_type, source_type, source_id),
    CONSTRAINT finance_invoices_amount_check CHECK (total_amount >= 0 AND paid_amount >= 0 AND paid_amount <= total_amount)
);

CREATE TABLE IF NOT EXISTS erp.finance_payments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    payment_type text NOT NULL,
    partner_id uuid NOT NULL,
    payment_date date NOT NULL,
    currency text NOT NULL DEFAULT 'CNY',
    amount numeric(19,4) NOT NULL,
    allocated_amount numeric(19,4) NOT NULL DEFAULT 0,
    status text NOT NULL DEFAULT 'unallocated',
    method text NOT NULL DEFAULT 'bank',
    reference text NOT NULL DEFAULT '',
    remark text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_payments_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT finance_payments_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT finance_payments_amount_check CHECK (amount > 0 AND allocated_amount >= 0 AND allocated_amount <= amount)
);

CREATE TABLE IF NOT EXISTS erp.finance_payment_allocations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    payment_id uuid NOT NULL,
    invoice_id uuid NOT NULL,
    amount numeric(19,4) NOT NULL,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_payment_allocations_payment_fk FOREIGN KEY (tenant_id, payment_id)
        REFERENCES erp.finance_payments (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT finance_payment_allocations_invoice_fk FOREIGN KEY (tenant_id, invoice_id)
        REFERENCES erp.finance_invoices (tenant_id, id),
    CONSTRAINT finance_payment_allocations_payment_invoice_key UNIQUE (payment_id, invoice_id),
    CONSTRAINT finance_payment_allocations_amount_check CHECK (amount > 0)
);

CREATE INDEX IF NOT EXISTS sales_orders_list_idx
    ON erp.sales_orders (tenant_id, org_id, status, order_date DESC);
CREATE INDEX IF NOT EXISTS sales_order_lines_order_idx
    ON erp.sales_order_lines (tenant_id, order_id);
CREATE INDEX IF NOT EXISTS sales_shipments_order_idx
    ON erp.sales_shipments (tenant_id, order_id, shipment_date DESC);
CREATE INDEX IF NOT EXISTS sales_shipment_lines_shipment_idx
    ON erp.sales_shipment_lines (tenant_id, shipment_id);
CREATE INDEX IF NOT EXISTS finance_invoices_list_idx
    ON erp.finance_invoices (tenant_id, org_id, invoice_type, status, invoice_date DESC);
CREATE INDEX IF NOT EXISTS finance_invoices_partner_idx
    ON erp.finance_invoices (tenant_id, partner_id, invoice_type);
CREATE INDEX IF NOT EXISTS finance_payments_list_idx
    ON erp.finance_payments (tenant_id, org_id, payment_type, payment_date DESC);
CREATE INDEX IF NOT EXISTS finance_payment_allocations_invoice_idx
    ON erp.finance_payment_allocations (tenant_id, invoice_id);

UPDATE erp.inventory_balances b
SET avg_cost = COALESCE((
    SELECT SUM(rl.amount) / NULLIF(SUM(rl.quantity), 0)
    FROM erp.purchase_receipt_lines rl
    JOIN erp.purchase_receipts r
      ON r.tenant_id = rl.tenant_id AND r.id = rl.receipt_id
    WHERE r.tenant_id = b.tenant_id
      AND r.warehouse_id = b.warehouse_id
      AND rl.material_id = b.material_id
), 0)
WHERE b.avg_cost = 0;

INSERT INTO erp.permissions (code, name, module, type) VALUES
('sales.order.read', '查看销售订单', 'sales', 'api'),
('sales.order.write', '管理销售订单', 'sales', 'api'),
('sales.order.approve', '审批销售订单', 'sales', 'api'),
('sales.shipment.read', '查看销售发货', 'sales', 'api'),
('finance.invoice.read', '查看应收应付', 'finance', 'api'),
('finance.payment.read', '查看收付款', 'finance', 'api'),
('finance.payment.write', '管理收付款', 'finance', 'api')
ON CONFLICT (code) DO NOTHING;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, p.code
FROM erp.roles r
CROSS JOIN erp.permissions p
WHERE r.code = 'admin'
  AND p.code IN (
    'sales.order.read', 'sales.order.write', 'sales.order.approve', 'sales.shipment.read',
    'finance.invoice.read', 'finance.payment.read', 'finance.payment.write'
  )
ON CONFLICT DO NOTHING;

INSERT INTO erp.code_rules (
    id, tenant_id, entity, prefix, date_pattern, separator,
    sequence_length, reset_policy, status
)
SELECT gen_random_uuid(), id, rule.entity, rule.prefix, 'yyyyMMdd', '-', 4, 'day', 'active'
FROM erp.tenants
CROSS JOIN (
    VALUES
        ('sales_order', 'SO'),
        ('sales_shipment', 'DN'),
        ('sales_receivable', 'AR'),
        ('purchase_payable', 'AP'),
        ('finance_receipt', 'RCPT'),
        ('finance_payment', 'PAY')
) AS rule(entity, prefix)
WHERE erp.tenants.code = 'default'
ON CONFLICT (tenant_id, entity) DO NOTHING;
