// teaching repo_assignment 文件封装作业、提交、草稿和判题 outbox 数据访问。
package teaching

import (
	"context"
	"time"

	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// CreateAssignment 创建作业草稿。
func (s *txStore) CreateAssignment(ctx context.Context, assignment Assignment) (Assignment, error) {
	penalty, err := encodeMap(assignment.LatePenalty)
	if err != nil {
		return Assignment{}, err
	}
	row, err := s.q.CreateAssignment(ctx, sqlcgen.CreateAssignmentParams{ID: assignment.ID, TenantID: assignment.TenantID, CourseID: assignment.CourseID, Title: assignment.Title, ChapterID: pgtypex.Int8(assignment.ChapterID), DueAt: timex.Timestamptz(assignment.DueAt), MaxAttempts: assignment.MaxAttempts, LatePolicy: assignment.LatePolicy, LatePenalty: penalty, Status: assignment.Status})
	if err != nil {
		return Assignment{}, err
	}
	return assignmentFromRow(row)
}

// GetAssignment 读取作业。
func (s *txStore) GetAssignment(ctx context.Context, tenantID, id int64) (Assignment, error) {
	row, err := s.q.GetAssignment(ctx, sqlcgen.GetAssignmentParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Assignment{}, err
	}
	return assignmentFromRow(row)
}

// ListAssignmentsByCourse 查询课程作业。
func (s *txStore) ListAssignmentsByCourse(ctx context.Context, tenantID, courseID int64) ([]Assignment, error) {
	rows, err := s.q.ListAssignmentsByCourse(ctx, sqlcgen.ListAssignmentsByCourseParams{TenantID: tenantID, CourseID: courseID})
	if err != nil {
		return nil, err
	}
	out := make([]Assignment, 0, len(rows))
	for _, row := range rows {
		item, err := assignmentFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// UpdateAssignment 更新草稿作业。
func (s *txStore) UpdateAssignment(ctx context.Context, assignment Assignment) (Assignment, error) {
	penalty, err := encodeMap(assignment.LatePenalty)
	if err != nil {
		return Assignment{}, err
	}
	row, err := s.q.UpdateAssignment(ctx, sqlcgen.UpdateAssignmentParams{TenantID: assignment.TenantID, ID: assignment.ID, Title: assignment.Title, ChapterID: pgtypex.Int8(assignment.ChapterID), DueAt: timex.Timestamptz(assignment.DueAt), MaxAttempts: assignment.MaxAttempts, LatePolicy: assignment.LatePolicy, LatePenalty: penalty})
	if err != nil {
		return Assignment{}, err
	}
	return assignmentFromRow(row)
}

// PublishAssignment 发布作业。
func (s *txStore) PublishAssignment(ctx context.Context, tenantID, id int64) (Assignment, error) {
	row, err := s.q.PublishAssignment(ctx, sqlcgen.PublishAssignmentParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Assignment{}, err
	}
	return assignmentFromRow(row)
}

// ReplaceAssignmentItems 覆盖作业题目引用。
func (s *txStore) ReplaceAssignmentItems(ctx context.Context, tenantID, assignmentID int64, items []AssignmentItem) ([]AssignmentItem, error) {
	if err := s.q.DeleteAssignmentItems(ctx, sqlcgen.DeleteAssignmentItemsParams{TenantID: tenantID, AssignmentID: assignmentID}); err != nil {
		return nil, err
	}
	out := make([]AssignmentItem, 0, len(items))
	for _, item := range items {
		row, err := s.q.CreateAssignmentItem(ctx, sqlcgen.CreateAssignmentItemParams{ID: item.ID, TenantID: tenantID, AssignmentID: assignmentID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, Seq: item.Seq, GradingMode: item.GradingMode, JudgerCode: pgtypex.Text(item.JudgerCode)})
		if err != nil {
			return nil, err
		}
		out = append(out, assignmentItemFromRow(row))
	}
	return out, nil
}

// ListAssignmentItems 查询作业题目引用。
func (s *txStore) ListAssignmentItems(ctx context.Context, tenantID, assignmentID int64) ([]AssignmentItem, error) {
	rows, err := s.q.ListAssignmentItems(ctx, sqlcgen.ListAssignmentItemsParams{TenantID: tenantID, AssignmentID: assignmentID})
	if err != nil {
		return nil, err
	}
	out := make([]AssignmentItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, assignmentItemFromRow(row))
	}
	return out, nil
}

// CountAssignmentSubmissions 统计作业提交数量。
func (s *txStore) CountAssignmentSubmissions(ctx context.Context, tenantID, assignmentID int64) (int64, error) {
	return s.q.CountAssignmentSubmissions(ctx, sqlcgen.CountAssignmentSubmissionsParams{TenantID: tenantID, AssignmentID: assignmentID})
}

// CountStudentAttempts 统计学生作业提交次数。
func (s *txStore) CountStudentAttempts(ctx context.Context, tenantID, assignmentID, studentID int64) (int64, error) {
	return s.q.CountStudentAttempts(ctx, sqlcgen.CountStudentAttemptsParams{TenantID: tenantID, AssignmentID: assignmentID, StudentID: studentID})
}

// CreateSubmission 创建正式提交。
func (s *txStore) CreateSubmission(ctx context.Context, sub Submission) (Submission, error) {
	content, err := encodeMap(sub.ContentRef)
	if err != nil {
		return Submission{}, err
	}
	row, err := s.q.CreateSubmission(ctx, sqlcgen.CreateSubmissionParams{ID: sub.ID, TenantID: sub.TenantID, AssignmentID: sub.AssignmentID, StudentID: sub.StudentID, AttemptNo: sub.AttemptNo, ContentRef: content, FinalScore: pgtypex.Int4(sub.FinalScore), IsLate: sub.IsLate, Status: sub.Status})
	if err != nil {
		return Submission{}, err
	}
	return submissionFromRow(row)
}

// GetSubmission 读取提交。
func (s *txStore) GetSubmission(ctx context.Context, tenantID, id int64) (Submission, error) {
	row, err := s.q.GetSubmission(ctx, sqlcgen.GetSubmissionParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Submission{}, err
	}
	return submissionFromRow(row)
}

// GetSubmissionBySourceRef 按判题来源读取提交。
func (s *txStore) GetSubmissionBySourceRef(ctx context.Context, tenantID int64, sourceRef string) (Submission, error) {
	row, err := s.q.GetSubmissionBySourceRef(ctx, sqlcgen.GetSubmissionBySourceRefParams{TenantID: tenantID, SourceRef: sourceRef})
	if err != nil {
		return Submission{}, err
	}
	return submissionFromRow(row)
}

// ListJudgeOutboxBySubmission 查询同一次提交的所有自动判题派发记录。
func (s *txStore) ListJudgeOutboxBySubmission(ctx context.Context, tenantID, submissionID int64) ([]JudgeOutbox, error) {
	rows, err := s.q.ListJudgeOutboxBySubmission(ctx, sqlcgen.ListJudgeOutboxBySubmissionParams{TenantID: tenantID, SubmissionID: submissionID})
	if err != nil {
		return nil, err
	}
	out := make([]JudgeOutbox, 0, len(rows))
	for _, row := range rows {
		item, err := outboxFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// ListSubmissionsByAssignment 查询作业提交分页。
func (s *txStore) ListSubmissionsByAssignment(ctx context.Context, tenantID, assignmentID int64, page, size int) ([]Submission, int64, error) {
	rows, err := s.q.ListSubmissionsByAssignment(ctx, sqlcgen.ListSubmissionsByAssignmentParams{TenantID: tenantID, AssignmentID: assignmentID, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountSubmissionsByAssignment(ctx, sqlcgen.CountSubmissionsByAssignmentParams{TenantID: tenantID, AssignmentID: assignmentID})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Submission, 0, len(rows))
	for _, row := range rows {
		item, err := submissionFromRow(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, nil
}

// UpdateSubmissionManualGrade 更新教师批改分。
func (s *txStore) UpdateSubmissionManualGrade(ctx context.Context, tenantID, id int64, manual, final int32, comment string) (Submission, error) {
	row, err := s.q.UpdateSubmissionManualGrade(ctx, sqlcgen.UpdateSubmissionManualGradeParams{TenantID: tenantID, ID: id, ManualScore: pgtypex.Int4(manual), FinalScore: pgtypex.Int4(final), Comment: pgtypex.Text(comment)})
	if err != nil {
		return Submission{}, err
	}
	return submissionFromRow(row)
}

// UpdateSubmissionJudgeRef 写回 M3 判题任务引用。
func (s *txStore) UpdateSubmissionJudgeRef(ctx context.Context, tenantID, id int64, taskRef string) (Submission, error) {
	row, err := s.q.UpdateSubmissionJudgeRef(ctx, sqlcgen.UpdateSubmissionJudgeRefParams{TenantID: tenantID, ID: id, JudgeTaskRef: pgtypex.Text(taskRef)})
	if err != nil {
		return Submission{}, err
	}
	return submissionFromRow(row)
}

// UpdateSubmissionAutoScore 按提交聚合自动分和迟交处理后的最终分。
func (s *txStore) UpdateSubmissionAutoScore(ctx context.Context, tenantID, submissionID int64, score, final int32) (Submission, error) {
	row, err := s.q.UpdateSubmissionAutoScore(ctx, sqlcgen.UpdateSubmissionAutoScoreParams{TenantID: tenantID, ID: submissionID, AutoScore: pgtypex.Int4(score), FinalScore: pgtypex.Int4(final)})
	if err != nil {
		return Submission{}, err
	}
	return submissionFromRow(row)
}

// CreateJudgeOutbox 创建自动判题 outbox。
func (s *txStore) CreateJudgeOutbox(ctx context.Context, outbox JudgeOutbox) (JudgeOutbox, error) {
	extra, err := encodeMap(outbox.ExtraInput)
	if err != nil {
		return JudgeOutbox{}, err
	}
	row, err := s.q.CreateJudgeOutbox(ctx, sqlcgen.CreateJudgeOutboxParams{ID: outbox.ID, TenantID: outbox.TenantID, SubmissionID: outbox.SubmissionID, AssignmentItemID: outbox.AssignmentItemID, AssignmentID: outbox.AssignmentID, SourceOwnerID: outbox.SourceOwnerID, SourceCourseID: outbox.SourceCourseID, SourceScope: outbox.SourceScope, StudentID: outbox.StudentID, ItemCode: outbox.ItemCode, ItemVersion: outbox.ItemVersion, JudgerCode: outbox.JudgerCode, CodeStorageKey: outbox.CodeStorageKey, CodeHash: outbox.CodeHash, ExtraInput: extra, SourceRef: outbox.SourceRef})
	if err != nil {
		return JudgeOutbox{}, err
	}
	return outboxFromRow(row)
}

// ClaimJudgeOutbox 声明待派发判题任务。
func (s *txStore) ClaimJudgeOutbox(ctx context.Context, tenantID int64, limit int32) ([]JudgeOutbox, error) {
	rows, err := s.q.ClaimJudgeOutbox(ctx, sqlcgen.ClaimJudgeOutboxParams{TenantID: tenantID, Limit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]JudgeOutbox, 0, len(rows))
	for _, row := range rows {
		item, err := outboxFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// ClaimJudgeOutboxAcrossTenants 声明所有租户待派发判题任务,仅供模块后台任务使用。
func (s *txStore) ClaimJudgeOutboxAcrossTenants(ctx context.Context, limit int32) ([]JudgeOutbox, error) {
	rows, err := s.q.ClaimJudgeOutboxAcrossTenants(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]JudgeOutbox, 0, len(rows))
	for _, row := range rows {
		item, err := outboxFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// CompleteJudgeOutbox 标记判题派发完成。
func (s *txStore) CompleteJudgeOutbox(ctx context.Context, tenantID, id int64) (JudgeOutbox, error) {
	row, err := s.q.CompleteJudgeOutbox(ctx, sqlcgen.CompleteJudgeOutboxParams{TenantID: tenantID, ID: id})
	if err != nil {
		return JudgeOutbox{}, err
	}
	return outboxFromRow(row)
}

// RetryJudgeOutbox 回退判题 outbox 到待派发。
func (s *txStore) RetryJudgeOutbox(ctx context.Context, tenantID, id int64, lastError string) (JudgeOutbox, error) {
	row, err := s.q.RetryJudgeOutbox(ctx, sqlcgen.RetryJudgeOutboxParams{TenantID: tenantID, ID: id, LastError: pgtypex.Text(lastError)})
	if err != nil {
		return JudgeOutbox{}, err
	}
	return outboxFromRow(row)
}

// MarkJudgeOutboxResult 回写单题判题结果。
func (s *txStore) MarkJudgeOutboxResult(ctx context.Context, tenantID int64, sourceRef string, score int32, completedAt time.Time) (JudgeOutbox, error) {
	row, err := s.q.MarkJudgeOutboxResult(ctx, sqlcgen.MarkJudgeOutboxResultParams{TenantID: tenantID, SourceRef: sourceRef, Score: pgtypex.Int4(score), CompletedAt: timex.Timestamptz(completedAt)})
	if err != nil {
		return JudgeOutbox{}, err
	}
	return outboxFromRow(row)
}

// MarkJudgeOutboxFailedResult 记录 M3 判题终态失败原因。
func (s *txStore) MarkJudgeOutboxFailedResult(ctx context.Context, tenantID int64, sourceRef, reason string, failedAt time.Time) (JudgeOutbox, error) {
	row, err := s.q.MarkJudgeOutboxFailedResult(ctx, sqlcgen.MarkJudgeOutboxFailedResultParams{TenantID: tenantID, SourceRef: sourceRef, LastError: pgtypex.Text(reason), CompletedAt: timex.Timestamptz(failedAt)})
	if err != nil {
		return JudgeOutbox{}, err
	}
	return outboxFromRow(row)
}

// UpsertDraft 保存作答草稿。
func (s *txStore) UpsertDraft(ctx context.Context, draft SubmissionDraft) (SubmissionDraft, error) {
	content, err := encodeMap(draft.Content)
	if err != nil {
		return SubmissionDraft{}, err
	}
	row, err := s.q.UpsertSubmissionDraft(ctx, sqlcgen.UpsertSubmissionDraftParams{ID: draft.ID, TenantID: draft.TenantID, AssignmentID: draft.AssignmentID, StudentID: draft.StudentID, Content: content})
	if err != nil {
		return SubmissionDraft{}, err
	}
	return draftFromRow(row)
}

// GetDraft 读取作答草稿。
func (s *txStore) GetDraft(ctx context.Context, tenantID, assignmentID, studentID int64) (SubmissionDraft, error) {
	row, err := s.q.GetSubmissionDraft(ctx, sqlcgen.GetSubmissionDraftParams{TenantID: tenantID, AssignmentID: assignmentID, StudentID: studentID})
	if err != nil {
		return SubmissionDraft{}, err
	}
	return draftFromRow(row)
}

// DeleteDraft 删除作答草稿。
func (s *txStore) DeleteDraft(ctx context.Context, tenantID, assignmentID, studentID int64) error {
	return s.q.DeleteSubmissionDraft(ctx, sqlcgen.DeleteSubmissionDraftParams{TenantID: tenantID, AssignmentID: assignmentID, StudentID: studentID})
}
