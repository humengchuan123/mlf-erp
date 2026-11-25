-- Stage 3K: repair payment base allocated amounts so they equal the sum of
-- active allocation base amounts. Older allocations rounded the payment total
-- once instead of summing per-allocation rounded base amounts, which made
-- exact reversal impossible.

UPDATE erp.finance_payments p
SET base_allocated_amount = COALESCE((
    SELECT SUM(a.base_amount)
    FROM erp.finance_payment_allocations a
    WHERE a.tenant_id = p.tenant_id AND a.payment_id = p.id
), 0)
WHERE p.base_allocated_amount <> COALESCE((
    SELECT SUM(a.base_amount)
    FROM erp.finance_payment_allocations a
    WHERE a.tenant_id = p.tenant_id AND a.payment_id = p.id
), 0);
