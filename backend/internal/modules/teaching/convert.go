// teaching convert 文件负责领域模型、DTO 与 contracts 对象之间的纯转换。
package teaching

import (
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/transfer"
)

// courseDTO 将课程领域模型转换为 HTTP 响应结构。
func courseDTO(c Course) (CourseDTO, error) {
	schedule, err := cloneMap(c.Schedule)
	if err != nil {
		return CourseDTO{}, err
	}
	return CourseDTO{ID: c.ID, TenantID: c.TenantID, TeacherID: c.TeacherID, Name: c.Name, Description: c.Description, Type: c.Type, Difficulty: c.Difficulty, CoverURL: c.CoverURL, Semester: c.Semester, Credits: c.Credits, Schedule: schedule, StartAt: formatTime(c.StartAt), EndAt: formatTime(c.EndAt), InviteCode: c.InviteCode, Status: c.Status, Visibility: c.Visibility, CreatedAt: formatTime(c.CreatedAt), UpdatedAt: formatTime(c.UpdatedAt)}, nil
}

// chapterDTO 将章节领域模型转换为 HTTP 响应结构。
func chapterDTO(c Chapter) ChapterDTO {
	return ChapterDTO{ID: c.ID, CourseID: c.CourseID, Title: c.Title, Sort: c.Sort, CreatedAt: formatTime(c.CreatedAt), UpdatedAt: formatTime(c.UpdatedAt)}
}

// lessonDTO 将课时领域模型转换为 HTTP 响应结构。
func lessonDTO(l Lesson) (LessonDTO, error) {
	contentRef, err := cloneMap(l.ContentRef)
	if err != nil {
		return LessonDTO{}, err
	}
	return LessonDTO{ID: l.ID, ChapterID: l.ChapterID, Title: l.Title, ContentType: l.ContentType, ContentRef: contentRef, Sort: l.Sort, CreatedAt: formatTime(l.CreatedAt), UpdatedAt: formatTime(l.UpdatedAt)}, nil
}

// memberDTO 将课程成员关系转换为 HTTP 响应结构。
func memberDTO(m CourseMember) MemberDTO {
	return MemberDTO{ID: m.ID, CourseID: m.CourseID, StudentID: m.StudentID, JoinMode: m.JoinMode, JoinedAt: formatTime(m.JoinedAt)}
}

// assignmentDTO 将作业外壳转换为 HTTP 响应结构。
func assignmentDTO(a Assignment) (AssignmentDTO, error) {
	latePenalty, err := cloneMap(a.LatePenalty)
	if err != nil {
		return AssignmentDTO{}, err
	}
	return AssignmentDTO{ID: a.ID, CourseID: a.CourseID, Title: a.Title, ChapterID: a.ChapterID, DueAt: formatTime(a.DueAt), MaxAttempts: a.MaxAttempts, LatePolicy: a.LatePolicy, LatePenalty: latePenalty, Status: a.Status, CreatedAt: formatTime(a.CreatedAt), UpdatedAt: formatTime(a.UpdatedAt)}, nil
}

// assignmentItemDTO 将作业题目引用和 M5 题面快照组合为响应结构。
func assignmentItemDTO(item AssignmentItemFace) (AssignmentItemDTO, error) {
	body, err := cloneMap(item.Body)
	if err != nil {
		return AssignmentItemDTO{}, err
	}
	return AssignmentItemDTO{ID: item.ID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, Seq: item.Seq, GradingMode: item.GradingMode, JudgerCode: item.JudgerCode, Title: item.Title, Type: item.Type, Difficulty: item.Difficulty, Body: body}, nil
}

// assignmentDetailDTO 将作业详情转换为学生或教师读取的完整响应。
func assignmentDetailDTO(detail AssignmentDetail) (AssignmentDetailDTO, error) {
	items := make([]AssignmentItemDTO, 0, len(detail.Items))
	for _, item := range detail.Items {
		dto, err := assignmentItemDTO(item)
		if err != nil {
			return AssignmentDetailDTO{}, err
		}
		items = append(items, dto)
	}
	assignment, err := assignmentDTO(detail.Assignment)
	if err != nil {
		return AssignmentDetailDTO{}, err
	}
	return AssignmentDetailDTO{Assignment: assignment, Items: items}, nil
}

// assignmentItemFaces 将纯作业题目引用提升为无题面快照的详情项。
func assignmentItemFaces(items []AssignmentItem) []AssignmentItemFace {
	out := make([]AssignmentItemFace, 0, len(items))
	for _, item := range items {
		out = append(out, AssignmentItemFace{AssignmentItem: item})
	}
	return out
}

// submissionDTO 将提交记录转换为 HTTP 响应结构,过滤对象存储 key/hash 等内部细节。
func submissionDTO(s Submission) (SubmissionDTO, error) {
	content, err := publicSubmissionContent(s.ContentRef)
	if err != nil {
		return SubmissionDTO{}, err
	}
	return SubmissionDTO{ID: s.ID, AssignmentID: s.AssignmentID, StudentID: s.StudentID, AttemptNo: s.AttemptNo, Content: content, JudgeTaskRef: s.JudgeTaskRef, AutoScore: s.AutoScore, ManualScore: s.ManualScore, FinalScore: s.FinalScore, Comment: s.Comment, IsLate: s.IsLate, Status: s.Status, SubmittedAt: formatTime(s.SubmittedAt)}, nil
}

// draftDTO 将服务端权威作答草稿转换为 HTTP 响应结构。
func draftDTO(d SubmissionDraft) (DraftDTO, error) {
	content, err := cloneMap(d.Content)
	if err != nil {
		return DraftDTO{}, err
	}
	return DraftDTO{AssignmentID: d.AssignmentID, StudentID: d.StudentID, Content: content, UpdatedAt: formatTime(d.UpdatedAt), Exists: true}, nil
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

// exportTaskDTO 将统一导入导出中心任务快照转换为课程成绩导出响应。
func exportTaskDTO(task transfer.Task) transfer.TaskDTO {
	return transfer.TaskToDTO(task)
}

// contractGrade 将 M6 单课程成绩转换为 M11 只读聚合契约对象。
func contractGrade(g CourseGrade) contracts.TeachingCourseGrade {
	override := (*float64)(nil)
	if g.IsOverridden {
		v := g.OverrideTotal
		override = &v
	}
	return contracts.TeachingCourseGrade{TenantID: g.TenantID, CourseID: g.CourseID, Semester: g.Semester, StudentID: g.StudentID, AutoTotal: g.AutoTotal, OverrideTotal: override, FinalTotal: finalTotal(g), IsOverridden: g.IsOverridden, Credits: g.Credits}
}

// contractCourse 将 M6 课程转换为 M11 所需的只读归属摘要。
func contractCourse(c Course) contracts.TeachingCourseInfo {
	return contracts.TeachingCourseInfo{TenantID: c.TenantID, CourseID: c.ID, TeacherID: c.TeacherID, Semester: c.Semester, Credits: c.Credits, Status: c.Status}
}

// finalTotal 按 M6 规则选择手动覆盖成绩或自动成绩作为最终分。
func finalTotal(g CourseGrade) float64 {
	if g.IsOverridden {
		return g.OverrideTotal
	}
	return g.AutoTotal
}

// cloneMap 深拷贝 JSON 对象,避免响应转换后调用方修改内部快照。
func cloneMap(in map[string]any) (map[string]any, error) {
	return jsonx.CloneObjectStrict(in)
}

// publicSubmissionContent 深拷贝提交内容并移除内部对象存储和哈希字段。
func publicSubmissionContent(in map[string]any) (map[string]any, error) {
	cloned, err := cloneMap(in)
	if err != nil {
		return nil, err
	}
	stripInternalSubmissionFields(cloned)
	return cloned, nil
}

// stripInternalSubmissionFields 递归移除不应出现在用户响应中的存储实现细节。
func stripInternalSubmissionFields(value any) {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			switch key {
			case "code_storage_key", "code_hash", "object_key", "storage_key", "bucket", "object_ref":
				delete(node, key)
				continue
			}
			stripInternalSubmissionFields(child)
		}
	case []any:
		for _, child := range node {
			stripInternalSubmissionFields(child)
		}
	}
}

// formatTime 输出统一的 UTC RFC3339 时间字符串。
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
