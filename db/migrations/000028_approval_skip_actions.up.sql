-- Stage 4B: record approval steps skipped because the requester cannot approve
-- their own step.

ALTER TABLE erp.approval_actions DROP CONSTRAINT IF EXISTS approval_actions_action_check;
ALTER TABLE erp.approval_actions
    ADD CONSTRAINT approval_actions_action_check
    CHECK (action IN ('approve', 'reject', 'cancel', 'skip'));
CREATE UNIQUE INDEX IF NOT EXISTS approval_actions_skip_idx
    ON erp.approval_actions (tenant_id, request_id, step_no)
    WHERE action = 'skip';
