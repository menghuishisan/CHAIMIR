// M6 行转换层:集中处理 sqlc 行到领域投影和响应 DTO 的纯转换。
package teaching

import (
	"chaimir/internal/contracts"
	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// courseDTOFromRow 转换课程数据库行。
func courseDTOFromRow(row sqlcgen.Course) CourseDTO {
	return CourseDTO{
		ID: ids.Format(row.ID), TeacherID: ids.Format(row.TeacherID), Name: row.Name, Description: row.Description,
		Type: row.Type, Difficulty: row.Difficulty, CoverURL: pgtypex.TextValue(row.CoverUrl), Semester: row.Semester,
		Credits: pgtypex.NumericValue(row.Credits), Schedule: jsonx.ObjectMap(row.Schedule), InviteCode: row.InviteCode,
		Status: row.Status, Visibility: row.Visibility, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt),
	}
}

// courseAccessSnapshotFromRow 转换课程行供 service 权限与状态机编排使用。
func courseAccessSnapshotFromRow(row sqlcgen.Course) CourseAccessSnapshot {
	return CourseAccessSnapshot{
		ID: row.ID, TeacherID: row.TeacherID, Name: row.Name,
		Visibility: row.Visibility, Status: row.Status,
	}
}

// courseDTOsFromRows 批量转换课程行。
func courseDTOsFromRows(rows []sqlcgen.Course) []CourseDTO {
	out := make([]CourseDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, courseDTOFromRow(row))
	}
	return out
}

// chapterDTOFromRow 转换章节行。
func chapterDTOFromRow(row sqlcgen.Chapter) ChapterDTO {
	return ChapterDTO{ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), Title: row.Title, Sort: row.Sort}
}

// chapterLocationFromRow 转换章节行供 service 做课程归属校验。
func chapterLocationFromRow(row sqlcgen.Chapter) ChapterLocation {
	return ChapterLocation{ID: row.ID, CourseID: row.CourseID}
}

// lessonDTOFromRow 转换课时行。
func lessonDTOFromRow(row sqlcgen.Lesson) LessonDTO {
	return LessonDTO{
		ID: ids.Format(row.ID), ChapterID: ids.Format(row.ChapterID), Title: row.Title,
		ContentType: row.ContentType, ContentRef: jsonx.ObjectMap(row.ContentRef), Sort: row.Sort,
	}
}

// lessonContentSnapshotFromRow 转换课时行供 service 返回详情和定位章节。
func lessonContentSnapshotFromRow(row sqlcgen.Lesson) LessonContentSnapshot {
	return LessonContentSnapshot{
		ID: row.ID, ChapterID: row.ChapterID, Title: row.Title,
		ContentType: row.ContentType, ContentRef: jsonx.ObjectMap(row.ContentRef), Sort: row.Sort,
	}
}

// assignmentDTOFromRow 转换作业行。
func assignmentDTOFromRow(row sqlcgen.Assignment, items []AssignmentItemDTO) AssignmentDTO {
	return AssignmentDTO{
		ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), Title: row.Title, ChapterID: pgtypex.IDString(row.ChapterID),
		DueAt: timex.FromTimestamptz(row.DueAt), MaxAttempts: row.MaxAttempts, LatePolicy: row.LatePolicy,
		LatePenalty: jsonx.ObjectMap(row.LatePenalty), Status: row.Status, Items: items,
	}
}

// assignmentPolicySnapshotFromRow 转换作业行供 service 做发布状态和提交策略判断。
func assignmentPolicySnapshotFromRow(row sqlcgen.Assignment) AssignmentPolicySnapshot {
	return AssignmentPolicySnapshot{
		ID: row.ID, CourseID: row.CourseID, Title: row.Title, ChapterID: pgtypex.Int8Value(row.ChapterID),
		DueAt: timex.FromTimestamptz(row.DueAt), MaxAttempts: row.MaxAttempts,
		LatePolicy: row.LatePolicy, LatePenalty: jsonx.ObjectMap(row.LatePenalty), Status: row.Status,
	}
}

// assignmentItemDTOFromRow 转换作业题目行。
func assignmentItemDTOFromRow(row sqlcgen.AssignmentItem, face map[string]any) AssignmentItemDTO {
	return AssignmentItemDTO{
		ID: ids.Format(row.ID), ItemCode: row.ItemCode, ItemVersion: row.ItemVersion, Score: row.Score,
		Seq: row.Seq, GradingMode: row.GradingMode, JudgerCode: pgtypex.TextValue(row.JudgerCode), Face: face,
	}
}

// assignmentItemSnapshotFromRow 转换作业题目行供 service 做题面展开和判题选择。
func assignmentItemSnapshotFromRow(row sqlcgen.AssignmentItem) AssignmentItemSnapshot {
	return AssignmentItemSnapshot{
		ID: row.ID, ItemCode: row.ItemCode, ItemVersion: row.ItemVersion, Score: row.Score,
		Seq: row.Seq, GradingMode: row.GradingMode, JudgerCode: pgtypex.TextValue(row.JudgerCode),
	}
}

// assignmentItemSnapshotsFromRows 批量转换作业题目行。
func assignmentItemSnapshotsFromRows(rows []sqlcgen.AssignmentItem) []AssignmentItemSnapshot {
	out := make([]AssignmentItemSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, assignmentItemSnapshotFromRow(row))
	}
	return out
}

// submissionDTOFromRow 转换提交行。
func submissionDTOFromRow(row sqlcgen.Submission) SubmissionDTO {
	return SubmissionDTO{
		ID: ids.Format(row.ID), AssignmentID: ids.Format(row.AssignmentID), StudentID: ids.Format(row.StudentID),
		AttemptNo: row.AttemptNo, ContentRef: jsonx.ObjectMap(row.ContentRef), JudgeTaskRef: pgtypex.TextValue(row.JudgeTaskRef),
		AutoScore: pgtypex.Int4PtrValue(row.AutoScore), ManualScore: pgtypex.Int4PtrValue(row.ManualScore), FinalScore: pgtypex.Int4PtrValue(row.FinalScore),
		Comment: pgtypex.TextValue(row.Comment), IsLate: row.IsLate, Status: row.Status, SubmittedAt: timex.FromTimestamptz(row.SubmittedAt),
	}
}

// submissionDTOsFromRows 批量转换提交。
func submissionDTOsFromRows(rows []sqlcgen.Submission) []SubmissionDTO {
	out := make([]SubmissionDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, submissionDTOFromRow(row))
	}
	return out
}

// submissionScoreSnapshotFromRow 转换提交行供 service 做归属校验和评分计算。
func submissionScoreSnapshotFromRow(row sqlcgen.Submission) SubmissionScoreSnapshot {
	return SubmissionScoreSnapshot{
		ID: row.ID, TenantID: row.TenantID, AssignmentID: row.AssignmentID, StudentID: row.StudentID, AttemptNo: row.AttemptNo,
		ContentRef: jsonx.ObjectMap(row.ContentRef), JudgeTaskRef: pgtypex.TextValue(row.JudgeTaskRef),
		AutoScore: pgtypex.Int4PtrValue(row.AutoScore), ManualScore: pgtypex.Int4PtrValue(row.ManualScore), FinalScore: pgtypex.Int4PtrValue(row.FinalScore),
		Comment: pgtypex.TextValue(row.Comment), IsLate: row.IsLate, Status: row.Status, SubmittedAt: timex.FromTimestamptz(row.SubmittedAt),
	}
}

// submissionJudgeOutboxSnapshotFromRow 转换判题 outbox 行供 service 派发 M3。
func submissionJudgeOutboxSnapshotFromRow(row sqlcgen.SubmissionJudgeOutbox) SubmissionJudgeOutboxSnapshot {
	return SubmissionJudgeOutboxSnapshot{
		ID: row.ID, TenantID: row.TenantID, SubmissionID: row.SubmissionID, AssignmentID: row.AssignmentID, StudentID: row.StudentID,
		ItemCode: row.ItemCode, ItemVersion: row.ItemVersion, JudgerCode: row.JudgerCode, CodeStorageKey: row.CodeStorageKey,
		CodeHash: row.CodeHash, ExtraInput: jsonx.ObjectMap(row.ExtraInput), SourceRef: row.SourceRef,
	}
}

// submissionJudgeOutboxSnapshotsFromRows 批量转换判题 outbox 行。
func submissionJudgeOutboxSnapshotsFromRows(rows []sqlcgen.SubmissionJudgeOutbox) []SubmissionJudgeOutboxSnapshot {
	out := make([]SubmissionJudgeOutboxSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, submissionJudgeOutboxSnapshotFromRow(row))
	}
	return out
}

// gradeDTOFromRow 转换课程成绩行。
func gradeDTOFromRow(row sqlcgen.CourseGrade) CourseGradeDTO {
	override := pgtypex.NumericPtrValue(row.OverrideTotal)
	final := pgtypex.NumericValue(row.AutoTotal)
	if override != nil {
		final = *override
	}
	return CourseGradeDTO{
		ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), StudentID: ids.Format(row.StudentID),
		AutoTotal: pgtypex.NumericValue(row.AutoTotal), OverrideTotal: override, FinalTotal: final, IsOverridden: row.IsOverridden,
	}
}

// gradeWeightInputFromRow 转换成绩权重行。
func gradeWeightInputFromRow(row sqlcgen.GradeWeight) GradeWeightInput {
	return GradeWeightInput{SourceType: row.SourceType, SourceRef: row.SourceRef, Weight: pgtypex.NumericValue(row.Weight)}
}

// assignmentScoreSnapshotFromRow 转换最新作业成绩行供 service 计算总评。
func assignmentScoreSnapshotFromRow(row sqlcgen.ListLatestAssignmentScoresForCourseRow) AssignmentScoreSnapshot {
	var final *int32
	if row.FinalScore.Valid {
		v := row.FinalScore.Int32
		final = &v
	}
	return AssignmentScoreSnapshot{AssignmentID: row.AssignmentID, StudentID: row.StudentID, FinalScore: final}
}

// assignmentScoreSnapshotsFromRows 批量转换最新作业成绩行。
func assignmentScoreSnapshotsFromRows(rows []sqlcgen.ListLatestAssignmentScoresForCourseRow) []AssignmentScoreSnapshot {
	out := make([]AssignmentScoreSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, assignmentScoreSnapshotFromRow(row))
	}
	return out
}

// progressSnapshotFromRow 转换学习进度行供 service 聚合统计。
func progressSnapshotFromRow(row sqlcgen.LessonProgress) ProgressSnapshot {
	return ProgressSnapshot{Status: row.Status, DurationSec: row.DurationSec}
}

// progressSnapshotsFromRows 批量转换学习进度行。
func progressSnapshotsFromRows(rows []sqlcgen.LessonProgress) []ProgressSnapshot {
	out := make([]ProgressSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, progressSnapshotFromRow(row))
	}
	return out
}

// gradeWeightInputsFromRows 批量转换成绩权重行。
func gradeWeightInputsFromRows(rows []sqlcgen.GradeWeight) []GradeWeightInput {
	out := make([]GradeWeightInput, 0, len(rows))
	for _, row := range rows {
		out = append(out, gradeWeightInputFromRow(row))
	}
	return out
}

// courseGradeSnapshotFromRow 转换成绩行供 service 审计和响应使用。
func courseGradeSnapshotFromRow(row sqlcgen.CourseGrade) CourseGradeSnapshot {
	dto := gradeDTOFromRow(row)
	return CourseGradeSnapshot{
		ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, StudentID: row.StudentID,
		AutoTotal: dto.AutoTotal, OverrideTotal: dto.OverrideTotal, FinalTotal: dto.FinalTotal, IsOverridden: row.IsOverridden,
	}
}

// courseGradeSnapshotWithCourseFromRows 合并课程学分和成绩行,供 M11 只读契约使用。
func courseGradeSnapshotWithCourseFromRows(course sqlcgen.Course, row sqlcgen.CourseGrade) CourseGradeSnapshot {
	out := courseGradeSnapshotFromRow(row)
	out.Credits = pgtypex.NumericValue(course.Credits)
	return out
}

// courseGradeSnapshotFromStudentCourseRow 转换学生跨课程成绩查询行供 M11 只读聚合。
func courseGradeSnapshotFromStudentCourseRow(row sqlcgen.ListStudentCourseGradesRow) CourseGradeSnapshot {
	out := CourseGradeSnapshot{
		ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, StudentID: row.StudentID,
		AutoTotal: pgtypex.NumericValue(row.AutoTotal), IsOverridden: row.IsOverridden, Credits: pgtypex.NumericValue(row.Credits),
	}
	if row.OverrideTotal.Valid {
		v := pgtypex.NumericValue(row.OverrideTotal)
		out.OverrideTotal = &v
		out.FinalTotal = v
	} else {
		out.FinalTotal = out.AutoTotal
	}
	return out
}

// memberDTOFromRow 转换课程成员行。
func memberDTOFromRow(row sqlcgen.CourseMember) MemberDTO {
	return MemberDTO{ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), StudentID: ids.Format(row.StudentID), JoinMode: row.JoinMode, JoinedAt: timex.FromTimestamptz(row.JoinedAt)}
}

// memberDTOsFromRows 批量转换课程成员。
func memberDTOsFromRows(rows []sqlcgen.CourseMember) []MemberDTO {
	out := make([]MemberDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, memberDTOFromRow(row))
	}
	return out
}

// lessonDTOsFromRows 批量转换课时。
func lessonDTOsFromRows(rows []sqlcgen.Lesson) []LessonDTO {
	out := make([]LessonDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, lessonDTOFromRow(row))
	}
	return out
}

// postDTOFromRow 转换讨论帖行。
func postDTOFromRow(row sqlcgen.DiscussionPost) PostDTO {
	return PostDTO{ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), ParentID: pgtypex.IDString(row.ParentID), AuthorID: ids.Format(row.AuthorID), Content: row.Content, IsPinned: row.IsPinned, LikeCount: row.LikeCount, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// postDTOsFromRows 批量转换讨论帖。
func postDTOsFromRows(rows []sqlcgen.DiscussionPost) []PostDTO {
	out := make([]PostDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, postDTOFromRow(row))
	}
	return out
}

// announcementDTOFromRow 转换公告行。
func announcementDTOFromRow(row sqlcgen.Announcement) AnnouncementDTO {
	return AnnouncementDTO{ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), Title: row.Title, Content: row.Content, IsPinned: row.IsPinned, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// announcementDTOsFromRows 批量转换公告。
func announcementDTOsFromRows(rows []sqlcgen.Announcement) []AnnouncementDTO {
	out := make([]AnnouncementDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, announcementDTOFromRow(row))
	}
	return out
}

// contractGradeFromSnapshot 转换 M6 成绩投影为 contracts DTO。
func contractGradeFromSnapshot(row CourseGradeSnapshot) contracts.TeachingCourseGrade {
	return contracts.TeachingCourseGrade{
		TenantID: row.TenantID, CourseID: row.CourseID, StudentID: row.StudentID,
		AutoTotal: row.AutoTotal, OverrideTotal: row.OverrideTotal, FinalTotal: row.FinalTotal,
		IsOverridden: row.IsOverridden, Credits: row.Credits,
	}
}
