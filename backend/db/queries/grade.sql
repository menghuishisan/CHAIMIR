-- M11 grade sqlc 查询:仅访问成绩中心自有表。

-- name: ListGradeLevelConfigs :many
SELECT * FROM grade_level_config
WHERE deleted_at IS NULL
ORDER BY is_default DESC, created_at DESC;

-- name: CreateGradeLevelConfig :one
INSERT INTO grade_level_config (id, tenant_id, name, mapping, warning_rules, is_default)
VALUES (@id, @tenant_id, @name, @mapping, @warning_rules, @is_default)
RETURNING *;

-- name: UpdateGradeLevelConfig :one
UPDATE grade_level_config
SET name = @name, mapping = @mapping, warning_rules = @warning_rules, is_default = @is_default, updated_at = now()
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: GetDefaultGradeLevelConfig :one
SELECT * FROM grade_level_config
WHERE is_default = true AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: ListSemesters :many
SELECT * FROM semester
WHERE deleted_at IS NULL
ORDER BY start_date DESC, id DESC;

-- name: CreateSemester :one
INSERT INTO semester (id, tenant_id, name, start_date, end_date, is_current)
VALUES (@id, @tenant_id, @name, @start_date, @end_date, @is_current)
RETURNING *;

-- name: GetCurrentSemester :one
SELECT * FROM semester
WHERE is_current = true AND deleted_at IS NULL
ORDER BY start_date DESC
LIMIT 1;

-- name: CreateGradeReview :one
INSERT INTO grade_review (id, tenant_id, course_id, submitter_id, status, is_locked)
VALUES (@id, @tenant_id, @course_id, @submitter_id, 1, false)
ON CONFLICT (tenant_id, course_id) WHERE deleted_at IS NULL DO UPDATE
SET submitter_id = EXCLUDED.submitter_id, reviewer_id = NULL, status = 1, is_locked = false,
    comment = NULL, submitted_at = now(), reviewed_at = NULL
RETURNING *;

-- name: GetGradeReview :one
SELECT * FROM grade_review
WHERE id = @id AND deleted_at IS NULL;

-- name: GetGradeReviewByCourse :one
SELECT * FROM grade_review
WHERE course_id = @course_id AND deleted_at IS NULL;

-- name: ListGradeReviews :many
SELECT * FROM grade_review
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::smallint IS NULL OR status = sqlc.narg('status'))
ORDER BY submitted_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountGradeReviews :one
SELECT count(*)::bigint FROM grade_review
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::smallint IS NULL OR status = sqlc.narg('status'));

-- name: ApproveGradeReview :one
UPDATE grade_review
SET reviewer_id = @reviewer_id, semester_id = @semester_id, status = 2, is_locked = true, comment = @comment, reviewed_at = now()
WHERE id = @id AND status = 1 AND deleted_at IS NULL
RETURNING *;

-- name: RejectGradeReview :one
UPDATE grade_review
SET reviewer_id = @reviewer_id, status = 3, is_locked = false, comment = @comment, reviewed_at = now()
WHERE id = @id AND status = 1 AND deleted_at IS NULL
RETURNING *;

-- name: UnlockGradeReview :one
UPDATE grade_review
SET reviewer_id = @reviewer_id, status = 1, is_locked = false, comment = @comment, reviewed_at = now()
WHERE id = @id AND status = 2 AND deleted_at IS NULL
RETURNING *;

-- name: RelockGradeReviewByCourse :one
UPDATE grade_review
SET status = 2, is_locked = true, reviewed_at = now()
WHERE course_id = @course_id AND deleted_at IS NULL
RETURNING *;

-- name: UpsertStudentSemesterGrade :one
INSERT INTO student_semester_grade (id, tenant_id, student_id, semester_id, total_credits, gpa, cumulative_gpa)
VALUES (@id, @tenant_id, @student_id, @semester_id, @total_credits, @gpa, @cumulative_gpa)
ON CONFLICT (tenant_id, student_id, semester_id) DO UPDATE
SET total_credits = EXCLUDED.total_credits, gpa = EXCLUDED.gpa, cumulative_gpa = EXCLUDED.cumulative_gpa, computed_at = now()
RETURNING *;

-- name: ListStudentSemesterGrades :many
SELECT * FROM student_semester_grade
WHERE student_id = @student_id
ORDER BY semester_id ASC;

-- name: ListSemesterGrades :many
SELECT * FROM student_semester_grade
WHERE semester_id = @semester_id
ORDER BY student_id ASC;

-- name: CreateGradeAppeal :one
INSERT INTO grade_appeal (id, tenant_id, student_id, course_id, reason, status)
VALUES (@id, @tenant_id, @student_id, @course_id, @reason, 1)
RETURNING *;

-- name: FindOpenGradeAppeal :one
SELECT * FROM grade_appeal
WHERE student_id = @student_id AND course_id = @course_id AND status IN (1, 2)
ORDER BY created_at DESC
LIMIT 1;

-- name: GetGradeAppeal :one
SELECT * FROM grade_appeal
WHERE id = @id;

-- name: ListGradeAppeals :many
SELECT * FROM grade_appeal
WHERE (sqlc.narg('status')::smallint IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountGradeAppeals :one
SELECT count(*)::bigint FROM grade_appeal
WHERE (sqlc.narg('status')::smallint IS NULL OR status = sqlc.narg('status'));

-- name: UpdateGradeAppealStatus :one
UPDATE grade_appeal
SET handler_id = @handler_id, status = @status, result_comment = @result_comment, handled_at = now()
WHERE id = @id
RETURNING *;

-- name: CreateAcademicWarning :one
INSERT INTO academic_warning (id, tenant_id, student_id, semester_id, type, detail, status)
VALUES (@id, @tenant_id, @student_id, @semester_id, @type, @detail, 1)
RETURNING *;

-- name: ListAcademicWarnings :many
SELECT * FROM academic_warning
WHERE (sqlc.narg('student_id')::bigint IS NULL OR student_id = sqlc.narg('student_id'))
  AND (sqlc.narg('semester_id')::bigint IS NULL OR semester_id = sqlc.narg('semester_id'))
  AND (sqlc.narg('status')::smallint IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountAcademicWarnings :one
SELECT count(*)::bigint FROM academic_warning
WHERE (sqlc.narg('student_id')::bigint IS NULL OR student_id = sqlc.narg('student_id'))
  AND (sqlc.narg('semester_id')::bigint IS NULL OR semester_id = sqlc.narg('semester_id'))
  AND (sqlc.narg('status')::smallint IS NULL OR status = sqlc.narg('status'));

-- name: AcknowledgeAcademicWarning :one
UPDATE academic_warning
SET status = 2
WHERE id = @id AND student_id = @student_id
RETURNING *;

-- name: DeleteAcademicWarning :exec
DELETE FROM academic_warning
WHERE id = @id;

-- name: CreateTranscriptRecord :one
INSERT INTO transcript_record (id, tenant_id, student_id, scope, semester_id, pdf_ref)
VALUES (@id, @tenant_id, @student_id, @scope, @semester_id, @pdf_ref)
RETURNING *;

-- name: GetTranscriptRecord :one
SELECT * FROM transcript_record
WHERE id = @id;
