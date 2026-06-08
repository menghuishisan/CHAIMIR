// M11 审计测试:确保成绩中心统一走审计且角色由服务端身份决定。
package grade

import (
	"context"
	"testing"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestWriteAuditRequiresConfiguredWriter 确认 M11 缺少审计 writer 时显式失败。
func TestWriteAuditRequiresConfiguredWriter(t *testing.T) {
	svc := &Service{}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	err := svc.writeAudit(ctx, 1001, 2001, "grade.review.submit", "grade_review", 3001, map[string]any{"course_id": "3001"})
	if err == nil {
		t.Fatalf("expected missing auditor to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrGradeAuditWriteFailed.Code {
		t.Fatalf("expected grade audit write error, got %v", err)
	}
}

// TestSubmitReviewWritesTeacherAuditRole 确认教师提交成绩审核时 actor_role 记录为教师而非固定学校管理员。
func TestSubmitReviewWritesTeacherAuditRole(t *testing.T) {
	store := newFakeGradeStore()
	writer := &captureGradeAuditWriter{}
	svc := newTestGradeService(store)
	svc.auditor = writer
	svc.identity = &gradeAuditIdentity{account: contracts.AccountInfo{AccountID: 7001, TenantID: 1001, BaseIdentity: 2, Roles: []string{"teacher"}}}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 7001})

	if _, err := svc.SubmitReview(ctx, ReviewCreateRequest{CourseID: "3001"}); err != nil {
		t.Fatalf("SubmitReview returned error: %v", err)
	}
	if len(writer.entries) != 1 {
		t.Fatalf("expected one audit entry, got %d", len(writer.entries))
	}
	if writer.entries[0].ActorRole != 3 {
		t.Fatalf("expected teacher actor role, got %d", writer.entries[0].ActorRole)
	}
}

// TestReviewAndAppealStateChangesWriteAudit 确认审核驳回、解锁、申诉提交和驳回等高敏感状态变更全程留痕。
func TestReviewAndAppealStateChangesWriteAudit(t *testing.T) {
	reviewedAt := time.Now().UTC()
	cases := []struct {
		name  string
		store *fakeGradeStore
		id    tenant.Identity
		roles []string
		run   func(context.Context, *Service) error
		want  string
	}{
		{
			name:  "reject review",
			store: &fakeGradeStore{level: defaultLevelConfig(), review: ReviewDTO{ID: "8101", TenantID: "1001", CourseID: "3001", Status: ReviewStatusPending}},
			id:    tenant.Identity{TenantID: 1001, AccountID: 9001},
			roles: []string{"school_admin"},
			run: func(ctx context.Context, svc *Service) error {
				_, err := svc.RejectReview(ctx, 8101, ReviewDecisionRequest{Comment: "退回修改"})
				return err
			},
			want: "grade.review.reject",
		},
		{
			name:  "unlock review",
			store: &fakeGradeStore{level: defaultLevelConfig(), review: ReviewDTO{ID: "8101", TenantID: "1001", CourseID: "3001", Status: ReviewStatusApproved, IsLocked: true}},
			id:    tenant.Identity{TenantID: 1001, AccountID: 9001},
			roles: []string{"school_admin"},
			run: func(ctx context.Context, svc *Service) error {
				_, err := svc.UnlockReview(ctx, 8101, ReviewDecisionRequest{Comment: "申诉解锁"})
				return err
			},
			want: "grade.review.unlock",
		},
		{
			name:  "create appeal",
			store: &fakeGradeStore{level: defaultLevelConfig(), review: ReviewDTO{ID: "8101", TenantID: "1001", CourseID: "3001", Status: ReviewStatusApproved, IsLocked: true, ReviewedAt: &reviewedAt}},
			id:    tenant.Identity{TenantID: 1001, AccountID: 5001},
			roles: []string{"student"},
			run: func(ctx context.Context, svc *Service) error {
				_, err := svc.CreateAppeal(ctx, AppealCreateRequest{CourseID: "3001", Reason: "成绩有误"})
				return err
			},
			want: "grade.appeal.create",
		},
		{
			name:  "reject appeal",
			store: &fakeGradeStore{level: defaultLevelConfig(), appeal: AppealDTO{ID: "8201", TenantID: "1001", StudentID: "5001", CourseID: "3001", Status: AppealStatusPending}},
			id:    tenant.Identity{TenantID: 1001, AccountID: 9001},
			roles: []string{"school_admin"},
			run: func(ctx context.Context, svc *Service) error {
				_, err := svc.RejectAppeal(ctx, 8201, AppealHandleRequest{ResultComment: "依据不足"})
				return err
			},
			want: "grade.appeal.reject",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writer := &captureGradeAuditWriter{}
			svc := newTestGradeService(tc.store)
			svc.auditor = writer
			svc.identity = &gradeAuditIdentity{account: contracts.AccountInfo{AccountID: tc.id.AccountID, TenantID: tc.id.TenantID, Roles: tc.roles}}
			ctx := tenant.WithContext(context.Background(), tc.id)

			if err := tc.run(ctx, svc); err != nil {
				t.Fatalf("operation returned error: %v", err)
			}
			if len(writer.entries) != 1 {
				t.Fatalf("expected one audit entry, got %d", len(writer.entries))
			}
			if writer.entries[0].Action != tc.want {
				t.Fatalf("expected action %s, got %s", tc.want, writer.entries[0].Action)
			}
		})
	}
}

// TestScanWarningsWritesAudit 确认学业预警扫描生成预警并通知后,也会追加一条 M11 审计记录。
func TestScanWarningsWritesAudit(t *testing.T) {
	store := newFakeGradeStore()
	store.level = defaultLevelConfig()
	store.semesterGrades = []SemesterGradeDTO{{StudentID: "5001", SemesterID: "202601", GPA: 1.5, TotalCredits: 5}}
	writer := &captureGradeAuditWriter{}
	svc := newTestGradeService(store)
	svc.auditor = writer
	svc.teaching = &fakeGradeTeaching{grades: []contracts.TeachingCourseGrade{
		{TenantID: 1001, CourseID: 3001, StudentID: 5001, FinalTotal: 55, Credits: 2},
	}}
	svc.notify = &fakeGradeNotify{}

	if _, err := svc.ScanWarnings(internalGradeContext(), WarningScanRequest{SemesterID: "202601"}); err != nil {
		t.Fatalf("ScanWarnings returned error: %v", err)
	}
	if len(writer.entries) != 1 {
		t.Fatalf("expected one audit entry, got %d", len(writer.entries))
	}
	if writer.entries[0].Action != "grade.warning.scan" {
		t.Fatalf("expected warning scan audit action, got %s", writer.entries[0].Action)
	}
}

type captureGradeAuditWriter struct {
	entries []audit.Entry
}

func (w *captureGradeAuditWriter) Write(_ context.Context, entry audit.Entry) error {
	w.entries = append(w.entries, entry)
	return nil
}

type gradeAuditIdentity struct {
	account contracts.AccountInfo
}

func (f *gradeAuditIdentity) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return f.account, nil
}

func (f *gradeAuditIdentity) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}

func (f *gradeAuditIdentity) HasRole(_ context.Context, _ int64, role string) (bool, error) {
	for _, actual := range f.account.Roles {
		if actual == role {
			return true, nil
		}
	}
	return false, nil
}
