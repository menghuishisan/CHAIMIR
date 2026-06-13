-- name: CreateTransferTask :one
INSERT INTO transfer_task (
    id, tenant_id, account_id, channel, subject, status, content_type, file_name,
    attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
    artifact_content_type, artifact_file_name, created_at, updated_at,
    completed_at, next_attempt_after
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8,
    $9, $10, $11, $12, $13,
    $14, $15, $16, $17,
    $18, $19
)
RETURNING id, tenant_id, account_id, channel, subject, status, content_type, file_name,
          attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
          artifact_content_type, artifact_file_name, created_at, updated_at,
          completed_at, next_attempt_after;

-- name: GetTransferTask :one
SELECT id, tenant_id, account_id, channel, subject, status, content_type, file_name,
       attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
       artifact_content_type, artifact_file_name, created_at, updated_at,
       completed_at, next_attempt_after
FROM transfer_task
WHERE tenant_id = $1 AND id = $2;

-- name: ListTransferTasks :many
SELECT id, tenant_id, account_id, channel, subject, status, content_type, file_name,
       attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
       artifact_content_type, artifact_file_name, created_at, updated_at,
       completed_at, next_attempt_after
FROM transfer_task
WHERE tenant_id = $1
  AND account_id = $2
  AND ($3::varchar = '' OR channel = $3)
  AND ($4::varchar = '' OR status = $4)
ORDER BY created_at DESC
LIMIT $5 OFFSET $6;

-- name: UpdateTransferTask :one
UPDATE transfer_task
SET status = $3,
    attempt_count = $4,
    max_attempts = $5,
    last_error = $6,
    artifact_ref = $7,
    artifact_size = $8,
    artifact_content_type = $9,
    artifact_file_name = $10,
    updated_at = $11,
    completed_at = $12,
    next_attempt_after = $13
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, account_id, channel, subject, status, content_type, file_name,
          attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
          artifact_content_type, artifact_file_name, created_at, updated_at,
          completed_at, next_attempt_after;

-- name: ClaimDueTransferTasks :many
WITH due AS (
    SELECT due_task.id
    FROM transfer_task AS due_task
    WHERE due_task.tenant_id = sqlc.arg(claim_tenant_id)
      AND due_task.status IN ('pending', 'retrying')
      AND (due_task.next_attempt_after IS NULL OR due_task.next_attempt_after <= sqlc.arg(now_at)::timestamptz)
    ORDER BY due_task.created_at ASC
    LIMIT sqlc.arg(batch_limit)
    FOR UPDATE SKIP LOCKED
)
UPDATE transfer_task AS task
SET status = 'running',
    updated_at = now()
FROM due
WHERE task.id = due.id
RETURNING task.id, task.tenant_id, task.account_id, task.channel, task.subject, task.status, task.content_type, task.file_name,
          task.attempt_count, task.max_attempts, task.last_error, task.artifact_ref, task.artifact_size,
          task.artifact_content_type, task.artifact_file_name, task.created_at, task.updated_at,
          task.completed_at, task.next_attempt_after;
