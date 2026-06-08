// M2 HTTP 接口层:注册 /api/v1/sandbox 下的运行时、工具、沙箱生命周期与配额接口。
package sandbox

import (
	"encoding/base64"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// API 是 M2 的 HTTP 处理器。
type API struct {
	svc     *Service
	authMgr *auth.Manager
}

// NewAPI 构造 M2 API。
func NewAPI(svc *Service, authMgr *auth.Manager) *API {
	return &API{svc: svc, authMgr: authMgr}
}

// Register 注册 M2 路由:用户侧只保留查询、文件、终端和工具入口,生命周期编排走服务鉴权。
func (a *API) Register(rg *gin.RouterGroup) {
	g := rg.Group("/sandbox", a.authMgr.Middleware())
	{
		g.GET("/sandboxes/:id", a.getSandbox)
		g.GET("/sandboxes/:id/progress", a.progressWS)
		g.GET("/sandboxes/:id/terminal", a.terminalWS)
		g.GET("/sandboxes/:id/files", a.getFiles)
		g.PUT("/sandboxes/:id/files", a.putFile)
		g.POST("/sandboxes/:id/files/save", a.saveFiles)
		g.Any("/sandboxes/:id/tools/:tool/*proxyPath", a.proxyTool)
		g.GET("/quota", a.getQuota)
		g.PATCH("/quota", a.requireQuotaUpdateAccess(), a.updateQuota)
	}
	admin := g.Group("", a.requirePlatformAdmin())
	{
		admin.GET("/runtimes", a.listRuntimes)
		admin.POST("/runtimes", a.createRuntime)
		admin.PATCH("/runtimes/:id", a.updateRuntime)
		admin.GET("/runtimes/:id/selftest", a.getRuntimeSelftest)
		admin.POST("/runtimes/:id/images", a.createRuntimeImage)
		admin.POST("/runtimes/:id/images/:img/prepull", a.prepullRuntimeImage)
		admin.GET("/runtimes/:id/images/:img/prepull", a.getRuntimeImagePrepull)
		admin.POST("/runtimes/:id/selftest", a.runRuntimeSelftest)
		admin.GET("/tools", a.listTools)
		admin.POST("/tools", a.createTool)
	}
	internal := rg.Group("/sandbox", a.authMgr.ServiceMiddleware())
	{
		internal.POST("/sandboxes", a.createSandbox)
		internal.POST("/sandboxes/:id/pause", a.pauseSandbox)
		internal.POST("/sandboxes/:id/resume", a.resumeSandbox)
		internal.DELETE("/sandboxes/:id", a.destroySandbox)
		internal.POST("/sandboxes/recycle", a.recycleBySourceRef)
		internal.POST("/sandboxes/:id/chain/deploy", a.chainDeploy)
		internal.POST("/sandboxes/:id/chain/tx", a.chainSendTx)
		internal.GET("/sandboxes/:id/chain/query", a.chainQuery)
		internal.POST("/sandboxes/:id/chain/reset", a.chainReset)
	}
}

// requirePlatformAdmin 限制运行时、镜像、工具等平台级配置只能由平台管理员维护。
func (a *API) requirePlatformAdmin() gin.HandlerFunc {
	return auth.RequirePlatformIdentity()
}

// requireQuotaUpdateAccess 限制配额调整为平台管理员或本租户学校管理员。
func (a *API) requireQuotaUpdateAccess() gin.HandlerFunc {
	var identity contracts.IdentityService
	if a.svc != nil {
		identity = a.svc.identity
	}
	return auth.RequirePlatformOrAnyRole(identity, contracts.RoleSchoolAdmin)
}

// listRuntimes 查询运行时列表。
func (a *API) listRuntimes(c *gin.Context) {
	rows, err := a.svc.ListRuntimes(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// createRuntime 注册运行时定义。
func (a *API) createRuntime(c *gin.Context) {
	var req CreateRuntimeRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrRuntimeCreateInvalid) {
		return
	}
	row, err := a.svc.CreateRuntime(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// updateRuntime 更新运行时配置。
func (a *API) updateRuntime(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req UpdateRuntimeRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrRuntimeUpdateInvalid) {
		return
	}
	row, err := a.svc.UpdateRuntime(c.Request.Context(), runtimeID, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// createRuntimeImage 登记运行时镜像版本。
func (a *API) createRuntimeImage(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req CreateRuntimeImageRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrRuntimeImageCreateInvalid) {
		return
	}
	row, err := a.svc.CreateRuntimeImage(c.Request.Context(), runtimeID, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// runRuntimeSelftest 触发运行时接入自检。
func (a *API) runRuntimeSelftest(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	row, err := a.svc.RunRuntimeSelftest(c.Request.Context(), runtimeID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// getRuntimeSelftest 查询最近一次自检结果。
func (a *API) getRuntimeSelftest(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	row, err := a.svc.GetRuntimeSelftest(c.Request.Context(), runtimeID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// prepullRuntimeImage 触发镜像预拉取。
func (a *API) prepullRuntimeImage(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	imageID, ok := ids.Parse(c.Param("img"))
	if !ok {
		response.Fail(c, apperr.ErrRuntimeImagePrepullInvalid)
		return
	}
	row, err := a.svc.PrepullRuntimeImage(c.Request.Context(), runtimeID, imageID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// getRuntimeImagePrepull 查询镜像预拉取状态。
func (a *API) getRuntimeImagePrepull(c *gin.Context) {
	runtimeID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	imageID, ok := ids.Parse(c.Param("img"))
	if !ok {
		response.Fail(c, apperr.ErrRuntimeImagePrepullInvalid)
		return
	}
	row, err := a.svc.GetRuntimeImagePrepull(c.Request.Context(), runtimeID, imageID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// listTools 查询工具定义列表。
func (a *API) listTools(c *gin.Context) {
	rows, err := a.svc.ListTools(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// createTool 注册工具定义。
func (a *API) createTool(c *gin.Context) {
	var req CreateToolRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrToolCreateInvalid) {
		return
	}
	row, err := a.svc.CreateTool(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// createSandbox 创建沙箱;当前模块化单体内部调用也走同一服务方法。
func (a *API) createSandbox(c *gin.Context) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	var req CreateSandboxRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxCreateRequestInvalid) {
		return
	}
	ownerID, ok := ids.Parse(req.OwnerAccountID)
	if !ok {
		response.Fail(c, apperr.ErrSandboxOwnerInvalid)
		return
	}
	info, err := a.svc.CreateSandbox(c.Request.Context(), contracts.SandboxCreateRequest{
		TenantID: id.TenantID, RuntimeCode: req.RuntimeCode, RuntimeImageVersion: req.RuntimeImageVersion, ToolCodes: req.Tools,
		InitCodeRef: req.InitCodeRef, InitScriptRef: req.InitScriptRef, OwnerAccountID: ownerID,
		SourceRef: req.SourceRef, KeepAlive: req.KeepAlive, SnapshotEnabled: req.SnapshotEnabled,
		KeepAliveMinutes: req.KeepAliveMinutes, SnapshotRetentionMinutes: req.SnapshotRetentionMinutes,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, sandboxInfoToView(info))
}

// getSandbox 查询沙箱状态。
func (a *API) getSandbox(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	info, err := a.svc.GetSandbox(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, sandboxInfoToView(info))
}

// progressWS 建立启动进度 WS。
func (a *API) progressWS(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.ServeProgressWS(c.Writer, c.Request, id); err != nil {
		response.Fail(c, err)
	}
}

// terminalWS 建立终端 WS。
func (a *API) terminalWS(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.ServeTerminalWS(c.Writer, c.Request, id, c.Query("container")); err != nil {
		response.Fail(c, err)
	}
}

// pauseSandbox 暂停运行中沙箱。
func (a *API) pauseSandbox(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.PauseSandbox(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"paused": true})
}

// resumeSandbox 恢复已暂停沙箱。
func (a *API) resumeSandbox(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.ResumeSandbox(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"resumed": true})
}

// getFiles 读取目录或文件。
func (a *API) getFiles(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	payload, err := a.svc.GetFile(c.Request.Context(), id, c.Query("path"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, payload)
}

// putFile 写文件内容。
func (a *API) putFile(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req FileWriteRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxFileWriteInvalid) {
		return
	}
	content, err := normalizeFileWriteContent(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := a.svc.PutFile(c.Request.Context(), id, c.Query("path"), content); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"saved": true})
}

// normalizeFileWriteContent 统一外部文件写入协议,service 层只接收 base64 内容。
func normalizeFileWriteContent(req FileWriteRequest) (string, error) {
	encoding := req.Encoding
	if encoding == "" {
		encoding = "utf-8"
	}
	switch encoding {
	case "utf-8":
		return base64.StdEncoding.EncodeToString([]byte(req.Content)), nil
	case "base64":
		if _, err := base64.StdEncoding.DecodeString(req.Content); err != nil {
			return "", apperr.ErrSandboxFileInvalid.WithCause(err)
		}
		return req.Content, nil
	default:
		return "", apperr.ErrSandboxFileWriteInvalid
	}
}

// saveFiles 立即持久化工作目录。
func (a *API) saveFiles(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.SaveFiles(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"saved": true})
}

// proxyTool 反向代理到沙箱内 web 工具 Service。
func (a *API) proxyTool(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	toolCode := c.Param("tool")
	target, err := a.svc.ToolProxyTarget(c.Request.Context(), id, toolCode)
	if err != nil {
		response.Fail(c, err)
		return
	}
	proxyURL, err := url.Parse(target)
	if err != nil {
		response.Fail(c, apperr.ErrToolProxyFail.WithCause(err))
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(proxyURL)
	c.Request.URL.Path = strings.TrimPrefix(c.Param("proxyPath"), "/")
	c.Request.Host = proxyURL.Host
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, proxyErr error) {
		c.JSON(http.StatusOK, response.Envelope{
			Code:    apperr.ErrToolProxyFail.Code,
			Message: apperr.ErrToolProxyFail.Message,
			TraceID: response.TraceFromContext(c.Request.Context()),
		})
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

// recycleBySourceRef 来源级联回收沙箱。
func (a *API) recycleBySourceRef(c *gin.Context) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	var req RecycleSandboxRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxRecycleRequestInvalid) {
		return
	}
	if err := a.svc.RecycleBySourceRef(c.Request.Context(), id.TenantID, req.SourceRef, req.Reason); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"recycled": true})
}

// destroySandbox 主动销毁沙箱。
func (a *API) destroySandbox(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.DestroySandbox(c.Request.Context(), id, "manual"); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"destroyed": true})
}

// chainDeploy 统一部署合约/链码入口。
func (a *API) chainDeploy(c *gin.Context) {
	a.chainAction(c, apperr.ErrSandboxChainDeployInvalid, func(id int64, payload map[string]any) (map[string]any, error) {
		return a.svc.ChainDeploy(c.Request.Context(), id, payload)
	})
}

// chainSendTx 统一发交易入口。
func (a *API) chainSendTx(c *gin.Context) {
	a.chainAction(c, apperr.ErrSandboxChainTxInvalid, func(id int64, payload map[string]any) (map[string]any, error) {
		return a.svc.ChainSendTx(c.Request.Context(), id, payload)
	})
}

// chainQuery 统一链状态查询入口。
func (a *API) chainQuery(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	res, err := a.svc.ChainQuery(c.Request.Context(), id, c.Query("target"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// chainReset 统一链重置入口。
func (a *API) chainReset(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.ChainReset(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"reset": true})
}

// chainAction 解析通用 JSON payload 后执行链能力动作。
func (a *API) chainAction(c *gin.Context, bindErr *apperr.Error, fn func(id int64, payload map[string]any) (map[string]any, error)) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var payload map[string]any
	if !httpx.BindJSONWithError(c, &payload, bindErr) {
		return
	}
	res, err := fn(id, payload)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// getQuota 查询本租户配额。
func (a *API) getQuota(c *gin.Context) {
	quota, err := a.svc.GetQuota(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, quota)
}

// updateQuota 调整本租户配额。
func (a *API) updateQuota(c *gin.Context) {
	var req QuotaRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrQuotaUpdateInvalid) {
		return
	}
	quota, err := a.svc.UpdateQuota(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, quota)
}

// sandboxInfoToView 转换 contracts DTO 为 HTTP 视图。
func sandboxInfoToView(info contracts.SandboxInfo) SandboxView {
	tools := make([]SandboxToolView, 0, len(info.ToolAccess))
	for _, item := range info.ToolAccess {
		tools = append(tools, SandboxToolView{
			ToolCode: item.ToolCode, Kind: item.Kind, Endpoint: item.Endpoint, Status: item.Status,
		})
	}
	return SandboxView{
		ID: ids.Format(info.SandboxID), Namespace: info.Namespace, SourceRef: info.SourceRef,
		OwnerAccountID: ids.Format(info.OwnerID), RuntimeImageVersion: info.RuntimeImageVersion,
		Phase: info.Phase, Status: info.Status, Tools: tools,
	}
}
