-- M3 judge sqlc 查询源。
-- 约定:租户表依赖 RLS 透明过滤;平台级 judger 通过 app 事务访问。

-- name: CreateJudger :one
INSERT INTO judger (id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, selftest_detail, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, selftest_detail, status, created_at, updated_at;

-- name: GetJudgerByID :one
SELECT id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, selftest_detail, status, created_at, updated_at FROM judger WHERE id = $1;

-- name: GetJudgerByCode :one
SELECT id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, selftest_detail, status, created_at, updated_at FROM judger WHERE code = $1;

-- name: ListJudgers :many
SELECT id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, selftest_detail, status, created_at, updated_at FROM judger
WHERE (sqlc.narg('status')::SMALLINT IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateJudger :one
UPDATE judger
SET name = $2,
    type = $3,
    executor_ref = $4,
    runtime_required = $5,
    default_timeout_sec = $6,
    resource_spec = $7,
    status = $8
WHERE id = $1
RETURNING id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, selftest_detail, status, created_at, updated_at;

-- name: UpdateJudgerSelftest :one
UPDATE judger
SET selftest_status = $2,
    selftest_detail = $3,
    status = $4
WHERE id = $1
RETURNING id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, selftest_detail, status, created_at, updated_at;

-- name: CreateJudgeTask :one
INSERT INTO judge_task (
    id, tenant_id, judger_id, source_ref, submitter_id, problem_ref,
    code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref,
    priority, status, max_retries
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, created_at, updated_at;

-- name: GetJudgeTaskByID :one
SELECT id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, created_at, updated_at FROM judge_task WHERE id = $1;

-- name: GetJudgeTaskBySourceRef :one
SELECT id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, created_at, updated_at FROM judge_task
WHERE tenant_id = @tenant_id
  AND source_ref = @source_ref;

-- name: GetJudgeTaskWithResult :one
SELECT
    jt.id, jt.tenant_id, jt.judger_id, jt.source_ref, jt.submitter_id, jt.problem_ref, jt.code_storage_key, jt.code_hash, jt.input_snapshot, jt.sandbox_mode, jt.target_sandbox_ref, jt.priority, jt.status, jt.retry_count, jt.max_retries, jt.created_at, jt.updated_at,
    jr.passed AS result_passed,
    jr.score AS result_score,
    jr.max_score AS result_max_score,
    jr.details AS result_details,
    jr.judge_sandbox_ref AS result_judge_sandbox_ref,
    jr.judged_at AS result_judged_at,
    jr.is_rejudge AS result_is_rejudge
FROM judge_task jt
LEFT JOIN judge_result jr ON jr.task_id = jt.id
WHERE jt.id = $1;

-- name: MarkJudgeTaskJudging :one
UPDATE judge_task
SET status = 2
WHERE id = $1 AND status = 1
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, created_at, updated_at;

-- name: UpdateJudgeTaskStatus :one
UPDATE judge_task
SET status = $2
WHERE id = $1
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, created_at, updated_at;

-- name: MarkJudgeTaskRejudge :one
UPDATE judge_task
SET status = 1,
    retry_count = 0,
    input_snapshot = jsonb_set(input_snapshot, '{rejudge}', 'true'::jsonb, true)
WHERE id = $1
  AND status IN (3, 7)
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, created_at, updated_at;

-- name: RetryJudgeTask :one
UPDATE judge_task
SET status = 1,
    retry_count = retry_count + 1
WHERE id = $1 AND retry_count < max_retries
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, created_at, updated_at;

-- name: FailJudgeTask :one
UPDATE judge_task
SET status = 7,
    retry_count = retry_count + 1
WHERE id = $1
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, created_at, updated_at;

-- name: CancelQueuedJudgeTask :one
UPDATE judge_task
SET status = 6
WHERE id = $1 AND status = 1
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, created_at, updated_at;

-- name: CreateJudgeResult :one
INSERT INTO judge_result (task_id, tenant_id, passed, score, max_score, details, judge_sandbox_ref, is_rejudge)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (task_id) DO UPDATE
SET passed = EXCLUDED.passed,
    score = EXCLUDED.score,
    max_score = EXCLUDED.max_score,
    details = EXCLUDED.details,
    judge_sandbox_ref = EXCLUDED.judge_sandbox_ref,
    judged_at = now(),
    is_rejudge = EXCLUDED.is_rejudge
RETURNING task_id, tenant_id, passed, score, max_score, details, judge_sandbox_ref, judged_at, is_rejudge;

-- name: CreateJudgeEventOutbox :one
INSERT INTO judge_event_outbox (id, tenant_id, task_id, subject, payload, status)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (tenant_id, task_id, subject) DO UPDATE
SET payload = EXCLUDED.payload,
    status = EXCLUDED.status,
    last_error = NULL
RETURNING id, tenant_id, task_id, subject, payload, status, retry_count, last_error, created_at, updated_at;

-- name: ListPendingJudgeEventOutboxTenants :many
SELECT DISTINCT tenant_id FROM judge_event_outbox
WHERE status IN (1, 3)
ORDER BY tenant_id
LIMIT $1;

-- name: ListPendingJudgeEventOutbox :many
SELECT id, tenant_id, task_id, subject, payload, status, retry_count, last_error, created_at, updated_at FROM judge_event_outbox
WHERE status IN (1, 3)
ORDER BY created_at ASC
LIMIT $1;

-- name: MarkJudgeEventOutboxPublished :one
UPDATE judge_event_outbox
SET status = 2,
    last_error = NULL
WHERE id = $1
RETURNING id, tenant_id, task_id, subject, payload, status, retry_count, last_error, created_at, updated_at;

-- name: FailJudgeEventOutbox :one
UPDATE judge_event_outbox
SET status = 3,
    retry_count = retry_count + 1,
    last_error = $2
WHERE id = $1
RETURNING id, tenant_id, task_id, subject, payload, status, retry_count, last_error, created_at, updated_at;

-- name: CreateSubmissionFingerprint :one
INSERT INTO submission_fingerprint (id, tenant_id, source_ref, problem_ref, submitter_id, code_hash, sim_vector)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, tenant_id, source_ref, problem_ref, submitter_id, code_hash, sim_vector, created_at;

-- name: ListExactFingerprints :many
SELECT id, tenant_id, source_ref, problem_ref, submitter_id, code_hash, sim_vector, created_at FROM submission_fingerprint
WHERE problem_ref = $1 AND code_hash = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListFingerprintsByProblem :many
SELECT id, tenant_id, source_ref, problem_ref, submitter_id, code_hash, sim_vector, created_at FROM submission_fingerprint
WHERE problem_ref = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListTasksBySourceRef :many
SELECT id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, created_at, updated_at FROM judge_task
WHERE source_ref = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListManualPendingTasks :many
SELECT jt.id, jt.tenant_id, jt.judger_id, jt.source_ref, jt.submitter_id, jt.problem_ref, jt.code_storage_key, jt.code_hash, jt.input_snapshot, jt.sandbox_mode, jt.target_sandbox_ref, jt.priority, jt.status, jt.retry_count, jt.max_retries, jt.created_at, jt.updated_at FROM judge_task jt
JOIN judger j ON j.id = jt.judger_id
WHERE jt.source_ref = $1 AND jt.status = 2 AND j.type = 6
ORDER BY jt.created_at ASC
LIMIT $2 OFFSET $3;
