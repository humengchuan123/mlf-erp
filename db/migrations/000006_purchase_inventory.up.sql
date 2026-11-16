CREATE TABLE IF NOT EXISTS erp.purchase_orders (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    org_id uuid NOT NULL,
    code text NOT NULL,
    supplier_id uuid NOT NULL,
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
    CONSTRAINT purchase_orders_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT purchase_orders_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.purchase_order_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    order_id uuid NOT NULL,
    line_no integer NOT NULL,
    material_id uuid NOT NULL,
    quantity numeric(19,6) NOT NULL,
    unit_price numeric(19,6) NOT NULL,
    amount numeric(19,4) NOT NULL,
    received_qty numeric(19,6) NOT NULL DEFAULT 0,
    CONSTRAINT purchase_order_lines_order_fk FOREIGN KEY (tenant_id, order_id)
        REFERENCES erp.purchase_orders (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT purchase_order_lines_order_line_key UNIQUE (order_id, line_no),
    CONSTRAINT purchase_order_lines_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.purchase_receipts (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    order_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    receipt_date date NOT NULL,
    status text NOT NULL DEFAULT 'posted',
    remark text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT purchase_receipts_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT purchase_receipts_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT purchase_receipts_order_fk FOREIGN KEY (tenant_id, order_id)
        REFERENCES erp.purchase_orders (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.purchase_receipt_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    receipt_id uuid NOT NULL,
    order_line_id uuid NOT NULL,
    material_id uuid NOT NULL,
    quantity numeric(19,6) NOT NULL,
    unit_price numeric(19,6) NOT NULL,
    amount numeric(19,4) NOT NULL,
    CONSTRAINT purchase_receipt_lines_receipt_fk FOREIGN KEY (tenant_id, receipt_id)
        REFERENCES erp.purchase_receipts (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT purchase_receipt_lines_order_line_fk FOREIGN KEY (tenant_id, order_line_id)
        REFERENCES erp.purchase_order_lines (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.inventory_balances (
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    material_id uuid NOT NULL,
    on_hand numeric(19,6) NOT NULL DEFAULT 0,
    reserved numeric(19,6) NOT NULL DEFAULT 0,
    available numeric(19,6) NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, warehouse_id, material_id)
);

CREATE TABLE IF NOT EXISTS erp.inventory_transactions (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    material_id uuid NOT NULL,
    direction text NOT NULL,
    biz_type text NOT NULL,
    biz_id uuid NOT NULL,
    biz_code text NOT NULL DEFAULT '',
    quantity numeric(19,6) NOT NULL,
    unit_cost numeric(19,6) NOT NULL,
    amount numeric(19,4) NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid NOT NULL
);

CREATE INDEX IF NOT EXISTS purchase_orders_list_idx
    ON erp.purchase_orders (tenant_id, org_id, status, order_date DESC);
CREATE INDEX IF NOT EXISTS purchase_orders_code_idx
    ON erp.purchase_orders (tenant_id, code);
CREATE INDEX IF NOT EXISTS purchase_order_lines_order_idx
    ON erp.purchase_order_lines (tenant_id, order_id);
CREATE INDEX IF NOT EXISTS purchase_order_lines_material_idx
    ON erp.purchase_order_lines (tenant_id, material_id);
CREATE INDEX IF NOT EXISTS purchase_receipts_order_idx
    ON erp.purchase_receipts (tenant_id, order_id, receipt_date DESC);
CREATE INDEX IF NOT EXISTS inventory_balances_material_idx
    ON erp.inventory_balances (tenant_id, material_id);
CREATE INDEX IF NOT EXISTS inventory_transactions_scope_idx
    ON erp.inventory_transactions (tenant_id, warehouse_id, material_id, occurred_at DESC);

INSERT INTO erp.permissions (code, name, module, type) VALUES
('purchase.order.read', '查看采购订单', 'purchase', 'api'),
('purchase.order.write', '管理采购订单', 'purchase', 'api'),
('purchase.order.approve', '审批采购订单', 'purchase', 'api'),
('purchase.receipt.read', '查看采购收货', 'purchase', 'api'),
('inventory.balance.read', '查看库存余额', 'inventory', 'api'),
('inventory.transaction.read', '查看库存流水', 'inventory', 'api')
ON CONFLICT (code) DO NOTHING;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, p.code
FROM erp.roles r
CROSS JOIN erp.permissions p
WHERE r.code = 'admin'
  AND p.code IN (
    'purchase.order.read', 'purchase.order.write', 'purchase.order.approve',
    'purchase.receipt.read', 'inventory.balance.read', 'inventory.transaction.read'
  )
ON CONFLICT DO NOTHING;

INSERT INTO erp.code_rules (
    id, tenant_id, entity, prefix, date_pattern, separator,
    sequence_length, reset_policy, status
)
SELECT gen_random_uuid(), id, 'purchase_receipt', 'GRN', 'yyyyMMdd', '-', 4, 'day', 'active'
FROM erp.tenants
WHERE code = 'default'
ON CONFLICT (tenant_id, entity) DO NOTHING;
