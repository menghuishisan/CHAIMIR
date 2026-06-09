// M6 转换层:处理领域 DTO、contracts DTO 与 HTTP 输出结构之间的纯转换。
package teaching

import (
	"chaimir/internal/platform/ids"
)

// lessonDTOFromContentSnapshot 转换课时内容投影为响应 DTO。
func lessonDTOFromContentSnapshot(row LessonContentSnapshot) LessonDTO {
	return LessonDTO{
		ID: ids.Format(row.ID), ChapterID: ids.Format(row.ChapterID), Title: row.Title,
		ContentType: row.ContentType, ContentRef: row.ContentRef, Sort: row.Sort,
	}
}

// assignmentDTOFromPolicySnapshot 转换作业策略投影为响应 DTO。
func assignmentDTOFromPolicySnapshot(row AssignmentPolicySnapshot, items []AssignmentItemDTO) AssignmentDTO {
	return AssignmentDTO{
		ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), Title: row.Title, ChapterID: ids.Format(row.ChapterID),
		DueAt: row.DueAt, MaxAttempts: row.MaxAttempts, LatePolicy: row.LatePolicy,
		LatePenalty: row.LatePenalty, Status: row.Status, Items: items,
	}
}

// assignmentItemDTOFromSnapshot 转换题目引用投影为响应 DTO。
func assignmentItemDTOFromSnapshot(row AssignmentItemSnapshot, face map[string]any) AssignmentItemDTO {
	return AssignmentItemDTO{
		ID: ids.Format(row.ID), ItemCode: row.ItemCode, ItemVersion: row.ItemVersion, Score: row.Score,
		Seq: row.Seq, GradingMode: row.GradingMode, JudgerCode: row.JudgerCode, Face: face,
	}
}

// assignmentItemDTOsFromSnapshots 批量转换题目引用投影。
func assignmentItemDTOsFromSnapshots(rows []AssignmentItemSnapshot) []AssignmentItemDTO {
	out := make([]AssignmentItemDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, assignmentItemDTOFromSnapshot(row, nil))
	}
	return out
}

// hasAutoGrading 判断作业是否包含自动判题项目。
func hasAutoGrading(items []AssignmentItemSnapshot) bool {
	for _, item := range items {
		if item.GradingMode == GradingModeAuto {
			return true
		}
	}
	return false
}

// firstAutoItem 返回第一个自动判题项目。
func firstAutoItem(items []AssignmentItemSnapshot) AssignmentItemSnapshot {
	for _, item := range items {
		if item.GradingMode == GradingModeAuto {
			return item
		}
	}
	return AssignmentItemSnapshot{}
}

// submissionDTOFromScoreSnapshot 转换提交评分投影为响应 DTO。
func submissionDTOFromScoreSnapshot(row SubmissionScoreSnapshot) SubmissionDTO {
	return SubmissionDTO{
		ID: ids.Format(row.ID), AssignmentID: ids.Format(row.AssignmentID), StudentID: ids.Format(row.StudentID),
		AttemptNo: row.AttemptNo, ContentRef: row.ContentRef, JudgeTaskRef: row.JudgeTaskRef,
		AutoScore: row.AutoScore, ManualScore: row.ManualScore, FinalScore: row.FinalScore,
		Comment: row.Comment, IsLate: row.IsLate, Status: row.Status, SubmittedAt: row.SubmittedAt,
	}
}

// submissionDTOsFromScoreSnapshots 批量转换提交评分投影。
func submissionDTOsFromScoreSnapshots(rows []SubmissionScoreSnapshot) []SubmissionDTO {
	out := make([]SubmissionDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, submissionDTOFromScoreSnapshot(row))
	}
	return out
}

// gradeDTOFromCourseGradeSnapshot 转换成绩投影为响应 DTO。
func gradeDTOFromCourseGradeSnapshot(row CourseGradeSnapshot) CourseGradeDTO {
	return CourseGradeDTO{
		ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), StudentID: ids.Format(row.StudentID),
		AutoTotal: row.AutoTotal, OverrideTotal: row.OverrideTotal, FinalTotal: row.FinalTotal, IsOverridden: row.IsOverridden,
	}
}

// gradeAuditSnapshot 转换成绩行供审计记录 old/new 值。
func gradeAuditSnapshot(row CourseGradeSnapshot) map[string]any {
	return map[string]any{
		"auto_total": row.AutoTotal, "override_total": row.OverrideTotal,
		"final_total": row.FinalTotal, "is_overridden": row.IsOverridden,
	}
}
