// M6 作业提交服务:作业题目引用、草稿、提交、判题提交与教师批改。
package teaching

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// CreateAssignment 创建作业并锁定 M5 内容版本。
func (s *Service) CreateAssignment(ctx context.Context, courseID int64, req AssignmentRequest) (AssignmentDTO, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return AssignmentDTO{}, err
	}
	if err := s.validateAssignmentRequest(ctx, req); err != nil {
		return AssignmentDTO{}, err
	}
	latePenalty, err := jsonx.ObjectBytes(req.LatePenalty, apperr.ErrAssignmentInvalid)
	if err != nil {
		return AssignmentDTO{}, err
	}
	id, _ := tenantFromContext(ctx)
	assignmentID := s.idgen.Generate()
	var row sqlcgen.Assignment
	var items []AssignmentItemDTO
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateAssignment(ctx, sqlcgen.CreateAssignmentParams{
			ID: assignmentID, TenantID: id.TenantID, CourseID: courseID, Title: req.Title, ChapterID: pgInt8(mustOptionalID(req.ChapterID)),
			DueAt: timex.RequiredTimestamptz(req.DueAt), MaxAttempts: req.MaxAttempts, LatePolicy: req.LatePolicy,
			LatePenalty: latePenalty, Status: AssignmentStatusDraft,
		})
		if createErr != nil {
			return createErr
		}
		items, createErr = s.replaceAssignmentItems(ctx, q, id.TenantID, assignmentID, req.Items)
		return createErr
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return AssignmentDTO{}, ae
		}
		return AssignmentDTO{}, apperr.ErrAssignmentInvalid.WithCause(err)
	}
	if err := s.recordAssignmentContentUsage(ctx, id.TenantID, req.Items); err != nil {
		return AssignmentDTO{}, err
	}
	return assignmentDTOFromRow(row, items), s.writeAudit(ctx, id.TenantID, auditActionAssignmentChange, auditTargetAssignment, assignmentID, map[string]any{"title": req.Title})
}

// UpdateAssignment 编辑草稿作业。
func (s *Service) UpdateAssignment(ctx context.Context, assignmentID int64, req AssignmentRequest) (AssignmentDTO, error) {
	assignment, err := s.loadAssignment(ctx, assignmentID)
	if err != nil {
		return AssignmentDTO{}, err
	}
	if assignment.Status != AssignmentStatusDraft {
		return AssignmentDTO{}, apperr.ErrAssignmentInvalidState
	}
	if err := s.ensureTeacherOfCourse(ctx, assignment.CourseID); err != nil {
		return AssignmentDTO{}, err
	}
	if err := s.validateAssignmentRequest(ctx, req); err != nil {
		return AssignmentDTO{}, err
	}
	latePenalty, err := jsonx.ObjectBytes(req.LatePenalty, apperr.ErrAssignmentInvalid)
	if err != nil {
		return AssignmentDTO{}, err
	}
	id, _ := tenantFromContext(ctx)
	var row sqlcgen.Assignment
	var items []AssignmentItemDTO
	var oldItems []sqlcgen.AssignmentItem
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		oldItems, updateErr = q.ListAssignmentItems(ctx, assignmentID)
		if updateErr != nil {
			return updateErr
		}
		row, updateErr = q.UpdateAssignment(ctx, sqlcgen.UpdateAssignmentParams{
			ID: assignmentID, Title: req.Title, ChapterID: pgInt8(mustOptionalID(req.ChapterID)),
			DueAt: timex.RequiredTimestamptz(req.DueAt), MaxAttempts: req.MaxAttempts, LatePolicy: req.LatePolicy, LatePenalty: latePenalty,
		})
		if updateErr != nil {
			return updateErr
		}
		if updateErr = q.DeleteAssignmentItems(ctx, assignmentID); updateErr != nil {
			return updateErr
		}
		items, updateErr = s.replaceAssignmentItems(ctx, q, id.TenantID, assignmentID, req.Items)
		return updateErr
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return AssignmentDTO{}, ae
		}
		return AssignmentDTO{}, apperr.ErrAssignmentInvalid.WithCause(err)
	}
	if err := s.recordAssignmentContentUsage(ctx, id.TenantID, newAssignmentRefs(oldItems, req.Items)); err != nil {
		return AssignmentDTO{}, err
	}
	return assignmentDTOFromRow(row, items), nil
}

// PublishAssignment 发布作业。
func (s *Service) PublishAssignment(ctx context.Context, assignmentID int64) (AssignmentDTO, error) {
	assignment, err := s.loadAssignment(ctx, assignmentID)
	if err != nil {
		return AssignmentDTO{}, err
	}
	if err := s.ensureTeacherOfCourse(ctx, assignment.CourseID); err != nil {
		return AssignmentDTO{}, err
	}
	var row sqlcgen.Assignment
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		row, updateErr = q.UpdateAssignmentStatus(ctx, sqlcgen.UpdateAssignmentStatusParams{ID: assignmentID, Status: AssignmentStatusPublished})
		return updateErr
	}); err != nil {
		return AssignmentDTO{}, apperr.ErrAssignmentInvalid.WithCause(err)
	}
	return assignmentDTOFromRow(row, nil), nil
}

// GetAssignment 查询作业详情并展开 M5 题面。
func (s *Service) GetAssignment(ctx context.Context, assignmentID int64) (AssignmentDTO, error) {
	assignment, err := s.loadAssignment(ctx, assignmentID)
	if err != nil {
		return AssignmentDTO{}, err
	}
	if err := s.ensureAssignmentAccessible(ctx, assignment); err != nil {
		return AssignmentDTO{}, err
	}
	items, err := s.listAssignmentItemsWithFace(ctx, assignmentID)
	if err != nil {
		return AssignmentDTO{}, err
	}
	return assignmentDTOFromRow(assignment, items), nil
}

// SaveDraft 保存学生作答草稿。
func (s *Service) SaveDraft(ctx context.Context, assignmentID int64, content map[string]any) (map[string]any, error) {
	assignment, err := s.loadAssignment(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	if err := s.ensurePublishedAssignment(assignment); err != nil {
		return nil, err
	}
	tenantID, studentID, err := s.ensureStudentCourseMember(ctx, assignment.CourseID, apperr.ErrSubmissionForbidden)
	if err != nil {
		return nil, err
	}
	data, err := jsonx.ObjectBytes(content, apperr.ErrSubmissionInvalid)
	if err != nil {
		return nil, err
	}
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, draftErr := q.UpsertSubmissionDraft(ctx, sqlcgen.UpsertSubmissionDraftParams{ID: s.idgen.Generate(), TenantID: tenantID, AssignmentID: assignmentID, StudentID: studentID, Content: data})
		if db.IsNoRows(draftErr) {
			return apperr.ErrSubmissionForbidden
		}
		return draftErr
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrSubmissionInvalid.WithCause(err)
	}
	return map[string]any{"saved": true}, nil
}

// GetDraft 读取当前学生的服务端作答草稿。
func (s *Service) GetDraft(ctx context.Context, assignmentID int64) (map[string]any, error) {
	assignment, err := s.loadAssignment(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	if err := s.ensurePublishedAssignment(assignment); err != nil {
		return nil, err
	}
	_, studentID, err := s.ensureStudentCourseMember(ctx, assignment.CourseID, apperr.ErrSubmissionForbidden)
	if err != nil {
		return nil, err
	}
	var content map[string]any
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		draft, err := q.GetSubmissionDraft(ctx, sqlcgen.GetSubmissionDraftParams{AssignmentID: assignmentID, StudentID: studentID})
		if db.IsNoRows(err) {
			content = map[string]any{}
			return nil
		}
		if err != nil {
			return err
		}
		content = jsonx.ObjectMap(draft.Content)
		return nil
	}); err != nil {
		return nil, apperr.ErrSubmissionQueryFailed.WithCause(err)
	}
	return content, nil
}

// SubmitAssignment 创建提交,自动题经 M3 判题。
func (s *Service) SubmitAssignment(ctx context.Context, assignmentID int64, req SubmitRequest) (SubmissionDTO, error) {
	assignment, err := s.loadAssignment(ctx, assignmentID)
	if err != nil {
		return SubmissionDTO{}, err
	}
	if assignment.Status != AssignmentStatusPublished {
		return SubmissionDTO{}, s.ensurePublishedAssignment(assignment)
	}
	tenantID, studentID, err := s.ensureStudentCourseMember(ctx, assignment.CourseID, apperr.ErrSubmissionForbidden)
	if err != nil {
		return SubmissionDTO{}, err
	}
	content, err := jsonx.ObjectBytes(req.ContentRef, apperr.ErrSubmissionInvalid)
	if err != nil {
		return SubmissionDTO{}, err
	}
	attempt, err := s.nextAttempt(ctx, assignment, studentID)
	if err != nil {
		return SubmissionDTO{}, err
	}
	late, err := applyLatePolicy(assignment.DueAt.Time, timex.Now(), assignment.LatePolicy, jsonx.ObjectMap(assignment.LatePenalty), 0)
	if err != nil && assignment.LatePolicy == LatePolicyReject {
		return SubmissionDTO{}, err
	}
	items, err := s.listAssignmentItemRows(ctx, assignmentID)
	if err != nil {
		return SubmissionDTO{}, err
	}
	status := SubmissionStatusPending
	if !hasAutoGrading(items) {
		status = SubmissionStatusSubmitted
	}
	submissionID := s.idgen.Generate()
	var row sqlcgen.Submission
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateSubmission(ctx, sqlcgen.CreateSubmissionParams{
			ID: submissionID, TenantID: tenantID, AssignmentID: assignmentID, StudentID: studentID, AttemptNo: attempt,
			ContentRef: content, JudgeTaskRef: pgText(""), IsLate: late.IsLate, Status: status,
		})
		if db.IsNoRows(createErr) {
			return apperr.ErrSubmissionForbidden
		}
		if createErr != nil {
			return createErr
		}
		if hasAutoGrading(items) {
			if _, createErr = s.createSubmissionJudgeOutbox(ctx, q, tenantID, assignmentID, submissionID, studentID, req, items); createErr != nil {
				return createErr
			}
		}
		return q.DeleteSubmissionDraft(ctx, sqlcgen.DeleteSubmissionDraftParams{AssignmentID: assignmentID, StudentID: studentID})
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return SubmissionDTO{}, ae
		}
		return SubmissionDTO{}, apperr.ErrSubmissionInvalid.WithCause(err)
	}
	if hasAutoGrading(items) {
		if err := s.dispatchPendingSubmissionJudgesForTenant(ctx, tenantID); err != nil {
			logging.ErrorContext(ctx, "teaching judge outbox dispatch failed", err.Error(),
				slog.Int64("tenant_id", tenantID),
				slog.Int64("submission_id", submissionID),
			)
		}
	}
	return submissionDTOFromRow(row), nil
}

// ListSubmissions 查询作业提交列表。
func (s *Service) ListSubmissions(ctx context.Context, assignmentID int64, page, size int) ([]SubmissionDTO, error) {
	assignment, err := s.loadAssignment(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureTeacherOfCourse(ctx, assignment.CourseID); err != nil {
		return nil, err
	}
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.Submission
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, findErr := q.ListSubmissionsByAssignment(ctx, sqlcgen.ListSubmissionsByAssignmentParams{AssignmentID: assignmentID, LimitCount: int32(size), OffsetCount: int32((page - 1) * size)})
		rows = found
		return findErr
	}); err != nil {
		return nil, apperr.ErrSubmissionQueryFailed.WithCause(err)
	}
	return submissionDTOsFromRows(rows), nil
}

// GradeSubmission 写入教师人工批改分。
func (s *Service) GradeSubmission(ctx context.Context, submissionID int64, req GradeSubmissionRequest) (SubmissionDTO, error) {
	if req.Score < 0 || req.Score > 100 {
		return SubmissionDTO{}, apperr.ErrGradeInvalid
	}
	submission, err := s.loadSubmission(ctx, submissionID)
	if err != nil {
		return SubmissionDTO{}, err
	}
	assignment, err := s.loadAssignment(ctx, submission.AssignmentID)
	if err != nil {
		return SubmissionDTO{}, err
	}
	if err := s.ensureTeacherOfCourse(ctx, assignment.CourseID); err != nil {
		return SubmissionDTO{}, err
	}
	var row sqlcgen.Submission
	finalScore, err := finalScoreForSubmission(assignment, submission, req.Score)
	if err != nil {
		return SubmissionDTO{}, err
	}
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		row, updateErr = q.UpdateSubmissionManualScore(ctx, sqlcgen.UpdateSubmissionManualScoreParams{ID: submissionID, ManualScore: pgInt4(req.Score), FinalScore: pgInt4(finalScore), Comment: pgText(req.Comment), Status: SubmissionStatusGraded})
		return updateErr
	}); err != nil {
		return SubmissionDTO{}, apperr.ErrGradeInvalid.WithCause(err)
	}
	id, _ := tenantFromContext(ctx)
	return submissionDTOFromRow(row), s.writeAudit(ctx, id.TenantID, auditActionSubmissionGrade, auditTargetSubmission, submissionID, map[string]any{"score": req.Score})
}

// GetSubmission 查询提交反馈。
func (s *Service) GetSubmission(ctx context.Context, submissionID int64) (SubmissionDTO, error) {
	row, err := s.loadSubmission(ctx, submissionID)
	if err != nil {
		return SubmissionDTO{}, err
	}
	id, ok := tenantFromContext(ctx)
	if !ok {
		return SubmissionDTO{}, apperr.ErrUnauthorized
	}
	if row.StudentID != id.AccountID {
		assignment, err := s.loadAssignment(ctx, row.AssignmentID)
		if err != nil {
			return SubmissionDTO{}, err
		}
		if err := s.ensureTeacherOfCourse(ctx, assignment.CourseID); err != nil {
			return SubmissionDTO{}, err
		}
	}
	return submissionDTOFromRow(row), nil
}

// validateAssignmentRequest 校验作业与 M5 内容引用。
func (s *Service) validateAssignmentRequest(ctx context.Context, req AssignmentRequest) error {
	if strings.TrimSpace(req.Title) == "" || req.DueAt.IsZero() || req.MaxAttempts <= 0 || len(req.Items) == 0 {
		return apperr.ErrAssignmentInvalid
	}
	for _, item := range req.Items {
		if item.Score <= 0 || strings.TrimSpace(item.ItemCode) == "" || strings.TrimSpace(item.ItemVersion) == "" ||
			(item.GradingMode != GradingModeAuto && item.GradingMode != GradingModeManual) {
			return apperr.ErrAssignmentInvalid
		}
		if item.GradingMode == GradingModeAuto && strings.TrimSpace(item.JudgerCode) == "" {
			return apperr.ErrAssignmentInvalid
		}
		if s.content != nil {
			id, ok := tenantFromContext(ctx)
			if !ok {
				return apperr.ErrUnauthorized
			}
			if _, err := s.content.GetContentFace(ctx, id.TenantID, contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion}); err != nil {
				return apperr.ErrAssignmentInvalid.WithCause(err)
			}
		}
	}
	return nil
}

// finalScoreForSubmission 按提交时间和作业迟交策略计算最终得分。
func finalScoreForSubmission(assignment sqlcgen.Assignment, submission sqlcgen.Submission, score int32) (int32, error) {
	result, err := applyLatePolicy(assignment.DueAt.Time, submission.SubmittedAt.Time, assignment.LatePolicy, jsonx.ObjectMap(assignment.LatePenalty), int(score))
	if err != nil {
		return 0, err
	}
	return int32(result.FinalScore), nil
}

// ensureAssignmentAccessible 允许教师查看草稿作业,学生只能查看已发布作业。
func (s *Service) ensureAssignmentAccessible(ctx context.Context, assignment sqlcgen.Assignment) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	course, err := s.loadCourse(ctx, assignment.CourseID)
	if err != nil {
		return err
	}
	if id.IsPlatform || course.TeacherID == id.AccountID {
		return nil
	}
	if err := s.ensurePublishedAssignment(assignment); err != nil {
		return err
	}
	_, _, err = s.ensureStudentCourseMember(ctx, assignment.CourseID, apperr.ErrSubmissionForbidden)
	return err
}

// ensurePublishedAssignment 阻止学生侧作业流程访问教师草稿。
func (s *Service) ensurePublishedAssignment(assignment sqlcgen.Assignment) error {
	if assignment.Status != AssignmentStatusPublished {
		return apperr.ErrAssignmentInvalidState
	}
	return nil
}

// recordAssignmentContentUsage 通知 M5 记录作业对题目版本的真实引用。
func (s *Service) recordAssignmentContentUsage(ctx context.Context, tenantID int64, items []AssignmentItemInput) error {
	if s.content == nil {
		return nil
	}
	seen := make(map[contracts.ContentItemRef]struct{}, len(items))
	for _, item := range items {
		ref := contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		if err := s.content.IncrementContentUsage(ctx, tenantID, ref); err != nil {
			return apperr.ErrAssignmentUsageFailed.WithCause(err)
		}
	}
	return nil
}

// newAssignmentRefs 只返回更新草稿后相对原作业新增的题目版本引用。
func newAssignmentRefs(oldItems []sqlcgen.AssignmentItem, newItems []AssignmentItemInput) []AssignmentItemInput {
	oldRefs := make(map[contracts.ContentItemRef]struct{}, len(oldItems))
	for _, item := range oldItems {
		oldRefs[contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion}] = struct{}{}
	}
	out := make([]AssignmentItemInput, 0, len(newItems))
	seenNew := make(map[contracts.ContentItemRef]struct{}, len(newItems))
	for _, item := range newItems {
		ref := contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion}
		if _, ok := oldRefs[ref]; ok {
			continue
		}
		if _, ok := seenNew[ref]; ok {
			continue
		}
		seenNew[ref] = struct{}{}
		out = append(out, item)
	}
	return out
}

// replaceAssignmentItems 重建作业题目引用。
func (s *Service) replaceAssignmentItems(ctx context.Context, q *sqlcgen.Queries, tenantID, assignmentID int64, req []AssignmentItemInput) ([]AssignmentItemDTO, error) {
	out := make([]AssignmentItemDTO, 0, len(req))
	for idx, item := range req {
		seq := item.Seq
		if seq <= 0 {
			seq = int32(idx + 1)
		}
		row, err := q.CreateAssignmentItem(ctx, sqlcgen.CreateAssignmentItemParams{
			ID: s.idgen.Generate(), TenantID: tenantID, AssignmentID: assignmentID, ItemCode: item.ItemCode,
			ItemVersion: item.ItemVersion, Score: item.Score, Seq: seq, GradingMode: item.GradingMode, JudgerCode: pgText(item.JudgerCode),
		})
		if err != nil {
			return nil, err
		}
		out = append(out, assignmentItemDTOFromRow(row, nil))
	}
	return out, nil
}

// listAssignmentItemsWithFace 查询题目并通过 M5 展开题面。
func (s *Service) listAssignmentItemsWithFace(ctx context.Context, assignmentID int64) ([]AssignmentItemDTO, error) {
	rows, err := s.listAssignmentItemRows(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	id, _ := tenantFromContext(ctx)
	out := make([]AssignmentItemDTO, 0, len(rows))
	for _, row := range rows {
		face := map[string]any{}
		if s.content != nil {
			item, err := s.content.GetContentFace(ctx, id.TenantID, contracts.ContentItemRef{ItemCode: row.ItemCode, ItemVersion: row.ItemVersion})
			if err != nil {
				return nil, apperr.ErrAssignmentInvalid.WithCause(err)
			}
			face = map[string]any{"title": item.Title, "type": item.Type, "difficulty": item.Difficulty, "body": item.Body, "tags": item.Tags}
		}
		out = append(out, assignmentItemDTOFromRow(row, face))
	}
	return out, nil
}

// listAssignmentItemRows 查询作业题目行。
func (s *Service) listAssignmentItemRows(ctx context.Context, assignmentID int64) ([]sqlcgen.AssignmentItem, error) {
	var rows []sqlcgen.AssignmentItem
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListAssignmentItems(ctx, assignmentID)
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrAssignmentQueryFailed.WithCause(err)
	}
	return rows, nil
}

// nextAttempt 计算下一次提交序号并检查次数上限。
func (s *Service) nextAttempt(ctx context.Context, assignment sqlcgen.Assignment, studentID int64) (int32, error) {
	var count int64
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		count, err = q.CountSubmissionsByStudent(ctx, sqlcgen.CountSubmissionsByStudentParams{AssignmentID: assignment.ID, StudentID: studentID})
		return err
	}); err != nil {
		return 0, apperr.ErrSubmissionInvalid.WithCause(err)
	}
	if count >= int64(assignment.MaxAttempts) {
		return 0, apperr.ErrSubmissionTooMany
	}
	return int32(count + 1), nil
}

// createSubmissionJudgeOutbox 在提交事务内记录待派发判题请求,保证 M6 本地状态可恢复。
func (s *Service) createSubmissionJudgeOutbox(ctx context.Context, q *sqlcgen.Queries, tenantID, assignmentID, submissionID, studentID int64, req SubmitRequest, items []sqlcgen.AssignmentItem) (sqlcgen.SubmissionJudgeOutbox, error) {
	auto := firstAutoItem(items)
	extraInput, err := jsonx.ObjectBytes(req.ExtraInput, apperr.ErrSubmissionInvalid)
	if err != nil {
		return sqlcgen.SubmissionJudgeOutbox{}, err
	}
	return q.CreateSubmissionJudgeOutbox(ctx, sqlcgen.CreateSubmissionJudgeOutboxParams{
		ID:             s.idgen.Generate(),
		TenantID:       tenantID,
		SubmissionID:   submissionID,
		AssignmentID:   assignmentID,
		StudentID:      studentID,
		ItemCode:       auto.ItemCode,
		ItemVersion:    auto.ItemVersion,
		JudgerCode:     textValue(auto.JudgerCode),
		CodeStorageKey: req.CodeStorageKey,
		CodeHash:       req.CodeHash,
		ExtraInput:     extraInput,
		SourceRef:      fmt.Sprintf("teaching:%d:submission:%d", timex.Now().Year(), submissionID),
		Status:         SubmissionJudgeOutboxPending,
	})
}

// DispatchPendingSubmissionJudges 扫描所有存在待派发判题任务的租户并逐租户派发。
func (s *Service) DispatchPendingSubmissionJudges(ctx context.Context) error {
	limit := s.normalizedJudgeOutboxBatchSize()
	var tenantIDs []int64
	if err := s.repo.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		var queryErr error
		tenantIDs, queryErr = q.ListPendingSubmissionJudgeOutboxTenants(ctx, sqlcgen.ListPendingSubmissionJudgeOutboxTenantsParams{
			Status:     SubmissionJudgeOutboxPending,
			LimitCount: int32(limit),
		})
		return queryErr
	}); err != nil {
		return apperr.ErrSubmissionJudgeLink.WithCause(err)
	}
	for _, tenantID := range tenantIDs {
		if err := s.dispatchPendingSubmissionJudgesForTenant(ctx, tenantID); err != nil {
			return err
		}
	}
	return nil
}

// dispatchPendingSubmissionJudgesForTenant 处理单租户 pending outbox,请求路径和后台 worker 共用。
func (s *Service) dispatchPendingSubmissionJudgesForTenant(ctx context.Context, tenantID int64) error {
	limit := s.normalizedJudgeOutboxBatchSize()
	var rows []sqlcgen.SubmissionJudgeOutbox
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var queryErr error
		rows, queryErr = q.ListPendingSubmissionJudgeOutbox(ctx, sqlcgen.ListPendingSubmissionJudgeOutboxParams{
			Status:     SubmissionJudgeOutboxPending,
			LimitCount: int32(limit),
		})
		return queryErr
	}); err != nil {
		return apperr.ErrSubmissionJudgeLink.WithCause(err)
	}
	for _, pending := range rows {
		claimed, err := s.claimPendingSubmissionJudgeOutbox(ctx, tenantID, pending.ID)
		if err != nil {
			if db.IsNoRows(err) {
				continue
			}
			return err
		}
		task, err := s.submitJudgeOutbox(ctx, claimed)
		if err != nil {
			if failErr := s.failSubmissionJudgeOutbox(ctx, claimed.TenantID, claimed.ID, err); failErr != nil {
				return failErr
			}
			continue
		}
		if err := s.completeSubmissionJudgeOutbox(ctx, claimed, task); err != nil {
			return err
		}
	}
	return nil
}

// claimPendingSubmissionJudgeOutbox 把 pending 行原子切到 running,避免并发 worker 重复处理。
func (s *Service) claimPendingSubmissionJudgeOutbox(ctx context.Context, tenantID, outboxID int64) (sqlcgen.SubmissionJudgeOutbox, error) {
	var row sqlcgen.SubmissionJudgeOutbox
	err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var queryErr error
		row, queryErr = q.MarkSubmissionJudgeOutboxRunning(ctx, sqlcgen.MarkSubmissionJudgeOutboxRunningParams{
			RunningStatus: SubmissionJudgeOutboxRunning,
			ID:            outboxID,
			PendingStatus: SubmissionJudgeOutboxPending,
		})
		return queryErr
	})
	return row, err
}

// submitJudgeOutbox 调用 M3 判题契约;M3 按 source_ref 幂等,支撑 outbox 重试。
func (s *Service) submitJudgeOutbox(ctx context.Context, row sqlcgen.SubmissionJudgeOutbox) (contracts.JudgeTaskInfo, error) {
	if s.judge == nil {
		return contracts.JudgeTaskInfo{}, apperr.ErrJudgeConfigUnavailable
	}
	return s.judge.SubmitJudgeTask(ctx, contracts.JudgeSubmitRequest{
		TenantID:       row.TenantID,
		JudgerCode:     row.JudgerCode,
		ItemCode:       row.ItemCode,
		ItemVersion:    row.ItemVersion,
		CodeStorageKey: row.CodeStorageKey,
		CodeHash:       row.CodeHash,
		SubmitterID:    row.StudentID,
		SourceRef:      row.SourceRef,
		SandboxMode:    "fresh",
		ExtraInput:     jsonx.ObjectMap(row.ExtraInput),
		Priority:       2,
	})
}

// completeSubmissionJudgeOutbox 绑定 M3 task 并标记 outbox 完成;M3 source_ref 幂等保证失败后重试不会重复建任务。
func (s *Service) completeSubmissionJudgeOutbox(ctx context.Context, row sqlcgen.SubmissionJudgeOutbox, task contracts.JudgeTaskInfo) error {
	return s.repo.inTenantID(ctx, row.TenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.UpdateSubmissionJudgeTaskRef(ctx, sqlcgen.UpdateSubmissionJudgeTaskRefParams{ID: row.SubmissionID, JudgeTaskRef: pgText(ids.Format(task.TaskID))}); err != nil {
			return apperr.ErrSubmissionJudgeLink.WithCause(err)
		}
		if _, err := q.CompleteSubmissionJudgeOutbox(ctx, sqlcgen.CompleteSubmissionJudgeOutboxParams{ID: row.ID, DoneStatus: SubmissionJudgeOutboxDone}); err != nil {
			return apperr.ErrSubmissionJudgeLink.WithCause(err)
		}
		return nil
	})
}

// failSubmissionJudgeOutbox 记录派发失败原因并恢复 pending,等待下一轮重试。
func (s *Service) failSubmissionJudgeOutbox(ctx context.Context, tenantID, outboxID int64, cause error) error {
	return s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.FailSubmissionJudgeOutbox(ctx, sqlcgen.FailSubmissionJudgeOutboxParams{
			ID:            outboxID,
			PendingStatus: SubmissionJudgeOutboxPending,
			LastError:     pgText(cause.Error()),
		}); err != nil {
			return apperr.ErrSubmissionJudgeLink.WithCause(err)
		}
		return nil
	})
}

// normalizedJudgeOutboxBatchSize 返回 outbox 每轮派发上限,避免配置缺省导致 worker 空跑或全表扫描。
func (s *Service) normalizedJudgeOutboxBatchSize() int {
	if s.judgeOutboxBatchSize <= 0 {
		return 10
	}
	return s.judgeOutboxBatchSize
}

// buildWeightedScores 按学生聚合作业得分与课程权重。
func buildWeightedScores(weights []sqlcgen.GradeWeight, scores []sqlcgen.ListLatestAssignmentScoresForCourseRow) map[int64][]WeightedScore {
	weightByAssignment := map[int64]float64{}
	for _, weight := range weights {
		if weight.SourceType != GradeSourceAssignment {
			continue
		}
		assignmentID, err := strconv.ParseInt(weight.SourceRef, 10, 64)
		if err != nil {
			continue
		}
		weightByAssignment[assignmentID] = numericValue(weight.Weight)
	}
	out := map[int64][]WeightedScore{}
	for _, score := range scores {
		if !score.FinalScore.Valid {
			continue
		}
		weight, ok := weightByAssignment[score.AssignmentID]
		if !ok {
			continue
		}
		out[score.StudentID] = append(out[score.StudentID], WeightedScore{Score: float64(score.FinalScore.Int32), Weight: weight})
	}
	return out
}
