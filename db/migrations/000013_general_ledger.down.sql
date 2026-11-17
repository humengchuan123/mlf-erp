DROP TABLE IF EXISTS erp.gl_voucher_lines;
DROP TABLE IF EXISTS erp.gl_vouchers;
DROP TABLE IF EXISTS erp.gl_account_mappings;
DROP TABLE IF EXISTS erp.gl_periods;
DROP TABLE IF EXISTS erp.gl_accounts;

DELETE FROM erp.role_permissions WHERE permission_code IN (
    'finance.account.read', 'finance.account.write', 'finance.period.read', 'finance.period.close',
    'finance.voucher.read', 'finance.voucher.write', 'finance.voucher.post', 'finance.voucher.reverse',
    'finance.report.read'
);

DELETE FROM erp.permissions WHERE code IN (
    'finance.account.read', 'finance.account.write', 'finance.period.read', 'finance.period.close',
    'finance.voucher.read', 'finance.voucher.write', 'finance.voucher.post', 'finance.voucher.reverse',
    'finance.report.read'
);

DELETE FROM erp.code_rules WHERE entity = 'gl_voucher';
