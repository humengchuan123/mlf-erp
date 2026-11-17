-- Permission backfill is intentionally not reversible; removing grants could
-- revoke access that was legitimately assigned after the migration.
SELECT 1;
