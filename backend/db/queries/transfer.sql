-- name: CreateTransferTask :one
INSERT INTO transfer_task (
    id, tenant_id, account_id, channel, subject, status, content_type, file_name,
    attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
    artifact_content_type, artifact_file_name, created_at, updated_at,
    completed_at, next_attempt_after, lease_token, lease_until
) VALUES (
    sqlc.arg(id), NULLIF(sqlc.arg(tenant_id)::bigint, 0), sqlc.arg(account_id), sqlc.arg(channel),
    sqlc.arg(subject), sqlc.arg(status), sqlc.arg(content_type), sqlc.arg(file_name),
    sqlc.arg(attempt_count), sqlc.arg(max_attempts), sqlc.arg(last_error), sqlc.arg(artifact_ref),
    sqlc.arg(artifact_size), sqlc.arg(artifact_content_type), sqlc.arg(artifact_file_name),
    sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.narg(completed_at), sqlc.narg(next_attempt_after),
    sqlc.arg(lease_token), sqlc.narg(lease_until)
)
RETURNING id, tenant_id, account_id, channel, subject, status, content_type, file_name,
          attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
          artifact_content_type, artifact_file_name, created_at, updated_at,
          completed_at, next_attempt_after, lease_token, lease_until;

-- name: GetTransferTask :one
SELECT id, tenant_id, account_id, channel, subject, status, content_type, file_name,
       attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
       artifact_content_type, artifact_file_name, created_at, updated_at,
       completed_at, next_attempt_after, lease_token, lease_until
FROM transfer_task
WHERE (tenant_id = sqlc.arg(tenant_id)::bigint OR (sqlc.arg(tenant_id)::bigint = 0 AND tenant_id IS NULL))
  AND id = sqlc.arg(id);

-- name: ListTransferTasks :many
SELECT id, tenant_id, account_id, channel, subject, status, content_type, file_name,
       attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
       artifact_content_type, artifact_file_name, created_at, updated_at,
       completed_at, next_attempt_after, lease_token, lease_until
FROM transfer_task
WHERE (tenant_id = sqlc.arg(tenant_id)::bigint OR (sqlc.arg(tenant_id)::bigint = 0 AND tenant_id IS NULL))
  AND account_id = sqlc.arg(account_id)
  AND (sqlc.arg(channel)::varchar = '' OR channel = sqlc.arg(channel)::varchar)
  AND (sqlc.arg(status)::varchar = '' OR status = sqlc.arg(status)::varchar)
ORDER BY created_at DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountTransferTasks :one
SELECT count(*)::bigint
FROM transfer_task
WHERE (tenant_id = sqlc.arg(tenant_id)::bigint OR (sqlc.arg(tenant_id)::bigint = 0 AND tenant_id IS NULL))
  AND account_id = sqlc.arg(account_id)
  AND (sqlc.arg(channel)::varchar = '' OR channel = sqlc.arg(channel)::varchar)
  AND (sqlc.arg(status)::varchar = '' OR status = sqlc.arg(status)::varchar);

-- name: CountTransferTasksByChannel :many
-- 任务与下载「全部 / 导入 / 导出」分段计数:本人任务,套用状态筛选,不套用类型筛选。
SELECT channel, count(*)::bigint AS count
FROM transfer_task
WHERE (tenant_id = sqlc.arg(tenant_id)::bigint OR (sqlc.arg(tenant_id)::bigint = 0 AND tenant_id IS NULL))
  AND account_id = sqlc.arg(account_id)::bigint
  AND (sqlc.arg(status)::varchar = '' OR status = sqlc.arg(status)::varchar)
GROUP BY channel;

-- name: ClaimTransferTask :one
WITH locked AS MATERIALIZED (
    SELECT owned.id, owned.status, owned.attempt_count, owned.max_attempts, owned.next_attempt_after, owned.lease_until
    FROM transfer_task AS owned
    WHERE (owned.tenant_id = sqlc.arg(tenant_id)::bigint OR (sqlc.arg(tenant_id)::bigint = 0 AND owned.tenant_id IS NULL))
      AND owned.id = sqlc.arg(id)
      AND owned.status IN ('pending', 'retrying', 'running')
    FOR UPDATE OF owned
), claim_time AS MATERIALIZED (
    SELECT locked.*, clock_timestamp() AS at FROM locked
)
UPDATE transfer_task AS task
SET status = CASE WHEN task.attempt_count < task.max_attempts THEN 'running' ELSE 'failed' END,
    attempt_count = CASE WHEN task.attempt_count < task.max_attempts THEN task.attempt_count + 1 ELSE task.attempt_count END,
    last_error = CASE WHEN task.attempt_count < task.max_attempts THEN '' ELSE '执行租约已过期' END,
    updated_at = claim_time.at,
    completed_at = CASE WHEN task.attempt_count < task.max_attempts THEN NULL ELSE claim_time.at END,
    next_attempt_after = NULL,
    lease_token = CASE WHEN task.attempt_count < task.max_attempts THEN sqlc.arg(lease_token)::text ELSE '' END,
    lease_until = CASE WHEN task.attempt_count < task.max_attempts THEN claim_time.at + sqlc.arg(lease_ms)::bigint * interval '1 ms' ELSE NULL END
FROM claim_time
WHERE task.id = claim_time.id
  AND (
      (claim_time.status IN ('pending', 'retrying') AND claim_time.attempt_count < claim_time.max_attempts AND (claim_time.next_attempt_after IS NULL OR claim_time.next_attempt_after <= claim_time.at))
      OR (claim_time.status = 'running' AND claim_time.lease_until <= claim_time.at)
  )
RETURNING task.*;

-- name: RenewTransferTaskLease :execrows
WITH current_lease AS MATERIALIZED (
    SELECT owned.id, owned.lease_until
    FROM transfer_task AS owned
    WHERE (owned.tenant_id = sqlc.arg(tenant_id)::bigint OR (sqlc.arg(tenant_id)::bigint = 0 AND owned.tenant_id IS NULL)) AND owned.id = sqlc.arg(id)
      AND owned.status = 'running' AND owned.lease_token = sqlc.arg(lease_token)
    FOR UPDATE OF owned
)
UPDATE transfer_task AS t
SET lease_until = greatest(updated.lease_until, updated.at + sqlc.arg(lease_ms)::bigint * interval '1 ms')
FROM (SELECT id, lease_until, clock_timestamp() AS at FROM current_lease) AS updated
WHERE t.id = updated.id AND updated.lease_until > updated.at;

-- name: CompleteTransferTask :one
WITH current_lease AS MATERIALIZED (
    SELECT owned.id, owned.lease_until
    FROM transfer_task AS owned
    WHERE (owned.tenant_id = sqlc.arg(tenant_id)::bigint OR (sqlc.arg(tenant_id)::bigint = 0 AND owned.tenant_id IS NULL))
      AND owned.id = sqlc.arg(id)
      AND owned.status = 'running'
      AND owned.lease_token = sqlc.arg(lease_token)
    FOR UPDATE OF owned
)
UPDATE transfer_task AS t
SET status = 'succeeded',
    last_error = '',
    artifact_ref = sqlc.arg(artifact_ref),
    artifact_size = sqlc.arg(artifact_size),
    artifact_content_type = sqlc.arg(artifact_content_type),
    artifact_file_name = sqlc.arg(artifact_file_name),
    updated_at = updated.at,
    completed_at = updated.at,
    next_attempt_after = NULL,
    lease_token = '',
    lease_until = NULL
FROM (SELECT id, lease_until, clock_timestamp() AS at FROM current_lease) AS updated
WHERE t.id = updated.id AND updated.lease_until > updated.at
RETURNING t.id, t.tenant_id, t.account_id, t.channel, t.subject, t.status, t.content_type, t.file_name,
          t.attempt_count, t.max_attempts, t.last_error, t.artifact_ref, t.artifact_size,
          t.artifact_content_type, t.artifact_file_name, t.created_at, t.updated_at,
          t.completed_at, t.next_attempt_after, t.lease_token, t.lease_until;

-- name: FailTransferTask :one
WITH current_lease AS MATERIALIZED (
    SELECT owned.id, owned.lease_until
    FROM transfer_task AS owned
    WHERE (owned.tenant_id = sqlc.arg(tenant_id)::bigint OR (sqlc.arg(tenant_id)::bigint = 0 AND owned.tenant_id IS NULL))
      AND owned.id = sqlc.arg(id)
      AND owned.status = 'running'
      AND owned.lease_token = sqlc.arg(lease_token)
    FOR UPDATE OF owned
)
UPDATE transfer_task AS t
SET status = CASE WHEN attempt_count < max_attempts THEN 'retrying' ELSE 'failed' END,
    last_error = sqlc.arg(last_error),
    updated_at = updated.at,
    completed_at = CASE WHEN attempt_count < max_attempts THEN NULL ELSE updated.at END,
    next_attempt_after = CASE WHEN attempt_count < max_attempts THEN updated.at + sqlc.arg(retry_ms)::bigint * interval '1 ms' ELSE NULL END,
    lease_token = '',
    lease_until = NULL
FROM (SELECT id, lease_until, clock_timestamp() AS at FROM current_lease) AS updated
WHERE t.id = updated.id AND updated.lease_until > updated.at
RETURNING t.id, t.tenant_id, t.account_id, t.channel, t.subject, t.status, t.content_type, t.file_name,
          t.attempt_count, t.max_attempts, t.last_error, t.artifact_ref, t.artifact_size,
          t.artifact_content_type, t.artifact_file_name, t.created_at, t.updated_at,
          t.completed_at, t.next_attempt_after, t.lease_token, t.lease_until;

-- name: CountFailedTransfers :one
SELECT count(*)::bigint FROM transfer_task
WHERE (tenant_id = sqlc.arg(tenant_id)::bigint OR (sqlc.arg(tenant_id)::bigint = 0 AND tenant_id IS NULL))
  AND status = 'failed' AND completed_at BETWEEN sqlc.arg(window_start) AND sqlc.arg(window_end);
