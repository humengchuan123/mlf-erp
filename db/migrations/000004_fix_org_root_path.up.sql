UPDATE erp.organizations
SET path = '/' || id::text
WHERE path = '/';
