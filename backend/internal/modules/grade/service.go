// grade service 文件实现 M11 成绩中心审核、GPA、申诉、预警和成绩单业务编排。
package grade

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"
)

const (
	gradeModuleName             = "grade"
	gradeTranscriptResourceType = "transcript"
	transcriptPDFContentType    = "application/pdf"
)

type objectStorage interface {
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	BucketReport() string
}

type fileService interface {
	PlanUpload(ctx context.Context, req storage.PlanUploadRequest) (storage.UploadPlan, error)
	IssueDownloadGrant(req storage.IssueDownloadGrantRequest) (string, storage.DownloadGrant, error)
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
	files    fileService
	cfg      config.GradeConfig
}

// ServiceDeps 是 M11 服务装配依赖。
type ServiceDeps struct {
	Store       Store
	IDs         snowflake.Generator
	Audit       audit.Writer
	Roles       roleReader
	Teaching    contracts.TeachingReadService
	Notify      contracts.NotifyService
	Bus         eventbus.Bus
	Storage     *storage.Storage
	Objects     objectStorage
	FileService fileService
	Config      config.GradeConfig
}

// NewService 构造 M11 服务。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil || deps.IDs == nil || deps.Audit == nil || deps.Roles == nil || deps.Teaching == nil || deps.Notify == nil || deps.Bus == nil {
		return nil, fmt.Errorf("grade service 依赖不完整")
	}
	objects := deps.Objects
	if objects == nil {
		objects = deps.Storage
	}
	if objects == nil {
		return nil, fmt.Errorf("grade service 对象存储依赖不完整")
	}
	if deps.FileService == nil {
		return nil, fmt.Errorf("grade service 统一文件服务依赖不完整")
	}
	if deps.Config.AppealWindowDays <= 0 || strings.TrimSpace(deps.Config.TranscriptSigningKey) == "" || deps.Config.TranscriptMaxBytes <= 0 || deps.Config.LockOutboxBatchSize <= 0 || deps.Config.LockOutboxStaleMs <= 0 {
		return nil, fmt.Errorf("grade service 配置不完整")
	}
	return &Service{store: deps.Store, ids: deps.IDs, audit: deps.Audit, roles: deps.Roles, teaching: deps.Teaching, notify: deps.Notify, bus: deps.Bus, storage: objects, files: deps.FileService, cfg: deps.Config}, nil
}

// CreateLevelConfig 创建等级映射配置。
func (s *Service) CreateLevelConfig(ctx context.Context, req LevelConfigRequest) (LevelConfigDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
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
	id, err := s.requireSchoolAdmin(ctx)
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
	id, err := s.requireSchoolAdmin(ctx)
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

// GetWarningRules 读取默认等级配置中的学业预警规则。
func (s *Service) GetWarningRules(ctx context.Context) (WarningRules, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return WarningRules{}, err
	}
	cfg, err := s.defaultConfig(ctx, id.TenantID)
	if err != nil {
		return WarningRules{}, err
	}
	return cfg.WarningRules, nil
}

// UpdateWarningRules 更新默认等级配置中的学业预警规则。
func (s *Service) UpdateWarningRules(ctx context.Context, rules WarningRules) (WarningRules, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return WarningRules{}, err
	}
	if err := validateWarningRules(rules); err != nil {
		return WarningRules{}, err
	}
	cfg, err := s.defaultConfig(ctx, id.TenantID)
	if err != nil {
		return WarningRules{}, err
	}
	req := LevelConfigRequest{Name: cfg.Name, Mapping: cfg.Mapping, WarningRules: rules, IsDefault: cfg.IsDefault}
	var out LevelConfigDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.UpdateLevelConfig(ctx, cfg.ID, req)
		return err
	})
	if err != nil {
		return WarningRules{}, mapGradeConfigErr(err)
	}
	return out.WarningRules, nil
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
	actorRole, err := s.gradeActorRole(ctx, id.AccountID)
	if err != nil {
		return ReviewDTO{}, err
	}
	course, err := s.validateReviewCourse(ctx, id, req.CourseID, req.SemesterID)
	if err != nil {
		return ReviewDTO{}, err
	}
	var out ReviewDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateGradeReview(ctx, s.ids.Generate(), id.TenantID, id.AccountID, req)
		return err
	})
	if err != nil {
		return ReviewDTO{}, mapGradeReviewErr(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, actorRole, "grade.review.submit", auditTargetGradeReview, out.ID, map[string]any{"course_id": course.CourseID, "semester": course.Semester}); err != nil {
		return ReviewDTO{}, err
	}
	return out, nil
}

// ListReviews 查询成绩审核列表。
func (s *Service) ListReviews(ctx context.Context, status int16, page, size int) ([]ReviewDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
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
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	if req.SemesterID <= 0 {
		return ReviewDTO{}, apperr.ErrGradeReviewInvalid
	}
	if _, err := s.getSemester(ctx, id.TenantID, req.SemesterID); err != nil {
		return ReviewDTO{}, err
	}
	var out ReviewDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ApproveGradeReview(ctx, reviewID, id.AccountID, req.SemesterID, req.Comment)
		if err != nil {
			return err
		}
		return s.enqueueLockOutbox(ctx, tx, out, true, "review_approved")
	})
	if err != nil {
		return ReviewDTO{}, mapGradeReviewErr(err)
	}
	if err := s.validateReviewCourseMatchesSemester(ctx, id.TenantID, out.CourseID, out.SemesterID); err != nil {
		return ReviewDTO{}, err
	}
	s.drainLockOutboxBestEffort(ctx)
	if err := s.recomputeCourse(ctx, id.TenantID, out.CourseID, out.SemesterID); err != nil {
		return ReviewDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleSchoolAdmin, "grade.review.approve", auditTargetGradeReview, out.ID, map[string]any{"course_id": out.CourseID, "semester_id": out.SemesterID}); err != nil {
		return ReviewDTO{}, err
	}
	return out, nil
}

// RejectReview 驳回成绩审核。
func (s *Service) RejectReview(ctx context.Context, reviewID int64, req ReviewDecisionRequest) (ReviewDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	var out ReviewDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.RejectGradeReview(ctx, reviewID, id.AccountID, req.Comment)
		return err
	})
	if err != nil {
		return ReviewDTO{}, mapGradeReviewErr(err)
	}
	if _, err := s.teaching.GetCourse(ctx, id.TenantID, out.CourseID); err != nil {
		return ReviewDTO{}, apperr.ErrGradeReviewInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleSchoolAdmin, "grade.review.reject", auditTargetGradeReview, out.ID, map[string]any{"course_id": out.CourseID}); err != nil {
		return ReviewDTO{}, err
	}
	return out, nil
}

// UnlockReview 解锁课程成绩,供审核重开或申诉受理后由 M6 改分。
func (s *Service) UnlockReview(ctx context.Context, reviewID int64, req ReviewDecisionRequest) (ReviewDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	var out ReviewDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.UnlockGradeReview(ctx, reviewID, id.AccountID, req.Comment)
		if err != nil {
			return err
		}
		return s.enqueueLockOutbox(ctx, tx, out, false, "review_unlocked")
	})
	if err != nil {
		return ReviewDTO{}, mapGradeReviewErr(err)
	}
	if _, err := s.teaching.GetCourse(ctx, id.TenantID, out.CourseID); err != nil {
		return ReviewDTO{}, apperr.ErrGradeReviewInvalid.WithCause(err)
	}
	s.drainLockOutboxBestEffort(ctx)
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleSchoolAdmin, "grade.review.unlock", auditTargetGradeReview, out.ID, map[string]any{"course_id": out.CourseID}); err != nil {
		return ReviewDTO{}, err
	}
	return out, nil
}

// StudentSummary 查询学生 GPA 汇总。
func (s *Service) StudentSummary(ctx context.Context, studentID int64) (GradeSummaryDTO, error) {
	id, studentID, err := s.normalizeReadableStudent(ctx, studentID)
	if err != nil {
		return GradeSummaryDTO{}, err
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

// StudentGrades 查询学生实时课程成绩明细并计算指定学期或全部 GPA。
func (s *Service) StudentGrades(ctx context.Context, studentID, semesterID int64) (GradeSummaryDTO, error) {
	id, studentID, err := s.normalizeReadableStudent(ctx, studentID)
	if err != nil {
		return GradeSummaryDTO{}, err
	}
	grades, err := s.teaching.ListStudentGrades(ctx, id.TenantID, studentID)
	if err != nil {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	if semesterID > 0 {
		semester, err := s.getSemester(ctx, id.TenantID, semesterID)
		if err != nil {
			return GradeSummaryDTO{}, err
		}
		grades = filterCourseGradesBySemester(grades, semester.Name)
	}
	cfg, err := s.defaultConfig(ctx, id.TenantID)
	if err != nil {
		return GradeSummaryDTO{}, err
	}
	inputs := courseInputs(grades)
	gpa, credits, err := ComputeGPA(inputs, cfg.Mapping)
	if err != nil {
		return GradeSummaryDTO{}, err
	}
	return GradeSummaryDTO{StudentID: studentID, SemesterID: semesterID, GPA: gpa, CumulativeGPA: gpa, TotalCredits: credits, CourseGrades: inputs, ComputedAt: timex.Now()}, nil
}

// StudentGPA 查询学生已落库的学期与累计 GPA 聚合结果。
func (s *Service) StudentGPA(ctx context.Context, studentID int64) ([]GradeSummaryDTO, error) {
	id, studentID, err := s.normalizeReadableStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	var out []GradeSummaryDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListStudentSemesterGrades(ctx, studentID)
		return err
	})
	if err != nil {
		return nil, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	return out, nil
}

// RecomputeStudentGrade 重算指定学生学期 GPA 并返回聚合结果。
func (s *Service) RecomputeStudentGrade(ctx context.Context, studentID int64, req RecomputeRequest) (GradeSummaryDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return GradeSummaryDTO{}, err
	}
	if studentID <= 0 || req.SemesterID <= 0 {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed
	}
	if err := s.recomputeStudent(ctx, id.TenantID, studentID, req.SemesterID); err != nil {
		return GradeSummaryDTO{}, err
	}
	var summaries []GradeSummaryDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		summaries, err = tx.ListStudentSemesterGrades(ctx, studentID)
		return err
	})
	if err != nil {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	for _, summary := range summaries {
		if summary.SemesterID == req.SemesterID {
			return summary, nil
		}
	}
	return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed
}

// CreateAppeal 创建成绩申诉。
func (s *Service) CreateAppeal(ctx context.Context, req AppealRequest) (AppealDTO, error) {
	id, err := s.requireStudent(ctx)
	if err != nil {
		return AppealDTO{}, err
	}
	if req.CourseID <= 0 || strings.TrimSpace(req.Reason) == "" {
		return AppealDTO{}, apperr.ErrGradeAppealInvalid
	}
	if err := s.validateAppealCourse(ctx, id.TenantID, req.CourseID, id.AccountID); err != nil {
		return AppealDTO{}, err
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
	if err != nil {
		return AppealDTO{}, mapGradeAppealErr(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleStudent, "grade.appeal.create", auditTargetAppeal, out.ID, map[string]any{"course_id": req.CourseID}); err != nil {
		return AppealDTO{}, err
	}
	return out, nil
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
	if studentID == 0 {
		studentID = id.AccountID
	}
	isAdmin, err := s.roles.HasRole(ctx, id.AccountID, contracts.RoleSchoolAdmin)
	if err != nil {
		return nil, apperr.ErrGradeForbidden.WithCause(err)
	}
	if !isAdmin {
		isStudent, err := s.roles.HasRole(ctx, id.AccountID, contracts.RoleStudent)
		if err != nil {
			return nil, apperr.ErrGradeForbidden.WithCause(err)
		}
		if !isStudent || studentID != id.AccountID {
			return nil, apperr.ErrGradeForbidden
		}
	}
	if studentID <= 0 {
		return nil, apperr.ErrGradeForbidden
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
	id, err := s.requireStudent(ctx)
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

// ScanWarnings 基于已有学生学期 GPA 聚合记录重算并复核预警。
func (s *Service) ScanWarnings(ctx context.Context, req WarningScanRequest) (WarningScanResultDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return WarningScanResultDTO{}, err
	}
	if req.StudentID < 0 || req.SemesterID < 0 {
		return WarningScanResultDTO{}, apperr.ErrGradeWarningInvalid
	}
	if req.SemesterID > 0 {
		if _, err := s.getSemester(ctx, id.TenantID, req.SemesterID); err != nil {
			return WarningScanResultDTO{}, err
		}
	}
	var targets []GradeSummaryDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		targets, err = tx.ListKnownStudentSemesterGrades(ctx, req.StudentID)
		return err
	})
	if err != nil {
		return WarningScanResultDTO{}, apperr.ErrGradeWarningInvalid.WithCause(err)
	}
	out := WarningScanResultDTO{}
	seen := map[[2]int64]struct{}{}
	for _, target := range targets {
		if req.SemesterID > 0 && target.SemesterID != req.SemesterID {
			continue
		}
		key := [2]int64{target.StudentID, target.SemesterID}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		created, err := s.recomputeStudentWarnings(ctx, id.TenantID, target.StudentID, target.SemesterID)
		if err != nil {
			return WarningScanResultDTO{}, err
		}
		out.Scanned++
		out.Created += created
	}
	return out, nil
}

// GenerateTranscript 生成成绩单 PDF 并保存元数据。
func (s *Service) GenerateTranscript(ctx context.Context, req TranscriptRequest) (TranscriptDTO, error) {
	id, err := s.normalizeStudentOrSchoolAdmin(ctx, req.StudentID)
	if err != nil {
		return TranscriptDTO{}, err
	}
	if req.Scope != TranscriptScopeSemester && req.Scope != TranscriptScopeFull {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed
	}
	if req.Scope == TranscriptScopeSemester && req.SemesterID <= 0 {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed
	}
	summary, err := s.transcriptSummary(ctx, req.StudentID, req.Scope, req.SemesterID)
	if err != nil {
		return TranscriptDTO{}, err
	}
	pdf, err := renderTranscriptPDF(summary, s.cfg.TranscriptSigningKey)
	if err != nil {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	fileName := fmt.Sprintf("%d.pdf", s.ids.Generate())
	plan, err := s.files.PlanUpload(ctx, storage.PlanUploadRequest{
		TenantID:        id.TenantID,
		AccountID:       id.AccountID,
		Module:          gradeModuleName,
		ResourceType:    gradeTranscriptResourceType,
		ResourceID:      fmt.Sprintf("%d", summary.StudentID),
		FileName:        fileName,
		ContentType:     transcriptPDFContentType,
		Size:            int64(len(pdf)),
		MaxBytes:        s.cfg.TranscriptMaxBytes,
		ExpectedBucket:  s.storage.BucketReport(),
		AllowedFileName: true,
		Content:         pdf,
	})
	if err != nil {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	if err := s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(pdf), int64(len(pdf)), transcriptPDFContentType); err != nil {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	req.StudentID = summary.StudentID
	var out TranscriptDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateTranscriptRecord(ctx, s.ids.Generate(), id.TenantID, req, plan.ObjectRef)
		return err
	})
	if err != nil {
		return TranscriptDTO{}, mapGradeTranscriptErr(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, s.transcriptActorRole(ctx, id.AccountID, summary.StudentID), "grade.transcript.generate", auditTargetTranscript, out.ID, map[string]any{"student_id": summary.StudentID, "scope": req.Scope, "semester_id": req.SemesterID}); err != nil {
		return TranscriptDTO{}, err
	}
	return out, nil
}

// GenerateTranscriptBatch 为多个学生生成成绩单,每份成绩单复用单份记录与对象存储流程。
func (s *Service) GenerateTranscriptBatch(ctx context.Context, req TranscriptBatchRequest) ([]TranscriptDTO, error) {
	if _, err := s.requireSchoolAdmin(ctx); err != nil {
		return nil, err
	}
	if req.Scope != TranscriptScopeSemester && req.Scope != TranscriptScopeFull {
		return nil, apperr.ErrGradeTranscriptFailed
	}
	if len(req.StudentIDs) == 0 || len(req.StudentIDs) > 200 {
		return nil, apperr.ErrGradeTranscriptFailed
	}
	out := make([]TranscriptDTO, 0, len(req.StudentIDs))
	seen := map[int64]struct{}{}
	for _, studentID := range req.StudentIDs {
		if studentID <= 0 {
			return nil, apperr.ErrGradeTranscriptFailed
		}
		if _, ok := seen[studentID]; ok {
			continue
		}
		seen[studentID] = struct{}{}
		item, err := s.GenerateTranscript(ctx, TranscriptRequest{StudentID: studentID, Scope: req.Scope, SemesterID: req.SemesterID})
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// DownloadTranscript 为成绩单对象签发短时下载授权。
func (s *Service) DownloadTranscript(ctx context.Context, transcriptID int64) (TranscriptDownloadGrantDTO, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return TranscriptDownloadGrantDTO{}, err
	}
	var record TranscriptDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		record, err = tx.GetTranscriptRecord(ctx, transcriptID)
		return err
	})
	if err != nil {
		return TranscriptDownloadGrantDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	if record.StudentID != id.AccountID {
		allowed, err := s.isSchoolAdmin(ctx, id.AccountID)
		if err != nil {
			return TranscriptDownloadGrantDTO{}, err
		}
		if !allowed {
			return TranscriptDownloadGrantDTO{}, apperr.ErrGradeForbidden
		}
	}
	token, grant, err := s.files.IssueDownloadGrant(storage.IssueDownloadGrantRequest{
		TenantID:     id.TenantID,
		AccountID:    id.AccountID,
		ObjectRef:    record.PDFRef,
		Module:       gradeModuleName,
		ResourceType: gradeTranscriptResourceType,
		ResourceID:   fmt.Sprintf("%d", record.StudentID),
		ExpiresAt:    time.Time{},
	})
	if err != nil {
		return TranscriptDownloadGrantDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, s.transcriptActorRole(ctx, id.AccountID, record.StudentID), "grade.transcript.download", auditTargetTranscript, record.ID, map[string]any{"student_id": record.StudentID}); err != nil {
		return TranscriptDownloadGrantDTO{}, err
	}
	return TranscriptDownloadGrantDTO{Token: token, Grant: grant, Transcript: record, ExpiresAt: grant.ExpiresAt.Format(time.RFC3339)}, nil
}

// HandleGradeUpdated 处理 M6 单课程成绩更新事件。
func (s *Service) HandleGradeUpdated(ctx context.Context, evt contracts.TeachingGradeUpdatedEvent) error {
	if evt.TenantID <= 0 || evt.CourseID <= 0 || evt.StudentID <= 0 {
		return apperr.ErrGradeAggregationFailed
	}
	var review ReviewDTO
	var acceptedAppeals []AppealDTO
	if err := s.store.TenantTx(ctx, evt.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		review, err = tx.GetLatestReviewByCourse(ctx, evt.CourseID)
		if err == nil {
			if review.SemesterID <= 0 {
				return apperr.ErrGradeReviewStateInvalid
			}
		} else {
			return err
		}
		acceptedAppeals, err = tx.ListAcceptedAppealsByCourseStudent(ctx, evt.CourseID, evt.StudentID)
		if err != nil {
			return err
		}
		for _, appeal := range acceptedAppeals {
			if _, err := tx.UpdateGradeAppealStatus(ctx, appeal.ID, AppealStatusCompleted, appeal.HandlerID, "成绩已更新"); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	if len(acceptedAppeals) == 0 {
		return nil
	}
	if err := s.recomputeStudent(ctx, evt.TenantID, evt.StudentID, review.SemesterID); err != nil {
		return err
	}
	var relocked ReviewDTO
	if err := s.store.TenantTx(ctx, evt.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		relocked, err = tx.RelockGradeReview(ctx, review.ID, review.ReviewerID, "成绩已更新并完成复核")
		if err != nil {
			return err
		}
		return s.enqueueLockOutbox(ctx, tx, relocked, true, "grade_updated")
	}); err != nil {
		return apperr.ErrGradeReviewStateInvalid.WithCause(err)
	}
	s.drainLockOutboxBestEffort(ctx)
	return nil
}

// decideAppeal 按目标状态处理申诉并在受理时发布解锁事件。
func (s *Service) decideAppeal(ctx context.Context, appealID int64, status int16, comment string) (AppealDTO, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return AppealDTO{}, err
	}
	actorRole, err := s.gradeActorRole(ctx, id.AccountID)
	if err != nil {
		return AppealDTO{}, err
	}
	var existing AppealDTO
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		existing, err = tx.GetGradeAppeal(ctx, appealID)
		return err
	}); err != nil {
		return AppealDTO{}, mapGradeAppealErr(err)
	}
	if err := s.ensureAppealHandlerCanAccessCourse(ctx, id, actorRole, existing.CourseID); err != nil {
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
	action := "grade.appeal.reject"
	if status == AppealStatusAccepted {
		action = "grade.appeal.accept"
	}
	if status == AppealStatusAccepted {
		var review ReviewDTO
		err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
			var err error
			review, err = tx.GetLatestApprovedReviewByCourse(ctx, out.CourseID)
			if err != nil {
				return err
			}
			review, err = tx.UnlockGradeReview(ctx, review.ID, id.AccountID, comment)
			if err != nil {
				return err
			}
			return s.enqueueLockOutbox(ctx, tx, review, false, "appeal_accepted")
		})
		if err == nil {
			s.drainLockOutboxBestEffort(ctx)
			if err := s.writeAudit(ctx, id.TenantID, id.AccountID, actorRole, action, auditTargetAppeal, out.ID, map[string]any{"course_id": out.CourseID}); err != nil {
				return AppealDTO{}, err
			}
			return out, nil
		}
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, actorRole, action, auditTargetAppeal, out.ID, map[string]any{"course_id": out.CourseID}); err != nil {
		return AppealDTO{}, err
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
	_, err := s.recomputeStudentWarnings(ctx, tenantID, studentID, semesterID)
	return err
}

// recomputeStudentWarnings 重算学生 GPA 并返回本次新建的预警数量。
func (s *Service) recomputeStudentWarnings(ctx context.Context, tenantID, studentID, semesterID int64) (int, error) {
	grades, err := s.teaching.ListStudentGrades(ctx, tenantID, studentID)
	if err != nil {
		return 0, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	if semesterID > 0 {
		semester, err := s.getSemester(ctx, tenantID, semesterID)
		if err != nil {
			return 0, err
		}
		grades = filterCourseGradesBySemester(grades, semester.Name)
	}
	cfg, err := s.defaultConfig(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	inputs := courseInputs(grades)
	gpa, credits, err := ComputeGPA(inputs, cfg.Mapping)
	if err != nil {
		return 0, err
	}
	created := 0
	err = s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.UpsertStudentSemesterGrade(ctx, s.ids.Generate(), tenantID, studentID, semesterID, credits, gpa, gpa); err != nil {
			return apperr.ErrGradeAggregationFailed.WithCause(err)
		}
		var err error
		created, err = s.createWarnings(ctx, tx, tenantID, studentID, semesterID, inputs, gpa, cfg.WarningRules)
		return err
	})
	return created, err
}

// createWarnings 根据预警规则写入学业预警并统一走通知模块提醒学生。
func (s *Service) createWarnings(ctx context.Context, tx TxStore, tenantID, studentID, semesterID int64, grades []CourseGradeInput, gpa float64, rules WarningRules) (int, error) {
	failCount := 0
	for _, row := range grades {
		if row.FinalTotal < 60 {
			failCount++
		}
	}
	created := 0
	if rules.FailCount > 0 && failCount >= rules.FailCount {
		if _, err := tx.CreateAcademicWarning(ctx, s.ids.Generate(), tenantID, studentID, semesterID, WarningTypeFailedCourse, map[string]any{"failed_count": failCount}); err != nil {
			return 0, apperr.ErrGradeWarningInvalid.WithCause(err)
		}
		created++
	}
	if rules.MinGPA > 0 && gpa < rules.MinGPA {
		if _, err := tx.CreateAcademicWarning(ctx, s.ids.Generate(), tenantID, studentID, semesterID, WarningTypeLowGPA, map[string]any{"gpa": gpa}); err != nil {
			return 0, apperr.ErrGradeWarningInvalid.WithCause(err)
		}
		created++
	}
	if failCount > 0 || (rules.MinGPA > 0 && gpa < rules.MinGPA) {
		if err := s.notify.Send(ctx, contracts.NotifySendRequest{TenantID: tenantID, Type: "grade.warning", Receivers: []int64{studentID}, Params: map[string]string{"gpa": fmt.Sprintf("%.3f", gpa)}}); err != nil {
			return 0, err
		}
		return created, nil
	}
	return created, nil
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

// getSemester 在租户内读取指定学期配置。
func (s *Service) getSemester(ctx context.Context, tenantID, semesterID int64) (SemesterDTO, error) {
	var semesters []SemesterDTO
	err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		semesters, err = tx.ListSemesters(ctx)
		return err
	})
	if err != nil {
		return SemesterDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	for _, semester := range semesters {
		if semester.ID == semesterID {
			return semester, nil
		}
	}
	return SemesterDTO{}, apperr.ErrGradeConfigInvalid
}

// enqueueLockOutbox 在状态变更同一事务内保存 M6 锁定投影事件。
func (s *Service) enqueueLockOutbox(ctx context.Context, tx TxStore, review ReviewDTO, locked bool, reason string) error {
	traceID := strings.TrimSpace(response.TraceFromContext(ctx))
	if review.TenantID <= 0 || review.ID <= 0 || review.CourseID <= 0 || traceID == "" || strings.TrimSpace(reason) == "" {
		return apperr.ErrGradeEventPublishFailed
	}
	_, err := tx.CreateGradeLockOutbox(ctx, s.ids.Generate(), review, locked, reason, traceID)
	if err != nil {
		return apperr.ErrGradeEventPublishFailed.WithCause(err)
	}
	return nil
}

// RunLockOutboxOnce 领取并发布 M11 成绩锁事件,供后台任务和事务后补偿调用。
func (s *Service) RunLockOutboxOnce(ctx context.Context) error {
	limit := int32(s.cfg.LockOutboxBatchSize)
	if limit <= 0 {
		return apperr.ErrGradeEventPublishFailed
	}
	staleBefore := timex.Now().Add(-time.Duration(s.cfg.LockOutboxStaleMs) * time.Millisecond)
	var items []GradeLockOutbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ClaimPendingGradeLockOutbox(ctx, limit, staleBefore)
		if err != nil {
			return apperr.ErrGradeEventPublishFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, item := range items {
		if err := s.publishLockOutboxItem(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// publishLockOutboxItem 发布单条锁定事件并按结果回写 outbox 状态。
func (s *Service) publishLockOutboxItem(ctx context.Context, item GradeLockOutbox) error {
	eventCtx := response.WithTrace(ctx, item.TraceID)
	payload := contracts.GradeReviewLockChangedEvent{TenantID: item.TenantID, TraceID: item.TraceID, ReviewID: item.ReviewID, CourseID: item.CourseID, Locked: item.Locked, Reason: item.Reason, ChangedAt: timex.Now()}
	if err := s.bus.Publish(eventCtx, contracts.SubjectGradeReviewLockChanged, payload); err != nil {
		s.recordLockOutboxFailure(eventCtx, item, err)
		return apperr.ErrGradeEventPublishFailed.WithCause(err)
	}
	return s.markLockOutboxPublished(eventCtx, item)
}

// markLockOutboxPublished 用特权事务标记锁定事件投递成功。
func (s *Service) markLockOutboxPublished(ctx context.Context, item GradeLockOutbox) error {
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkGradeLockOutboxPublished(ctx, item.TenantID, item.ID)
		if err != nil {
			return apperr.ErrGradeEventPublishFailed.WithCause(err)
		}
		return nil
	})
}

// recordLockOutboxFailure 记录锁定事件投递失败并保留脱敏原因供后台重试。
func (s *Service) recordLockOutboxFailure(ctx context.Context, item GradeLockOutbox, cause error) {
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkGradeLockOutboxFailed(ctx, item.TenantID, item.ID, logging.SanitizeError(cause.Error()))
		return err
	}); err != nil {
		logging.ErrorContext(ctx, "grade lock outbox failure mark failed", err.Error(), slog.Int64("tenant_id", item.TenantID), slog.Int64("review_id", item.ReviewID), slog.Int64("outbox_id", item.ID))
	}
}

// drainLockOutboxBestEffort 在请求提交后尽快投递,失败只记录日志并交给后台任务补偿。
func (s *Service) drainLockOutboxBestEffort(ctx context.Context) {
	if err := s.RunLockOutboxOnce(ctx); err != nil {
		logging.ErrorContext(ctx, "grade lock outbox drain failed", err.Error())
	}
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

// gradeActorRole 按成绩中心角色优先级解析审计角色。
func (s *Service) gradeActorRole(ctx context.Context, accountID int64) (int16, error) {
	if has, err := s.roles.HasRole(ctx, accountID, contracts.RoleSchoolAdmin); err != nil {
		return 0, apperr.ErrGradeForbidden.WithCause(err)
	} else if has {
		return audit.ActorRoleSchoolAdmin, nil
	}
	if has, err := s.roles.HasRole(ctx, accountID, contracts.RoleTeacher); err != nil {
		return 0, apperr.ErrGradeForbidden.WithCause(err)
	} else if has {
		return audit.ActorRoleTeacher, nil
	}
	if has, err := s.roles.HasRole(ctx, accountID, contracts.RoleStudent); err != nil {
		return 0, apperr.ErrGradeForbidden.WithCause(err)
	} else if has {
		return audit.ActorRoleStudent, nil
	}
	return 0, apperr.ErrGradeForbidden
}

// transcriptActorRole 返回成绩单审计角色,管理员代生成和学生本人生成分开记录。
func (s *Service) transcriptActorRole(ctx context.Context, actorID, studentID int64) int16 {
	if actorID == studentID {
		return audit.ActorRoleStudent
	}
	return audit.ActorRoleSchoolAdmin
}

// validateReviewCourse 校验审核提交的课程存在、学期匹配且教师只能提交本人课程。
func (s *Service) validateReviewCourse(ctx context.Context, id tenant.Identity, courseID, semesterID int64) (contracts.TeachingCourseInfo, error) {
	course, err := s.teaching.GetCourse(ctx, id.TenantID, courseID)
	if err != nil {
		return contracts.TeachingCourseInfo{}, apperr.ErrGradeReviewInvalid.WithCause(err)
	}
	if course.TenantID != id.TenantID || course.CourseID != courseID {
		return contracts.TeachingCourseInfo{}, apperr.ErrGradeReviewInvalid
	}
	hasAdmin, err := s.isSchoolAdmin(ctx, id.AccountID)
	if err != nil {
		return contracts.TeachingCourseInfo{}, err
	}
	if !hasAdmin && course.TeacherID != id.AccountID {
		return contracts.TeachingCourseInfo{}, apperr.ErrGradeForbidden
	}
	if semesterID > 0 {
		if err := s.courseMatchesSemester(ctx, id.TenantID, course, semesterID); err != nil {
			return contracts.TeachingCourseInfo{}, err
		}
	}
	return course, nil
}

// validateReviewCourseMatchesSemester 校验审核通过时选择的 M11 学期与 M6 课程学期一致。
func (s *Service) validateReviewCourseMatchesSemester(ctx context.Context, tenantID, courseID, semesterID int64) error {
	course, err := s.teaching.GetCourse(ctx, tenantID, courseID)
	if err != nil {
		return apperr.ErrGradeReviewInvalid.WithCause(err)
	}
	return s.courseMatchesSemester(ctx, tenantID, course, semesterID)
}

// courseMatchesSemester 防止把课程成绩审核到错误学期。
func (s *Service) courseMatchesSemester(ctx context.Context, tenantID int64, course contracts.TeachingCourseInfo, semesterID int64) error {
	semester, err := s.getSemester(ctx, tenantID, semesterID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(course.Semester) != "" && course.Semester != semester.Name {
		return apperr.ErrGradeReviewInvalid
	}
	return nil
}

// validateAppealCourse 校验学生只能对自己所在课程且已有成绩的课程发起申诉。
func (s *Service) validateAppealCourse(ctx context.Context, tenantID, courseID, studentID int64) error {
	member, err := s.teaching.IsCourseMember(ctx, tenantID, courseID, studentID)
	if err != nil {
		return apperr.ErrGradeAppealInvalid.WithCause(err)
	}
	if !member {
		return apperr.ErrGradeForbidden
	}
	if _, err := s.teaching.GetCourseGrade(ctx, tenantID, courseID, studentID); err != nil {
		return apperr.ErrGradeAppealInvalid.WithCause(err)
	}
	return nil
}

// ensureAppealHandlerCanAccessCourse 限制教师只能处理本人课程的申诉,管理员可处理全校。
func (s *Service) ensureAppealHandlerCanAccessCourse(ctx context.Context, id tenant.Identity, actorRole int16, courseID int64) error {
	course, err := s.teaching.GetCourse(ctx, id.TenantID, courseID)
	if err != nil {
		return apperr.ErrGradeAppealInvalid.WithCause(err)
	}
	if actorRole == audit.ActorRoleTeacher && course.TeacherID != id.AccountID {
		return apperr.ErrGradeForbidden
	}
	return nil
}

// transcriptSummary 根据成绩单范围读取正确的 M6 成绩明细,避免学期成绩单混入全量课程。
func (s *Service) transcriptSummary(ctx context.Context, studentID int64, scope int16, semesterID int64) (GradeSummaryDTO, error) {
	if scope == TranscriptScopeSemester {
		return s.StudentGrades(ctx, studentID, semesterID)
	}
	return s.StudentSummary(ctx, studentID)
}

// requireStudent 校验当前账号具备学生角色。
func (s *Service) requireStudent(ctx context.Context) (tenant.Identity, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return tenant.Identity{}, err
	}
	has, err := s.roles.HasRole(ctx, id.AccountID, contracts.RoleStudent)
	if err != nil {
		return tenant.Identity{}, apperr.ErrGradeForbidden.WithCause(err)
	}
	if !has {
		return tenant.Identity{}, apperr.ErrGradeForbidden
	}
	return id, nil
}

// isSchoolAdmin 判断账号是否具备学校管理员权限。
func (s *Service) isSchoolAdmin(ctx context.Context, accountID int64) (bool, error) {
	has, err := s.roles.HasRole(ctx, accountID, contracts.RoleSchoolAdmin)
	if err != nil {
		return false, apperr.ErrGradeForbidden.WithCause(err)
	}
	return has, nil
}

// normalizeStudentOrSchoolAdmin 校验当前账号可操作指定学生范围。
func (s *Service) normalizeStudentOrSchoolAdmin(ctx context.Context, studentID int64) (tenant.Identity, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return tenant.Identity{}, err
	}
	if studentID == 0 {
		studentID = id.AccountID
	}
	if studentID <= 0 {
		return tenant.Identity{}, apperr.ErrGradeForbidden
	}
	if studentID == id.AccountID {
		hasStudent, err := s.roles.HasRole(ctx, id.AccountID, contracts.RoleStudent)
		if err != nil {
			return tenant.Identity{}, apperr.ErrGradeForbidden.WithCause(err)
		}
		if hasStudent {
			return id, nil
		}
	}
	hasAdmin, err := s.isSchoolAdmin(ctx, id.AccountID)
	if err != nil {
		return tenant.Identity{}, err
	}
	if !hasAdmin {
		return tenant.Identity{}, apperr.ErrGradeForbidden
	}
	return id, nil
}

// normalizeReadableStudent 统一处理学生本人默认值和横向读取权限。
func (s *Service) normalizeReadableStudent(ctx context.Context, studentID int64) (tenant.Identity, int64, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return tenant.Identity{}, 0, err
	}
	if studentID == 0 {
		studentID = id.AccountID
	}
	if id.AccountID != studentID {
		allowed, err := s.canReadStudent(ctx, id.AccountID)
		if err != nil {
			return tenant.Identity{}, 0, err
		}
		if !allowed {
			return tenant.Identity{}, 0, apperr.ErrGradeForbidden
		}
	}
	return id, studentID, nil
}

// courseInputs 把 M6 单课程成绩转换为 GPA 计算输入。
func courseInputs(rows []contracts.TeachingCourseGrade) []CourseGradeInput {
	out := make([]CourseGradeInput, 0, len(rows))
	for _, row := range rows {
		out = append(out, CourseGradeInput{CourseID: row.CourseID, StudentID: row.StudentID, FinalTotal: row.FinalTotal, Credits: row.Credits})
	}
	return out
}

// filterCourseGradesBySemester 按 M6 课程学期名过滤实时成绩明细。
func filterCourseGradesBySemester(rows []contracts.TeachingCourseGrade, semesterName string) []contracts.TeachingCourseGrade {
	out := make([]contracts.TeachingCourseGrade, 0, len(rows))
	for _, row := range rows {
		if row.Semester == semesterName {
			out = append(out, row)
		}
	}
	return out
}

// validateLevelConfig 校验等级配置可用于 GPA 计算。
func validateLevelConfig(req LevelConfigRequest) error {
	if strings.TrimSpace(req.Name) == "" || len(req.Mapping) == 0 {
		return apperr.ErrGradeConfigInvalid
	}
	if err := validateWarningRules(req.WarningRules); err != nil {
		return err
	}
	_, _, err := ComputeGPA([]CourseGradeInput{{FinalTotal: 100, Credits: 1}}, req.Mapping)
	return err
}

// validateWarningRules 校验学业预警阈值不会产生无意义配置。
func validateWarningRules(rules WarningRules) error {
	if rules.FailCount < 0 || rules.MinGPA < 0 || rules.MinGPA > 4 {
		return apperr.ErrGradeConfigInvalid
	}
	return nil
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

// requireSchoolAdmin 校验当前账号具备学校管理员角色。
func (s *Service) requireSchoolAdmin(ctx context.Context) (tenant.Identity, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return tenant.Identity{}, err
	}
	has, err := s.roles.HasRole(ctx, id.AccountID, contracts.RoleSchoolAdmin)
	if err != nil {
		return tenant.Identity{}, apperr.ErrGradeForbidden.WithCause(err)
	}
	if !has {
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
