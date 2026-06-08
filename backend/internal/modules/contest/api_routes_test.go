// M8 API 路由测试:确认竞赛接口按文档路径注册并进入统一鉴权链。
package contest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// TestContestRoutesAreProtected 确认 M8 用户路径未登录时返回统一未登录错误。
func TestContestRoutesAreProtected(t *testing.T) {
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
		{http.MethodGet, "/api/v1/contest/contests?status=1", ""},
		{http.MethodPost, "/api/v1/contest/contests", `{}`},
		{http.MethodPatch, "/api/v1/contest/contests/1", `{}`},
		{http.MethodPost, "/api/v1/contest/contests/1/problems", `{}`},
		{http.MethodPost, "/api/v1/contest/contests/1/publish", `{}`},
		{http.MethodPost, "/api/v1/contest/contests/1/start", `{}`},
		{http.MethodPost, "/api/v1/contest/contests/1/end", `{}`},
		{http.MethodPost, "/api/v1/contest/contests/1/archive", `{}`},
		{http.MethodPost, "/api/v1/contest/contests/1/signup", `{}`},
		{http.MethodPost, "/api/v1/contest/teams/1/join", `{}`},
		{http.MethodGet, "/api/v1/contest/teams/1", ""},
		{http.MethodPost, "/api/v1/contest/teams/1/lock", `{}`},
		{http.MethodGet, "/api/v1/contest/contests/1/problems", ""},
		{http.MethodPost, "/api/v1/contest/contests/1/problems/2/env", `{}`},
		{http.MethodPost, "/api/v1/contest/contests/1/problems/2/submit", `{}`},
		{http.MethodGet, "/api/v1/contest/submissions/1", ""},
		{http.MethodPost, "/api/v1/contest/contests/1/battle/entry", `{}`},
		{http.MethodGet, "/api/v1/contest/contests/1/battle/entries", ""},
		{http.MethodGet, "/api/v1/contest/contests/1/battle/matches?team_id=2", ""},
		{http.MethodGet, "/api/v1/contest/matches/1/replay", ""},
		{http.MethodGet, "/api/v1/contest/contests/1/ladder", ""},
		{http.MethodGet, "/api/v1/contest/my/contest-records", ""},
		{http.MethodGet, "/api/v1/contest/contests/1/result-snapshot", ""},
		{http.MethodGet, "/api/v1/contest/contests/1/cheat-suspects", ""},
		{http.MethodPost, "/api/v1/contest/contests/1/cheat-records", `{}`},
		{http.MethodGet, "/api/v1/contest/vuln-sources", ""},
		{http.MethodPost, "/api/v1/contest/vuln-sources", `{}`},
		{http.MethodPost, "/api/v1/contest/vuln-sources/1/sync", `{}`},
		{http.MethodPost, "/api/v1/contest/vuln-sources/import", `{}`},
		{http.MethodPost, "/api/v1/contest/vuln-problems/1/prevalidate", `{}`},
		{http.MethodPost, "/api/v1/contest/vuln-problems/1/finalize", `{}`},
		{http.MethodGet, "/api/v1/contest/internal/stats?tenant_id=1", ""},
		{http.MethodGet, "/api/v1/contest/students/1/contest-achievements", ""},
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
		if contestInternalRoute(tc.path) {
			wantCode = `"code":"11008"`
		}
		if !strings.Contains(rec.Body.String(), wantCode) {
			t.Fatalf("%s %s expected %s response, got %s", tc.method, tc.path, wantCode, rec.Body.String())
		}
	}
}

func contestInternalRoute(path string) bool {
	return strings.Contains(path, "/internal/stats") || strings.Contains(path, "/students/1/contest-achievements")
}
