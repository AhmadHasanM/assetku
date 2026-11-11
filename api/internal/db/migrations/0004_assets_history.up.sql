CREATE TABLE assets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    name text NOT NULL,
    asset_tag text NOT NULL,
    category text NOT NULL CHECK (category IN ('it_equipment', 'vehicle', 'building_facility', 'furniture', 'electronics', 'other')),
    manufacturer text NOT NULL DEFAULT '',
    model text NOT NULL DEFAULT '',
    serial_number text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'available' CHECK (status IN ('available', 'in_use', 'maintenance', 'damaged', 'lost', 'archived')),
    location_id uuid REFERENCES locations(id) ON DELETE RESTRICT,
    assigned_to text NOT NULL DEFAULT '',
    organization_id uuid REFERENCES organizations(id) ON DELETE RESTRICT,
    supplier_id uuid REFERENCES suppliers(id) ON DELETE RESTRICT,
    order_number text NOT NULL DEFAULT '',
    purchase_date date,
    purchase_cost numeric(16,2) CHECK (purchase_cost >= 0),
    warranty_months int CHECK (warranty_months >= 0),
    notes text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX assets_tenant_tag_key ON assets (tenant_id, lower(asset_tag));
CREATE INDEX assets_tenant_status_idx ON assets (tenant_id, status);
CREATE INDEX assets_tenant_category_idx ON assets (tenant_id, category);
CREATE INDEX assets_search_trgm_idx ON assets USING gin
    (lower(asset_tag || ' ' || name || ' ' || serial_number || ' ' || model || ' ' || assigned_to) gin_trgm_ops);
-- Back the ON DELETE RESTRICT checks and master data asset counts.
CREATE INDEX assets_location_idx ON assets (location_id) WHERE location_id IS NOT NULL;
CREATE INDEX assets_organization_idx ON assets (organization_id) WHERE organization_id IS NOT NULL;
CREATE INDEX assets_supplier_idx ON assets (supplier_id) WHERE supplier_id IS NOT NULL;

CREATE TABLE asset_history (
    id bigserial PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    asset_id uuid NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    event text NOT NULL,
    actor_id uuid,
    actor_name text NOT NULL,
    detail jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX asset_history_asset_idx ON asset_history (asset_id, id DESC);
