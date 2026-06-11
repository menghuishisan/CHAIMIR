// sandbox api 文件负责注册 M2 HTTP/WS 路由、绑定请求、组合鉴权并调用 service。
package sandbox

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册 sandbox 模块 HTTP 与 WebSocket API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles auth.RoleChecker) error {
	if r == nil {
		return fmt.Errorf("sandbox routes 缺少 HTTP router")
	}
	if svc == nil {
		return fmt.Errorf("sandbox routes 缺少 service")
	}
	if authn == nil {
		return fmt.Errorf("sandbox routes 缺少 auth manager")
	}
	api := sandboxAPI{svc: svc}
	g := r.Group("/api/v1/sandbox")
	api.registerPlatformRoutes(g.Group("", authn.Middleware(), auth.RequirePlatformIdentity()))
	api.registerInternalRoutes(g.Group("", authn.ServiceMiddleware()))
	api.registerUserRoutes(g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin)))
	api.registerQuotaRoutes(g, authn, roles)
	return nil
}

type sandboxAPI struct {
	svc *Service
}

// registerPlatformRoutes 注册运行时、镜像和工具管理接口。
func (a sandboxAPI) registerPlatformRoutes(g gin.IRouter) {
	g.GET("/runtimes", a.listRuntimes)
	g.POST("/runtimes", a.registerRuntime)
	g.PATCH("/runtimes/:id", a.updateRuntime)
	g.POST("/runtimes/:id/selftest", a.runRuntimeSelftest)
	g.GET("/runtimes/:id/selftest", a.getRuntimeSelftest)
	g.POST("/runtimes/:id/images", a.registerRuntimeImage)
	g.GET("/runtimes/:id/images", a.listRuntimeImages)
	g.DELETE("/runtimes/:id/images/:img", a.disableRuntimeImage)
	g.POST("/runtimes/:id/images/:img/prepull", a.prepullRuntimeImage)
	g.GET("/runtimes/:id/images/:img/prepull", a.getRuntimeImagePrepull)
	g.GET("/tools", a.listTools)
	g.POST("/tools", a.registerTool)
}

// runRuntimeSelftest 触发运行时接入即测。
func (a sandboxAPI) runRuntimeSelftest(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.RunRuntimeSelftest(c.Request.Context(), runtimeID)
	httpx.Write(c, out, err)
}

// getRuntimeSelftest 查询运行时接入即测结果。
func (a sandboxAPI) getRuntimeSelftest(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetRuntimeSelftest(c.Request.Context(), runtimeID)
	httpx.Write(c, out, err)
}

// registerInternalRoutes 注册内部服务签名保护的生命周期和链能力接口。
func (a sandboxAPI) registerInternalRoutes(g gin.IRouter) {
	g.POST("/sandboxes", a.createSandbox)
	g.POST("/sandboxes/recycle", a.recycleBySourceRef)
	g.POST("/sandboxes/:id/pause", a.pauseSandbox)
	g.POST("/sandboxes/:id/resume", a.resumeSandbox)
	g.DELETE("/sandboxes/:id", a.destroySandbox)
	g.POST("/sandboxes/:id/chain/deploy", a.chainDeploy)
	g.POST("/sandboxes/:id/chain/tx", a.chainSendTx)
	g.GET("/sandboxes/:id/chain/query", a.chainQuery)
	g.POST("/sandboxes/:id/chain/reset", a.chainReset)
}

// registerUserRoutes 注册用户侧沙箱查询、文件和进度接口。
func (a sandboxAPI) registerUserRoutes(g gin.IRouter) {
	g.GET("/sandboxes/:id", a.getSandbox)
	g.GET("/sandboxes/:id/progress", a.progress)
	g.GET("/sandboxes/:id/terminal", a.terminal)
	g.GET("/sandboxes/:id/files", a.getFiles)
	g.PUT("/sandboxes/:id/files", a.writeFile)
	g.POST("/sandboxes/:id/files/save", a.saveFiles)
	g.Any("/sandboxes/:id/tools/:tool_code/*proxy_path", a.toolProxy)
}

// registerQuotaRoutes 注册配额查询和调整接口。
func (a sandboxAPI) registerQuotaRoutes(g gin.IRouter, authn *auth.Manager, roles auth.RoleChecker) {
	g.GET("/quota", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleSchoolAdmin), a.quotaStats)
	g.PATCH("/quota", authn.Middleware(), auth.RequirePlatformOrAnyRole(roles, contracts.RoleSchoolAdmin), a.upsertQuota)
}

// listRuntimes 返回运行时列表。
func (a sandboxAPI) listRuntimes(c *gin.Context) {
	out, err := a.svc.ListRuntimes(c.Request.Context())
	httpx.Write(c, out, err)
}

// registerRuntime 绑定运行时注册或更新请求。
func (a sandboxAPI) registerRuntime(c *gin.Context) {
	var req RuntimeRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxRuntimeCreateInvalid) {
		return
	}
	out, err := a.svc.RegisterRuntime(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// updateRuntime 绑定运行时更新请求并校验路径 ID。
func (a sandboxAPI) updateRuntime(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req RuntimeRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxRuntimeUpdateInvalid) {
		return
	}
	out, err := a.svc.UpdateRuntime(c.Request.Context(), runtimeID, req)
	httpx.Write(c, out, err)
}

// registerRuntimeImage 绑定运行时镜像登记请求。
func (a sandboxAPI) registerRuntimeImage(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req RuntimeImageRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxImageCreateInvalid) {
		return
	}
	out, err := a.svc.RegisterRuntimeImage(c.Request.Context(), runtimeID, req)
	httpx.Write(c, out, err)
}

// listRuntimeImages 返回指定运行时的镜像版本。
func (a sandboxAPI) listRuntimeImages(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.ListRuntimeImages(c.Request.Context(), runtimeID)
	httpx.Write(c, out, err)
}

// disableRuntimeImage 绑定镜像停用请求,停用前由 service 清理预拉取 DaemonSet。
func (a sandboxAPI) disableRuntimeImage(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	imageID, ok := httpx.PathID(c, "img")
	if !ok {
		return
	}
	out, err := a.svc.DisableRuntimeImage(c.Request.Context(), runtimeID, imageID)
	httpx.Write(c, out, err)
}

// prepullRuntimeImage 触发镜像预拉取。
func (a sandboxAPI) prepullRuntimeImage(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	imageID, ok := httpx.PathID(c, "img")
	if !ok {
		return
	}
	out, err := a.svc.PrepullRuntimeImage(c.Request.Context(), runtimeID, imageID)
	httpx.Write(c, out, err)
}

// getRuntimeImagePrepull 查询镜像预拉取闭环状态。
func (a sandboxAPI) getRuntimeImagePrepull(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	imageID, ok := httpx.PathID(c, "img")
	if !ok {
		return
	}
	out, err := a.svc.GetRuntimeImagePrepull(c.Request.Context(), runtimeID, imageID)
	httpx.Write(c, out, err)
}

// listTools 返回平台工具列表。
func (a sandboxAPI) listTools(c *gin.Context) {
	out, err := a.svc.ListTools(c.Request.Context())
	httpx.Write(c, out, err)
}

// registerTool 绑定工具注册请求。
func (a sandboxAPI) registerTool(c *gin.Context) {
	var req ToolRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxToolCreateInvalid) {
		return
	}
	out, err := a.svc.RegisterTool(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// createSandbox 绑定内部创建沙箱请求。
func (a sandboxAPI) createSandbox(c *gin.Context) {
	var req CreateSandboxRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxCreateRequestInvalid) {
		return
	}
	req.TenantID = serviceTenantID(c)
	if req.TenantID <= 0 {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		return
	}
	sourceRef, ok := serviceSourceRef(c)
	if !ok {
		return
	}
	req.SourceRef = sourceRef
	out, err := a.svc.CreateSandbox(c.Request.Context(), contractCreateFromDTO(req))
	httpx.Write(c, out, err)
}

// getSandbox 查询当前用户自己的沙箱。
func (a sandboxAPI) getSandbox(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	out, err := a.svc.GetSandboxForOwner(c.Request.Context(), current.TenantID, current.AccountID, id)
	httpx.Write(c, out, err)
}

// pauseSandbox 绑定内部暂停请求。
func (a sandboxAPI) pauseSandbox(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	sourceRef, ok := serviceSourceRef(c)
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.PauseSandbox(c.Request.Context(), contracts.SandboxControlRequest{TenantID: tenantID, SandboxID: id, SourceRef: sourceRef}))
}

// resumeSandbox 绑定内部恢复请求。
func (a sandboxAPI) resumeSandbox(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	sourceRef, ok := serviceSourceRef(c)
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.ResumeSandbox(c.Request.Context(), contracts.SandboxControlRequest{TenantID: tenantID, SandboxID: id, SourceRef: sourceRef}))
}

// destroySandbox 绑定内部销毁请求。
func (a sandboxAPI) destroySandbox(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	sourceRef, ok := serviceSourceRef(c)
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.DestroySandbox(c.Request.Context(), contracts.SandboxControlRequest{TenantID: tenantID, SandboxID: id, SourceRef: sourceRef}))
}

// recycleBySourceRef 绑定来源级联回收请求。
func (a sandboxAPI) recycleBySourceRef(c *gin.Context) {
	var req RecycleRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxRecycleRequestInvalid) {
		return
	}
	req.TenantID = serviceTenantID(c)
	if req.TenantID <= 0 {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		return
	}
	sourceRef, ok := serviceSourceRef(c)
	if !ok {
		return
	}
	req.SourceRef = sourceRef
	httpx.Write(c, gin.H{}, a.svc.RecycleBySourceRef(c.Request.Context(), contracts.SandboxRecycleRequest(req)))
}

// getFiles 绑定工作区文件读取或目录列表请求。
func (a sandboxAPI) getFiles(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	if strings.EqualFold(c.Query("mode"), "list") {
		out, err := a.svc.ListSandboxFilesForOwner(c.Request.Context(), current.TenantID, current.AccountID, id, c.Query("path"))
		httpx.Write(c, out, err)
		return
	}
	out, err := a.svc.ReadSandboxFileForOwner(c.Request.Context(), current.TenantID, current.AccountID, id, c.Query("path"))
	httpx.Write(c, out, err)
}

// writeFile 绑定工作区文件写入请求。
func (a sandboxAPI) writeFile(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	var req FileWriteRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxFileWriteRequestInvalid) {
		return
	}
	req.TenantID = current.TenantID
	if strings.TrimSpace(req.RelativePath) == "" {
		req.RelativePath = c.Query("path")
	}
	httpx.Write(c, gin.H{}, a.svc.PutSandboxFileForOwner(c.Request.Context(), current.TenantID, current.AccountID, id, req))
}

// saveFiles 绑定立即持久化工作区请求。
func (a sandboxAPI) saveFiles(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	out, err := a.svc.SaveSandboxFilesForOwner(c.Request.Context(), current.TenantID, current.AccountID, id)
	httpx.Write(c, out, err)
}

// progress 建立统一 WebSocket Hub 进度订阅。
func (a sandboxAPI) progress(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	if a.svc.wsHub == nil {
		response.Fail(c, apperr.ErrSandboxToolProxyUnavailable)
		return
	}
	if err := a.svc.wsHub.Serve(c.Writer, c.Request, func(conn *ws.Conn) error {
		topic, initial, err := a.svc.ProgressSubscription(c.Request.Context(), current.TenantID, current.AccountID, id)
		if err != nil {
			return err
		}
		if err := conn.BindSession(ws.SessionKey{TenantID: current.TenantID, AccountID: current.AccountID}); err != nil {
			return apperr.ErrSandboxOwnershipInvalid.WithCause(err)
		}
		a.svc.wsHub.Subscribe(conn, topic)
		return conn.SendJSON(initial)
	}); err != nil {
		response.Fail(c, apperr.ErrSandboxToolProxyUnavailable.WithCause(err))
	}
}

// terminal 建立终端 WebSocket 并把字节流代理到 Kubernetes exec PTY。
func (a sandboxAPI) terminal(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	if a.svc.wsHub == nil {
		response.Fail(c, apperr.ErrSandboxToolProxyUnavailable)
		return
	}
	container := strings.TrimSpace(c.Query("container"))
	target, err := a.svc.TerminalTargetForOwner(c.Request.Context(), current.TenantID, current.AccountID, id, container)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := a.svc.wsHub.ServeInteractive(c.Writer, c.Request, func(conn *ws.Conn) error {
		if err := conn.BindSession(ws.SessionKey{TenantID: current.TenantID, AccountID: current.AccountID}); err != nil {
			return apperr.ErrSandboxOwnershipInvalid.WithCause(err)
		}
		return a.svc.AttachTerminal(c.Request.Context(), target, conn.Reader(), conn.Writer())
	}); err != nil {
		response.Fail(c, apperr.ErrSandboxToolProxyUnavailable.WithCause(err))
	}
}

// toolProxy 把用户侧 Web 工具请求反向代理到沙箱内 ClusterIP Service。
func (a sandboxAPI) toolProxy(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	sb, tool, err := a.svc.ToolProxyTargetForOwner(c.Request.Context(), current.TenantID, current.AccountID, id, c.Param("tool_code"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	a.svc.ObserveToolAccess(c.Request.Context(), sb, tool)
	target := toolProxyURL(sb, tool)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		response.Fail(c, apperr.ErrSandboxToolProxyUnavailable.WithCause(err))
	}
	proxy.Rewrite = func(pr *httputil.ProxyRequest) {
		pr.SetURL(target)
		pr.Out.URL.Path = strings.TrimPrefix(c.Param("proxy_path"), "/")
		if pr.Out.URL.Path == "" {
			pr.Out.URL.Path = "/"
		} else {
			pr.Out.URL.Path = "/" + pr.Out.URL.Path
		}
		pr.Out.Host = target.Host
		sanitizeToolProxyHeaders(pr.Out.Header)
	}
	proxy.ServeHTTP(c.Writer, c.Request)
	a.svc.ObserveToolAccess(c.Request.Context(), sb, tool)
}

// chainDeploy 绑定链部署请求。
func (a sandboxAPI) chainDeploy(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ChainRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxDeployRequestInvalid) {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	sourceRef, ok := serviceSourceRef(c)
	if !ok {
		return
	}
	out, err := a.svc.ChainDeploy(c.Request.Context(), contracts.SandboxChainDeployRequest{TenantID: tenantID, SandboxID: id, SourceRef: sourceRef, Payload: req.Payload})
	httpx.Write(c, out, err)
}

// chainSendTx 绑定链交易请求。
func (a sandboxAPI) chainSendTx(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ChainRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxTxRequestInvalid) {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	sourceRef, ok := serviceSourceRef(c)
	if !ok {
		return
	}
	out, err := a.svc.ChainSendTx(c.Request.Context(), contracts.SandboxChainTxRequest{TenantID: tenantID, SandboxID: id, SourceRef: sourceRef, Payload: req.Payload})
	httpx.Write(c, out, err)
}

// chainQuery 绑定链查询请求。
func (a sandboxAPI) chainQuery(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	sourceRef, ok := serviceSourceRef(c)
	if !ok {
		return
	}
	out, err := a.svc.ChainQuery(c.Request.Context(), contracts.SandboxChainQueryRequest{TenantID: tenantID, SandboxID: id, SourceRef: sourceRef, Target: c.Query("target")})
	httpx.Write(c, out, err)
}

// chainReset 绑定链重置请求。
func (a sandboxAPI) chainReset(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	sourceRef, ok := serviceSourceRef(c)
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.ChainReset(c.Request.Context(), contracts.SandboxChainResetRequest{TenantID: tenantID, SandboxID: id, SourceRef: sourceRef}))
}

// quotaStats 查询当前租户配额和活跃数量。
func (a sandboxAPI) quotaStats(c *gin.Context) {
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	out, err := a.svc.Stats(c.Request.Context(), current.TenantID)
	httpx.Write(c, out, err)
}

// upsertQuota 绑定配额调整请求。
func (a sandboxAPI) upsertQuota(c *gin.Context) {
	var req QuotaRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxQuotaUpdateInvalid) {
		return
	}
	current, _ := tenant.FromContext(c.Request.Context())
	if !current.IsPlatform {
		req.TenantID = current.TenantID
	}
	out, err := a.svc.UpsertQuota(c.Request.Context(), TenantQuota(req))
	httpx.Write(c, out, err)
}

// currentTenantIdentity 从服务端鉴权上下文读取租户身份。
func currentTenantIdentity(c *gin.Context) (tenant.Identity, bool) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 {
		response.Fail(c, apperr.ErrUnauthorized)
		return tenant.Identity{}, false
	}
	return id, true
}

// serviceTenantID 从内部服务签名上下文读取租户边界。
func serviceTenantID(c *gin.Context) int64 {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || id.TenantID <= 0 || !id.IsSystem {
		return 0
	}
	return id.TenantID
}

// currentServiceTenantID 读取内部服务租户边界,缺失时立即返回统一鉴权错误。
func currentServiceTenantID(c *gin.Context) (int64, bool) {
	tenantID := serviceTenantID(c)
	if tenantID <= 0 {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		return 0, false
	}
	return tenantID, true
}

// serviceSourceRef 读取服务签名已校验来源,防止调用方伪造请求体来源。
func serviceSourceRef(c *gin.Context) (string, bool) {
	if sourceRef, ok := auth.ServiceSourceRefFromContext(c.Request.Context()); ok {
		return sourceRef, true
	}
	response.Fail(c, apperr.ErrServiceUnauthorized)
	return "", false
}

// toolProxyURL 生成集群内 Web 工具 Service 目标地址,不暴露外部直链。
func toolProxyURL(sb Sandbox, tool SandboxTool) *url.URL {
	return &url.URL{
		Scheme: "http",
		Host:   "tool-" + tool.ToolCode + "." + sb.Namespace + ".svc.cluster.local",
	}
}

// sanitizeToolProxyHeaders 删除平台身份凭据和内部服务签名,防止沙箱工具读取用户或服务端 token。
func sanitizeToolProxyHeaders(header http.Header) {
	for _, key := range []string{
		"Authorization",
		"Cookie",
		"Proxy-Authorization",
		"X-Api-Key",
		"X-CSRF-Token",
		auth.ServiceNameHeader,
		auth.ServiceTenantHeader,
		auth.ServiceSourceRefHeader,
		auth.ServiceTimestampHeader,
		auth.ServiceSignatureHeader,
	} {
		header.Del(key)
	}
}
