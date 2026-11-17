DROP INDEX IF EXISTS erp.gl_period_closes_open_key;

ALTER TABLE erp.gl_voucher_lines
    DROP CONSTRAINT IF EXISTS gl_voucher_lines_project_code_len,
    DROP CONSTRAINT IF EXISTS gl_voucher_lines_department_fk,
    DROP CONSTRAINT IF EXISTS gl_voucher_lines_partner_fk;

ALTER TABLE erp.partners
    DROP CONSTRAINT IF EXISTS partners_tenant_id_key;

CREATE INDEX IF NOT EXISTS gl_year_closes_year_idx
    ON erp.gl_year_closes (tenant_id, year);
