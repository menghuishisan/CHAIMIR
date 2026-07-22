// teaching row_convert 文件负责 sqlc 行到 M6 领域模型的纯转换。
package teaching

import (
	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
)

// courseFromCreateRow 转换建课返回行为领域模型。
func courseFromCreateRow(row sqlcgen.CreateCourseRow) (Course, error) {
	return courseFromFields(row.ID, row.TenantID, row.TeacherID, row.Name, row.Description, row.Type, row.Difficulty, row.CoverUrl, row.Semester, row.Credits, row.Schedule, row.StartAt, row.EndAt, row.InviteCode, row.Status, row.Visibility, row.CreatedAt, row.UpdatedAt)
}

// courseFromGetRow 转换按 ID 读取的课程行。
func courseFromGetRow(row sqlcgen.GetCourseByIDRow) (Course, error) {
	return courseFromFields(row.ID, row.TenantID, row.TeacherID, row.Name, row.Description, row.Type, row.Difficulty, row.CoverUrl, row.Semester, row.Credits, row.Schedule, row.StartAt, row.EndAt, row.InviteCode, row.Status, row.Visibility, row.CreatedAt, row.UpdatedAt)
}

// courseFromCloneableRow 转换跨租户共享课程读取行。
func courseFromCloneableRow(row sqlcgen.GetCloneableCourseByIDRow) (Course, error) {
	return courseFromFields(row.ID, row.TenantID, row.TeacherID, row.Name, row.Description, row.Type, row.Difficulty, row.CoverUrl, row.Semester, row.Credits, row.Schedule, row.StartAt, row.EndAt, row.InviteCode, row.Status, row.Visibility, row.CreatedAt, row.UpdatedAt)
}

// courseFromInviteRow 转换按邀请码读取的课程行。
func courseFromInviteRow(row sqlcgen.GetCourseByInviteCodeRow) (Course, error) {
	return courseFromFields(row.ID, row.TenantID, row.TeacherID, row.Name, row.Description, row.Type, row.Difficulty, row.CoverUrl, row.Semester, row.Credits, row.Schedule, row.StartAt, row.EndAt, row.InviteCode, row.Status, row.Visibility, row.CreatedAt, row.UpdatedAt)
}

// courseFromUpdateRow 转换课程编辑返回行。
func courseFromUpdateRow(row sqlcgen.UpdateCourseRow) (Course, error) {
	return courseFromFields(row.ID, row.TenantID, row.TeacherID, row.Name, row.Description, row.Type, row.Difficulty, row.CoverUrl, row.Semester, row.Credits, row.Schedule, row.StartAt, row.EndAt, row.InviteCode, row.Status, row.Visibility, row.CreatedAt, row.UpdatedAt)
}

// courseFromStatusRow 转换课程状态流转返回行。
func courseFromStatusRow(row sqlcgen.SetCourseStatusRow) (Course, error) {
	return courseFromFields(row.ID, row.TenantID, row.TeacherID, row.Name, row.Description, row.Type, row.Difficulty, row.CoverUrl, row.Semester, row.Credits, row.Schedule, row.StartAt, row.EndAt, row.InviteCode, row.Status, row.Visibility, row.CreatedAt, row.UpdatedAt)
}

// courseFromVisibilityRow 转换课程共享状态返回行。
func courseFromVisibilityRow(row sqlcgen.SetCourseVisibilityRow) (Course, error) {
	return courseFromFields(row.ID, row.TenantID, row.TeacherID, row.Name, row.Description, row.Type, row.Difficulty, row.CoverUrl, row.Semester, row.Credits, row.Schedule, row.StartAt, row.EndAt, row.InviteCode, row.Status, row.Visibility, row.CreatedAt, row.UpdatedAt)
}

// courseFromRefreshRow 转换邀请码刷新返回行。
func courseFromRefreshRow(row sqlcgen.RefreshCourseInviteCodeRow) (Course, error) {
	return courseFromFields(row.ID, row.TenantID, row.TeacherID, row.Name, row.Description, row.Type, row.Difficulty, row.CoverUrl, row.Semester, row.Credits, row.Schedule, row.StartAt, row.EndAt, row.InviteCode, row.Status, row.Visibility, row.CreatedAt, row.UpdatedAt)
}

// courseFromTeacherRow 转换教师课程列表行。
func courseFromTeacherRow(row sqlcgen.ListTeacherCoursesRow) (Course, error) {
	return courseFromFields(row.ID, row.TenantID, row.TeacherID, row.Name, row.Description, row.Type, row.Difficulty, row.CoverUrl, row.Semester, row.Credits, row.Schedule, row.StartAt, row.EndAt, row.InviteCode, row.Status, row.Visibility, row.CreatedAt, row.UpdatedAt)
}

// courseFromStudentRow 转换学生课程列表行。
func courseFromStudentRow(row sqlcgen.ListStudentCoursesRow) (Course, error) {
	return courseFromFields(row.ID, row.TenantID, row.TeacherID, row.Name, row.Description, row.Type, row.Difficulty, row.CoverUrl, row.Semester, row.Credits, row.Schedule, row.StartAt, row.EndAt, row.InviteCode, row.Status, row.Visibility, row.CreatedAt, row.UpdatedAt)
}

// courseFromFields 统一转换 sqlc 为各课程查询生成的同构字段集合。
func courseFromFields(id, tenantID, teacherID int64, name, description string, typ, difficulty int16, cover pgtype.Text, semester string, credits float64, scheduleRaw []byte, startAt, endAt pgtype.Timestamptz, invite string, status, visibility int16, createdAt, updatedAt pgtype.Timestamptz) (Course, error) {
	schedule, err := jsonx.ObjectMapStrict(scheduleRaw)
	if err != nil {
		return Course{}, err
	}
	return Course{ID: id, TenantID: tenantID, TeacherID: teacherID, Name: name, Description: description, Type: typ, Difficulty: difficulty, CoverURL: pgtypex.TextValue(cover), Semester: semester, Credits: credits, Schedule: schedule, StartAt: timex.FromTimestamptz(startAt), EndAt: timex.FromTimestamptz(endAt), InviteCode: invite, Status: status, Visibility: visibility, CreatedAt: timex.FromTimestamptz(createdAt), UpdatedAt: timex.FromTimestamptz(updatedAt)}, nil
}

// chapterFromRow 转换章节表行。
func chapterFromRow(row sqlcgen.Chapter) Chapter {
	return Chapter{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, Title: row.Title, Sort: row.Sort, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// lessonFromRow 转换课时表行并解析内容引用 JSON。
func lessonFromRow(row sqlcgen.Lesson) (Lesson, error) {
	ref, err := jsonx.ObjectMapStrict(row.ContentRef)
	if err != nil {
		return Lesson{}, err
	}
	return Lesson{ID: row.ID, TenantID: row.TenantID, ChapterID: row.ChapterID, Title: row.Title, ContentType: row.ContentType, ContentRef: ref, Sort: row.Sort, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}, nil
}

// memberFromRow 转换课程成员表行。
func memberFromRow(row sqlcgen.CourseMember) CourseMember {
	return CourseMember{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, StudentID: row.StudentID, JoinedAt: timex.FromTimestamptz(row.JoinedAt), JoinMode: row.JoinMode}
}

// assignmentFromRow 转换作业表行并解析迟交策略 JSON。
func assignmentFromRow(row sqlcgen.Assignment) (Assignment, error) {
	penalty, err := jsonx.ObjectMapStrict(row.LatePenalty)
	if err != nil {
		return Assignment{}, err
	}
	return Assignment{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, Title: row.Title, ChapterID: pgtypex.Int8Value(row.ChapterID), DueAt: timex.FromTimestamptz(row.DueAt), MaxAttempts: row.MaxAttempts, LatePolicy: row.LatePolicy, LatePenalty: penalty, Status: row.Status, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}, nil
}

// assignmentItemFromRow 转换作业题目引用表行。
func assignmentItemFromRow(row sqlcgen.AssignmentItem) AssignmentItem {
	return AssignmentItem{ID: row.ID, TenantID: row.TenantID, AssignmentID: row.AssignmentID, ItemCode: row.ItemCode, ItemVersion: row.ItemVersion, Score: row.Score, Seq: row.Seq, GradingMode: row.GradingMode, JudgerCode: pgtypex.TextValue(row.JudgerCode), CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// submissionFromRow 转换提交表行并解析提交内容引用 JSON。
func submissionFromRow(row sqlcgen.Submission) (Submission, error) {
	ref, err := jsonx.ObjectMapStrict(row.ContentRef)
	if err != nil {
		return Submission{}, err
	}
	return Submission{ID: row.ID, TenantID: row.TenantID, AssignmentID: row.AssignmentID, StudentID: row.StudentID, AttemptNo: row.AttemptNo, ContentRef: ref, JudgeTaskRef: pgtypex.TextValue(row.JudgeTaskRef), AutoScore: pgtypex.Int4Value(row.AutoScore), ManualScore: pgtypex.Int4Value(row.ManualScore), FinalScore: pgtypex.Int4Value(row.FinalScore), Comment: pgtypex.TextValue(row.Comment), IsLate: row.IsLate, Status: row.Status, SubmittedAt: timex.FromTimestamptz(row.SubmittedAt)}, nil
}

// outboxFromRow 转换自动判题 outbox 行并解析额外输入。
func outboxFromRow(row sqlcgen.SubmissionJudgeOutbox) (JudgeOutbox, error) {
	extra, err := jsonx.ObjectMapStrict(row.ExtraInput)
	if err != nil {
		return JudgeOutbox{}, err
	}
	return JudgeOutbox{ID: row.ID, TenantID: row.TenantID, SubmissionID: row.SubmissionID, AssignmentItemID: row.AssignmentItemID, AssignmentID: row.AssignmentID, SourceOwnerID: row.SourceOwnerID, SourceCourseID: row.SourceCourseID, SourceScope: row.SourceScope, StudentID: row.StudentID, ItemCode: row.ItemCode, ItemVersion: row.ItemVersion, JudgerCode: row.JudgerCode, CodeStorageKey: row.CodeStorageKey, CodeHash: row.CodeHash, ExtraInput: extra, SourceRef: row.SourceRef, Status: row.Status, RetryCount: row.RetryCount, LastError: pgtypex.TextValue(row.LastError), Score: pgtypex.Int4Value(row.Score), CompletedAt: timex.FromTimestamptz(row.CompletedAt), CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}, nil
}

// draftFromRow 转换作答草稿行并解析草稿 JSON。
func draftFromRow(row sqlcgen.SubmissionDraft) (SubmissionDraft, error) {
	content, err := jsonx.ObjectMapStrict(row.Content)
	if err != nil {
		return SubmissionDraft{}, err
	}
	return SubmissionDraft{ID: row.ID, TenantID: row.TenantID, AssignmentID: row.AssignmentID, StudentID: row.StudentID, Content: content, UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}, nil
}

// progressFromRow 转换学习进度行。
func progressFromRow(row sqlcgen.LessonProgress) LessonProgress {
	return LessonProgress{ID: row.ID, TenantID: row.TenantID, LessonID: row.LessonID, StudentID: row.StudentID, Status: row.Status, VideoPos: pgtypex.Int4Value(row.VideoPos), DurationSec: row.DurationSec, UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// postFromRow 转换讨论帖或回复行。
func postFromRow(row sqlcgen.DiscussionPost) DiscussionPost {
	return DiscussionPost{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, ParentID: pgtypex.Int8Value(row.ParentID), AuthorID: row.AuthorID, Content: row.Content, IsPinned: row.IsPinned, LikeCount: row.LikeCount, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// announcementFromRow 转换公告表行。
func announcementFromRow(row sqlcgen.Announcement) Announcement {
	return Announcement{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, Title: row.Title, Content: row.Content, IsPinned: row.IsPinned, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// reviewFromRow 转换课程评价行。
func reviewFromRow(row sqlcgen.CourseReview) CourseReview {
	return CourseReview{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, StudentID: row.StudentID, Rating: row.Rating, Comment: pgtypex.TextValue(row.Comment), CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// weightFromCreateRow 转换创建权重返回行。
func weightFromCreateRow(row sqlcgen.CreateGradeWeightRow) GradeWeight {
	return GradeWeight{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, SourceType: row.SourceType, SourceRef: row.SourceRef, Weight: row.Weight, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// weightFromListRow 转换权重列表行。
func weightFromListRow(row sqlcgen.ListGradeWeightsRow) GradeWeight {
	return GradeWeight{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, SourceType: row.SourceType, SourceRef: row.SourceRef, Weight: row.Weight, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// gradeFromListRow 转换课程成绩列表行。
func gradeFromListRow(row sqlcgen.ListCourseGradesRow) CourseGrade {
	return CourseGrade{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, Semester: row.Semester, StudentID: row.StudentID, AutoTotal: row.AutoTotal, OverrideTotal: row.OverrideTotal, IsOverridden: row.IsOverridden, IsLocked: row.IsLocked, Credits: row.Credits, UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// gradeFromGetRow 转换单个课程成绩读取行。
func gradeFromGetRow(row sqlcgen.GetCourseGradeRow) CourseGrade {
	return CourseGrade{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, StudentID: row.StudentID, AutoTotal: row.AutoTotal, OverrideTotal: row.OverrideTotal, IsOverridden: row.IsOverridden, IsLocked: row.IsLocked, UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// gradeFromStudentRow 转换学生成绩列表行。
func gradeFromStudentRow(row sqlcgen.ListStudentGradesRow) CourseGrade {
	return CourseGrade{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, Semester: row.Semester, StudentID: row.StudentID, AutoTotal: row.AutoTotal, OverrideTotal: row.OverrideTotal, IsOverridden: row.IsOverridden, IsLocked: row.IsLocked, Credits: row.Credits, UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// gradeFromOverrideRow 转换手动调分返回行。
func gradeFromOverrideRow(row sqlcgen.OverrideCourseGradeRow) CourseGrade {
	return CourseGrade{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, StudentID: row.StudentID, AutoTotal: row.AutoTotal, OverrideTotal: row.OverrideTotal, IsOverridden: row.IsOverridden, IsLocked: row.IsLocked, UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// gradeFromUpsertRow 转换成绩计算写入返回行。
func gradeFromUpsertRow(row sqlcgen.UpsertCourseGradeRow) CourseGrade {
	return CourseGrade{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, StudentID: row.StudentID, AutoTotal: row.AutoTotal, OverrideTotal: row.OverrideTotal, IsOverridden: row.IsOverridden, IsLocked: row.IsLocked, UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// teachingGradeEventOutbox 转换成绩变更事件 outbox 行。
func teachingGradeEventOutbox(row sqlcgen.TeachingGradeEventOutbox) TeachingGradeEventOutbox {
	return TeachingGradeEventOutbox{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, StudentID: row.StudentID, TraceID: row.TraceID, EventUpdatedAt: timex.FromTimestamptz(row.EventUpdatedAt), Status: row.Status, RetryCount: row.RetryCount, LastError: pgtypex.TextValue(row.LastError), CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// encodeMap 将 JSON 对象编码为 JSONB 参数。
func encodeMap(value map[string]any) ([]byte, error) {
	if value == nil {
		value = map[string]any{}
	}
	return jsonx.ObjectBytes(value, apperr.ErrTeachingLessonInvalid)
}
