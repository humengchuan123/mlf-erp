DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM erp.approval_actions WHERE action = 'skip') THEN
        RAISE EXCEPTION 'cannot rollback approval skip actions while skip actions exist';
    END IF;
END $$;
DROP INDEX IF EXISTS erp.approval_actions_skip_idx;
ALTER TABLE erp.approval_actions DROP CONSTRAINT IF EXISTS approval_actions_action_check;
ALTER TABLE erp.approval_actions
    ADD CONSTRAINT approval_actions_action_check
    CHECK (action IN ('approve', 'reject', 'cancel'));
