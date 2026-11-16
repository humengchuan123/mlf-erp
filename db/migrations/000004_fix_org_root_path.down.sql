UPDATE erp.organizations
SET path = '/'
WHERE parent_id IS NULL AND path = '/' || id::text;
