-- Rollback Stage 3K. Refuse when reversal records or reversed FX settlements
-- exist because dropping the schema would lose reversal/GL audit state.

LOCK TABLE erp.finance_payment_allocation_reversals IN ACCESS EXCLUSIVE MODE;
LOCK TABLE erp.finance_fx_settlements IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM erp.finance_payment_allocation_reversals) THEN
        RAISE EXCEPTION 'cannot roll back 000024_payment_reversals while reversal records exist';
    END IF;
    IF EXISTS (
        SELECT 1 FROM erp.finance_fx_settlements
        WHERE status = 'reversed'
           OR reversal_id IS NOT NULL
           OR reversed_at IS NOT NULL
           OR reversed_by IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'cannot roll back 000024_payment_reversals while FX settlements are reversed';
    END IF;
END $$;

DELETE FROM erp.role_permissions WHERE permission_code = 'finance.payment.reverse';
DELETE FROM erp.permissions WHERE code = 'finance.payment.reverse';

ALTER TABLE erp.finance_fx_settlements
    DROP CONSTRAINT IF EXISTS finance_fx_settlements_reversal_state_check,
    DROP CONSTRAINT IF EXISTS finance_fx_settlements_status_check,
    DROP CONSTRAINT IF EXISTS finance_fx_settlements_reversal_fk,
    DROP CONSTRAINT IF EXISTS finance_fx_settlements_reversal_voucher_fk;
DROP INDEX IF EXISTS erp.finance_fx_settlements_reversal_idx;
DROP INDEX IF EXISTS erp.finance_fx_settlements_payment_invoice_status_idx;
DROP INDEX IF EXISTS erp.finance_fx_settlements_reversal_id_idx;
ALTER TABLE erp.finance_fx_settlements
    DROP COLUMN IF EXISTS reversed_by,
    DROP COLUMN IF EXISTS reversed_at,
    DROP COLUMN IF EXISTS reversal_voucher_id,
    DROP COLUMN IF EXISTS reversal_id,
    DROP COLUMN IF EXISTS status;

DROP TABLE IF EXISTS erp.finance_payment_allocation_reversals;
