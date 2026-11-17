-- Data backfill is intentionally not reversible; deleting legacy payables could
-- remove invoices that already have allocations.
SELECT 1;
