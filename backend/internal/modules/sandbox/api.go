// sandbox api 文件负责注册 M2 HTTP/WS 路由、绑定请求、组合鉴权并调用 service。
package sandbox

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册 sandbox 模块 HTTP 与 WebSocket API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles contracts.IdentityService) error {
	if r == nil {
		return fmt.Errorf("sandbox routes 缺少 HTTP router")
	}
	if svc == nil {
		return fmt.Errorf("sandbox routes 缺少 service")
	}
	if authn == nil {
		return fmt.Errorf("sandbox routes 缺少 auth manager")
	}
	api := sandboxAPI{svc: svc, authn: authn}
	g := r.Group("/api/v1/sandbox")
	api.registerPlatformRoutes(g.Group("", authn.Middleware(), auth.RequirePlatformIdentity()))
	api.registerInternalRoutes(g.Group("/internal", authn.ServiceMiddleware()))
	api.registerChainRoutes(g.Group("", authn.ServiceOrTenantAnyRoleMiddleware(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin)))
	api.registerUserRoutes(g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin)))
	api.registerInteractiveSocketRoutes(g.Group("", authn.WebSocketMiddleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin)))
	api.registerToolProxyRoutes(g.Group("", authn.BrowserAccessMiddleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin)))
	api.registerQuotaRoutes(g, authn, roles)
	return nil
}

type sandboxAPI struct {
	svc   *Service
	authn *auth.Manager
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

// registerInternalRoutes 注册内部服务签名保护的生命周期接口。
func (a sandboxAPI) registerInternalRoutes(g gin.IRouter) {
	g.POST("/sandboxes", a.createSandbox)
	g.POST("/sandboxes/recycle", a.recycleBySourceRef)
	g.POST("/sandboxes/:id/pause", a.pauseSandbox)
	g.POST("/sandboxes/:id/resume", a.resumeSandbox)
	g.DELETE("/sandboxes/:id", a.destroySandbox)
	g.POST("/sandboxes/:id/chain/reset", a.chainReset)
}

// registerChainRoutes 注册链能力入口,同一路径同时服务内部业务回调和用户工作台。
func (a sandboxAPI) registerChainRoutes(g gin.IRouter) {
	g.POST("/sandboxes/:id/chain/deploy", a.chainDeploy)
	g.POST("/sandboxes/:id/chain/tx", a.chainSendTx)
	g.GET("/sandboxes/:id/chain/query", a.chainQuery)
}

// registerUserRoutes 注册用户侧沙箱查询、文件和进度接口。
func (a sandboxAPI) registerUserRoutes(g gin.IRouter) {
	g.GET("/sandboxes/:id", a.getSandbox)
	g.GET("/sandboxes/:id/files", a.getFiles)
	g.PUT("/sandboxes/:id/files", a.writeFile)
	g.POST("/sandboxes/:id/files/save", a.saveFiles)
	g.POST("/sandboxes/:id/command-tools/:tool_code/run", a.runCommandTool)
}

// registerInteractiveSocketRoutes 注册终端和进度 WebSocket 入口,统一使用短时连接票据。
func (a sandboxAPI) registerInteractiveSocketRoutes(g gin.IRouter) {
	g.GET("/sandboxes/:id/progress", a.progress)
	g.GET("/sandboxes/:id/terminal", a.terminal)
}

// registerToolProxyRoutes 注册浏览器工具代理入口,允许路径受限 Cookie 支撑工具资源加载。
func (a sandboxAPI) registerToolProxyRoutes(g gin.IRouter) {
	g.Match([]string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions}, "/sandboxes/:id/tools/:tool_code/*proxy_path", a.toolProxy)
}

// registerQuotaRoutes 注册配额查询和调整接口。
func (a sandboxAPI) registerQuotaRoutes(g gin.IRouter, authn *auth.Manager, roles contracts.IdentityService) {
	g.GET("/quota", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleSchoolAdmin), a.quotaStats)
	g.PATCH("/quota", authn.Middleware(), auth.RequirePlatformOrAnyRole(roles, contracts.RoleSchoolAdmin), a.upsertQuota)
}

// listRuntimes 返回运行时列表。
func (a sandboxAPI) listRuntimes(c *gin.Context) {
	out, err := a.svc.ListRuntimes(c.Request.Context())
	httpx.Write(c, runtimeResponsesFromModels(out), err)
}

// registerRuntime 绑定运行时注册或更新请求。
func (a sandboxAPI) registerRuntime(c *gin.Context) {
	var req RuntimeRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxRuntimeCreateInvalid) {
		return
	}
	out, err := a.svc.RegisterRuntime(c.Request.Context(), req)
	httpx.Write(c, runtimeResponseFromModel(out), err)
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
	httpx.Write(c, runtimeResponseFromModel(out), err)
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
	httpx.Write(c, runtimeImageResponseFromModel(out), err)
}

// listRuntimeImages 返回指定运行时的镜像版本。
func (a sandboxAPI) listRuntimeImages(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.ListRuntimeImages(c.Request.Context(), runtimeID)
	httpx.Write(c, runtimeImageResponsesFromModels(out), err)
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
	httpx.Write(c, runtimeImageResponseFromModel(out), err)
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
	httpx.Write(c, toolResponsesFromModels(out), err)
}

// registerTool 绑定工具注册请求。
func (a sandboxAPI) registerTool(c *gin.Context) {
	var req ToolRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxToolCreateInvalid) {
		return
	}
	out, err := a.svc.RegisterTool(c.Request.Context(), req)
	httpx.Write(c, toolResponseFromModel(out), err)
}

// createSandbox 绑定内部创建沙箱请求。
func (a sandboxAPI) createSandbox(c *gin.Context) {
	var req CreateSandboxRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxCreateRequestInvalid) {
		return
	}
	req.TenantID = ids.ID(serviceTenantID(c))
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
	current, ok := httpx.CurrentTenantIdentity(c)
	if !ok {
		return
	}
	out, err := a.svc.GetSandboxForOwner(c.Request.Context(), current.TenantID, current.AccountID, id)
	if err != nil {
		httpx.Write(c, nil, err)
		return
	}
	httpx.Write(c, sandboxResponseFromInfo(out), nil)
}

// pauseSandbox 绑定内部暂停请求。
func (a sandboxAPI) pauseSandbox(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantID, ok := httpx.CurrentServiceTenantID(c)
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
	tenantID, ok := httpx.CurrentServiceTenantID(c)
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
	tenantID, ok := httpx.CurrentServiceTenantID(c)
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
	req.TenantID = ids.ID(serviceTenantID(c))
	if req.TenantID <= 0 {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		return
	}
	sourceRef, ok := serviceSourceRef(c)
	if !ok {
		return
	}
	req.SourceRef = sourceRef
	httpx.Write(c, gin.H{}, a.svc.RecycleBySourceRef(c.Request.Context(), contracts.SandboxRecycleRequest{TenantID: req.TenantID.Int64(), SourceRef: req.SourceRef, Reason: req.Reason}))
}

// getFiles 绑定工作区文件读取或目录列表请求。
func (a sandboxAPI) getFiles(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := httpx.CurrentTenantIdentity(c)
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
	current, ok := httpx.CurrentTenantIdentity(c)
	if !ok {
		return
	}
	var req FileWriteRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxFileWriteRequestInvalid) {
		return
	}
	req.TenantID = ids.ID(current.TenantID)
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
	current, ok := httpx.CurrentTenantIdentity(c)
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
	current, ok := httpx.CurrentTenantIdentity(c)
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
		if err := a.svc.wsHub.Subscribe(conn, topic); err != nil {
			return apperr.ErrSandboxToolProxyUnavailable.WithCause(err)
		}
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
	current, ok := httpx.CurrentTenantIdentity(c)
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
	current, ok := httpx.CurrentTenantIdentity(c)
	if !ok {
		return
	}
	sb, tool, err := a.svc.ToolProxyTargetForOwner(c.Request.Context(), current.TenantID, current.AccountID, id, c.Param("tool_code"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	a.svc.ObserveToolAccess(c.Request.Context(), sb)
	target := toolProxyURL(sb, tool)
	externalPrefix := toolProxyExternalPrefix(id, c.Param("tool_code"))
	if !a.prepareToolBrowserAccess(c, externalPrefix) {
		return
	}
	proxy := httpx.NewPrefixReverseProxy(httpx.PrefixReverseProxyConfig{
		Target:         target,
		ProxyPath:      c.Param("proxy_path"),
		ExternalPrefix: externalPrefix,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			response.Fail(c, apperr.ErrSandboxToolProxyUnavailable.WithCause(err))
		},
	})
	proxy.ServeHTTP(c.Writer, c.Request)
	a.svc.ObserveToolAccess(c.Request.Context(), sb)
}

// prepareToolBrowserAccess 为浏览器工具写入路径受限 Cookie,并清除一次性 query token 后再进入上游代理。
func (a sandboxAPI) prepareToolBrowserAccess(c *gin.Context, externalPrefix string) bool {
	token, ok := auth.BrowserAccessToken(c)
	if ok && a.authn != nil {
		a.authn.SetBrowserAccessCookie(c, externalPrefix, token)
	}
	if !auth.BrowserAccessFromQuery(c) {
		return true
	}
	c.Redirect(http.StatusFound, toolProxyCleanRequestURI(c))
	return false
}

// runCommandTool 执行命令类工具的一次受控命令。
func (a sandboxAPI) runCommandTool(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := httpx.CurrentTenantIdentity(c)
	if !ok {
		return
	}
	var req ToolRunRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxToolRunRequestInvalid) {
		return
	}
	out, err := a.svc.RunCommandToolForOwner(c.Request.Context(), current.TenantID, current.AccountID, id, c.Param("tool_code"), req)
	httpx.Write(c, out, err)
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
	identity, ok := currentChainIdentity(c)
	if !ok {
		return
	}
	var out map[string]any
	var err error
	if identity.IsSystem {
		out, err = a.svc.ChainDeploy(c.Request.Context(), contracts.SandboxChainDeployRequest{TenantID: identity.TenantID, SandboxID: id, SourceRef: identity.SourceRef, Payload: req.Payload})
	} else {
		out, err = a.svc.ChainDeployForOwner(c.Request.Context(), identity.TenantID, identity.AccountID, id, req.Payload)
	}
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
	identity, ok := currentChainIdentity(c)
	if !ok {
		return
	}
	var out map[string]any
	var err error
	if identity.IsSystem {
		out, err = a.svc.ChainSendTx(c.Request.Context(), contracts.SandboxChainTxRequest{TenantID: identity.TenantID, SandboxID: id, SourceRef: identity.SourceRef, Payload: req.Payload})
	} else {
		out, err = a.svc.ChainSendTxForOwner(c.Request.Context(), identity.TenantID, identity.AccountID, id, req.Payload)
	}
	httpx.Write(c, out, err)
}

// chainQuery 绑定链查询请求。
func (a sandboxAPI) chainQuery(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	identity, ok := currentChainIdentity(c)
	if !ok {
		return
	}
	var out map[string]any
	var err error
	if identity.IsSystem {
		out, err = a.svc.ChainQuery(c.Request.Context(), contracts.SandboxChainQueryRequest{TenantID: identity.TenantID, SandboxID: id, SourceRef: identity.SourceRef, Target: c.Query("target")})
	} else {
		out, err = a.svc.ChainQueryForOwner(c.Request.Context(), identity.TenantID, identity.AccountID, id, c.Query("target"))
	}
	httpx.Write(c, out, err)
}

// chainReset 绑定链重置请求。
func (a sandboxAPI) chainReset(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	identity, ok := currentChainIdentity(c)
	if !ok {
		return
	}
	var err error
	if identity.IsSystem {
		err = a.svc.ChainReset(c.Request.Context(), contracts.SandboxChainResetRequest{TenantID: identity.TenantID, SandboxID: id, SourceRef: identity.SourceRef})
	} else {
		err = apperr.ErrForbidden
	}
	httpx.Write(c, gin.H{}, err)
}

// quotaStats 查询当前租户配额和活跃数量。
func (a sandboxAPI) quotaStats(c *gin.Context) {
	current, ok := httpx.CurrentTenantIdentity(c)
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
	current, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	if !current.IsPlatform {
		req.TenantID = ids.ID(current.TenantID)
	}
	out, err := a.svc.UpsertQuota(c.Request.Context(), TenantQuota{TenantID: req.TenantID.Int64(), MaxConcurrentSandbox: req.MaxConcurrentSandbox, MaxCPU: req.MaxCPU, MaxMemoryMB: req.MaxMemoryMB, IdleTimeoutMin: req.IdleTimeoutMin, MaxLifetimeMin: req.MaxLifetimeMin, MaxKeepaliveMin: req.MaxKeepaliveMin, MaxSnapshotRetentionMin: req.MaxSnapshotRetentionMin})
	httpx.Write(c, out, err)
}

// serviceTenantID 从内部服务签名上下文读取租户边界。
func serviceTenantID(c *gin.Context) int64 {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || id.TenantID <= 0 || !id.IsSystem {
		return 0
	}
	return id.TenantID
}

type chainRequestIdentity struct {
	TenantID  int64
	AccountID int64
	SourceRef string
	IsSystem  bool
}

// currentChainIdentity 解析链能力调用身份,内部服务走 source_ref 边界,用户工作台走 owner 边界。
func currentChainIdentity(c *gin.Context) (chainRequestIdentity, bool) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || id.TenantID <= 0 {
		response.Fail(c, apperr.ErrUnauthorized)
		return chainRequestIdentity{}, false
	}
	if id.IsSystem {
		sourceRef, ok := serviceSourceRef(c)
		if !ok {
			return chainRequestIdentity{}, false
		}
		return chainRequestIdentity{TenantID: id.TenantID, SourceRef: sourceRef, IsSystem: true}, true
	}
	if id.AccountID <= 0 {
		response.Fail(c, apperr.ErrUnauthorized)
		return chainRequestIdentity{}, false
	}
	return chainRequestIdentity{TenantID: id.TenantID, AccountID: id.AccountID}, true
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
	serviceName, servicePort, ok := toolProxyRouteTarget(tool.ResourceSpec)
	if !ok {
		return &url.URL{Scheme: "http"}
	}
	return &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.cluster.local:%d", serviceName, sb.Namespace, servicePort),
	}
}

// toolProxyExternalPrefix 返回浏览器可见的工具代理前缀,用于保持 Web 工具重定向不跳出鉴权入口。
func toolProxyExternalPrefix(sandboxID int64, toolCode string) string {
	return fmt.Sprintf("/api/v1/sandbox/sandboxes/%d/tools/%s", sandboxID, url.PathEscape(strings.TrimSpace(toolCode)))
}

// toolProxyCleanRequestURI 删除浏览器入口一次性 token,确保 token 不进入上游工具日志或重定向。
func toolProxyCleanRequestURI(c *gin.Context) string {
	u := *c.Request.URL
	query := u.Query()
	query.Del(auth.BrowserAccessTokenQuery)
	u.RawQuery = query.Encode()
	return u.RequestURI()
}

// toolProxyRouteTarget 解析工具声明的首个平台代理路由,与 K8s Service 创建共用 resource_spec。
func toolProxyRouteTarget(spec ToolResourceSpec) (string, int32, bool) {
	for _, route := range spec.Routes {
		serviceName := strings.TrimSpace(route.Service)
		portName := strings.TrimSpace(route.Port)
		if serviceName == "" || portName == "" {
			continue
		}
		for _, service := range spec.Services {
			if strings.TrimSpace(service.Name) != serviceName {
				continue
			}
			for _, port := range service.Ports {
				if strings.TrimSpace(port.Name) == portName && port.Port > 0 {
					return serviceName, port.Port, true
				}
			}
		}
	}
	return "", 0, false
}
