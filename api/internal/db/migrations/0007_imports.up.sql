CREATE TABLE imports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    file_name text NOT NULL,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'done', 'failed')),
    total_rows int NOT NULL DEFAULT 0,
    processed_rows int NOT NULL DEFAULT 0,
    result jsonb,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);
CREATE INDEX imports_tenant_idx ON imports (tenant_id, created_at DESC);
-- Backs the startup sweep of interrupted jobs.
CREATE INDEX imports_active_idx ON imports (status) WHERE status IN ('queued', 'running');
