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
	return out, mapGradeReviewErr(err)
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
		return err
	})
	if err != nil {
		return ReviewDTO{}, mapGradeReviewErr(err)
	}
	return out, s.publishLock(ctx, out, false, "review_unlocked")
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
		allowed, err := s.isSchoolAdmin(ctx, id.AccountID)
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
		return err
	}); err != nil {
		return apperr.ErrGradeReviewStateInvalid.WithCause(err)
	}
	return s.publishLock(ctx, relocked, true, "grade_updated")
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
			if err != nil {
				return err
			}
			review, err = tx.UnlockGradeReview(ctx, review.ID, id.AccountID, comment)
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
	if s.notify != nil && (failCount > 0 || (rules.MinGPA > 0 && gpa < rules.MinGPA)) {
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
