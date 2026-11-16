DROP TABLE IF EXISTS erp.materials;
DROP TABLE IF EXISTS erp.partners;
DROP TABLE IF EXISTS erp.warehouses;
DROP TABLE IF EXISTS erp.units;
DROP TABLE IF EXISTS erp.currencies;
DROP TABLE IF EXISTS erp.tax_rates;
DROP TABLE IF EXISTS erp.code_rules;

DELETE FROM erp.role_permissions WHERE permission_code IN (
    'masterdata.partner.read',
    'masterdata.partner.write',
    'masterdata.material.read',
    'masterdata.material.write',
    'masterdata.warehouse.read',
    'masterdata.warehouse.write',
    'masterdata.unit.read',
    'masterdata.unit.write',
    'masterdata.currency.read',
    'masterdata.currency.write',
    'masterdata.tax.read',
    'masterdata.tax.write',
    'system.code_rule.read',
    'system.code_rule.write'
);

DELETE FROM erp.permissions WHERE code IN (
    'masterdata.partner.read',
    'masterdata.partner.write',
    'masterdata.material.read',
    'masterdata.material.write',
    'masterdata.warehouse.read',
    'masterdata.warehouse.write',
    'masterdata.unit.read',
    'masterdata.unit.write',
    'masterdata.currency.read',
    'masterdata.currency.write',
    'masterdata.tax.read',
    'masterdata.tax.write',
    'system.code_rule.read',
    'system.code_rule.write'
);
