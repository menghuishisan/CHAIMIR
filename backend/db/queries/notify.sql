-- M10 notify sqlc 查询:仅访问通知模块自有表。

-- name: GetNotificationTemplate :one
SELECT * FROM notification_template
WHERE type = @type;

-- name: GetNotificationPreference :one
SELECT * FROM notification_preference
WHERE account_id = @account_id AND type = @type;

-- name: CreateNotification :one
INSERT INTO notification (id, tenant_id, receiver_id, type, title, content, link)
VALUES (@id, @tenant_id, @receiver_id, @type, @title, @content, @link)
RETURNING *;

-- name: ListInbox :many
SELECT * FROM notification
WHERE receiver_id = @receiver_id
  AND deleted_at IS NULL
  AND (sqlc.narg('type')::varchar IS NULL OR type = sqlc.narg('type'))
  AND (sqlc.narg('is_read')::boolean IS NULL OR is_read = sqlc.narg('is_read'))
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountInbox :one
SELECT count(*)::bigint FROM notification
WHERE receiver_id = @receiver_id
  AND deleted_at IS NULL
  AND (sqlc.narg('type')::varchar IS NULL OR type = sqlc.narg('type'))
  AND (sqlc.narg('is_read')::boolean IS NULL OR is_read = sqlc.narg('is_read'));

-- name: MarkNotificationRead :one
UPDATE notification
SET is_read = true, read_at = COALESCE(read_at, now())
WHERE id = @id AND receiver_id = @receiver_id AND deleted_at IS NULL
RETURNING *;

-- name: MarkAllNotificationsRead :exec
UPDATE notification
SET is_read = true, read_at = COALESCE(read_at, now())
WHERE receiver_id = @receiver_id AND deleted_at IS NULL AND is_read = false;

-- name: SoftDeleteNotification :one
UPDATE notification
SET deleted_at = now()
WHERE id = @id AND receiver_id = @receiver_id AND deleted_at IS NULL
RETURNING *;

-- name: ListPreferences :many
SELECT
    t.type,
    COALESCE(p.enabled, true)::boolean AS enabled,
    t.force
FROM notification_template t
LEFT JOIN notification_preference p
  ON p.type = t.type AND p.account_id = @account_id
ORDER BY t.type ASC;

-- name: UpsertNotificationPreference :one
INSERT INTO notification_preference (id, tenant_id, account_id, type, enabled)
VALUES (@id, @tenant_id, @account_id, @type, @enabled)
ON CONFLICT (tenant_id, account_id, type) DO UPDATE
SET enabled = EXCLUDED.enabled
RETURNING *;

-- name: CreateSystemAnnouncement :one
INSERT INTO system_announcement (id, tenant_id, title, content, scope, target_roles, publisher_id, expire_at)
VALUES (@id, @tenant_id, @title, @content, @scope, @target_roles, @publisher_id, @expire_at)
RETURNING *;

-- name: ListAnnouncements :many
SELECT
    a.id,
    a.tenant_id,
    a.title,
    a.content,
    a.scope,
    a.target_roles,
    a.publisher_id,
    a.published_at,
    a.expire_at,
    (r.id IS NOT NULL)::boolean AS is_read
FROM system_announcement a
LEFT JOIN announcement_read r
  ON r.tenant_id = @tenant_id AND r.announcement_id = a.id AND r.account_id = @account_id
WHERE (a.expire_at IS NULL OR a.expire_at > now())
  AND (
    a.scope = 1
    OR (a.tenant_id = @tenant_id AND a.scope = 2)
    OR (a.tenant_id = @tenant_id AND a.scope = 3 AND a.target_roles && @roles::smallint[])
  )
ORDER BY a.published_at DESC;

-- name: GetAnnouncement :one
SELECT * FROM system_announcement
WHERE id = @id
  AND (expire_at IS NULL OR expire_at > now())
  AND (tenant_id IS NULL OR tenant_id = @tenant_id);

-- name: MarkAnnouncementRead :one
INSERT INTO announcement_read (id, tenant_id, announcement_id, account_id)
VALUES (@id, @tenant_id, @announcement_id, @account_id)
ON CONFLICT (tenant_id, announcement_id, account_id) DO UPDATE
SET read_at = announcement_read.read_at
RETURNING *;
