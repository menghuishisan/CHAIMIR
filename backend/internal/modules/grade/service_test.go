// M11 服务测试:覆盖 GPA 聚合、审核锁定、申诉状态机和 M6 成绩事件重算。
package grade

import (
	"context"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestRecomputeStudentGPAUsesCreditWeightedGradePoints 确认 GPA 按学分加权且只读 M6 成绩。
func TestRecomputeStudentGPAUsesCreditWeightedGradePoints(t *testing.T) {
	store := newFakeGradeStore()
	store.review = ReviewDTO{ID: "8101", TenantID: "1001", CourseID: "3001", SemesterID: "202601", Status: ReviewStatusApproved, IsLocked: true}
	store.level = LevelConfigDTO{
		ID:        "9001",
		TenantID:  "1001",
		Name:      "default",
		IsDefault: true,
		Mapping: []LevelMappingDTO{
			{Min: 90, Grade: "A", GPA: 4.0},
			{Min: 80, Grade: "B", GPA: 3.0},
			{Min: 60, Grade: "D", GPA: 1.0},
			{Min: 0, Grade: "F", GPA: 0},
		},
		WarningRules: WarningRuleDTO{FailCount: 1, MinGPA: 2.0},
	}
	svc := newTestGradeService(store)
	svc.teaching = &fakeGradeTeaching{grades: []contracts.TeachingCourseGrade{
		{TenantID: 1001, CourseID: 3001, StudentID: 5001, FinalTotal: 95, Credits: 2},
		{TenantID: 1001, CourseID: 3001, StudentID: 5002, FinalTotal: 50, Credits: 2},
	}}
	svc.identity = &fakeGradeIdentity{roles: []string{"school_admin"}}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 7001})

	out, err := svc.RecomputeStudent(ctx, 5001, RecomputeRequest{CourseID: "3001", SemesterID: "202601"})
	if err != nil {
		t.Fatalf("RecomputeStudent returned error: %v", err)
	}
	if out.GPA != 4.0 || out.CumulativeGPA != 4.0 || out.TotalCredits != 2 {
		t.Fatalf("unexpected GPA aggregate: %#v", out)
	}
	if store.semesterGrade.StudentID != "5001" || store.semesterGrade.SemesterID != "202601" {
		t.Fatalf("semester grade was not persisted for student: %#v", store.semesterGrade)
	}
	if store.writeCount != 1 {
		t.Fatalf("expected one aggregate write, got %d", store.writeCount)
	}
}

// TestRecomputeStudentGPAUsesOnlyReviewedSemesterCourses 确认本学期 GPA 不混入其他学期或待审课程成绩。
func TestRecomputeStudentGPAUsesOnlyReviewedSemesterCourses(t *testing.T) {
	store := newFakeGradeStore()
	store.reviews = []ReviewDTO{
		{ID: "8101", TenantID: "1001", CourseID: "3001", SemesterID: "202601", Status: ReviewStatusApproved, IsLocked: true},
		{ID: "8102", TenantID: "1001", CourseID: "3002", SemesterID: "202602", Status: ReviewStatusApproved, IsLocked: true},
		{ID: "8103", TenantID: "1001", CourseID: "3003", SemesterID: "202601", Status: ReviewStatusPending, IsLocked: false},
	}
	store.level = defaultLevelConfig()
	svc := newTestGradeService(store)
	svc.teaching = &fakeGradeTeaching{grades: []contracts.TeachingCourseGrade{
		{TenantID: 1001, CourseID: 3001, StudentID: 5001, FinalTotal: 95, Credits: 2},
		{TenantID: 1001, CourseID: 3002, StudentID: 5001, FinalTotal: 60, Credits: 2},
		{TenantID: 1001, CourseID: 3003, StudentID: 5001, FinalTotal: 80, Credits: 2},
	}}
	svc.identity = &fakeGradeIdentity{roles: []string{contracts.RoleSchoolAdmin}}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 7001})

	out, err := svc.RecomputeStudent(ctx, 5001, RecomputeRequest{CourseID: "3001", SemesterID: "202601"})
	if err != nil {
		t.Fatalf("RecomputeStudent returned error: %v", err)
	}
	if out.TotalCredits != 2 || out.GPA != 4.0 {
		t.Fatalf("expected semester GPA to use only course 3001, got %#v", out)
	}
}

// TestGradeBoundariesUsePlatformContracts 守护 M11 角色、时间和 no rows 边界统一走平台契约。
func TestGradeBoundariesUsePlatformContracts(t *testing.T) {
	apiSrc, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api: %v", err)
	}
	serviceSrc, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	repoSrc, err := os.ReadFile("repo.go")
	if err != nil {
		t.Fatalf("read repo: %v", err)
	}
	apiText := string(apiSrc)
	serviceText := string(serviceSrc)
	repoText := string(repoSrc)
	for _, text := range []string{apiText, serviceText} {
		if strings.Contains(text, `"school_admin"`) || strings.Contains(text, `"teacher"`) {
			t.Fatalf("M11 role checks must use contracts role constants")
		}
	}
	for _, required := range []string{"contracts.RoleSchoolAdmin", "contracts.RoleTeacher"} {
		if !strings.Contains(apiText, required) || !strings.Contains(serviceText, required) {
			t.Fatalf("M11 role boundary missing %s", required)
		}
	}
	if strings.Contains(serviceText, "time.Since(") || !strings.Contains(serviceText, "timex.Now()") {
		t.Fatalf("M11 appeal time window must use platform/timex")
	}
	if strings.Contains(repoText, "pgx.ErrNoRows") || strings.Contains(repoText, "errors.Is(") {
		t.Fatalf("M11 repo must use platform db.IsNoRows instead of direct pgx no rows checks")
	}
}

// TestApproveReviewLocksAndRecomputesAllCourseStudents 确认审核通过会锁定课程成绩并触发全班 GPA 聚合。
func TestApproveReviewLocksAndRecomputesAllCourseStudents(t *testing.T) {
	store := newFakeGradeStore()
	store.review = ReviewDTO{ID: "8101", TenantID: "1001", CourseID: "3001", SubmitterID: "7001", Status: ReviewStatusPending}
	store.level = defaultLevelConfig()
	svc := newTestGradeService(store)
	svc.teaching = &fakeGradeTeaching{grades: []contracts.TeachingCourseGrade{
		{TenantID: 1001, CourseID: 3001, StudentID: 5001, FinalTotal: 92, Credits: 3},
		{TenantID: 1001, CourseID: 3001, StudentID: 5002, FinalTotal: 81, Credits: 2},
	}}
	svc.identity = &fakeGradeIdentity{roles: []string{"school_admin"}}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 9001})

	out, err := svc.ApproveReview(ctx, 8101, ReviewDecisionRequest{Comment: "通过", SemesterID: "202601"})
	if err != nil {
		t.Fatalf("ApproveReview returned error: %v", err)
	}
	if out.Status != ReviewStatusApproved || !out.IsLocked {
		t.Fatalf("expected approved and locked review, got %#v", out)
	}
	if len(store.semesterGrades) != 2 {
		t.Fatalf("expected GPA recompute for two students, got %d", len(store.semesterGrades))
	}
}

// TestAcceptAppealDoesNotWriteTeachingAndUnlocksReview 确认 M11 受理申诉只更新自有状态,不触碰 M6 成绩。
func TestAcceptAppealDoesNotWriteTeachingAndUnlocksReview(t *testing.T) {
	store := newFakeGradeStore()
	store.appeal = AppealDTO{ID: "8201", TenantID: "1001", StudentID: "5001", CourseID: "3001", Status: AppealStatusPending}
	store.review = ReviewDTO{ID: "8101", TenantID: "1001", CourseID: "3001", Status: ReviewStatusApproved, IsLocked: true}
	teaching := &fakeGradeTeaching{}
	svc := newTestGradeService(store)
	svc.teaching = teaching
	svc.identity = &fakeGradeIdentity{roles: []string{"teacher"}}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 9001})

	out, err := svc.AcceptAppeal(ctx, 8201, AppealHandleRequest{ResultComment: "请任课教师复核"})
	if err != nil {
		t.Fatalf("AcceptAppeal returned error: %v", err)
	}
	if out.Status != AppealStatusAccepted || store.review.IsLocked {
		t.Fatalf("expected accepted appeal and unlocked review, appeal=%#v review=%#v", out, store.review)
	}
	if teaching.updateCalls != 0 {
		t.Fatalf("M11 must not call M6 write interface, calls=%d", teaching.updateCalls)
	}
}

// TestAcceptAppealFailsWhenReviewLookupFails 确认申诉受理前必须确认审核锁定状态,不能在状态未知时继续提交半成功变更。
func TestAcceptAppealFailsWhenReviewLookupFails(t *testing.T) {
	store := newFakeGradeStore()
	store.appeal = AppealDTO{ID: "8201", TenantID: "1001", StudentID: "5001", CourseID: "3001", Status: AppealStatusPending}
	store.reviewErr = errors.New("review lookup failed")
	svc := newTestGradeService(store)
	svc.identity = &fakeGradeIdentity{roles: []string{"teacher"}}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 9001})

	_, err := svc.AcceptAppeal(ctx, 8201, AppealHandleRequest{ResultComment: "请任课教师复核"})
	if err == nil {
		t.Fatalf("expected review lookup failure")
	}
	if store.appeal.Status != AppealStatusPending {
		t.Fatalf("appeal must stay pending when review lookup fails, got %#v", store.appeal)
	}
}

// TestGradeUpdatedEventCompletesAcceptedAppealAndRelocksReview 确认 M6 改分事件驱动 M11 重算并完成申诉。
func TestGradeUpdatedEventCompletesAcceptedAppealAndRelocksReview(t *testing.T) {
	store := newFakeGradeStore()
	store.appeal = AppealDTO{ID: "8201", TenantID: "1001", StudentID: "5001", CourseID: "3001", Status: AppealStatusAccepted}
	store.review = ReviewDTO{ID: "8101", TenantID: "1001", CourseID: "3001", SemesterID: "202601", Status: ReviewStatusPending, IsLocked: false}
	store.level = defaultLevelConfig()
	svc := newTestGradeService(store)
	svc.teaching = &fakeGradeTeaching{grades: []contracts.TeachingCourseGrade{
		{TenantID: 1001, CourseID: 3001, StudentID: 5001, FinalTotal: 88, Credits: 3},
	}}

	err := svc.HandleTeachingGradeUpdated(context.Background(), contracts.TeachingGradeUpdatedEvent{
		TenantID: 1001, CourseID: 3001, StudentID: 5001, UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("HandleTeachingGradeUpdated returned error: %v", err)
	}
	if store.appeal.Status != AppealStatusCompleted || !store.review.IsLocked {
		t.Fatalf("expected completed appeal and relocked review, appeal=%#v review=%#v", store.appeal, store.review)
	}
	if store.semesterGrade.GPA != 3.0 {
		t.Fatalf("expected recomputed GPA 3.0, got %#v", store.semesterGrade)
	}
}

// TestSubscribeEventsRequiresConfiguredBus 确认成绩聚合事件入口缺少总线时显式失败,避免 GPA 重算链路静默停摆。
func TestSubscribeEventsRequiresConfiguredBus(t *testing.T) {
	svc := newTestGradeService(newFakeGradeStore())

	err := svc.SubscribeEvents(nil)
	if err == nil {
		t.Fatalf("expected missing event bus to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrGradeAggregateFailed.Code {
		t.Fatalf("expected grade aggregate failed, got %v", err)
	}
}

// TestScanWarningsCreatesLowGPAAndFailWarnings 确认预警扫描按配置生成低 GPA 和挂科预警并经通知发送。
func TestScanWarningsCreatesLowGPAAndFailWarnings(t *testing.T) {
	store := newFakeGradeStore()
	store.level = defaultLevelConfig()
	store.semesterGrades = []SemesterGradeDTO{{StudentID: "5001", SemesterID: "202601", GPA: 1.5, TotalCredits: 5}}
	svc := newTestGradeService(store)
	svc.teaching = &fakeGradeTeaching{grades: []contracts.TeachingCourseGrade{
		{TenantID: 1001, CourseID: 3001, StudentID: 5001, FinalTotal: 55, Credits: 2},
		{TenantID: 1001, CourseID: 3002, StudentID: 5001, FinalTotal: 72, Credits: 3},
	}}
	notify := &fakeGradeNotify{}
	svc.notify = notify
	svc.identity = &fakeGradeIdentity{roles: []string{"school_admin"}}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 9001})

	out, err := svc.ScanWarnings(ctx, WarningScanRequest{SemesterID: "202601"})
	if err != nil {
		t.Fatalf("ScanWarnings returned error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected two warnings, got %#v", out)
	}
	if notify.sentCount != 2 {
		t.Fatalf("expected two notifications, got %d", notify.sentCount)
	}
}

// TestInternalServiceCanQueryCourseLockStatus 确认服务签名内部入口可查询课程锁定状态,供 M6 改分前校验。
func TestInternalServiceCanQueryCourseLockStatus(t *testing.T) {
	store := newFakeGradeStore()
	store.review = ReviewDTO{ID: "8101", TenantID: "1001", CourseID: "3001", Status: ReviewStatusApproved, IsLocked: true}
	svc := newTestGradeService(store)
	ctx := internalGradeContext()

	out, err := svc.CourseLockStatus(ctx, 3001)
	if err != nil {
		t.Fatalf("CourseLockStatus returned error: %v", err)
	}
	if !out.IsLocked || out.Status != ReviewStatusApproved {
		t.Fatalf("expected locked review, got %#v", out)
	}
}

// TestInternalServiceCanRecomputeStudentGPA 确认申诉/改分链路可用服务签名触发 GPA 重算,不要求伪造管理员账号。
func TestInternalServiceCanRecomputeStudentGPA(t *testing.T) {
	store := newFakeGradeStore()
	store.level = defaultLevelConfig()
	store.review = ReviewDTO{ID: "8101", TenantID: "1001", CourseID: "3001", SemesterID: "202601", Status: ReviewStatusApproved, IsLocked: true}
	svc := newTestGradeService(store)
	svc.teaching = &fakeGradeTeaching{grades: []contracts.TeachingCourseGrade{
		{TenantID: 1001, CourseID: 3001, StudentID: 5001, FinalTotal: 88, Credits: 3},
	}}

	out, err := svc.RecomputeStudent(internalGradeContext(), 5001, RecomputeRequest{CourseID: "3001", SemesterID: "202601"})
	if err != nil {
		t.Fatalf("RecomputeStudent returned error: %v", err)
	}
	if out.StudentID != "5001" || out.SemesterID != "202601" || out.GPA != 3.0 {
		t.Fatalf("expected internal recompute to persist GPA, got %#v", out)
	}
}

// TestInternalServiceCanScanWarnings 确认周期任务等内部服务签名入口可执行学业预警扫描。
func TestInternalServiceCanScanWarnings(t *testing.T) {
	store := newFakeGradeStore()
	store.level = defaultLevelConfig()
	store.semesterGrades = []SemesterGradeDTO{{StudentID: "5001", SemesterID: "202601", GPA: 1.5, TotalCredits: 5}}
	svc := newTestGradeService(store)
	svc.teaching = &fakeGradeTeaching{grades: []contracts.TeachingCourseGrade{
		{TenantID: 1001, CourseID: 3001, StudentID: 5001, FinalTotal: 55, Credits: 2},
	}}
	svc.notify = &fakeGradeNotify{}

	out, err := svc.ScanWarnings(internalGradeContext(), WarningScanRequest{SemesterID: "202601"})
	if err != nil {
		t.Fatalf("ScanWarnings returned error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected two warnings from internal scan, got %#v", out)
	}
}

// TestScanWarningsRequiresNotifyService 确认文档要求“预警经 M10 通知”时,缺少通知依赖会显式失败而不是静默跳过。
func TestScanWarningsRequiresNotifyService(t *testing.T) {
	store := newFakeGradeStore()
	store.level = defaultLevelConfig()
	store.semesterGrades = []SemesterGradeDTO{{StudentID: "5001", SemesterID: "202601", GPA: 1.5, TotalCredits: 5}}
	svc := newTestGradeService(store)
	svc.teaching = &fakeGradeTeaching{grades: []contracts.TeachingCourseGrade{
		{TenantID: 1001, CourseID: 3001, StudentID: 5001, FinalTotal: 55, Credits: 2},
	}}
	svc.notify = nil
	svc.identity = &fakeGradeIdentity{roles: []string{"school_admin"}}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 9001})

	_, err := svc.ScanWarnings(ctx, WarningScanRequest{SemesterID: "202601"})
	if err == nil {
		t.Fatalf("expected missing notify dependency to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrGradeWarningFailed.Code {
		t.Fatalf("expected grade warning failed, got %v", err)
	}
}

// TestScanWarningsDoesNotPersistWarningsWhenNotifyFails 确认预警通知失败时不会留下仅落库未通知的半成功状态。
func TestScanWarningsDoesNotPersistWarningsWhenNotifyFails(t *testing.T) {
	store := newFakeGradeStore()
	store.level = defaultLevelConfig()
	store.semesterGrades = []SemesterGradeDTO{{StudentID: "5001", SemesterID: "202601", GPA: 1.5, TotalCredits: 5}}
	svc := newTestGradeService(store)
	svc.teaching = &fakeGradeTeaching{grades: []contracts.TeachingCourseGrade{
		{TenantID: 1001, CourseID: 3001, StudentID: 5001, FinalTotal: 55, Credits: 2},
	}}
	svc.notify = &fakeGradeNotify{sendErr: errors.New("notify down")}
	svc.identity = &fakeGradeIdentity{roles: []string{"school_admin"}}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 9001})

	_, err := svc.ScanWarnings(ctx, WarningScanRequest{SemesterID: "202601"})
	if err == nil {
		t.Fatalf("expected notify failure")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrGradeWarningFailed.Code {
		t.Fatalf("expected grade warning failed, got %v", err)
	}
	if len(store.warnings) != 0 {
		t.Fatalf("warning write must rollback on notify failure, got %#v", store.warnings)
	}
}

// TestCreateAppealRejectsDuplicatePendingAppeal 确认同一学生同一课程不能反复提交未闭环申诉。
func TestCreateAppealRejectsDuplicatePendingAppeal(t *testing.T) {
	store := newFakeGradeStore()
	store.appeal = AppealDTO{ID: "8201", StudentID: "5001", CourseID: "3001", Status: AppealStatusPending}
	svc := newTestGradeService(store)
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 5001})

	_, err := svc.CreateAppeal(ctx, AppealCreateRequest{CourseID: "3001", Reason: "成绩有误"})
	if err == nil {
		t.Fatalf("expected duplicate appeal error")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrGradeAppealState.Code {
		t.Fatalf("expected appeal state error, got %v", err)
	}
}

// TestCreateAppealRejectsAppealAfterReviewWindow 确认申诉必须落在审核通过后的时效窗口内,避免旧成绩被无限期反复争议。
func TestCreateAppealRejectsAppealAfterReviewWindow(t *testing.T) {
	store := newFakeGradeStore()
	reviewedAt := time.Now().UTC().AddDate(0, 0, -31)
	store.review = ReviewDTO{ID: "8101", TenantID: "1001", CourseID: "3001", Status: ReviewStatusApproved, IsLocked: true, ReviewedAt: &reviewedAt}
	svc := newTestGradeService(store)
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 5001})

	_, err := svc.CreateAppeal(ctx, AppealCreateRequest{CourseID: "3001", Reason: "成绩有误"})
	if err == nil {
		t.Fatalf("expected appeal window error")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrGradeAppealExpired.Code {
		t.Fatalf("expected appeal expired error, got %v", err)
	}
}

// TestCreateAppealRequiresApprovedReview 确认申诉只针对已审核锁定的正式成绩,避免对草稿或待审成绩进入申诉流程。
func TestCreateAppealRequiresApprovedReview(t *testing.T) {
	store := newFakeGradeStore()
	store.review = ReviewDTO{ID: "8101", TenantID: "1001", CourseID: "3001", Status: ReviewStatusPending, IsLocked: false}
	svc := newTestGradeService(store)
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 5001})

	_, err := svc.CreateAppeal(ctx, AppealCreateRequest{CourseID: "3001", Reason: "成绩有误"})
	if err == nil {
		t.Fatalf("expected appeal state error")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrGradeAppealState.Code {
		t.Fatalf("expected appeal state error, got %v", err)
	}
}

// TestGradeServiceReturnsModuleSpecificValidationErrors 确认 M11 业务参数错误不复用平台通用 bad request,避免多个场景共用同一错误码。
func TestGradeServiceReturnsModuleSpecificValidationErrors(t *testing.T) {
	store := newFakeGradeStore()
	svc := newTestGradeService(store)
	svc.identity = &fakeGradeIdentity{roles: []string{"teacher"}}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 9001})

	cases := []struct {
		name string
		run  func() error
		code string
	}{
		{name: "review course", run: func() error {
			svc.identity = &fakeGradeIdentity{roles: []string{"teacher"}}
			_, err := svc.SubmitReview(ctx, ReviewCreateRequest{CourseID: "bad"})
			return err
		}, code: apperr.ErrGradeReviewInvalid.Code},
		{name: "recompute ids", run: func() error {
			svc.identity = &fakeGradeIdentity{roles: []string{"school_admin"}}
			_, err := svc.RecomputeStudent(ctx, 5001, RecomputeRequest{CourseID: "bad", SemesterID: "202601"})
			return err
		}, code: apperr.ErrGradeAggregateInvalid.Code},
		{name: "warning semester", run: func() error {
			svc.identity = &fakeGradeIdentity{roles: []string{"school_admin"}}
			_, err := svc.ScanWarnings(ctx, WarningScanRequest{SemesterID: "bad"})
			return err
		}, code: apperr.ErrGradeWarningInvalid.Code},
		{name: "warning acknowledge id", run: func() error {
			_, err := svc.AcknowledgeWarning(ctx, 0)
			return err
		}, code: apperr.ErrGradeWarningInvalid.Code},
		{name: "transcript student", run: func() error {
			_, err := svc.GenerateTranscript(ctx, TranscriptRequest{StudentID: "bad", Scope: TranscriptScopeAll})
			return err
		}, code: apperr.ErrGradeTranscriptInvalid.Code},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if ae, ok := apperr.As(err); !ok || ae.Code != tc.code {
				t.Fatalf("expected code %s, got %v", tc.code, err)
			}
		})
	}
}

// TestStudentCannotReadOtherStudentGrades 确认服务层兜底拒绝学生读取他人成绩。
func TestStudentCannotReadOtherStudentGrades(t *testing.T) {
	store := newFakeGradeStore()
	svc := newTestGradeService(store)
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 5001})

	_, err := svc.StudentGPA(ctx, 5002)
	if err == nil {
		t.Fatalf("expected forbidden error")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrForbidden.Code {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

// TestTeacherCannotReadArbitraryStudentGradesWithoutCourseScope 确认普通教师没有任课范围契约时不能默认读取任意学生成绩。
func TestTeacherCannotReadArbitraryStudentGradesWithoutCourseScope(t *testing.T) {
	store := newFakeGradeStore()
	svc := newTestGradeService(store)
	svc.identity = &fakeGradeIdentity{roles: []string{"teacher"}}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 7001})

	_, err := svc.StudentGPA(ctx, 5002)
	if err == nil {
		t.Fatalf("expected forbidden error")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrForbidden.Code {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

// TestSchoolAdminOnlyActionsRejectNonAdmin 确认 M11 服务层对学校管理员动作做服务端角色兜底。
func TestSchoolAdminOnlyActionsRejectNonAdmin(t *testing.T) {
	store := newFakeGradeStore()
	store.review = ReviewDTO{ID: "8101", TenantID: "1001", CourseID: "3001", Status: ReviewStatusPending}
	svc := newTestGradeService(store)
	svc.identity = &fakeGradeIdentity{}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 5001})

	cases := []struct {
		name string
		run  func() error
	}{
		{name: "list level configs", run: func() error {
			_, err := svc.ListLevelConfigs(ctx)
			return err
		}},
		{name: "create level config", run: func() error {
			_, err := svc.CreateLevelConfig(ctx, defaultLevelConfigRequest())
			return err
		}},
		{name: "update level config", run: func() error {
			_, err := svc.UpdateLevelConfig(ctx, 9001, defaultLevelConfigRequest())
			return err
		}},
		{name: "list semesters", run: func() error {
			_, err := svc.ListSemesters(ctx)
			return err
		}},
		{name: "create semester", run: func() error {
			_, err := svc.CreateSemester(ctx, SemesterRequest{Name: "2026 春"})
			return err
		}},
		{name: "warning rules", run: func() error {
			_, err := svc.WarningRules(ctx)
			return err
		}},
		{name: "update warning rules", run: func() error {
			_, err := svc.UpdateWarningRules(ctx, WarningRuleDTO{FailCount: 1, MinGPA: 2.0})
			return err
		}},
		{name: "list reviews", run: func() error {
			_, _, err := svc.ListReviews(ctx, 0, 1, 20)
			return err
		}},
		{name: "approve review", run: func() error {
			_, err := svc.ApproveReview(ctx, 8101, ReviewDecisionRequest{SemesterID: "202601"})
			return err
		}},
		{name: "reject review", run: func() error {
			_, err := svc.RejectReview(ctx, 8101, ReviewDecisionRequest{Comment: "reject"})
			return err
		}},
		{name: "unlock review", run: func() error {
			_, err := svc.UnlockReview(ctx, 8101, ReviewDecisionRequest{Comment: "unlock"})
			return err
		}},
		{name: "recompute student", run: func() error {
			_, err := svc.RecomputeStudent(ctx, 5001, RecomputeRequest{CourseID: "3001", SemesterID: "202601"})
			return err
		}},
		{name: "scan warnings", run: func() error {
			_, err := svc.ScanWarnings(ctx, WarningScanRequest{SemesterID: "202601"})
			return err
		}},
		{name: "batch transcripts", run: func() error {
			_, err := svc.BatchGenerateTranscripts(ctx, TranscriptBatchRequest{StudentIDs: []string{"5001"}, Scope: TranscriptScopeAll})
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil {
				t.Fatalf("expected forbidden error")
			}
			if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrForbidden.Code {
				t.Fatalf("expected forbidden error, got %v", err)
			}
		})
	}
}

// TestTeacherOrAdminActionsRejectStudent 确认教师/学校管理员专属动作不会被学生直接调用。
func TestTeacherOrAdminActionsRejectStudent(t *testing.T) {
	store := newFakeGradeStore()
	store.review = ReviewDTO{ID: "8101", TenantID: "1001", CourseID: "3001", Status: ReviewStatusApproved}
	store.appeal = AppealDTO{ID: "8201", TenantID: "1001", StudentID: "5002", CourseID: "3001", Status: AppealStatusPending}
	svc := newTestGradeService(store)
	svc.identity = &fakeGradeIdentity{}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 5001})

	cases := []struct {
		name string
		run  func() error
	}{
		{name: "submit review", run: func() error {
			_, err := svc.SubmitReview(ctx, ReviewCreateRequest{CourseID: "3001"})
			return err
		}},
		{name: "course lock status", run: func() error {
			_, err := svc.CourseLockStatus(ctx, 3001)
			return err
		}},
		{name: "list appeals", run: func() error {
			_, _, err := svc.ListAppeals(ctx, 0, 1, 20)
			return err
		}},
		{name: "accept appeal", run: func() error {
			_, err := svc.AcceptAppeal(ctx, 8201, AppealHandleRequest{ResultComment: "accept"})
			return err
		}},
		{name: "reject appeal", run: func() error {
			_, err := svc.RejectAppeal(ctx, 8201, AppealHandleRequest{ResultComment: "reject"})
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil {
				t.Fatalf("expected forbidden error")
			}
			if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrForbidden.Code {
				t.Fatalf("expected forbidden error, got %v", err)
			}
		})
	}
}

// TestPgNumericRejectsInvalidFloat 确认成绩数值转换失败必须显式返回错误,不能静默写入零值。
func TestPgNumericRejectsInvalidFloat(t *testing.T) {
	if _, err := pgNumeric(math.Inf(1)); err == nil {
		t.Fatalf("expected invalid numeric error")
	}
}

// TestRenderTranscriptPDFIncludesVerificationCode 确认正式成绩单带服务端签名验证码,便于学校核验 PDF 未被伪造。
func TestRenderTranscriptPDFIncludesVerificationCode(t *testing.T) {
	store := newFakeGradeStore()
	store.semesterGrades = []SemesterGradeDTO{{StudentID: "5001", SemesterID: "202601", GPA: 3.5, CumulativeGPA: 3.5, TotalCredits: 4}}
	svc := newTestGradeService(store)
	svc.teaching = &fakeGradeTeaching{grades: []contracts.TeachingCourseGrade{
		{TenantID: 1001, CourseID: 3001, StudentID: 5001, FinalTotal: 90, Credits: 4},
	}}

	content, err := svc.renderTranscriptPDF(context.Background(), 1001, 5001, TranscriptRequest{StudentID: "5001", Scope: TranscriptScopeSemester, SemesterID: "202601"})
	if err != nil {
		t.Fatalf("renderTranscriptPDF returned error: %v", err)
	}
	text := string(content)
	if !strings.HasPrefix(text, "%PDF-") {
		t.Fatalf("expected PDF content, got %q", text[:min(len(text), 20)])
	}
	if !strings.Contains(text, "Verification:") {
		t.Fatalf("expected verification code in transcript PDF")
	}
	if strings.Contains(text, "minimalPDF") {
		t.Fatalf("transcript renderer should not expose legacy minimal PDF marker")
	}
}

// TestGenerateTranscriptRequiresStorage 确认正式成绩单必须先写入对象存储,不能生成不可下载的空记录。
func TestGenerateTranscriptRequiresStorage(t *testing.T) {
	store := newFakeGradeStore()
	svc := newTestGradeService(store)
	svc.identity = &fakeGradeIdentity{roles: []string{contracts.RoleSchoolAdmin}}
	svc.teaching = &fakeGradeTeaching{}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 9001})

	_, err := svc.GenerateTranscript(ctx, TranscriptRequest{StudentID: "5001", Scope: TranscriptScopeAll})
	if err == nil {
		t.Fatalf("expected missing storage to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrGradeTranscriptFailed.Code {
		t.Fatalf("expected transcript failed error, got %v", err)
	}
	if store.transcriptWriteCount != 0 {
		t.Fatalf("transcript metadata must not be written without stored PDF, writes=%d", store.transcriptWriteCount)
	}
}

// newTestGradeService 构造测试用成绩中心服务。
func newTestGradeService(store gradeStore) *Service {
	return &Service{
		store:   store,
		idgen:   fixedIDGen(10001),
		auditor: &captureGradeAuditWriter{},
		cfg:     config.GradeConfig{AppealWindowDays: 30, TranscriptSigningKey: "test-transcript-hmac-key"},
	}
}

type fixedIDGen int64

// Generate 返回递增固定 ID,让测试断言稳定。
func (g fixedIDGen) Generate() int64 { return int64(g) }

type fakeGradeStore struct {
	level                LevelConfigDTO
	review               ReviewDTO
	reviewErr            error
	reviews              []ReviewDTO
	appeal               AppealDTO
	semesterGrade        SemesterGradeDTO
	semesterGrades       []SemesterGradeDTO
	warnings             []WarningDTO
	writeCount           int
	transcriptWriteCount int
}

// newFakeGradeStore 构造默认测试存储。
func newFakeGradeStore() *fakeGradeStore {
	return &fakeGradeStore{level: defaultLevelConfig()}
}

func (f *fakeGradeStore) ListLevelConfigs(context.Context, int64) ([]LevelConfigDTO, error) {
	return []LevelConfigDTO{f.level}, nil
}
func (f *fakeGradeStore) CreateLevelConfig(context.Context, int64, int64, LevelConfigRequest) (LevelConfigDTO, error) {
	return LevelConfigDTO{}, nil
}
func (f *fakeGradeStore) UpdateLevelConfig(context.Context, int64, int64, LevelConfigRequest) (LevelConfigDTO, error) {
	return LevelConfigDTO{}, nil
}
func (f *fakeGradeStore) DefaultLevelConfig(context.Context, int64) (LevelConfigDTO, error) {
	return f.level, nil
}
func (f *fakeGradeStore) ListSemesters(context.Context, int64) ([]SemesterDTO, error) {
	return nil, nil
}
func (f *fakeGradeStore) CreateSemester(context.Context, int64, int64, SemesterRequest) (SemesterDTO, error) {
	return SemesterDTO{}, nil
}
func (f *fakeGradeStore) UpsertSemesterGrade(_ context.Context, tenantID int64, req SemesterGradeUpsert) (SemesterGradeDTO, error) {
	f.writeCount++
	f.semesterGrade = SemesterGradeDTO{
		ID: ids.Format(req.ID), TenantID: ids.Format(tenantID), StudentID: ids.Format(req.StudentID), SemesterID: ids.Format(req.SemesterID),
		TotalCredits: req.TotalCredits, GPA: req.GPA, CumulativeGPA: req.CumulativeGPA,
	}
	f.semesterGrades = append(f.semesterGrades, f.semesterGrade)
	return f.semesterGrade, nil
}
func (f *fakeGradeStore) ListSemesterGrades(context.Context, int64, int64) ([]SemesterGradeDTO, error) {
	return f.semesterGrades, nil
}
func (f *fakeGradeStore) ListStudentSemesterGrades(context.Context, int64, int64) ([]SemesterGradeDTO, error) {
	return f.semesterGrades, nil
}
func (f *fakeGradeStore) CreateReview(_ context.Context, tenantID int64, id int64, req ReviewCreateRequest, submitterID int64) (ReviewDTO, error) {
	f.review = ReviewDTO{ID: ids.Format(id), TenantID: ids.Format(tenantID), CourseID: req.CourseID, SubmitterID: ids.Format(submitterID), Status: ReviewStatusPending}
	return f.review, nil
}
func (f *fakeGradeStore) GetReview(context.Context, int64, int64) (ReviewDTO, error) {
	return f.review, nil
}
func (f *fakeGradeStore) GetReviewByCourse(context.Context, int64, int64) (ReviewDTO, error) {
	if f.reviewErr != nil {
		return ReviewDTO{}, f.reviewErr
	}
	return f.review, nil
}
func (f *fakeGradeStore) ListReviews(_ context.Context, _ int64, status int16, _ int, _ int) ([]ReviewDTO, int64, error) {
	filter := func(rows []ReviewDTO, status int16) []ReviewDTO {
		if status == 0 {
			return rows
		}
		out := make([]ReviewDTO, 0, len(rows))
		for _, row := range rows {
			if row.Status == status {
				out = append(out, row)
			}
		}
		return out
	}
	if len(f.reviews) > 0 {
		rows := filter(f.reviews, status)
		return rows, int64(len(rows)), nil
	}
	if f.review.ID != "" {
		rows := filter([]ReviewDTO{f.review}, status)
		return rows, int64(len(rows)), nil
	}
	return nil, 0, nil
}
func (f *fakeGradeStore) ApproveReview(_ context.Context, _ int64, reviewerID int64, reviewID int64, semesterID int64, comment string) (ReviewDTO, error) {
	f.review.ID = ids.Format(reviewID)
	f.review.ReviewerID = ids.Format(reviewerID)
	f.review.SemesterID = ids.Format(semesterID)
	f.review.Status = ReviewStatusApproved
	f.review.IsLocked = true
	f.review.Comment = comment
	return f.review, nil
}
func (f *fakeGradeStore) RejectReview(context.Context, int64, int64, int64, string) (ReviewDTO, error) {
	return ReviewDTO{}, nil
}
func (f *fakeGradeStore) UnlockReview(_ context.Context, _ int64, reviewerID int64, reviewID int64, comment string) (ReviewDTO, error) {
	f.review.ID = ids.Format(reviewID)
	f.review.ReviewerID = ids.Format(reviewerID)
	f.review.Status = ReviewStatusPending
	f.review.IsLocked = false
	f.review.Comment = comment
	return f.review, nil
}
func (f *fakeGradeStore) RelockReviewByCourse(_ context.Context, tenantID int64, courseID int64) (ReviewDTO, error) {
	f.review.TenantID = ids.Format(tenantID)
	f.review.CourseID = ids.Format(courseID)
	f.review.Status = ReviewStatusApproved
	f.review.IsLocked = true
	return f.review, nil
}
func (f *fakeGradeStore) CreateAppeal(_ context.Context, tenantID int64, appealID int64, req AppealCreateRequest, studentID int64) (AppealDTO, error) {
	f.appeal = AppealDTO{ID: ids.Format(appealID), TenantID: ids.Format(tenantID), StudentID: ids.Format(studentID), CourseID: req.CourseID, Reason: req.Reason, Status: AppealStatusPending}
	return f.appeal, nil
}
func (f *fakeGradeStore) FindOpenAppeal(context.Context, int64, int64, int64) (AppealDTO, bool, error) {
	if f.appeal.Status == AppealStatusPending || f.appeal.Status == AppealStatusAccepted {
		return f.appeal, true, nil
	}
	return AppealDTO{}, false, nil
}
func (f *fakeGradeStore) GetAppeal(context.Context, int64, int64) (AppealDTO, error) {
	return f.appeal, nil
}
func (f *fakeGradeStore) ListAppeals(context.Context, int64, int16, int, int) ([]AppealDTO, int64, error) {
	return []AppealDTO{f.appeal}, 1, nil
}
func (f *fakeGradeStore) UpdateAppealStatus(_ context.Context, _ int64, appealID int64, handlerID int64, status int16, comment string) (AppealDTO, error) {
	f.appeal.ID = ids.Format(appealID)
	f.appeal.HandlerID = ids.Format(handlerID)
	f.appeal.Status = status
	f.appeal.ResultComment = comment
	return f.appeal, nil
}
func (f *fakeGradeStore) CreateWarning(_ context.Context, tenantID int64, warningID int64, req WarningCreate) (WarningDTO, error) {
	dto := WarningDTO{ID: ids.Format(warningID), TenantID: ids.Format(tenantID), StudentID: ids.Format(req.StudentID), SemesterID: ids.Format(req.SemesterID), Type: req.Type, Detail: req.Detail, Status: WarningStatusPending}
	f.warnings = append(f.warnings, dto)
	return dto, nil
}
func (f *fakeGradeStore) DeleteWarning(_ context.Context, _ int64, warningID int64) error {
	filtered := f.warnings[:0]
	for _, warn := range f.warnings {
		if warn.ID != ids.Format(warningID) {
			filtered = append(filtered, warn)
		}
	}
	f.warnings = filtered
	return nil
}
func (f *fakeGradeStore) ListWarnings(context.Context, int64, int64, int64, int16, int, int) ([]WarningDTO, int64, error) {
	return f.warnings, int64(len(f.warnings)), nil
}
func (f *fakeGradeStore) AcknowledgeWarning(context.Context, int64, int64, int64) (WarningDTO, error) {
	return WarningDTO{}, nil
}
func (f *fakeGradeStore) CreateTranscript(context.Context, int64, int64, TranscriptRequest, string) (TranscriptDTO, error) {
	f.transcriptWriteCount++
	return TranscriptDTO{}, nil
}
func (f *fakeGradeStore) GetTranscript(context.Context, int64, int64) (TranscriptDTO, error) {
	return TranscriptDTO{}, nil
}

type fakeGradeTeaching struct {
	grades      []contracts.TeachingCourseGrade
	updateCalls int
}

func (f *fakeGradeTeaching) ListCourseGrades(context.Context, int64, int64) ([]contracts.TeachingCourseGrade, error) {
	return f.grades, nil
}
func (f *fakeGradeTeaching) ListStudentGrades(_ context.Context, _ int64, studentID int64) ([]contracts.TeachingCourseGrade, error) {
	out := make([]contracts.TeachingCourseGrade, 0, len(f.grades))
	for _, grade := range f.grades {
		if grade.StudentID == studentID {
			out = append(out, grade)
		}
	}
	return out, nil
}
func (f *fakeGradeTeaching) Stats(context.Context, int64) (contracts.TeachingStats, error) {
	return contracts.TeachingStats{}, nil
}

type fakeGradeNotify struct {
	sentCount int
	sendErr   error
}

func (f *fakeGradeNotify) Send(context.Context, contracts.NotifySendRequest) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	f.sentCount++
	return nil
}
func (f *fakeGradeNotify) Push(context.Context, contracts.NotifyPushRequest) error { return nil }

type fakeGradeEventBus struct{}

func (fakeGradeEventBus) Publish(context.Context, string, any) error { return nil }
func (fakeGradeEventBus) Subscribe(string, string, eventbus.Handler) (eventbus.Subscription, error) {
	return nil, nil
}
func (fakeGradeEventBus) Close() {}

// internalGradeContext 构造服务 HMAC 中间件注入后的租户上下文。
func internalGradeContext() context.Context {
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001})
	return auth.WithServiceSourceRef(ctx, "grade:2026:warning-scan:1")
}

// defaultLevelConfig 返回测试用成绩等级与预警规则。
func defaultLevelConfig() LevelConfigDTO {
	return LevelConfigDTO{
		ID:        "9001",
		TenantID:  "1001",
		Name:      "default",
		IsDefault: true,
		Mapping: []LevelMappingDTO{
			{Min: 90, Grade: "A", GPA: 4.0},
			{Min: 80, Grade: "B", GPA: 3.0},
			{Min: 60, Grade: "D", GPA: 1.0},
			{Min: 0, Grade: "F", GPA: 0},
		},
		WarningRules: WarningRuleDTO{FailCount: 1, MinGPA: 2.0},
	}
}

func defaultLevelConfigRequest() LevelConfigRequest {
	return LevelConfigRequest{
		Name: "default",
		Mapping: []LevelMappingDTO{
			{Min: 90, Grade: "A", GPA: 4.0},
			{Min: 80, Grade: "B", GPA: 3.0},
			{Min: 60, Grade: "D", GPA: 1.0},
			{Min: 0, Grade: "F", GPA: 0},
		},
		WarningRules: WarningRuleDTO{FailCount: 1, MinGPA: 2.0},
		IsDefault:    true,
	}
}
