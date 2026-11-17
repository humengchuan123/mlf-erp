DROP TABLE IF EXISTS erp.finance_payment_allocations;
DROP TABLE IF EXISTS erp.finance_payments;
DROP TABLE IF EXISTS erp.finance_invoices;
DROP TABLE IF EXISTS erp.sales_shipment_lines;
DROP TABLE IF EXISTS erp.sales_shipments;
DROP TABLE IF EXISTS erp.sales_order_lines;
DROP TABLE IF EXISTS erp.sales_orders;

ALTER TABLE erp.inventory_balances
    DROP CONSTRAINT IF EXISTS inventory_balances_non_negative;

ALTER TABLE erp.inventory_balances
    DROP COLUMN IF EXISTS avg_cost;

DELETE FROM erp.role_permissions WHERE permission_code IN (
    'sales.order.read', 'sales.order.write', 'sales.order.approve', 'sales.shipment.read',
    'finance.invoice.read', 'finance.payment.read', 'finance.payment.write'
);

DELETE FROM erp.permissions WHERE code IN (
    'sales.order.read', 'sales.order.write', 'sales.order.approve', 'sales.shipment.read',
    'finance.invoice.read', 'finance.payment.read', 'finance.payment.write'
);

DELETE FROM erp.code_rules WHERE entity IN (
    'sales_order', 'sales_shipment', 'sales_receivable',
    'purchase_payable', 'finance_receipt', 'finance_payment'
);
