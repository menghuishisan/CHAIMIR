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

-- name: ListExperimentGroups :many
-- 按实验列出全部分组,供教师编组视角一次取齐(小组编号只在创建响应出现过一次,无列表则无法再定位)。
SELECT id, tenant_id, experiment_id, name, created_at
FROM experiment_group
WHERE tenant_id = $1 AND experiment_id = $2
ORDER BY id ASC;

-- name: ListGroupMembersByExperiment :many
-- 按实验一次取回全部分组成员,避免教师编组页逐组调用形成 N+1。
SELECT gm.id, gm.tenant_id, gm.group_id, gm.student_id, gm.role, gm.created_at
FROM group_member gm
JOIN experiment_group eg ON eg.tenant_id = gm.tenant_id AND eg.id = gm.group_id
WHERE gm.tenant_id = $1 AND eg.experiment_id = $2
ORDER BY gm.group_id ASC, gm.id ASC;

-- name: ListGroupMembers :many
SELECT id, tenant_id, group_id, student_id, role, created_at
FROM group_member
WHERE tenant_id = $1 AND group_id = $2
ORDER BY id ASC;

-- name: GetGroupMember :one
SELECT id, tenant_id, group_id, student_id, role, created_at
FROM group_member
WHERE tenant_id = $1 AND group_id = $2 AND student_id = $3;

-- name: ListStudentGroupsForExperiments :many
-- 按学生视角批量解析其在给定实验集合中所属的小组,供学生实验列表/详情一次性填充 my_group_id。
SELECT eg.experiment_id, gm.group_id
FROM group_member gm
JOIN experiment_group eg ON eg.tenant_id = gm.tenant_id AND eg.id = gm.group_id
WHERE gm.tenant_id = $1 AND gm.student_id = $2 AND eg.experiment_id = ANY(sqlc.arg(experiment_ids)::bigint[]);

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
LIMIT 1
FOR UPDATE;

-- name: GetActiveOwnerInstance :one
SELECT id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at
FROM experiment_instance
WHERE tenant_id = $1 AND experiment_id = $2 AND owner_account_id = $3 AND group_id IS NULL AND status IN (1, 2, 3, 7)
ORDER BY started_at DESC, id DESC
LIMIT 1
FOR UPDATE;

-- name: LockInstanceCreation :exec
SELECT pg_advisory_xact_lock(sqlc.arg(lock_key)::bigint);

-- name: CreateExperimentInstance :one
INSERT INTO experiment_instance (id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, score, started_at, finished_at, last_active_at)
VALUES ($1, $2, $3, $4, $5, $6, '[]'::jsonb, '[]'::jsonb, 1, NULL, now(), NULL, now())
RETURNING id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at;

-- name: GetExperimentInstance :one
SELECT id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at
FROM experiment_instance
WHERE tenant_id = $1 AND id = $2;

-- name: GetExperimentInstanceForUpdate :one
SELECT id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, COALESCE(score::float8, 0)::float8 AS score, started_at, finished_at, last_active_at
FROM experiment_instance
WHERE tenant_id = $1 AND id = $2
FOR UPDATE;

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

-- name: UpdateExperimentInstanceScoreIfChanged :one
-- 仅在总分真正变化时写入,让重复 M3 事件不能生成新的得分 outbox 修订版。
UPDATE experiment_instance
SET score = $3::text::numeric,
    last_active_at = now()
WHERE tenant_id = $1
  AND id = $2
  AND score IS DISTINCT FROM $3::text::numeric
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

-- name: ListLiveInstancesByCourse :many
-- 列出某课程下仍占用引擎资源的实验实例,供课程结束时级联回收(M7 需求 D3)。
-- 只取 creating/running/paused/released 四态:已完成、已回收与错误态不再持有沙箱或仿真会话。
SELECT i.id, i.tenant_id, i.experiment_id, i.owner_account_id, i.group_id, i.source_ref, i.sandbox_refs, i.sim_session_refs, i.status,
       COALESCE(i.score::float8, 0)::float8 AS score, i.started_at, i.finished_at, i.last_active_at
FROM experiment_instance i
JOIN experiment e ON e.tenant_id = i.tenant_id AND e.id = i.experiment_id
WHERE i.tenant_id = $1 AND e.course_id = $2 AND i.status IN (1, 2, 3, 7)
ORDER BY i.id;

-- name: UpsertCheckpointResult :one
INSERT INTO checkpoint_result (id, tenant_id, instance_id, checkpoint_id, judge_task_ref, passed, score, detail_ref, binding_output, judged_at)
VALUES ($1, $2, $3, $4, $5, $6, $7::text::numeric, $8, $9, now())
ON CONFLICT (tenant_id, instance_id, checkpoint_id) DO UPDATE
SET judge_task_ref = EXCLUDED.judge_task_ref,
    passed = EXCLUDED.passed,
    score = EXCLUDED.score,
    detail_ref = EXCLUDED.detail_ref,
    binding_output = CASE WHEN EXCLUDED.binding_output = '{}'::jsonb THEN checkpoint_result.binding_output ELSE EXCLUDED.binding_output END,
    judged_at = now()
RETURNING id, tenant_id, instance_id, checkpoint_id, judge_task_ref, passed, score::float8 AS score, detail_ref, binding_output, judged_at;

-- name: GetCheckpointResult :one
SELECT id, tenant_id, instance_id, checkpoint_id, judge_task_ref, passed, score::float8 AS score, detail_ref, binding_output, judged_at
FROM checkpoint_result
WHERE tenant_id = $1 AND instance_id = $2 AND checkpoint_id = $3;

-- name: GetCheckpointResultByJudgeTask :one
SELECT id, tenant_id, instance_id, checkpoint_id, judge_task_ref, passed, score::float8 AS score, detail_ref, binding_output, judged_at
FROM checkpoint_result
WHERE tenant_id = $1 AND judge_task_ref = $2;

-- name: ListCheckpointResults :many
SELECT id, tenant_id, instance_id, checkpoint_id, judge_task_ref, passed, score::float8 AS score, detail_ref, binding_output, judged_at
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
-- status 传 0 不按状态过滤;传具体状态供批改台按「待批改/已批改」取数与计数。
SELECT r.id, r.tenant_id, r.instance_id, r.student_id, r.content_ref, COALESCE(r.manual_score::float8, 0)::float8 AS manual_score, r.comment, r.status, r.submitted_at
FROM experiment_report r
JOIN experiment_instance i ON i.tenant_id = r.tenant_id AND i.id = r.instance_id
WHERE r.tenant_id = $1 AND i.experiment_id = $2
  AND (sqlc.arg(status)::smallint = 0 OR r.status = sqlc.arg(status)::smallint)
ORDER BY r.submitted_at DESC, r.id DESC
LIMIT $3 OFFSET $4;

-- name: CountExperimentReports :one
SELECT COUNT(*)::bigint
FROM experiment_report r
JOIN experiment_instance i ON i.tenant_id = r.tenant_id AND i.id = r.instance_id
WHERE r.tenant_id = $1 AND i.experiment_id = $2
  AND (sqlc.arg(status)::smallint = 0 OR r.status = sqlc.arg(status)::smallint);

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

-- name: CreateExperimentScoreOutbox :one
WITH next_revision AS (
    SELECT COALESCE(MAX(score_revision), 0) + 1 AS score_revision
    FROM experiment_score_outbox
    WHERE tenant_id = $2 AND instance_id = $4
)
INSERT INTO experiment_score_outbox (id, tenant_id, experiment_id, instance_id, student_id, score, score_revision, trace_id, scored_at, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until)
SELECT $1, $2, $3, $4, $5, $6::text::numeric, next_revision.score_revision, $7, $8, 1, 0, NULL, now(), now(), '', NULL
FROM next_revision
RETURNING id, tenant_id, experiment_id, instance_id, student_id, score, score_revision, trace_id, scored_at, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until;

-- name: ClaimPendingExperimentScoreOutbox :many
WITH exhausted AS (
    UPDATE experiment_score_outbox AS expired
    SET status = 4, last_error = 'score lease expired after retry limit', updated_at = now(), lease_token = '', lease_until = NULL
    WHERE expired.status = 2 AND expired.lease_until <= @stale_before::timestamptz AND expired.retry_count >= @max_attempts
    RETURNING expired.id
), candidates AS (
    SELECT o.id
    FROM experiment_score_outbox o
    WHERE (o.status IN (1, 4) OR (o.status = 2 AND o.lease_until <= @stale_before::timestamptz))
      AND o.retry_count < @max_attempts
    ORDER BY o.created_at ASC, o.id ASC
    LIMIT @page_limit
    FOR UPDATE SKIP LOCKED
)
UPDATE experiment_score_outbox AS outbox
SET status = 2, retry_count = outbox.retry_count + 1, updated_at = now(), lease_token = @lease_token, lease_until = @lease_until::timestamptz
FROM candidates
WHERE outbox.id = candidates.id
RETURNING outbox.id, outbox.tenant_id, outbox.experiment_id, outbox.instance_id, outbox.student_id, outbox.score, outbox.score_revision, outbox.trace_id, outbox.scored_at, outbox.status, outbox.retry_count, outbox.last_error, outbox.created_at, outbox.updated_at, outbox.lease_token, outbox.lease_until;

-- name: MarkExperimentScoreOutboxPublished :one
UPDATE experiment_score_outbox
SET status = 3, last_error = NULL, updated_at = now(), lease_token = '', lease_until = NULL
WHERE tenant_id = $1 AND id = $2 AND status = 2 AND lease_token = $3 AND lease_until > now()
RETURNING id, tenant_id, experiment_id, instance_id, student_id, score, score_revision, trace_id, scored_at, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until;

-- name: MarkExperimentScoreOutboxFailed :one
UPDATE experiment_score_outbox
SET status = 4, last_error = $3, updated_at = now(), lease_token = '', lease_until = NULL
WHERE tenant_id = $1 AND id = $2 AND status = 2 AND lease_token = $4 AND lease_until > now()
RETURNING id, tenant_id, experiment_id, instance_id, student_id, score, score_revision, trace_id, scored_at, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until;
