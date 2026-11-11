-- name: ListMaintenance :many
SELECT id, date, type, performed_by, cost, next_date, reminder_days, notes, created_by_name, created_at
FROM maintenance_records
WHERE tenant_id = @tenant_id AND asset_id = @asset_id
ORDER BY date DESC, created_at DESC;

-- name: LatestMaintenanceWithNextDate :one
-- The "next due" rule, shared with the reminder job: only the LATEST record of
-- the asset (date DESC, created_at DESC, id DESC) counts, and it is due only
-- if that record has a next_date.
SELECT id, next_date
FROM (
    SELECT id, next_date
    FROM maintenance_records
    WHERE tenant_id = @tenant_id AND asset_id = @asset_id
    ORDER BY date DESC, created_at DESC, id DESC
    LIMIT 1
) latest
WHERE next_date IS NOT NULL;

-- name: InsertMaintenance :one
INSERT INTO maintenance_records (tenant_id, asset_id, date, type, performed_by, cost, next_date,
                                 reminder_days, notes, created_by_name)
VALUES (@tenant_id, @asset_id, @date, @type, @performed_by, @cost, @next_date,
        @reminder_days, @notes, @created_by_name)
RETURNING id, date, type, performed_by, cost, next_date, reminder_days, notes, created_by_name, created_at;

-- name: LockMaintenance :one
-- Tenant-scoped row lock (serialises concurrent deletes of the same record).
SELECT id, asset_id, type, date FROM maintenance_records
WHERE tenant_id = @tenant_id AND id = @id FOR UPDATE;

-- name: DeleteMaintenance :execrows
DELETE FROM maintenance_records WHERE tenant_id = @tenant_id AND id = @id;
