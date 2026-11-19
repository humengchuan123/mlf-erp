-- Stage 3J: credit notes with no original-currency remainder are applied even
-- when a base-currency residual remains. Repair notes left open by the old
-- tolerance-based status rule.

UPDATE erp.finance_credit_notes
SET status = 'applied', updated_at = now()
WHERE status = 'open' AND remaining_amount = 0;
