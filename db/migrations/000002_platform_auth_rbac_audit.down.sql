DROP TABLE IF EXISTS erp.audit_logs;
DROP TABLE IF EXISTS erp.login_logs;
DROP TABLE IF EXISTS erp.refresh_tokens;
DROP TABLE IF EXISTS erp.role_permissions;
DROP TABLE IF EXISTS erp.permissions;

ALTER TABLE erp.roles DROP COLUMN IF EXISTS is_system;
ALTER TABLE erp.users DROP CONSTRAINT IF EXISTS users_primary_org_fk;
ALTER TABLE erp.users DROP COLUMN IF EXISTS primary_org_id;
ALTER TABLE erp.users DROP COLUMN IF EXISTS last_login_at;
ALTER TABLE erp.users DROP COLUMN IF EXISTS is_super_admin;
