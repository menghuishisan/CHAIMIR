-- name: CreateAlertRule :one
INSERT INTO alert_rule (id, scope, tenant_id, name, metric, condition, level, enabled, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now(), now())
RETURNING id, scope, tenant_id, name, metric, condition, level, enabled, revision, created_at, updated_at;

-- name: ListAlertRules :many
SELECT id, scope, tenant_id, name, metric, condition, level, enabled, revision, created_at, updated_at
FROM alert_rule
WHERE (sqlc.arg(scope)::smallint = 0 OR scope = sqlc.arg(scope)::smallint)
  AND ((sqlc.narg(tenant_id)::bigint IS NULL AND tenant_id IS NULL) OR tenant_id = sqlc.narg(tenant_id)::bigint)
ORDER BY updated_at DESC;

-- name: UpdateAlertRule :one
UPDATE alert_rule
SET name = $2, metric = $3, condition = $4, level = $5, enabled = $6, revision = revision + 1, updated_at = now()
WHERE id = $1
  AND scope = sqlc.arg(scope)::smallint
  AND ((tenant_id IS NULL AND sqlc.narg(tenant_id)::bigint IS NULL) OR tenant_id = sqlc.narg(tenant_id)::bigint)
RETURNING id, scope, tenant_id, name, metric, condition, level, enabled, revision, created_at, updated_at;

-- name: CreateAlertEvent :one
INSERT INTO alert_event (id, rule_id, tenant_id, level, message, status, triggered_at)
VALUES ($1, $2, $3, $4, $5, 1, now())
RETURNING *;

-- name: ListAlertEvents :many
SELECT sqlc.embed(alert_event), (SELECT name FROM alert_rule WHERE id=alert_event.rule_id)::text AS rule_name
FROM alert_event
WHERE (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
  AND (sqlc.arg(level)::smallint = 0 OR level = sqlc.arg(level)::smallint)
  AND ((sqlc.narg(tenant_id)::bigint IS NULL) OR tenant_id = sqlc.narg(tenant_id)::bigint)
ORDER BY triggered_at DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountAlertEventsByStatus :many
-- 告警列表的状态分段计数:套用级别与学校范围,不套用状态筛选。
SELECT status, count(*)::bigint AS count
FROM alert_event
WHERE (sqlc.arg(level)::smallint = 0 OR level = sqlc.arg(level)::smallint)
  AND ((sqlc.narg(tenant_id)::bigint IS NULL) OR tenant_id = sqlc.narg(tenant_id)::bigint)
GROUP BY status;

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
RETURNING *;

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
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, type, storage_ref, size_bytes, status, started_at, finished_at;

-- name: ListBackupRecords :many
-- status 传 0 不筛选，其余按成功/失败结果查询。
SELECT id, type, storage_ref, size_bytes, status, started_at, finished_at
FROM backup_record
WHERE (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
ORDER BY started_at DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountBackupRecordsByStatus :many
-- 备份成功/失败结果的分段计数。
SELECT status, count(*)::bigint AS count
FROM backup_record
GROUP BY status;

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

-- name: ListEvaluationRules :many
SELECT * FROM alert_rule WHERE enabled ORDER BY id;

-- name: LockEvaluationRule :one
SELECT * FROM alert_rule WHERE id = $1 FOR SHARE;

-- name: EnsureAlertEvaluation :exec
INSERT INTO alert_evaluation (rule_id, tenant_id, revision, observed_at)
VALUES ($1, $2, 0, '-infinity') ON CONFLICT DO NOTHING;

-- name: LockAlertEvaluation :one
SELECT * FROM alert_evaluation WHERE rule_id = $1 AND tenant_id = $2 FOR UPDATE;

-- name: SaveAlertEvaluation :exec
UPDATE alert_evaluation SET revision=$3, observed_at=$4, breach_since=$5, fired=$6
WHERE rule_id=$1 AND tenant_id=$2;

-- name: SetAlertNotificationTrace :exec
UPDATE alert_event SET notify_trace_id=$2 WHERE id=$1;

-- name: ClaimAlertNotifications :many
WITH due AS (
 SELECT id FROM alert_event
 WHERE notify_published_at IS NULL AND notify_next_at <= now()
   AND notify_attempts < sqlc.arg(max_attempts)::integer AND tenant_id > 0
 ORDER BY notify_next_at, id LIMIT sqlc.arg(batch_size)::integer FOR UPDATE SKIP LOCKED
)
UPDATE alert_event e
SET notify_attempts=notify_attempts+1, notify_lease_token=sqlc.arg(lease_token)::bigint,
    notify_next_at=sqlc.arg(lease_until)::timestamptz
FROM due WHERE e.id=due.id RETURNING e.*;

-- name: FinishAlertNotification :execrows
UPDATE alert_event SET notify_published_at=CASE WHEN sqlc.arg(success)::boolean THEN now() ELSE NULL END,
 notify_last_error=sqlc.arg(last_error)::text, notify_lease_token=0
WHERE id=sqlc.arg(id)::bigint AND notify_lease_token=sqlc.arg(lease_token)::bigint AND notify_published_at IS NULL;
