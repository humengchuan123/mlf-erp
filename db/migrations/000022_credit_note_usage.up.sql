-- Stage 3J: credit note allocations, refunds and usage balances.

ALTER TABLE erp.finance_credit_notes
    ADD COLUMN IF NOT EXISTS refunded_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS refunded_base_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.finance_credit_notes
    DROP CONSTRAINT IF EXISTS finance_credit_notes_amount_check;
ALTER TABLE erp.finance_credit_notes
    ADD CONSTRAINT finance_credit_notes_amount_check CHECK (
        amount > 0 AND base_amount > 0
        AND applied_amount >= 0 AND applied_base_amount >= 0
        AND refunded_amount >= 0 AND refunded_base_amount >= 0
        AND remaining_amount >= 0 AND remaining_base_amount >= 0
        AND applied_amount + refunded_amount + remaining_amount = amount
        AND applied_base_amount + refunded_base_amount + remaining_base_amount = base_amount
        AND (applied_amount > 0 OR applied_base_amount = 0)
        AND (refunded_amount > 0 OR refunded_base_amount = 0)
    );

CREATE TABLE IF NOT EXISTS erp.finance_credit_note_allocations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    credit_note_id uuid NOT NULL,
    invoice_id uuid NOT NULL,
    amount numeric(19,4) NOT NULL CHECK (amount > 0),
    base_amount numeric(19,4) NOT NULL DEFAULT 0 CHECK (base_amount >= 0),
    allocation_date date NOT NULL,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_credit_note_allocations_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT finance_credit_note_allocations_note_fk
        FOREIGN KEY (tenant_id, credit_note_id)
        REFERENCES erp.finance_credit_notes (tenant_id, id),
    CONSTRAINT finance_credit_note_allocations_invoice_fk
        FOREIGN KEY (tenant_id, invoice_id)
        REFERENCES erp.finance_invoices (tenant_id, id)
);

CREATE INDEX IF NOT EXISTS finance_credit_note_allocations_note_idx
    ON erp.finance_credit_note_allocations (tenant_id, credit_note_id, allocation_date);
CREATE INDEX IF NOT EXISTS finance_credit_note_allocations_invoice_idx
    ON erp.finance_credit_note_allocations (tenant_id, invoice_id);

CREATE TABLE IF NOT EXISTS erp.finance_credit_note_refunds (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    credit_note_id uuid NOT NULL,
    refund_date date NOT NULL,
    status text NOT NULL DEFAULT 'posted',
    currency text NOT NULL,
    exchange_rate numeric(19,8) NOT NULL CHECK (exchange_rate > 0),
    amount numeric(19,4) NOT NULL CHECK (amount > 0),
    base_amount numeric(19,4) NOT NULL CHECK (base_amount >= 0),
    cash_base_amount numeric(19,4) NOT NULL CHECK (cash_base_amount > 0),
    fx_adjustment numeric(19,4) NOT NULL DEFAULT 0,
    method text NOT NULL DEFAULT 'bank',
    reference text NOT NULL DEFAULT '',
    remark text NOT NULL DEFAULT '',
    voucher_code text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_credit_note_refunds_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT finance_credit_note_refunds_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT finance_credit_note_refunds_note_fk
        FOREIGN KEY (tenant_id, credit_note_id)
        REFERENCES erp.finance_credit_notes (tenant_id, id),
    CONSTRAINT finance_credit_note_refunds_status_check
        CHECK (status IN ('posted')),
    CONSTRAINT finance_credit_note_refunds_fx_check
        CHECK (cash_base_amount - base_amount = fx_adjustment)
);

CREATE INDEX IF NOT EXISTS finance_credit_note_refunds_note_idx
    ON erp.finance_credit_note_refunds (tenant_id, credit_note_id, refund_date DESC);
CREATE INDEX IF NOT EXISTS finance_credit_note_refunds_list_idx
    ON erp.finance_credit_note_refunds (tenant_id, org_id, refund_date DESC);

INSERT INTO erp.finance_credit_note_allocations (
    id, tenant_id, org_id, credit_note_id, invoice_id,
    amount, base_amount, allocation_date, created_by, created_at
)
SELECT gen_random_uuid(), c.tenant_id, c.org_id, c.id, c.invoice_id,
       c.applied_amount, c.applied_base_amount, c.note_date, c.created_by, c.created_at
FROM erp.finance_credit_notes c
WHERE c.applied_amount > 0
  AND NOT EXISTS (
      SELECT 1 FROM erp.finance_credit_note_allocations a
      WHERE a.tenant_id = c.tenant_id AND a.credit_note_id = c.id
  );

INSERT INTO erp.code_rules (
    id, tenant_id, entity, prefix, date_pattern, separator, sequence_length, reset_policy, status
)
SELECT gen_random_uuid(), t.id, 'credit_note_refund', 'RF', 'yyyyMMdd', '-', 4, 'day', 'active'
FROM erp.tenants t
ON CONFLICT (tenant_id, entity) DO NOTHING;

INSERT INTO erp.permissions (code, name, module, type)
VALUES
    ('finance.credit_note.apply', '贷项单抵用', 'finance', 'api'),
    ('finance.credit_note.refund', '贷项单退款', 'finance', 'api')
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name, module = EXCLUDED.module, type = EXCLUDED.type;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, p.code
FROM erp.roles r
CROSS JOIN (VALUES ('finance.credit_note.apply'), ('finance.credit_note.refund')) AS p(code)
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;
