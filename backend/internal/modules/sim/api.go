// sim api 文件负责注册 M4 HTTP/WS 路由、绑定请求和组合鉴权,不承载仿真业务逻辑。
package sim

import (
	"errors"
	"io"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/upload"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// 执行位置与运行能力由服务端派生，不接受上传者指定。
var simPackageMultipartValues = map[string]struct{}{
	"code":        {},
	"version":     {},
	"name":        {},
	"category":    {},
	"scale_limit": {},
}

// RegisterRoutes 注册仿真引擎 HTTP 与 WebSocket API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles contracts.IdentityService) error {
	if r == nil {
		return apperr.ErrHTTPRouterMissing
	}
	if svc == nil {
		return apperr.ErrHTTPServiceMissing
	}
	if authn == nil {
		return apperr.ErrHTTPAuthMissing
	}
	api := simAPI{svc: svc}
	g := r.Group("/api/v1/sim")
	api.registerPublicRoutes(g)
	api.registerUserRoutes(g.Group("", authn.Middleware(contracts.TenantModuleSim), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin)))
	api.registerStreamRoutes(g.Group("", authn.WebSocketMiddleware(contracts.TenantModuleSim), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin)))
	api.registerTeacherRoutes(g.Group("", authn.Middleware(contracts.TenantModuleSim), auth.RequireTenantAnyRole(roles, contracts.RoleTeacher, contracts.RoleSchoolAdmin)))
	api.registerInternalRoutes(g.Group("/internal", authn.ServiceMiddleware()))
	api.registerPlatformRoutes(g.Group("", authn.Middleware(), auth.RequirePlatformIdentity()))
	return nil
}

type simAPI struct {
	svc *Service
}

func (a simAPI) registerPublicRoutes(g gin.IRouter) {
	g.GET("/shared/:code", a.getShared)
}

// 扩展包正文只投递到隔离容器，不提供浏览器下载入口。
func (a simAPI) registerUserRoutes(g gin.IRouter) {
	g.GET("/packages", a.listPackages)
	g.GET("/packages/:key/versions", a.listPackageVersions)
	g.POST("/sessions/:id/actions", a.reportAction)
	g.GET("/sessions/:id/replay", a.getReplay)
	g.POST("/sessions/:id/share", a.shareSession)
}

func (a simAPI) registerStreamRoutes(g gin.IRouter) {
	g.GET("/sessions/:id/stream", a.streamSession)
}

func (a simAPI) registerTeacherRoutes(g gin.IRouter) {
	g.POST("/packages", a.submitPackage)
	g.PATCH("/packages/:key", a.updatePackage)
	g.GET("/packages/:key/preview", a.previewPackage)
}

func (a simAPI) registerInternalRoutes(g gin.IRouter) {
	g.POST("/sessions", a.createSession)
	g.DELETE("/sessions/:id", a.destroySession)
	g.POST("/sessions/recycle", a.recycleSessions)
}

func (a simAPI) registerPlatformRoutes(g gin.IRouter) {
	g.GET("/reviews", a.listReviews)
	g.POST("/reviews/:id/approve", a.approveReview)
	g.POST("/reviews/:id/reject", a.rejectReview)
	g.POST("/packages/:key/archive", a.archivePackage)
	g.POST("/packages/:key/republish", a.republishPackage)
}

// listPackages 查询包列表。
// 默认列出可用场景(仅已上架);带 mine=true 时列出当前账号作为作者提交的包,
// 此时可按任一状态过滤 —— 作者边界由服务端会话的账号编号强制,不接受客户端传作者参数。
func (a simAPI) listPackages(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	mine, err := mineFlagFromQuery(c.Query("mine"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	status, err := packageListStatus(c.Query("status"), mine)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var authorID int64
	if mine {
		current, ok := currentTenantIdentity(c)
		if !ok {
			return
		}
		authorID = current.AccountID
	}
	items, total, p, s, facets, err := a.svc.ListPackages(c.Request.Context(), status, c.Query("category"), c.Query("keyword"), authorID, page, size)
	httpx.WritePageWithFacets(c, items, total, p, s, facets, err)
}

func (a simAPI) listPackageVersions(c *gin.Context) {
	out, err := a.svc.ListPackageVersions(c.Request.Context(), c.Param("key"))
	httpx.Write(c, out, err)
}

func (a simAPI) submitPackage(c *gin.Context) {
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	req, bundle, ok := readPackageMultipart(c, a.svc.upload.SimBundleMetadataMaxBytes, a.svc.upload.SimBundleMaxBytes)
	if !ok {
		return
	}
	out, err := a.svc.SubmitPackage(c.Request.Context(), current.TenantID, current.AccountID, req, bundle)
	httpx.Write(c, out, err)
}

func (a simAPI) updatePackage(c *gin.Context) {
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	packageID, ok := httpx.PathID(c, "key")
	if !ok {
		return
	}
	req, bundle, ok := readPackageMultipart(c, a.svc.upload.SimBundleMetadataMaxBytes, a.svc.upload.SimBundleMaxBytes)
	if !ok {
		return
	}
	out, err := a.svc.UpdatePackage(c.Request.Context(), current.TenantID, current.AccountID, packageID, req, bundle)
	httpx.Write(c, out, err)
}

func (a simAPI) previewPackage(c *gin.Context) {
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	packageID, ok := httpx.PathID(c, "key")
	if !ok {
		return
	}
	out, err := a.svc.PackagePreview(c.Request.Context(), current.AccountID, packageID)
	httpx.Write(c, out, err)
}

func (a simAPI) listReviews(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	result, err := reviewResultFromQuery(c.DefaultQuery("result", "pending"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	items, total, p, s, facets, err := a.svc.ListReviews(c.Request.Context(), result, page, size)
	httpx.WritePageWithFacets(c, items, total, p, s, facets.Map(), err)
}

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

func (a simAPI) archivePackage(c *gin.Context) {
	packageID, ok := httpx.PathID(c, "key")
	if !ok {
		return
	}
	out, err := a.svc.ArchivePackage(c.Request.Context(), packageID)
	httpx.Write(c, out, err)
}

func (a simAPI) republishPackage(c *gin.Context) {
	packageID, ok := httpx.PathID(c, "key")
	if !ok {
		return
	}
	out, err := a.svc.RepublishPackage(c.Request.Context(), packageID)
	httpx.Write(c, out, err)
}

func (a simAPI) createSession(c *gin.Context) {
	var req CreateSessionRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimSessionInvalid) {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	sourceRef, ok := auth.ServiceSourceRefFromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		return
	}
	info, err := a.svc.CreateSession(c.Request.Context(), contractCreateFromDTO(req, tenantID, sourceRef))
	httpx.Write(c, SimSessionCreateResponse{SessionID: ids.ID(info.SessionID), Compute: info.Compute}, err)
}

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

func (a simAPI) destroySession(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	sourceRef, ok := auth.ServiceSourceRefFromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		return
	}
	err := a.svc.DestroySession(c.Request.Context(), contracts.SimDestroySessionRequest{TenantID: tenantID, SessionID: id, SourceRef: sourceRef})
	httpx.Write(c, struct{}{}, err)
}

func (a simAPI) recycleSessions(c *gin.Context) {
	var req RecycleRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimSessionInvalid) {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	sourceRef, ok := auth.ServiceSourceRefFromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		return
	}
	err := a.svc.RecycleByScopeRef(c.Request.Context(), contracts.SimRecycleRequest{TenantID: tenantID, SourceRef: sourceRef, ScopeRef: req.ScopeRef, Reason: req.Reason})
	httpx.Write(c, struct{}{}, err)
}

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

func (a simAPI) getShared(c *gin.Context) {
	out, err := a.svc.GetSharedReplay(c.Request.Context(), c.Param("code"))
	httpx.Write(c, out, err)
}

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
		return a.svc.ServeBackendStream(conn.Context(), conn, current.TenantID, current.AccountID, id)
	}); err != nil {
		response.Fail(c, apperr.ErrSimBackendComputeUnavailable.WithCause(err))
	}
}

// readPackageMultipart 以 streaming 方式读取仿真包 multipart,拒绝未知字段和重复字段。
func readPackageMultipart(c *gin.Context, metadataMaxBytes, bundleMaxBytes int64) (SubmitPackageRequest, BundleInput, bool) {
	if metadataMaxBytes <= 0 || bundleMaxBytes <= 0 {
		response.Fail(c, apperr.ErrSimPackageInvalid)
		return SubmitPackageRequest{}, BundleInput{}, false
	}
	reader, err := c.Request.MultipartReader()
	if err != nil {
		response.Fail(c, apperr.ErrSimPackageInvalid.WithCause(err))
		return SubmitPackageRequest{}, BundleInput{}, false
	}
	fields := map[string]string{}
	var bundle BundleInput
	var metadataUsed int64
	for {
		part, err := reader.NextPart()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			response.Fail(c, apperr.ErrSimPackageInvalid.WithCause(err))
			return SubmitPackageRequest{}, BundleInput{}, false
		}
		name := strings.TrimSpace(part.FormName())
		if name == "" {
			response.Fail(c, apperr.ErrSimPackageInvalid)
			return SubmitPackageRequest{}, BundleInput{}, false
		}
		if part.FileName() != "" {
			if name != "bundle" || bundle.FileName != "" {
				response.Fail(c, apperr.ErrSimPackageInvalid)
				return SubmitPackageRequest{}, BundleInput{}, false
			}
			content, result, err := upload.ReadBounded(part, bundleMaxBytes)
			if err != nil {
				response.Fail(c, apperr.ErrSimBundleUnreadable.WithCause(err))
				return SubmitPackageRequest{}, BundleInput{}, false
			}
			if result != upload.SizeOK {
				response.Fail(c, apperr.ErrSimBundleUnreadable)
				return SubmitPackageRequest{}, BundleInput{}, false
			}
			bundle = BundleInput{FileName: part.FileName(), ContentType: part.Header.Get("Content-Type"), Data: content}
			continue
		}
		if _, ok := simPackageMultipartValues[name]; !ok {
			response.Fail(c, apperr.ErrSimPackageInvalid)
			return SubmitPackageRequest{}, BundleInput{}, false
		}
		if _, exists := fields[name]; exists {
			response.Fail(c, apperr.ErrSimPackageInvalid)
			return SubmitPackageRequest{}, BundleInput{}, false
		}
		remaining := metadataMaxBytes - metadataUsed
		if remaining <= 0 {
			response.Fail(c, apperr.ErrSimPackageInvalid)
			return SubmitPackageRequest{}, BundleInput{}, false
		}
		content, result, err := upload.ReadBounded(part, remaining)
		if err != nil {
			response.Fail(c, apperr.ErrSimPackageInvalid.WithCause(err))
			return SubmitPackageRequest{}, BundleInput{}, false
		}
		if result != upload.SizeOK {
			response.Fail(c, apperr.ErrSimPackageInvalid)
			return SubmitPackageRequest{}, BundleInput{}, false
		}
		metadataUsed += int64(len(content))
		fields[name] = string(content)
	}
	// 未知和重复字段已拒绝，字段数相等即表示必填项齐全。
	if len(fields) != len(simPackageMultipartValues) || bundle.FileName == "" || len(bundle.Data) == 0 {
		response.Fail(c, apperr.ErrSimPackageInvalid)
		return SubmitPackageRequest{}, BundleInput{}, false
	}
	req := SubmitPackageRequest{Code: fields["code"], Version: fields["version"], Name: fields["name"], Category: fields["category"], ScaleLimit: []byte(fields["scale_limit"])}
	return req, bundle, true
}

func currentTenantIdentity(c *gin.Context) (tenant.Identity, bool) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 {
		response.Fail(c, apperr.ErrUnauthorized)
		return tenant.Identity{}, false
	}
	return id, true
}

func currentPlatformIdentity(c *gin.Context) (tenant.Identity, bool) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || !id.IsPlatform || id.AccountID <= 0 {
		response.Fail(c, apperr.ErrUnauthorized)
		return tenant.Identity{}, false
	}
	return id, true
}

func currentServiceTenantID(c *gin.Context) (int64, bool) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || id.TenantID <= 0 || !id.IsSystem {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		return 0, false
	}
	return id.TenantID, true
}
