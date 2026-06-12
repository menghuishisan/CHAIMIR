// sim api 文件负责注册 M4 HTTP/WS 路由、绑定请求和组合鉴权,不承载仿真业务逻辑。
package sim

import (
	"encoding/json"
	"io"
	"net/http"
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

// RegisterRoutes 注册仿真引擎 HTTP 与 WebSocket API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles auth.RoleChecker) error {
	if r == nil {
		return apperr.ErrInternal.WithMessage("sim routes 缺少 HTTP router")
	}
	if svc == nil {
		return apperr.ErrInternal.WithMessage("sim routes 缺少 service")
	}
	if authn == nil {
		return apperr.ErrInternal.WithMessage("sim routes 缺少 auth manager")
	}
	api := simAPI{svc: svc}
	g := r.Group("/api/v1/sim")
	api.registerPublicRoutes(g)
	api.registerUserRoutes(g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin)))
	api.registerTeacherRoutes(g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleTeacher, contracts.RoleSchoolAdmin)))
	api.registerInternalRoutes(g.Group("", authn.ServiceMiddleware()))
	g.POST("/packages/:key/validation-report", authn.PlatformOrServiceMiddleware(), api.validationReport)
	api.registerPlatformRoutes(g.Group("", authn.Middleware(), auth.RequirePlatformIdentity()))
	return nil
}

type simAPI struct {
	svc *Service
}

// registerPublicRoutes 注册公开分享入口。
func (a simAPI) registerPublicRoutes(g gin.IRouter) {
	g.GET("/shared/:code", a.getShared)
}

// registerUserRoutes 注册租户用户可访问的查询、回放、分享和后端计算接口。
func (a simAPI) registerUserRoutes(g gin.IRouter) {
	g.GET("/packages", a.listPackages)
	g.GET("/packages/:key/versions", a.listPackageVersions)
	g.GET("/packages/:key/:version/bundle", a.getBundle)
	g.POST("/sessions/:id/actions", a.reportAction)
	g.GET("/sessions/:id/replay", a.getReplay)
	g.POST("/sessions/:id/share", a.shareSession)
	g.GET("/sessions/:id/stream", a.streamSession)
}

// registerTeacherRoutes 注册教师/学校管理员仿真包扩展接入接口。
func (a simAPI) registerTeacherRoutes(g gin.IRouter) {
	g.POST("/packages", a.submitPackage)
	g.PATCH("/packages/:key", a.updatePackage)
	g.GET("/packages/:key/preview", a.previewPackage)
}

// registerInternalRoutes 注册内部服务会话和检查点接口。
func (a simAPI) registerInternalRoutes(g gin.IRouter) {
	g.POST("/sessions", a.createSession)
	g.DELETE("/sessions/:id", a.destroySession)
	g.POST("/sessions/recycle", a.recycleSessions)
	g.POST("/sessions/:id/checkpoints", a.reportCheckpoint)
}

// registerPlatformRoutes 注册平台管理员审核和生命周期接口。
func (a simAPI) registerPlatformRoutes(g gin.IRouter) {
	g.GET("/reviews", a.listReviews)
	g.POST("/reviews/:id/approve", a.approveReview)
	g.POST("/reviews/:id/reject", a.rejectReview)
	g.POST("/packages/:key/archive", a.archivePackage)
	g.POST("/packages/:key/republish", a.republishPackage)
}

// listPackages 查询用户可见的已上架包列表。
func (a simAPI) listPackages(c *gin.Context) {
	page, size, ok := pageQuery(c)
	if !ok {
		return
	}
	status, err := userPackageListStatus(c.Query("status"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	items, total, p, s, err := a.svc.ListPackages(c.Request.Context(), status, c.Query("category"), c.Query("keyword"), page, size)
	httpx.WritePage(c, items, total, p, s, err)
}

// listPackageVersions 查询某包所有版本。
func (a simAPI) listPackageVersions(c *gin.Context) {
	out, err := a.svc.ListPackageVersions(c.Request.Context(), c.Param("key"))
	httpx.Write(c, out, err)
}

// getBundle 流式返回已上架仿真包正文。
func (a simAPI) getBundle(c *gin.Context) {
	rc, hash, err := a.svc.ReadPublishedBundle(c.Request.Context(), c.Param("key"), c.Param("version"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer rc.Close()
	c.Header("X-Bundle-SHA256", hash)
	c.DataFromReader(http.StatusOK, -1, "application/octet-stream", rc, nil)
}

// submitPackage 绑定 multipart 仿真包上传。
func (a simAPI) submitPackage(c *gin.Context) {
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	req, bundle, ok := bindPackageMultipart(c)
	if !ok {
		return
	}
	out, err := a.svc.SubmitPackage(c.Request.Context(), current.TenantID, current.AccountID, req, bundle)
	httpx.Write(c, out, err)
}

// updatePackage 绑定包更新请求。
func (a simAPI) updatePackage(c *gin.Context) {
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	packageID, ok := httpx.PathID(c, "key")
	if !ok {
		return
	}
	req, bundle, ok := bindPackageMultipart(c)
	if !ok {
		return
	}
	out, err := a.svc.UpdatePackage(c.Request.Context(), current.TenantID, current.AccountID, packageID, req, bundle)
	httpx.Write(c, out, err)
}

// previewPackage 返回包预览所需审核报告。
func (a simAPI) previewPackage(c *gin.Context) {
	packageID, ok := httpx.PathID(c, "key")
	if !ok {
		return
	}
	out, err := a.svc.PackagePreview(c.Request.Context(), packageID)
	httpx.Write(c, out, err)
}

// validationReport 绑定受控预览回写的动态报告。
func (a simAPI) validationReport(c *gin.Context) {
	packageID, ok := httpx.PathID(c, "key")
	if !ok {
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		response.Fail(c, apperr.ErrSimPackageValidationFailed.WithCause(err))
		return
	}
	var req ValidationReportRequest
	if err := jsonUnmarshal(raw, &req); err != nil {
		response.Fail(c, apperr.ErrSimPackageValidationFailed.WithCause(err))
		return
	}
	out, err := a.svc.SubmitValidationReport(c.Request.Context(), packageID, req, raw)
	httpx.Write(c, out, err)
}

// listReviews 查询审核记录。
func (a simAPI) listReviews(c *gin.Context) {
	page, size, ok := pageQuery(c)
	if !ok {
		return
	}
	items, total, p, s, err := a.svc.ListReviews(c.Request.Context(), reviewResultFromQuery(c.DefaultQuery("result", "pending")), page, size)
	httpx.WritePage(c, items, total, p, s, err)
}

// approveReview 通过审核。
func (a simAPI) approveReview(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentPlatformIdentity(c)
	if !ok {
		return
	}
	out, err := a.svc.ApproveReview(c.Request.Context(), current.AccountID, id)
	httpx.Write(c, out, err)
}

// rejectReview 退回审核。
func (a simAPI) rejectReview(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentPlatformIdentity(c)
	if !ok {
		return
	}
	var req RejectReviewRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimReviewStateInvalid) {
		return
	}
	out, err := a.svc.RejectReview(c.Request.Context(), current.AccountID, id, req.Comment)
	httpx.Write(c, out, err)
}

// archivePackage 下架包。
func (a simAPI) archivePackage(c *gin.Context) {
	packageID, ok := httpx.PathID(c, "key")
	if !ok {
		return
	}
	out, err := a.svc.ArchivePackage(c.Request.Context(), packageID)
	httpx.Write(c, out, err)
}

// republishPackage 重新上架已下架包。
func (a simAPI) republishPackage(c *gin.Context) {
	packageID, ok := httpx.PathID(c, "key")
	if !ok {
		return
	}
	out, err := a.svc.RepublishPackage(c.Request.Context(), packageID)
	httpx.Write(c, out, err)
}

// createSession 绑定内部会话创建请求。
func (a simAPI) createSession(c *gin.Context) {
	var req CreateSessionRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimSessionInvalid) {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	if sourceRef, ok := auth.ServiceSourceRefFromContext(c.Request.Context()); ok {
		req.SourceRef = sourceRef
	}
	out, err := a.svc.CreateSessionFromHTTP(c.Request.Context(), tenantID, req)
	httpx.Write(c, out, err)
}

// reportAction 绑定用户操作上报。
func (a simAPI) reportAction(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	var req ReportActionRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimActionSeqInvalid) {
		return
	}
	out, err := a.svc.ReportAction(c.Request.Context(), current.TenantID, current.AccountID, id, req)
	httpx.Write(c, out, err)
}

// getReplay 查询用户自己的回放。
func (a simAPI) getReplay(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	out, err := a.svc.GetReplayForUser(c.Request.Context(), current.TenantID, current.AccountID, id)
	httpx.Write(c, out, err)
}

// destroySession 归档单个内部会话。
func (a simAPI) destroySession(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.DestroySession(c.Request.Context(), tenantID, id))
}

// recycleSessions 按来源批量归档内部会话。
func (a simAPI) recycleSessions(c *gin.Context) {
	var req RecycleRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimSessionInvalid) {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	if sourceRef, ok := auth.ServiceSourceRefFromContext(c.Request.Context()); ok {
		req.SourceRef = sourceRef
	}
	err := a.svc.RecycleBySourceRef(c.Request.Context(), contracts.SimRecycleRequest{TenantID: tenantID, SourceRef: req.SourceRef, Reason: req.Reason})
	httpx.Write(c, gin.H{}, err)
}

// reportCheckpoint 绑定内部检查点上报。
func (a simAPI) reportCheckpoint(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	var req ReportCheckpointRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimCheckpointInvalid) {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.ReportCheckpointFromHTTP(c.Request.Context(), tenantID, id, req))
}

// shareSession 创建分享码。
func (a simAPI) shareSession(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	var req CreateShareRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimShareCodeInvalid) {
		return
	}
	out, err := a.svc.ShareSession(c.Request.Context(), current.TenantID, current.AccountID, id, req.ExpireAt)
	httpx.Write(c, out, err)
}

// getShared 按分享码读取公开剧本。
func (a simAPI) getShared(c *gin.Context) {
	out, err := a.svc.GetSharedReplay(c.Request.Context(), c.Param("code"))
	httpx.Write(c, out, err)
}

// streamSession 建立后端计算仿真 WebSocket。
func (a simAPI) streamSession(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	if a.svc.wsHub == nil {
		response.Fail(c, apperr.ErrSimBackendComputeUnavailable)
		return
	}
	if err := a.svc.wsHub.ServeInteractive(c.Writer, c.Request, func(conn *ws.Conn) error {
		return a.svc.ServeBackendStream(c.Request.Context(), conn, current.TenantID, current.AccountID, id)
	}); err != nil {
		response.Fail(c, apperr.ErrSimBackendComputeUnavailable.WithCause(err))
	}
}

// bindPackageMultipart 读取仿真包 multipart 元数据和 bundle 文件。
func bindPackageMultipart(c *gin.Context) (SubmitPackageRequest, BundleInput, bool) {
	file, header, err := c.Request.FormFile("bundle")
	if err != nil {
		response.Fail(c, apperr.ErrSimBundleUnreadable.WithCause(err))
		return SubmitPackageRequest{}, BundleInput{}, false
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		response.Fail(c, apperr.ErrSimBundleUnreadable.WithCause(err))
		return SubmitPackageRequest{}, BundleInput{}, false
	}
	req := SubmitPackageRequest{Code: c.PostForm("code"), Version: c.PostForm("version"), Name: c.PostForm("name"), Category: c.PostForm("category"), Compute: c.PostForm("compute"), BackendAdapter: c.PostForm("backend_adapter"), ScaleLimit: []byte(defaultJSON(c.PostForm("scale_limit"))), BackendConfig: []byte(defaultJSON(c.PostForm("backend_config"))), AuthorType: int16(httpx.Int(c.PostForm("author_type")))}
	return req, BundleInput{FileName: header.Filename, ContentType: header.Header.Get("Content-Type"), Data: data}, true
}

// pageQuery 统一解析分页参数。
func pageQuery(c *gin.Context) (int, int, bool) {
	page, ok := httpx.QueryInt(c, "page", httpx.QueryIntRule{Default: 1, Min: 1})
	if !ok {
		return 0, 0, false
	}
	size, ok := httpx.QueryInt(c, "size", httpx.QueryIntRule{Default: 20, Min: 1, Max: 100, HasMax: true})
	if !ok {
		return 0, 0, false
	}
	return int(page), int(size), true
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

// currentPlatformIdentity 从上下文读取平台管理员身份。
func currentPlatformIdentity(c *gin.Context) (tenant.Identity, bool) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || !id.IsPlatform || id.AccountID <= 0 {
		response.Fail(c, apperr.ErrUnauthorized)
		return tenant.Identity{}, false
	}
	return id, true
}

// currentServiceTenantID 读取内部服务租户边界。
func currentServiceTenantID(c *gin.Context) (int64, bool) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || id.TenantID <= 0 || !id.IsSystem {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		return 0, false
	}
	return id.TenantID, true
}

// defaultJSON 为空表单字段补 JSON 对象。
func defaultJSON(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "{}"
	}
	return raw
}

// jsonUnmarshal 包装 JSON 解码,避免 api 文件直接散落 encoding/json 依赖。
func jsonUnmarshal(raw []byte, dst any) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
