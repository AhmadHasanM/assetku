CREATE TABLE tenants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    name text NOT NULL,
    email text NOT NULL,
    position text NOT NULL DEFAULT '',
    password_hash text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    locale text NOT NULL DEFAULT 'id' CHECK (locale IN ('id', 'en')),
    theme text NOT NULL DEFAULT 'dark' CHECK (theme IN ('dark', 'light')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));
CREATE INDEX users_tenant_id_idx ON users (tenant_id);

CREATE TABLE super_admins (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    email text NOT NULL,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX super_admins_email_lower_key ON super_admins (lower(email));

CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY,
    subject_type text NOT NULL CHECK (subject_type IN ('user', 'super_admin')),
    subject_id uuid NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);
