// teaching repo 文件定义 M6 持久化接口和数据库事务边界,只操作教学模块自有表。
package teaching

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

// Store 定义 service 所需的 teaching 持久化事务入口。
type Store interface {
	// TenantTx 在注入 RLS 租户变量后访问 M6 租户表。
	TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error
	// PrivilegedTx 在受控模块事务中读取共享课程并写入目标租户克隆。
	PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error
}

// TxStore 定义单个事务内可调用的数据访问能力,不暴露 sqlc 行类型。
type TxStore interface {
	CreateCourse(context.Context, Course) (Course, error)
	GetCourse(context.Context, int64, int64) (Course, error)
	GetCloneableCourse(context.Context, int64, int64) (Course, error)
	GetCourseByInviteCode(context.Context, string) (Course, error)
	ListTeacherCourses(context.Context, int64, int64, CourseListFilter) ([]Course, int64, error)
	ListStudentCourses(context.Context, int64, int64, CourseListFilter) ([]Course, int64, error)
	UpdateCourse(context.Context, Course) (Course, error)
	SetCourseStatus(context.Context, int64, int64, int16) (Course, error)
	SetCourseVisibility(context.Context, int64, int64, int16) (Course, error)
	RefreshCourseInviteCode(context.Context, int64, int64, string) (Course, error)
	CountCourseLessons(context.Context, int64, int64) (int64, error)
	ListCoursesDueToRun(context.Context, time.Time) ([]Course, error)
	ListCoursesDueToEnd(context.Context, time.Time) ([]Course, error)
	CreateChapter(context.Context, Chapter) (Chapter, error)
	GetChapter(context.Context, int64, int64) (Chapter, error)
	ListChapters(context.Context, int64, int64) ([]Chapter, error)
	UpdateChapter(context.Context, Chapter) (Chapter, error)
	DeleteChapter(context.Context, int64, int64) (Chapter, error)
	CreateLesson(context.Context, Lesson) (Lesson, error)
	GetLesson(context.Context, int64, int64) (Lesson, error)
	ListLessonsByChapter(context.Context, int64, int64) ([]Lesson, error)
	ListLessonsByCourse(context.Context, int64, int64) ([]Lesson, error)
	UpdateLesson(context.Context, Lesson) (Lesson, error)
	SetLessonContent(context.Context, int64, int64, int16, map[string]any) (Lesson, error)
	DeleteLesson(context.Context, int64, int64) (Lesson, error)
	CreateCourseMember(context.Context, CourseMember) (CourseMember, error)
	GetCourseMember(context.Context, int64, int64, int64) (CourseMember, error)
	ListCourseMembers(context.Context, int64, int64, int, int) ([]CourseMember, int64, error)
	DeleteCourseMember(context.Context, int64, int64, int64) error
	CreateAssignment(context.Context, Assignment) (Assignment, error)
	GetAssignment(context.Context, int64, int64) (Assignment, error)
	ListAssignmentItems(context.Context, int64, int64) ([]AssignmentItem, error)
	ListAssignmentsByCourse(context.Context, int64, int64) ([]Assignment, error)
	UpdateAssignment(context.Context, Assignment) (Assignment, error)
	ReplaceAssignmentItems(context.Context, int64, int64, []AssignmentItem) ([]AssignmentItem, error)
	PublishAssignment(context.Context, int64, int64) (Assignment, error)
	CountAssignmentSubmissions(context.Context, int64, int64) (int64, error)
	CountStudentAttempts(context.Context, int64, int64, int64) (int64, error)
	CreateSubmission(context.Context, Submission) (Submission, error)
	GetSubmission(context.Context, int64, int64) (Submission, error)
	GetSubmissionBySourceRef(context.Context, int64, string) (Submission, error)
	ListJudgeOutboxBySubmission(context.Context, int64, int64) ([]JudgeOutbox, error)
	ListSubmissionsByAssignment(context.Context, int64, int64, int, int) ([]Submission, int64, error)
	UpdateSubmissionManualGrade(context.Context, int64, int64, int32, int32, string) (Submission, error)
	UpdateSubmissionJudgeRef(context.Context, int64, int64, string) (Submission, error)
	UpdateSubmissionAutoScore(context.Context, int64, int64, int32, int32) (Submission, error)
	CreateJudgeOutbox(context.Context, JudgeOutbox) (JudgeOutbox, error)
	ClaimJudgeOutbox(context.Context, int64, int32) ([]JudgeOutbox, error)
	ClaimJudgeOutboxAcrossTenants(context.Context, int32) ([]JudgeOutbox, error)
	CompleteJudgeOutbox(context.Context, int64, int64) (JudgeOutbox, error)
	RetryJudgeOutbox(context.Context, int64, int64, string) (JudgeOutbox, error)
	MarkJudgeOutboxResult(context.Context, int64, string, int32, time.Time) (JudgeOutbox, error)
	MarkJudgeOutboxFailedResult(context.Context, int64, string, string, time.Time) (JudgeOutbox, error)
	UpsertDraft(context.Context, SubmissionDraft) (SubmissionDraft, error)
	GetDraft(context.Context, int64, int64, int64) (SubmissionDraft, error)
	DeleteDraft(context.Context, int64, int64, int64) error
	UpsertProgress(context.Context, LessonProgress) (LessonProgress, error)
	GetProgress(context.Context, int64, int64, int64) (LessonProgress, error)
	ListProgressByCourse(context.Context, int64, int64) ([]LessonProgress, error)
	ListStudentProgressByCourse(context.Context, int64, int64, int64) ([]LessonProgress, error)
	CreatePost(context.Context, DiscussionPost) (DiscussionPost, error)
	GetPost(context.Context, int64, int64) (DiscussionPost, error)
	ListPosts(context.Context, int64, int64, int, int) ([]DiscussionPost, error)
	LikePost(context.Context, int64, int64) (DiscussionPost, error)
	PinPost(context.Context, int64, int64, bool) (DiscussionPost, error)
	DeletePost(context.Context, int64, int64) (DiscussionPost, error)
	CreateAnnouncement(context.Context, Announcement) (Announcement, error)
	ListAnnouncements(context.Context, int64, int64) ([]Announcement, error)
	PinAnnouncement(context.Context, int64, int64, bool) (Announcement, error)
	UpsertReview(context.Context, CourseReview) (CourseReview, error)
	ReplaceGradeWeights(context.Context, int64, int64, []GradeWeight) ([]GradeWeight, error)
	ListGradeWeights(context.Context, int64, int64) ([]GradeWeight, error)
	UpsertCourseGrade(context.Context, CourseGrade) (CourseGrade, error)
	GetCourseGrade(context.Context, int64, int64, int64) (CourseGrade, error)
	ListCourseGrades(context.Context, int64, int64, int32, int32) ([]CourseGrade, error)
	ListStudentGrades(context.Context, int64, int64) ([]CourseGrade, error)
	OverrideCourseGrade(context.Context, int64, int64, int64, float64) (CourseGrade, error)
	SetCourseGradesLock(context.Context, int64, int64, bool) error
	CreateTeachingGradeEventOutbox(context.Context, int64, int64, int64, int64, string, time.Time) (TeachingGradeEventOutbox, error)
	ClaimPendingTeachingGradeEventOutbox(context.Context, int32, time.Time) ([]TeachingGradeEventOutbox, error)
	MarkTeachingGradeEventOutboxPublished(context.Context, int64, int64) (TeachingGradeEventOutbox, error)
	MarkTeachingGradeEventOutboxFailed(context.Context, int64, int64, string) (TeachingGradeEventOutbox, error)
	Stats(context.Context, int64) (contractsStats, error)
}

// contractsStats 是 repo 返回给 service 的租户级教学统计投影。
type contractsStats struct {
	CourseCount         int64
	ActiveCourseCount   int64
	LearningDurationSec int64
}

type store struct{ database *db.DB }
type txStore struct{ q *sqlcgen.Queries }

// NewStore 创建 teaching 模块持久化入口,仅装配层应调用。
func NewStore(database *db.DB) Store { return &store{database: database} }

// TenantTx 在当前租户事务中执行 M6 自有表读写。
func (s *store) TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("teaching store 未初始化")
	}
	return s.database.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// PrivilegedTx 在 teaching 模块自有表内执行受控跨租户克隆事务。
func (s *store) PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("teaching store 未初始化")
	}
	return s.database.WithPrivilegedModuleTx(ctx, "teaching", func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// isNoRows 统一识别未命中错误,让 service 不直接依赖 pgx。
func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
