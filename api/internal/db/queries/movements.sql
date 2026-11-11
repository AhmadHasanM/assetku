-- name: ListMovements :many
SELECT m.id, m.from_location_id, m.from_location_name, m.to_location_id, l.name AS to_location_name,
       m.date, m.moved_by, m.reason, m.created_at
FROM movements m
JOIN locations l ON l.id = m.to_location_id
WHERE m.tenant_id = @tenant_id AND m.asset_id = @asset_id
ORDER BY m.date DESC, m.created_at DESC;

-- name: InsertMovement :one
INSERT INTO movements (tenant_id, asset_id, from_location_id, from_location_name, to_location_id,
                       date, moved_by, reason)
VALUES (@tenant_id, @asset_id, @from_location_id, @from_location_name, @to_location_id,
        @date, @moved_by, @reason)
RETURNING id, created_at;

-- name: GetMovementForDelete :one
SELECT m.id, m.asset_id, m.from_location_name, l.name AS to_location_name, m.date
FROM movements m
JOIN locations l ON l.id = m.to_location_id
WHERE m.tenant_id = @tenant_id AND m.id = @id;

-- name: DeleteMovement :execrows
DELETE FROM movements WHERE tenant_id = @tenant_id AND id = @id;

-- name: SetAssetLocation :execrows
UPDATE assets SET location_id = @location_id, updated_at = now()
WHERE tenant_id = @tenant_id AND id = @id;
