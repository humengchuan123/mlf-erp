CREATE TABLE IF NOT EXISTS erp.gl_accounts (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    code text NOT NULL,
    name text NOT NULL,
    type text NOT NULL,
    direction text NOT NULL,
    parent_id uuid,
    level integer NOT NULL DEFAULT 1,
    is_leaf boolean NOT NULL DEFAULT true,
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT gl_accounts_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT gl_accounts_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT gl_accounts_type_check CHECK (type IN ('asset', 'liability', 'equity', 'revenue', 'expense')),
    CONSTRAINT gl_accounts_direction_check CHECK (direction IN ('debit', 'credit'))
);

CREATE TABLE IF NOT EXISTS erp.gl_periods (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    year integer NOT NULL,
    month integer NOT NULL,
    start_date date NOT NULL,
    end_date date NOT NULL,
    status text NOT NULL DEFAULT 'open',
    closed_at timestamptz,
    closed_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT gl_periods_tenant_year_month_key UNIQUE (tenant_id, year, month),
    CONSTRAINT gl_periods_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT gl_periods_month_check CHECK (month BETWEEN 1 AND 12),
    CONSTRAINT gl_periods_status_check CHECK (status IN ('open', 'closed'))
);

CREATE TABLE IF NOT EXISTS erp.gl_account_mappings (
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    key text NOT NULL,
    account_code text NOT NULL,
    PRIMARY KEY (tenant_id, key)
);

CREATE TABLE IF NOT EXISTS erp.gl_vouchers (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    voucher_date date NOT NULL,
    period_id uuid NOT NULL,
    voucher_type text NOT NULL,
    source_type text NOT NULL DEFAULT 'manual',
    source_id uuid,
    source_code text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft',
    memo text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    posted_by uuid,
    posted_at timestamptz,
    reversal_of_voucher_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT gl_vouchers_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT gl_vouchers_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT gl_vouchers_period_fk FOREIGN KEY (tenant_id, period_id)
        REFERENCES erp.gl_periods (tenant_id, id),
    CONSTRAINT gl_vouchers_status_check CHECK (status IN ('draft', 'posted', 'reversed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS gl_vouchers_source_key
    ON erp.gl_vouchers (tenant_id, source_type, source_id)
    WHERE source_id IS NOT NULL AND source_type NOT IN ('manual', 'reversal');

CREATE TABLE IF NOT EXISTS erp.gl_voucher_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    voucher_id uuid NOT NULL,
    line_no integer NOT NULL,
    account_id uuid NOT NULL,
    summary text NOT NULL DEFAULT '',
    debit numeric(19,4) NOT NULL DEFAULT 0,
    credit numeric(19,4) NOT NULL DEFAULT 0,
    partner_id uuid,
    department_id uuid,
    CONSTRAINT gl_voucher_lines_voucher_fk FOREIGN KEY (tenant_id, voucher_id)
        REFERENCES erp.gl_vouchers (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT gl_voucher_lines_line_key UNIQUE (voucher_id, line_no),
    CONSTRAINT gl_voucher_lines_account_fk FOREIGN KEY (tenant_id, account_id)
        REFERENCES erp.gl_accounts (tenant_id, id),
    CONSTRAINT gl_voucher_lines_amount_check CHECK (
        debit >= 0 AND credit >= 0
        AND ((debit > 0 AND credit = 0) OR (credit > 0 AND debit = 0))
    )
);

CREATE INDEX IF NOT EXISTS gl_vouchers_list_idx
    ON erp.gl_vouchers (tenant_id, org_id, status, voucher_date DESC);
CREATE INDEX IF NOT EXISTS gl_vouchers_source_idx
    ON erp.gl_vouchers (tenant_id, source_type, source_code);
CREATE INDEX IF NOT EXISTS gl_voucher_lines_voucher_idx
    ON erp.gl_voucher_lines (tenant_id, voucher_id);
CREATE INDEX IF NOT EXISTS gl_voucher_lines_account_idx
    ON erp.gl_voucher_lines (tenant_id, account_id);

INSERT INTO erp.permissions (code, name, module, type) VALUES
('finance.account.read', '查看会计科目', 'finance', 'api'),
('finance.account.write', '管理会计科目', 'finance', 'api'),
('finance.period.read', '查看会计期间', 'finance', 'api'),
('finance.period.close', '关账/反关账', 'finance', 'api'),
('finance.voucher.read', '查看凭证', 'finance', 'api'),
('finance.voucher.write', '管理凭证', 'finance', 'api'),
('finance.voucher.post', '过账凭证', 'finance', 'api'),
('finance.voucher.reverse', '反过账凭证', 'finance', 'api'),
('finance.report.read', '查看财务报表', 'finance', 'api')
ON CONFLICT (code) DO NOTHING;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, p.code
FROM erp.roles r
CROSS JOIN erp.permissions p
WHERE r.code = 'admin'
  AND p.code IN (
    'finance.account.read', 'finance.account.write', 'finance.period.read', 'finance.period.close',
    'finance.voucher.read', 'finance.voucher.write', 'finance.voucher.post', 'finance.voucher.reverse',
    'finance.report.read'
  )
ON CONFLICT DO NOTHING;

INSERT INTO erp.code_rules (
    id, tenant_id, entity, prefix, date_pattern, separator,
    sequence_length, reset_policy, status
)
SELECT gen_random_uuid(), id, 'gl_voucher', 'GL', 'yyyyMM', '-', 4, 'month', 'active'
FROM erp.tenants
ON CONFLICT (tenant_id, entity) DO NOTHING;

INSERT INTO erp.gl_accounts (
    id, tenant_id, code, name, type, direction, level, is_leaf, status
)
SELECT gen_random_uuid(), t.id, seed.code, seed.name, seed.type, seed.direction, seed.level, seed.is_leaf, 'active'
FROM erp.tenants t
CROSS JOIN (
    VALUES
        ('1000', '资产', 'asset', 'debit', 1, false),
        ('1001', '库存现金', 'asset', 'debit', 2, true),
        ('1002', '银行存款', 'asset', 'debit', 2, true),
        ('1122', '应收账款', 'asset', 'debit', 2, true),
        ('1405', '库存商品', 'asset', 'debit', 2, true),
        ('1601', '固定资产', 'asset', 'debit', 2, true),
        ('2000', '负债', 'liability', 'credit', 1, false),
        ('2202', '应付账款', 'liability', 'credit', 2, true),
        ('2211', '应付职工薪酬', 'liability', 'credit', 2, true),
        ('2221', '应交税费', 'liability', 'credit', 2, true),
        ('4000', '所有者权益', 'equity', 'credit', 1, false),
        ('4001', '实收资本', 'equity', 'credit', 2, true),
        ('4103', '本年利润', 'equity', 'credit', 2, true),
        ('4104', '利润分配', 'equity', 'credit', 2, true),
        ('6000', '收入', 'revenue', 'credit', 1, false),
        ('6001', '主营业务收入', 'revenue', 'credit', 2, true),
        ('6051', '其他业务收入', 'revenue', 'credit', 2, true),
        ('6400', '成本费用', 'expense', 'debit', 1, false),
        ('6401', '主营业务成本', 'expense', 'debit', 2, true),
        ('6601', '销售费用', 'expense', 'debit', 2, true),
        ('6602', '管理费用', 'expense', 'debit', 2, true),
        ('6603', '财务费用', 'expense', 'debit', 2, true),
        ('6801', '所得税费用', 'expense', 'debit', 2, true)
) AS seed(code, name, type, direction, level, is_leaf)
ON CONFLICT (tenant_id, code) DO NOTHING;

UPDATE erp.gl_accounts child
SET parent_id = parent.id
FROM erp.gl_accounts parent
WHERE child.tenant_id = parent.tenant_id
  AND (
      (parent.code = '1000' AND child.code IN ('1001', '1002', '1122', '1405', '1601')) OR
      (parent.code = '2000' AND child.code IN ('2202', '2211', '2221')) OR
      (parent.code = '4000' AND child.code IN ('4001', '4103', '4104')) OR
      (parent.code = '6000' AND child.code IN ('6001', '6051')) OR
      (parent.code = '6400' AND child.code IN ('6401', '6601', '6602', '6603', '6801'))
  )
  AND child.parent_id IS NULL;

INSERT INTO erp.gl_account_mappings (tenant_id, key, account_code)
SELECT t.id, mapping.key, mapping.account_code
FROM erp.tenants t
CROSS JOIN (
    VALUES
        ('ar', '1122'),
        ('ap', '2202'),
        ('revenue', '6001'),
        ('inventory', '1405'),
        ('cogs', '6401'),
        ('bank', '1002'),
        ('cash', '1001')
) AS mapping(key, account_code)
ON CONFLICT DO NOTHING;

INSERT INTO erp.gl_periods (id, tenant_id, year, month, start_date, end_date, status)
SELECT gen_random_uuid(), t.id, y.year, m.month,
       make_date(y.year, m.month, 1),
       (make_date(y.year, m.month, 1) + interval '1 month - 1 day')::date,
       'open'
FROM erp.tenants t
CROSS JOIN (SELECT extract(year FROM current_date)::int AS year) y
CROSS JOIN generate_series(1, 12) AS m(month)
ON CONFLICT (tenant_id, year, month) DO NOTHING;
