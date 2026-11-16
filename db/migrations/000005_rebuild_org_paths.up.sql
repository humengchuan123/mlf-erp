WITH RECURSIVE org_paths AS (
    SELECT id, tenant_id, parent_id, '/' || id::text AS new_path
    FROM erp.organizations
    WHERE parent_id IS NULL

    UNION ALL

    SELECT o.id, o.tenant_id, o.parent_id, p.new_path || '/' || o.id::text
    FROM erp.organizations o
    JOIN org_paths p ON o.tenant_id = p.tenant_id AND o.parent_id = p.id
)
UPDATE erp.organizations o
SET path = p.new_path
FROM org_paths p
WHERE o.id = p.id;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, p.code
FROM erp.roles r
CROSS JOIN erp.permissions p
WHERE r.code = 'admin'
  AND (p.code LIKE 'masterdata.%' OR p.code LIKE 'system.code_rule.%')
ON CONFLICT DO NOTHING;
