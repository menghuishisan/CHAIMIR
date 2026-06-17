// identity api_audit 文件承接审计日志查询 HTTP 请求,只做查询参数绑定和鉴权组合。
package identity

import (
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// auditAPI 封装审计查询 HTTP handler 依赖。
type auditAPI struct {
	svc *Service
}

// registerAuditRoutes 注册审计查询路由。
func registerAuditRoutes(r gin.IRouter, svc *Service, authn *auth.Manager) {
	api := auditAPI{svc: svc}
	r.GET("/audit", authn.Middleware(), auth.RequirePlatformOrAnyRole(svc, contracts.RoleSchoolAdmin), api.queryAudit)
}

// queryAudit 解析审计过滤条件并委托 service 按当前身份收敛范围。
func (a auditAPI) queryAudit(c *gin.Context) {
	req, ok := bindAuditQuery(c)
	if !ok {
		return
	}
	out, err := a.svc.QueryAuditLogsForCurrent(c.Request.Context(), req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// bindAuditQuery 解析审计查询参数,时间参数使用 RFC3339 避免时区歧义。
func bindAuditQuery(c *gin.Context) (AuditQueryRequest, bool) {
	req := AuditQueryRequest{}
	actorID, ok := httpx.QueryInt(c, "actor_id", httpx.QueryIntRule{BitSize: 64, Min: 0})
	if !ok {
		return AuditQueryRequest{}, false
	}
	req.ActorID = actorID
	page, size, ok := httpx.Page(c)
	if !ok {
		return AuditQueryRequest{}, false
	}
	req.Page = int32(page)
	req.Size = int32(size)
	if req.From, ok = queryTime(c, "from"); !ok {
		return AuditQueryRequest{}, false
	}
	if req.To, ok = queryTime(c, "to"); !ok {
		return AuditQueryRequest{}, false
	}
	req.Action = strings.TrimSpace(c.Query("action"))
	req.TargetType = strings.TrimSpace(c.Query("target_type"))
	return req, true
}

// queryTime 解析可选 RFC3339 时间查询参数。
func queryTime(c *gin.Context, key string) (time.Time, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return time.Time{}, true
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		httpx.Write(c, gin.H{}, apperr.ErrQueryParamInvalid)
		return time.Time{}, false
	}
	return value, true
}
