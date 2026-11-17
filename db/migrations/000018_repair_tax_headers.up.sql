-- Repair Stage 3E historical shipment/receipt headers. 000017 added
-- total_amount/net_amount/tax_amount with default 0 and could not backfill
-- these two headers because the source total column did not exist before it.
UPDATE erp.sales_shipments s
SET net_amount = x.net_amount,
    tax_amount = x.tax_amount,
    total_amount = x.gross_amount
FROM (
    SELECT tenant_id, shipment_id,
           COALESCE(SUM(amount), 0)::numeric(19,4) AS net_amount,
           COALESCE(SUM(tax_amount), 0)::numeric(19,4) AS tax_amount,
           COALESCE(SUM(gross_amount), 0)::numeric(19,4) AS gross_amount
    FROM erp.sales_shipment_lines
    GROUP BY tenant_id, shipment_id
) x
WHERE s.tenant_id = x.tenant_id
  AND s.id = x.shipment_id
  AND s.total_amount = 0
  AND x.gross_amount <> 0;

UPDATE erp.purchase_receipts r
SET net_amount = x.net_amount,
    tax_amount = x.tax_amount,
    total_amount = x.gross_amount
FROM (
    SELECT tenant_id, receipt_id,
           COALESCE(SUM(amount), 0)::numeric(19,4) AS net_amount,
           COALESCE(SUM(tax_amount), 0)::numeric(19,4) AS tax_amount,
           COALESCE(SUM(gross_amount), 0)::numeric(19,4) AS gross_amount
    FROM erp.purchase_receipt_lines
    GROUP BY tenant_id, receipt_id
) x
WHERE r.tenant_id = x.tenant_id
  AND r.id = x.receipt_id
  AND r.total_amount = 0
  AND x.gross_amount <> 0;
