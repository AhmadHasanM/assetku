-- name: GetSuperAdminForLogin :one
SELECT id, name, email, password_hash FROM super_admins WHERE lower(email) = lower(@email::text);

-- name: GetSuperAdmin :one
SELECT id, name, email FROM super_admins WHERE id = $1;

-- name: UpsertSuperAdmin :one
INSERT INTO super_admins (name, email, password_hash)
VALUES (@name, @email, @password_hash)
ON CONFLICT (lower(email)) DO UPDATE SET name = EXCLUDED.name, password_hash = EXCLUDED.password_hash
RETURNING id;

-- name: ListTenantsAdmin :many
-- asset_count is a placeholder until the assets table exists (Task 5 replaces
-- the 0 with a count over assets joined on tenant_id). Never select asset rows here.
SELECT t.id, t.name, t.active, t.created_at,
       (SELECT count(*) FROM users u WHERE u.tenant_id = t.id)::bigint AS user_count,
       0::bigint AS asset_count
FROM tenants t
ORDER BY t.created_at, t.id;

-- name: GetTenantAdmin :one
SELECT t.id, t.name, t.active, t.created_at,
       (SELECT count(*) FROM users u WHERE u.tenant_id = t.id)::bigint AS user_count,
       0::bigint AS asset_count -- Task 5: replace with real asset count
FROM tenants t
WHERE t.id = $1;

-- name: CreateTenant :one
INSERT INTO tenants (name) VALUES ($1) RETURNING id;

-- name: UpdateTenant :execrows
UPDATE tenants SET
    name = COALESCE(sqlc.narg('name'), name),
    active = COALESCE(sqlc.narg('active'), active),
    updated_at = now()
WHERE id = @id;

-- name: ListTenantUsers :many
SELECT id, name, email, position, active FROM users WHERE tenant_id = $1 ORDER BY created_at, id;

-- name: CreateUserAdmin :one
INSERT INTO users (tenant_id, name, email, position, password_hash)
VALUES (@tenant_id, @name, @email, @position, @password_hash)
RETURNING id, name, email, position, active;

-- name: UpdateUserAdmin :one
UPDATE users SET
    name = COALESCE(sqlc.narg('name'), name),
    position = COALESCE(sqlc.narg('position'), position),
    active = COALESCE(sqlc.narg('active'), active),
    updated_at = now()
WHERE id = @id
RETURNING id, name, email, position, active;

-- name: UserExists :one
SELECT EXISTS (SELECT 1 FROM users WHERE id = $1);
