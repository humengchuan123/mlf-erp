CREATE TABLE IF NOT EXISTS erp.finance_payment_allocation_reversals (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    payment_id uuid NOT NULL,
    invoice_id uuid NOT NULL,
    allocation_id uuid NOT NULL,
    amount numeric(19,4) NOT NULL CHECK (amount > 0),
    base_amount numeric(19,4) NOT NULL CHECK (base_amount >= 0),
    invoice_base_amount numeric(19,4) NOT NULL CHECK (invoice_base_amount >= 0),
    reversal_date date NOT NULL,
    reason text NOT NULL,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_payment_allocation_reversals_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT finance_payment_allocation_reversals_payment_fk
        FOREIGN KEY (tenant_id, payment_id)
        REFERENCES erp.finance_payments (tenant_id, id),
    CONSTRAINT finance_payment_allocation_reversals_invoice_fk
        FOREIGN KEY (tenant_id, invoice_id)
        REFERENCES erp.finance_invoices (tenant_id, id)
);

CREATE INDEX IF NOT EXISTS finance_payment_allocation_reversals_payment_idx
    ON erp.finance_payment_allocation_reversals (tenant_id, payment_id, reversal_date DESC);
CREATE INDEX IF NOT EXISTS finance_payment_allocation_reversals_invoice_idx
    ON erp.finance_payment_allocation_reversals (tenant_id, invoice_id);

ALTER TABLE erp.finance_fx_settlements
    ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'posted',
    ADD COLUMN IF NOT EXISTS reversal_id uuid,
    ADD COLUMN IF NOT EXISTS reversal_voucher_id uuid,
    ADD COLUMN IF NOT EXISTS reversed_at timestamptz,
    ADD COLUMN IF NOT EXISTS reversed_by uuid;

ALTER TABLE erp.finance_fx_settlements
    ADD CONSTRAINT finance_fx_settlements_status_check
        CHECK (status IN ('posted', 'reversed')),
    ADD CONSTRAINT finance_fx_settlements_reversal_fk
        FOREIGN KEY (tenant_id, reversal_id)
        REFERENCES erp.finance_payment_allocation_reversals (tenant_id, id),
    ADD CONSTRAINT finance_fx_settlements_reversal_voucher_fk
        FOREIGN KEY (tenant_id, reversal_voucher_id)
        REFERENCES erp.gl_vouchers (tenant_id, id);

CREATE INDEX IF NOT EXISTS finance_fx_settlements_reversal_idx
    ON erp.finance_fx_settlements (tenant_id, payment_id, invoice_id, status);

INSERT INTO erp.permissions (code, name, module, type)
VALUES ('finance.payment.reverse', '收付款反核销', 'finance', 'api')
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name, module = EXCLUDED.module, type = EXCLUDED.type;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, 'finance.payment.reverse'
FROM erp.roles r
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;
