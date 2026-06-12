-- experiment.sql 定义 M7 实验模块的 sqlc 查询,仅访问实验模块自有表。
-- name: CreateExperiment :one
INSERT INTO experiment (id, tenant_id, course_id, author_id, template_ref, template_version, name, description, components, collab_mode, group_config, require_report, wizard_step, status, created_at, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 1, now(), now(), NULL)
RETURNING id, tenant_id, course_id, author_id, template_ref, template_version, name, description, components, collab_mode, group_config, require_report, wizard_step, status, created_at, updated_at, deleted_at;

-- name: GetExperiment :one
SELECT id, tenant_id, course_id, author_id, template_ref, template_version, name, description, components, collab_mode, group_config, require_report, wizard_step, status, created_at, updated_at, deleted_at
FROM experiment
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: ListExperiments :many
SELECT id, tenant_id, course_id, author_id, template_ref, template_version, name, description, components, collab_mode, group_config, require_report, wizard_step, status, created_at, updated_at, deleted_at
FROM experiment
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND ($2::bigint = 0 OR course_id = $2)
  AND ($3::smallint = 0 OR status = $3)
ORDER BY updated_at DESC, id DESC
LIMIT $4 OFFSET $5;

-- name: CountExperiments :one
SELECT COUNT(*)::bigint
FROM experiment
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND ($2::bigint = 0 OR course_id = $2)
  AND ($3::smallint = 0 OR status = $3);

-- name: UpdateExperiment :one
UPDATE experiment
SET course_id = $3,
    template_ref = $4,
    template_version = $5,
    name = $6,
    description = $7,
    components = $8,
    collab_mode = $9,
    group_config = $10,
    require_report = $11,
    wizard_step = $12,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 1 AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, author_id, template_ref, template_version, name, description, components, collab_mode, group_config, require_report, wizard_step, status, created_at, updated_at, deleted_at;

-- name: SetExperimentStatus :one
UPDATE experiment
SET status = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, author_id, template_ref, template_version, name, description, components, collab_mode, group_config, require_report, wizard_step, status, created_at, updated_at, deleted_at;

-- name: CreateExperimentGroup :one
INSERT INTO experiment_group (id, tenant_id, experiment_id, name, created_at)
VALUES ($1, $2, $3, $4, now())
RETURNING id, tenant_id, experiment_id, name, created_at;

-- name: GetExperimentGroup :one
SELECT id, tenant_id, experiment_id, name, created_at
FROM experiment_group
WHERE tenant_id = $1 AND id = $2;

-- name: ListGroupMembers :many
SELECT id, tenant_id, group_id, student_id, role, created_at
FROM group_member
WHERE tenant_id = $1 AND group_id = $2
ORDER BY id ASC;

-- name: GetGroupMember :one
SELECT id, tenant_id, group_id, student_id, role, created_at
FROM group_member
WHERE tenant_id = $1 AND group_id = $2 AND student_id = $3;

-- name: UpsertGroupMember :one
INSERT INTO group_member (id, tenant_id, group_id, student_id, role, created_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (tenant_id, group_id, student_id) DO UPDATE SET role = EXCLUDED.role
RETURNING id, tenant_id, group_id, student_id, role, created_at;

-- name: GetActiveGroupInstance :one
SELECT id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at
FROM experiment_instance
WHERE tenant_id = $1 AND experiment_id = $2 AND group_id = $3 AND status IN (1, 2, 3, 7)
ORDER BY started_at DESC, id DESC
LIMIT 1;

-- name: CreateExperimentInstance :one
INSERT INTO experiment_instance (id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, score, started_at, finished_at, last_active_at)
VALUES ($1, $2, $3, $4, $5, $6, '[]'::jsonb, '[]'::jsonb, 1, NULL, now(), NULL, now())
RETURNING id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at;

-- name: GetExperimentInstance :one
SELECT id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at
FROM experiment_instance
WHERE tenant_id = $1 AND id = $2;

-- name: GetExperimentInstanceBySourceRef :one
SELECT id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at
FROM experiment_instance
WHERE tenant_id = $1 AND source_ref = $2;

-- name: UpdateInstanceResources :one
UPDATE experiment_instance
SET sandbox_refs = $3,
    sim_session_refs = $4,
    status = $5,
    last_active_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at;

-- name: SetInstanceStatus :one
UPDATE experiment_instance
SET status = $3,
    last_active_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at;

-- name: FinishExperimentInstance :one
UPDATE experiment_instance
SET status = 4,
    score = $3::text::numeric,
    finished_at = now(),
    last_active_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at;

-- name: UpdateExperimentInstanceScore :one
UPDATE experiment_instance
SET score = $3::text::numeric,
    last_active_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at;

-- name: TouchExperimentInstance :one
UPDATE experiment_instance
SET last_active_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at;

-- name: ClaimRecyclableInstancesAcrossTenants :many
UPDATE experiment_instance
SET last_active_at = now()
WHERE id IN (
    SELECT id FROM experiment_instance
    WHERE status IN (2, 3, 4, 6)
      AND (
          (status = 3 AND last_active_at < now() - ($1::text || ' seconds')::interval)
          OR (status <> 3 AND last_active_at < now() - ($2::text || ' seconds')::interval)
      )
    ORDER BY last_active_at ASC
    LIMIT $3
    FOR UPDATE SKIP LOCKED
)
RETURNING id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at;

-- name: UpsertCheckpointResult :one
INSERT INTO checkpoint_result (id, tenant_id, instance_id, checkpoint_id, judge_task_ref, passed, score, detail_ref, judged_at)
VALUES ($1, $2, $3, $4, $5, $6, $7::text::numeric, $8, now())
ON CONFLICT (tenant_id, instance_id, checkpoint_id) DO UPDATE
SET judge_task_ref = EXCLUDED.judge_task_ref,
    passed = EXCLUDED.passed,
    score = EXCLUDED.score,
    detail_ref = EXCLUDED.detail_ref,
    judged_at = now()
RETURNING id, tenant_id, instance_id, checkpoint_id, judge_task_ref, passed, score::float8 AS score, detail_ref, judged_at;

-- name: GetCheckpointResult :one
SELECT id, tenant_id, instance_id, checkpoint_id, judge_task_ref, passed, score::float8 AS score, detail_ref, judged_at
FROM checkpoint_result
WHERE tenant_id = $1 AND instance_id = $2 AND checkpoint_id = $3;

-- name: GetCheckpointResultByJudgeTask :one
SELECT id, tenant_id, instance_id, checkpoint_id, judge_task_ref, passed, score::float8 AS score, detail_ref, judged_at
FROM checkpoint_result
WHERE tenant_id = $1 AND judge_task_ref = $2;

-- name: ListCheckpointResults :many
SELECT id, tenant_id, instance_id, checkpoint_id, judge_task_ref, passed, score::float8 AS score, detail_ref, judged_at
FROM checkpoint_result
WHERE tenant_id = $1 AND instance_id = $2
ORDER BY checkpoint_id ASC;

-- name: UpsertExperimentReport :one
INSERT INTO experiment_report (id, tenant_id, instance_id, student_id, content_ref, manual_score, comment, status, submitted_at)
VALUES ($1, $2, $3, $4, $5, NULL, NULL, 1, now())
ON CONFLICT (tenant_id, instance_id, student_id) DO UPDATE
SET content_ref = EXCLUDED.content_ref,
    status = 1,
    submitted_at = now()
RETURNING id, tenant_id, instance_id, student_id, content_ref, COALESCE(manual_score::float8, 0)::float8 AS manual_score, comment, status, submitted_at;

-- name: GradeExperimentReport :one
UPDATE experiment_report
SET manual_score = $3::text::numeric,
    comment = $4,
    status = 2
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, instance_id, student_id, content_ref, COALESCE(manual_score::float8, 0)::float8 AS manual_score, comment, status, submitted_at;

-- name: GetExperimentReport :one
SELECT id, tenant_id, instance_id, student_id, content_ref, COALESCE(manual_score::float8, 0)::float8 AS manual_score, comment, status, submitted_at
FROM experiment_report
WHERE tenant_id = $1 AND id = $2;

-- name: GetExperimentReportByInstanceStudent :one
SELECT id, tenant_id, instance_id, student_id, content_ref, COALESCE(manual_score::float8, 0)::float8 AS manual_score, comment, status, submitted_at
FROM experiment_report
WHERE tenant_id = $1 AND instance_id = $2 AND student_id = $3;

-- name: ListExperimentReports :many
SELECT r.id, r.tenant_id, r.instance_id, r.student_id, r.content_ref, COALESCE(r.manual_score::float8, 0)::float8 AS manual_score, r.comment, r.status, r.submitted_at
FROM experiment_report r
JOIN experiment_instance i ON i.tenant_id = r.tenant_id AND i.id = r.instance_id
WHERE r.tenant_id = $1 AND i.experiment_id = $2
ORDER BY r.submitted_at DESC, r.id DESC
LIMIT $3 OFFSET $4;

-- name: CountExperimentReports :one
SELECT COUNT(*)::bigint
FROM experiment_report r
JOIN experiment_instance i ON i.tenant_id = r.tenant_id AND i.id = r.instance_id
WHERE r.tenant_id = $1 AND i.experiment_id = $2;

-- name: SumCheckpointScores :one
SELECT COALESCE(SUM(score)::float8, 0)::float8
FROM checkpoint_result
WHERE tenant_id = $1 AND instance_id = $2;

-- name: SumReportScores :one
SELECT COALESCE(SUM(manual_score)::float8, 0)::float8
FROM experiment_report
WHERE tenant_id = $1 AND instance_id = $2 AND status = 2;

-- name: ExperimentStats :one
SELECT
    COUNT(*)::bigint AS experiment_count,
    COALESCE((SELECT COUNT(*)::bigint FROM experiment_instance i JOIN experiment e ON e.tenant_id = i.tenant_id AND e.id = i.experiment_id WHERE i.tenant_id = $1 AND ($2::bigint = 0 OR e.course_id = $2) AND i.status IN (1, 2, 3, 7)), 0)::bigint AS active_instance_count
FROM experiment e
WHERE e.tenant_id = $1 AND e.deleted_at IS NULL AND ($2::bigint = 0 OR e.course_id = $2);
