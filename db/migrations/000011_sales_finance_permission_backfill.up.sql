-- Preserve access for roles that held the old umbrella permissions.
WITH mapping(old_code, new_code) AS (
    VALUES
        ('sales.read', 'sales.order.read'),
        ('sales.read', 'sales.shipment.read'),
        ('sales.write', 'sales.order.write'),
        ('finance.read', 'finance.invoice.read'),
        ('finance.read', 'finance.payment.read'),
        ('finance.write', 'finance.payment.write')
)
INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT rp.tenant_id, rp.role_id, mapping.new_code
FROM erp.role_permissions rp
JOIN mapping ON mapping.old_code = rp.permission_code
JOIN erp.permissions p ON p.code = mapping.new_code
ON CONFLICT DO NOTHING;
