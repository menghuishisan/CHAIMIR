// M6 服务规则测试:覆盖课程状态、权重、迟交策略与成绩计算等核心教学边界。
package teaching

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"

	"github.com/xuri/excelize/v2"
)

// TestCourseStatusTransitionsFollowDocumentedLifecycle 确认课程生命周期只能按文档方向流转。
func TestCourseStatusTransitionsFollowDocumentedLifecycle(t *testing.T) {
	if err := validateCourseTransition(CourseStatusDraft, CourseStatusPublished); err != nil {
		t.Fatalf("draft should publish: %v", err)
	}
	if err := validateCourseTransition(CourseStatusPublished, CourseStatusEnded); err != nil {
		t.Fatalf("published should end manually: %v", err)
	}
	if err := validateCourseTransition(CourseStatusEnded, CourseStatusArchived); err != nil {
		t.Fatalf("ended should archive: %v", err)
	}
	if err := validateCourseTransition(CourseStatusArchived, CourseStatusPublished); err == nil {
		t.Fatalf("archived course must not be republished")
	}
}

// TestNormalizeCourseListRoleKeepsRoleDefaultInService 确认课程列表 role 默认值和校验在服务规则层完成。
func TestNormalizeCourseListRoleKeepsRoleDefaultInService(t *testing.T) {
	role, err := normalizeCourseListRole("")
	if err != nil {
		t.Fatalf("empty role should use teacher default: %v", err)
	}
	if role != contracts.RoleTeacher {
		t.Fatalf("empty role expected teacher, got %q", role)
	}
	role, err = normalizeCourseListRole(contracts.RoleStudent)
	if err != nil {
		t.Fatalf("student role should be accepted: %v", err)
	}
	if role != contracts.RoleStudent {
		t.Fatalf("student role changed to %q", role)
	}
	if _, err := normalizeCourseListRole("admin"); err != apperr.ErrCourseInvalid {
		t.Fatalf("invalid role should return course invalid error, got %v", err)
	}
}

// TestPublishCourseRequiresOutlineCompleteness 守护课程发布必须校验至少 1 个章节且每章至少 1 个课时。
func TestPublishCourseRequiresOutlineCompleteness(t *testing.T) {
	src, err := os.ReadFile("service_helpers.go")
	if err != nil {
		t.Fatalf("read service helpers: %v", err)
	}
	text := functionSource(string(src), "updateCourseStatus")
	for _, required := range []string{"CourseStatusPublished", "EnsureCoursePublishable", "ErrCourseInvalidState"} {
		if !strings.Contains(text, required) {
			t.Fatalf("course publish must validate documented outline completeness, missing %s", required)
		}
	}
}

// TestCreateCourseRequiresTeacherRole 确认服务层建课入口校验教师角色,不只依赖路由层。
func TestCreateCourseRequiresTeacherRole(t *testing.T) {
	src, err := os.ReadFile("course_service.go")
	if err != nil {
		t.Fatalf("read course service: %v", err)
	}
	text := functionSource(string(src), "CreateCourse")
	if !strings.Contains(text, "ensureTeacherRole") {
		t.Fatalf("create course must verify teacher role in service layer")
	}

	helpers, err := os.ReadFile("service_helpers.go")
	if err != nil {
		t.Fatalf("read service helpers: %v", err)
	}
	helperText := string(helpers)
	for _, required := range []string{"func (s *Service) ensureTeacherRole", "contracts.RoleTeacher", "contracts.RoleSchoolAdmin", "contracts.HasAnyRole"} {
		if !strings.Contains(helperText, required) {
			t.Fatalf("teacher role helper must use unified contracts roles, missing %s", required)
		}
	}
}

// TestCourseMembershipWritesRequireStudentRole 确认课程成员写入只允许学生账号进入成员表。
func TestCourseMembershipWritesRequireStudentRole(t *testing.T) {
	src, err := os.ReadFile("course_service.go")
	if err != nil {
		t.Fatalf("read course service: %v", err)
	}
	text := string(src)
	join := functionSource(text, "JoinCourseByInvite")
	if !strings.Contains(join, "ensureStudentRole") {
		t.Fatalf("invite join must verify current account is a student")
	}
	add := functionSource(text, "AddMembers")
	if !strings.Contains(add, "ensureAccountStudent") {
		t.Fatalf("teacher batch add must verify every target account is a student")
	}

	helpers, err := os.ReadFile("service_helpers.go")
	if err != nil {
		t.Fatalf("read service helpers: %v", err)
	}
	helperText := string(helpers)
	for _, required := range []string{"func (s *Service) ensureAccountStudent", "HasRole", "contracts.RoleStudent"} {
		if !strings.Contains(helperText, required) {
			t.Fatalf("student target helper must use identity role contract, missing %s", required)
		}
	}
}

// TestLessonContentRefsAreValidatedByType 确认课时内容按文档类型校验引用结构。
func TestLessonContentRefsAreValidatedByType(t *testing.T) {
	cases := []struct {
		name string
		req  LessonContentRequest
		ok   bool
	}{
		{name: "video", req: LessonContentRequest{ContentType: LessonContentVideo, ContentRef: map[string]any{"storage_key": "tenant/course/video.mp4"}}, ok: true},
		{name: "markdown", req: LessonContentRequest{ContentType: LessonContentMarkdown, ContentRef: map[string]any{"markdown": "# intro"}}, ok: true},
		{name: "attachment", req: LessonContentRequest{ContentType: LessonContentAttachment, ContentRef: map[string]any{"storage_key": "tenant/course/file.pdf"}}, ok: true},
		{name: "experiment", req: LessonContentRequest{ContentType: LessonContentExperiment, ContentRef: map[string]any{"experiment_id": "7001"}}, ok: true},
		{name: "simulation", req: LessonContentRequest{ContentType: LessonContentSimulation, ContentRef: map[string]any{"package_code": "evm-demo", "version": "1.0.0"}}, ok: true},
		{name: "bad type", req: LessonContentRequest{ContentType: 99, ContentRef: map[string]any{"storage_key": "x"}}, ok: false},
		{name: "missing ref", req: LessonContentRequest{ContentType: LessonContentVideo, ContentRef: map[string]any{}}, ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLessonContentRef(tc.req.ContentType, tc.req.ContentRef)
			if tc.ok && err != nil {
				t.Fatalf("valid lesson content rejected: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("invalid lesson content accepted: %#v", tc.req)
			}
		})
	}
}

// TestCloneCourseCopiesCourseStructure 确认课程克隆复制章节与课时结构,不只复制课程基础字段。
func TestCloneCourseCopiesCourseStructure(t *testing.T) {
	src, err := os.ReadFile("course_service.go")
	if err != nil {
		t.Fatalf("read course service: %v", err)
	}
	text := functionSource(string(src), "CloneCourse")
	for _, required := range []string{"cloneCourseStructure"} {
		if !strings.Contains(text, required) {
			t.Fatalf("clone course must copy chapters and lessons, missing %s", required)
		}
	}

	helpers, err := os.ReadFile("service_helpers.go")
	if err != nil {
		t.Fatalf("read service helpers: %v", err)
	}
	helperText := string(helpers)
	for _, required := range []string{"func (s *Service) cloneCourseStructure", "ListChaptersByCourse", "ListLessonsByChapter", "CreateChapter", "CreateLesson"} {
		if !strings.Contains(helperText, required) {
			t.Fatalf("clone helper must persist copied course structure, missing %s", required)
		}
	}
}

// TestValidateGradeWeightsRequiresExactlyOneHundred 确认成绩权重必须精确合计 100%。
func TestValidateGradeWeightsRequiresExactlyOneHundred(t *testing.T) {
	if err := validateGradeWeights([]GradeWeightInput{{Weight: 40}, {Weight: 60}}); err != nil {
		t.Fatalf("valid weights rejected: %v", err)
	}
	if err := validateGradeWeights([]GradeWeightInput{{Weight: 40}, {Weight: 59.99}}); err == nil {
		t.Fatalf("weights not totaling 100 must be rejected")
	}
}

// TestGradeOverrideAuditRecordsOldAndNewValues 确认手动调分审计记录原值和新值。
func TestGradeOverrideAuditRecordsOldAndNewValues(t *testing.T) {
	src, err := os.ReadFile("grade_service.go")
	if err != nil {
		t.Fatalf("read grade service: %v", err)
	}
	text := functionSource(string(src), "OverrideGrade")
	for _, required := range []string{"before", "after", "getCourseGradeSnapshot", "gradeAuditSnapshot"} {
		if !strings.Contains(text, required) {
			t.Fatalf("grade override audit must include old/new grade values, missing %s", required)
		}
	}
	helpers, err := os.ReadFile("service_helpers.go")
	if err != nil {
		t.Fatalf("read service helpers: %v", err)
	}
	helperText := string(helpers)
	for _, required := range []string{"auto_total", "override_total", "final_total"} {
		if !strings.Contains(helperText, required) {
			t.Fatalf("grade audit snapshot must include %s", required)
		}
	}
}

// TestApplyLatePolicyRejectsDisallowedLateSubmission 确认不允许迟交时会拒收截止后提交。
func TestApplyLatePolicyRejectsDisallowedLateSubmission(t *testing.T) {
	due := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	if _, err := applyLatePolicy(due, due.Add(time.Minute), LatePolicyReject, nil, 100); err == nil {
		t.Fatalf("late submission should be rejected when policy rejects late work")
	}
	result, err := applyLatePolicy(due, due.Add(time.Minute), LatePolicyNoPenalty, nil, 100)
	if err != nil {
		t.Fatalf("no-penalty late policy should accept submission: %v", err)
	}
	if !result.IsLate || result.FinalScore != 100 {
		t.Fatalf("unexpected late result: %#v", result)
	}
}

// TestSubmissionScoringAppliesLatePenalty 确认自动判题和教师批改写最终分时应用迟交扣分。
func TestSubmissionScoringAppliesLatePenalty(t *testing.T) {
	src, err := os.ReadFile("assignment_service.go")
	if err != nil {
		t.Fatalf("read assignment service: %v", err)
	}
	text := string(src)
	for _, name := range []string{"GradeSubmission", "finalScoreForSubmission"} {
		if !strings.Contains(text, name) {
			t.Fatalf("manual grading must calculate final score with late policy, missing %s", name)
		}
	}

	events, err := os.ReadFile("events.go")
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if !strings.Contains(string(events), "finalScoreForSubmission") {
		t.Fatalf("judge completion must calculate final score with late policy")
	}
}

// TestSubmitAssignmentBindsJudgeTaskToCreatedSubmission 确认自动判题提交先持久化 submission 与 outbox,不在请求事务内直接调用 M3。
func TestSubmitAssignmentBindsJudgeTaskToCreatedSubmission(t *testing.T) {
	src, err := os.ReadFile("assignment_service.go")
	if err != nil {
		t.Fatalf("read assignment service: %v", err)
	}
	text := functionSource(string(src), "SubmitAssignment")
	createAt := strings.Index(text, "CreateSubmission")
	outboxAt := strings.Index(text, "CreateSubmissionJudgeOutbox")
	outboxAt = strings.Index(text, "createSubmissionJudgeOutbox")
	if createAt < 0 || outboxAt < 0 {
		t.Fatalf("submit assignment must create submission and local judge outbox")
	}
	createCall := text[createAt:]
	if !strings.Contains(createCall, "JudgeTaskRef: pgText(\"\")") {
		t.Fatalf("auto-graded submission must be created with an empty task ref before outbox dispatch succeeds")
	}
	if outboxAt < createAt {
		t.Fatalf("judge outbox must be written after submission id is created")
	}
	deleteAt := strings.Index(text, "DeleteSubmissionDraft")
	if deleteAt < outboxAt {
		t.Fatalf("server draft must only be deleted after local outbox is persisted")
	}
	if strings.Count(text, "repo.inTenant(ctx, func(q *sqlcgen.Queries) error") != 1 {
		t.Fatalf("create submission, local outbox and draft cleanup must stay in one tenant transaction")
	}

	outboxHelper := functionSource(string(src), "createSubmissionJudgeOutbox")
	if !strings.Contains(outboxHelper, `fmt.Sprintf("teaching:%d:submission:%d"`) {
		t.Fatalf("submit assignment must store source_ref bound to the concrete submission id")
	}
	if strings.Contains(text, "submitJudge(ctx") {
		t.Fatalf("submit request path must not call M3 before the local transaction commits")
	}
}

// TestAutoJudgeSubmissionUsesRecoverableOutbox 确认自动判题提交通过 M6 自有 outbox 派发,避免跨模块副作用半提交。
func TestAutoJudgeSubmissionUsesRecoverableOutbox(t *testing.T) {
	src, err := os.ReadFile("assignment_service.go")
	if err != nil {
		t.Fatalf("read assignment service: %v", err)
	}
	submit := functionSource(string(src), "SubmitAssignment")
	if !strings.Contains(submit, "createSubmissionJudgeOutbox") {
		t.Fatalf("auto judge submit must persist a local outbox row in the submission transaction")
	}
	if strings.Contains(submit, "submitJudge(ctx") {
		t.Fatalf("submit request path must not call M3 before the local transaction commits")
	}
	for _, required := range []string{"DispatchPendingSubmissionJudges", "claimPendingSubmissionJudgeOutbox", "completeSubmissionJudgeOutbox", "failSubmissionJudgeOutbox"} {
		if !strings.Contains(string(src), required) {
			t.Fatalf("outbox dispatcher must provide recoverable judge dispatch step, missing %s", required)
		}
	}

	sql, err := os.ReadFile("../../../db/queries/teaching.sql")
	if err != nil {
		t.Fatalf("read teaching sql: %v", err)
	}
	queryText := string(sql)
	for _, required := range []string{
		"CreateSubmissionJudgeOutbox",
		"ListPendingSubmissionJudgeOutbox",
		"MarkSubmissionJudgeOutboxRunning",
		"CompleteSubmissionJudgeOutbox",
		"FailSubmissionJudgeOutbox",
	} {
		if !strings.Contains(queryText, required) {
			t.Fatalf("teaching SQL must include recoverable outbox query %s", required)
		}
	}
}

// TestJudgeEventHandlersWrapPersistenceFailures 确认事件处理持久化失败不会向外泄漏原始数据库错误。
func TestJudgeEventHandlersWrapPersistenceFailures(t *testing.T) {
	src, err := os.ReadFile("events.go")
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	for _, name := range []string{"HandleJudgeCompleted", "HandleJudgeFailed"} {
		text := functionSource(string(src), name)
		if !strings.Contains(text, "ErrSubmissionEventInvalid.WithCause(err)") {
			t.Fatalf("%s must wrap persistence failures with M6 event error code", name)
		}
	}
}

// TestComputeWeightedTotalUsesOverrideWhenPresent 确认总评计算会优先使用手动调整后的来源成绩。
func TestComputeWeightedTotalUsesOverrideWhenPresent(t *testing.T) {
	total, err := computeWeightedTotal([]WeightedScore{
		{Score: 80, Weight: 40},
		{Score: 90, OverrideScore: ptrFloat(95), Weight: 60},
	})
	if err != nil {
		t.Fatalf("valid weighted scores rejected: %v", err)
	}
	if total != 89 {
		t.Fatalf("unexpected weighted total: %v", total)
	}
}

// TestBuildGradeExportWorkbookProducesReadableXLSX 确认成绩导出使用真实 xlsx 工作簿,避免 CSV 或手写格式伪装成 Excel。
func TestBuildGradeExportWorkbookProducesReadableXLSX(t *testing.T) {
	override := 93.5
	data, err := buildGradeExportWorkbook([]CourseGradeDTO{
		{CourseID: "101", StudentID: "2001", AutoTotal: 88, OverrideTotal: &override, FinalTotal: 93.5, IsOverridden: true},
	})
	if err != nil {
		t.Fatalf("grade export should build workbook: %v", err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("grade export must be a readable xlsx file: %v", err)
	}
	defer func() {
		if err := book.Close(); err != nil {
			t.Fatalf("close workbook: %v", err)
		}
	}()
	rows, err := book.GetRows(gradeExportSheetName)
	if err != nil {
		t.Fatalf("read export sheet: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected header and one grade row, got %#v", rows)
	}
	wantHeader := []string{"课程ID", "学生ID", "自动成绩", "覆盖成绩", "最终成绩", "是否手动调整"}
	for i, want := range wantHeader {
		if rows[0][i] != want {
			t.Fatalf("header[%d] expected %q, got %q", i, want, rows[0][i])
		}
	}
	if rows[1][0] != "101" || rows[1][1] != "2001" || rows[1][4] != "93.5" || rows[1][5] != "是" {
		t.Fatalf("unexpected grade row: %#v", rows[1])
	}
}

// TestTeachingJSONBoundaryErrorsUseModuleCodes 确认 M6 使用统一 jsonx JSONB 边界时传入模块错误码,不复用平台通用请求错误。
func TestTeachingJSONBoundaryErrorsUseModuleCodes(t *testing.T) {
	invalid := map[string]any{"bad": make(chan int)}
	cases := []struct {
		name string
		run  func() error
		code string
	}{
		{name: "course schedule", run: func() error {
			_, err := jsonx.ObjectBytes(invalid, apperr.ErrCourseInvalid)
			return err
		}, code: apperr.ErrCourseInvalid.Code},
		{name: "lesson content", run: func() error {
			_, err := jsonx.ObjectBytes(invalid, apperr.ErrCourseInvalid)
			return err
		}, code: apperr.ErrCourseInvalid.Code},
		{name: "assignment late penalty", run: func() error {
			_, err := jsonx.ObjectBytes(invalid, apperr.ErrAssignmentInvalid)
			return err
		}, code: apperr.ErrAssignmentInvalid.Code},
		{name: "submission content", run: func() error {
			_, err := jsonx.ObjectBytes(invalid, apperr.ErrSubmissionInvalid)
			return err
		}, code: apperr.ErrSubmissionInvalid.Code},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil {
				t.Fatalf("expected JSON boundary error")
			}
			if ae, ok := apperr.As(err); !ok || ae.Code != tc.code {
				t.Fatalf("expected code %s, got %v", tc.code, err)
			}
		})
	}
}

// TestInviteCodeUsesCryptographicRandomness 守护邀请码生成必须复用平台 CSPRNG 随机凭证能力。
func TestInviteCodeUsesCryptographicRandomness(t *testing.T) {
	src, err := os.ReadFile("service_helpers.go")
	if err != nil {
		t.Fatalf("read service helpers: %v", err)
	}
	text := string(src)
	if strings.Contains(text, `"math/rand"`) || strings.Contains(text, "time.Now()") || strings.Contains(text, `"crypto/rand"`) {
		t.Fatalf("invite code generation must use platform crypto.RandomToken instead of local randomness")
	}
	if !strings.Contains(text, "crypto.RandomToken") {
		t.Fatalf("invite code generation must call crypto.RandomToken")
	}
}

// TestPublishGradeUpdatedRequiresConfiguredBus 确认 M6 成绩事件缺少总线时显式失败,避免 M11 聚合闭环静默丢失。
func TestPublishGradeUpdatedRequiresConfiguredBus(t *testing.T) {
	svc := &Service{}

	err := svc.publishGradeUpdated(t.Context(), 1001, 3001, 5001)
	if err == nil {
		t.Fatalf("expected missing event bus to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrGradeEventFailed.Code {
		t.Fatalf("expected grade event failed error, got %v", err)
	}
}

// TestCourseContentAccessDoesNotTreatSharedVisibilityAsMembership 确认共享课程库不等于课程内容访问授权。
func TestCourseContentAccessDoesNotTreatSharedVisibilityAsMembership(t *testing.T) {
	if canAccessCourseContent(false, 10, 20, CourseVisibilityShared, false) {
		t.Fatalf("non-member must not access shared course learning content")
	}
	if !canAccessCourseContent(false, 10, 10, CourseVisibilityPrivate, false) {
		t.Fatalf("course teacher should access course content")
	}
	if !canAccessCourseContent(false, 10, 20, CourseVisibilityPrivate, true) {
		t.Fatalf("course member should access course content")
	}
	if !canAccessCourseContent(true, 10, 20, CourseVisibilityPrivate, false) {
		t.Fatalf("platform context should access course content")
	}
}

// TestTeachingModerationUsesAtomicAuthorizedMutation 防止讨论和公告管理走“先读再转化再写”的重复路径。
func TestTeachingModerationUsesAtomicAuthorizedMutation(t *testing.T) {
	src, err := os.ReadFile("interaction_service.go")
	if err != nil {
		t.Fatalf("read interaction service: %v", err)
	}
	text := string(src)
	for _, forbidden := range []string{"loadDiscussionPost", "loadAnnouncement", "GetDiscussionPostByID", "GetAnnouncementByID"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("moderation must use atomic authorized SQL instead of helper %s", forbidden)
		}
	}
	for _, required := range []string{"TogglePostPinParams", "SoftDeletePostParams", "ToggleAnnouncementPinParams", "ActorID: id.AccountID", "IsPlatform: id.IsPlatform"} {
		if !strings.Contains(text, required) {
			t.Fatalf("moderation mutation must pass verified actor context, missing %s", required)
		}
	}

	sql, err := os.ReadFile("../../../db/queries/teaching.sql")
	if err != nil {
		t.Fatalf("read teaching sql: %v", err)
	}
	queryText := string(sql)
	for _, required := range []string{"FROM course c", "c.id = p.course_id", "c.id = a.course_id", "c.teacher_id = @actor_id", "@is_platform::boolean"} {
		if !strings.Contains(queryText, required) {
			t.Fatalf("moderation SQL must enforce teacher/platform ownership atomically, missing %s", required)
		}
	}
}

// TestTeachingInteractionWritesUseAtomicMembershipChecks 确认点赞和评价不绕过课程成员边界。
func TestTeachingInteractionWritesUseAtomicMembershipChecks(t *testing.T) {
	src, err := os.ReadFile("interaction_service.go")
	if err != nil {
		t.Fatalf("read interaction service: %v", err)
	}
	text := string(src)
	for _, required := range []string{"IncrementPostLikeParams", "UpsertCourseReviewParams", "ensureStudentCourseMember"} {
		if !strings.Contains(text, required) {
			t.Fatalf("interaction writes must use unified contracts and authorized SQL params, missing %s", required)
		}
	}
	helpers, err := os.ReadFile("service_helpers.go")
	if err != nil {
		t.Fatalf("read service helpers: %v", err)
	}
	if !strings.Contains(string(helpers), "contracts.RoleStudent") {
		t.Fatalf("student role matching must stay in the unified service helper")
	}

	sql, err := os.ReadFile("../../../db/queries/teaching.sql")
	if err != nil {
		t.Fatalf("read teaching sql: %v", err)
	}
	queryText := string(sql)
	for _, required := range []string{"course_member", "m.student_id = @actor_id", "m.student_id = @student_id"} {
		if !strings.Contains(queryText, required) {
			t.Fatalf("interaction write SQL must enforce course membership atomically, missing %s", required)
		}
	}
}

// TestDiscussionReplyParentMustBelongToSameCourse 确认回复不能挂到其他课程的帖子下。
func TestDiscussionReplyParentMustBelongToSameCourse(t *testing.T) {
	sql, err := os.ReadFile("../../../db/queries/teaching.sql")
	if err != nil {
		t.Fatalf("read teaching sql: %v", err)
	}
	text := string(sql)
	createPost := sqlQuerySource(text, "CreateDiscussionPost")
	for _, required := range []string{"WHERE", "parent_id", "course_id", "discussion_post parent"} {
		if !strings.Contains(createPost, required) {
			t.Fatalf("discussion reply parent must be validated in insert SQL, missing %s", required)
		}
	}
}

// TestTeachingStudentWorkflowsUseStudentMemberBoundary 防止学生侧写入和个人进度复用过宽的课程可访问权限。
func TestTeachingStudentWorkflowsUseStudentMemberBoundary(t *testing.T) {
	files := []string{"interaction_service.go", "assignment_service.go", "service_helpers.go"}
	combined := ""
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		combined += string(src)
	}
	for _, required := range []string{"ensureStudentRole", "contracts.RoleStudent", "ListLessonProgressByCourseAndStudent"} {
		if !strings.Contains(combined, required) {
			t.Fatalf("student workflow must use unified student/member boundary, missing %s", required)
		}
	}

	interaction, err := os.ReadFile("interaction_service.go")
	if err != nil {
		t.Fatalf("read interaction service: %v", err)
	}
	text := string(interaction)
	progressStats := functionSource(text, "ProgressStats")
	if !strings.Contains(progressStats, "ensureTeacherOfCourse") {
		t.Fatalf("class progress stats must be teacher/platform only")
	}
	myProgress := functionSource(text, "MyProgress")
	if strings.Contains(myProgress, "ProgressStats(") {
		t.Fatalf("student my-progress must not reuse class aggregate progress stats")
	}

	sql, err := os.ReadFile("../../../db/queries/teaching.sql")
	if err != nil {
		t.Fatalf("read teaching sql: %v", err)
	}
	queryText := string(sql)
	for _, required := range []string{"ListLessonProgressByCourseAndStudent", "course_member", "m.student_id = @student_id", "INSERT INTO submission_draft", "INSERT INTO submission"} {
		if !strings.Contains(queryText, required) {
			t.Fatalf("student workflow SQL must enforce course membership atomically, missing %s", required)
		}
	}
}

// TestSubmitAssignmentInvalidatesServerDraft 确认正式提交后服务端草稿失效。
func TestSubmitAssignmentInvalidatesServerDraft(t *testing.T) {
	src, err := os.ReadFile("assignment_service.go")
	if err != nil {
		t.Fatalf("read assignment service: %v", err)
	}
	text := functionSource(string(src), "SubmitAssignment")
	if !strings.Contains(text, "DeleteSubmissionDraft") {
		t.Fatalf("submit assignment must invalidate server draft after creating a submission")
	}

	sql, err := os.ReadFile("../../../db/queries/teaching.sql")
	if err != nil {
		t.Fatalf("read teaching sql: %v", err)
	}
	if !strings.Contains(string(sql), "DELETE FROM submission_draft") {
		t.Fatalf("teaching SQL must provide server draft invalidation query")
	}
}

// TestAssignmentDraftCanBeLoadedFromServer 确认作答页可读取服务端草稿用于跨设备恢复。
func TestAssignmentDraftCanBeLoadedFromServer(t *testing.T) {
	api, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api: %v", err)
	}
	apiText := string(api)
	for _, required := range []string{`g.GET("/assignments/:id/draft"`, "a.getDraft", "GetDraft"} {
		if !strings.Contains(apiText, required) {
			t.Fatalf("assignment draft must expose server read route, missing %s", required)
		}
	}

	service, err := os.ReadFile("assignment_service.go")
	if err != nil {
		t.Fatalf("read assignment service: %v", err)
	}
	serviceText := string(service)
	for _, required := range []string{"func (s *Service) GetDraft", "GetSubmissionDraft", "ensureStudentCourseMember"} {
		if !strings.Contains(serviceText, required) {
			t.Fatalf("assignment draft read must enforce student membership and load JSONB content, missing %s", required)
		}
	}

	sql, err := os.ReadFile("../../../db/queries/teaching.sql")
	if err != nil {
		t.Fatalf("read teaching sql: %v", err)
	}
	if !strings.Contains(string(sql), "-- name: GetSubmissionDraft :one") {
		t.Fatalf("teaching SQL must provide server draft read query")
	}
}

// TestStudentAssignmentWorkflowsRequirePublishedAssignment 确认学生侧作业详情和草稿入口不能访问教师草稿作业。
func TestStudentAssignmentWorkflowsRequirePublishedAssignment(t *testing.T) {
	src, err := os.ReadFile("assignment_service.go")
	if err != nil {
		t.Fatalf("read assignment service: %v", err)
	}
	text := string(src)
	getAssignment := functionSource(text, "GetAssignment")
	if !strings.Contains(getAssignment, "ensureAssignmentAccessible") {
		t.Fatalf("assignment detail must use assignment-level published/teacher boundary")
	}
	for _, name := range []string{"SaveDraft", "GetDraft", "SubmitAssignment"} {
		fn := functionSource(text, name)
		if !strings.Contains(fn, "ensurePublishedAssignment") {
			t.Fatalf("%s must reject unpublished assignments for student workflow", name)
		}
	}
}

// TestTeachingProductionCodeDoesNotExposePlatformInternalErrors 确认 M6 业务失败不会复用平台内部错误码。
func TestTeachingProductionCodeDoesNotExposePlatformInternalErrors(t *testing.T) {
	files := []string{
		"assignment_service.go",
		"course_service.go",
		"events.go",
		"grade_service.go",
		"interaction_service.go",
		"lesson_service.go",
		"service_helpers.go",
	}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if strings.Contains(string(src), "ErrInternal.WithCause") {
			t.Fatalf("%s must use M6-specific error codes instead of ErrInternal.WithCause", file)
		}
	}
}

// TestRecordAssignmentContentUsageCountsM5Refs 确认 M6 锁定 M5 题目版本后会上报引用计数,支撑 M5 有引用禁删。
func TestRecordAssignmentContentUsageCountsM5Refs(t *testing.T) {
	content := &teachingContentUsageFake{}
	svc := &Service{content: content}
	ctx := tenantTestContext(10, 501)
	items := []AssignmentItemInput{
		{ItemCode: "q1", ItemVersion: "1.0.0", Score: 10, GradingMode: GradingModeManual},
		{ItemCode: "q1", ItemVersion: "1.0.0", Score: 10, GradingMode: GradingModeManual},
	}

	if err := svc.recordAssignmentContentUsage(ctx, 10, items); err != nil {
		t.Fatalf("content usage record rejected: %v", err)
	}
	if len(content.counted) != 1 || content.counted[0].ItemCode != "q1" || content.counted[0].ItemVersion != "1.0.0" {
		t.Fatalf("expected assignment reference to increment M5 usage, got %#v", content.counted)
	}
}

// functionSource 返回指定方法的大致源码片段,用于守护高风险权限入口不退回旧路径。
func functionSource(src, name string) string {
	start := strings.Index(src, "func (s *Service) "+name+"(")
	if start < 0 {
		return ""
	}
	next := strings.Index(src[start+1:], "\nfunc (s *Service) ")
	if next < 0 {
		return src[start:]
	}
	return src[start : start+1+next]
}

// sqlQuerySource 返回指定 sqlc 查询的大致 SQL 片段。
func sqlQuerySource(src, name string) string {
	start := strings.Index(src, "-- name: "+name+" ")
	if start < 0 {
		return ""
	}
	next := strings.Index(src[start+1:], "\n-- name: ")
	if next < 0 {
		return src[start:]
	}
	return src[start : start+1+next]
}

// ptrFloat 返回 float64 指针,用于测试可选覆盖分。
func ptrFloat(v float64) *float64 { return &v }

// tenantTestContext 构造带租户身份的测试上下文。
func tenantTestContext(tenantID, accountID int64) context.Context {
	return tenant.WithContext(context.Background(), tenant.Identity{TenantID: tenantID, AccountID: accountID})
}

type teachingContentUsageFake struct {
	counted []contracts.ContentItemRef
}

func (f *teachingContentUsageFake) GetContentFace(context.Context, int64, contracts.ContentItemRef) (contracts.ContentItemSnapshot, error) {
	return contracts.ContentItemSnapshot{Status: 2}, nil
}

func (f *teachingContentUsageFake) GetContentFull(context.Context, int64, contracts.ContentItemRef) (contracts.ContentItemSnapshot, error) {
	return contracts.ContentItemSnapshot{}, nil
}

func (f *teachingContentUsageFake) BatchGetContentFace(context.Context, int64, []contracts.ContentItemRef) ([]contracts.ContentItemSnapshot, error) {
	return nil, nil
}

func (f *teachingContentUsageFake) IncrementContentUsage(_ context.Context, _ int64, ref contracts.ContentItemRef) error {
	f.counted = append(f.counted, ref)
	return nil
}
