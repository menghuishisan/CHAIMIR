// grade repo 文件定义 M11 持久化边界,只操作成绩中心自有表。
package grade

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

// roleReader 定义 M11 service 层复用的角色查询契约。
type roleReader interface {
	// HasRole 判断账号是否具备指定角色。
	HasRole(ctx context.Context, accountID int64, role string) (bool, error)
}

// Store 定义 M11 service 所需事务入口。
type Store interface {
	// TenantTx 在租户 RLS 事务中访问 M11 自有表。
	TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error
	// PrivilegedTx 在受控后台任务中跨租户领取 M11 自有 outbox。
	PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error
}

// TxStore 定义 M11 单事务数据访问能力。
type TxStore interface {
	CreateLevelConfig(context.Context, int64, int64, LevelConfigRequest) (LevelConfigDTO, error)
	ListLevelConfigs(context.Context) ([]LevelConfigDTO, error)
	GetDefaultLevelConfig(context.Context) (LevelConfigDTO, error)
	UpdateLevelConfig(context.Context, int64, LevelConfigRequest) (LevelConfigDTO, error)
	CreateSemester(context.Context, int64, int64, SemesterRequest) (SemesterDTO, error)
	ListSemesters(context.Context) ([]SemesterDTO, error)
	GetCurrentSemester(context.Context) (SemesterDTO, error)
	CreateGradeReview(context.Context, int64, int64, int64, ReviewRequest) (ReviewDTO, error)
	ListGradeReviews(context.Context, int16, int, int) ([]ReviewDTO, int64, error)
	GetGradeReview(context.Context, int64) (ReviewDTO, error)
	GetLatestApprovedReviewByCourse(context.Context, int64) (ReviewDTO, error)
	GetLatestReviewByCourse(context.Context, int64) (ReviewDTO, error)
	ApproveGradeReview(context.Context, int64, int64, int64, string) (ReviewDTO, error)
	RejectGradeReview(context.Context, int64, int64, string) (ReviewDTO, error)
	UnlockGradeReview(context.Context, int64, int64, string) (ReviewDTO, error)
	RelockGradeReview(context.Context, int64, int64, string) (ReviewDTO, error)
	CreateGradeLockOutbox(context.Context, int64, ReviewDTO, bool, string, string) (GradeLockOutbox, error)
	ClaimPendingGradeLockOutbox(context.Context, int32, time.Time) ([]GradeLockOutbox, error)
	MarkGradeLockOutboxPublished(context.Context, int64, int64) (GradeLockOutbox, error)
	MarkGradeLockOutboxFailed(context.Context, int64, int64, string) (GradeLockOutbox, error)
	UpsertStudentSemesterGrade(context.Context, int64, int64, int64, int64, float64, float64, float64) (GradeSummaryDTO, error)
	ListStudentSemesterGrades(context.Context, int64) ([]GradeSummaryDTO, error)
	ListKnownStudentSemesterGrades(context.Context, int64) ([]GradeSummaryDTO, error)
	CreateGradeAppeal(context.Context, int64, int64, int64, AppealRequest) (AppealDTO, error)
	HasOpenGradeAppeal(context.Context, int64, int64) (bool, error)
	GetGradeAppeal(context.Context, int64) (AppealDTO, error)
	ListGradeAppeals(context.Context, int16, int, int) ([]AppealDTO, int64, error)
	ListAcceptedAppealsByCourseStudent(context.Context, int64, int64) ([]AppealDTO, error)
	UpdateGradeAppealStatus(context.Context, int64, int16, int16, int64, string) (AppealDTO, error)
	CreateAcademicWarning(context.Context, int64, int64, int64, int64, int16, map[string]any) (WarningDTO, error)
	ListAcademicWarnings(context.Context, int64, int, int) ([]WarningDTO, int64, error)
	AckAcademicWarning(context.Context, int64, int64) (WarningDTO, error)
	CreateTranscriptRecord(context.Context, int64, int64, TranscriptRequest, string) (TranscriptDTO, error)
	GetTranscriptRecord(context.Context, int64) (TranscriptDTO, error)
	ListTranscriptRecords(context.Context, int64, int, int) ([]TranscriptDTO, error)
}

type store struct{ database *db.DB }
type txStore struct{ q *sqlcgen.Queries }

// NewStore 创建 M11 持久化入口。
func NewStore(database *db.DB) Store { return &store{database: database} }

// TenantTx 在租户事务中执行 M11 表访问。
func (s *store) TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("grade store 未初始化")
	}
	return s.database.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// PrivilegedTx 在 M11 模块自有表内执行后台跨租户扫描,不得用于普通业务路径。
func (s *store) PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("grade store 未初始化")
	}
	return s.database.WithPrivilegedModuleTx(ctx, "grade", func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// isNoRows 统一识别未命中错误。
func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
