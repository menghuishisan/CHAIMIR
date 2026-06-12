// teaching convert 文件负责领域模型、DTO 与 contracts 对象之间的纯转换。
package teaching

import (
	"encoding/json"
	"time"

	"chaimir/internal/contracts"
)

// courseDTO 将课程领域模型转换为 HTTP 响应结构。
func courseDTO(c Course) CourseDTO {
	return CourseDTO{ID: c.ID, TenantID: c.TenantID, TeacherID: c.TeacherID, Name: c.Name, Description: c.Description, Type: c.Type, Difficulty: c.Difficulty, CoverURL: c.CoverURL, Semester: c.Semester, Credits: c.Credits, Schedule: cloneMap(c.Schedule), InviteCode: c.InviteCode, Status: c.Status, Visibility: c.Visibility, CreatedAt: formatTime(c.CreatedAt), UpdatedAt: formatTime(c.UpdatedAt)}
}

// chapterDTO 将章节领域模型转换为 HTTP 响应结构。
func chapterDTO(c Chapter) ChapterDTO {
	return ChapterDTO{ID: c.ID, CourseID: c.CourseID, Title: c.Title, Sort: c.Sort, CreatedAt: formatTime(c.CreatedAt), UpdatedAt: formatTime(c.UpdatedAt)}
}

// lessonDTO 将课时领域模型转换为 HTTP 响应结构。
func lessonDTO(l Lesson) LessonDTO {
	return LessonDTO{ID: l.ID, ChapterID: l.ChapterID, Title: l.Title, ContentType: l.ContentType, ContentRef: cloneMap(l.ContentRef), Sort: l.Sort, CreatedAt: formatTime(l.CreatedAt), UpdatedAt: formatTime(l.UpdatedAt)}
}

// memberDTO 将课程成员关系转换为 HTTP 响应结构。
func memberDTO(m CourseMember) MemberDTO {
	return MemberDTO{ID: m.ID, CourseID: m.CourseID, StudentID: m.StudentID, JoinMode: m.JoinMode, JoinedAt: formatTime(m.JoinedAt)}
}

// assignmentDTO 将作业外壳转换为 HTTP 响应结构。
func assignmentDTO(a Assignment) AssignmentDTO {
	return AssignmentDTO{ID: a.ID, CourseID: a.CourseID, Title: a.Title, ChapterID: a.ChapterID, DueAt: formatTime(a.DueAt), MaxAttempts: a.MaxAttempts, LatePolicy: a.LatePolicy, LatePenalty: cloneMap(a.LatePenalty), Status: a.Status, CreatedAt: formatTime(a.CreatedAt), UpdatedAt: formatTime(a.UpdatedAt)}
}

// assignmentItemDTO 将作业题目引用和 M5 题面快照组合为响应结构。
func assignmentItemDTO(item AssignmentItemFace) AssignmentItemDTO {
	return AssignmentItemDTO{ID: item.ID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, Seq: item.Seq, GradingMode: item.GradingMode, JudgerCode: item.JudgerCode, Title: item.Title, Type: item.Type, Difficulty: item.Difficulty, Body: cloneMap(item.Body)}
}

// assignmentDetailDTO 将作业详情转换为学生或教师读取的完整响应。
func assignmentDetailDTO(detail AssignmentDetail) AssignmentDetailDTO {
	items := make([]AssignmentItemDTO, 0, len(detail.Items))
	for _, item := range detail.Items {
		items = append(items, assignmentItemDTO(item))
	}
	return AssignmentDetailDTO{Assignment: assignmentDTO(detail.Assignment), Items: items}
}

// assignmentItemFaces 将纯作业题目引用提升为无题面快照的详情项。
func assignmentItemFaces(items []AssignmentItem) []AssignmentItemFace {
	out := make([]AssignmentItemFace, 0, len(items))
	for _, item := range items {
		out = append(out, AssignmentItemFace{AssignmentItem: item})
	}
	return out
}

// submissionDTO 将提交记录转换为 HTTP 响应结构。
func submissionDTO(s Submission) SubmissionDTO {
	return SubmissionDTO{ID: s.ID, AssignmentID: s.AssignmentID, StudentID: s.StudentID, AttemptNo: s.AttemptNo, ContentRef: cloneMap(s.ContentRef), JudgeTaskRef: s.JudgeTaskRef, AutoScore: s.AutoScore, ManualScore: s.ManualScore, FinalScore: s.FinalScore, Comment: s.Comment, IsLate: s.IsLate, Status: s.Status, SubmittedAt: formatTime(s.SubmittedAt)}
}

// progressDTO 将学习进度转换为 HTTP 响应结构。
func progressDTO(p LessonProgress) ProgressDTO {
	return ProgressDTO{LessonID: p.LessonID, StudentID: p.StudentID, Status: p.Status, VideoPos: p.VideoPos, DurationSec: p.DurationSec, UpdatedAt: formatTime(p.UpdatedAt)}
}

// postDTO 将讨论帖或回复转换为 HTTP 响应结构。
func postDTO(p DiscussionPost) PostDTO {
	return PostDTO{ID: p.ID, CourseID: p.CourseID, ParentID: p.ParentID, AuthorID: p.AuthorID, Content: p.Content, IsPinned: p.IsPinned, LikeCount: p.LikeCount, CreatedAt: formatTime(p.CreatedAt)}
}

// announcementDTO 将课程公告转换为 HTTP 响应结构。
func announcementDTO(a Announcement) AnnouncementDTO {
	return AnnouncementDTO{ID: a.ID, CourseID: a.CourseID, Title: a.Title, Content: a.Content, IsPinned: a.IsPinned, CreatedAt: formatTime(a.CreatedAt)}
}

// reviewDTO 将课程评价转换为 HTTP 响应结构。
func reviewDTO(r CourseReview) ReviewDTO {
	return ReviewDTO{ID: r.ID, CourseID: r.CourseID, StudentID: r.StudentID, Rating: r.Rating, Comment: r.Comment, CreatedAt: formatTime(r.CreatedAt)}
}

// gradeWeightDTO 将成绩权重配置转换为 HTTP 响应结构。
func gradeWeightDTO(w GradeWeight) GradeWeightDTO {
	return GradeWeightDTO{ID: w.ID, SourceType: w.SourceType, SourceRef: w.SourceRef, Weight: w.Weight}
}

// gradeDTO 将单课程成绩转换为 HTTP 响应结构。
func gradeDTO(g CourseGrade) GradeDTO {
	return GradeDTO{CourseID: g.CourseID, StudentID: g.StudentID, AutoTotal: g.AutoTotal, OverrideTotal: g.OverrideTotal, FinalTotal: finalTotal(g), IsOverridden: g.IsOverridden, IsLocked: g.IsLocked, Credits: g.Credits, UpdatedAt: formatTime(g.UpdatedAt)}
}

// contractGrade 将 M6 单课程成绩转换为 M11 只读聚合契约对象。
func contractGrade(g CourseGrade) contracts.TeachingCourseGrade {
	override := (*float64)(nil)
	if g.IsOverridden {
		v := g.OverrideTotal
		override = &v
	}
	return contracts.TeachingCourseGrade{TenantID: g.TenantID, CourseID: g.CourseID, StudentID: g.StudentID, AutoTotal: g.AutoTotal, OverrideTotal: override, FinalTotal: finalTotal(g), IsOverridden: g.IsOverridden, Credits: g.Credits}
}

// finalTotal 按 M6 规则选择手动覆盖成绩或自动成绩作为最终分。
func finalTotal(g CourseGrade) float64 {
	if g.IsOverridden {
		return g.OverrideTotal
	}
	return g.AutoTotal
}

// cloneMap 深拷贝 JSON 对象,避免响应转换后调用方修改内部快照。
func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(in)
	if err != nil {
		out := make(map[string]any, len(in))
		for k, v := range in {
			out[k] = v
		}
		return out
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

// formatTime 输出统一的 UTC RFC3339 时间字符串。
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
