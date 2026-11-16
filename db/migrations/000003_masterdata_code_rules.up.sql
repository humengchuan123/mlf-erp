CREATE TABLE IF NOT EXISTS erp.units (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    code text NOT NULL,
    name text NOT NULL,
    precision integer NOT NULL DEFAULT 0,
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT units_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT units_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.currencies (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    code text NOT NULL,
    name text NOT NULL,
    symbol text NOT NULL DEFAULT '',
    decimal_places integer NOT NULL DEFAULT 2,
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT currencies_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT currencies_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.tax_rates (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    code text NOT NULL,
    name text NOT NULL,
    rate numeric(9,4) NOT NULL DEFAULT 0,
    type text NOT NULL DEFAULT 'vat_output',
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tax_rates_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT tax_rates_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.partners (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    org_id uuid NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    type text NOT NULL DEFAULT 'both',
    category text NOT NULL DEFAULT '',
    contact text NOT NULL DEFAULT '',
    phone text NOT NULL DEFAULT '',
    email text NOT NULL DEFAULT '',
    address text NOT NULL DEFAULT '',
    tax_no text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT partners_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT partners_org_fk FOREIGN KEY (tenant_id, org_id)
        REFERENCES erp.organizations (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.materials (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    org_id uuid NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    spec text NOT NULL DEFAULT '',
    category text NOT NULL DEFAULT '',
    unit_id uuid,
    tax_rate_id uuid,
    barcode text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT materials_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT materials_org_fk FOREIGN KEY (tenant_id, org_id)
        REFERENCES erp.organizations (tenant_id, id),
    CONSTRAINT materials_unit_fk FOREIGN KEY (tenant_id, unit_id)
        REFERENCES erp.units (tenant_id, id),
    CONSTRAINT materials_tax_rate_fk FOREIGN KEY (tenant_id, tax_rate_id)
        REFERENCES erp.tax_rates (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.warehouses (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    org_id uuid NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    type text NOT NULL DEFAULT 'normal',
    address text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT warehouses_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT warehouses_org_fk FOREIGN KEY (tenant_id, org_id)
        REFERENCES erp.organizations (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.code_rules (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    entity text NOT NULL,
    prefix text NOT NULL,
    date_pattern text NOT NULL DEFAULT 'none',
    separator text NOT NULL DEFAULT '-',
    sequence_length integer NOT NULL DEFAULT 4,
    current_value bigint NOT NULL DEFAULT 0,
    reset_policy text NOT NULL DEFAULT 'never',
    last_reset_date date,
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT code_rules_tenant_entity_key UNIQUE (tenant_id, entity)
);

CREATE INDEX IF NOT EXISTS partners_scope_idx ON erp.partners (tenant_id, org_id, created_by, status);
CREATE INDEX IF NOT EXISTS materials_scope_idx ON erp.materials (tenant_id, org_id, created_by, status);
CREATE INDEX IF NOT EXISTS warehouses_scope_idx ON erp.warehouses (tenant_id, org_id, created_by, status);

INSERT INTO erp.permissions (code, name, module, type) VALUES
('masterdata.partner.read', '查看客商', 'masterdata', 'api'),
('masterdata.partner.write', '管理客商', 'masterdata', 'api'),
('masterdata.material.read', '查看物料', 'masterdata', 'api'),
('masterdata.material.write', '管理物料', 'masterdata', 'api'),
('masterdata.warehouse.read', '查看仓库', 'masterdata', 'api'),
('masterdata.warehouse.write', '管理仓库', 'masterdata', 'api'),
('masterdata.unit.read', '查看计量单位', 'masterdata', 'api'),
('masterdata.unit.write', '管理计量单位', 'masterdata', 'api'),
('masterdata.currency.read', '查看币种', 'masterdata', 'api'),
('masterdata.currency.write', '管理币种', 'masterdata', 'api'),
('masterdata.tax.read', '查看税率', 'masterdata', 'api'),
('masterdata.tax.write', '管理税率', 'masterdata', 'api'),
('system.code_rule.read', '查看编码规则', 'system', 'api'),
('system.code_rule.write', '管理编码规则', 'system', 'api')
ON CONFLICT (code) DO NOTHING;
