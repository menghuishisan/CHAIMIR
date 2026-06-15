// teaching service_assignment 文件实现作业、提交、草稿和自动判题派发业务。
package teaching

import (
	"context"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// CreateAssignment 创建作业草稿并锁定 M5 题目版本。
func (s *Service) CreateAssignment(ctx context.Context, courseID int64, req AssignmentRequest) (AssignmentDetailDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return AssignmentDetailDTO{}, err
	}
	req, due, err := validateAssignmentRequest(req)
	if err != nil {
		return AssignmentDetailDTO{}, err
	}
	assignment := Assignment{ID: s.ids.Generate(), TenantID: id.TenantID, CourseID: courseID, Title: req.Title, ChapterID: req.ChapterID, DueAt: due, MaxAttempts: req.MaxAttempts, LatePolicy: req.LatePolicy, LatePenalty: req.LatePenalty, Status: AssignmentStatusDraft}
	items := make([]AssignmentItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, AssignmentItem{ID: s.ids.Generate(), TenantID: id.TenantID, AssignmentID: assignment.ID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, Seq: item.Seq, GradingMode: item.GradingMode, JudgerCode: item.JudgerCode})
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		for _, item := range items {
			if _, err := s.content.GetContentFace(ctx, id.TenantID, contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion}); err != nil {
				return apperr.ErrTeachingAssignmentInvalid.WithCause(err)
			}
			if err := s.content.IncrementUsage(ctx, id.TenantID, contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion}); err != nil {
				return apperr.ErrTeachingAssignmentInvalid.WithCause(err)
			}
		}
		assignment, err = tx.CreateAssignment(ctx, assignment)
		if err != nil {
			return err
		}
		items, err = tx.ReplaceAssignmentItems(ctx, id.TenantID, assignment.ID, items)
		return err
	}); err != nil {
		return AssignmentDetailDTO{}, mapAssignmentError(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "teaching.assignment.create", auditTargetAssignment, assignment.ID, map[string]any{"course_id": courseID}); err != nil {
		return AssignmentDetailDTO{}, err
	}
	return assignmentDetailDTO(AssignmentDetail{Assignment: assignment, Items: assignmentItemFaces(items)}), nil
}

// UpdateAssignment 更新草稿作业。
func (s *Service) UpdateAssignment(ctx context.Context, assignmentID int64, req AssignmentRequest) (AssignmentDetailDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return AssignmentDetailDTO{}, err
	}
	req, due, err := validateAssignmentRequest(req)
	if err != nil {
		return AssignmentDetailDTO{}, err
	}
	var assignment Assignment
	var items []AssignmentItem
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetAssignment(ctx, id.TenantID, assignmentID)
		if err != nil {
			return err
		}
		course, err := tx.GetCourse(ctx, id.TenantID, current.CourseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		current.Title, current.ChapterID, current.DueAt, current.MaxAttempts, current.LatePolicy, current.LatePenalty = req.Title, req.ChapterID, due, req.MaxAttempts, req.LatePolicy, req.LatePenalty
		assignment, err = tx.UpdateAssignment(ctx, current)
		if err != nil {
			return err
		}
		next := make([]AssignmentItem, 0, len(req.Items))
		for _, item := range req.Items {
			next = append(next, AssignmentItem{ID: s.ids.Generate(), TenantID: id.TenantID, AssignmentID: assignmentID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, Seq: item.Seq, GradingMode: item.GradingMode, JudgerCode: item.JudgerCode})
		}
		items, err = tx.ReplaceAssignmentItems(ctx, id.TenantID, assignmentID, next)
		return err
	}); err != nil {
		return AssignmentDetailDTO{}, mapAssignmentError(err)
	}
	return assignmentDetailDTO(AssignmentDetail{Assignment: assignment, Items: assignmentItemFaces(items)}), nil
}

// PublishAssignment 发布作业。
func (s *Service) PublishAssignment(ctx context.Context, assignmentID int64) (AssignmentDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return AssignmentDTO{}, err
	}
	var assignment Assignment
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetAssignment(ctx, id.TenantID, assignmentID)
		if err != nil {
			return err
		}
		course, err := tx.GetCourse(ctx, id.TenantID, current.CourseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		items, err := tx.ListAssignmentItems(ctx, id.TenantID, assignmentID)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return apperr.ErrTeachingAssignmentStateInvalid
		}
		assignment, err = tx.PublishAssignment(ctx, id.TenantID, assignmentID)
		return err
	}); err != nil {
		return AssignmentDTO{}, mapAssignmentError(err)
	}
	return assignmentDTO(assignment), nil
}

// GetAssignmentForStudent 读取作业详情并从 M5 展开题面。
func (s *Service) GetAssignmentForStudent(ctx context.Context, assignmentID int64) (AssignmentDetailDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return AssignmentDetailDTO{}, err
	}
	var assignment Assignment
	var items []AssignmentItem
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		assignment, err = tx.GetAssignment(ctx, id.TenantID, assignmentID)
		if err != nil {
			return err
		}
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, assignment.CourseID, id.AccountID); err != nil {
			return err
		}
		items, err = tx.ListAssignmentItems(ctx, id.TenantID, assignmentID)
		return err
	}); err != nil {
		return AssignmentDetailDTO{}, mapAssignmentError(err)
	}
	detail := AssignmentDetail{Assignment: assignment, Items: make([]AssignmentItemFace, 0, len(items))}
	for _, item := range items {
		face, err := s.content.GetContentFace(ctx, id.TenantID, contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion})
		if err != nil {
			return AssignmentDetailDTO{}, apperr.ErrTeachingAssignmentInvalid.WithCause(err)
		}
		detail.Items = append(detail.Items, AssignmentItemFace{AssignmentItem: item, Title: face.Title, Type: face.Type, Difficulty: face.Difficulty, Body: face.Body})
	}
	return assignmentDetailDTO(detail), nil
}

// SaveDraft 保存服务端权威作答草稿。
func (s *Service) SaveDraft(ctx context.Context, assignmentID int64, req DraftRequest) (map[string]any, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	req, err = validateDraftRequest(req)
	if err != nil {
		return nil, err
	}
	var draft SubmissionDraft
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		assignment, err := tx.GetAssignment(ctx, id.TenantID, assignmentID)
		if err != nil {
			return err
		}
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, assignment.CourseID, id.AccountID); err != nil {
			return err
		}
		draft, err = tx.UpsertDraft(ctx, SubmissionDraft{ID: s.ids.Generate(), TenantID: id.TenantID, AssignmentID: assignmentID, StudentID: id.AccountID, Content: req.Content})
		return err
	}); err != nil {
		return nil, mapAssignmentError(err)
	}
	return map[string]any{"updated_at": formatTime(draft.UpdatedAt)}, nil
}

// GetDraft 读取服务端权威作答草稿,不存在时显式返回 exists=false。
func (s *Service) GetDraft(ctx context.Context, assignmentID int64) (DraftDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return DraftDTO{}, err
	}
	var draft SubmissionDraft
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		assignment, err := tx.GetAssignment(ctx, id.TenantID, assignmentID)
		if err != nil {
			return err
		}
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, assignment.CourseID, id.AccountID); err != nil {
			return err
		}
		draft, err = tx.GetDraft(ctx, id.TenantID, assignmentID, id.AccountID)
		if isNoRows(err) {
			draft = SubmissionDraft{TenantID: id.TenantID, AssignmentID: assignmentID, StudentID: id.AccountID, Content: map[string]any{}}
			return nil
		}
		return err
	}); err != nil {
		return DraftDTO{}, mapAssignmentError(err)
	}
	if draft.ID == 0 {
		return DraftDTO{AssignmentID: assignmentID, StudentID: id.AccountID, Content: map[string]any{}, Exists: false}, nil
	}
	return draftDTO(draft), nil
}

// SubmitAssignment 创建正式提交并写入自动判题 outbox。
func (s *Service) SubmitAssignment(ctx context.Context, assignmentID int64, req SubmitAssignmentRequest) (SubmissionDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return SubmissionDTO{}, err
	}
	req, err = validateSubmissionRequest(req)
	if err != nil {
		return SubmissionDTO{}, err
	}
	var sub Submission
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		assignment, err := tx.GetAssignment(ctx, id.TenantID, assignmentID)
		if err != nil {
			return err
		}
		if assignment.Status != AssignmentStatusPublished {
			return apperr.ErrTeachingAssignmentStateInvalid
		}
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, assignment.CourseID, id.AccountID); err != nil {
			return err
		}
		attempts, err := tx.CountStudentAttempts(ctx, id.TenantID, assignmentID, id.AccountID)
		if err != nil {
			return err
		}
		if attempts >= int64(assignment.MaxAttempts) {
			return apperr.ErrTeachingSubmissionLimitExceeded
		}
		isLate, err := applyLatePolicy(assignment, timex.Now())
		if err != nil {
			return err
		}
		items, err := tx.ListAssignmentItems(ctx, id.TenantID, assignmentID)
		if err != nil {
			return err
		}
		status := SubmissionStatusPending
		final := int32(0)
		hasAuto := false
		for _, item := range items {
			if item.GradingMode == GradingModeAuto {
				hasAuto = true
				status = SubmissionStatusSubmitted
				break
			}
		}
		sub, err = tx.CreateSubmission(ctx, Submission{ID: s.ids.Generate(), TenantID: id.TenantID, AssignmentID: assignmentID, StudentID: id.AccountID, AttemptNo: int32(attempts + 1), ContentRef: req.ContentRef, FinalScore: final, IsLate: isLate, Status: status})
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.GradingMode != GradingModeAuto {
				continue
			}
			codeKey, _ := req.ContentRef["code_storage_key"].(string)
			codeHash, _ := req.ContentRef["code_hash"].(string)
			if codeKey == "" || codeHash == "" {
				return apperr.ErrTeachingSubmissionInvalid
			}
			if _, err := tx.CreateJudgeOutbox(ctx, JudgeOutbox{ID: s.ids.Generate(), TenantID: id.TenantID, SubmissionID: sub.ID, AssignmentItemID: item.ID, AssignmentID: assignmentID, StudentID: id.AccountID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, JudgerCode: item.JudgerCode, CodeStorageKey: codeKey, CodeHash: codeHash, ExtraInput: map[string]any{"assignment_item_id": item.ID}, SourceRef: sourceRefForSubmissionItem(sub.ID, item.ID)}); err != nil {
				return err
			}
		}
		if !hasAuto {
			sub.Status = SubmissionStatusPending
		}
		if err := tx.DeleteDraft(ctx, id.TenantID, assignmentID, id.AccountID); err != nil && !isNoRows(err) {
			return err
		}
		return nil
	}); err != nil {
		return SubmissionDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumStudent, "teaching.assignment.submit", auditTargetSubmission, sub.ID, map[string]any{"assignment_id": assignmentID}); err != nil {
		return SubmissionDTO{}, err
	}
	return submissionDTO(sub), nil
}

// GradeSubmission 教师批改主观题或报告提交。
func (s *Service) GradeSubmission(ctx context.Context, submissionID int64, req GradeSubmissionRequest) (SubmissionDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return SubmissionDTO{}, err
	}
	req, err = validateGradeRequest(req)
	if err != nil {
		return SubmissionDTO{}, err
	}
	var sub Submission
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetSubmission(ctx, id.TenantID, submissionID)
		if err != nil {
			return err
		}
		assignment, err := tx.GetAssignment(ctx, id.TenantID, current.AssignmentID)
		if err != nil {
			return err
		}
		course, err := tx.GetCourse(ctx, id.TenantID, assignment.CourseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		final, err := applyLatePenalty(assignment, req.Score, current.IsLate)
		if err != nil {
			return err
		}
		sub, err = tx.UpdateSubmissionManualGrade(ctx, id.TenantID, submissionID, req.Score, final, req.Comment)
		return err
	}); err != nil {
		return SubmissionDTO{}, mapAssignmentError(err)
	}
	return submissionDTO(sub), nil
}

// ListSubmissions 查询作业提交情况。
func (s *Service) ListSubmissions(ctx context.Context, assignmentID int64, page, size int) ([]SubmissionDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	page, size = pagex.Normalize(page, size)
	var subs []Submission
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		assignment, err := tx.GetAssignment(ctx, id.TenantID, assignmentID)
		if err != nil {
			return err
		}
		course, err := tx.GetCourse(ctx, id.TenantID, assignment.CourseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		subs, total, err = tx.ListSubmissionsByAssignment(ctx, id.TenantID, assignmentID, page, size)
		return err
	}); err != nil {
		return nil, 0, 0, 0, mapAssignmentError(err)
	}
	out := make([]SubmissionDTO, 0, len(subs))
	for _, sub := range subs {
		out = append(out, submissionDTO(sub))
	}
	return out, total, page, size, nil
}

// GetSubmissionForUser 读取提交反馈。
func (s *Service) GetSubmissionForUser(ctx context.Context, submissionID int64) (SubmissionDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return SubmissionDTO{}, err
	}
	var sub Submission
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sub, err = tx.GetSubmission(ctx, id.TenantID, submissionID)
		if err != nil {
			return err
		}
		if sub.StudentID != id.AccountID {
			assignment, err := tx.GetAssignment(ctx, id.TenantID, sub.AssignmentID)
			if err != nil {
				return err
			}
			course, err := tx.GetCourse(ctx, id.TenantID, assignment.CourseID)
			if err != nil {
				return err
			}
			if err := ensureTeacherOwned(course, id.AccountID); err != nil {
				return apperr.ErrTeachingCourseForbidden
			}
		}
		return nil
	}); err != nil {
		return SubmissionDTO{}, mapAssignmentError(err)
	}
	return submissionDTO(sub), nil
}

// RunJudgeOutboxOnce 派发一轮 M6 本地自动判题 outbox。
func (s *Service) RunJudgeOutboxOnce(ctx context.Context, tenantID int64) error {
	if s.judge == nil {
		return apperr.ErrTeachingJudgeServiceUnavailable
	}
	var outboxes []JudgeOutbox
	claim := s.store.TenantTx
	if tenantID <= 0 {
		claim = func(ctx context.Context, _ int64, fn func(context.Context, TxStore) error) error {
			return s.store.PrivilegedTx(ctx, fn)
		}
	}
	if err := claim(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		if tenantID > 0 {
			outboxes, err = tx.ClaimJudgeOutbox(ctx, tenantID, int32(s.cfg.JudgeOutboxBatchSize))
			return err
		}
		outboxes, err = tx.ClaimJudgeOutboxAcrossTenants(ctx, int32(s.cfg.JudgeOutboxBatchSize))
		return err
	}); err != nil {
		return apperr.ErrTeachingJudgeOutboxInvalid.WithCause(err)
	}
	for _, item := range outboxes {
		info, err := s.judge.SubmitJudgeTask(ctx, contracts.JudgeSubmitRequest{TenantID: item.TenantID, JudgerCode: item.JudgerCode, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, CodeStorageKey: item.CodeStorageKey, CodeHash: item.CodeHash, SubmitterID: item.StudentID, SourceRef: item.SourceRef, SandboxMode: contracts.JudgeSandboxModeFresh, ExtraInput: item.ExtraInput, Priority: 5})
		if err != nil {
			if retryErr := s.store.TenantTx(ctx, item.TenantID, func(ctx context.Context, tx TxStore) error {
				_, retryErr := tx.RetryJudgeOutbox(ctx, item.TenantID, item.ID, safeStoredError(ctx, err))
				return retryErr
			}); retryErr != nil {
				return apperr.ErrTeachingJudgeOutboxInvalid.WithCause(retryErr)
			}
			continue
		}
		if err := s.store.TenantTx(ctx, item.TenantID, func(ctx context.Context, tx TxStore) error {
			if _, err := tx.UpdateSubmissionJudgeRef(ctx, item.TenantID, item.SubmissionID, fmt.Sprint(info.TaskID)); err != nil {
				return err
			}
			_, err := tx.CompleteJudgeOutbox(ctx, item.TenantID, item.ID)
			return err
		}); err != nil {
			return apperr.ErrTeachingJudgeOutboxInvalid.WithCause(err)
		}
	}
	return nil
}

// safeStoredError 只持久化错误码、用户向文案和 trace_id,详细原因保留在日志链路中。
func safeStoredError(ctx context.Context, err error) string {
	ae := apperr.AsAppError(err)
	if ae == nil {
		return ""
	}
	return fmt.Sprintf("code=%s message=%s trace_id=%s", ae.UserCode(), ae.UserMessage(), response.TraceFromContext(ctx))
}

// HandleJudgeCompleted 处理 M3 判题完成事件。
func (s *Service) HandleJudgeCompleted(ctx context.Context, event contracts.JudgeCompletedEvent) error {
	var sub Submission
	var ready bool
	if err := s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetSubmissionBySourceRef(ctx, event.TenantID, event.SourceRef)
		if err != nil {
			return err
		}
		if _, err := tx.MarkJudgeOutboxResult(ctx, event.TenantID, event.SourceRef, event.Score, event.FinishedAt); err != nil {
			return err
		}
		outboxes, err := tx.ListJudgeOutboxBySubmission(ctx, event.TenantID, current.ID)
		if err != nil {
			return err
		}
		total, allDone := aggregateCompletedAutoScore(outboxes)
		if !allDone {
			sub = current
			return nil
		}
		assignment, err := tx.GetAssignment(ctx, event.TenantID, current.AssignmentID)
		if err != nil {
			return err
		}
		final, err := applyLatePenalty(assignment, total, current.IsLate)
		if err != nil {
			return err
		}
		sub, err = tx.UpdateSubmissionAutoScore(ctx, event.TenantID, current.ID, total, final)
		ready = true
		return err
	}); err != nil {
		return apperr.ErrTeachingSubmissionInvalid.WithCause(err)
	}
	if !ready {
		return nil
	}
	return s.publishGradeUpdated(ctx, event.TenantID, sub.AssignmentID, sub.StudentID)
}

// HandleJudgeFailed 处理 M3 判题失败事件。
func (s *Service) HandleJudgeFailed(ctx context.Context, event contracts.JudgeFailedEvent) error {
	return s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkJudgeOutboxFailedResult(ctx, event.TenantID, event.SourceRef, event.Reason, event.FailedAt)
		if err != nil {
			return apperr.ErrTeachingSubmissionInvalid.WithCause(err)
		}
		return nil
	})
}

// aggregateCompletedAutoScore 汇总一次提交内所有自动题得分,存在未完成题则不结算提交。
func aggregateCompletedAutoScore(outboxes []JudgeOutbox) (int32, bool) {
	if len(outboxes) == 0 {
		return 0, false
	}
	var total int32
	for _, outbox := range outboxes {
		if outbox.CompletedAt.IsZero() || outbox.LastError != "" {
			return 0, false
		}
		total += outbox.Score
	}
	return total, true
}

// sourceRefForSubmissionItem 构造 M6 提交题目来源标识。
func sourceRefForSubmissionItem(submissionID, assignmentItemID int64) string {
	return fmt.Sprintf("teaching:%d:submission:%d:item:%d", timex.Now().Year(), submissionID, assignmentItemID)
}
