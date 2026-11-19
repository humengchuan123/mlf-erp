-- Rollback Stage 3J. Refuse when refund data exists because dropping the
-- tables would leave credit note/GL state inconsistent.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM erp.finance_credit_notes
        WHERE refunded_amount > 0 OR refunded_base_amount > 0
    ) THEN
        RAISE EXCEPTION 'cannot roll back 000022_credit_note_usage while credit note refunds exist; reverse refunds first';
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
