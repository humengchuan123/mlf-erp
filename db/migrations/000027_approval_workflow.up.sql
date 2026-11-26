-- Stage 4B: configurable approval workflow for sales and purchase orders.

CREATE TABLE erp.approval_flows (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid,
    document_type text NOT NULL,
    name text NOT NULL,
    condition_type text NOT NULL DEFAULT 'always',
    threshold_amount numeric(19,4) NOT NULL DEFAULT 0,
    priority integer NOT NULL DEFAULT 0,
    active boolean NOT NULL DEFAULT true,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT approval_flows_document_type_check
        CHECK (document_type IN ('sales_order', 'purchase_order')),
    CONSTRAINT approval_flows_condition_type_check
        CHECK (condition_type IN ('always', 'amount_gte')),
    CONSTRAINT approval_flows_threshold_check CHECK (threshold_amount >= 0),
    CONSTRAINT approval_flows_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE INDEX approval_flows_match_idx
    ON erp.approval_flows (tenant_id, document_type, active, priority DESC);
CREATE INDEX approval_flows_org_idx ON erp.approval_flows (tenant_id, org_id);

CREATE TABLE erp.approval_flow_steps (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    flow_id uuid NOT NULL,
    step_no integer NOT NULL CHECK (step_no > 0),
    name text NOT NULL,
    approver_type text NOT NULL,
    approver_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT approval_flow_steps_approver_type_check
        CHECK (approver_type IN ('role', 'user')),
    CONSTRAINT approval_flow_steps_flow_fk
        FOREIGN KEY (tenant_id, flow_id)
        REFERENCES erp.approval_flows (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT approval_flow_steps_unique UNIQUE (tenant_id, flow_id, step_no)
);

CREATE TABLE erp.approval_requests (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    document_type text NOT NULL,
    document_id uuid NOT NULL,
    document_code text NOT NULL,
    flow_id uuid NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    current_step integer NOT NULL DEFAULT 1,
    requested_by uuid NOT NULL,
    requested_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    amount numeric(19,4) NOT NULL,
    currency text NOT NULL,
    base_amount numeric(19,4) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT approval_requests_document_type_check
        CHECK (document_type IN ('sales_order', 'purchase_order')),
    CONSTRAINT approval_requests_status_check
        CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
    CONSTRAINT approval_requests_flow_fk
        FOREIGN KEY (tenant_id, flow_id)
        REFERENCES erp.approval_flows (tenant_id, id),
    CONSTRAINT approval_requests_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE UNIQUE INDEX approval_requests_pending_document_idx
    ON erp.approval_requests (tenant_id, document_type, document_id)
    WHERE status = 'pending';
CREATE INDEX approval_requests_pending_idx
    ON erp.approval_requests (tenant_id, status, current_step);
CREATE INDEX approval_requests_document_idx
    ON erp.approval_requests (tenant_id, document_type, document_id, requested_at DESC);

CREATE TABLE erp.approval_actions (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    request_id uuid NOT NULL,
    step_no integer NOT NULL CHECK (step_no > 0),
    action text NOT NULL,
    actor_id uuid NOT NULL,
    comment text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT approval_actions_action_check
        CHECK (action IN ('approve', 'reject', 'cancel')),
    CONSTRAINT approval_actions_request_fk
        FOREIGN KEY (tenant_id, request_id)
        REFERENCES erp.approval_requests (tenant_id, id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX approval_actions_step_idx
    ON erp.approval_actions (tenant_id, request_id, step_no)
    WHERE action IN ('approve', 'reject');
CREATE INDEX approval_actions_request_idx
    ON erp.approval_actions (tenant_id, request_id, created_at);

INSERT INTO erp.permissions (code, name, module, type) VALUES
    ('system.approval.read', '查看审批', 'system', 'api'),
    ('system.approval.write', '管理审批流程', 'system', 'api'),
    ('system.approval.act', '审批动作', 'system', 'api')
ON CONFLICT (code) DO NOTHING;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, p.code
FROM erp.roles r
JOIN erp.permissions p
  ON p.code IN ('system.approval.read', 'system.approval.write', 'system.approval.act')
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;
