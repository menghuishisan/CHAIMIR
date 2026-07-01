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
WITH inserted AS (
    INSERT INTO judge_task (
        id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope,
        submitter_id, problem_ref, code_storage_key, code_hash,
        input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error,
        created_at, updated_at
    )
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, 0, $17, NULL, now(), now())
    ON CONFLICT (tenant_id, source_ref, problem_ref) DO NOTHING
    RETURNING id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at
)
SELECT id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at
FROM inserted
UNION ALL
SELECT id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at
FROM judge_task
WHERE tenant_id = $2 AND source_ref = $4 AND problem_ref = $9 AND NOT EXISTS (SELECT 1 FROM inserted)
LIMIT 1;

-- name: GetJudgeTask :one
SELECT id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at
FROM judge_task
WHERE tenant_id = $1 AND id = $2;

-- name: GetJudgeTaskBySourceRef :one
SELECT id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at
FROM judge_task
WHERE tenant_id = $1 AND source_ref = $2 AND problem_ref = $3;

-- name: GetJudgeTaskWithResult :one
SELECT
    t.id, t.tenant_id, t.judger_id, t.source_ref, t.source_owner_id, t.source_course_id, t.source_scope,
    t.submitter_id, t.problem_ref, t.code_storage_key, t.code_hash,
    t.input_snapshot, t.sandbox_mode, t.target_sandbox_ref, t.priority, t.status, t.retry_count, t.max_retries, t.last_error, t.created_at, t.updated_at,
    COALESCE(r.id, 0)::bigint AS result_id,
    COALESCE(r.version, 0)::int AS result_version,
    COALESCE(r.passed, false)::boolean AS passed,
    COALESCE(r.score, 0)::int AS score,
    COALESCE(r.max_score, 0)::int AS max_score,
    COALESCE(r.details, '[]'::jsonb) AS details,
    COALESCE(r.judge_sandbox_ref, '')::varchar AS judge_sandbox_ref,
    r.judged_at,
    COALESCE(r.is_rejudge, false)::boolean AS is_rejudge
FROM judge_task t
LEFT JOIN LATERAL (
    SELECT id, version, passed, score, max_score, details, judge_sandbox_ref, judged_at, is_rejudge
    FROM judge_result
    WHERE tenant_id = t.tenant_id AND task_id = t.id
    ORDER BY version DESC
    LIMIT 1
) r ON true
WHERE t.tenant_id = $1 AND t.id = $2;

-- name: ListJudgeTasksBySourceRef :many
SELECT id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at
FROM judge_task
WHERE tenant_id = $1 AND source_ref = $2
ORDER BY created_at DESC, id DESC;

-- name: ListRecentJudgeTasksBySubmitterProblem :many
SELECT id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at
FROM judge_task
WHERE tenant_id = $1 AND submitter_id = $2 AND problem_ref = $3 AND created_at >= now() - make_interval(secs => $4::int)
ORDER BY created_at DESC, id DESC;

-- name: ListJudgeTasks :many
SELECT
    t.id, t.tenant_id, t.judger_id, t.source_ref, t.source_owner_id, t.source_course_id, t.source_scope,
    t.submitter_id, t.problem_ref, t.code_storage_key, t.code_hash,
    t.input_snapshot, t.sandbox_mode, t.target_sandbox_ref, t.priority, t.status, t.retry_count, t.max_retries, t.last_error, t.created_at, t.updated_at,
    COALESCE(r.id, 0)::bigint AS result_id,
    COALESCE(r.version, 0)::int AS result_version,
    COALESCE(r.passed, false)::boolean AS passed,
    COALESCE(r.score, 0)::int AS score,
    COALESCE(r.max_score, 0)::int AS max_score,
    COALESCE(r.details, '[]'::jsonb) AS details,
    COALESCE(r.judge_sandbox_ref, '')::varchar AS judge_sandbox_ref,
    r.judged_at,
    COALESCE(r.is_rejudge, false)::boolean AS is_rejudge
FROM judge_task t
LEFT JOIN LATERAL (
    SELECT id, version, passed, score, max_score, details, judge_sandbox_ref, judged_at, is_rejudge
    FROM judge_result
    WHERE tenant_id = t.tenant_id AND task_id = t.id
    ORDER BY version DESC
    LIMIT 1
) r ON true
JOIN judger j ON j.id = t.judger_id
WHERE t.tenant_id = $1
  AND ($2::text = '' OR t.source_ref = $2)
  AND ($3::boolean = false OR (t.status = 2 AND j.type = 6))
  AND ($4::bigint = 0 OR t.source_owner_id = $4 OR t.submitter_id = $4)
ORDER BY t.created_at DESC, t.id DESC
LIMIT $5 OFFSET $6;

-- name: CountJudgeTasks :one
SELECT COUNT(*)::bigint
FROM judge_task
JOIN judger ON judger.id = judge_task.judger_id
WHERE judge_task.tenant_id = $1
  AND ($2::text = '' OR judge_task.source_ref = $2)
  AND ($3::boolean = false OR (judge_task.status = 2 AND judger.type = 6))
  AND ($4::bigint = 0 OR judge_task.source_owner_id = $4 OR judge_task.submitter_id = $4);

-- name: CancelQueuedJudgeTask :one
UPDATE judge_task
SET status = 7, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 1
RETURNING id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: ResetJudgeTaskForRejudge :one
UPDATE judge_task
SET status = 1,
    retry_count = 0,
    input_snapshot = $3,
    last_error = NULL,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status IN (3, 5)
RETURNING id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

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
RETURNING id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: CompleteJudgeTask :one
UPDATE judge_task
SET status = 3, last_error = NULL, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: RetryJudgeTask :one
UPDATE judge_task
SET status = 1, retry_count = retry_count + 1, last_error = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: MarkJudgeTaskTimeout :one
UPDATE judge_task
SET status = 4, last_error = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: MarkJudgeTaskError :one
UPDATE judge_task
SET status = 6, last_error = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: FailJudgeTask :one
UPDATE judge_task
SET status = 5, last_error = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope, submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode, target_sandbox_ref, priority, status, retry_count, max_retries, last_error, created_at, updated_at;

-- name: UpsertJudgeResult :one
INSERT INTO judge_result (id, task_id, tenant_id, version, passed, score, max_score, details, judge_sandbox_ref, judged_at, is_rejudge)
VALUES (
    $1, $2, $3,
    COALESCE((SELECT max(version) + 1 FROM judge_result WHERE tenant_id = $3 AND task_id = $2), 1),
    $4, $5, $6, $7, $8, now(), $9
)
RETURNING id, task_id, tenant_id, version, passed, score, max_score, details, judge_sandbox_ref, judged_at, is_rejudge;

-- name: CreateJudgeOutbox :one
INSERT INTO judge_event_outbox (id, tenant_id, task_id, subject, payload, status, retry_count, last_error, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, 1, 0, NULL, now(), now())
RETURNING id, tenant_id, task_id, subject, payload, status, retry_count, next_attempt_at, last_error, created_at, updated_at;

-- name: ListPendingJudgeOutbox :many
SELECT id, tenant_id, task_id, subject, payload, status, retry_count, next_attempt_at, last_error, created_at, updated_at
FROM judge_event_outbox
WHERE status IN (1, 3) AND next_attempt_at <= now()
ORDER BY next_attempt_at ASC, created_at ASC, id ASC
LIMIT $1;

-- name: MarkJudgeOutboxPublished :one
UPDATE judge_event_outbox
SET status = 2, updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, task_id, subject, payload, status, retry_count, next_attempt_at, last_error, created_at, updated_at;

-- name: MarkJudgeOutboxFailed :one
UPDATE judge_event_outbox
SET status = 3,
    retry_count = retry_count + 1,
    next_attempt_at = now() + (LEAST(300, power(2, LEAST(retry_count + 1, 8))::int) || ' seconds')::interval,
    last_error = $3,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, task_id, subject, payload, status, retry_count, next_attempt_at, last_error, created_at, updated_at;

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
