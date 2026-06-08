// M11 服务层:实现跨课程 GPA 聚合、审核锁定、申诉、预警和成绩单元数据逻辑。
package grade

import (
	"bytes"
	"context"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

// gradeStore 抽象 M11 数据访问,便于服务规则测试。
type gradeStore interface {
	ListLevelConfigs(context.Context, int64) ([]LevelConfigDTO, error)
	CreateLevelConfig(context.Context, int64, int64, LevelConfigRequest) (LevelConfigDTO, error)
	UpdateLevelConfig(context.Context, int64, int64, LevelConfigRequest) (LevelConfigDTO, error)
	DefaultLevelConfig(context.Context, int64) (LevelConfigDTO, error)
	ListSemesters(context.Context, int64) ([]SemesterDTO, error)
	CreateSemester(context.Context, int64, int64, SemesterRequest) (SemesterDTO, error)
	UpsertSemesterGrade(context.Context, int64, SemesterGradeUpsert) (SemesterGradeDTO, error)
	ListSemesterGrades(context.Context, int64, int64) ([]SemesterGradeDTO, error)
	ListStudentSemesterGrades(context.Context, int64, int64) ([]SemesterGradeDTO, error)
	CreateReview(context.Context, int64, int64, ReviewCreateRequest, int64) (ReviewDTO, error)
	GetReview(context.Context, int64, int64) (ReviewDTO, error)
	GetReviewByCourse(context.Context, int64, int64) (ReviewDTO, error)
	ListReviews(context.Context, int64, int16, int, int) ([]ReviewDTO, int64, error)
	ApproveReview(context.Context, int64, int64, int64, int64, string) (ReviewDTO, error)
	RejectReview(context.Context, int64, int64, int64, string) (ReviewDTO, error)
	UnlockReview(context.Context, int64, int64, int64, string) (ReviewDTO, error)
	RelockReviewByCourse(context.Context, int64, int64) (ReviewDTO, error)
	CreateAppeal(context.Context, int64, int64, AppealCreateRequest, int64) (AppealDTO, error)
	FindOpenAppeal(context.Context, int64, int64, int64) (AppealDTO, bool, error)
	GetAppeal(context.Context, int64, int64) (AppealDTO, error)
	ListAppeals(context.Context, int64, int16, int, int) ([]AppealDTO, int64, error)
	UpdateAppealStatus(context.Context, int64, int64, int64, int16, string) (AppealDTO, error)
	CreateWarning(context.Context, int64, int64, WarningCreate) (WarningDTO, error)
	DeleteWarning(context.Context, int64, int64) error
	ListWarnings(context.Context, int64, int64, int64, int16, int, int) ([]WarningDTO, int64, error)
	AcknowledgeWarning(context.Context, int64, int64, int64) (WarningDTO, error)
	CreateTranscript(context.Context, int64, int64, TranscriptRequest, string) (TranscriptDTO, error)
	GetTranscript(context.Context, int64, int64) (TranscriptDTO, error)
}

// Service 是 M11 成绩中心服务。
type Service struct {
	store    gradeStore
	idgen    snowflake.Generator
	auditor  audit.Writer
	identity contracts.IdentityService
	teaching contracts.TeachingService
	contest  contracts.ContestService
	notify   contracts.NotifyService
	storage  *storage.Storage
	cfg      config.GradeConfig
}

// NewService 构造 M11 服务并注入下层只读 contracts。
func NewService(database *db.DB, idgen *snowflake.Node, auditor audit.Writer, identity contracts.IdentityService, teaching contracts.TeachingService, contest contracts.ContestService, notify contracts.NotifyService, store *storage.Storage, cfg config.GradeConfig) *Service {
	return &Service{store: newRepo(database), idgen: idgen, auditor: auditor, identity: identity, teaching: teaching, contest: contest, notify: notify, storage: store, cfg: cfg}
}

// ListLevelConfigs 查询当前租户等级配置。
func (s *Service) ListLevelConfigs(ctx context.Context) ([]LevelConfigDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
		return nil, err
	}
	return s.store.ListLevelConfigs(ctx, id.TenantID)
}

// CreateLevelConfig 创建等级映射配置。
func (s *Service) CreateLevelConfig(ctx context.Context, req LevelConfigRequest) (LevelConfigDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
		return LevelConfigDTO{}, err
	}
	if err := validateLevelConfig(req); err != nil {
		return LevelConfigDTO{}, err
	}
	return s.store.CreateLevelConfig(ctx, id.TenantID, s.nextID(), req)
}

// UpdateLevelConfig 更新等级映射配置。
func (s *Service) UpdateLevelConfig(ctx context.Context, configID int64, req LevelConfigRequest) (LevelConfigDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
		return LevelConfigDTO{}, err
	}
	if err := validateLevelConfig(req); err != nil {
		return LevelConfigDTO{}, err
	}
	return s.store.UpdateLevelConfig(ctx, id.TenantID, configID, req)
}

// ListSemesters 查询当前租户学期列表。
func (s *Service) ListSemesters(ctx context.Context) ([]SemesterDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
		return nil, err
	}
	return s.store.ListSemesters(ctx, id.TenantID)
}

// CreateSemester 创建学期配置。
func (s *Service) CreateSemester(ctx context.Context, req SemesterRequest) (SemesterDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return SemesterDTO{}, err
	}
	if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
		return SemesterDTO{}, err
	}
	if strings.TrimSpace(req.Name) == "" {
		return SemesterDTO{}, apperr.ErrGradeSemesterInvalid
	}
	return s.store.CreateSemester(ctx, id.TenantID, s.nextID(), req)
}

// WarningRules 返回默认成绩配置中的学业预警规则。
func (s *Service) WarningRules(ctx context.Context) (WarningRuleDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return WarningRuleDTO{}, err
	}
	if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
		return WarningRuleDTO{}, err
	}
	level, err := s.store.DefaultLevelConfig(ctx, id.TenantID)
	if err != nil {
		return WarningRuleDTO{}, err
	}
	return level.WarningRules, nil
}

// UpdateWarningRules 更新默认成绩配置中的学业预警规则。
func (s *Service) UpdateWarningRules(ctx context.Context, req WarningRuleDTO) (WarningRuleDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return WarningRuleDTO{}, err
	}
	if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
		return WarningRuleDTO{}, err
	}
	if req.FailCount < 0 || req.MinGPA < 0 {
		return WarningRuleDTO{}, apperr.ErrGradeConfigInvalid
	}
	level, err := s.store.DefaultLevelConfig(ctx, id.TenantID)
	if err != nil {
		return WarningRuleDTO{}, err
	}
	level.WarningRules = req
	update := LevelConfigRequest{Name: level.Name, Mapping: level.Mapping, WarningRules: level.WarningRules, IsDefault: level.IsDefault}
	if _, err = s.store.UpdateLevelConfig(ctx, id.TenantID, ids.ParseOrZero(level.ID), update); err != nil {
		return WarningRuleDTO{}, err
	}
	return req, nil
}

// SubmitReview 创建或重提课程成绩审核。
func (s *Service) SubmitReview(ctx context.Context, req ReviewCreateRequest) (ReviewDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	if err := s.ensureTeacherOrAdmin(ctx, id); err != nil {
		return ReviewDTO{}, err
	}
	if _, ok := ids.Parse(req.CourseID); !ok {
		return ReviewDTO{}, apperr.ErrGradeReviewInvalid
	}
	out, err := s.store.CreateReview(ctx, id.TenantID, s.nextID(), req, id.AccountID)
	if err != nil {
		return ReviewDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, id.AccountID, "grade.review.submit", "grade_review", ids.ParseOrZero(out.ID), map[string]any{"course_id": out.CourseID})
}

// ListReviews 查询成绩审核记录。
func (s *Service) ListReviews(ctx context.Context, status int16, page, size int) ([]ReviewDTO, int64, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return nil, 0, err
	}
	if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
		return nil, 0, err
	}
	return s.store.ListReviews(ctx, id.TenantID, status, page, size)
}

// CourseLockStatus 查询课程成绩锁定状态,供 M6 改分前展示或内部校验。
func (s *Service) CourseLockStatus(ctx context.Context, courseID int64) (ReviewDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	if !isInternalService(ctx) {
		if err := s.ensureTeacherOrAdmin(ctx, id); err != nil {
			return ReviewDTO{}, err
		}
	}
	if courseID <= 0 {
		return ReviewDTO{}, apperr.ErrGradeReviewInvalid
	}
	return s.store.GetReviewByCourse(ctx, id.TenantID, courseID)
}

// ApproveReview 审核通过课程成绩,锁定后重算本课程所有学生 GPA。
func (s *Service) ApproveReview(ctx context.Context, reviewID int64, req ReviewDecisionRequest) (ReviewDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
		return ReviewDTO{}, err
	}
	semesterID, ok := ids.Parse(req.SemesterID)
	if !ok {
		return ReviewDTO{}, apperr.ErrGradeReviewInvalid
	}
	current, err := s.store.GetReview(ctx, id.TenantID, reviewID)
	if err != nil {
		return ReviewDTO{}, err
	}
	if current.Status != ReviewStatusPending {
		return ReviewDTO{}, apperr.ErrGradeReviewState
	}
	out, err := s.store.ApproveReview(ctx, id.TenantID, id.AccountID, reviewID, semesterID, req.Comment)
	if err != nil {
		return ReviewDTO{}, err
	}
	if err := s.recomputeCourseStudents(ctx, id.TenantID, ids.ParseOrZero(out.CourseID), semesterID); err != nil {
		return ReviewDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, id.AccountID, "grade.review.approve", "grade_review", reviewID, map[string]any{"course_id": out.CourseID})
}

// RejectReview 驳回待审课程成绩。
func (s *Service) RejectReview(ctx context.Context, reviewID int64, req ReviewDecisionRequest) (ReviewDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
		return ReviewDTO{}, err
	}
	out, err := s.store.RejectReview(ctx, id.TenantID, id.AccountID, reviewID, req.Comment)
	if err != nil {
		return ReviewDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, id.AccountID, "grade.review.reject", "grade_review", reviewID, map[string]any{"course_id": out.CourseID})
}

// UnlockReview 解锁已审核课程成绩,回到待审状态。
func (s *Service) UnlockReview(ctx context.Context, reviewID int64, req ReviewDecisionRequest) (ReviewDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
		return ReviewDTO{}, err
	}
	out, err := s.store.UnlockReview(ctx, id.TenantID, id.AccountID, reviewID, req.Comment)
	if err != nil {
		return ReviewDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, id.AccountID, "grade.review.unlock", "grade_review", reviewID, map[string]any{"course_id": out.CourseID})
}

// RecomputeStudent 按 M6 单课程成绩重算指定学生 GPA。
func (s *Service) RecomputeStudent(ctx context.Context, studentID int64, req RecomputeRequest) (SemesterGradeDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return SemesterGradeDTO{}, err
	}
	if !isInternalService(ctx) {
		if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
			return SemesterGradeDTO{}, err
		}
	}
	courseID, ok := ids.Parse(req.CourseID)
	if !ok {
		return SemesterGradeDTO{}, apperr.ErrGradeAggregateInvalid
	}
	semesterID, ok := ids.Parse(req.SemesterID)
	if !ok {
		return SemesterGradeDTO{}, apperr.ErrGradeAggregateInvalid
	}
	return s.recomputeStudent(ctx, id.TenantID, courseID, studentID, semesterID)
}

// StudentGrades 查询学生课程明细与 GPA 聚合结果。
func (s *Service) StudentGrades(ctx context.Context, studentID int64, semesterID int64) (StudentGradesDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return StudentGradesDTO{}, err
	}
	if err := s.ensureSelfOrAdmin(ctx, id, studentID); err != nil {
		return StudentGradesDTO{}, err
	}
	gpa, err := s.store.ListStudentSemesterGrades(ctx, id.TenantID, studentID)
	if err != nil {
		return StudentGradesDTO{}, err
	}
	courses, err := s.studentCourseDTOs(ctx, id.TenantID, studentID)
	if err != nil {
		return StudentGradesDTO{}, err
	}
	return StudentGradesDTO{StudentID: ids.Format(studentID), Semester: ids.Format(semesterID), Courses: courses, GPA: gpa}, nil
}

// StudentGPA 查询学生 GPA 轨迹。
func (s *Service) StudentGPA(ctx context.Context, studentID int64) ([]SemesterGradeDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.ensureSelfOrAdmin(ctx, id, studentID); err != nil {
		return nil, err
	}
	return s.store.ListStudentSemesterGrades(ctx, id.TenantID, studentID)
}

// CreateAppeal 创建学生成绩申诉。
func (s *Service) CreateAppeal(ctx context.Context, req AppealCreateRequest) (AppealDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return AppealDTO{}, err
	}
	courseID, ok := ids.Parse(req.CourseID)
	if !ok || strings.TrimSpace(req.Reason) == "" {
		return AppealDTO{}, apperr.ErrGradeAppealInvalid
	}
	if _, found, err := s.store.FindOpenAppeal(ctx, id.TenantID, id.AccountID, courseID); err != nil {
		return AppealDTO{}, err
	} else if found {
		return AppealDTO{}, apperr.ErrGradeAppealState
	}
	if err := s.ensureAppealAllowed(ctx, id.TenantID, courseID); err != nil {
		return AppealDTO{}, err
	}
	out, err := s.store.CreateAppeal(ctx, id.TenantID, s.nextID(), req, id.AccountID)
	if err != nil {
		return AppealDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, id.AccountID, "grade.appeal.create", "grade_appeal", ids.ParseOrZero(out.ID), map[string]any{"course_id": out.CourseID})
}

// ensureAppealAllowed 将申诉入口限定在已审核锁定成绩的时效窗口内,避免学生绕过审核流程或长期重复争议历史成绩。
func (s *Service) ensureAppealAllowed(ctx context.Context, tenantID, courseID int64) error {
	review, err := s.store.GetReviewByCourse(ctx, tenantID, courseID)
	if err != nil {
		return err
	}
	if review.Status != ReviewStatusApproved || !review.IsLocked || review.ReviewedAt == nil {
		return apperr.ErrGradeAppealState
	}
	windowDays := s.cfg.AppealWindowDays
	if windowDays <= 0 {
		return apperr.ErrGradeAppealExpired
	}
	if timex.Now().Sub(review.ReviewedAt.UTC()) > time.Duration(windowDays)*24*time.Hour {
		return apperr.ErrGradeAppealExpired
	}
	return nil
}

// AcceptAppeal 受理申诉并解锁 M11 审核状态;后续单课程改分仍由 M6 自有入口完成。
func (s *Service) AcceptAppeal(ctx context.Context, appealID int64, req AppealHandleRequest) (AppealDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return AppealDTO{}, err
	}
	if err := s.ensureTeacherOrAdmin(ctx, id); err != nil {
		return AppealDTO{}, err
	}
	current, err := s.store.GetAppeal(ctx, id.TenantID, appealID)
	if err != nil {
		return AppealDTO{}, err
	}
	if current.Status != AppealStatusPending {
		return AppealDTO{}, apperr.ErrGradeAppealState
	}
	review, err := s.store.GetReviewByCourse(ctx, id.TenantID, ids.ParseOrZero(current.CourseID))
	if err != nil {
		return AppealDTO{}, err
	}
	if review.IsLocked {
		if _, err = s.store.UnlockReview(ctx, id.TenantID, id.AccountID, ids.ParseOrZero(review.ID), "申诉受理解锁"); err != nil {
			return AppealDTO{}, err
		}
	}
	out, err := s.store.UpdateAppealStatus(ctx, id.TenantID, appealID, id.AccountID, AppealStatusAccepted, req.ResultComment)
	if err != nil {
		return AppealDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, id.AccountID, "grade.appeal.accept", "grade_appeal", appealID, map[string]any{"course_id": out.CourseID})
}

// RejectAppeal 驳回申诉。
func (s *Service) RejectAppeal(ctx context.Context, appealID int64, req AppealHandleRequest) (AppealDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return AppealDTO{}, err
	}
	if err := s.ensureTeacherOrAdmin(ctx, id); err != nil {
		return AppealDTO{}, err
	}
	current, err := s.store.GetAppeal(ctx, id.TenantID, appealID)
	if err != nil {
		return AppealDTO{}, err
	}
	if current.Status != AppealStatusPending {
		return AppealDTO{}, apperr.ErrGradeAppealState
	}
	out, err := s.store.UpdateAppealStatus(ctx, id.TenantID, appealID, id.AccountID, AppealStatusRejected, req.ResultComment)
	if err != nil {
		return AppealDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, id.AccountID, "grade.appeal.reject", "grade_appeal", appealID, map[string]any{"course_id": out.CourseID})
}

// ListAppeals 查询申诉列表。
func (s *Service) ListAppeals(ctx context.Context, status int16, page, size int) ([]AppealDTO, int64, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return nil, 0, err
	}
	if err := s.ensureTeacherOrAdmin(ctx, id); err != nil {
		return nil, 0, err
	}
	return s.store.ListAppeals(ctx, id.TenantID, status, page, size)
}

// HandleTeachingGradeUpdated 处理 M6 成绩变更事件,完成已受理申诉闭环。
func (s *Service) HandleTeachingGradeUpdated(ctx context.Context, event contracts.TeachingGradeUpdatedEvent) error {
	if event.TenantID <= 0 || event.CourseID <= 0 || event.StudentID <= 0 {
		return apperr.ErrGradeAggregateFailed
	}
	review, err := s.store.GetReviewByCourse(ctx, event.TenantID, event.CourseID)
	if err != nil {
		return err
	}
	semesterID := ids.ParseOrZero(review.SemesterID)
	if semesterID <= 0 {
		return apperr.ErrGradeAggregateFailed
	}
	if _, err = s.recomputeStudent(ctx, event.TenantID, event.CourseID, event.StudentID, semesterID); err != nil {
		return err
	}
	appeal, found, err := s.store.FindOpenAppeal(ctx, event.TenantID, event.StudentID, event.CourseID)
	if err != nil {
		return err
	}
	if found && appeal.Status == AppealStatusAccepted {
		if _, err = s.store.UpdateAppealStatus(ctx, event.TenantID, ids.ParseOrZero(appeal.ID), 0, AppealStatusCompleted, "成绩已复核完成"); err != nil {
			return err
		}
	}
	_, err = s.store.RelockReviewByCourse(ctx, event.TenantID, event.CourseID)
	return err
}

// ScanWarnings 扫描并生成指定学期的学业预警。
func (s *Service) ScanWarnings(ctx context.Context, req WarningScanRequest) ([]WarningDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !isInternalService(ctx) {
		if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
			return nil, err
		}
	}
	semesterID, ok := ids.Parse(req.SemesterID)
	if !ok {
		return nil, apperr.ErrGradeWarningInvalid
	}
	level, err := s.store.DefaultLevelConfig(ctx, id.TenantID)
	if err != nil {
		return nil, err
	}
	aggregates, err := s.store.ListSemesterGrades(ctx, id.TenantID, semesterID)
	if err != nil {
		return nil, err
	}
	var out []WarningDTO
	for _, agg := range aggregates {
		studentID := ids.ParseOrZero(agg.StudentID)
		if level.WarningRules.MinGPA > 0 && agg.GPA < level.WarningRules.MinGPA {
			warn, err := s.createWarning(ctx, id.TenantID, studentID, semesterID, WarningTypeLowGPA, map[string]any{"gpa": agg.GPA, "min_gpa": level.WarningRules.MinGPA})
			if err != nil {
				return nil, err
			}
			out = append(out, warn)
		}
		failCount, err := s.failCount(ctx, id.TenantID, studentID, level)
		if err != nil {
			return nil, err
		}
		if level.WarningRules.FailCount > 0 && failCount >= level.WarningRules.FailCount {
			warn, err := s.createWarning(ctx, id.TenantID, studentID, semesterID, WarningTypeFailedCourse, map[string]any{"fail_count": failCount})
			if err != nil {
				return nil, err
			}
			out = append(out, warn)
		}
	}
	if err := s.writeWarningScanAudit(ctx, id, semesterID, len(out)); err != nil {
		return nil, err
	}
	return out, nil
}

// ListWarnings 查询学业预警。
func (s *Service) ListWarnings(ctx context.Context, studentID, semesterID int64, status int16, page, size int) ([]WarningDTO, int64, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return nil, 0, err
	}
	if studentID > 0 {
		if err := s.ensureSelfOrAdmin(ctx, id, studentID); err != nil {
			return nil, 0, err
		}
	} else if err := s.ensureTeacherOrAdmin(ctx, id); err != nil {
		studentID = id.AccountID
	}
	return s.store.ListWarnings(ctx, id.TenantID, studentID, semesterID, status, page, size)
}

// AcknowledgeWarning 标记当前学生本人的预警已知悉。
func (s *Service) AcknowledgeWarning(ctx context.Context, warningID int64) (WarningDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return WarningDTO{}, err
	}
	if warningID <= 0 {
		return WarningDTO{}, apperr.ErrGradeWarningInvalid
	}
	return s.store.AcknowledgeWarning(ctx, id.TenantID, id.AccountID, warningID)
}

// GenerateTranscript 生成成绩单元数据并写入对象存储。
func (s *Service) GenerateTranscript(ctx context.Context, req TranscriptRequest) (TranscriptDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return TranscriptDTO{}, err
	}
	return s.generateTranscript(ctx, id, req, true)
}

// GetTranscript 读取成绩单记录并校验本人访问权限。
func (s *Service) GetTranscript(ctx context.Context, transcriptID int64) (TranscriptDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return TranscriptDTO{}, err
	}
	row, err := s.store.GetTranscript(ctx, id.TenantID, transcriptID)
	if err != nil {
		return TranscriptDTO{}, err
	}
	if err := s.ensureSelfOrAdmin(ctx, id, ids.ParseOrZero(row.StudentID)); err != nil {
		return TranscriptDTO{}, err
	}
	return row, nil
}

// DownloadTranscript 读取成绩单记录并打开对象存储中的 PDF 内容。
func (s *Service) DownloadTranscript(ctx context.Context, transcriptID int64) (TranscriptDTO, io.ReadCloser, error) {
	row, err := s.GetTranscript(ctx, transcriptID)
	if err != nil {
		return TranscriptDTO{}, nil, err
	}
	if s.storage == nil || strings.TrimSpace(row.PDFRef) == "" {
		return TranscriptDTO{}, nil, apperr.ErrGradeTranscriptFailed
	}
	reader, err := s.storage.Get(ctx, s.storage.BucketReport(), row.PDFRef)
	if err != nil {
		return TranscriptDTO{}, nil, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	return row, reader, nil
}

// BatchGenerateTranscripts 批量生成成绩单记录。
func (s *Service) BatchGenerateTranscripts(ctx context.Context, req TranscriptBatchRequest) ([]TranscriptDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
		return nil, err
	}
	if len(req.StudentIDs) == 0 {
		return nil, apperr.ErrGradeTranscriptInvalid
	}
	out := make([]TranscriptDTO, 0, len(req.StudentIDs))
	for _, studentID := range req.StudentIDs {
		if _, ok := ids.Parse(studentID); !ok {
			return nil, apperr.ErrGradeTranscriptInvalid
		}
		row, err := s.generateTranscript(ctx, id, TranscriptRequest{StudentID: studentID, Scope: req.Scope, SemesterID: req.SemesterID}, false)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

// generateTranscript 执行成绩单生成,allowSelf 控制是否允许学生生成自己的成绩单。
func (s *Service) generateTranscript(ctx context.Context, id tenant.Identity, req TranscriptRequest, allowSelf bool) (TranscriptDTO, error) {
	studentID, ok := ids.Parse(req.StudentID)
	if !ok || (req.Scope != TranscriptScopeSemester && req.Scope != TranscriptScopeAll) {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptInvalid
	}
	if allowSelf {
		if err := s.ensureSelfOrAdmin(ctx, id, studentID); err != nil {
			return TranscriptDTO{}, err
		}
	} else if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err != nil {
		return TranscriptDTO{}, err
	}
	if s.storage == nil {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed
	}
	transcriptID := s.nextID()
	pdfRef, err := storage.ObjectKey(id.TenantID, "transcript", "record", ids.Format(transcriptID)+".pdf")
	if err != nil {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	content, err := s.renderTranscriptPDF(ctx, id.TenantID, studentID, req)
	if err != nil {
		return TranscriptDTO{}, err
	}
	reader := bytes.NewReader(content)
	if err := s.storage.Put(ctx, s.storage.BucketReport(), pdfRef, reader, int64(len(content)), "application/pdf"); err != nil {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	return s.store.CreateTranscript(ctx, id.TenantID, transcriptID, req, pdfRef)
}

// renderTranscriptPDF 汇总 M6 课程成绩和 M11 GPA 元数据后交给统一 PDF 渲染器,服务层不拼接文件格式细节。
func (s *Service) renderTranscriptPDF(ctx context.Context, tenantID, studentID int64, req TranscriptRequest) ([]byte, error) {
	courses, err := s.studentCourseDTOs(ctx, tenantID, studentID)
	if err != nil {
		return nil, err
	}
	gpa, err := s.store.ListStudentSemesterGrades(ctx, tenantID, studentID)
	if err != nil {
		return nil, err
	}
	content, err := renderTranscriptDocument(transcriptDocument{
		TenantID:   tenantID,
		StudentID:  studentID,
		Scope:      req.Scope,
		SemesterID: req.SemesterID,
		Courses:    courses,
		GPA:        gpa,
		SigningKey: s.cfg.TranscriptSigningKey,
	})
	if err != nil {
		return nil, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	return content, nil
}

// recomputeCourseStudents 重算课程内所有学生 GPA。
func (s *Service) recomputeCourseStudents(ctx context.Context, tenantID, courseID, semesterID int64) error {
	if s.teaching == nil {
		return apperr.ErrGradeAggregateFailed
	}
	grades, err := s.teaching.ListCourseGrades(ctx, tenantID, courseID)
	if err != nil {
		return apperr.ErrGradeAggregateFailed.WithCause(err)
	}
	seen := map[int64]struct{}{}
	for _, grade := range grades {
		if _, ok := seen[grade.StudentID]; ok {
			continue
		}
		seen[grade.StudentID] = struct{}{}
		if _, err := s.recomputeStudent(ctx, tenantID, courseID, grade.StudentID, semesterID); err != nil {
			return err
		}
	}
	return nil
}

// recomputeStudent 读取 M6 单课程成绩并写入 M11 学期 GPA 聚合。
func (s *Service) recomputeStudent(ctx context.Context, tenantID, courseID, studentID, semesterID int64) (SemesterGradeDTO, error) {
	if s.teaching == nil {
		return SemesterGradeDTO{}, apperr.ErrGradeAggregateFailed
	}
	level, err := s.store.DefaultLevelConfig(ctx, tenantID)
	if err != nil {
		return SemesterGradeDTO{}, err
	}
	semesterCourses, cumulativeCourses, err := s.approvedCourseSets(ctx, tenantID, studentID, semesterID)
	if err != nil {
		return SemesterGradeDTO{}, err
	}
	if _, ok := semesterCourses[courseID]; !ok {
		return SemesterGradeDTO{}, apperr.ErrGradeAggregateInvalid
	}
	grades, err := s.teaching.ListStudentGrades(ctx, tenantID, studentID)
	if err != nil {
		return SemesterGradeDTO{}, apperr.ErrGradeAggregateFailed.WithCause(err)
	}
	totalCredits, weighted := 0.0, 0.0
	cumulativeCredits, cumulativeWeighted := 0.0, 0.0
	for _, grade := range grades {
		_, gpa := mapScore(level.Mapping, grade.FinalTotal)
		if _, ok := semesterCourses[grade.CourseID]; ok {
			totalCredits += grade.Credits
			weighted += gpa * grade.Credits
		}
		if _, ok := cumulativeCourses[grade.CourseID]; ok {
			cumulativeCredits += grade.Credits
			cumulativeWeighted += gpa * grade.Credits
		}
	}
	gpa := 0.0
	if totalCredits > 0 {
		gpa = round3(weighted / totalCredits)
	}
	cumulativeGPA := 0.0
	if cumulativeCredits > 0 {
		cumulativeGPA = round3(cumulativeWeighted / cumulativeCredits)
	}
	return s.store.UpsertSemesterGrade(ctx, tenantID, SemesterGradeUpsert{
		ID: s.nextID(), StudentID: studentID, SemesterID: semesterID, TotalCredits: totalCredits, GPA: gpa, CumulativeGPA: cumulativeGPA,
	})
}

// approvedCourseSets 返回本学期与累计 GPA 可计入的课程集合,申诉受理中的临时解锁课程允许重算后再重锁。
func (s *Service) approvedCourseSets(ctx context.Context, tenantID, studentID, semesterID int64) (map[int64]struct{}, map[int64]struct{}, error) {
	semesterCourses := make(map[int64]struct{})
	cumulativeCourses := make(map[int64]struct{})
	page, size := pagex.Normalize(1, 100)
	for {
		reviews, total, err := s.store.ListReviews(ctx, tenantID, 0, page, size)
		if err != nil {
			return nil, nil, err
		}
		for _, review := range reviews {
			reviewCourseID := ids.ParseOrZero(review.CourseID)
			reviewSemesterID := ids.ParseOrZero(review.SemesterID)
			if reviewCourseID <= 0 || reviewSemesterID <= 0 {
				continue
			}
			if !isApprovedLockedReview(review) {
				appeal, found, err := s.store.FindOpenAppeal(ctx, tenantID, studentID, reviewCourseID)
				if err != nil {
					return nil, nil, err
				}
				if !found || appeal.Status != AppealStatusAccepted {
					continue
				}
			}
			cumulativeCourses[reviewCourseID] = struct{}{}
			if reviewSemesterID == semesterID {
				semesterCourses[reviewCourseID] = struct{}{}
			}
		}
		if len(reviews) < size || int64(page*size) >= total {
			return semesterCourses, cumulativeCourses, nil
		}
		page++
	}
}

// isApprovedLockedReview 判断课程成绩是否已进入正式 GPA 口径。
func isApprovedLockedReview(review ReviewDTO) bool {
	return review.Status == ReviewStatusApproved && review.IsLocked
}

// createWarning 写入预警并经 M10 发送通知。
func (s *Service) createWarning(ctx context.Context, tenantID, studentID, semesterID int64, typ int16, detail map[string]any) (WarningDTO, error) {
	warn, err := s.store.CreateWarning(ctx, tenantID, s.nextID(), WarningCreate{StudentID: studentID, SemesterID: semesterID, Type: typ, Detail: detail})
	if err != nil {
		return WarningDTO{}, err
	}
	if s.notify == nil {
		if rollbackErr := s.store.DeleteWarning(ctx, tenantID, ids.ParseOrZero(warn.ID)); rollbackErr != nil {
			return WarningDTO{}, apperr.ErrGradeWarningFailed.WithCause(rollbackErr)
		}
		return WarningDTO{}, apperr.ErrGradeWarningFailed
	}
	if err := s.notify.Send(ctx, contracts.NotifySendRequest{
		TenantID: tenantID, Type: "grade.warning", Receivers: []int64{studentID},
		Params: map[string]string{"type": strconvInt16(typ)}, Link: "/grade-center/warnings",
	}); err != nil {
		if rollbackErr := s.store.DeleteWarning(ctx, tenantID, ids.ParseOrZero(warn.ID)); rollbackErr != nil {
			return WarningDTO{}, apperr.ErrGradeWarningFailed.WithCause(rollbackErr)
		}
		return WarningDTO{}, apperr.ErrGradeWarningFailed.WithCause(err)
	}
	return warn, nil
}

// failCount 统计学生在 M6 成绩中的挂科数量。
func (s *Service) failCount(ctx context.Context, tenantID, studentID int64, level LevelConfigDTO) (int, error) {
	if s.teaching == nil {
		return 0, apperr.ErrGradeAggregateFailed
	}
	var count int
	grades, err := s.teaching.ListStudentGrades(ctx, tenantID, studentID)
	if err != nil {
		return 0, apperr.ErrGradeAggregateFailed.WithCause(err)
	}
	for _, grade := range grades {
		mapped, _ := mapScore(level.Mapping, grade.FinalTotal)
		if mapped == "F" {
			count++
		}
	}
	return count, nil
}

// studentCourseDTOs 读取并映射学生跨课程成绩明细。
func (s *Service) studentCourseDTOs(ctx context.Context, tenantID, studentID int64) ([]CourseGradeDTO, error) {
	if s.teaching == nil {
		return nil, apperr.ErrGradeAggregateFailed
	}
	level, err := s.store.DefaultLevelConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	grades, err := s.teaching.ListStudentGrades(ctx, tenantID, studentID)
	if err != nil {
		return nil, apperr.ErrGradeAggregateFailed.WithCause(err)
	}
	out := make([]CourseGradeDTO, 0, len(grades))
	for _, grade := range grades {
		letter, gpa := mapScore(level.Mapping, grade.FinalTotal)
		out = append(out, CourseGradeDTO{
			CourseID: ids.Format(grade.CourseID), StudentID: ids.Format(grade.StudentID), FinalTotal: grade.FinalTotal,
			Credits: grade.Credits, Grade: letter, GPA: gpa,
		})
	}
	return out, nil
}

// validateLevelConfig 校验等级映射和预警规则。
func validateLevelConfig(req LevelConfigRequest) error {
	if strings.TrimSpace(req.Name) == "" || len(req.Mapping) == 0 {
		return apperr.ErrGradeConfigInvalid
	}
	for _, item := range req.Mapping {
		if strings.TrimSpace(item.Grade) == "" || item.Min < 0 || item.Min > 100 || item.GPA < 0 {
			return apperr.ErrGradeConfigInvalid
		}
	}
	return nil
}

// mapScore 按配置将百分制成绩映射为等级和绩点。
func mapScore(mapping []LevelMappingDTO, score float64) (string, float64) {
	items := append([]LevelMappingDTO(nil), mapping...)
	sort.Slice(items, func(i, j int) bool { return items[i].Min > items[j].Min })
	for _, item := range items {
		if score >= item.Min {
			return item.Grade, item.GPA
		}
	}
	return "", 0
}

// round3 把 GPA 保留三位小数。
func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

// currentTenant 读取当前租户身份。
func currentTenant(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	if id.IsPlatform || id.TenantID <= 0 {
		return tenant.Identity{}, apperr.ErrForbidden
	}
	return id, nil
}

// isInternalService 判断请求是否已通过服务间 HMAC 鉴权,仅供文档标注的内部入口放行。
func isInternalService(ctx context.Context) bool {
	_, ok := auth.ServiceSourceRefFromContext(ctx)
	return ok
}

// ensureSelfOrAdmin 限制学生只能读取自己的成绩;跨学生读取仅限学校管理员。
func (s *Service) ensureSelfOrAdmin(ctx context.Context, id tenant.Identity, studentID int64) error {
	if id.AccountID == studentID {
		return nil
	}
	if s.identity == nil {
		return apperr.ErrForbidden
	}
	admin, err := s.identity.HasRole(ctx, id.AccountID, contracts.RoleSchoolAdmin)
	if err != nil {
		return apperr.ErrForbidden.WithCause(err)
	}
	if admin {
		return nil
	}
	return apperr.ErrForbidden
}

// ensureTeacherOrAdmin 要求当前账号具备教师或学校管理员角色。
func (s *Service) ensureTeacherOrAdmin(ctx context.Context, id tenant.Identity) error {
	if err := s.ensureRole(ctx, id, contracts.RoleSchoolAdmin); err == nil {
		return nil
	}
	return s.ensureRole(ctx, id, contracts.RoleTeacher)
}

// ensureRole 经 M1 身份契约校验服务端角色。
func (s *Service) ensureRole(ctx context.Context, id tenant.Identity, role string) error {
	if s.identity == nil {
		return apperr.ErrForbidden
	}
	has, err := s.identity.HasRole(ctx, id.AccountID, role)
	if err != nil {
		return apperr.ErrForbidden.WithCause(err)
	}
	if !has {
		return apperr.ErrForbidden
	}
	return nil
}

// nextID 生成 M11 自有表主键。
func (s *Service) nextID() int64 {
	return s.idgen.Generate()
}

// writeAudit 通过平台审计 Writer 追加 M11 高敏感操作审计。
func (s *Service) writeAudit(ctx context.Context, tenantID, actorID int64, action, targetType string, targetID int64, detail map[string]any) error {
	if s.auditor == nil {
		return apperr.ErrGradeAuditWriteFailed
	}
	resolvedActorID, actorRole, err := audit.ResolveActor(ctx, s.identity)
	if err != nil {
		return apperr.ErrGradeAuditWriteFailed.WithCause(err)
	}
	if actorID != 0 && actorID != resolvedActorID {
		return apperr.ErrGradeAuditWriteFailed
	}
	entry, err := audit.BuildEntry(ctx, tenantID, resolvedActorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrGradeAuditWriteFailed.WithCause(err)
	}
	if err := s.auditor.Write(ctx, entry); err != nil {
		return apperr.ErrGradeAuditWriteFailed.WithCause(err)
	}
	return nil
}

// writeWarningScanAudit 记录预警扫描审计;内部服务入口没有用户 actor,使用系统角色留痕。
func (s *Service) writeWarningScanAudit(ctx context.Context, id tenant.Identity, semesterID int64, count int) error {
	detail := map[string]any{"semester_id": ids.Format(semesterID), "warning_count": count}
	if sourceRef, ok := auth.ServiceSourceRefFromContext(ctx); ok {
		detail["source_ref"] = sourceRef
		return s.writeSystemAudit(ctx, id.TenantID, "grade.warning.scan", "academic_warning", semesterID, detail)
	}
	return s.writeAudit(ctx, id.TenantID, id.AccountID, "grade.warning.scan", "academic_warning", semesterID, detail)
}

// writeSystemAudit 通过平台审计 Writer 追加内部系统任务审计。
func (s *Service) writeSystemAudit(ctx context.Context, tenantID int64, action, targetType string, targetID int64, detail map[string]any) error {
	if s.auditor == nil {
		return apperr.ErrGradeAuditWriteFailed
	}
	entry, err := audit.BuildEntry(ctx, tenantID, 0, audit.ActorRoleSystem, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrGradeAuditWriteFailed.WithCause(err)
	}
	if err := s.auditor.Write(ctx, entry); err != nil {
		return apperr.ErrGradeAuditWriteFailed.WithCause(err)
	}
	return nil
}

// strconvInt16 把预警类型转为通知模板参数。
func strconvInt16(v int16) string {
	if v == WarningTypeFailedCourse {
		return "挂科预警"
	}
	return "低 GPA 预警"
}
