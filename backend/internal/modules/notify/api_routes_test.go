// M10 API 路由测试:确认通知、公告和实时入口按文档路径注册并执行鉴权边界。
package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// TestNotifyRoutesAreProtected 确认 M10 用户与内部 HTTP 路径未登录时返回统一未登录错误。
func TestNotifyRoutesAreProtected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(response.TraceMiddleware())
	api := NewAPI(&fakeNotifyAPIService{}, newTestAuth(), &fakeNotifyIdentity{}, nil)
	api.Register(engine.Group("/api/v1"))

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/notify/send", `{}`},
		{http.MethodPost, "/api/v1/notify/push", `{}`},
		{http.MethodGet, "/api/v1/notify/inbox", ""},
		{http.MethodGet, "/api/v1/notify/inbox/unread-count", ""},
		{http.MethodPost, "/api/v1/notify/inbox/1/read", `{}`},
		{http.MethodPost, "/api/v1/notify/inbox/read-all", `{}`},
		{http.MethodDelete, "/api/v1/notify/inbox/1", ""},
		{http.MethodGet, "/api/v1/notify/preferences", ""},
		{http.MethodPut, "/api/v1/notify/preferences", `[]`},
		{http.MethodPost, "/api/v1/notify/announcements", `{}`},
		{http.MethodGet, "/api/v1/notify/announcements", ""},
		{http.MethodPost, "/api/v1/notify/announcements/1/read", `{}`},
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
		if tc.path == "/api/v1/notify/send" || tc.path == "/api/v1/notify/push" {
			wantCode = `"code":"11008"`
		}
		if !strings.Contains(rec.Body.String(), wantCode) {
			t.Fatalf("%s %s expected %s response, got %s", tc.method, tc.path, wantCode, rec.Body.String())
		}
	}
}

// TestNotifySendRejectsOrdinaryUserToken 确认普通登录态不能调用内部发送接口。
func TestNotifySendRejectsOrdinaryUserToken(t *testing.T) {
	engine, authMgr, svc := newNotifyRouteEngine([]string{"teacher"})
	token, err := authMgr.IssueAccess(10, 501, 9001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	body := `{"tenant_id":10,"type":"assignment.due","receivers":[1001],"params":{"course":"区块链基础"},"link":"/x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/send", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"code":"11008"`) {
		t.Fatalf("expected service auth failure, got %s", rec.Body.String())
	}
	if svc.sent.Type != "" {
		t.Fatalf("ordinary user token should not call service: %#v", svc.sent)
	}
}

// TestNotifySendRouteCallsService 确认内部发送接口只接受服务签名并调用统一 Send 服务。
func TestNotifySendRouteCallsService(t *testing.T) {
	engine, _, svc := newNotifyRouteEngine([]string{"teacher"})
	body := `{"tenant_id":10,"type":"assignment.due","receivers":[1001],"params":{"course":"区块链基础"},"link":"/x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/send", strings.NewReader(body))
	signNotifyServiceRequest(req, "judge", "10", "experiment:2026:instance:55", "trace-1", currentNotifyServiceTimestamp())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"code":"0"`) {
		t.Fatalf("expected ok response, got %s", rec.Body.String())
	}
	if svc.sent.Type != "assignment.due" || len(svc.sent.Receivers) != 1 {
		t.Fatalf("send request was not passed to service: %#v", svc.sent)
	}
}

// TestSchoolAdminCanPublishAnnouncement 确认学校管理员可以发布学校公告。
func TestSchoolAdminCanPublishAnnouncement(t *testing.T) {
	engine, authMgr, svc := newNotifyRouteEngine([]string{"school_admin"})
	token, err := authMgr.IssueAccess(10, 501, 9001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/announcements", strings.NewReader(`{"title":"维护","content":"今晚维护","scope":2}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"code":"0"`) {
		t.Fatalf("expected ok response, got %s", rec.Body.String())
	}
	if svc.announcement.Title != "维护" {
		t.Fatalf("announcement request not passed to service: %#v", svc.announcement)
	}
}

// TestStudentCannotPublishAnnouncement 确认普通学生不能伪造系统公告。
func TestStudentCannotPublishAnnouncement(t *testing.T) {
	engine, authMgr, _ := newNotifyRouteEngine([]string{"student"})
	token, err := authMgr.IssueAccess(10, 501, 9001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/announcements", strings.NewReader(`{"title":"维护","content":"今晚维护","scope":2}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"code":"11002"`) {
		t.Fatalf("expected forbidden response, got %s", rec.Body.String())
	}
}

// TestPlatformAdminCannotPublishTenantAnnouncement 确认平台管理员不能发布缺少租户归属的本校/角色公告。
func TestPlatformAdminCannotPublishTenantAnnouncement(t *testing.T) {
	engine, authMgr, svc := newNotifyRouteEngine([]string{})
	token, err := authMgr.IssueAccess(0, 501, 9001, true)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/announcements", strings.NewReader(`{"title":"维护","content":"今晚维护","scope":2}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"code":"A0006"`) {
		t.Fatalf("expected announcement invalid response, got %s", rec.Body.String())
	}
	if svc.announcement.Title != "" {
		t.Fatalf("invalid tenant announcement should not reach service: %#v", svc.announcement)
	}
}

// TestPlatformAdminCanPublishPlatformAnnouncement 确认平台管理员只能发布全平台公告。
func TestPlatformAdminCanPublishPlatformAnnouncement(t *testing.T) {
	engine, authMgr, svc := newNotifyRouteEngine([]string{})
	token, err := authMgr.IssueAccess(0, 501, 9001, true)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/announcements", strings.NewReader(`{"title":"维护","content":"今晚维护","scope":1}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"code":"0"`) {
		t.Fatalf("expected ok response, got %s", rec.Body.String())
	}
	if svc.announcement.Scope != AnnouncementScopePlatform {
		t.Fatalf("platform announcement not passed to service: %#v", svc.announcement)
	}
}

// TestNotifyAPIBoundariesUseModuleCodesAndRoleContracts 防止 M10 API 回退通用错误码或硬编码角色。
func TestNotifyAPIBoundariesUseModuleCodesAndRoleContracts(t *testing.T) {
	apiSrc, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api: %v", err)
	}
	text := string(apiSrc)
	if strings.Contains(text, "ErrBadRequest") {
		t.Fatalf("M10 API must use notify module error codes instead of ErrBadRequest")
	}
	if strings.Contains(text, `"school_admin"`) || strings.Contains(text, `"teacher"`) || strings.Contains(text, `"student"`) {
		t.Fatalf("M10 API role handling must use contracts role constants")
	}
	for _, required := range []string{"contracts.RoleSchoolAdmin", "contracts.RoleNumber", "ErrNotifyInboxQueryInvalid"} {
		if !strings.Contains(text, required) {
			t.Fatalf("M10 API boundary missing %s", required)
		}
	}
	codes, err := os.ReadFile("../../../pkg/apperr/notify_codes.go")
	if err != nil {
		t.Fatalf("read notify codes: %v", err)
	}
	if !strings.Contains(string(codes), "ErrNotifyInboxQueryInvalid") {
		t.Fatalf("M10 error code registry must include inbox query invalid code")
	}
}

// TestNotifyInvalidInboxQueryReturnsPreciseCode 确认站内信查询参数错误使用 M10 专属错误码。
func TestNotifyInvalidInboxQueryReturnsPreciseCode(t *testing.T) {
	engine, authMgr, _ := newNotifyRouteEngine([]string{contracts.RoleStudent})
	token, err := authMgr.IssueAccess(10, 501, 9001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notify/inbox?is_read=maybe", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"code":"A0012"`) {
		t.Fatalf("expected inbox query error code, got %s", rec.Body.String())
	}
}

// TestListAnnouncementsFailsWhenRoleLookupFails 确认公告角色过滤不能在身份查询失败时静默当成无角色。
func TestListAnnouncementsFailsWhenRoleLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authMgr := newTestAuth()
	engine := gin.New()
	engine.Use(response.TraceMiddleware())
	svc := &fakeNotifyAPIService{}
	api := NewAPI(svc, authMgr, &fakeNotifyIdentity{err: errors.New("identity unavailable")}, nil)
	api.Register(engine.Group("/api/v1"))
	token, err := authMgr.IssueAccess(10, 501, 9001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notify/announcements", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"code":"11002"`) {
		t.Fatalf("expected forbidden response when role lookup fails, got %s", rec.Body.String())
	}
	if svc.listAnnouncementsCalled {
		t.Fatalf("announcement query must not run with empty roles after identity lookup failure")
	}
}

// TestNotifyRepoUsesPlatformNoRows 守护 M10 数据访问未命中判断统一走 platform/db.IsNoRows。
func TestNotifyRepoUsesPlatformNoRows(t *testing.T) {
	data, err := os.ReadFile("repo.go")
	if err != nil {
		t.Fatalf("read repo: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "pgx.ErrNoRows") || strings.Contains(text, "errors.Is(") {
		t.Fatalf("notify repo must use platform db.IsNoRows instead of direct pgx no rows checks")
	}
}

// TestNotifyRepoBatchCreateKeepsSingleTenantTransaction 守护批量站内信写入必须保持单事务原子语义。
func TestNotifyRepoBatchCreateKeepsSingleTenantTransaction(t *testing.T) {
	data, err := os.ReadFile("repo.go")
	if err != nil {
		t.Fatalf("read repo: %v", err)
	}
	text := string(data)
	start := strings.Index(text, "func (r *repo) CreateNotifications")
	if start < 0 {
		t.Fatalf("notify repo must expose CreateNotifications")
	}
	end := strings.Index(text[start:], "\n// ListInbox")
	if end < 0 {
		t.Fatalf("CreateNotifications must stay before ListInbox with clear file responsibility")
	}
	body := text[start : start+end]
	if strings.Count(body, "r.inTenant(ctx, tenantID") != 1 {
		t.Fatalf("CreateNotifications must use one tenant transaction for the whole batch")
	}
	if strings.Contains(body, "CreateNotification(ctx") && !strings.Contains(body, "for _, row := range rows") {
		t.Fatalf("CreateNotifications must write the supplied batch, not a single row API path")
	}
}

// newNotifyRouteEngine 构造带 M10 路由的测试 Gin 引擎。
func newNotifyRouteEngine(roles []string) (*gin.Engine, *auth.Manager, *fakeNotifyAPIService) {
	gin.SetMode(gin.TestMode)
	authMgr := newTestAuth()
	engine := gin.New()
	engine.Use(response.TraceMiddleware())
	svc := &fakeNotifyAPIService{}
	api := NewAPI(svc, authMgr, &fakeNotifyIdentity{roles: roles}, nil)
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

func signNotifyServiceRequest(req *http.Request, service, tenantID, sourceRef, traceID, timestamp string) {
	req.Header.Set(auth.ServiceNameHeader, service)
	req.Header.Set(auth.ServiceTenantHeader, tenantID)
	req.Header.Set(auth.ServiceSourceRefHeader, sourceRef)
	req.Header.Set(auth.ServiceTimestampHeader, timestamp)
	req.Header.Set(response.TraceHeader, traceID)
	req.Header.Set(auth.ServiceSignatureHeader, notifyServiceSignature(req.Method, req.URL.EscapedPath(), tenantID, sourceRef, timestamp, traceID))
}

func notifyServiceSignature(method, path, tenantID, sourceRef, timestamp, traceID string) string {
	mac := hmac.New(sha256.New, []byte("test-service-hmac-key"))
	mac.Write([]byte(method + "\n" + path + "\n" + tenantID + "\n" + sourceRef + "\n" + timestamp + "\n" + traceID))
	return hex.EncodeToString(mac.Sum(nil))
}

// currentNotifyServiceTimestamp 返回内部服务签名测试使用的当前 UTC 秒。
func currentNotifyServiceTimestamp() string {
	return strconv.FormatInt(timex.Now().Unix(), 10)
}

type fakeNotifyAPIService struct {
	sent                    contracts.NotifySendRequest
	pushed                  contracts.NotifyPushRequest
	announcement            AnnouncementRequest
	listAnnouncementsCalled bool
}

func (f *fakeNotifyAPIService) Send(_ context.Context, req contracts.NotifySendRequest) error {
	f.sent = req
	return nil
}

func (f *fakeNotifyAPIService) Push(_ context.Context, req contracts.NotifyPushRequest) error {
	f.pushed = req
	return nil
}

func (f *fakeNotifyAPIService) ListInbox(context.Context, InboxQuery) ([]NotificationDTO, int64, error) {
	return nil, 0, nil
}

func (f *fakeNotifyAPIService) UnreadCount(context.Context) (int64, error) {
	return 0, nil
}

func (f *fakeNotifyAPIService) MarkNotificationRead(context.Context, int64) error {
	return nil
}

func (f *fakeNotifyAPIService) MarkAllNotificationsRead(context.Context) error {
	return nil
}

func (f *fakeNotifyAPIService) DeleteNotification(context.Context, int64) error {
	return nil
}

func (f *fakeNotifyAPIService) ListPreferences(context.Context) ([]PreferenceDTO, error) {
	return nil, nil
}

func (f *fakeNotifyAPIService) UpdatePreferences(context.Context, []PreferenceRequest) error {
	return nil
}

func (f *fakeNotifyAPIService) CreateAnnouncement(_ context.Context, req AnnouncementRequest) (AnnouncementDTO, error) {
	f.announcement = req
	return AnnouncementDTO{ID: "7001", Title: req.Title, Content: req.Content, Scope: req.Scope}, nil
}

func (f *fakeNotifyAPIService) ListAnnouncements(context.Context, []int16) ([]AnnouncementDTO, error) {
	f.listAnnouncementsCalled = true
	return nil, nil
}

func (f *fakeNotifyAPIService) MarkAnnouncementRead(context.Context, int64) error {
	return nil
}

type fakeNotifyIdentity struct {
	roles []string
	err   error
}

func (f *fakeNotifyIdentity) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	if f.err != nil {
		return contracts.AccountInfo{}, f.err
	}
	return contracts.AccountInfo{Roles: f.roles}, nil
}

func (f *fakeNotifyIdentity) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}

func (f *fakeNotifyIdentity) HasRole(_ context.Context, _ int64, role string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	for _, actual := range f.roles {
		if actual == role {
			return true, nil
		}
	}
	return false, nil
}
