CREATE TABLE notifications (
    id bigserial PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type text NOT NULL,
    title_id text NOT NULL,
    title_en text NOT NULL,
    body_id text NOT NULL,
    body_en text NOT NULL,
    asset_id uuid REFERENCES assets(id) ON DELETE CASCADE,
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications (user_id, id DESC);
CREATE INDEX notifications_asset_idx ON notifications (asset_id) WHERE asset_id IS NOT NULL;
