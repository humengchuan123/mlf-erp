-- Stage 4A: idempotency keys for money and inventory POST operations.

CREATE TABLE IF NOT EXISTS erp.idempotency_keys (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    idempotency_key text NOT NULL,
    method text NOT NULL,
    path text NOT NULL,
    request_hash text NOT NULL,
    status text NOT NULL DEFAULT 'in_progress',
    response_code integer,
    response_body text,
    response_content_type text,
    response_truncated boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    CONSTRAINT idempotency_keys_status_check
        CHECK (status IN ('in_progress', 'completed', 'failed')),
    CONSTRAINT idempotency_keys_scope_key
        UNIQUE (tenant_id, user_id, idempotency_key, method, path)
);

CREATE INDEX IF NOT EXISTS idempotency_keys_expires_at_idx
    ON erp.idempotency_keys (expires_at);
