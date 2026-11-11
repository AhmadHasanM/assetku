CREATE TABLE locations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX locations_tenant_name_key ON locations (tenant_id, lower(btrim(name)));

CREATE TABLE organizations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX organizations_tenant_name_key ON organizations (tenant_id, lower(btrim(name)));

CREATE TABLE suppliers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    name text NOT NULL,
    contact_name text NOT NULL DEFAULT '',
    phone text NOT NULL DEFAULT '',
    email text NOT NULL DEFAULT '',
    address text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX suppliers_tenant_name_key ON suppliers (tenant_id, lower(btrim(name)));
