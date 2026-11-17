-- Guard Stage 3D close records and auxiliary references.
ALTER TABLE erp.partners
    ADD CONSTRAINT partners_tenant_id_key UNIQUE (tenant_id, id);

ALTER TABLE erp.gl_voucher_lines
    ADD CONSTRAINT gl_voucher_lines_partner_fk
        FOREIGN KEY (tenant_id, partner_id)
        REFERENCES erp.partners (tenant_id, id) NOT VALID,
    ADD CONSTRAINT gl_voucher_lines_department_fk
        FOREIGN KEY (tenant_id, department_id)
        REFERENCES erp.organizations (tenant_id, id) NOT VALID,
    ADD CONSTRAINT gl_voucher_lines_project_code_len
        CHECK (char_length(project_code) <= 64) NOT VALID;

CREATE UNIQUE INDEX IF NOT EXISTS gl_period_closes_open_key
    ON erp.gl_period_closes (tenant_id, period_id)
    WHERE status = 'closed';

DROP INDEX IF EXISTS erp.gl_year_closes_year_idx;
