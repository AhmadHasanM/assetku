-- name: GetUserForLogin :one
SELECT u.id, u.tenant_id, u.password_hash, u.active AS user_active, t.active AS tenant_active
FROM users u JOIN tenants t ON t.id = u.tenant_id
WHERE lower(u.email) = lower(@email::text);

-- name: GetMe :one
SELECT u.id, u.name, u.email, u.position, u.locale, u.theme, u.password_hash,
       u.tenant_id, t.name AS tenant_name
FROM users u JOIN tenants t ON t.id = u.tenant_id
WHERE u.id = $1;

-- name: UpdateProfile :exec
UPDATE users SET
    name = COALESCE(sqlc.narg('name'), name),
    position = COALESCE(sqlc.narg('position'), position),
    locale = COALESCE(sqlc.narg('locale'), locale),
    theme = COALESCE(sqlc.narg('theme'), theme),
    updated_at = now()
WHERE id = @id;

-- name: UpdatePassword :exec
UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1;

-- name: CreateSession :exec
INSERT INTO sessions (token_hash, subject_type, subject_id, expires_at)
VALUES ($1, $2, $3, $4);

-- name: GetSession :one
SELECT subject_type, subject_id, expires_at FROM sessions
WHERE token_hash = $1 AND expires_at > now();

-- name: ExtendSession :exec
UPDATE sessions SET expires_at = $2 WHERE token_hash = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteSessionsForSubject :exec
DELETE FROM sessions WHERE subject_type = $1 AND subject_id = $2;

-- name: GetPrincipalUser :one
SELECT u.id, u.tenant_id, u.name, u.position, u.locale, u.theme
FROM users u JOIN tenants t ON t.id = u.tenant_id
WHERE u.id = $1 AND u.active AND t.active;

-- name: GetPrincipalAdmin :one
SELECT id FROM super_admins WHERE id = $1;

-- name: DeleteOtherSessionsForSubject :exec
DELETE FROM sessions WHERE subject_type = $1 AND subject_id = $2 AND token_hash <> $3;
