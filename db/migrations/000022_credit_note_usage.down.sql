-- Rollback Stage 3J. Refuse when refunds or non-initial allocations exist
-- because dropping the tables would lose credit note/GL state.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM erp.finance_credit_note_refunds) THEN
        RAISE EXCEPTION 'cannot roll back 000022_credit_note_usage while credit note refunds exist; reverse refunds first';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM erp.finance_credit_notes c
        JOIN erp.finance_credit_note_allocations a
          ON a.tenant_id = c.tenant_id AND a.credit_note_id = c.id
        GROUP BY c.tenant_id, c.id, c.invoice_id, c.applied_amount, c.applied_base_amount
        HAVING count(*) > 1
            OR bool_or(a.invoice_id <> c.invoice_id)
            OR bool_or(a.amount <> c.applied_amount)
            OR bool_or(a.base_amount <> c.applied_base_amount)
    ) THEN
        RAISE EXCEPTION 'cannot roll back 000022_credit_note_usage while non-initial credit note allocations exist; reverse allocations first';
    END IF;
    IF EXISTS (
        SELECT 1 FROM erp.finance_credit_notes
        WHERE refunded_amount > 0 OR refunded_base_amount > 0
    ) THEN
        RAISE EXCEPTION 'cannot roll back 000022_credit_note_usage while credit note refunded balances exist; reverse refunds first';
    END IF;
END $$;

DELETE FROM erp.role_permissions WHERE permission_code IN ('finance.credit_note.apply', 'finance.credit_note.refund');
DELETE FROM erp.permissions WHERE code IN ('finance.credit_note.apply', 'finance.credit_note.refund');
DELETE FROM erp.code_rules WHERE entity = 'credit_note_refund';

DROP TABLE IF EXISTS erp.finance_credit_note_refunds;
DROP TABLE IF EXISTS erp.finance_credit_note_allocations;

ALTER TABLE erp.finance_credit_notes
    DROP CONSTRAINT IF EXISTS finance_credit_notes_amount_check;
ALTER TABLE erp.finance_credit_notes
    ADD CONSTRAINT finance_credit_notes_amount_check CHECK (
        amount > 0 AND base_amount > 0
        AND applied_amount >= 0 AND applied_base_amount >= 0
        AND remaining_amount >= 0 AND remaining_base_amount >= 0
        AND applied_amount + remaining_amount = amount
        AND applied_base_amount + remaining_base_amount = base_amount
    );

ALTER TABLE erp.finance_credit_notes
    DROP COLUMN IF EXISTS refunded_base_amount,
    DROP COLUMN IF EXISTS refunded_amount;
