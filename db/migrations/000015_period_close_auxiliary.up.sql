ALTER TABLE erp.gl_voucher_lines
    ADD COLUMN IF NOT EXISTS project_code text NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS erp.gl_period_closes (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    period_id uuid NOT NULL,
    voucher_id uuid,
    reversal_voucher_id uuid,
    status text NOT NULL DEFAULT 'closed',
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    reopened_by uuid,
    reopened_at timestamptz,
    CONSTRAINT gl_period_closes_period_fk FOREIGN KEY (tenant_id, period_id)
        REFERENCES erp.gl_periods (tenant_id, id),
    CONSTRAINT gl_period_closes_voucher_fk FOREIGN KEY (tenant_id, voucher_id)
        REFERENCES erp.gl_vouchers (tenant_id, id),
    CONSTRAINT gl_period_closes_reversal_fk FOREIGN KEY (tenant_id, reversal_voucher_id)
        REFERENCES erp.gl_vouchers (tenant_id, id),
    CONSTRAINT gl_period_closes_status_check CHECK (status IN ('closed', 'reopened'))
);

CREATE INDEX IF NOT EXISTS gl_period_closes_period_idx
    ON erp.gl_period_closes (tenant_id, period_id, created_at DESC);

CREATE TABLE IF NOT EXISTS erp.gl_year_closes (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    year integer NOT NULL,
    voucher_id uuid,
    status text NOT NULL DEFAULT 'closed',
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT gl_year_closes_tenant_year_key UNIQUE (tenant_id, year),
    CONSTRAINT gl_year_closes_voucher_fk FOREIGN KEY (tenant_id, voucher_id)
        REFERENCES erp.gl_vouchers (tenant_id, id),
    CONSTRAINT gl_year_closes_status_check CHECK (status IN ('closed'))
);

CREATE INDEX IF NOT EXISTS gl_year_closes_year_idx
    ON erp.gl_year_closes (tenant_id, year);

CREATE INDEX IF NOT EXISTS gl_voucher_lines_project_idx
    ON erp.gl_voucher_lines (tenant_id, project_code);

INSERT INTO erp.gl_account_mappings (tenant_id, key, account_code)
SELECT t.id, mapping.key, mapping.account_code
FROM erp.tenants t
CROSS JOIN (
    VALUES
        ('profit', '4103'),
        ('retained_earnings', '4104')
) AS mapping(key, account_code)
ON CONFLICT DO NOTHING;
