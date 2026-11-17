CREATE INDEX IF NOT EXISTS gl_vouchers_report_idx
    ON erp.gl_vouchers (tenant_id, status, voucher_date);

CREATE INDEX IF NOT EXISTS gl_vouchers_period_status_idx
    ON erp.gl_vouchers (tenant_id, period_id, status);

CREATE INDEX IF NOT EXISTS gl_vouchers_org_report_idx
    ON erp.gl_vouchers (tenant_id, org_id, status, voucher_date);
