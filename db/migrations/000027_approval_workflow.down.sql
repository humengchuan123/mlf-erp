DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM erp.approval_requests) OR EXISTS (SELECT 1 FROM erp.approval_actions) THEN
        RAISE EXCEPTION 'cannot rollback approval workflow while approval data exists';
    END IF;
END $$;

DROP TABLE IF EXISTS erp.approval_actions;
DROP TABLE IF EXISTS erp.approval_requests;
DROP TABLE IF EXISTS erp.approval_flow_steps;
DROP TABLE IF EXISTS erp.approval_flows;

DELETE FROM erp.role_permissions
WHERE permission_code IN ('system.approval.read', 'system.approval.write', 'system.approval.act');

DELETE FROM erp.permissions
WHERE code IN ('system.approval.read', 'system.approval.write', 'system.approval.act');
