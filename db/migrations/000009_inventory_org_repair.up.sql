-- Repair Stage 3A inventory rows whose org was derived from the acting user
-- instead of the warehouse owner. Inventory belongs to the warehouse org.
UPDATE erp.inventory_balances b
SET org_id = w.org_id
FROM erp.warehouses w
WHERE w.tenant_id = b.tenant_id
  AND w.id = b.warehouse_id
  AND b.org_id <> w.org_id;

UPDATE erp.inventory_transactions t
SET org_id = w.org_id
FROM erp.warehouses w
WHERE w.tenant_id = t.tenant_id
  AND w.id = t.warehouse_id
  AND t.org_id <> w.org_id;
