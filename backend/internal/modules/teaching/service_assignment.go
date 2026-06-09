// M6 作业提交服务:作业题目引用、草稿、提交、判题提交、判题结果回写与教师批改。
package teaching

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
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
	assignment, items, err := s.repo.createAssignmentWithItems(ctx, id.TenantID, assignmentID, courseID, req, latePenalty, s.idgen.Generate)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return AssignmentDTO{}, ae
		}
		return AssignmentDTO{}, apperr.ErrAssignmentInvalid.WithCause(err)
	}
	if err := s.recordAssignmentContentUsage(ctx, id.TenantID, req.Items); err != nil {
		return AssignmentDTO{}, err
	}
	return assignmentDTOFromPolicySnapshot(assignment, assignmentItemDTOsFromSnapshots(items)), s.writeAudit(ctx, id.TenantID, auditActionAssignmentChange, auditTargetAssignment, assignmentID, map[string]any{"title": req.Title})
}

// UpdateAssignment 编辑草稿作业及题目引用,发布后的作业不能再通过草稿入口修改。
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
	updated, items, oldItems, err := s.repo.updateAssignmentWithItems(ctx, id.TenantID, assignmentID, req, latePenalty, s.idgen.Generate)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return AssignmentDTO{}, ae
		}
		return AssignmentDTO{}, apperr.ErrAssignmentInvalid.WithCause(err)
	}
	if err := s.recordAssignmentContentUsage(ctx, id.TenantID, newAssignmentRefs(oldItems, req.Items)); err != nil {
		return AssignmentDTO{}, err
	}
	return assignmentDTOFromPolicySnapshot(updated, assignmentItemDTOsFromSnapshots(items)), nil
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
	published, err := s.repo.publishAssignment(ctx, assignmentID)
	if err != nil {
		return AssignmentDTO{}, apperr.ErrAssignmentInvalid.WithCause(err)
	}
	return assignmentDTOFromPolicySnapshot(published, nil), nil
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
	return assignmentDTOFromPolicySnapshot(assignment, items), nil
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
	if err := s.repo.upsertSubmissionDraft(ctx, tenantID, s.idgen.Generate(), assignmentID, studentID, data); err != nil {
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
	content, err := s.repo.getSubmissionDraftContent(ctx, assignmentID, studentID)
	if err != nil {
		return nil, apperr.ErrSubmissionQueryFailed.WithCause(err)
	}
	return content, nil
}

// SubmitAssignment 创建学生正式提交,自动题写入本模块 outbox 后再异步派发给 M3 判题。
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
	// 先计算尝试次数和迟交策略,拒交策略在写提交前直接返回。
	attempt, err := s.nextAttempt(ctx, assignment, studentID)
	if err != nil {
		return SubmissionDTO{}, err
	}
	late, err := applyLatePolicy(assignment.DueAt, timex.Now(), assignment.LatePolicy, assignment.LatePenalty, 0)
	if err != nil && assignment.LatePolicy == LatePolicyReject {
		return SubmissionDTO{}, err
	}
	items, err := s.listAssignmentItemRows(ctx, assignmentID)
	if err != nil {
		return SubmissionDTO{}, err
	}
	// 再根据题目是否需要自动判题决定初始状态,纯人工作业可直接进入 submitted。
	status := SubmissionStatusPending
	if !hasAutoGrading(items) {
		status = SubmissionStatusSubmitted
	}
	submissionID := s.idgen.Generate()
	outbox, err := s.buildSubmissionJudgeOutbox(req, items, assignmentID, submissionID, studentID)
	if err != nil {
		return SubmissionDTO{}, err
	}
	row, err := s.repo.createSubmissionWithOutbox(ctx, tenantID, submissionID, assignmentID, studentID, attempt, content, late.IsLate, status, outbox, s.idgen.Generate)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return SubmissionDTO{}, ae
		}
		return SubmissionDTO{}, apperr.ErrSubmissionInvalid.WithCause(err)
	}
	if hasAutoGrading(items) {
		// 判题派发失败只记录日志,本地 outbox 已持久化,后续 worker 可恢复派发。
		if err := s.dispatchPendingSubmissionJudgesForTenant(ctx, tenantID); err != nil {
			logging.ErrorContext(ctx, "teaching judge outbox dispatch failed", err.Error(),
				slog.Int64("tenant_id", tenantID),
				slog.Int64("submission_id", submissionID),
			)
		}
	}
	return submissionDTOFromScoreSnapshot(row), nil
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
	rows, err := s.repo.listSubmissionsByAssignment(ctx, assignmentID, size, (page-1)*size)
	if err != nil {
		return nil, apperr.ErrSubmissionQueryFailed.WithCause(err)
	}
	return submissionDTOsFromScoreSnapshots(rows), nil
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
	finalScore, err := finalScoreForSubmission(assignment.DueAt, assignment.LatePolicy, assignment.LatePenalty, submission.SubmittedAt, req.Score)
	if err != nil {
		return SubmissionDTO{}, err
	}
	row, err := s.repo.updateSubmissionManualScore(ctx, submissionID, int64(req.Score), int64(finalScore), req.Comment)
	if err != nil {
		return SubmissionDTO{}, apperr.ErrGradeInvalid.WithCause(err)
	}
	id, _ := tenantFromContext(ctx)
	return submissionDTOFromScoreSnapshot(row), s.writeAudit(ctx, id.TenantID, auditActionSubmissionGrade, auditTargetSubmission, submissionID, map[string]any{"score": req.Score})
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
	return submissionDTOFromScoreSnapshot(row), nil
}

// HandleJudgeCompleted 处理 M3 判题完成事件并回写提交分。
func (s *Service) HandleJudgeCompleted(ctx context.Context, event contracts.JudgeCompletedEvent) error {
	submission, assignment, err := s.repo.getSubmissionWithAssignmentByJudgeTask(ctx, event.TenantID, ids.Format(event.TaskID))
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrSubmissionEventInvalid.WithCause(err)
	}
	finalScore, err := finalScoreForSubmission(assignment.DueAt, assignment.LatePolicy, assignment.LatePenalty, submission.SubmittedAt, int32(event.Score))
	if err != nil {
		return apperr.ErrSubmissionEventInvalid.WithCause(err)
	}
	if err := s.repo.updateSubmissionAutoScoreForEvent(ctx, event.TenantID, submission.ID, int32(event.Score), finalScore); err != nil {
		return apperr.ErrSubmissionEventInvalid.WithCause(err)
	}
	return nil
}

// HandleJudgeFailed 处理 M3 判题失败事件并保留待批改状态。
func (s *Service) HandleJudgeFailed(ctx context.Context, event contracts.JudgeFailedEvent) error {
	if err := s.repo.markSubmissionJudgeFailedForEvent(ctx, event.TenantID, ids.Format(event.TaskID), "自动判题失败,请等待教师处理"); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrSubmissionEventInvalid.WithCause(err)
	}
	return nil
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
func finalScoreForSubmission(dueAt time.Time, latePolicy int16, latePenalty map[string]any, submittedAt time.Time, score int32) (int32, error) {
	result, err := applyLatePolicy(dueAt, submittedAt, latePolicy, latePenalty, int(score))
	if err != nil {
		return 0, err
	}
	return int32(result.FinalScore), nil
}

// ensureAssignmentAccessible 允许教师查看草稿作业,学生只能查看已发布作业。
func (s *Service) ensureAssignmentAccessible(ctx context.Context, assignment AssignmentPolicySnapshot) error {
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
func (s *Service) ensurePublishedAssignment(assignment AssignmentPolicySnapshot) error {
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
func newAssignmentRefs(oldItems []AssignmentItemSnapshot, newItems []AssignmentItemInput) []AssignmentItemInput {
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
		out = append(out, assignmentItemDTOFromSnapshot(row, face))
	}
	return out, nil
}

// listAssignmentItemRows 查询作业题目引用投影。
func (s *Service) listAssignmentItemRows(ctx context.Context, assignmentID int64) ([]AssignmentItemSnapshot, error) {
	rows, err := s.repo.listAssignmentItems(ctx, assignmentID)
	if err != nil {
		return nil, apperr.ErrAssignmentQueryFailed.WithCause(err)
	}
	return rows, nil
}

// nextAttempt 计算下一次提交序号并检查次数上限。
func (s *Service) nextAttempt(ctx context.Context, assignment AssignmentPolicySnapshot, studentID int64) (int32, error) {
	count, err := s.repo.countSubmissionsByStudent(ctx, assignment.ID, studentID)
	if err != nil {
		return 0, apperr.ErrSubmissionInvalid.WithCause(err)
	}
	if count >= int64(assignment.MaxAttempts) {
		return 0, apperr.ErrSubmissionTooMany
	}
	return int32(count + 1), nil
}

// buildSubmissionJudgeOutbox 生成提交事务内的判题 outbox 参数,保证 M6 本地状态可恢复。
func (s *Service) buildSubmissionJudgeOutbox(req SubmitRequest, items []AssignmentItemSnapshot, assignmentID, submissionID, studentID int64) (*SubmissionJudgeOutboxCreate, error) {
	if !hasAutoGrading(items) {
		return nil, nil
	}
	auto := firstAutoItem(items)
	extraInput, err := jsonx.ObjectBytes(req.ExtraInput, apperr.ErrSubmissionInvalid)
	if err != nil {
		return nil, err
	}
	return &SubmissionJudgeOutboxCreate{
		ItemCode: auto.ItemCode, ItemVersion: auto.ItemVersion, JudgerCode: auto.JudgerCode,
		CodeStorageKey: req.CodeStorageKey, CodeHash: req.CodeHash, ExtraInput: extraInput,
		SourceRef: fmt.Sprintf("teaching:%d:submission:%d", timex.Now().Year(), submissionID),
	}, nil
}

// DispatchPendingSubmissionJudges 扫描所有存在待派发判题任务的租户并逐租户派发。
func (s *Service) DispatchPendingSubmissionJudges(ctx context.Context) error {
	limit := s.normalizedJudgeOutboxBatchSize()
	tenantIDs, err := s.repo.listPendingSubmissionJudgeOutboxTenants(ctx, limit)
	if err != nil {
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
	rows, err := s.repo.listPendingSubmissionJudgeOutbox(ctx, tenantID, limit)
	if err != nil {
		return apperr.ErrSubmissionJudgeLink.WithCause(err)
	}
	for _, pending := range rows {
		claimed, found, err := s.repo.claimPendingSubmissionJudgeOutbox(ctx, tenantID, pending.ID)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		task, err := s.submitJudgeOutbox(ctx, claimed)
		if err != nil {
			if failErr := s.repo.failSubmissionJudgeOutbox(ctx, claimed.TenantID, claimed.ID, err); failErr != nil {
				return failErr
			}
			continue
		}
		if err := s.repo.completeSubmissionJudgeOutbox(ctx, claimed, task.TaskID); err != nil {
			return err
		}
	}
	return nil
}

// submitJudgeOutbox 调用 M3 判题契约;M3 按 source_ref 幂等,支撑 outbox 重试。
func (s *Service) submitJudgeOutbox(ctx context.Context, row SubmissionJudgeOutboxSnapshot) (contracts.JudgeTaskInfo, error) {
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
		ExtraInput:     row.ExtraInput,
		Priority:       2,
	})
}

// normalizedJudgeOutboxBatchSize 返回 outbox 每轮派发上限;启动配置已保证该值为正数。
func (s *Service) normalizedJudgeOutboxBatchSize() int {
	return s.judgeOutboxBatchSize
}

// buildWeightedScores 按学生聚合作业得分与课程权重。
func buildWeightedScores(weights []GradeWeightInput, scores []AssignmentScoreSnapshot) map[int64][]WeightedScore {
	weightByAssignment := map[int64]float64{}
	for _, weight := range weights {
		if weight.SourceType != GradeSourceAssignment {
			continue
		}
		assignmentID, err := strconv.ParseInt(weight.SourceRef, 10, 64)
		if err != nil {
			continue
		}
		weightByAssignment[assignmentID] = weight.Weight
	}
	out := map[int64][]WeightedScore{}
	for _, score := range scores {
		if score.FinalScore == nil {
			continue
		}
		weight, ok := weightByAssignment[score.AssignmentID]
		if !ok {
			continue
		}
		out[score.StudentID] = append(out[score.StudentID], WeightedScore{Score: float64(*score.FinalScore), Weight: weight})
	}
	return out
}
