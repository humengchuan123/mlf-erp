CREATE SCHEMA IF NOT EXISTS erp;

CREATE TABLE IF NOT EXISTS erp.tenants (
    id uuid PRIMARY KEY,
    code text NOT NULL UNIQUE,
    name text NOT NULL,
    edition text NOT NULL DEFAULT 'standard',
    status text NOT NULL DEFAULT 'active',
    isolation_mode text NOT NULL DEFAULT 'shared',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS erp.organizations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    parent_id uuid,
    type text NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    path text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT organizations_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT organizations_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT organizations_parent_fk FOREIGN KEY (tenant_id, parent_id)
        REFERENCES erp.organizations (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.users (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    username text NOT NULL,
    display_name text NOT NULL,
    email text,
    password_hash text NOT NULL,
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_tenant_username_key UNIQUE (tenant_id, username),
    CONSTRAINT users_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.roles (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    code text NOT NULL,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT roles_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT roles_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.user_roles (
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    user_id uuid NOT NULL,
    role_id uuid NOT NULL,
    PRIMARY KEY (tenant_id, user_id, role_id),
    CONSTRAINT user_roles_user_fk FOREIGN KEY (tenant_id, user_id)
        REFERENCES erp.users (tenant_id, id),
    CONSTRAINT user_roles_role_fk FOREIGN KEY (tenant_id, role_id)
        REFERENCES erp.roles (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.org_memberships (
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    user_id uuid NOT NULL,
    org_id uuid NOT NULL,
    role_id uuid,
    data_scope text NOT NULL DEFAULT 'self',
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id, org_id),
    CONSTRAINT org_memberships_user_fk FOREIGN KEY (tenant_id, user_id)
        REFERENCES erp.users (tenant_id, id),
    CONSTRAINT org_memberships_org_fk FOREIGN KEY (tenant_id, org_id)
        REFERENCES erp.organizations (tenant_id, id),
    CONSTRAINT org_memberships_role_fk FOREIGN KEY (tenant_id, role_id)
        REFERENCES erp.roles (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.outbox_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    aggregate_type text NOT NULL,
    aggregate_id text NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    available_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS outbox_events_pending_idx
    ON erp.outbox_events (tenant_id, status, available_at);

INSERT INTO erp.tenants (id, code, name)
VALUES ('00000000-0000-0000-0000-000000000001', 'default', 'Default Tenant')
ON CONFLICT (id) DO NOTHING;

INSERT INTO erp.organizations (id, tenant_id, parent_id, type, code, name, path)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    '00000000-0000-0000-0000-000000000001',
    NULL,
    'company',
    'HQ',
    'Default Company',
    '/'
)
ON CONFLICT (id) DO NOTHING;
