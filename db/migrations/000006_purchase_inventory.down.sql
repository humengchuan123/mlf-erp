DROP TABLE IF EXISTS erp.inventory_transactions;
DROP TABLE IF EXISTS erp.inventory_balances;
DROP TABLE IF EXISTS erp.purchase_receipt_lines;
DROP TABLE IF EXISTS erp.purchase_receipts;
DROP TABLE IF EXISTS erp.purchase_order_lines;
DROP TABLE IF EXISTS erp.purchase_orders;

DELETE FROM erp.role_permissions WHERE permission_code IN (
    'purchase.order.read', 'purchase.order.write', 'purchase.order.approve',
    'purchase.receipt.read', 'inventory.balance.read', 'inventory.transaction.read'
);

DELETE FROM erp.permissions WHERE code IN (
    'purchase.order.read', 'purchase.order.write', 'purchase.order.approve',
    'purchase.receipt.read', 'inventory.balance.read', 'inventory.transaction.read'
);

DELETE FROM erp.code_rules WHERE entity = 'purchase_receipt';
