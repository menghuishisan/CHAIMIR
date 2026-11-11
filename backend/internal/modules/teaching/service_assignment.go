// teaching service_assignment 文件实现作业、提交、草稿和自动判题派发业务。
package teaching

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/question"
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
	if err := s.validateAssignmentItems(ctx, id.TenantID, req.Items); err != nil {
		return AssignmentDetailDTO{}, err
	}
	assignment := Assignment{ID: s.ids.Generate(), TenantID: id.TenantID, CourseID: courseID, Title: req.Title, ChapterID: req.ChapterID.Int64(), DueAt: due, MaxAttempts: req.MaxAttempts, LatePolicy: req.LatePolicy, LatePenalty: req.LatePenalty, Status: AssignmentStatusDraft}
	items := make([]AssignmentItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, AssignmentItem{ID: s.ids.Generate(), TenantID: id.TenantID, AssignmentID: assignment.ID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, Seq: item.Seq})
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		assignment, err = tx.CreateAssignment(ctx, assignment)
		if err != nil {
			return err
		}
		items, err = tx.ReplaceAssignmentItems(ctx, id.TenantID, assignment.ID, items)
		if err != nil {
			return err
		}
		if err := s.refreshAssignmentUsageRefs(ctx, id.TenantID, assignment, items); err != nil {
			return err
		}
		return err
	}); err != nil {
		return AssignmentDetailDTO{}, mapAssignmentError(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "teaching.assignment.create", auditTargetAssignment, assignment.ID, map[string]any{"course_id": courseID}); err != nil {
		return AssignmentDetailDTO{}, err
	}
	return assignmentDetailDTO(AssignmentDetail{Assignment: assignment, Items: assignmentItemFaces(items)})
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
	if err := s.validateAssignmentItems(ctx, id.TenantID, req.Items); err != nil {
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
		current.Title, current.ChapterID, current.DueAt, current.MaxAttempts, current.LatePolicy, current.LatePenalty = req.Title, req.ChapterID.Int64(), due, req.MaxAttempts, req.LatePolicy, req.LatePenalty
		assignment, err = tx.UpdateAssignment(ctx, current)
		if err != nil {
			return err
		}
		next := make([]AssignmentItem, 0, len(req.Items))
		for _, item := range req.Items {
			next = append(next, AssignmentItem{ID: s.ids.Generate(), TenantID: id.TenantID, AssignmentID: assignmentID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, Seq: item.Seq})
		}
		items, err = tx.ReplaceAssignmentItems(ctx, id.TenantID, assignmentID, next)
		if err != nil {
			return err
		}
		if err := s.refreshAssignmentUsageRefs(ctx, id.TenantID, assignment, items); err != nil {
			return err
		}
		return err
	}); err != nil {
		return AssignmentDetailDTO{}, mapAssignmentError(err)
	}
	return assignmentDetailDTO(AssignmentDetail{Assignment: assignment, Items: assignmentItemFaces(items)})
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
		for _, item := range items {
			face, err := s.content.GetContentFace(ctx, id.TenantID, contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion})
			if err != nil {
				return apperr.ErrTeachingAssignmentInvalid.WithCause(err)
			}
			if err := validateTheoryQuestionFace(face); err != nil {
				return err
			}
			if item.Score != jsonx.Int32FromAny(face.Body["max_score"], 0) {
				return apperr.ErrTeachingAssignmentInvalid
			}
		}
		assignment, err = tx.PublishAssignment(ctx, id.TenantID, assignmentID)
		return err
	}); err != nil {
		return AssignmentDTO{}, mapAssignmentError(err)
	}
	return assignmentDTO(assignment)
}

// ListCourseAssignments 查询课程作业清单。
// 这是学生取得 assignment_id 的唯一入口:作业详情、草稿和提交都以该编号为参数,
// 而课程大纲只含章节课时。授课教师看到含草稿的全量,课程成员只看到已发布作业。
func (s *Service) ListCourseAssignments(ctx context.Context, courseID int64, page, size int) ([]AssignmentDTO, int64, int, int, error) {
	page, size = pagex.Normalize(page, size)
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, page, size, err
	}
	var assignments []Assignment
	var assignmentTotal int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		isTeacher, err := s.courseReadAccess(ctx, tx, id.TenantID, courseID, id.AccountID)
		if err != nil {
			return err
		}
		var total int64
		assignments, total, err = tx.ListAssignmentsByCourse(ctx, id.TenantID, courseID, !isTeacher, page, size)
		if err == nil {
			assignmentTotal = total
		}
		return err
	}); err != nil {
		return nil, 0, page, size, mapAssignmentError(err)
	}
	out := make([]AssignmentDTO, 0, len(assignments))
	for _, assignment := range assignments {
		dto, err := assignmentDTO(assignment)
		if err != nil {
			return nil, 0, page, size, mapAssignmentError(err)
		}
		out = append(out, dto)
	}
	return out, assignmentTotal, page, size, nil
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
		if err := validateTheoryQuestionFace(face); err != nil {
			return AssignmentDetailDTO{}, err
		}
		if item.Score != jsonx.Int32FromAny(face.Body["max_score"], 0) {
			return AssignmentDetailDTO{}, apperr.ErrTeachingAssignmentInvalid
		}
		detail.Items = append(detail.Items, AssignmentItemFace{AssignmentItem: item, Title: face.Title, Type: face.Type, Difficulty: face.Difficulty, Body: face.Body})
	}
	return assignmentDetailDTO(detail)
}

// SaveDraft 保存服务端权威作答草稿。
func (s *Service) SaveDraft(ctx context.Context, assignmentID int64, req DraftRequest) (DraftSaveDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return DraftSaveDTO{}, err
	}
	req, err = validateDraftRequest(req)
	if err != nil {
		return DraftSaveDTO{}, err
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
		items, err := tx.ListAssignmentItems(ctx, id.TenantID, assignmentID)
		if err != nil {
			return err
		}
		faces, err := s.loadAssignmentQuestionFaces(ctx, id.TenantID, items)
		if err != nil {
			return err
		}
		if err := validateAssignmentAnswers(items, faces, req.Content); err != nil {
			return err
		}
		draft, err = tx.UpsertDraft(ctx, SubmissionDraft{ID: s.ids.Generate(), TenantID: id.TenantID, AssignmentID: assignmentID, StudentID: id.AccountID, Content: req.Content})
		return err
	}); err != nil {
		return DraftSaveDTO{}, mapAssignmentError(err)
	}
	return DraftSaveDTO{UpdatedAt: timex.RFC3339OrEmpty(draft.UpdatedAt)}, nil
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
		return DraftDTO{AssignmentID: ids.ID(assignmentID), StudentID: ids.ID(id.AccountID), Content: map[string]any{}, Exists: false}, nil
	}
	return draftDTO(draft)
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
		course, err := tx.GetCourse(ctx, id.TenantID, assignment.CourseID)
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
		faces, err := s.loadAssignmentQuestionFaces(ctx, id.TenantID, items)
		if err != nil {
			return err
		}
		if err := validateAssignmentAnswers(items, faces, req.Answers); err != nil {
			return err
		}
		manualMaxScore := int32(0)
		autoMaxScore := int32(0)
		autoCount := 0
		for _, item := range items {
			if questionTypeFromFace(faces[item.ID]) == "short_answer" {
				manualMaxScore += item.Score
			} else {
				autoCount++
				autoMaxScore += item.Score
			}
		}
		answers := make(map[string]any, len(req.Answers))
		for itemID, answer := range req.Answers {
			answers[itemID] = answer
		}
		contentRef := map[string]any{"answers": answers, "manual_max_score": manualMaxScore, "auto_max_score": autoMaxScore}
		status := SubmissionStatusPending
		if autoCount > 0 {
			status = SubmissionStatusSubmitted
		}
		attemptNo, attemptOK := intx.Int64ToInt32(attempts + 1)
		if !attemptOK || attemptNo <= 0 {
			return apperr.ErrTeachingSubmissionLimitExceeded
		}
		sub, err = tx.CreateSubmission(ctx, Submission{ID: s.ids.Generate(), TenantID: id.TenantID, AssignmentID: assignmentID, StudentID: id.AccountID, AttemptNo: attemptNo, ContentRef: contentRef, IsLate: isLate, Status: status})
		if err != nil {
			return err
		}
		for _, item := range items {
			questionType := questionTypeFromFace(faces[item.ID])
			if questionType == "short_answer" {
				continue
			}
			extra := map[string]any{}
			if answer, exists := req.Answers[ids.Format(item.ID)]; exists {
				extra["student_answer"] = answer
			}
			if _, err := tx.CreateJudgeOutbox(ctx, JudgeOutbox{ID: s.ids.Generate(), TenantID: id.TenantID, SubmissionID: sub.ID, AssignmentItemID: item.ID, AssignmentID: assignmentID, SourceOwnerID: course.TeacherID, SourceCourseID: assignment.CourseID, SourceScope: "teaching", StudentID: id.AccountID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, ExtraInput: extra, SourceRef: sourceRefForSubmissionItem(sub.ID, item.ID)}); err != nil {
				return err
			}
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
	return submissionDTO(sub)
}

// GradeSubmission 教师批改作业中的简答题。
func (s *Service) GradeSubmission(ctx context.Context, submissionID int64, req GradeSubmissionRequest) (SubmissionDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return SubmissionDTO{}, err
	}
	req, err = validateGradeRequest(req)
	if err != nil {
		return SubmissionDTO{}, err
	}
	score := *req.Score
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
		manualMaxScore := manualMaximumScore(current.ContentRef)
		if manualMaxScore <= 0 || score < 0 || score > manualMaxScore {
			return apperr.ErrTeachingSubmissionInvalid
		}
		course, err := tx.GetCourse(ctx, id.TenantID, assignment.CourseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		final := int32(0)
		if jsonx.Int32FromAny(current.ContentRef["auto_max_score"], 0) == 0 || current.AutoScore != nil {
			autoScore := int32(0)
			if current.AutoScore != nil {
				autoScore = *current.AutoScore
			}
			final, err = applyLatePenalty(assignment, autoScore+score, current.IsLate)
			if err != nil {
				return err
			}
		}
		sub, err = tx.UpdateSubmissionManualGrade(ctx, id.TenantID, submissionID, score, final, req.Comment)
		return err
	}); err != nil {
		return SubmissionDTO{}, mapAssignmentError(err)
	}
	if sub.FinalScore != nil {
		if err := s.publishGradeUpdated(ctx, id.TenantID, sub.AssignmentID, sub.StudentID); err != nil {
			return SubmissionDTO{}, err
		}
	}
	return submissionDTO(sub)
}

// ListSubmissions 查询作业提交情况。
// 授课教师看到该作业全部学生提交(批改视角);课程内学生只看到本人的历次提交
// (自读视角 —— 学生提交后刷新页面需据此取回 submission_id 再看反馈)。
// 学生编号取自服务端会话,不接受客户端传参;status 传 0 表示不按批改状态过滤。
func (s *Service) ListSubmissions(ctx context.Context, assignmentID int64, status, isLate int16, page, size int) ([]SubmissionDTO, int64, int, int, error) {
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
		isTeacher, err := s.courseReadAccess(ctx, tx, id.TenantID, assignment.CourseID, id.AccountID)
		if err != nil {
			return err
		}
		studentFilter := id.AccountID
		if isTeacher {
			studentFilter = 0
		}
		subs, total, err = tx.ListSubmissionsByAssignment(ctx, SubmissionListQuery{
			TenantID:     id.TenantID,
			AssignmentID: assignmentID,
			StudentID:    studentFilter,
			Status:       status,
			IsLate:       isLate,
			Page:         page,
			Size:         size,
		})
		return err
	}); err != nil {
		return nil, 0, 0, 0, mapAssignmentError(err)
	}
	out := make([]SubmissionDTO, 0, len(subs))
	for _, sub := range subs {
		dto, err := submissionDTO(sub)
		if err != nil {
			return nil, 0, 0, 0, mapAssignmentError(err)
		}
		out = append(out, dto)
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
	return submissionDTO(sub)
}

// RunJudgeOutboxOnce 派发一轮 M6 本地自动判题 outbox。
func (s *Service) RunJudgeOutboxOnce(ctx context.Context, tenantID int64) error {
	if s.judge == nil {
		return apperr.ErrTeachingJudgeServiceUnavailable
	}
	batchSize, ok := intx.Int32(s.cfg.JudgeOutboxBatchSize)
	if !ok || batchSize <= 0 {
		return apperr.ErrTeachingJudgeOutboxInvalid
	}
	now := timex.Now()
	leaseDuration := time.Duration(s.cfg.JudgeOutboxStaleMs) * time.Millisecond
	if leaseDuration <= 0 {
		return apperr.ErrTeachingJudgeOutboxInvalid
	}
	leaseToken, err := pkgcrypto.RandomToken(48)
	if err != nil {
		return apperr.ErrTeachingJudgeOutboxInvalid.WithCause(err)
	}
	staleBefore := now
	leaseUntil := now.Add(leaseDuration)
	maxAttempts, ok := intx.Int32(s.cfg.JudgeOutboxMaxAttempts)
	if !ok || maxAttempts <= 0 {
		return apperr.ErrTeachingJudgeOutboxInvalid
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
			if _, err = tx.ExhaustExpiredJudgeOutbox(ctx, tenantID, maxAttempts, staleBefore); err != nil {
				return err
			}
			outboxes, err = tx.ClaimJudgeOutbox(ctx, tenantID, batchSize, maxAttempts, staleBefore, leaseToken, leaseUntil)
			return err
		}
		if _, err = tx.ExhaustExpiredJudgeOutboxAcrossTenants(ctx, maxAttempts, staleBefore); err != nil {
			return err
		}
		outboxes, err = tx.ClaimJudgeOutboxAcrossTenants(ctx, batchSize, maxAttempts, staleBefore, leaseToken, leaseUntil)
		return err
	}); err != nil {
		return apperr.ErrTeachingJudgeOutboxInvalid.WithCause(err)
	}
	for _, item := range outboxes {
		info, err := s.judge.SubmitJudgeTask(ctx, contracts.JudgeSubmitRequest{TenantID: item.TenantID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, SubmitterTenantID: item.TenantID, SubmitterID: item.StudentID, SourceRef: item.SourceRef, SourceOwnerID: item.SourceOwnerID, SourceCourseID: item.SourceCourseID, SourceScope: item.SourceScope, SandboxMode: contracts.JudgeSandboxModeFresh, ExtraInput: item.ExtraInput, Priority: 5})
		if err != nil {
			if retryErr := s.store.TenantTx(ctx, item.TenantID, func(ctx context.Context, tx TxStore) error {
				_, retryErr := tx.RetryJudgeOutbox(ctx, item.TenantID, item.ID, maxAttempts, safeStoredError(ctx, err), item.LeaseToken)
				return retryErr
			}); retryErr != nil {
				return apperr.ErrTeachingJudgeOutboxInvalid.WithCause(retryErr)
			}
			continue
		}
		if err := s.store.TenantTx(ctx, item.TenantID, func(ctx context.Context, tx TxStore) error {
			if _, err := tx.UpdateSubmissionJudgeRef(ctx, item.TenantID, item.SubmissionID, ids.Format(info.TaskID)); err != nil {
				return err
			}
			_, err := tx.CompleteJudgeOutbox(ctx, item.TenantID, item.ID, item.LeaseToken)
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
		manualComplete := manualMaximumScore(current.ContentRef) == 0 || current.ManualScore != nil
		combined := total
		if current.ManualScore != nil {
			combined += *current.ManualScore
		}
		final := int32(0)
		if manualComplete {
			final, err = applyLatePenalty(assignment, combined, current.IsLate)
			if err != nil {
				return err
			}
		}
		sub, err = tx.UpdateSubmissionAutoScore(ctx, event.TenantID, current.ID, total, final)
		ready = manualComplete && err == nil
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
	var sub Submission
	var ready bool
	if err := s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetSubmissionBySourceRef(ctx, event.TenantID, event.SourceRef)
		if err != nil {
			return err
		}
		if _, err := tx.MarkJudgeOutboxFailedResult(ctx, event.TenantID, event.SourceRef, event.Reason, event.FailedAt); err != nil {
			return err
		}
		outboxes, err := tx.ListJudgeOutboxBySubmission(ctx, event.TenantID, current.ID)
		if err != nil {
			return err
		}
		autoScore, allDone := aggregateCompletedAutoScore(outboxes)
		if !allDone {
			sub = current
			return nil
		}
		assignment, err := tx.GetAssignment(ctx, event.TenantID, current.AssignmentID)
		if err != nil {
			return err
		}
		manualComplete := manualMaximumScore(current.ContentRef) == 0 || current.ManualScore != nil
		combined := autoScore
		if current.ManualScore != nil {
			combined += *current.ManualScore
		}
		final := int32(0)
		if manualComplete {
			final, err = applyLatePenalty(assignment, combined, current.IsLate)
			if err != nil {
				return err
			}
		}
		sub, err = tx.UpdateSubmissionAutoScore(ctx, event.TenantID, current.ID, autoScore, final)
		ready = manualComplete && err == nil
		return err
	}); err != nil {
		return apperr.ErrTeachingSubmissionInvalid.WithCause(err)
	}
	if !ready {
		return nil
	}
	return s.publishGradeUpdated(ctx, event.TenantID, sub.AssignmentID, sub.StudentID)
}

// aggregateCompletedAutoScore 汇总一次提交内所有自动题得分,存在未完成题则不结算提交。
func aggregateCompletedAutoScore(outboxes []JudgeOutbox) (int32, bool) {
	if len(outboxes) == 0 {
		return 0, false
	}
	var total int32
	for _, outbox := range outboxes {
		if outbox.CompletedAt.IsZero() {
			return 0, false
		}
		total += outbox.Score
	}
	return total, true
}

// refreshAssignmentUsageRefs 按作业来源替换 M5 内容引用集合,由 M5 幂等维护 usage_count。
func (s *Service) refreshAssignmentUsageRefs(ctx context.Context, tenantID int64, assignment Assignment, items []AssignmentItem) error {
	refs := make([]contracts.ContentItemRef, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		key := strings.TrimSpace(item.ItemCode) + "\x00" + strings.TrimSpace(item.ItemVersion)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		refs = append(refs, contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion})
	}
	sourceRef := fmt.Sprintf("teaching:%d:assignment:%s", assignment.CreatedAt.Year(), ids.Format(assignment.ID))
	if err := s.content.ReplaceUsageRefs(ctx, tenantID, "teaching.assignment", sourceRef, refs); err != nil {
		return apperr.ErrTeachingAssignmentInvalid.WithCause(err)
	}
	return nil
}

// sourceRefForSubmissionItem 构造 M6 提交题目来源标识。
func sourceRefForSubmissionItem(submissionID, assignmentItemID int64) string {
	return fmt.Sprintf("teaching:%d:submission-item:%s-%s", timex.Now().Year(), ids.Format(submissionID), ids.Format(assignmentItemID))
}

// validateAssignmentItems 校验理论题锁定引用及 M5 定义的满分。
func (s *Service) validateAssignmentItems(ctx context.Context, tenantID int64, items []AssignmentItemInput) error {
	for _, item := range items {
		face, err := s.content.GetContentFace(ctx, tenantID, contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion})
		if err != nil {
			return apperr.ErrTeachingAssignmentInvalid.WithCause(err)
		}
		if err := validateTheoryQuestionFace(face); err != nil {
			return err
		}
		if item.Score != jsonx.Int32FromAny(face.Body["max_score"], 0) {
			return apperr.ErrTeachingAssignmentInvalid
		}
	}
	return nil
}

// validateTheoryQuestionFace 验证不含标准答案的 M5 理论题定义。
func validateTheoryQuestionFace(face contracts.ContentItemSnapshot) error {
	if face.Type != contracts.ContentTypeTheoryQuestion {
		return apperr.ErrTeachingAssignmentInvalid
	}
	if _, err := question.Decode(face.Body, false); err != nil {
		return apperr.ErrTeachingAssignmentInvalid.WithCause(err)
	}
	return nil
}

// questionTypeFromFace 在验证公开题面后读取唯一题型。
func questionTypeFromFace(face contracts.ContentItemSnapshot) string {
	if err := validateTheoryQuestionFace(face); err != nil {
		return ""
	}
	return jsonx.StringValue(face.Body["q_type"])
}

// validateAssignmentAnswers 拒绝其他作业题号及不符合锁定题型的答案。
func validateAssignmentAnswers(items []AssignmentItem, faces map[int64]contracts.ContentItemSnapshot, answers map[string]any) error {
	byID := make(map[string]AssignmentItem, len(items))
	for _, item := range items {
		byID[ids.Format(item.ID)] = item
	}
	for itemID, answer := range answers {
		item, exists := byID[itemID]
		if !exists {
			return apperr.ErrTeachingSubmissionInvalid
		}
		face, exists := faces[item.ID]
		if !exists || !validateTheoryStudentAnswer(answer, face.Body) {
			return apperr.ErrTeachingSubmissionInvalid
		}
	}
	return nil
}

// loadAssignmentQuestionFaces 通过 M5 读取锁定题面的定义,不复制题目正本。
func (s *Service) loadAssignmentQuestionFaces(ctx context.Context, tenantID int64, items []AssignmentItem) (map[int64]contracts.ContentItemSnapshot, error) {
	faces := make(map[int64]contracts.ContentItemSnapshot, len(items))
	for _, item := range items {
		face, err := s.content.GetContentFace(ctx, tenantID, contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion})
		if err != nil {
			return nil, apperr.ErrTeachingAssignmentInvalid.WithCause(err)
		}
		if err := validateTheoryQuestionFace(face); err != nil {
			return nil, err
		}
		faces[item.ID] = face
	}
	return faces, nil
}
