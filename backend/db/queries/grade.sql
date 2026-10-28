-- name: CreateLevelConfig :one
INSERT INTO grade_level_config (id, tenant_id, name, mapping, warning_rules, is_default, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now(), now())
RETURNING id, tenant_id, name, mapping, warning_rules, is_default, created_at, updated_at;

-- name: ListLevelConfigs :many
SELECT id, tenant_id, name, mapping, warning_rules, is_default, created_at, updated_at
FROM grade_level_config
ORDER BY is_default DESC, updated_at DESC;

-- name: GetDefaultLevelConfig :one
SELECT id, tenant_id, name, mapping, warning_rules, is_default, created_at, updated_at
FROM grade_level_config
WHERE is_default = true
LIMIT 1;

-- name: LockGradeLevelDefaultScope :exec
SELECT pg_advisory_xact_lock(sqlc.arg(lock_key)::bigint);

-- name: ClearDefaultLevelConfigs :exec
UPDATE grade_level_config
SET is_default = false, updated_at = now()
WHERE is_default = true;

-- name: UpdateLevelConfig :one
UPDATE grade_level_config
SET name = $2, mapping = $3, warning_rules = $4, is_default = $5, updated_at = now()
WHERE id = $1
RETURNING id, tenant_id, name, mapping, warning_rules, is_default, created_at, updated_at;

-- name: CreateSemester :one
INSERT INTO semester (id, tenant_id, name, start_date, end_date, is_current)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, tenant_id, name, start_date, end_date, is_current;

-- name: ListSemesters :many
SELECT id, tenant_id, name, start_date, end_date, is_current
FROM semester
ORDER BY start_date DESC;

-- name: GetCurrentSemester :one
SELECT id, tenant_id, name, start_date, end_date, is_current
FROM semester
WHERE is_current = true
LIMIT 1;

-- name: LockSemesterCurrentScope :exec
SELECT pg_advisory_xact_lock(sqlc.arg(lock_key)::bigint);

-- name: ClearCurrentSemesters :exec
UPDATE semester SET is_current = false WHERE is_current = true;

-- name: LockGradeCourseScope :exec
SELECT pg_advisory_xact_lock(hashtextextended('grade.course:' || $1::bigint::text || ':' || $2::bigint::text, 0));

-- name: LockGradeStudentScope :exec
SELECT pg_advisory_xact_lock(hashtextextended('grade.student:' || $1::bigint::text || ':' || $2::bigint::text, 0));

-- name: ApplyGradeEventRevision :one
INSERT INTO grade_update_cursor (tenant_id, course_id, student_id, revision)
VALUES ($1, $2, $3, $4)
ON CONFLICT (tenant_id, course_id, student_id) DO UPDATE
SET revision = EXCLUDED.revision
WHERE grade_update_cursor.revision < EXCLUDED.revision
RETURNING revision;

-- name: GetLatestGradeLockOutboxByCourse :one
SELECT id, revision, tenant_id, review_id, course_id, locked, reason, trace_id, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until
FROM grade_lock_outbox WHERE course_id = $1 ORDER BY revision DESC LIMIT 1;

-- name: HasAcceptedAppealsByCourse :one
SELECT EXISTS (SELECT 1 FROM grade_appeal WHERE course_id = $1 AND status = 2)::boolean;

-- name: AcceptGradeAppeal :one
UPDATE grade_appeal
SET status = 2, handler_id = $2, result_comment = $3, handled_at = now(), unlock_revision = $4
WHERE grade_appeal.id = $1 AND grade_appeal.status = 1 AND $4::bigint > 0
  AND EXISTS (SELECT 1 FROM grade_lock_outbox o WHERE o.revision = $4 AND o.tenant_id = grade_appeal.tenant_id AND o.course_id = grade_appeal.course_id AND NOT o.locked)
RETURNING grade_appeal.id, grade_appeal.tenant_id, grade_appeal.student_id, grade_appeal.course_id, grade_appeal.reason, grade_appeal.status, grade_appeal.handler_id, grade_appeal.result_comment, grade_appeal.created_at, grade_appeal.handled_at, grade_appeal.unlock_revision;

-- name: CreateGradeReview :one
INSERT INTO grade_review (id, tenant_id, course_id, semester_id, submitter_id, status, is_locked, comment, submitted_at)
VALUES ($1, $2, $3, $4, $5, 1, false, $6, now())
RETURNING id, tenant_id, course_id, semester_id, submitter_id, reviewer_id, status, is_locked, comment, submitted_at, reviewed_at;

-- name: GetGradeReview :one
SELECT id, tenant_id, course_id, semester_id, submitter_id, reviewer_id, status, is_locked, comment, submitted_at, reviewed_at
FROM grade_review
WHERE id = $1;

-- name: GetLatestApprovedReviewByCourse :one
SELECT id, tenant_id, course_id, semester_id, submitter_id, reviewer_id, status, is_locked, comment, submitted_at, reviewed_at
FROM grade_review
WHERE course_id = $1 AND status = 2
ORDER BY reviewed_at DESC
LIMIT 1;

-- name: GetLatestReviewByCourse :one
SELECT id, tenant_id, course_id, semester_id, submitter_id, reviewer_id, status, is_locked, comment, submitted_at, reviewed_at
FROM grade_review
WHERE course_id = $1
ORDER BY reviewed_at DESC NULLS LAST, submitted_at DESC
LIMIT 1;

-- name: ListAcceptedAppealsByCourseStudent :many
SELECT id, tenant_id, student_id, course_id, reason, status, handler_id, result_comment, created_at, handled_at, unlock_revision
FROM grade_appeal
WHERE course_id = $1 AND student_id = $2 AND status = 2
ORDER BY created_at DESC;

-- name: ListGradeReviews :many
SELECT id, tenant_id, course_id, semester_id, submitter_id, reviewer_id, status, is_locked, comment, submitted_at, reviewed_at
FROM grade_review
WHERE (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
  AND (sqlc.arg(is_locked)::smallint = 0 OR is_locked = (sqlc.arg(is_locked)::smallint = 1))
ORDER BY submitted_at DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountGradeReviews :one
SELECT count(*)::bigint
FROM grade_review
WHERE (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
  AND (sqlc.arg(is_locked)::smallint = 0 OR is_locked = (sqlc.arg(is_locked)::smallint = 1));

-- name: ListOwnGradeReviews :many
SELECT id, tenant_id, course_id, semester_id, submitter_id, reviewer_id, status, is_locked, comment, submitted_at, reviewed_at
FROM grade_review
WHERE submitter_id = sqlc.arg(submitter_id)
  AND (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
ORDER BY submitted_at DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountOwnGradeReviews :one
SELECT count(*)::bigint
FROM grade_review
WHERE submitter_id = sqlc.arg(submitter_id)
  AND (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint);

-- name: ApproveGradeReview :one
UPDATE grade_review
SET status = 2, is_locked = true, reviewer_id = $2, semester_id = $3, comment = $4, reviewed_at = now()
WHERE id = $1 AND status = 1
RETURNING id, tenant_id, course_id, semester_id, submitter_id, reviewer_id, status, is_locked, comment, submitted_at, reviewed_at;

-- name: RejectGradeReview :one
UPDATE grade_review
SET status = 3, is_locked = false, reviewer_id = $2, comment = $3, reviewed_at = now()
WHERE id = $1 AND status = 1
RETURNING id, tenant_id, course_id, semester_id, submitter_id, reviewer_id, status, is_locked, comment, submitted_at, reviewed_at;

-- name: UnlockGradeReview :one
UPDATE grade_review
SET status = 1, is_locked = false, reviewer_id = $2, comment = $3, reviewed_at = now()
WHERE id = $1 AND status = 2
RETURNING id, tenant_id, course_id, semester_id, submitter_id, reviewer_id, status, is_locked, comment, submitted_at, reviewed_at;

-- name: RelockGradeReview :one
UPDATE grade_review
SET status = 2, is_locked = true, reviewer_id = $2, comment = $3, reviewed_at = now()
WHERE id = $1 AND status = 1
RETURNING id, tenant_id, course_id, semester_id, submitter_id, reviewer_id, status, is_locked, comment, submitted_at, reviewed_at;

-- name: CreateGradeLockOutbox :one
INSERT INTO grade_lock_outbox (id, tenant_id, review_id, course_id, locked, reason, trace_id, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until)
VALUES ($1, $2, $3, $4, $5, $6, $7, 1, 0, NULL, now(), now(), '', NULL)
RETURNING id, revision, tenant_id, review_id, course_id, locked, reason, trace_id, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until;

-- name: ClaimPendingGradeLockOutbox :many
WITH exhausted AS (
    UPDATE grade_lock_outbox AS expired
    SET status = 4, last_error = 'grade lock lease expired after retry limit', updated_at = now(), lease_token = '', lease_until = NULL
    WHERE expired.status = 2 AND expired.lease_until <= @stale_before::timestamptz AND expired.retry_count >= @max_attempts
    RETURNING expired.id
), candidates AS (
    SELECT o.id
    FROM grade_lock_outbox o
    WHERE (o.status IN (1, 4) OR (o.status = 2 AND o.lease_until <= @stale_before::timestamptz))
      AND o.retry_count < @max_attempts
    ORDER BY o.created_at ASC, o.id ASC
    LIMIT @page_limit
    FOR UPDATE SKIP LOCKED
)
UPDATE grade_lock_outbox AS outbox
SET status = 2, retry_count = outbox.retry_count + 1, updated_at = now(), lease_token = @lease_token, lease_until = @lease_until::timestamptz
FROM candidates
WHERE outbox.id = candidates.id
RETURNING outbox.id, outbox.revision, outbox.tenant_id, outbox.review_id, outbox.course_id, outbox.locked, outbox.reason, outbox.trace_id, outbox.status, outbox.retry_count, outbox.last_error, outbox.created_at, outbox.updated_at, outbox.lease_token, outbox.lease_until;

-- name: RequeueExhaustedGradeLockOutbox :execrows
UPDATE grade_lock_outbox
SET status = 1,
    retry_count = 0,
    last_error = NULL,
    updated_at = now(),
    lease_token = '',
    lease_until = NULL
WHERE status = 4
  AND updated_at <= now() - ($1::bigint * interval '1 second');

-- name: MarkGradeLockOutboxPublished :one
UPDATE grade_lock_outbox
SET status = 3, last_error = NULL, updated_at = now(), lease_token = '', lease_until = NULL
WHERE tenant_id = $1 AND id = $2 AND status = 2 AND lease_token = $3 AND lease_until > now()
RETURNING id, revision, tenant_id, review_id, course_id, locked, reason, trace_id, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until;

-- name: MarkGradeLockOutboxFailed :one
UPDATE grade_lock_outbox
SET status = 4, last_error = $3, updated_at = now(), lease_token = '', lease_until = NULL
WHERE tenant_id = $1 AND id = $2 AND status = 2 AND lease_token = $4 AND lease_until > now()
RETURNING id, revision, tenant_id, review_id, course_id, locked, reason, trace_id, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until;

-- name: UpsertStudentSemesterGrade :one
INSERT INTO student_semester_grade (id, tenant_id, student_id, semester_id, total_credits, gpa, cumulative_gpa, computed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (tenant_id, student_id, semester_id)
DO UPDATE SET total_credits = EXCLUDED.total_credits, gpa = EXCLUDED.gpa, cumulative_gpa = EXCLUDED.cumulative_gpa, computed_at = now()
RETURNING id, tenant_id, student_id, semester_id, total_credits, gpa, cumulative_gpa, computed_at;

-- name: ListStudentSemesterGrades :many
SELECT id, tenant_id, student_id, semester_id, total_credits, gpa, cumulative_gpa, computed_at
FROM student_semester_grade
WHERE student_id = $1
ORDER BY computed_at DESC;

-- name: ListKnownStudentSemesterGrades :many
SELECT id, tenant_id, student_id, semester_id, total_credits, gpa, cumulative_gpa, computed_at
FROM student_semester_grade
WHERE (sqlc.arg(student_id)::bigint = 0 OR student_id = sqlc.arg(student_id)::bigint)
ORDER BY student_id ASC, semester_id ASC;

-- name: CreateGradeAppeal :one
INSERT INTO grade_appeal (id, tenant_id, student_id, course_id, reason, status, created_at)
VALUES ($1, $2, $3, $4, $5, 1, now())
RETURNING id, tenant_id, student_id, course_id, reason, status, handler_id, result_comment, created_at, handled_at, unlock_revision;

-- name: HasOpenGradeAppeal :one
SELECT EXISTS (
    SELECT 1
    FROM grade_appeal
    WHERE course_id = $1 AND student_id = $2 AND status IN (1, 2)
)::boolean;

-- name: GetGradeAppeal :one
SELECT id, tenant_id, student_id, course_id, reason, status, handler_id, result_comment, created_at, handled_at, unlock_revision
FROM grade_appeal
WHERE id = $1;

-- name: ListGradeAppeals :many
SELECT id, tenant_id, student_id, course_id, reason, status, handler_id, result_comment, created_at, handled_at, unlock_revision
FROM grade_appeal
WHERE (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
  AND (sqlc.arg(all_courses)::boolean OR course_id = ANY(sqlc.arg(course_ids)::bigint[]))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountGradeAppeals :one
SELECT count(*)::bigint
FROM grade_appeal
WHERE (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
  AND (sqlc.arg(all_courses)::boolean OR course_id = ANY(sqlc.arg(course_ids)::bigint[]));

-- name: UpdateGradeAppealStatus :one
UPDATE grade_appeal
SET status = sqlc.arg(to_status)::smallint,
    handler_id = sqlc.arg(handler_id)::bigint,
    result_comment = sqlc.arg(result_comment)::text,
    handled_at = now()
WHERE id = sqlc.arg(id)::bigint AND status = sqlc.arg(from_status)::smallint
RETURNING id, tenant_id, student_id, course_id, reason, status, handler_id, result_comment, created_at, handled_at, unlock_revision;

-- name: CreateAcademicWarning :one
INSERT INTO academic_warning (id, tenant_id, student_id, semester_id, type, detail, status, created_at, notify_trace_id)
VALUES ($1, $2, $3, $4, $5, $6, 1, now(), $7)
RETURNING id, tenant_id, student_id, semester_id, type, detail, status, created_at, notify_trace_id, notify_status, notify_attempts, notify_lease_token, notify_lease_until, notify_next_attempt_at, notify_last_error;

-- name: ListAcademicWarnings :many
SELECT id, tenant_id, student_id, semester_id, type, detail, status, created_at, notify_trace_id, notify_status, notify_attempts, notify_lease_token, notify_lease_until, notify_next_attempt_at, notify_last_error
FROM academic_warning
WHERE (sqlc.arg(student_id)::bigint = 0 OR student_id = sqlc.arg(student_id)::bigint)
  AND (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
ORDER BY created_at DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountAcademicWarnings :one
SELECT count(*)::bigint
FROM academic_warning
WHERE (sqlc.arg(student_id)::bigint = 0 OR student_id = sqlc.arg(student_id)::bigint)
  AND (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint);

-- name: AckAcademicWarning :one
UPDATE academic_warning
SET status = 2
WHERE id = $1 AND student_id = $2
RETURNING id, tenant_id, student_id, semester_id, type, detail, status, created_at, notify_trace_id, notify_status, notify_attempts, notify_lease_token, notify_lease_until, notify_next_attempt_at, notify_last_error;

-- name: CreateTranscriptRecord :one
INSERT INTO transcript_record (id, tenant_id, student_id, scope, semester_id, pdf_ref, generated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
RETURNING id, tenant_id, student_id, scope, semester_id, pdf_ref, generated_at;

-- name: GetTranscriptRecord :one
SELECT id, tenant_id, student_id, scope, semester_id, pdf_ref, generated_at
FROM transcript_record
WHERE id = $1;

-- name: ListTranscriptRecords :many
SELECT id, tenant_id, student_id, scope, semester_id, pdf_ref, generated_at
FROM transcript_record
WHERE student_id = $1
ORDER BY generated_at DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: ClaimGradeWarningNotifications :many
WITH candidates AS (
 SELECT id FROM academic_warning
 WHERE (notify_status IN (1,4) AND notify_next_attempt_at <= sqlc.arg(now_at)::timestamptz)
    OR (notify_status = 2 AND notify_lease_until <= sqlc.arg(now_at)::timestamptz)
 ORDER BY created_at, id LIMIT sqlc.arg(page_limit)::int FOR UPDATE SKIP LOCKED
)
UPDATE academic_warning AS w
SET notify_status=2, notify_attempts=w.notify_attempts+1,
    notify_lease_token=sqlc.arg(lease_token), notify_lease_until=sqlc.arg(lease_until)::timestamptz
FROM candidates WHERE w.id=candidates.id
RETURNING w.id, w.tenant_id, w.student_id, w.semester_id, w.created_at, w.notify_trace_id, w.notify_lease_token, COALESCE(w.detail->>'gpa', '')::text AS gpa_text;

-- name: MarkGradeWarningNotificationPublished :execrows
UPDATE academic_warning
SET notify_status=3, notify_last_error=NULL, notify_lease_token='', notify_lease_until=NULL
WHERE tenant_id=$1 AND id=$2 AND notify_status=2 AND notify_lease_token=$3 AND notify_lease_until > now();

-- name: MarkGradeWarningNotificationFailed :execrows
UPDATE academic_warning
SET notify_status=4, notify_last_error=$4, notify_lease_token='', notify_lease_until=NULL,
    notify_next_attempt_at=sqlc.arg(next_at)::timestamptz
WHERE tenant_id=$1 AND id=$2 AND notify_status=2 AND notify_lease_token=$3 AND notify_lease_until > now();
