-- M9 admin sqlc 查询:仅访问 admin 模块自有运维元数据表。

-- name: ListStatistics :many
SELECT id, scope, tenant_id, stat_date, metrics, created_at FROM platform_statistics
WHERE scope = @scope
  AND (sqlc.narg('tenant_id')::bigint IS NULL OR tenant_id = sqlc.narg('tenant_id'))
  AND stat_date >= @from_date
  AND stat_date <= @to_date
ORDER BY stat_date DESC;

-- name: ListConfigs :many
SELECT id, scope, tenant_id, key, value, version, updated_by, updated_at FROM system_config
WHERE scope = @scope
  AND (sqlc.narg('tenant_id')::bigint IS NULL OR tenant_id = sqlc.narg('tenant_id'))
ORDER BY key ASC;

-- name: GetConfigByKey :one
SELECT id, scope, tenant_id, key, value, version, updated_by, updated_at FROM system_config
WHERE scope = @scope
  AND ((sqlc.narg('tenant_id')::bigint IS NULL AND tenant_id IS NULL) OR tenant_id = sqlc.narg('tenant_id'))
  AND key = @key;

-- name: UpdateConfigWithVersion :one
UPDATE system_config
SET value = @value, version = version + 1, updated_by = @updated_by, updated_at = now()
WHERE id = @id AND version = @version
RETURNING id, scope, tenant_id, key, value, version, updated_by, updated_at;

-- name: CreateConfigChangeLog :one
INSERT INTO config_change_log (id, config_id, tenant_id, old_value, new_value, operator_id)
VALUES (@id, @config_id, @tenant_id, @old_value, @new_value, @operator_id)
RETURNING id, config_id, tenant_id, old_value, new_value, operator_id, created_at;

-- name: ListConfigHistory :many
SELECT id, config_id, tenant_id, old_value, new_value, operator_id, created_at FROM config_change_log
WHERE config_id = @config_id
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountConfigHistory :one
SELECT count(*)::bigint FROM config_change_log WHERE config_id = @config_id;

-- name: GetConfigHistoryByID :one
SELECT id, config_id, tenant_id, old_value, new_value, operator_id, created_at FROM config_change_log
WHERE id = @id AND config_id = @config_id;

-- name: ListAlertRules :many
SELECT id, scope, tenant_id, name, metric, condition, level, enabled, created_at, updated_at FROM alert_rule
WHERE scope = @scope
  AND (sqlc.narg('tenant_id')::bigint IS NULL OR tenant_id = sqlc.narg('tenant_id'))
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountAlertRules :one
SELECT count(*)::bigint FROM alert_rule
WHERE scope = @scope
  AND (sqlc.narg('tenant_id')::bigint IS NULL OR tenant_id = sqlc.narg('tenant_id'));

-- name: CreateAlertRule :one
INSERT INTO alert_rule (id, scope, tenant_id, name, metric, condition, level, enabled)
VALUES (@id, @scope, @tenant_id, @name, @metric, @condition, @level, @enabled)
RETURNING id, scope, tenant_id, name, metric, condition, level, enabled, created_at, updated_at;

-- name: UpdateAlertRule :one
UPDATE alert_rule
SET name = @name, metric = @metric, condition = @condition, level = @level, enabled = @enabled
WHERE id = @id
  AND (sqlc.narg('tenant_id')::bigint IS NULL OR tenant_id = sqlc.narg('tenant_id'))
RETURNING id, scope, tenant_id, name, metric, condition, level, enabled, created_at, updated_at;

-- name: ListAlertEvents :many
SELECT id, rule_id, tenant_id, level, message, status, handler_id, triggered_at, handled_at FROM alert_event
WHERE (sqlc.narg('status')::smallint IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('tenant_id')::bigint IS NULL OR tenant_id = sqlc.narg('tenant_id'))
ORDER BY triggered_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountAlertEvents :one
SELECT count(*)::bigint FROM alert_event
WHERE (sqlc.narg('status')::smallint IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('tenant_id')::bigint IS NULL OR tenant_id = sqlc.narg('tenant_id'));

-- name: GetAlertEventByID :one
SELECT id, rule_id, tenant_id, level, message, status, handler_id, triggered_at, handled_at FROM alert_event
WHERE id = @id
  AND (sqlc.narg('tenant_id')::bigint IS NULL OR tenant_id = sqlc.narg('tenant_id'));

-- name: HandleAlertEvent :one
UPDATE alert_event
SET status = @status, handler_id = @handler_id, handled_at = now()
WHERE id = @id AND status = 1
  AND (sqlc.narg('tenant_id')::bigint IS NULL OR tenant_id = sqlc.narg('tenant_id'))
RETURNING id, rule_id, tenant_id, level, message, status, handler_id, triggered_at, handled_at;

-- name: RevertAlertEvent :one
UPDATE alert_event
SET status = 1, handler_id = NULL, handled_at = NULL
WHERE id = @id
  AND (sqlc.narg('tenant_id')::bigint IS NULL OR tenant_id = sqlc.narg('tenant_id'))
RETURNING id, rule_id, tenant_id, level, message, status, handler_id, triggered_at, handled_at;

-- name: ListBackups :many
SELECT id, type, storage_ref, size_bytes, status, started_at, finished_at FROM backup_record
ORDER BY started_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountBackups :one
SELECT count(*)::bigint FROM backup_record;

-- name: CreateBackupRecord :one
INSERT INTO backup_record (id, type, storage_ref, size_bytes, status)
VALUES (@id, @type, @storage_ref, 0, @status)
RETURNING id, type, storage_ref, size_bytes, status, started_at, finished_at;
