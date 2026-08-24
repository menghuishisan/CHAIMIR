-- name: ListSystemConfigs :many
SELECT id, scope, tenant_id, key, value, version, updated_by, updated_at
FROM system_config
WHERE (sqlc.arg(scope)::smallint = 0 OR scope = sqlc.arg(scope)::smallint)
  AND ((sqlc.narg(tenant_id)::bigint IS NULL AND tenant_id IS NULL) OR tenant_id = sqlc.narg(tenant_id)::bigint)
ORDER BY key;

-- name: GetSystemConfig :one
SELECT id, scope, tenant_id, key, value, version, updated_by, updated_at
FROM system_config
WHERE scope = sqlc.arg(scope)::smallint
  AND ((tenant_id IS NULL AND sqlc.narg(tenant_id)::bigint IS NULL) OR tenant_id = sqlc.narg(tenant_id)::bigint)
  AND key = sqlc.arg(key)::text;

-- name: CreateSystemConfig :one
INSERT INTO system_config (id, scope, tenant_id, key, value, version, updated_by, updated_at)
VALUES ($1, $2, $3, $4, $5, 1, $6, now())
RETURNING id, scope, tenant_id, key, value, version, updated_by, updated_at;

-- name: UpdateSystemConfig :one
UPDATE system_config
SET value = sqlc.arg(value), version = version + 1, updated_by = sqlc.arg(updated_by)::bigint, updated_at = now()
WHERE scope = sqlc.arg(scope)::smallint
  AND ((tenant_id IS NULL AND sqlc.narg(tenant_id)::bigint IS NULL) OR tenant_id = sqlc.narg(tenant_id)::bigint)
  AND key = sqlc.arg(key)::text
  AND version = sqlc.arg(version)::int
RETURNING id, scope, tenant_id, key, value, version, updated_by, updated_at;

-- name: CreateConfigChangeLog :one
INSERT INTO config_change_log (id, config_id, tenant_id, old_value, new_value, operator_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
RETURNING id, config_id, tenant_id, old_value, new_value, operator_id, created_at;

-- name: ListConfigChangeLogs :many
SELECT id, config_id, tenant_id, old_value, new_value, operator_id, created_at
FROM config_change_log
WHERE config_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountConfigChangeLogs :one
SELECT count(*)::bigint
FROM config_change_log
WHERE config_id = $1;

-- name: GetConfigChangeLog :one
SELECT id, config_id, tenant_id, old_value, new_value, operator_id, created_at
FROM config_change_log
WHERE id = $1 AND config_id = $2;

-- name: CreateAlertRule :one
INSERT INTO alert_rule (id, scope, tenant_id, name, metric, condition, level, enabled, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now(), now())
RETURNING id, scope, tenant_id, name, metric, condition, level, enabled, created_at, updated_at;

-- name: ListAlertRules :many
SELECT id, scope, tenant_id, name, metric, condition, level, enabled, created_at, updated_at
FROM alert_rule
WHERE (sqlc.arg(scope)::smallint = 0 OR scope = sqlc.arg(scope)::smallint)
  AND ((sqlc.narg(tenant_id)::bigint IS NULL AND tenant_id IS NULL) OR tenant_id = sqlc.narg(tenant_id)::bigint)
ORDER BY updated_at DESC;

-- name: GetAlertRule :one
SELECT id, scope, tenant_id, name, metric, condition, level, enabled, created_at, updated_at
FROM alert_rule
WHERE id = $1;

-- name: UpdateAlertRule :one
UPDATE alert_rule
SET name = $2, metric = $3, condition = $4, level = $5, enabled = $6, updated_at = now()
WHERE id = $1
  AND scope = sqlc.arg(scope)::smallint
  AND ((tenant_id IS NULL AND sqlc.narg(tenant_id)::bigint IS NULL) OR tenant_id = sqlc.narg(tenant_id)::bigint)
RETURNING id, scope, tenant_id, name, metric, condition, level, enabled, created_at, updated_at;

-- name: CreateAlertEvent :one
INSERT INTO alert_event (id, rule_id, tenant_id, level, message, status, triggered_at)
VALUES ($1, $2, $3, $4, $5, 1, now())
RETURNING id, rule_id, tenant_id, level, message, status, handler_id, triggered_at, handled_at;

-- name: ListAlertEvents :many
SELECT id, rule_id, tenant_id, level, message, status, handler_id, triggered_at, handled_at
FROM alert_event
WHERE (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
  AND (sqlc.arg(level)::smallint = 0 OR level = sqlc.arg(level)::smallint)
  AND ((sqlc.narg(tenant_id)::bigint IS NULL) OR tenant_id = sqlc.narg(tenant_id)::bigint)
ORDER BY triggered_at DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountAlertEvents :one
SELECT count(*)::bigint
FROM alert_event
WHERE (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
  AND (sqlc.arg(level)::smallint = 0 OR level = sqlc.arg(level)::smallint)
  AND ((sqlc.narg(tenant_id)::bigint IS NULL) OR tenant_id = sqlc.narg(tenant_id)::bigint);

-- name: HandleAlertEvent :one
UPDATE alert_event
SET status = $2, handler_id = $3, handled_at = now()
WHERE id = $1 AND status = 1
  AND ((sqlc.narg(tenant_id)::bigint IS NULL) OR tenant_id = sqlc.narg(tenant_id)::bigint)
RETURNING id, rule_id, tenant_id, level, message, status, handler_id, triggered_at, handled_at;

-- name: UpsertGlobalPlatformStatistics :one
INSERT INTO platform_statistics (id, scope, tenant_id, stat_date, metrics, created_at)
VALUES ($1, 1, NULL, $2, $3, now())
ON CONFLICT (scope, stat_date) WHERE tenant_id IS NULL
DO UPDATE SET metrics = EXCLUDED.metrics, created_at = now()
RETURNING id, scope, tenant_id, stat_date, metrics, created_at;

-- name: UpsertTenantPlatformStatistics :one
INSERT INTO platform_statistics (id, scope, tenant_id, stat_date, metrics, created_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (scope, tenant_id, stat_date) WHERE tenant_id IS NOT NULL
DO UPDATE SET metrics = EXCLUDED.metrics, created_at = now()
RETURNING id, scope, tenant_id, stat_date, metrics, created_at;

-- name: ListPlatformStatistics :many
SELECT id, scope, tenant_id, stat_date, metrics, created_at
FROM platform_statistics
WHERE scope = sqlc.arg(scope)::smallint
  AND ((sqlc.narg(tenant_id)::bigint IS NULL AND tenant_id IS NULL) OR tenant_id = sqlc.narg(tenant_id)::bigint)
  AND stat_date BETWEEN sqlc.arg(from_date)::date AND sqlc.arg(to_date)::date
ORDER BY stat_date;

-- name: CreateBackupRecord :one
INSERT INTO backup_record (id, type, storage_ref, size_bytes, status, started_at, finished_at)
VALUES ($1, $2, $3, $4, $5, now(), $6)
RETURNING id, type, storage_ref, size_bytes, status, started_at, finished_at;

-- name: ListBackupRecords :many
-- status 传 0 不按结果过滤;传具体结果供备份巡检按「已成功/已失败/进行中」取数与计数。
SELECT id, type, storage_ref, size_bytes, status, started_at, finished_at
FROM backup_record
WHERE (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
ORDER BY started_at DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountBackupRecords :one
SELECT count(*)::bigint
FROM backup_record
WHERE (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint);

-- name: CreateAuditExportRequest :one
INSERT INTO audit_export_request (transfer_task_id, tenant_id, account_id, query_snapshot, next_check_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, now(), now(), now())
RETURNING transfer_task_id, tenant_id, account_id, query_snapshot, next_check_at, created_at, updated_at;

-- name: ListDueAuditExportRequests :many
SELECT transfer_task_id, tenant_id, account_id, query_snapshot, next_check_at, created_at, updated_at
FROM audit_export_request
WHERE next_check_at <= $1
ORDER BY next_check_at ASC, created_at ASC
LIMIT $2;

-- name: SetAuditExportRequestNextCheck :one
UPDATE audit_export_request
SET next_check_at = $3, updated_at = now()
WHERE transfer_task_id = $1 AND tenant_id = $2
RETURNING transfer_task_id, tenant_id, account_id, query_snapshot, next_check_at, created_at, updated_at;

-- name: DeleteAuditExportRequest :exec
DELETE FROM audit_export_request
WHERE transfer_task_id = $1 AND tenant_id = $2;
