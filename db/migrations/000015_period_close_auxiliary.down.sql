DROP INDEX IF EXISTS erp.gl_voucher_lines_project_idx;
DROP INDEX IF EXISTS erp.gl_year_closes_year_idx;
DROP TABLE IF EXISTS erp.gl_year_closes;
DROP INDEX IF EXISTS erp.gl_period_closes_period_idx;
DROP TABLE IF EXISTS erp.gl_period_closes;
ALTER TABLE erp.gl_voucher_lines DROP COLUMN IF EXISTS project_code;
-- Keep gl_account_mappings rows: 000015 uses ON CONFLICT DO NOTHING and may
-- have found profit/retained_earnings mappings already present before upgrade.
