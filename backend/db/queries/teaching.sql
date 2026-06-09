-- M6 teaching sqlc 查询:仅访问 teaching 模块自有租户表。

-- name: CreateCourse :one
INSERT INTO course (id, tenant_id, teacher_id, name, description, type, difficulty, cover_url, semester, credits, schedule, invite_code, status, visibility)
VALUES (@id, @tenant_id, @teacher_id, @name, @description, @type, @difficulty, @cover_url, @semester, @credits, @schedule, @invite_code, @status, @visibility)
RETURNING id, tenant_id, teacher_id, name, description, type, difficulty, cover_url, semester, credits, schedule, invite_code, status, visibility, deleted_at, created_at, updated_at;

-- name: GetCourseByID :one
SELECT id, tenant_id, teacher_id, name, description, type, difficulty, cover_url, semester, credits, schedule, invite_code, status, visibility, deleted_at, created_at, updated_at FROM course WHERE id = @id AND deleted_at IS NULL;

-- name: GetCourseByInviteCode :one
SELECT id, tenant_id, teacher_id, name, description, type, difficulty, cover_url, semester, credits, schedule, invite_code, status, visibility, deleted_at, created_at, updated_at FROM course WHERE invite_code = @invite_code AND status IN (2, 3) AND deleted_at IS NULL;

-- name: ListTeacherCourses :many
SELECT id, tenant_id, teacher_id, name, description, type, difficulty, cover_url, semester, credits, schedule, invite_code, status, visibility, deleted_at, created_at, updated_at FROM course
WHERE deleted_at IS NULL
  AND teacher_id = @teacher_id
  AND (sqlc.narg('status')::smallint IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: ListStudentCourses :many
SELECT c.id, c.tenant_id, c.teacher_id, c.name, c.description, c.type, c.difficulty, c.cover_url, c.semester, c.credits, c.schedule, c.invite_code, c.status, c.visibility, c.deleted_at, c.created_at, c.updated_at FROM course c
JOIN course_member m ON m.course_id = c.id AND m.tenant_id = c.tenant_id
WHERE c.deleted_at IS NULL
  AND m.student_id = @student_id
  AND (sqlc.narg('status')::smallint IS NULL OR c.status = sqlc.narg('status'))
ORDER BY c.created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: UpdateCourse :one
UPDATE course SET
  name = @name,
  description = @description,
  type = @type,
  difficulty = @difficulty,
  cover_url = @cover_url,
  semester = @semester,
  credits = @credits,
  schedule = @schedule
WHERE id = @id AND deleted_at IS NULL
RETURNING id, tenant_id, teacher_id, name, description, type, difficulty, cover_url, semester, credits, schedule, invite_code, status, visibility, deleted_at, created_at, updated_at;

-- name: UpdateCourseStatus :one
UPDATE course SET status = @status
WHERE id = @id AND deleted_at IS NULL
RETURNING id, tenant_id, teacher_id, name, description, type, difficulty, cover_url, semester, credits, schedule, invite_code, status, visibility, deleted_at, created_at, updated_at;

-- name: EnsureCoursePublishable :one
SELECT (
  EXISTS (
    SELECT 1 FROM chapter ch
    WHERE ch.course_id = @course_id
      AND ch.deleted_at IS NULL
  )
  AND NOT EXISTS (
    SELECT 1 FROM chapter ch
    WHERE ch.course_id = @course_id
      AND ch.deleted_at IS NULL
      AND NOT EXISTS (
        SELECT 1 FROM lesson l
        WHERE l.chapter_id = ch.id
          AND l.deleted_at IS NULL
      )
  )
)::boolean AS publishable;

-- name: UpdateCourseVisibility :one
UPDATE course SET visibility = @visibility
WHERE id = @id AND deleted_at IS NULL
RETURNING id, tenant_id, teacher_id, name, description, type, difficulty, cover_url, semester, credits, schedule, invite_code, status, visibility, deleted_at, created_at, updated_at;

-- name: UpdateCourseInviteCode :one
UPDATE course SET invite_code = @invite_code
WHERE id = @id AND deleted_at IS NULL
RETURNING id, tenant_id, teacher_id, name, description, type, difficulty, cover_url, semester, credits, schedule, invite_code, status, visibility, deleted_at, created_at, updated_at;

-- name: SoftDeleteCourse :one
UPDATE course SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL
RETURNING id, tenant_id, teacher_id, name, description, type, difficulty, cover_url, semester, credits, schedule, invite_code, status, visibility, deleted_at, created_at, updated_at;

-- name: CreateChapter :one
INSERT INTO chapter (id, tenant_id, course_id, title, sort)
VALUES (@id, @tenant_id, @course_id, @title, @sort)
RETURNING id, tenant_id, course_id, title, sort, deleted_at, created_at, updated_at;

-- name: ListChaptersByCourse :many
SELECT id, tenant_id, course_id, title, sort, deleted_at, created_at, updated_at FROM chapter WHERE course_id = @course_id AND deleted_at IS NULL ORDER BY sort ASC, id ASC;

-- name: GetChapterByID :one
SELECT id, tenant_id, course_id, title, sort, deleted_at, created_at, updated_at FROM chapter WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateChapter :one
UPDATE chapter SET title = @title, sort = @sort
WHERE id = @id AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, title, sort, deleted_at, created_at, updated_at;

-- name: SoftDeleteChapter :one
UPDATE chapter SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, title, sort, deleted_at, created_at, updated_at;

-- name: CreateLesson :one
INSERT INTO lesson (id, tenant_id, chapter_id, title, content_type, content_ref, sort)
VALUES (@id, @tenant_id, @chapter_id, @title, @content_type, @content_ref, @sort)
RETURNING id, tenant_id, chapter_id, title, content_type, content_ref, sort, deleted_at, created_at, updated_at;

-- name: ListLessonsByChapter :many
SELECT id, tenant_id, chapter_id, title, content_type, content_ref, sort, deleted_at, created_at, updated_at FROM lesson WHERE chapter_id = @chapter_id AND deleted_at IS NULL ORDER BY sort ASC, id ASC;

-- name: GetLessonByID :one
SELECT id, tenant_id, chapter_id, title, content_type, content_ref, sort, deleted_at, created_at, updated_at FROM lesson WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateLesson :one
UPDATE lesson SET title = @title, content_type = @content_type, content_ref = @content_ref, sort = @sort
WHERE id = @id AND deleted_at IS NULL
RETURNING id, tenant_id, chapter_id, title, content_type, content_ref, sort, deleted_at, created_at, updated_at;

-- name: UpdateLessonContent :one
UPDATE lesson SET content_type = @content_type, content_ref = @content_ref
WHERE id = @id AND deleted_at IS NULL
RETURNING id, tenant_id, chapter_id, title, content_type, content_ref, sort, deleted_at, created_at, updated_at;

-- name: SoftDeleteLesson :one
UPDATE lesson SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL
RETURNING id, tenant_id, chapter_id, title, content_type, content_ref, sort, deleted_at, created_at, updated_at;

-- name: AddCourseMember :one
INSERT INTO course_member (id, tenant_id, course_id, student_id, join_mode)
VALUES (@id, @tenant_id, @course_id, @student_id, @join_mode)
ON CONFLICT (tenant_id, course_id, student_id) DO UPDATE SET join_mode = EXCLUDED.join_mode
RETURNING id, tenant_id, course_id, student_id, joined_at, join_mode;

-- name: ListCourseMembers :many
SELECT id, tenant_id, course_id, student_id, joined_at, join_mode FROM course_member WHERE course_id = @course_id ORDER BY joined_at DESC LIMIT @limit_count OFFSET @offset_count;

-- name: GetCourseMember :one
SELECT id, tenant_id, course_id, student_id, joined_at, join_mode FROM course_member WHERE course_id = @course_id AND student_id = @student_id;

-- name: RemoveCourseMember :exec
DELETE FROM course_member WHERE course_id = @course_id AND student_id = @student_id;

-- name: CreateAssignment :one
INSERT INTO assignment (id, tenant_id, course_id, title, chapter_id, due_at, max_attempts, late_policy, late_penalty, status)
VALUES (@id, @tenant_id, @course_id, @title, @chapter_id, @due_at, @max_attempts, @late_policy, @late_penalty, @status)
RETURNING id, tenant_id, course_id, title, chapter_id, due_at, max_attempts, late_policy, late_penalty, status, deleted_at, created_at, updated_at;

-- name: GetAssignmentByID :one
SELECT id, tenant_id, course_id, title, chapter_id, due_at, max_attempts, late_policy, late_penalty, status, deleted_at, created_at, updated_at FROM assignment WHERE id = @id AND deleted_at IS NULL;

-- name: ListAssignmentsByCourse :many
SELECT id, tenant_id, course_id, title, chapter_id, due_at, max_attempts, late_policy, late_penalty, status, deleted_at, created_at, updated_at FROM assignment WHERE course_id = @course_id AND deleted_at IS NULL ORDER BY due_at ASC, id ASC;

-- name: UpdateAssignment :one
UPDATE assignment SET
  title = @title,
  chapter_id = @chapter_id,
  due_at = @due_at,
  max_attempts = @max_attempts,
  late_policy = @late_policy,
  late_penalty = @late_penalty
WHERE id = @id AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, title, chapter_id, due_at, max_attempts, late_policy, late_penalty, status, deleted_at, created_at, updated_at;

-- name: UpdateAssignmentStatus :one
UPDATE assignment SET status = @status
WHERE id = @id AND deleted_at IS NULL
RETURNING id, tenant_id, course_id, title, chapter_id, due_at, max_attempts, late_policy, late_penalty, status, deleted_at, created_at, updated_at;

-- name: DeleteAssignmentItems :exec
DELETE FROM assignment_item WHERE assignment_id = @assignment_id;

-- name: CreateAssignmentItem :one
INSERT INTO assignment_item (id, tenant_id, assignment_id, item_code, item_version, score, seq, grading_mode, judger_code)
VALUES (@id, @tenant_id, @assignment_id, @item_code, @item_version, @score, @seq, @grading_mode, @judger_code)
RETURNING id, tenant_id, assignment_id, item_code, item_version, score, seq, grading_mode, judger_code, created_at;

-- name: ListAssignmentItems :many
SELECT id, tenant_id, assignment_id, item_code, item_version, score, seq, grading_mode, judger_code, created_at FROM assignment_item WHERE assignment_id = @assignment_id ORDER BY seq ASC, id ASC;

-- name: CountSubmissionsByStudent :one
SELECT count(*)::bigint FROM submission WHERE assignment_id = @assignment_id AND student_id = @student_id;

-- name: CreateSubmission :one
INSERT INTO submission (id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status)
SELECT @id, @tenant_id, @assignment_id, @student_id, @attempt_no, @content_ref, @judge_task_ref, @auto_score, @manual_score, @final_score, @comment, @is_late, @status
WHERE EXISTS (
  SELECT 1 FROM assignment a
  JOIN course c ON c.id = a.course_id AND c.tenant_id = a.tenant_id
  JOIN course_member m ON m.tenant_id = c.tenant_id AND m.course_id = c.id
  WHERE a.id = @assignment_id
    AND a.deleted_at IS NULL
    AND c.deleted_at IS NULL
    AND m.student_id = @student_id
)
RETURNING id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at;

-- name: GetSubmissionByID :one
SELECT id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at FROM submission WHERE id = @id;

-- name: ListSubmissionsByAssignment :many
SELECT id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at FROM submission WHERE assignment_id = @assignment_id ORDER BY submitted_at DESC LIMIT @limit_count OFFSET @offset_count;

-- name: GetSubmissionByJudgeTaskRef :one
SELECT id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at FROM submission WHERE judge_task_ref = @judge_task_ref;

-- name: UpdateSubmissionJudgeTaskRef :one
UPDATE submission SET judge_task_ref = @judge_task_ref
WHERE id = @id
RETURNING id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at;

-- name: CreateSubmissionJudgeOutbox :one
INSERT INTO submission_judge_outbox (
  id, tenant_id, submission_id, assignment_id, student_id, item_code, item_version,
  judger_code, code_storage_key, code_hash, extra_input, source_ref, status
)
VALUES (
  @id, @tenant_id, @submission_id, @assignment_id, @student_id, @item_code, @item_version,
  @judger_code, @code_storage_key, @code_hash, @extra_input, @source_ref, @status
)
RETURNING id, tenant_id, submission_id, assignment_id, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, created_at, updated_at;

-- name: ListPendingSubmissionJudgeOutbox :many
SELECT id, tenant_id, submission_id, assignment_id, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, created_at, updated_at FROM submission_judge_outbox
WHERE status = @status
ORDER BY created_at ASC
LIMIT @limit_count;

-- name: ListPendingSubmissionJudgeOutboxTenants :many
SELECT DISTINCT tenant_id FROM submission_judge_outbox
WHERE status = @status
ORDER BY tenant_id ASC
LIMIT @limit_count;

-- name: MarkSubmissionJudgeOutboxRunning :one
UPDATE submission_judge_outbox
SET status = @running_status, retry_count = retry_count + 1, last_error = NULL
WHERE id = @id
  AND status = @pending_status
RETURNING id, tenant_id, submission_id, assignment_id, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, created_at, updated_at;

-- name: CompleteSubmissionJudgeOutbox :one
UPDATE submission_judge_outbox
SET status = @done_status, last_error = NULL
WHERE id = @id
RETURNING id, tenant_id, submission_id, assignment_id, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, created_at, updated_at;

-- name: FailSubmissionJudgeOutbox :one
UPDATE submission_judge_outbox
SET status = @pending_status, last_error = @last_error
WHERE id = @id
RETURNING id, tenant_id, submission_id, assignment_id, student_id, item_code, item_version, judger_code, code_storage_key, code_hash, extra_input, source_ref, status, retry_count, last_error, created_at, updated_at;

-- name: UpdateSubmissionAutoScore :one
UPDATE submission SET auto_score = @auto_score, final_score = @final_score, status = @status
WHERE id = @id
RETURNING id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at;

-- name: UpdateSubmissionManualScore :one
UPDATE submission SET manual_score = @manual_score, final_score = @final_score, comment = @comment, status = @status
WHERE id = @id
RETURNING id, tenant_id, assignment_id, student_id, attempt_no, content_ref, judge_task_ref, auto_score, manual_score, final_score, comment, is_late, status, submitted_at;

-- name: UpsertSubmissionDraft :one
INSERT INTO submission_draft (id, tenant_id, assignment_id, student_id, content)
SELECT @id, @tenant_id, @assignment_id, @student_id, @content
WHERE EXISTS (
  SELECT 1 FROM assignment a
  JOIN course c ON c.id = a.course_id AND c.tenant_id = a.tenant_id
  JOIN course_member m ON m.tenant_id = c.tenant_id AND m.course_id = c.id
  WHERE a.id = @assignment_id
    AND a.deleted_at IS NULL
    AND c.deleted_at IS NULL
    AND m.student_id = @student_id
)
ON CONFLICT (tenant_id, assignment_id, student_id) DO UPDATE SET content = EXCLUDED.content, updated_at = now()
RETURNING id, tenant_id, assignment_id, student_id, content, updated_at;

-- name: GetSubmissionDraft :one
SELECT id, tenant_id, assignment_id, student_id, content, updated_at FROM submission_draft
WHERE assignment_id = @assignment_id
  AND student_id = @student_id;

-- name: DeleteSubmissionDraft :exec
DELETE FROM submission_draft
WHERE assignment_id = @assignment_id
  AND student_id = @student_id;

-- name: UpsertLessonProgress :one
INSERT INTO lesson_progress (id, tenant_id, lesson_id, student_id, status, video_pos, duration_sec)
SELECT @id, @tenant_id, @lesson_id, @student_id, @status, @video_pos, @duration_sec
WHERE EXISTS (
  SELECT 1 FROM lesson l
  JOIN chapter ch ON ch.id = l.chapter_id AND ch.tenant_id = l.tenant_id
  JOIN course c ON c.id = ch.course_id AND c.tenant_id = ch.tenant_id
  JOIN course_member m ON m.tenant_id = c.tenant_id AND m.course_id = c.id
  WHERE l.id = @lesson_id
    AND l.deleted_at IS NULL
    AND ch.deleted_at IS NULL
    AND c.deleted_at IS NULL
    AND m.student_id = @student_id
)
ON CONFLICT (tenant_id, lesson_id, student_id) DO UPDATE SET status = EXCLUDED.status, video_pos = EXCLUDED.video_pos, duration_sec = lesson_progress.duration_sec + EXCLUDED.duration_sec, updated_at = now()
RETURNING id, tenant_id, lesson_id, student_id, status, video_pos, duration_sec, updated_at;

-- name: GetLessonProgress :one
SELECT id, tenant_id, lesson_id, student_id, status, video_pos, duration_sec, updated_at FROM lesson_progress WHERE lesson_id = @lesson_id AND student_id = @student_id;

-- name: ListLessonProgressByCourse :many
SELECT p.id, p.tenant_id, p.lesson_id, p.student_id, p.status, p.video_pos, p.duration_sec, p.updated_at FROM lesson_progress p
JOIN lesson l ON l.id = p.lesson_id
JOIN chapter c ON c.id = l.chapter_id
WHERE c.course_id = @course_id
ORDER BY p.updated_at DESC;

-- name: ListLessonProgressByCourseAndStudent :many
SELECT p.id, p.tenant_id, p.lesson_id, p.student_id, p.status, p.video_pos, p.duration_sec, p.updated_at FROM lesson_progress p
JOIN lesson l ON l.id = p.lesson_id
JOIN chapter c ON c.id = l.chapter_id
WHERE c.course_id = @course_id
  AND p.student_id = @student_id
ORDER BY p.updated_at DESC;

-- name: CreateDiscussionPost :one
INSERT INTO discussion_post (id, tenant_id, course_id, parent_id, author_id, content)
SELECT @id, @tenant_id, @course_id, @parent_id, @author_id, @content
WHERE sqlc.narg('parent_id')::bigint IS NULL
   OR EXISTS (
     SELECT 1 FROM discussion_post parent
     WHERE parent.id = sqlc.narg('parent_id')::bigint
       AND parent.course_id = @course_id
       AND parent.deleted_at IS NULL
   )
RETURNING id, tenant_id, course_id, parent_id, author_id, content, is_pinned, like_count, deleted_at, created_at, updated_at;

-- name: ListDiscussionPosts :many
SELECT id, tenant_id, course_id, parent_id, author_id, content, is_pinned, like_count, deleted_at, created_at, updated_at FROM discussion_post
WHERE course_id = @course_id AND deleted_at IS NULL
ORDER BY is_pinned DESC, created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: IncrementPostLike :one
UPDATE discussion_post p
SET like_count = p.like_count + 1
FROM course c
WHERE p.id = @id
  AND p.deleted_at IS NULL
  AND c.id = p.course_id
  AND c.deleted_at IS NULL
  AND (
    @is_platform::boolean
    OR c.teacher_id = @actor_id
    OR EXISTS (
      SELECT 1 FROM course_member m
      WHERE m.tenant_id = p.tenant_id
        AND m.course_id = p.course_id
        AND m.student_id = @actor_id
    )
  )
RETURNING p.id, p.tenant_id, p.course_id, p.parent_id, p.author_id, p.content, p.is_pinned, p.like_count, p.deleted_at, p.created_at, p.updated_at;

-- name: TogglePostPin :one
UPDATE discussion_post p
SET is_pinned = NOT p.is_pinned
FROM course c
WHERE p.id = @id
  AND p.deleted_at IS NULL
  AND c.id = p.course_id
  AND c.deleted_at IS NULL
  AND (@is_platform::boolean OR c.teacher_id = @actor_id)
RETURNING p.id, p.tenant_id, p.course_id, p.parent_id, p.author_id, p.content, p.is_pinned, p.like_count, p.deleted_at, p.created_at, p.updated_at;

-- name: SoftDeletePost :one
UPDATE discussion_post p
SET deleted_at = now()
FROM course c
WHERE p.id = @id
  AND p.deleted_at IS NULL
  AND c.id = p.course_id
  AND c.deleted_at IS NULL
  AND (@is_platform::boolean OR c.teacher_id = @actor_id)
RETURNING p.id, p.tenant_id, p.course_id, p.parent_id, p.author_id, p.content, p.is_pinned, p.like_count, p.deleted_at, p.created_at, p.updated_at;

-- name: CreateAnnouncement :one
INSERT INTO announcement (id, tenant_id, course_id, title, content)
VALUES (@id, @tenant_id, @course_id, @title, @content)
RETURNING id, tenant_id, course_id, title, content, is_pinned, deleted_at, created_at, updated_at;

-- name: ListAnnouncements :many
SELECT id, tenant_id, course_id, title, content, is_pinned, deleted_at, created_at, updated_at FROM announcement
WHERE course_id = @course_id AND deleted_at IS NULL
ORDER BY is_pinned DESC, created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: ToggleAnnouncementPin :one
UPDATE announcement a
SET is_pinned = NOT a.is_pinned
FROM course c
WHERE a.id = @id
  AND a.deleted_at IS NULL
  AND c.id = a.course_id
  AND c.deleted_at IS NULL
  AND (@is_platform::boolean OR c.teacher_id = @actor_id)
RETURNING a.id, a.tenant_id, a.course_id, a.title, a.content, a.is_pinned, a.deleted_at, a.created_at, a.updated_at;

-- name: UpsertCourseReview :one
INSERT INTO course_review (id, tenant_id, course_id, student_id, rating, comment)
SELECT @id, @tenant_id, @course_id, @student_id, @rating, @comment
WHERE EXISTS (
  SELECT 1 FROM course c
  JOIN course_member m ON m.tenant_id = c.tenant_id AND m.course_id = c.id
  WHERE c.id = @course_id
    AND c.deleted_at IS NULL
    AND m.student_id = @student_id
)
ON CONFLICT (tenant_id, course_id, student_id) DO UPDATE SET rating = EXCLUDED.rating, comment = EXCLUDED.comment
RETURNING id, tenant_id, course_id, student_id, rating, comment, created_at;

-- name: DeleteGradeWeightsByCourse :exec
DELETE FROM grade_weight WHERE course_id = @course_id;

-- name: CreateGradeWeight :one
INSERT INTO grade_weight (id, tenant_id, course_id, source_type, source_ref, weight)
VALUES (@id, @tenant_id, @course_id, @source_type, @source_ref, @weight)
RETURNING id, tenant_id, course_id, source_type, source_ref, weight;

-- name: ListGradeWeightsByCourse :many
SELECT id, tenant_id, course_id, source_type, source_ref, weight FROM grade_weight WHERE course_id = @course_id ORDER BY source_type ASC, source_ref ASC;

-- name: ListLatestAssignmentScoresForCourse :many
SELECT DISTINCT ON (s.assignment_id, s.student_id)
  s.assignment_id, s.student_id, s.final_score
FROM submission s
JOIN assignment a ON a.id = s.assignment_id
WHERE a.course_id = @course_id AND s.final_score IS NOT NULL
ORDER BY s.assignment_id, s.student_id, s.attempt_no DESC, s.submitted_at DESC;

-- name: UpsertCourseGrade :one
INSERT INTO course_grade (id, tenant_id, course_id, student_id, auto_total, override_total, is_overridden)
VALUES (@id, @tenant_id, @course_id, @student_id, @auto_total, @override_total, @is_overridden)
ON CONFLICT (tenant_id, course_id, student_id) DO UPDATE SET
  auto_total = EXCLUDED.auto_total,
  override_total = EXCLUDED.override_total,
  is_overridden = EXCLUDED.is_overridden,
  updated_at = now()
RETURNING id, tenant_id, course_id, student_id, auto_total, override_total, is_overridden, updated_at;

-- name: ListCourseGrades :many
SELECT id, tenant_id, course_id, student_id, auto_total, override_total, is_overridden, updated_at FROM course_grade WHERE course_id = @course_id ORDER BY student_id ASC LIMIT @limit_count OFFSET @offset_count;

-- name: ListStudentCourseGrades :many
SELECT
    cg.id,
    cg.tenant_id,
    cg.course_id,
    cg.student_id,
    cg.auto_total,
    cg.override_total,
    cg.is_overridden,
    cg.updated_at,
    c.credits
FROM course_grade cg
JOIN course c ON c.id = cg.course_id
WHERE cg.student_id = @student_id
ORDER BY c.semester ASC, cg.course_id ASC;

-- name: GetCourseGrade :one
SELECT id, tenant_id, course_id, student_id, auto_total, override_total, is_overridden, updated_at FROM course_grade WHERE course_id = @course_id AND student_id = @student_id;

-- name: CountCourses :one
SELECT count(*)::bigint FROM course WHERE deleted_at IS NULL;

-- name: CountActiveCourses :one
SELECT count(*)::bigint FROM course WHERE deleted_at IS NULL AND status IN (2, 3);

-- name: SumLearningDuration :one
SELECT coalesce(sum(duration_sec), 0)::bigint FROM lesson_progress;
