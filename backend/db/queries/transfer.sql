-- name: CreateTransferTask :one
INSERT INTO transfer_task (
    id, tenant_id, account_id, channel, subject, status, content_type, file_name,
    attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
    artifact_content_type, artifact_file_name, created_at, updated_at,
    completed_at, next_attempt_after, lease_token, lease_until
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8,
    $9, $10, $11, $12, $13,
    $14, $15, $16, $17,
    $18, $19, $20, $21
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
WHERE tenant_id = $1 AND id = $2;

-- name: DeletePendingTransferTask :exec
DELETE FROM transfer_task
WHERE tenant_id = $1 AND id = $2 AND status = 'pending' AND attempt_count = 0;

-- name: ListTransferTasks :many
SELECT id, tenant_id, account_id, channel, subject, status, content_type, file_name,
       attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
       artifact_content_type, artifact_file_name, created_at, updated_at,
       completed_at, next_attempt_after, lease_token, lease_until
FROM transfer_task
WHERE tenant_id = $1
  AND account_id = $2
  AND ($3::varchar = '' OR channel = $3)
  AND ($4::varchar = '' OR status = $4)
ORDER BY created_at DESC
LIMIT $5 OFFSET $6;

-- name: CountTransferTasks :one
SELECT count(*)::bigint
FROM transfer_task
WHERE tenant_id = $1
  AND account_id = $2
  AND ($3::varchar = '' OR channel = $3)
  AND ($4::varchar = '' OR status = $4);

-- name: ClaimTransferTask :one
WITH expire_exhausted AS (
    UPDATE transfer_task AS task
    SET status = 'failed',
        last_error = '执行租约已过期',
        updated_at = sqlc.arg(updated_at)::timestamptz,
        completed_at = sqlc.arg(updated_at)::timestamptz,
        next_attempt_after = NULL,
        lease_token = '',
        lease_until = NULL
    WHERE task.tenant_id = sqlc.arg(tenant_id)
      AND task.id = sqlc.arg(id)
      AND task.status = 'running'
      AND task.lease_until <= sqlc.arg(updated_at)::timestamptz
      AND task.attempt_count >= task.max_attempts
)
UPDATE transfer_task AS task
SET status = 'running',
    attempt_count = attempt_count + 1,
    last_error = '',
    updated_at = sqlc.arg(updated_at)::timestamptz,
    completed_at = NULL,
    next_attempt_after = NULL,
    lease_token = sqlc.arg(lease_token),
    lease_until = sqlc.arg(lease_until)::timestamptz
WHERE task.tenant_id = sqlc.arg(tenant_id)
  AND task.id = sqlc.arg(id)
  AND task.attempt_count < task.max_attempts
  AND (
      (task.status IN ('pending', 'retrying') AND (task.next_attempt_after IS NULL OR task.next_attempt_after <= sqlc.arg(updated_at)::timestamptz))
      OR (task.status = 'running' AND task.lease_until <= sqlc.arg(updated_at)::timestamptz)
  )
RETURNING id, tenant_id, account_id, channel, subject, status, content_type, file_name,
          attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
          artifact_content_type, artifact_file_name, created_at, updated_at,
          completed_at, next_attempt_after, lease_token, lease_until;

-- name: CompleteTransferTask :one
UPDATE transfer_task
SET status = 'succeeded',
    last_error = '',
    artifact_ref = sqlc.arg(artifact_ref),
    artifact_size = sqlc.arg(artifact_size),
    artifact_content_type = sqlc.arg(artifact_content_type),
    artifact_file_name = sqlc.arg(artifact_file_name),
    updated_at = sqlc.arg(updated_at)::timestamptz,
    completed_at = sqlc.arg(completed_at)::timestamptz,
    next_attempt_after = NULL,
    lease_token = '',
    lease_until = NULL
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
  AND status = 'running'
  AND lease_token = sqlc.arg(lease_token)
  AND lease_until > sqlc.arg(updated_at)::timestamptz
RETURNING id, tenant_id, account_id, channel, subject, status, content_type, file_name,
          attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
          artifact_content_type, artifact_file_name, created_at, updated_at,
          completed_at, next_attempt_after, lease_token, lease_until;

-- name: FailTransferTask :one
UPDATE transfer_task
SET status = CASE WHEN attempt_count < max_attempts THEN 'retrying' ELSE 'failed' END,
    last_error = sqlc.arg(last_error),
    updated_at = sqlc.arg(updated_at)::timestamptz,
    completed_at = CASE WHEN attempt_count < max_attempts THEN NULL ELSE sqlc.arg(completed_at)::timestamptz END,
    next_attempt_after = CASE WHEN attempt_count < max_attempts THEN sqlc.arg(next_attempt_after)::timestamptz ELSE NULL END,
    lease_token = '',
    lease_until = NULL
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
  AND status = 'running'
  AND lease_token = sqlc.arg(lease_token)
  AND lease_until > sqlc.arg(updated_at)::timestamptz
RETURNING id, tenant_id, account_id, channel, subject, status, content_type, file_name,
          attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
          artifact_content_type, artifact_file_name, created_at, updated_at,
          completed_at, next_attempt_after, lease_token, lease_until;
