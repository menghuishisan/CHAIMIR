// M11 API 路由测试:确认成绩中心接口注册、统一鉴权与角色边界。
package grade

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// TestGradeRoutesAreProtected 确认 M11 路径未登录时返回统一未登录错误。
func TestGradeRoutesAreProtected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	api := NewAPI(&fakeGradeAPIService{}, newTestAuth(), &fakeGradeIdentity{})
	api.Register(engine.Group("/api/v1"))

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/grade-center/level-configs", ""},
		{http.MethodGet, "/api/v1/grade-center/semesters", ""},
		{http.MethodGet, "/api/v1/grade-center/warning-rules", ""},
		{http.MethodGet, "/api/v1/grade-center/reviews", ""},
		{http.MethodPost, "/api/v1/grade-center/reviews", `{}`},
		{http.MethodPost, "/api/v1/grade-center/reviews/1/approve", `{}`},
		{http.MethodGet, "/api/v1/grade-center/courses/1/lock-status", ""},
		{http.MethodGet, "/api/v1/grade-center/students/1/grades", ""},
		{http.MethodGet, "/api/v1/grade-center/students/1/gpa", ""},
		{http.MethodPost, "/api/v1/grade-center/appeals", `{}`},
		{http.MethodGet, "/api/v1/grade-center/appeals", ""},
		{http.MethodPost, "/api/v1/grade-center/warnings/scan", `{}`},
		{http.MethodPost, "/api/v1/grade-center/warnings/1/ack", `{}`},
		{http.MethodPost, "/api/v1/grade-center/transcripts", `{}`},
		{http.MethodGet, "/api/v1/grade-center/transcripts/1", ""},
		{http.MethodPost, "/api/v1/grade-center/transcripts/batch", `{}`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		wantCode := `"code":"11001"`
		if gradeInternalRoute(tc.method, tc.path) {
			wantCode = `"code":"11008"`
		}
		if !strings.Contains(rec.Body.String(), wantCode) {
			t.Fatalf("%s %s expected %s response, got %s", tc.method, tc.path, wantCode, rec.Body.String())
		}
	}
}

func gradeInternalRoute(method, path string) bool {
	switch {
	case method == http.MethodGet && strings.Contains(path, "/lock-status"):
		return true
	case method == http.MethodPost && path == "/api/v1/grade-center/warnings/scan":
		return true
	default:
		return false
	}
}

// TestStudentCannotApproveReview 确认学生不能执行成绩审核。
func TestStudentCannotApproveReview(t *testing.T) {
	engine, authMgr, _ := newGradeRouteEngine([]string{"student"})
	token, err := authMgr.IssueAccess(10, 501, 9001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/grade-center/reviews/1/approve", strings.NewReader(`{"semester_id":"202601"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"code":"11002"`) {
		t.Fatalf("expected forbidden response, got %s", rec.Body.String())
	}
}

// TestSchoolAdminCanApproveReview 确认学校管理员可进入审核服务。
func TestSchoolAdminCanApproveReview(t *testing.T) {
	engine, authMgr, svc := newGradeRouteEngine([]string{"school_admin"})
	token, err := authMgr.IssueAccess(10, 501, 9001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/grade-center/reviews/1/approve", strings.NewReader(`{"comment":"通过","semester_id":"202601"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"code":"0"`) {
		t.Fatalf("expected ok response, got %s", rec.Body.String())
	}
	if svc.approvedReviewID != 1 {
		t.Fatalf("approve request was not passed to service")
	}
}

// TestInternalServiceCanRecomputeStudent 确认文档标注的 GPA 重算入口支持内部服务签名。
func TestInternalServiceCanRecomputeStudent(t *testing.T) {
	engine, _, svc := newGradeRouteEngine([]string{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/grade-center/students/501/recompute", strings.NewReader(`{"course_id":"3001","semester_id":"202601"}`))
	req.Header.Set("Content-Type", "application/json")
	signGradeServiceRequest(req, "grade", "10", "grade:2026:appeal:1", "trace-1", currentGradeServiceTimestamp())
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), `"code":"11001"`) || strings.Contains(rec.Body.String(), `"code":"11008"`) || strings.Contains(rec.Body.String(), `"code":"11002"`) {
		t.Fatalf("service-signed recompute route should pass auth boundary, got %s", rec.Body.String())
	}
	if svc.recomputedStudentID != 501 {
		t.Fatalf("recompute request was not passed to service, got %d", svc.recomputedStudentID)
	}
}

// TestTranscriptResponsesDoNotExposeObjectStorageRef 确认成绩单生成响应不暴露服务端对象存储 key。
func TestTranscriptResponsesDoNotExposeObjectStorageRef(t *testing.T) {
	body, err := json.Marshal(TranscriptDTO{ID: "9001", StudentID: "5001", Scope: TranscriptScopeAll, PDFRef: "1001/transcript/record/9001.pdf"})
	if err != nil {
		t.Fatalf("marshal transcript dto: %v", err)
	}
	if strings.Contains(string(body), "pdf_ref") || strings.Contains(string(body), "1001/transcript/record/9001.pdf") {
		t.Fatalf("transcript response must not expose object storage ref: %s", string(body))
	}
}

// newGradeRouteEngine 构造带 M11 路由的测试 Gin 引擎。
func newGradeRouteEngine(roles []string) (*gin.Engine, *auth.Manager, *fakeGradeAPIService) {
	gin.SetMode(gin.TestMode)
	authMgr := newTestAuth()
	engine := gin.New()
	engine.Use(response.TraceMiddleware())
	svc := &fakeGradeAPIService{}
	api := NewAPI(svc, authMgr, &fakeGradeIdentity{roles: roles})
	api.Register(engine.Group("/api/v1"))
	return engine, authMgr, svc
}

// newTestAuth 构造测试用 JWT 管理器。
func newTestAuth() *auth.Manager {
	return auth.NewManager(config.AuthConfig{
		JWTSigningKey:             "test-signing-key",
		AccessTTLMin:              15,
		JWTIssuer:                 "chaimir-test",
		HMACKey:                   "test-service-hmac-key",
		ServiceAuthMaxSkewSeconds: 300,
	})
}

type fakeGradeAPIService struct {
	approvedReviewID    int64
	recomputedStudentID int64
}

func (f *fakeGradeAPIService) ListLevelConfigs(context.Context) ([]LevelConfigDTO, error) {
	return nil, nil
}
func (f *fakeGradeAPIService) CreateLevelConfig(context.Context, LevelConfigRequest) (LevelConfigDTO, error) {
	return LevelConfigDTO{}, nil
}
func (f *fakeGradeAPIService) UpdateLevelConfig(context.Context, int64, LevelConfigRequest) (LevelConfigDTO, error) {
	return LevelConfigDTO{}, nil
}
func (f *fakeGradeAPIService) ListSemesters(context.Context) ([]SemesterDTO, error) { return nil, nil }
func (f *fakeGradeAPIService) CreateSemester(context.Context, SemesterRequest) (SemesterDTO, error) {
	return SemesterDTO{}, nil
}
func (f *fakeGradeAPIService) WarningRules(context.Context) (WarningRuleDTO, error) {
	return WarningRuleDTO{}, nil
}
func (f *fakeGradeAPIService) UpdateWarningRules(context.Context, WarningRuleDTO) (WarningRuleDTO, error) {
	return WarningRuleDTO{}, nil
}
func (f *fakeGradeAPIService) SubmitReview(context.Context, ReviewCreateRequest) (ReviewDTO, error) {
	return ReviewDTO{}, nil
}
func (f *fakeGradeAPIService) ListReviews(context.Context, int16, int, int) ([]ReviewDTO, int64, error) {
	return nil, 0, nil
}
func (f *fakeGradeAPIService) CourseLockStatus(context.Context, int64) (ReviewDTO, error) {
	return ReviewDTO{}, nil
}
func (f *fakeGradeAPIService) ApproveReview(_ context.Context, id int64, _ ReviewDecisionRequest) (ReviewDTO, error) {
	f.approvedReviewID = id
	return ReviewDTO{ID: ids.Format(id), Status: ReviewStatusApproved, IsLocked: true}, nil
}
func (f *fakeGradeAPIService) RejectReview(context.Context, int64, ReviewDecisionRequest) (ReviewDTO, error) {
	return ReviewDTO{}, nil
}
func (f *fakeGradeAPIService) UnlockReview(context.Context, int64, ReviewDecisionRequest) (ReviewDTO, error) {
	return ReviewDTO{}, nil
}
func (f *fakeGradeAPIService) StudentGrades(context.Context, int64, int64) (StudentGradesDTO, error) {
	return StudentGradesDTO{}, nil
}
func (f *fakeGradeAPIService) StudentGPA(context.Context, int64) ([]SemesterGradeDTO, error) {
	return nil, nil
}
func (f *fakeGradeAPIService) RecomputeStudent(_ context.Context, id int64, _ RecomputeRequest) (SemesterGradeDTO, error) {
	f.recomputedStudentID = id
	return SemesterGradeDTO{}, nil
}
func (f *fakeGradeAPIService) CreateAppeal(context.Context, AppealCreateRequest) (AppealDTO, error) {
	return AppealDTO{}, nil
}
func (f *fakeGradeAPIService) AcceptAppeal(context.Context, int64, AppealHandleRequest) (AppealDTO, error) {
	return AppealDTO{}, nil
}
func (f *fakeGradeAPIService) RejectAppeal(context.Context, int64, AppealHandleRequest) (AppealDTO, error) {
	return AppealDTO{}, nil
}
func (f *fakeGradeAPIService) ListAppeals(context.Context, int16, int, int) ([]AppealDTO, int64, error) {
	return nil, 0, nil
}
func (f *fakeGradeAPIService) ScanWarnings(context.Context, WarningScanRequest) ([]WarningDTO, error) {
	return nil, nil
}
func (f *fakeGradeAPIService) ListWarnings(context.Context, int64, int64, int16, int, int) ([]WarningDTO, int64, error) {
	return nil, 0, nil
}
func (f *fakeGradeAPIService) AcknowledgeWarning(context.Context, int64) (WarningDTO, error) {
	return WarningDTO{}, nil
}
func (f *fakeGradeAPIService) GenerateTranscript(context.Context, TranscriptRequest) (TranscriptDTO, error) {
	return TranscriptDTO{}, nil
}
func (f *fakeGradeAPIService) GetTranscript(context.Context, int64) (TranscriptDTO, error) {
	return TranscriptDTO{}, nil
}
func (f *fakeGradeAPIService) DownloadTranscript(context.Context, int64) (TranscriptDTO, io.ReadCloser, error) {
	return TranscriptDTO{ID: "1"}, io.NopCloser(strings.NewReader("%PDF-1.4")), nil
}
func (f *fakeGradeAPIService) BatchGenerateTranscripts(context.Context, TranscriptBatchRequest) ([]TranscriptDTO, error) {
	return nil, nil
}

type fakeGradeIdentity struct{ roles []string }

func (f *fakeGradeIdentity) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return contracts.AccountInfo{Roles: f.roles}, nil
}
func (f *fakeGradeIdentity) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}
func (f *fakeGradeIdentity) HasRole(_ context.Context, _ int64, role string) (bool, error) {
	for _, actual := range f.roles {
		if actual == role {
			return true, nil
		}
	}
	return false, nil
}

func signGradeServiceRequest(req *http.Request, service, tenantID, sourceRef, traceID, timestamp string) {
	req.Header.Set(auth.ServiceNameHeader, service)
	req.Header.Set(auth.ServiceTenantHeader, tenantID)
	req.Header.Set(auth.ServiceSourceRefHeader, sourceRef)
	req.Header.Set(auth.ServiceTimestampHeader, timestamp)
	req.Header.Set("X-Trace-Id", traceID)
	req.Header.Set(auth.ServiceSignatureHeader, gradeServiceSignature(req.Method, req.URL.EscapedPath(), tenantID, sourceRef, timestamp, traceID))
}

func gradeServiceSignature(method, path, tenantID, sourceRef, timestamp, traceID string) string {
	mac := hmac.New(sha256.New, []byte("test-service-hmac-key"))
	mac.Write([]byte(method + "\n" + path + "\n" + tenantID + "\n" + sourceRef + "\n" + timestamp + "\n" + traceID))
	return hex.EncodeToString(mac.Sum(nil))
}

// currentGradeServiceTimestamp 返回内部服务签名测试使用的当前 UTC 秒。
func currentGradeServiceTimestamp() string {
	return strconv.FormatInt(timex.Now().Unix(), 10)
}
