-- name: InsertHistory :exec
INSERT INTO asset_history (tenant_id, asset_id, event, actor_id, actor_name, detail)
VALUES (@tenant_id, @asset_id, @event, @actor_id, @actor_name, @detail);

-- name: AssetExists :one
SELECT EXISTS (SELECT 1 FROM assets WHERE tenant_id = @tenant_id AND id = @id);

-- name: ListHistory :many
-- Newest first; before (exclusive) pages backwards; limit is page size + 1.
SELECT id, event, actor_name, detail, created_at FROM asset_history
WHERE tenant_id = @tenant_id AND asset_id = @asset_id
  AND (sqlc.narg('before')::bigint IS NULL OR id < sqlc.narg('before')::bigint)
ORDER BY id DESC
LIMIT @lim;
