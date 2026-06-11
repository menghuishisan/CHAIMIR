-- name: GetJudgerByCode :one
SELECT id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, status, created_at, updated_at
FROM judger
WHERE code = $1;

-- name: GetJudgerByID :one
SELECT id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, status, created_at, updated_at
FROM judger
WHERE id = $1;

-- name: ListJudgers :many
SELECT id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, status, created_at, updated_at
FROM judger
ORDER BY created_at DESC, id DESC;

-- name: UpsertJudger :one
INSERT INTO judger (id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now(), now())
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name,
    type = EXCLUDED.type,
    executor_ref = EXCLUDED.executor_ref,
    runtime_required = EXCLUDED.runtime_required,
    default_timeout_sec = EXCLUDED.default_timeout_sec,
    resource_spec = EXCLUDED.resource_spec,
    selftest_status = EXCLUDED.selftest_status,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, status, created_at, updated_at;

-- name: UpdateJudgerSelftest :one
UPDATE judger
SET selftest_status = $2, status = $3, updated_at = now()
WHERE id = $1
RETURNING id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, status, created_at, updated_at;

-- name: CreateJudgeTask :one
INSERT INTO judge_task (
    id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash,
    input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error,
    created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 0, $14, NULL, now(), now())
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: GetJudgeTask :one
SELECT id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at
FROM judge_task
WHERE tenant_id = $1 AND id = $2;

-- name: GetJudgeTaskWithResult :one
SELECT
    t.id, t.tenant_id, t.judger_id, t.source_ref, t.submitter_id, t.problem_ref, t.code_storage_key, t.code_hash,
    t.input_snapshot, t.sandbox_mode, t.target_sandbox_ref, t.priority, t.status, t.retry_count, t.max_retries, t.last_error, t.created_at, t.updated_at,
    r.passed, r.score, r.max_score, r.details, r.judge_sandbox_ref, r.judged_at, r.is_rejudge
FROM judge_task t
LEFT JOIN judge_result r ON r.tenant_id = t.tenant_id AND r.task_id = t.id
WHERE t.tenant_id = $1 AND t.id = $2;

-- name: ListJudgeTasksBySourceRef :many
SELECT id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at
FROM judge_task
WHERE tenant_id = $1 AND source_ref = $2
ORDER BY created_at DESC, id DESC;

-- name: ListJudgeTasks :many
SELECT
    t.id, t.tenant_id, t.judger_id, t.source_ref, t.submitter_id, t.problem_ref, t.code_storage_key, t.code_hash,
    t.input_snapshot, t.sandbox_mode, t.target_sandbox_ref, t.priority, t.status, t.retry_count, t.max_retries, t.last_error, t.created_at, t.updated_at,
    r.passed, r.score, r.max_score, r.details, r.judge_sandbox_ref, r.judged_at, r.is_rejudge
FROM judge_task t
LEFT JOIN judge_result r ON r.tenant_id = t.tenant_id AND r.task_id = t.id
JOIN judger j ON j.id = t.judger_id
WHERE t.tenant_id = $1
  AND ($2::text = '' OR t.source_ref = $2)
  AND ($3::boolean = false OR (t.status = 2 AND j.type = 6))
ORDER BY t.created_at DESC, t.id DESC
LIMIT $4 OFFSET $5;

-- name: CountJudgeTasks :one
SELECT COUNT(*)::bigint
FROM judge_task
JOIN judger ON judger.id = judge_task.judger_id
WHERE tenant_id = $1
  AND ($2::text = '' OR source_ref = $2)
  AND ($3::boolean = false OR (status = 2 AND judger.type = 6));

-- name: CancelQueuedJudgeTask :one
UPDATE judge_task
SET status = 7, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 1
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: ResetJudgeTaskForRejudge :one
UPDATE judge_task
SET status = 1,
    retry_count = 0,
    input_snapshot = $3,
    last_error = NULL,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status IN (3, 5)
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: DequeueJudgeTasks :many
UPDATE judge_task t
SET status = 2, updated_at = now()
WHERE t.id IN (
    SELECT id
    FROM judge_task
    WHERE status = 1
    ORDER BY priority DESC, created_at ASC, id ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: CompleteJudgeTask :one
UPDATE judge_task
SET status = 3, last_error = NULL, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: RetryJudgeTask :one
UPDATE judge_task
SET status = 1, retry_count = retry_count + 1, last_error = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: MarkJudgeTaskTimeout :one
UPDATE judge_task
SET status = 4, last_error = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: MarkJudgeTaskError :one
UPDATE judge_task
SET status = 6, last_error = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: FailJudgeTask :one
UPDATE judge_task
SET status = 5, last_error = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, judger_id, source_ref, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: UpsertJudgeResult :one
INSERT INTO judge_result (task_id, tenant_id, passed, score, max_score, details, judge_sandbox_ref, judged_at, is_rejudge)
VALUES ($1, $2, $3, $4, $5, $6, $7, now(), $8)
ON CONFLICT (task_id) DO UPDATE
SET passed = EXCLUDED.passed,
    score = EXCLUDED.score,
    max_score = EXCLUDED.max_score,
    details = EXCLUDED.details,
    judge_sandbox_ref = EXCLUDED.judge_sandbox_ref,
    judged_at = now(),
    is_rejudge = EXCLUDED.is_rejudge
RETURNING task_id, tenant_id, passed, score, max_score, details, judge_sandbox_ref, judged_at, is_rejudge;

-- name: CreateJudgeOutbox :one
INSERT INTO judge_event_outbox (id, tenant_id, task_id, subject, payload, status, retry_count, last_error, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, 1, 0, NULL, now(), now())
RETURNING id, tenant_id, task_id, subject, payload, status, retry_count, last_error, created_at, updated_at;

-- name: ListPendingJudgeOutbox :many
SELECT id, tenant_id, task_id, subject, payload, status, retry_count, last_error, created_at, updated_at
FROM judge_event_outbox
WHERE status IN (1, 3)
ORDER BY created_at ASC, id ASC
LIMIT $1;

-- name: MarkJudgeOutboxPublished :one
UPDATE judge_event_outbox
SET status = 2, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, task_id, subject, payload, status, retry_count, last_error, created_at, updated_at;

-- name: MarkJudgeOutboxFailed :one
UPDATE judge_event_outbox
SET status = 3, retry_count = retry_count + 1, last_error = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, task_id, subject, payload, status, retry_count, last_error, created_at, updated_at;

-- name: CreateSubmissionFingerprint :one
INSERT INTO submission_fingerprint (id, tenant_id, source_ref, problem_ref, submitter_id, code_hash, sim_vector, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
RETURNING id, tenant_id, source_ref, problem_ref, submitter_id, code_hash, sim_vector, created_at;

-- name: FindExactFingerprints :many
SELECT id, tenant_id, source_ref, problem_ref, submitter_id, code_hash, sim_vector, created_at
FROM submission_fingerprint
WHERE tenant_id = $1 AND problem_ref = $2 AND code_hash = $3
ORDER BY created_at DESC, id DESC;

-- name: ListFingerprintsForProblem :many
SELECT id, tenant_id, source_ref, problem_ref, submitter_id, code_hash, sim_vector, created_at
FROM submission_fingerprint
WHERE tenant_id = $1 AND problem_ref = $2 AND ($3::text = '' OR source_ref <> $3)
ORDER BY created_at DESC, id DESC;
