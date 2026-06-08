// M3 API 路由注册测试:确认评测引擎控制面接口进入鉴权链。
package judge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// TestJudgeRoutesAreProtected 确认 M3 文档中的 HTTP 路由已注册且需要登录。
func TestJudgeRoutesAreProtected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(response.TraceMiddleware())
	api := NewAPI(nil, auth.NewManager(config.AuthConfig{
		JWTSigningKey: "test-signing-key",
		AccessTTLMin:  15,
		JWTIssuer:     "chaimir-test",
		HMACKey:       "test-service-hmac-key",
	}), nil)
	api.Register(engine.Group("/api/v1"))

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/judge/judgers", ""},
		{http.MethodPost, "/api/v1/judge/judgers", `{}`},
		{http.MethodPatch, "/api/v1/judge/judgers/1", `{}`},
		{http.MethodPost, "/api/v1/judge/judgers/1/selftest", `{}`},
		{http.MethodPost, "/api/v1/judge/tasks", `{}`},
		{http.MethodGet, "/api/v1/judge/tasks/1", ""},
		{http.MethodGet, "/api/v1/judge/tasks/1/progress", ""},
		{http.MethodDelete, "/api/v1/judge/tasks/1", ""},
		{http.MethodPost, "/api/v1/judge/tasks/1/rejudge", `{}`},
		{http.MethodPost, "/api/v1/judge/rejudge/batch", `{}`},
		{http.MethodGet, "/api/v1/judge/tasks?source_ref=experiment:2026:instance:55&pending_manual=true", ""},
		{http.MethodPost, "/api/v1/judge/tasks/1/manual-score", `{}`},
		{http.MethodGet, "/api/v1/judge/fingerprints/exact?problem_ref=p:1&code_hash=h", ""},
		{http.MethodPost, "/api/v1/judge/fingerprints/similarity", `{}`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s expected unified status 200, got %d", tc.method, tc.path, rec.Code)
		}
		wantCode := `"code":"11001"`
		if judgeInternalRoute(tc.method, tc.path) {
			wantCode = `"code":"11008"`
		}
		if !strings.Contains(rec.Body.String(), wantCode) {
			t.Fatalf("%s %s expected %s response, got %s", tc.method, tc.path, wantCode, rec.Body.String())
		}
	}
}

// TestStudentCannotListManualPendingTasks 确认 J6 待人工评分列表只对教师/学校管理员开放。
func TestStudentCannotListManualPendingTasks(t *testing.T) {
	engine, authMgr := newJudgeRouteEngine([]string{"student"})
	token, err := authMgr.IssueAccess(1001, 2001, 3001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/judge/tasks?pending_manual=true", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected unified status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"11002"`) {
		t.Fatalf("student manual pending list must be forbidden, got %s", rec.Body.String())
	}
}

// TestJudgeAPIReturnsModuleSpecificValidationCodes 确认 M3 HTTP 边界不把业务校验折叠为通用 BadRequest。
func TestJudgeAPIReturnsModuleSpecificValidationCodes(t *testing.T) {
	engine, authMgr := newJudgeRouteEngine([]string{"teacher"})
	teacherToken, err := authMgr.IssueAccess(1001, 2001, 3001, false)
	if err != nil {
		t.Fatalf("issue teacher token: %v", err)
	}
	platformToken, err := authMgr.IssueAccess(0, 9001, 3001, true)
	if err != nil {
		t.Fatalf("issue platform token: %v", err)
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		token  string
		code   string
	}{
		{name: "judger path id", method: http.MethodPost, path: "/api/v1/judge/judgers/bad/selftest", token: platformToken, code: `"code":"31003"`},
		{name: "judger body", method: http.MethodPost, path: "/api/v1/judge/judgers", body: `{`, token: platformToken, code: `"code":"31003"`},
		{name: "task path id", method: http.MethodPost, path: "/api/v1/judge/tasks/bad/rejudge", token: teacherToken, code: `"code":"32002"`},
		{name: "manual score body", method: http.MethodPost, path: "/api/v1/judge/tasks/8801/manual-score", body: `{`, token: teacherToken, code: `"code":"32008"`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+tc.token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s expected unified status 200, got %d", tc.name, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), tc.code) {
			t.Fatalf("%s expected %s response, got %s", tc.name, tc.code, rec.Body.String())
		}
	}
}

// TestTaskReaderAuthorizationUsesSubmitterOrTeacherRole 确认任务结果/进度读取不只依赖租户边界。
func TestTaskReaderAuthorizationUsesSubmitterOrTeacherRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	studentAPI := &API{identity: &judgeRouteIdentity{roles: []string{"student"}}}
	studentCtx, _ := judgeTestContext(tenantContext(1001, 2001, false))
	if !studentAPI.authorizeTaskReader(studentCtx, contracts.JudgeTaskInfo{TenantID: 1001, SubmitterID: 2001}) {
		t.Fatal("submitter must read own judge task")
	}

	otherCtx, otherRec := judgeTestContext(tenantContext(1001, 2002, false))
	if studentAPI.authorizeTaskReader(otherCtx, contracts.JudgeTaskInfo{TenantID: 1001, SubmitterID: 2001}) {
		t.Fatal("student must not read another submitter's judge task")
	}
	if !strings.Contains(otherRec.Body.String(), `"code":"11002"`) {
		t.Fatalf("cross-submitter read must return forbidden, got %s", otherRec.Body.String())
	}

	teacherAPI := &API{identity: &judgeRouteIdentity{roles: []string{"teacher"}}}
	teacherCtx, _ := judgeTestContext(tenantContext(1001, 3001, false))
	if !teacherAPI.authorizeTaskReader(teacherCtx, contracts.JudgeTaskInfo{TenantID: 1001, SubmitterID: 2001}) {
		t.Fatal("teacher must read tenant judge tasks for grading")
	}
}

func judgeInternalRoute(method, path string) bool {
	switch {
	case method == http.MethodPost && path == "/api/v1/judge/tasks":
		return true
	case method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/judge/tasks/"):
		return true
	case method == http.MethodPost && path == "/api/v1/judge/rejudge/batch":
		return true
	case strings.Contains(path, "/api/v1/judge/fingerprints/"):
		return true
	default:
		return false
	}
}

func tenantContext(tenantID, accountID int64, isPlatform bool) context.Context {
	return tenant.WithContext(context.Background(), tenant.Identity{TenantID: tenantID, AccountID: accountID, IsPlatform: isPlatform})
}

func judgeTestContext(ctx context.Context) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/judge/tasks/1", nil).WithContext(ctx)
	return c, rec
}

// newJudgeRouteEngine 构造带指定角色的 M3 路由测试环境。
func newJudgeRouteEngine(roles []string) (*gin.Engine, *auth.Manager) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(response.TraceMiddleware(), gin.Recovery())
	authMgr := auth.NewManager(config.AuthConfig{
		JWTSigningKey: "test-signing-key",
		AccessTTLMin:  15,
		JWTIssuer:     "chaimir-test",
		HMACKey:       "test-service-hmac-key",
	})
	api := NewAPI(nil, authMgr, &judgeRouteIdentity{roles: roles})
	api.Register(engine.Group("/api/v1"))
	return engine, authMgr
}

type judgeRouteIdentity struct {
	roles []string
}

func (f *judgeRouteIdentity) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return contracts.AccountInfo{Roles: f.roles}, nil
}

func (f *judgeRouteIdentity) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}

func (f *judgeRouteIdentity) HasRole(_ context.Context, _ int64, role string) (bool, error) {
	for _, actual := range f.roles {
		if actual == role {
			return true, nil
		}
	}
	return false, nil
}
