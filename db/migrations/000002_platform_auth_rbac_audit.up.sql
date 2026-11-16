ALTER TABLE erp.users ADD COLUMN IF NOT EXISTS is_super_admin boolean NOT NULL DEFAULT false;
ALTER TABLE erp.users ADD COLUMN IF NOT EXISTS last_login_at timestamptz;
ALTER TABLE erp.users ADD COLUMN IF NOT EXISTS primary_org_id uuid;
ALTER TABLE erp.roles ADD COLUMN IF NOT EXISTS is_system boolean NOT NULL DEFAULT false;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'users_primary_org_fk'
    ) THEN
        ALTER TABLE erp.users
            ADD CONSTRAINT users_primary_org_fk
            FOREIGN KEY (tenant_id, primary_org_id)
            REFERENCES erp.organizations (tenant_id, id);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS erp.permissions (
    code text PRIMARY KEY,
    name text NOT NULL,
    module text NOT NULL,
    type text NOT NULL DEFAULT 'api',
    description text NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS erp.role_permissions (
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    role_id uuid NOT NULL,
    permission_code text NOT NULL REFERENCES erp.permissions (code),
    PRIMARY KEY (tenant_id, role_id, permission_code),
    CONSTRAINT role_permissions_role_fk FOREIGN KEY (tenant_id, role_id)
        REFERENCES erp.roles (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS erp.refresh_tokens (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    user_id uuid NOT NULL,
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    user_agent text NOT NULL DEFAULT '',
    ip text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT refresh_tokens_user_fk FOREIGN KEY (tenant_id, user_id)
        REFERENCES erp.users (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS erp.login_logs (
    id uuid PRIMARY KEY,
    tenant_id uuid,
    user_id uuid,
    username text NOT NULL,
    status text NOT NULL,
    failure_reason text NOT NULL DEFAULT '',
    ip text NOT NULL DEFAULT '',
    user_agent text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS erp.audit_logs (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid,
    user_id uuid,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text NOT NULL DEFAULT '',
    request_id text NOT NULL DEFAULT '',
    ip text NOT NULL DEFAULT '',
    user_agent text NOT NULL DEFAULT '',
    before_data jsonb,
    after_data jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS refresh_tokens_user_idx
    ON erp.refresh_tokens (tenant_id, user_id, expires_at);

CREATE INDEX IF NOT EXISTS audit_logs_tenant_created_idx
    ON erp.audit_logs (tenant_id, created_at DESC);

INSERT INTO erp.permissions (code, name, module, type) VALUES
('system.user.read', '查看用户', 'system', 'api'),
('system.user.write', '管理用户', 'system', 'api'),
('system.role.read', '查看角色', 'system', 'api'),
('system.role.write', '管理角色', 'system', 'api'),
('system.org.read', '查看组织', 'system', 'api'),
('system.org.write', '管理组织', 'system', 'api'),
('system.audit.read', '查看审计日志', 'system', 'api'),
('purchase.read', '查看采购', 'purchase', 'api'),
('purchase.write', '管理采购', 'purchase', 'api'),
('sales.read', '查看销售', 'sales', 'api'),
('sales.write', '管理销售', 'sales', 'api'),
('inventory.read', '查看库存', 'inventory', 'api'),
('inventory.write', '管理库存', 'inventory', 'api'),
('finance.read', '查看财务', 'finance', 'api'),
('finance.write', '管理财务', 'finance', 'api')
ON CONFLICT (code) DO NOTHING;
