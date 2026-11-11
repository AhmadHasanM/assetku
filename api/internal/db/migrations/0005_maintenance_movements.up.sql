CREATE TABLE maintenance_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    asset_id uuid NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    date date NOT NULL,
    type text NOT NULL CHECK (type IN ('preventive', 'corrective', 'inspection', 'other')),
    performed_by text NOT NULL DEFAULT '',
    cost numeric(16,2) CHECK (cost >= 0),
    next_date date,
    reminder_days int CHECK (reminder_days >= 0),
    notes text NOT NULL DEFAULT '',
    reminder_sent_at timestamptz,
    created_by_name text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
-- Serves the per-asset list and the "latest record per asset" lookup.
CREATE INDEX maintenance_asset_idx ON maintenance_records (asset_id, date DESC, created_at DESC);
CREATE INDEX maintenance_tenant_idx ON maintenance_records (tenant_id);

CREATE TABLE movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    asset_id uuid NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    from_location_id uuid REFERENCES locations(id) ON DELETE SET NULL,
    from_location_name text NOT NULL DEFAULT '',
    to_location_id uuid NOT NULL REFERENCES locations(id) ON DELETE RESTRICT,
    date date NOT NULL,
    moved_by text NOT NULL,
    reason text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX movements_asset_idx ON movements (asset_id, date DESC, created_at DESC);
CREATE INDEX movements_tenant_idx ON movements (tenant_id);
CREATE INDEX movements_to_location_idx ON movements (to_location_id);
CREATE INDEX movements_from_location_idx ON movements (from_location_id) WHERE from_location_id IS NOT NULL;
