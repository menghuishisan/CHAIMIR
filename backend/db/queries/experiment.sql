-- M7 experiment sqlc 查询:仅访问 experiment 模块自有租户表。

-- name: CreateExperiment :one
INSERT INTO experiment (id, tenant_id, course_id, author_id, template_ref, template_version, name, description, components, collab_mode, group_config, require_report, wizard_step, status)
VALUES (@id, @tenant_id, @course_id, @author_id, @template_ref, @template_version, @name, @description, @components, @collab_mode, @group_config, @require_report, @wizard_step, @status)
RETURNING *;

-- name: GetExperimentByID :one
SELECT * FROM experiment WHERE id = @id AND deleted_at IS NULL;

-- name: ListExperiments :many
SELECT * FROM experiment
WHERE deleted_at IS NULL
  AND (sqlc.narg('course_id')::bigint IS NULL OR course_id = sqlc.narg('course_id'))
  AND (sqlc.narg('status')::smallint IS NULL OR status = sqlc.narg('status'))
ORDER BY updated_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: UpdateExperiment :one
UPDATE experiment SET
  course_id = @course_id,
  template_ref = @template_ref,
  template_version = @template_version,
  name = @name,
  description = @description,
  components = @components,
  collab_mode = @collab_mode,
  group_config = @group_config,
  require_report = @require_report,
  wizard_step = @wizard_step
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: UpdateExperimentStatus :one
UPDATE experiment SET status = @status
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: CreateExperimentInstance :one
INSERT INTO experiment_instance (id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status)
VALUES (@id, @tenant_id, @experiment_id, @owner_account_id, @group_id, @source_ref, @sandbox_refs, @sim_session_refs, @status)
RETURNING *;

-- name: GetExperimentInstanceByID :one
SELECT * FROM experiment_instance WHERE id = @id;

-- name: UpdateExperimentInstanceResources :one
UPDATE experiment_instance SET sandbox_refs = @sandbox_refs, sim_session_refs = @sim_session_refs, status = @status, last_active_at = now()
WHERE id = @id
RETURNING *;

-- name: UpdateExperimentInstanceStatus :one
UPDATE experiment_instance SET status = @status, last_active_at = now()
WHERE id = @id
RETURNING *;

-- name: UpdateExperimentInstanceScore :one
UPDATE experiment_instance SET score = @score, status = @status, finished_at = now(), last_active_at = now()
WHERE id = @id
RETURNING *;

-- name: MarkInstancesReleasedBySandbox :many
UPDATE experiment_instance
SET status = @status, last_active_at = now()
WHERE tenant_id = @tenant_id
  AND sandbox_refs @> @sandbox_ref_json::jsonb
  AND status IN (2, 3)
RETURNING *;

-- name: CreateExperimentGroup :one
INSERT INTO experiment_group (id, tenant_id, experiment_id, name)
VALUES (@id, @tenant_id, @experiment_id, @name)
RETURNING *;

-- name: GetExperimentGroupByID :one
SELECT * FROM experiment_group WHERE id = @id;

-- name: GetExperimentGroupByIDAndExperiment :one
SELECT * FROM experiment_group WHERE id = @id AND experiment_id = @experiment_id;

-- name: AddGroupMemberAuthorized :one
WITH target_group AS (
  SELECT g.id
  FROM experiment_group g
  JOIN experiment e ON e.id = g.experiment_id
  WHERE g.id = @group_id
    AND (
      @is_platform::boolean
      OR @is_school_admin::boolean
      OR e.author_id = @actor_id
    )
),
upserted AS (
  INSERT INTO group_member (id, tenant_id, group_id, student_id, role)
  SELECT @id, @tenant_id, @group_id, @student_id, @role FROM target_group
  ON CONFLICT (tenant_id, group_id, student_id) DO UPDATE SET role = EXCLUDED.role
  RETURNING group_member.*, TRUE AS authorized
)
SELECT * FROM upserted
UNION ALL
SELECT 0::bigint AS id, 0::bigint AS tenant_id, @group_id::bigint AS group_id, @student_id::bigint AS student_id, @role::varchar AS role, FALSE AS authorized
WHERE EXISTS (SELECT 1 FROM experiment_group WHERE id = @group_id)
  AND NOT EXISTS (SELECT 1 FROM upserted)
LIMIT 1;

-- name: GetGroupMember :one
SELECT * FROM group_member WHERE group_id = @group_id AND student_id = @student_id;

-- name: ListGroupMembers :many
SELECT * FROM group_member WHERE group_id = @group_id ORDER BY id ASC;

-- name: UpsertCheckpointResult :one
INSERT INTO checkpoint_result (id, tenant_id, instance_id, checkpoint_id, judge_task_ref, passed, score, detail_ref)
VALUES (@id, @tenant_id, @instance_id, @checkpoint_id, @judge_task_ref, @passed, @score, @detail_ref)
ON CONFLICT (tenant_id, instance_id, checkpoint_id) DO UPDATE SET
  judge_task_ref = EXCLUDED.judge_task_ref,
  passed = EXCLUDED.passed,
  score = EXCLUDED.score,
  detail_ref = EXCLUDED.detail_ref,
  judged_at = now()
RETURNING *;

-- name: GetCheckpointResultByJudgeTask :one
SELECT cr.*, i.source_ref FROM checkpoint_result cr
JOIN experiment_instance i ON i.id = cr.instance_id
WHERE cr.judge_task_ref = @judge_task_ref;

-- name: ListCheckpointResultsByInstance :many
SELECT * FROM checkpoint_result WHERE instance_id = @instance_id ORDER BY checkpoint_id ASC;

-- name: CreateExperimentReport :one
INSERT INTO experiment_report (id, tenant_id, instance_id, student_id, content_ref, status)
VALUES (@id, @tenant_id, @instance_id, @student_id, @content_ref, @status)
ON CONFLICT (tenant_id, instance_id, student_id) DO UPDATE SET
  content_ref = EXCLUDED.content_ref,
  status = EXCLUDED.status,
  submitted_at = now()
RETURNING *;

-- name: ListReportsByExperiment :many
SELECT r.* FROM experiment_report r
JOIN experiment_instance i ON i.id = r.instance_id
WHERE i.experiment_id = @experiment_id
ORDER BY r.submitted_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: ListReportsByInstance :many
SELECT * FROM experiment_report WHERE instance_id = @instance_id ORDER BY submitted_at DESC;

-- name: GetReportByID :one
SELECT * FROM experiment_report WHERE id = @id;

-- name: GradeExperimentReportAuthorized :one
WITH target_report AS (
  SELECT r.id
  FROM experiment_report r
  JOIN experiment_instance i ON i.id = r.instance_id
  JOIN experiment e ON e.id = i.experiment_id
  WHERE r.id = @id
    AND (
      @is_platform::boolean
      OR @is_school_admin::boolean
      OR e.author_id = @actor_id
    )
),
updated AS (
  UPDATE experiment_report r
  SET manual_score = @manual_score, comment = @comment, status = @status
  FROM target_report tr
  WHERE r.id = tr.id
  RETURNING r.*, TRUE AS authorized
)
SELECT * FROM updated
UNION ALL
SELECT r.*, FALSE AS authorized
FROM experiment_report r
WHERE r.id = @id
  AND NOT EXISTS (SELECT 1 FROM updated)
LIMIT 1;

-- name: CountExperiments :one
SELECT count(*)::bigint FROM experiment WHERE deleted_at IS NULL AND (sqlc.narg('course_id')::bigint IS NULL OR course_id = sqlc.narg('course_id'));

-- name: CountActiveInstances :one
SELECT count(*)::bigint FROM experiment_instance i
JOIN experiment e ON e.id = i.experiment_id
WHERE i.status IN (1, 2, 3, 7)
  AND (sqlc.narg('course_id')::bigint IS NULL OR e.course_id = sqlc.narg('course_id'));
