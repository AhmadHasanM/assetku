-- name: DueReminderCandidates :many
-- Latest record per asset first (R12 tie-break), THEN the reminder filters, so
-- an older record never fires once a newer one exists.
SELECT l.id, l.tenant_id, l.asset_id, l.next_date, a.name AS asset_name, a.asset_tag
FROM (
    SELECT DISTINCT ON (asset_id) id, tenant_id, asset_id, next_date, reminder_days, reminder_sent_at
    FROM maintenance_records
    ORDER BY asset_id, date DESC, created_at DESC, id DESC
) l
JOIN assets a ON a.id = l.asset_id
JOIN tenants t ON t.id = l.tenant_id
WHERE t.active
  AND l.next_date IS NOT NULL
  AND l.reminder_days IS NOT NULL
  AND l.reminder_sent_at IS NULL
  AND @today::date >= l.next_date - l.reminder_days
ORDER BY l.id;

-- name: ClaimReminder :execrows
-- Atomic claim: only one concurrent run can flip reminder_sent_at.
UPDATE maintenance_records SET reminder_sent_at = now()
WHERE id = @id AND reminder_sent_at IS NULL;

-- name: InsertReminderNotifications :execrows
INSERT INTO notifications (tenant_id, user_id, type, title_id, title_en, body_id, body_en, asset_id)
SELECT u.tenant_id, u.id, 'maintenance_reminder', @title_id, @title_en, @body_id, @body_en, @asset_id
FROM users u
WHERE u.tenant_id = @tenant_id AND u.active;

-- name: ListNotifications :many
SELECT id, type, title_id, title_en, body_id, body_en, asset_id, read_at, created_at
FROM notifications
WHERE user_id = @user_id
ORDER BY id DESC
LIMIT 20;

-- name: CountUnreadNotifications :one
SELECT count(*) FROM notifications WHERE user_id = @user_id AND read_at IS NULL;

-- name: MarkNotificationRead :execrows
UPDATE notifications SET read_at = COALESCE(read_at, now())
WHERE id = @id AND user_id = @user_id AND tenant_id = @tenant_id;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications SET read_at = now() WHERE user_id = @user_id AND read_at IS NULL;
