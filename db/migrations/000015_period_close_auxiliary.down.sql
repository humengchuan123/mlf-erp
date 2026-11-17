DROP INDEX IF EXISTS erp.gl_voucher_lines_project_idx;
DROP INDEX IF EXISTS erp.gl_year_closes_year_idx;
DROP TABLE IF EXISTS erp.gl_year_closes;
DROP INDEX IF EXISTS erp.gl_period_closes_period_idx;
DROP TABLE IF EXISTS erp.gl_period_closes;
ALTER TABLE erp.gl_voucher_lines DROP COLUMN IF EXISTS project_code;
DELETE FROM erp.gl_account_mappings WHERE key IN ('profit', 'retained_earnings');
