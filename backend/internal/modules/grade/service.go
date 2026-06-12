// grade service 文件实现 M11 成绩中心审核、GPA、申诉、预警和成绩单业务编排。
package grade

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

type objectStorage interface {
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	BucketReport() string
}

// Service 承载 M11 成绩中心业务编排。
type Service struct {
	store    Store
	ids      snowflake.Generator
	audit    audit.Writer
	roles    roleReader
	teaching contracts.TeachingReadService
	notify   contracts.NotifyService
	bus      eventbus.Bus
	storage  objectStorage
	cfg      config.GradeConfig
}

// ServiceDeps 是 M11 服务装配依赖。
type ServiceDeps struct {
	Store    Store
	IDs      snowflake.Generator
	Audit    audit.Writer
	Roles    roleReader
	Teaching contracts.TeachingReadService
	Notify   contracts.NotifyService
	Bus      eventbus.Bus
	Storage  *storage.Storage
	Objects  objectStorage
	Config   config.GradeConfig
}

// NewService 构造 M11 服务。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil || deps.IDs == nil || deps.Audit == nil || deps.Roles == nil || deps.Teaching == nil || deps.Bus == nil {
		return nil, fmt.Errorf("grade service 依赖不完整")
	}
	objects := deps.Objects
	if objects == nil {
		objects = deps.Storage
	}
	if objects == nil {
		return nil, fmt.Errorf("grade service 对象存储依赖不完整")
	}
	if deps.Config.AppealWindowDays <= 0 || strings.TrimSpace(deps.Config.TranscriptSigningKey) == "" {
		return nil, fmt.Errorf("grade service 配置不完整")
	}
	return &Service{store: deps.Store, ids: deps.IDs, audit: deps.Audit, roles: deps.Roles, teaching: deps.Teaching, notify: deps.Notify, bus: deps.Bus, storage: objects, cfg: deps.Config}, nil
}

// CreateLevelConfig 创建等级映射配置。
func (s *Service) CreateLevelConfig(ctx context.Context, req LevelConfigRequest) (LevelConfigDTO, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	if err := validateLevelConfig(req); err != nil {
		return LevelConfigDTO{}, err
	}
	var out LevelConfigDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateLevelConfig(ctx, s.ids.Generate(), id.TenantID, req)
		return err
	})
	return out, mapGradeConfigErr(err)
}

// ListLevelConfigs 查询等级映射配置。
func (s *Service) ListLevelConfigs(ctx context.Context) ([]LevelConfigDTO, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return nil, err
	}
	var out []LevelConfigDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListLevelConfigs(ctx)
		return err
	})
	return out, mapGradeConfigErr(err)
}

// UpdateLevelConfig 更新等级映射配置。
func (s *Service) UpdateLevelConfig(ctx context.Context, configID int64, req LevelConfigRequest) (LevelConfigDTO, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	if err := validateLevelConfig(req); err != nil {
		return LevelConfigDTO{}, err
	}
	var out LevelConfigDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.UpdateLevelConfig(ctx, configID, req)
		return err
	})
	return out, mapGradeConfigErr(err)
}

// CreateSemester 创建学期。
func (s *Service) CreateSemester(ctx context.Context, req SemesterRequest) (SemesterDTO, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return SemesterDTO{}, err
	}
	if strings.TrimSpace(req.Name) == "" {
		return SemesterDTO{}, apperr.ErrGradeConfigInvalid
	}
	var out SemesterDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateSemester(ctx, s.ids.Generate(), id.TenantID, req)
		return err
	})
	return out, mapGradeConfigErr(err)
}

// ListSemesters 查询学期列表。
func (s *Service) ListSemesters(ctx context.Context) ([]SemesterDTO, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return nil, err
	}
	var out []SemesterDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListSemesters(ctx)
		return err
	})
	return out, mapGradeConfigErr(err)
}

// SubmitReview 提交课程成绩审核。
func (s *Service) SubmitReview(ctx context.Context, req ReviewRequest) (ReviewDTO, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	if req.CourseID <= 0 {
		return ReviewDTO{}, apperr.ErrGradeReviewInvalid
	}
	var out ReviewDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateGradeReview(ctx, s.ids.Generate(), id.TenantID, id.AccountID, req)
		return err
	})
	return out, mapGradeReviewErr(err)
}

// ListReviews 查询成绩审核列表。
func (s *Service) ListReviews(ctx context.Context, status int16, page, size int) ([]ReviewDTO, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return nil, err
	}
	var out []ReviewDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListGradeReviews(ctx, status, page, size)
		return err
	})
	return out, mapGradeReviewErr(err)
}

// ApproveReview 通过审核、锁定 M6 单课程成绩并重算 GPA。
func (s *Service) ApproveReview(ctx context.Context, reviewID int64, req ReviewDecisionRequest) (ReviewDTO, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	if req.SemesterID <= 0 {
		return ReviewDTO{}, apperr.ErrGradeReviewInvalid
	}
	var out ReviewDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ApproveGradeReview(ctx, reviewID, id.AccountID, req.SemesterID, req.Comment)
		return err
	})
	if err != nil {
		return ReviewDTO{}, mapGradeReviewErr(err)
	}
	if err := s.publishLock(ctx, out, true, "review_approved"); err != nil {
		return ReviewDTO{}, err
	}
	if err := s.recomputeCourse(ctx, id.TenantID, out.CourseID, out.SemesterID); err != nil {
		return ReviewDTO{}, err
	}
	return out, nil
}

// RejectReview 驳回成绩审核。
func (s *Service) RejectReview(ctx context.Context, reviewID int64, req ReviewDecisionRequest) (ReviewDTO, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	var out ReviewDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.RejectGradeReview(ctx, reviewID, id.AccountID, req.Comment)
		return err
	})
	return out, mapGradeReviewErr(err)
}

// UnlockReview 解锁课程成绩,供审核重开或申诉受理后由 M6 改分。
func (s *Service) UnlockReview(ctx context.Context, reviewID int64, req ReviewDecisionRequest) (ReviewDTO, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	var out ReviewDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.UnlockGradeReview(ctx, reviewID, id.AccountID, req.Comment)
		return err
	})
	if err != nil {
		return ReviewDTO{}, mapGradeReviewErr(err)
	}
	return out, s.publishLock(ctx, out, false, "review_unlocked")
}

// StudentSummary 查询学生 GPA 汇总。
func (s *Service) StudentSummary(ctx context.Context, studentID int64) (GradeSummaryDTO, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return GradeSummaryDTO{}, err
	}
	if studentID == 0 {
		studentID = id.AccountID
	}
	if id.AccountID != studentID {
		// API 层负责角色授权,service 层仍避免普通学生横向读取。
		allowed, err := s.canReadStudent(ctx, id.AccountID)
		if err != nil {
			return GradeSummaryDTO{}, err
		}
		if !allowed {
			return GradeSummaryDTO{}, apperr.ErrGradeForbidden
		}
	}
	grades, err := s.teaching.ListStudentGrades(ctx, id.TenantID, studentID)
	if err != nil {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	inputs := courseInputs(grades)
	cfg, err := s.defaultConfig(ctx, id.TenantID)
	if err != nil {
		return GradeSummaryDTO{}, err
	}
	gpa, credits, err := ComputeGPA(inputs, cfg.Mapping)
	if err != nil {
		return GradeSummaryDTO{}, err
	}
	return GradeSummaryDTO{StudentID: studentID, GPA: gpa, CumulativeGPA: gpa, TotalCredits: credits, CourseGrades: inputs, ComputedAt: timex.Now()}, nil
}

// CreateAppeal 创建成绩申诉。
func (s *Service) CreateAppeal(ctx context.Context, req AppealRequest) (AppealDTO, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return AppealDTO{}, err
	}
	if req.CourseID <= 0 || strings.TrimSpace(req.Reason) == "" {
		return AppealDTO{}, apperr.ErrGradeAppealInvalid
	}
	var out AppealDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		review, err := tx.GetLatestApprovedReviewByCourse(ctx, req.CourseID)
		if err != nil {
			return apperr.ErrGradeAppealInvalid.WithCause(err)
		}
		reviewedAt, err := time.Parse(time.RFC3339, review.ReviewedAt)
		if err != nil {
			return apperr.ErrGradeAppealInvalid.WithCause(err)
		}
		if err := EnsureAppealWithinWindow(reviewedAt, timex.Now(), s.cfg.AppealWindowDays); err != nil {
			return err
		}
		out, err = tx.CreateGradeAppeal(ctx, s.ids.Generate(), id.TenantID, id.AccountID, req)
		return err
	})
	return out, mapGradeAppealErr(err)
}

// ListAppeals 查询申诉列表。
func (s *Service) ListAppeals(ctx context.Context, status int16, page, size int) ([]AppealDTO, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return nil, err
	}
	var out []AppealDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListGradeAppeals(ctx, status, page, size)
		return err
	})
	return out, mapGradeAppealErr(err)
}

// AcceptAppeal 受理申诉并解锁对应课程审核。
func (s *Service) AcceptAppeal(ctx context.Context, appealID int64, req AppealDecisionRequest) (AppealDTO, error) {
	return s.decideAppeal(ctx, appealID, AppealStatusAccepted, req.Comment)
}

// RejectAppeal 驳回申诉。
func (s *Service) RejectAppeal(ctx context.Context, appealID int64, req AppealDecisionRequest) (AppealDTO, error) {
	return s.decideAppeal(ctx, appealID, AppealStatusRejected, req.Comment)
}

// ListWarnings 查询学业预警。
func (s *Service) ListWarnings(ctx context.Context, studentID int64, page, size int) ([]WarningDTO, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return nil, err
	}
	var out []WarningDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListAcademicWarnings(ctx, studentID, page, size)
		return err
	})
	return out, mapGradeWarningErr(err)
}

// AckWarning 确认本人学业预警。
func (s *Service) AckWarning(ctx context.Context, warningID int64) (WarningDTO, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return WarningDTO{}, err
	}
	var out WarningDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.AckAcademicWarning(ctx, warningID, id.AccountID)
		return err
	})
	return out, mapGradeWarningErr(err)
}

// GenerateTranscript 生成成绩单 PDF 并保存元数据。
func (s *Service) GenerateTranscript(ctx context.Context, req TranscriptRequest) (TranscriptDTO, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return TranscriptDTO{}, err
	}
	if req.StudentID == 0 {
		req.StudentID = id.AccountID
	}
	if req.Scope != TranscriptScopeSemester && req.Scope != TranscriptScopeFull {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed
	}
	summary, err := s.StudentSummary(ctx, req.StudentID)
	if err != nil {
		return TranscriptDTO{}, err
	}
	pdf, err := renderTranscriptPDF(summary, s.cfg.TranscriptSigningKey)
	if err != nil {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	key, err := storage.ObjectKey(id.TenantID, "grade", "transcript", fmt.Sprintf("%d.pdf", s.ids.Generate()))
	if err != nil {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	if err := s.storage.Put(ctx, s.storage.BucketReport(), key, bytes.NewReader(pdf), int64(len(pdf)), "application/pdf"); err != nil {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	req.StudentID = summary.StudentID
	ref := "minio://" + s.storage.BucketReport() + "/" + key
	var out TranscriptDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateTranscriptRecord(ctx, s.ids.Generate(), id.TenantID, req, ref)
		return err
	})
	return out, mapGradeTranscriptErr(err)
}

// DownloadTranscript 打开成绩单对象流。
func (s *Service) DownloadTranscript(ctx context.Context, transcriptID int64) (TranscriptDTO, io.ReadCloser, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return TranscriptDTO{}, nil, err
	}
	var record TranscriptDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		record, err = tx.GetTranscriptRecord(ctx, transcriptID)
		return err
	})
	if err != nil {
		return TranscriptDTO{}, nil, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	if record.StudentID != id.AccountID {
		allowed, err := s.canReadStudent(ctx, id.AccountID)
		if err != nil {
			return TranscriptDTO{}, nil, err
		}
		if !allowed {
			return TranscriptDTO{}, nil, apperr.ErrGradeForbidden
		}
	}
	obj, err := storage.ParseObjectRef(record.PDFRef)
	if err != nil {
		return TranscriptDTO{}, nil, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	reader, err := s.storage.Get(ctx, obj.Bucket, obj.Key)
	if err != nil {
		return TranscriptDTO{}, nil, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	return record, reader, nil
}

// HandleGradeUpdated 处理 M6 单课程成绩更新事件。
func (s *Service) HandleGradeUpdated(ctx context.Context, evt contracts.TeachingGradeUpdatedEvent) error {
	if evt.TenantID <= 0 || evt.CourseID <= 0 || evt.StudentID <= 0 {
		return apperr.ErrGradeAggregationFailed
	}
	var semesterID int64
	if err := s.store.TenantTx(ctx, evt.TenantID, func(ctx context.Context, tx TxStore) error {
		review, err := tx.GetLatestApprovedReviewByCourse(ctx, evt.CourseID)
		if err == nil {
			semesterID = review.SemesterID
		}
		appeals, err := tx.ListAcceptedAppealsByCourseStudent(ctx, evt.CourseID, evt.StudentID)
		if err != nil {
			return err
		}
		for _, appeal := range appeals {
			if _, err := tx.UpdateGradeAppealStatus(ctx, appeal.ID, AppealStatusCompleted, appeal.HandlerID, "成绩已更新"); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	if semesterID > 0 {
		if err := s.recomputeStudent(ctx, evt.TenantID, evt.StudentID, semesterID); err != nil {
			return err
		}
		return s.bus.Publish(ctx, contracts.SubjectGradeReviewLockChanged, contracts.GradeReviewLockChangedEvent{TenantID: evt.TenantID, CourseID: evt.CourseID, Locked: true, Reason: "grade_updated", ChangedAt: timex.Now()})
	}
	return nil
}

// decideAppeal 按目标状态处理申诉并在受理时发布解锁事件。
func (s *Service) decideAppeal(ctx context.Context, appealID int64, status int16, comment string) (AppealDTO, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return AppealDTO{}, err
	}
	var out AppealDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.UpdateGradeAppealStatus(ctx, appealID, status, id.AccountID, comment)
		return err
	})
	if err != nil {
		return AppealDTO{}, mapGradeAppealErr(err)
	}
	if status == AppealStatusAccepted {
		var review ReviewDTO
		err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
			var err error
			review, err = tx.GetLatestApprovedReviewByCourse(ctx, out.CourseID)
			return err
		})
		if err == nil {
			return out, s.publishLock(ctx, review, false, "appeal_accepted")
		}
	}
	return out, nil
}

// recomputeCourse 按课程成绩列表重算涉及学生的 GPA。
func (s *Service) recomputeCourse(ctx context.Context, tenantID, courseID, semesterID int64) error {
	grades, err := s.teaching.ListCourseGrades(ctx, tenantID, courseID)
	if err != nil {
		return apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	seen := map[int64]struct{}{}
	for _, row := range grades {
		if _, ok := seen[row.StudentID]; ok {
			continue
		}
		seen[row.StudentID] = struct{}{}
		if err := s.recomputeStudent(ctx, tenantID, row.StudentID, semesterID); err != nil {
			return err
		}
	}
	return nil
}

// recomputeStudent 从 M6 读取单课程成绩并保存 M11 自有聚合结果。
func (s *Service) recomputeStudent(ctx context.Context, tenantID, studentID, semesterID int64) error {
	grades, err := s.teaching.ListStudentGrades(ctx, tenantID, studentID)
	if err != nil {
		return apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	cfg, err := s.defaultConfig(ctx, tenantID)
	if err != nil {
		return err
	}
	inputs := courseInputs(grades)
	gpa, credits, err := ComputeGPA(inputs, cfg.Mapping)
	if err != nil {
		return err
	}
	return s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.UpsertStudentSemesterGrade(ctx, s.ids.Generate(), tenantID, studentID, semesterID, credits, gpa, gpa); err != nil {
			return apperr.ErrGradeAggregationFailed.WithCause(err)
		}
		return s.createWarnings(ctx, tx, tenantID, studentID, semesterID, inputs, gpa, cfg.WarningRules)
	})
}

// createWarnings 根据预警规则写入学业预警并统一走通知模块提醒学生。
func (s *Service) createWarnings(ctx context.Context, tx TxStore, tenantID, studentID, semesterID int64, grades []CourseGradeInput, gpa float64, rules WarningRules) error {
	failCount := 0
	for _, row := range grades {
		if row.FinalTotal < 60 {
			failCount++
		}
	}
	if rules.FailCount > 0 && failCount >= rules.FailCount {
		if _, err := tx.CreateAcademicWarning(ctx, s.ids.Generate(), tenantID, studentID, semesterID, WarningTypeFailedCourse, map[string]any{"failed_count": failCount}); err != nil {
			return apperr.ErrGradeWarningInvalid.WithCause(err)
		}
	}
	if rules.MinGPA > 0 && gpa < rules.MinGPA {
		if _, err := tx.CreateAcademicWarning(ctx, s.ids.Generate(), tenantID, studentID, semesterID, WarningTypeLowGPA, map[string]any{"gpa": gpa}); err != nil {
			return apperr.ErrGradeWarningInvalid.WithCause(err)
		}
	}
	if s.notify != nil && (failCount > 0 || (rules.MinGPA > 0 && gpa < rules.MinGPA)) {
		return s.notify.Send(ctx, contracts.NotifySendRequest{TenantID: tenantID, Type: "grade.warning", Receivers: []int64{studentID}, Params: map[string]string{"gpa": fmt.Sprintf("%.3f", gpa)}})
	}
	return nil
}

// defaultConfig 读取租户默认等级映射配置。
func (s *Service) defaultConfig(ctx context.Context, tenantID int64) (LevelConfigDTO, error) {
	var cfg LevelConfigDTO
	err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		cfg, err = tx.GetDefaultLevelConfig(ctx)
		return err
	})
	if err != nil {
		return LevelConfigDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	return cfg, nil
}

// publishLock 发布成绩审核锁定状态变更事件给 M6。
func (s *Service) publishLock(ctx context.Context, review ReviewDTO, locked bool, reason string) error {
	if review.CourseID <= 0 {
		return apperr.ErrGradeReviewInvalid
	}
	err := s.bus.Publish(ctx, contracts.SubjectGradeReviewLockChanged, contracts.GradeReviewLockChangedEvent{TenantID: review.TenantID, ReviewID: review.ID, CourseID: review.CourseID, Locked: locked, Reason: reason, ChangedAt: timex.Now()})
	if err != nil {
		return apperr.ErrGradeReviewStateInvalid.WithCause(err)
	}
	return nil
}

// canReadStudent 判断账号是否具备查看其他学生成绩的角色。
func (s *Service) canReadStudent(ctx context.Context, accountID int64) (bool, error) {
	for _, role := range []string{contracts.RoleTeacher, contracts.RoleSchoolAdmin} {
		has, err := s.roles.HasRole(ctx, accountID, role)
		if err != nil {
			return false, apperr.ErrGradeForbidden.WithCause(err)
		}
		if has {
			return true, nil
		}
	}
	return false, nil
}

// courseInputs 把 M6 单课程成绩转换为 GPA 计算输入。
func courseInputs(rows []contracts.TeachingCourseGrade) []CourseGradeInput {
	out := make([]CourseGradeInput, 0, len(rows))
	for _, row := range rows {
		out = append(out, CourseGradeInput{CourseID: row.CourseID, StudentID: row.StudentID, FinalTotal: row.FinalTotal, Credits: row.Credits})
	}
	return out
}

// validateLevelConfig 校验等级配置可用于 GPA 计算。
func validateLevelConfig(req LevelConfigRequest) error {
	if strings.TrimSpace(req.Name) == "" || len(req.Mapping) == 0 {
		return apperr.ErrGradeConfigInvalid
	}
	_, _, err := ComputeGPA([]CourseGradeInput{{FinalTotal: 100, Credits: 1}}, req.Mapping)
	return err
}

// requireTenantUser 校验当前请求来自租户内账号。
func requireTenantUser(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 || id.IsPlatform {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	return id, nil
}

// requireTeacherAdmin 校验当前账号具备教师或学校管理员角色。
func (s *Service) requireTeacherAdmin(ctx context.Context) (tenant.Identity, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return tenant.Identity{}, err
	}
	allowed, err := s.canReadStudent(ctx, id.AccountID)
	if err != nil {
		return tenant.Identity{}, err
	}
	if !allowed {
		return tenant.Identity{}, apperr.ErrGradeForbidden
	}
	return id, nil
}

// mapGradeConfigErr 统一转换配置类错误码。
func mapGradeConfigErr(err error) error {
	if err == nil {
		return nil
	}
	return apperr.ErrGradeConfigInvalid.WithCause(err)
}

// mapGradeReviewErr 统一转换审核类错误码。
func mapGradeReviewErr(err error) error {
	if err == nil {
		return nil
	}
	return apperr.ErrGradeReviewStateInvalid.WithCause(err)
}

// mapGradeAppealErr 保留申诉窗口等专属错误码。
func mapGradeAppealErr(err error) error {
	if err == nil {
		return nil
	}
	return apperr.AsAppError(err)
}

// mapGradeWarningErr 统一转换预警类错误码。
func mapGradeWarningErr(err error) error {
	if err == nil {
		return nil
	}
	return apperr.ErrGradeWarningInvalid.WithCause(err)
}

// mapGradeTranscriptErr 统一转换成绩单类错误码。
func mapGradeTranscriptErr(err error) error {
	if err == nil {
		return nil
	}
	return apperr.ErrGradeTranscriptFailed.WithCause(err)
}
