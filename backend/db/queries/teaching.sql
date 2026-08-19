-- name: CreateCourse :one
INSERT INTO course (id, tenant_id, teacher_id, name, description, type, difficulty, cover_ref, semester, credits, schedule, start_at, end_at, invite_code, status, visibility, created_at, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::text::numeric, $11, $12, $13, $14, $15, $16, now(), now(), NULL)
RETURNING id, tenant_id, teacher_id, name, description, type, difficulty, cover_ref, semester, credits::float8 AS credits, schedule, start_at, end_at, invite_code, status, visibility, created_at, updated_at, deleted_at;

-- name: GetCourseByID :one
SELECT id, tenant_id, teacher_id, name, description, type, difficulty, cover_ref, semester, credits::float8 AS credits, schedule, start_at, end_at, invite_code, status, visibility, created_at, updated_at, deleted_at
FROM course
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: GetCloneableCourseByID :one
SELECT id, tenant_id, teacher_id, name, description, type, difficulty, cover_ref, semester, credits::float8 AS credits, schedule, start_at, end_at, invite_code, status, visibility, created_at, updated_at, deleted_at
FROM course
WHERE id = $1 AND deleted_at IS NULL AND (tenant_id = $2 OR visibility = 2);

-- name: GetCourseByInviteCode :one
SELECT id, tenant_id, teacher_id, name, description, type, difficulty, cover_ref, semester, credits::float8 AS credits, schedule, start_at, end_at, invite_code, status, visibility, created_at, updated_at, deleted_at
FROM course
WHERE invite_code = $1 AND deleted_at IS NULL;

-- name: ListTeacherCourses :many
SELECT id, tenant_id, teacher_id, name, description, type, difficulty, cover_ref, semester, credits::float8 AS credits, schedule, start_at, end_at, invite_code, status, visibility, created_at, updated_at, deleted_at
FROM course
WHERE tenant_id = $1 AND teacher_id = $2 AND deleted_at IS NULL AND ($3::smallint = 0 OR status = $3)
ORDER BY updated_at DESC, id DESC
LIMIT $4 OFFSET $5;

-- name: CountTeacherCourses :one
SELECT COUNT(*)::bigint
FROM course
WHERE tenant_id = $1 AND teacher_id = $2 AND deleted_at IS NULL AND ($3::smallint = 0 OR status = $3);

-- name: ListStudentCourses :many
SELECT c.id, c.tenant_id, c.teacher_id, c.name, c.description, c.type, c.difficulty, c.cover_ref, c.semester, c.credits::float8 AS credits, c.schedule, c.start_at, c.end_at, c.invite_code, c.status, c.visibility, c.created_at, c.updated_at, c.deleted_at
FROM course c
JOIN course_member m ON m.tenant_id = c.tenant_id AND m.course_id = c.id
WHERE c.tenant_id = $1 AND m.student_id = $2 AND c.deleted_at IS NULL AND ($3::smallint = 0 OR c.status = $3)
ORDER BY c.updated_at DESC, c.id DESC
LIMIT $4 OFFSET $5;

-- name: CountStudentCourses :one
SELECT COUNT(*)::bigint
FROM course c
JOIN course_member m ON m.tenant_id = c.tenant_id AND m.course_id = c.id
WHERE c.tenant_id = $1 AND m.student_id = $2 AND c.deleted_at IS NULL AND ($3::smallint = 0 OR c.status = $3);

-- name: UpdateCourse :one
UPDATE course
SET name = $3,
    description = $4,
    type = $5,
    difficulty = $6,
    cover_ref = $7,
    semester = $8,
    credits = $9::text::numeric,
    schedule = $10,
    start_at = $11,
    end_at = $12,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, teacher_id, name, description, type, difficulty, cover_ref, semester, credits::float8 AS credits, schedule, start_at, end_at, invite_code, status, visibility, created_at, updated_at, deleted_at;

-- name: SetCourseStatus :one
UPDATE course
SET status = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, teacher_id, name, description, type, difficulty, cover_ref, semester, credits::float8 AS credits, schedule, start_at, end_at, invite_code, status, visibility, created_at, updated_at, deleted_at;

-- name: SetCourseVisibility :one
UPDATE course
SET visibility = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, teacher_id, name, description, type, difficulty, cover_ref, semester, credits::float8 AS credits, schedule, start_at, end_at, invite_code, status, visibility, created_at, updated_at, deleted_at;

-- name: RefreshCourseInviteCode :one
UPDATE course
SET invite_code = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, teacher_id, name, description, type, difficulty, cover_ref, semester, credits::float8 AS credits, schedule, start_at, end_at, invite_code, status, visibility, created_at, updated_at, deleted_at;

-- name: CountCourseLessons :one
SELECT COUNT(*)::bigint
FROM lesson l
JOIN chapter c ON c.tenant_id = l.tenant_id AND c.id = l.chapter_id
WHERE c.tenant_id = $1 AND c.course_id = $2 AND c.deleted_at IS NULL AND l.deleted_at IS NULL;

-- name: ListCoursesDueToRun :many
SELECT id, tenant_id, teacher_id, name, description, type, difficulty, cover_ref, semester, credits::float8 AS credits, schedule, start_at, end_at, invite_code, status, visibility, created_at, updated_at, deleted_at
FROM course
WHERE deleted_at IS NULL AND status = 2 AND start_at <= $1
ORDER BY start_at ASC, id ASC;

-- name: ListCoursesDueToEnd :many
SELECT id, tenant_id, teacher_id, name, description, type, difficulty, cover_ref, semester, credits::float8 AS credits, schedule, start_at, end_at, invite_code, status, visibility, created_at, updated_at, deleted_at
FROM course
WHERE deleted_at IS NULL AND status IN (2, 3) AND end_at <= $1
ORDER BY end_at ASC, id ASC;

-- name: SoftDeleteCourse :one
UPDATE course
SET deleted_at = now(), updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 1 AND deleted_at IS NULL
RETURNING id, tenant_id, teacher_id, name, description, type, difficulty, cover_ref, semester, credits::float8 AS credits, schedule, start_at, end_at, invite_code, status, visibility, created_at, updated_at, deleted_at;

-- name: CreateChapter :one
INSERT INTO chapter (id, tenant_id, course_id, title, sort, created_at, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, now(), now(), NULL)
RETURNING id, tenant_id, course_id, title, sort, created_at, updated_at, deleted_at;

-- name: GetChapter :one
SELECT id, tenant_id, course_id, title, sort, created_at, updated_at, deleted_at
FROM chapter
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: ListChapters :many
SELECT id, tenant_id, course_id, title, sort, created_at, updated_at, deleted_at
FROM chapter
WHERE tenant_id = $1 AND course_id = $2 AND deleted_at IS NULL
ORDER BY sort ASC, id ASC;

-- name: UpdateChapter :one
UPDATE chapter
SET title = $3, sort = $4, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, title, sort, created_at, updated_at, deleted_at;

-- name: SoftDeleteChapter :one
UPDATE chapter
SET deleted_at = now(), updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, title, sort, created_at, updated_at, deleted_at;

-- name: CreateLesson :one
INSERT INTO lesson (id, tenant_id, chapter_id, title, content_type, content_ref, sort, created_at, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now(), NULL)
RETURNING id, tenant_id, chapter_id, title, content_type, content_ref, sort, created_at, updated_at, deleted_at;

-- name: GetLesson :one
SELECT id, tenant_id, chapter_id, title, content_type, content_ref, sort, created_at, updated_at, deleted_at
FROM lesson
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: ListLessonsByChapter :many
SELECT id, tenant_id, chapter_id, title, content_type, content_ref, sort, created_at, updated_at, deleted_at
FROM lesson
WHERE tenant_id = $1 AND chapter_id = $2 AND deleted_at IS NULL
ORDER BY sort ASC, id ASC;

-- name: ListLessonsByCourse :many
SELECT l.id, l.tenant_id, l.chapter_id, l.title, l.content_type, l.content_ref, l.sort, l.created_at, l.updated_at, l.deleted_at
FROM lesson l
JOIN chapter c ON c.tenant_id = l.tenant_id AND c.id = l.chapter_id
WHERE c.tenant_id = $1 AND c.course_id = $2 AND c.deleted_at IS NULL AND l.deleted_at IS NULL
ORDER BY c.sort ASC, l.sort ASC, l.id ASC;

-- name: UpdateLesson :one
UPDATE lesson
SET title = $3, content_type = $4, content_ref = $5, sort = $6, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, chapter_id, title, content_type, content_ref, sort, created_at, updated_at, deleted_at;

-- name: SoftDeleteLesson :one
UPDATE lesson
SET deleted_at = now(), updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, chapter_id, title, content_type, content_ref, sort, created_at, updated_at, deleted_at;

-- name: CreateCourseMember :one
INSERT INTO course_member (id, tenant_id, course_id, student_id, joined_at, join_mode)
VALUES ($1, $2, $3, $4, now(), $5)
ON CONFLICT (tenant_id, course_id, student_id) DO UPDATE SET joined_at = course_member.joined_at
RETURNING id, tenant_id, course_id, student_id, joined_at, join_mode;

-- name: GetCourseMember :one
SELECT id, tenant_id, course_id, student_id, joined_at, join_mode
FROM course_member
WHERE tenant_id = $1 AND course_id = $2 AND student_id = $3;

-- name: ListCourseMembers :many
SELECT id, tenant_id, course_id, student_id, joined_at, join_mode
FROM course_member
WHERE tenant_id = $1 AND course_id = $2
ORDER BY joined_at DESC, id DESC
LIMIT $3 OFFSET $4;

-- name: CountCourseMembers :one
SELECT COUNT(*)::bigint FROM course_member WHERE tenant_id = $1 AND course_id = $2;

-- name: DeleteCourseMember :exec
DELETE FROM course_member WHERE tenant_id = $1 AND course_id = $2 AND student_id = $3;

-- name: CreateAssignment :one
INSERT INTO assignment (id, tenant_id, course_id, title, chapter_id, due_at, max_attempts, late_policy, late_penalty, status, created_at, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now(), now(), NULL)
RETURNING id, tenant_id, course_id, title, chapter_id, due_at, max_attempts, late_policy, late_penalty, status, created_at, updated_at, deleted_at;

-- name: GetAssignment :one
SELECT id, tenant_id, course_id, title, chapter_id, due_at, max_attempts, late_policy, late_penalty, status, created_at, updated_at, deleted_at
FROM assignment
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: ListAssignmentsByCourse :many
-- published_only=true 只回已发布作业(学生视角),false 回全部含草稿(授课教师与课程克隆)。
-- 只返回作业外壳,题目与题面仍只经 GetAssignment 走 M5 题面接口。
SELECT id, tenant_id, course_id, title, chapter_id, due_at, max_attempts, late_policy, late_penalty, status, created_at, updated_at, deleted_at
FROM assignment
WHERE tenant_id = $1
  AND course_id = $2
  AND deleted_at IS NULL
  AND (sqlc.arg(published_only)::boolean = false OR status = 2)
ORDER BY due_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountAssignmentsByCourse :one
SELECT count(*)
FROM assignment
WHERE tenant_id = $1
  AND course_id = $2
  AND deleted_at IS NULL
  AND (sqlc.arg(published_only)::boolean = false OR status = 2);

-- name: UpdateAssignment :one
UPDATE assignment
SET title = $3,
    chapter_id = $4,
    due_at = $5,
    max_attempts = $6,
    late_policy = $7,
    late_penalty = $8,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 1 AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, title, chapter_id, due_at, max_attempts, late_policy, late_penalty, status, created_at, updated_at, deleted_at;

-- name: PublishAssignment :one
UPDATE assignment
SET status = 2, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 1 AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, title, chapter_id, due_at, max_attempts, late_policy, late_penalty, status, created_at, updated_at, deleted_at;

-- name: DeleteAssignmentItems :exec
DELETE FROM assignment_item WHERE tenant_id = $1 AND assignment_id = $2;

-- name: CreateAssignmentItem :one
INSERT INTO assignment_item (id, tenant_id, assignment_id, item_code, item_version, score, seq, grading_mode, judger_code, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
RETURNING id, tenant_id, assignment_id, item_code, item_version, score, seq, grading_mode, judger_code, created_at;

-- name: ListAssignmentItems :many
SELECT id, tenant_id, assignment_id, item_code, item_version, score, seq, grading_mode, judger_code, created_at
FROM assignment_item
WHERE tenant_id = $1 AND assignment_id = $2
ORDER BY seq ASC;

-- name: CountAssignmentSubmissions :one
SELECT COUNT(*)::bigint FROM submission WHERE tenant_id = $1 AND assignment_id = $2;

-- name: CountStudentAttempts :one
SELECT COUNT(*)::bigint FROM submission WHERE tenant_id = $1 AND assignment_id = $2 AND student_id = $3;

-- name: CreateSubmission :one
INSERT INTO submission (id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at)
VALUES ($1, $2, $3, $4, $5, $6, NULL, NULL, NULL, $7, NULL, $8, $9, now())
RETURNING id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at;

-- name: GetSubmission :one
SELECT id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at
FROM submission
WHERE tenant_id = $1 AND id = $2;

-- name: GetSubmissionBySourceRef :one
SELECT s.id, s.tenant_id, s.assignment_id, s.student_id, s.attempt_no, s.content_ref, s.judge_task_ref, s.auto_score, s.manual_score, s.final_score, s.comment, s.is_late, s.status, s.submitted_at
FROM submission s
JOIN submission_judge_outbox o ON o.tenant_id = s.tenant_id AND o.submission_id = s.id
WHERE o.tenant_id = $1 AND o.source_ref = $2;

-- name: ListJudgeOutboxBySubmission :many
SELECT id, tenant_id, submission_id, assignment_item_id, assignment_id, source_owner_id, source_course_id, source_scope, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, score, completed_at, created_at, updated_at, lease_token, lease_until
FROM submission_judge_outbox
WHERE tenant_id = $1 AND submission_id = $2
ORDER BY id ASC;

-- name: ListSubmissionsByAssignment :many
-- student_id 传 0 回该作业全部学生提交(授课教师批改视角);传具体学生只回其本人提交
-- (学生自读视角,student_id 由服务端填会话账号,不接受客户端传参)。
-- status 传 0 不按状态过滤;传具体状态供批改台按「待批改/已出分」取数与计数。
SELECT id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at
FROM submission
WHERE tenant_id = $1
  AND assignment_id = $2
  AND (sqlc.arg(student_id)::bigint = 0 OR submission.student_id = sqlc.arg(student_id)::bigint)
  AND (sqlc.arg(status)::smallint = 0 OR submission.status = sqlc.arg(status)::smallint)
ORDER BY submitted_at DESC, id DESC
LIMIT $3 OFFSET $4;

-- name: CountSubmissionsByAssignment :one
SELECT COUNT(*)::bigint
FROM submission
WHERE tenant_id = $1
  AND assignment_id = $2
  AND (sqlc.arg(student_id)::bigint = 0 OR submission.student_id = sqlc.arg(student_id)::bigint)
  AND (sqlc.arg(status)::smallint = 0 OR submission.status = sqlc.arg(status)::smallint);

-- name: UpdateSubmissionManualGrade :one
UPDATE submission
SET manual_score = $3, final_score = $4, comment = $5, status = 3
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at;

-- name: UpdateSubmissionJudgeRef :one
UPDATE submission
SET judge_task_ref = $3
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at;

-- name: UpdateSubmissionAutoScore :one
UPDATE submission
SET auto_score = $3, final_score = $4, status = 3
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at;

-- name: CreateJudgeOutbox :one
INSERT INTO submission_judge_outbox (id, tenant_id, submission_id, assignment_item_id, assignment_id, source_owner_id, source_course_id, source_scope, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, score, completed_at, created_at, updated_at, lease_token, lease_until)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, 1, 0, NULL, NULL, NULL, now(), now(), '', NULL)
RETURNING id, tenant_id, submission_id, assignment_item_id, assignment_id, source_owner_id, source_course_id, source_scope, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, score, completed_at, created_at, updated_at, lease_token, lease_until;

-- name: ClaimJudgeOutbox :many
UPDATE submission_judge_outbox
SET status = 2, retry_count = retry_count + 1, updated_at = now(), lease_token = @lease_token, lease_until = @lease_until::timestamptz
WHERE id IN (
    SELECT o.id FROM submission_judge_outbox o
    WHERE o.tenant_id = $1
      AND (o.status = 1 OR (o.status = 2 AND o.lease_until <= @stale_before::timestamptz))
      AND o.retry_count < @max_attempts
    ORDER BY o.created_at ASC
    LIMIT $2
    FOR UPDATE SKIP LOCKED
)
RETURNING id, tenant_id, submission_id, assignment_item_id, assignment_id, source_owner_id, source_course_id, source_scope, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, score, completed_at, created_at, updated_at, lease_token, lease_until;

-- name: ClaimJudgeOutboxAcrossTenants :many
UPDATE submission_judge_outbox
SET status = 2, retry_count = retry_count + 1, updated_at = now(), lease_token = @lease_token, lease_until = @lease_until::timestamptz
WHERE id IN (
    SELECT o.id FROM submission_judge_outbox o
    WHERE (o.status = 1 OR (o.status = 2 AND o.lease_until <= @stale_before::timestamptz))
      AND o.retry_count < @max_attempts
    ORDER BY o.created_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, tenant_id, submission_id, assignment_item_id, assignment_id, source_owner_id, source_course_id, source_scope, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, score, completed_at, created_at, updated_at, lease_token, lease_until;

-- name: CompleteJudgeOutbox :one
UPDATE submission_judge_outbox
SET status = 3,
    last_error = CASE WHEN status = 2 THEN NULL ELSE last_error END,
    updated_at = now(),
    lease_token = '',
    lease_until = NULL
WHERE (tenant_id = $1 AND id = $2 AND status = 2 AND lease_token = $3 AND lease_until > now())
   OR (tenant_id = $1 AND id = $2 AND status = 3)
RETURNING id, tenant_id, submission_id, assignment_item_id, assignment_id, source_owner_id, source_course_id, source_scope, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, score, completed_at, created_at, updated_at, lease_token, lease_until;

-- name: RetryJudgeOutbox :one
UPDATE submission_judge_outbox
SET status = CASE WHEN retry_count >= @max_attempts THEN 4 ELSE 1 END,
    last_error = $3, updated_at = now(), lease_token = '', lease_until = NULL
WHERE tenant_id = $1 AND id = $2 AND status = 2 AND lease_token = $4 AND lease_until > now()
RETURNING id, tenant_id, submission_id, assignment_item_id, assignment_id, source_owner_id, source_course_id, source_scope, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, score, completed_at, created_at, updated_at, lease_token, lease_until;

-- name: ExhaustExpiredJudgeOutbox :execrows
UPDATE submission_judge_outbox
SET status = 4,
    last_error = 'dispatch lease expired after retry limit',
    updated_at = now(),
    lease_token = '',
    lease_until = NULL
WHERE tenant_id = $1
  AND status = 2
  AND lease_until <= @stale_before::timestamptz
  AND retry_count >= @max_attempts;

-- name: ExhaustExpiredJudgeOutboxAcrossTenants :execrows
UPDATE submission_judge_outbox
SET status = 4,
    last_error = 'dispatch lease expired after retry limit',
    updated_at = now(),
    lease_token = '',
    lease_until = NULL
WHERE status = 2
  AND lease_until <= @stale_before::timestamptz
  AND retry_count >= @max_attempts;

-- name: MarkJudgeOutboxResult :one
UPDATE submission_judge_outbox
SET score = $3, last_error = NULL, completed_at = $4, status = 3, updated_at = now(), lease_token = '', lease_until = NULL
WHERE tenant_id = $1 AND source_ref = $2 AND status IN (2, 3, 4)
  AND (completed_at IS NULL OR (completed_at = $4 AND score = $3))
RETURNING id, tenant_id, submission_id, assignment_item_id, assignment_id, source_owner_id, source_course_id, source_scope, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, score, completed_at, created_at, updated_at, lease_token, lease_until;

-- name: MarkJudgeOutboxFailedResult :one
UPDATE submission_judge_outbox
SET last_error = $3, completed_at = $4, status = 3, updated_at = now(), lease_token = '', lease_until = NULL
WHERE tenant_id = $1 AND source_ref = $2 AND status IN (2, 3, 4)
  AND (completed_at IS NULL OR (completed_at = $4 AND last_error = $3))
RETURNING id, tenant_id, submission_id, assignment_item_id, assignment_id, source_owner_id, source_course_id, source_scope, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, score, completed_at, created_at, updated_at, lease_token, lease_until;

-- name: UpsertSubmissionDraft :one
INSERT INTO submission_draft (id, tenant_id, assignment_id, student_id, content, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (tenant_id, assignment_id, student_id) DO UPDATE SET content = EXCLUDED.content, updated_at = now()
RETURNING id, tenant_id, assignment_id, student_id, content, updated_at;

-- name: GetSubmissionDraft :one
SELECT id, tenant_id, assignment_id, student_id, content, updated_at
FROM submission_draft
WHERE tenant_id = $1 AND assignment_id = $2 AND student_id = $3;

-- name: DeleteSubmissionDraft :exec
DELETE FROM submission_draft WHERE tenant_id = $1 AND assignment_id = $2 AND student_id = $3;

-- name: UpsertLessonProgress :one
INSERT INTO lesson_progress (id, tenant_id, lesson_id, student_id, status, video_pos, duration_sec, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (tenant_id, lesson_id, student_id) DO UPDATE
SET status = GREATEST(lesson_progress.status, EXCLUDED.status),
    video_pos = EXCLUDED.video_pos,
    duration_sec = lesson_progress.duration_sec + EXCLUDED.duration_sec,
    updated_at = now()
RETURNING id, tenant_id, lesson_id, student_id, status, video_pos, duration_sec, updated_at;

-- name: GetLessonProgress :one
SELECT id, tenant_id, lesson_id, student_id, status, video_pos, duration_sec, updated_at
FROM lesson_progress
WHERE tenant_id = $1 AND lesson_id = $2 AND student_id = $3;

-- name: ListProgressByCourse :many
SELECT p.id, p.tenant_id, p.lesson_id, p.student_id, p.status, p.video_pos, p.duration_sec, p.updated_at
FROM lesson_progress p
JOIN lesson l ON l.tenant_id = p.tenant_id AND l.id = p.lesson_id
JOIN chapter c ON c.tenant_id = l.tenant_id AND c.id = l.chapter_id
WHERE p.tenant_id = $1 AND c.course_id = $2 AND c.deleted_at IS NULL AND l.deleted_at IS NULL;

-- name: ListStudentProgressByCourse :many
SELECT p.id, p.tenant_id, p.lesson_id, p.student_id, p.status, p.video_pos, p.duration_sec, p.updated_at
FROM lesson_progress p
JOIN lesson l ON l.tenant_id = p.tenant_id AND l.id = p.lesson_id
JOIN chapter c ON c.tenant_id = l.tenant_id AND c.id = l.chapter_id
WHERE p.tenant_id = $1 AND c.course_id = $2 AND p.student_id = $3 AND c.deleted_at IS NULL AND l.deleted_at IS NULL;

-- name: CreateDiscussionPost :one
INSERT INTO discussion_post (id, tenant_id, course_id, parent_id, author_id, content, is_pinned, like_count, created_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, false, 0, now(), NULL)
RETURNING id, tenant_id, course_id, parent_id, author_id, content, is_pinned, like_count, created_at, deleted_at;

-- name: ListDiscussionPosts :many
SELECT id, tenant_id, course_id, parent_id, author_id, content, is_pinned, like_count, created_at, deleted_at
FROM discussion_post
WHERE tenant_id = $1 AND course_id = $2 AND deleted_at IS NULL
ORDER BY is_pinned DESC, created_at DESC, id DESC
LIMIT $3 OFFSET $4;

-- name: CountDiscussionPosts :one
SELECT count(*)::bigint
FROM discussion_post
WHERE tenant_id = $1 AND course_id = $2 AND deleted_at IS NULL;

-- name: GetDiscussionPost :one
SELECT id, tenant_id, course_id, parent_id, author_id, content, is_pinned, like_count, created_at, deleted_at
FROM discussion_post
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: LikeDiscussionPost :one
UPDATE discussion_post
SET like_count = like_count + 1
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, parent_id, author_id, content, is_pinned, like_count, created_at, deleted_at;

-- name: PinDiscussionPost :one
UPDATE discussion_post
SET is_pinned = $3
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, parent_id, author_id, content, is_pinned, like_count, created_at, deleted_at;

-- name: SoftDeleteDiscussionPost :one
UPDATE discussion_post
SET deleted_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, parent_id, author_id, content, is_pinned, like_count, created_at, deleted_at;

-- name: CreateAnnouncement :one
INSERT INTO announcement (id, tenant_id, course_id, title, content, is_pinned, created_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, now(), NULL)
RETURNING id, tenant_id, course_id, title, content, is_pinned, created_at, deleted_at;

-- name: ListAnnouncements :many
SELECT id, tenant_id, course_id, title, content, is_pinned, created_at, deleted_at
FROM announcement
WHERE tenant_id = $1 AND course_id = $2 AND deleted_at IS NULL
ORDER BY is_pinned DESC, created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountCourseAnnouncements :one
SELECT count(*)
FROM announcement
WHERE tenant_id = $1 AND course_id = $2 AND deleted_at IS NULL;

-- name: PinAnnouncement :one
UPDATE announcement
SET is_pinned = $3
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, title, content, is_pinned, created_at, deleted_at;

-- name: UpsertCourseReview :one
INSERT INTO course_review (id, tenant_id, course_id, student_id, rating, comment, created_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (tenant_id, course_id, student_id) DO UPDATE SET rating = EXCLUDED.rating, comment = EXCLUDED.comment, created_at = now()
RETURNING id, tenant_id, course_id, student_id, rating, comment, created_at;

-- name: DeleteGradeWeights :exec
DELETE FROM grade_weight WHERE tenant_id = $1 AND course_id = $2;

-- name: CreateGradeWeight :one
INSERT INTO grade_weight (id, tenant_id, course_id, source_type, source_ref, weight, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6::text::numeric, now(), now())
RETURNING id, tenant_id, course_id, source_type, source_ref, weight::float8 AS weight, created_at, updated_at;

-- name: ListGradeWeights :many
SELECT id, tenant_id, course_id, source_type, source_ref, weight::float8 AS weight, created_at, updated_at
FROM grade_weight
WHERE tenant_id = $1 AND course_id = $2
ORDER BY source_type ASC, source_ref ASC;

-- name: UpsertCourseGrade :one
INSERT INTO course_grade (id, tenant_id, course_id, student_id, auto_total, override_total, is_overridden, is_locked, updated_at)
VALUES ($1, $2, $3, $4, $5::text::numeric, NULL, false, false, now())
ON CONFLICT (tenant_id, course_id, student_id) DO UPDATE
SET auto_total = EXCLUDED.auto_total,
    updated_at = now()
WHERE course_grade.is_locked = false
RETURNING id, tenant_id, course_id, student_id, auto_total::float8 AS auto_total, COALESCE(override_total::float8, 0)::float8 AS override_total, is_overridden, is_locked, updated_at;

-- name: GetCourseGrade :one
SELECT id, tenant_id, course_id, student_id, auto_total::float8 AS auto_total, COALESCE(override_total::float8, 0)::float8 AS override_total, is_overridden, is_locked, updated_at
FROM course_grade
WHERE tenant_id = $1 AND course_id = $2 AND student_id = $3;

-- name: ListCourseGrades :many
SELECT g.id, g.tenant_id, g.course_id, c.semester, g.student_id, g.auto_total::float8 AS auto_total, COALESCE(g.override_total::float8, 0)::float8 AS override_total, g.is_overridden, g.is_locked, g.updated_at, c.credits::float8 AS credits
FROM course_grade g
JOIN course c ON c.tenant_id = g.tenant_id AND c.id = g.course_id
WHERE g.tenant_id = $1 AND g.course_id = $2
ORDER BY g.student_id ASC
LIMIT $3 OFFSET $4;

-- name: CountCourseGrades :one
SELECT count(*)
FROM course_grade
WHERE tenant_id = $1 AND course_id = $2;

-- name: ListStudentGrades :many
SELECT g.id, g.tenant_id, g.course_id, c.semester, g.student_id, g.auto_total::float8 AS auto_total, COALESCE(g.override_total::float8, 0)::float8 AS override_total, g.is_overridden, g.is_locked, g.updated_at, c.credits::float8 AS credits
FROM course_grade g
JOIN course c ON c.tenant_id = g.tenant_id AND c.id = g.course_id
WHERE g.tenant_id = $1 AND g.student_id = $2 AND (sqlc.arg(semester)::text = '' OR c.semester = sqlc.arg(semester)::text)
ORDER BY c.semester DESC, g.course_id ASC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountStudentGrades :one
SELECT count(*)
FROM course_grade g
JOIN course c ON c.tenant_id = g.tenant_id AND c.id = g.course_id
WHERE g.tenant_id = $1 AND g.student_id = $2 AND (sqlc.arg(semester)::text = '' OR c.semester = sqlc.arg(semester)::text);

-- name: OverrideCourseGrade :one
UPDATE course_grade
SET override_total = $4::text::numeric, is_overridden = true, updated_at = now()
WHERE tenant_id = $1 AND course_id = $2 AND student_id = $3 AND is_locked = false
RETURNING id, tenant_id, course_id, student_id, auto_total::float8 AS auto_total, COALESCE(override_total::float8, 0)::float8 AS override_total, is_overridden, is_locked, updated_at;

-- name: SetCourseGradesLock :exec
UPDATE course_grade
SET is_locked = $3, updated_at = now()
WHERE tenant_id = $1 AND course_id = $2;

-- name: CreateTeachingGradeEventOutbox :one
INSERT INTO teaching_grade_event_outbox (id, tenant_id, course_id, student_id, trace_id, event_updated_at, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until)
VALUES ($1, $2, $3, $4, $5, $6, 1, 0, NULL, now(), now(), '', NULL)
RETURNING id, tenant_id, course_id, student_id, trace_id, event_updated_at, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until;

-- name: ClaimPendingTeachingGradeEventOutbox :many
WITH exhausted AS (
    UPDATE teaching_grade_event_outbox AS expired
    SET status = 4, last_error = 'grade event lease expired after retry limit', updated_at = now(), lease_token = '', lease_until = NULL
    WHERE expired.status = 2 AND expired.lease_until <= @stale_before::timestamptz AND expired.retry_count >= @max_attempts
    RETURNING expired.id
), candidates AS (
    SELECT o.id
    FROM teaching_grade_event_outbox o
    WHERE (o.status IN (1, 4) OR (o.status = 2 AND o.lease_until <= @stale_before::timestamptz))
      AND o.retry_count < @max_attempts
    ORDER BY o.created_at ASC, o.id ASC
    LIMIT @page_limit
    FOR UPDATE SKIP LOCKED
)
UPDATE teaching_grade_event_outbox AS outbox
SET status = 2, retry_count = outbox.retry_count + 1, updated_at = now(), lease_token = @lease_token, lease_until = @lease_until::timestamptz
FROM candidates
WHERE outbox.id = candidates.id
RETURNING outbox.id, outbox.tenant_id, outbox.course_id, outbox.student_id, outbox.trace_id, outbox.event_updated_at, outbox.status, outbox.retry_count, outbox.last_error, outbox.created_at, outbox.updated_at, outbox.lease_token, outbox.lease_until;

-- name: MarkTeachingGradeEventOutboxPublished :one
UPDATE teaching_grade_event_outbox
SET status = 3, last_error = NULL, updated_at = now(), lease_token = '', lease_until = NULL
WHERE tenant_id = $1 AND id = $2 AND status = 2 AND lease_token = $3 AND lease_until > now()
RETURNING id, tenant_id, course_id, student_id, trace_id, event_updated_at, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until;

-- name: MarkTeachingGradeEventOutboxFailed :one
UPDATE teaching_grade_event_outbox
SET status = 4, last_error = $3, updated_at = now(), lease_token = '', lease_until = NULL
WHERE tenant_id = $1 AND id = $2 AND status = 2 AND lease_token = $4 AND lease_until > now()
RETURNING id, tenant_id, course_id, student_id, trace_id, event_updated_at, status, retry_count, last_error, created_at, updated_at, lease_token, lease_until;

-- name: TeachingStats :one
SELECT
    COUNT(*)::bigint AS course_count,
    COUNT(*) FILTER (WHERE status IN (2, 3))::bigint AS active_course_count,
    COALESCE((SELECT SUM(p.duration_sec)::bigint FROM lesson_progress p WHERE p.tenant_id = $1), 0)::bigint AS learning_duration_sec
FROM course c
WHERE c.tenant_id = $1 AND c.deleted_at IS NULL;

-- name: CreateCourseGradeExportRequest :one
INSERT INTO course_grade_export_request (transfer_task_id, tenant_id, account_id, course_id, next_check_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, now(), now(), now())
RETURNING transfer_task_id, tenant_id, account_id, course_id, next_check_at, created_at, updated_at;

-- name: ListDueCourseGradeExportRequests :many
SELECT transfer_task_id, tenant_id, account_id, course_id, next_check_at, created_at, updated_at
FROM course_grade_export_request
WHERE next_check_at <= $1
ORDER BY next_check_at ASC, created_at ASC
LIMIT $2;

-- name: SetCourseGradeExportRequestNextCheck :one
UPDATE course_grade_export_request
SET next_check_at = $3, updated_at = now()
WHERE transfer_task_id = $1 AND tenant_id = $2
RETURNING transfer_task_id, tenant_id, account_id, course_id, next_check_at, created_at, updated_at;

-- name: DeleteCourseGradeExportRequest :exec
DELETE FROM course_grade_export_request
WHERE transfer_task_id = $1 AND tenant_id = $2;
