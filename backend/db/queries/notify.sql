-- name: GetNotificationTemplate :one
SELECT id, type, title_tpl, content_tpl, channels, force, created_at, updated_at
FROM notification_template
WHERE type = $1;

-- name: ListNotificationTemplates :many
SELECT id, type, title_tpl, content_tpl, channels, force, created_at, updated_at
FROM notification_template
ORDER BY type;

-- name: CreateNotifications :copyfrom
INSERT INTO notification (id, tenant_id, receiver_id, type, title, content, link, is_read, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListNotifications :many
SELECT id, tenant_id, receiver_id, type, title, content, link, is_read, read_at, created_at, deleted_at
FROM notification
WHERE receiver_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg(is_read)::boolean IS NULL OR is_read = sqlc.narg(is_read)::boolean)
  AND (sqlc.arg(type)::text = '' OR type = sqlc.arg(type)::text)
ORDER BY created_at DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountNotifications :one
SELECT COUNT(*) FROM notification
WHERE receiver_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg(is_read)::boolean IS NULL OR is_read = sqlc.narg(is_read)::boolean)
  AND (sqlc.arg(type)::text = '' OR type = sqlc.arg(type)::text);

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM notification
WHERE receiver_id = $1 AND is_read = false AND deleted_at IS NULL;

-- name: MarkNotificationRead :one
UPDATE notification
SET is_read = true, read_at = now()
WHERE id = $1 AND receiver_id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, receiver_id, type, title, content, link, is_read, read_at, created_at, deleted_at;

-- name: MarkAllNotificationsRead :exec
UPDATE notification
SET is_read = true, read_at = now()
WHERE receiver_id = $1 AND is_read = false AND deleted_at IS NULL;

-- name: DeleteNotification :one
UPDATE notification
SET deleted_at = now()
WHERE id = $1 AND receiver_id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, receiver_id, type, title, content, link, is_read, read_at, created_at, deleted_at;

-- name: ListPreferences :many
SELECT id, tenant_id, account_id, type, enabled
FROM notification_preference
WHERE account_id = $1
ORDER BY type;

-- name: UpsertPreference :one
INSERT INTO notification_preference (id, tenant_id, account_id, type, enabled)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (tenant_id, account_id, type)
DO UPDATE SET enabled = EXCLUDED.enabled
RETURNING id, tenant_id, account_id, type, enabled;

-- name: PreferenceEnabled :one
SELECT COALESCE((
    SELECT enabled FROM notification_preference WHERE tenant_id = $1 AND account_id = $2 AND type = $3
), true)::boolean;

-- name: CreateAnnouncement :one
INSERT INTO system_announcement (id, tenant_id, title, content, scope, target_roles, publisher_id, published_at, expire_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now(), $8)
RETURNING id, tenant_id, title, content, scope, target_roles, publisher_id, published_at, expire_at;

-- name: ListAnnouncements :many
SELECT a.id, a.tenant_id, a.title, a.content, a.scope, a.target_roles, a.publisher_id, a.published_at, a.expire_at,
       (r.id IS NOT NULL)::boolean AS is_read
FROM system_announcement a
LEFT JOIN announcement_read r ON r.announcement_id = a.id AND r.tenant_id = $1 AND r.account_id = $2
WHERE (a.tenant_id IS NULL OR a.tenant_id = $1)
  AND (a.expire_at IS NULL OR a.expire_at > now())
ORDER BY a.published_at DESC
LIMIT $3 OFFSET $4;

-- name: MarkAnnouncementRead :one
INSERT INTO announcement_read (id, tenant_id, announcement_id, account_id, read_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (tenant_id, announcement_id, account_id)
DO UPDATE SET read_at = now()
RETURNING id, tenant_id, announcement_id, account_id, read_at;
