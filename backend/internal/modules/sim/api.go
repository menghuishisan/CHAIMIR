// M4 HTTP 接口层:注册 /api/v1/sim 下的仿真包、审核、会话、回放、分享与检查点接口。
package sim

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// API 是 M4 的 HTTP 处理器。
type API struct {
	svc      *Service
	authMgr  *auth.Manager
	identity contracts.IdentityService
	upload   config.UploadConfig
}

// NewAPI 构造带上传边界配置的 M4 API。
func NewAPI(svc *Service, authMgr *auth.Manager, identity contracts.IdentityService, upload config.UploadConfig) *API {
	return &API{svc: svc, authMgr: authMgr, identity: identity, upload: upload}
}

// Register 注册 M4 路由,会话创建/回收走服务鉴权,用户交互流仍走登录鉴权。
func (a *API) Register(rg *gin.RouterGroup) {
	public := rg.Group("/sim")
	{
		public.GET("/shared/:code", a.getSharedReplay)
	}
	g := rg.Group("/sim", a.authMgr.Middleware())
	{
		g.GET("/packages", a.listPackages)
		g.GET("/packages/:pkg/versions", a.listVersions)
		g.GET("/packages/:pkg/:version/bundle", a.getBundle)

		teacherG := g.Group("", a.requireTeacher())
		teacherG.POST("/packages", a.submitPackage)
		teacherG.GET("/packages/:pkg/preview", a.previewPackage)
		teacherG.PATCH("/packages/:pkg", a.updatePackage)

		platformG := g.Group("", a.requirePlatformAdmin())
		platformG.GET("/reviews", a.listReviews)
		platformG.POST("/reviews/:id/approve", a.approveReview)
		platformG.POST("/reviews/:id/reject", a.rejectReview)
		platformG.POST("/packages/:pkg/archive", a.archivePackage)
		platformG.POST("/packages/:pkg/republish", a.republishPackage)

		g.POST("/sessions/:id/actions", a.reportAction)
		g.GET("/sessions/:id/replay", a.getReplay)
		g.GET("/sessions/:id/stream", a.streamSession)
		g.POST("/sessions/:id/share", a.shareSession)
	}
	reviewReportG := rg.Group("/sim", a.authMgr.PlatformOrServiceMiddleware())
	{
		reviewReportG.POST("/packages/:pkg/validation-report", a.updateValidationReport)
	}
	internal := rg.Group("/sim", a.authMgr.ServiceMiddleware())
	{
		internal.POST("/sessions", a.createSession)
		internal.DELETE("/sessions/:id", a.archiveSession)
		internal.POST("/sessions/recycle", a.recycleSessions)
		internal.POST("/sessions/:id/checkpoints", a.reportCheckpoint)
	}
}

// requirePlatformAdmin 要求当前请求来自平台管理员。
func (a *API) requirePlatformAdmin() gin.HandlerFunc {
	return auth.RequirePlatformIdentity()
}

// requireTeacher 要求当前账号具备教师或学校管理员角色。
func (a *API) requireTeacher() gin.HandlerFunc {
	return auth.RequirePlatformOrAnyRole(a.identity, contracts.RoleTeacher, contracts.RoleSchoolAdmin)
}

// listPackages 查询仿真包列表。
func (a *API) listPackages(c *gin.Context) {
	rows, err := a.svc.ListPackages(c.Request.Context(), c.Query("category"), c.Query("keyword"), parseStatus(c.Query("status")))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// listVersions 查询指定仿真包的已上架版本。
func (a *API) listVersions(c *gin.Context) {
	rows, err := a.svc.ListVersions(c.Request.Context(), c.Param("pkg"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// getBundle 返回仿真包 bundle 引用和 hash。
func (a *API) getBundle(c *gin.Context) {
	out, err := a.svc.GetBundleRef(c.Request.Context(), c.Param("pkg"), c.Param("version"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// submitPackage 读取 multipart 仿真包并提交审核,包内容必须走后端扫描与对象存储路径。
func (a *API) submitPackage(c *gin.Context) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	// 第一步解析表单中的 JSON 配置,上传包体还未读取,失败不会产生对象存储副作用。
	scaleLimit, err := jsonForm(c.PostForm("scale_limit"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	backendConfig, err := jsonForm(c.PostForm("backend_config"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	req := SubmitPackageRequest{
		Code: c.PostForm("code"), Version: c.PostForm("version"), Name: c.PostForm("name"),
		Category: c.PostForm("category"), Compute: c.PostForm("compute"),
		BackendAdapter: c.PostForm("backend_adapter"), AuthorID: c.PostForm("author_id"),
		ScaleLimit: scaleLimit, BackendConfig: backendConfig,
	}
	// 第二步补齐教师作者默认值,但仍交由校验函数确认命名空间和作者身份一致。
	if req.AuthorID == "" {
		req.AuthorID = ids.Format(id.AccountID)
	}
	authorType, _ := strconv.Atoi(c.PostForm("author_type"))
	req.AuthorType = int16(authorType)
	if req.AuthorType == 0 {
		req.AuthorType = AuthorTypeTeacher
	}
	if err := validatePackageUploadMetadata(id, req); err != nil {
		response.Fail(c, err)
		return
	}
	// 先校验元数据再读取上传包,避免无效请求提前污染对象存储。
	bundle, err := a.readAndStoreBundle(c, id.TenantID, req.Code, req.Version)
	if err != nil {
		response.Fail(c, err)
		return
	}
	req.BundleHash = bundle.Hash
	req.BundleKey = bundle.Key
	// 第三步把扫描报告和对象引用交给服务层提交审核,API 层不直接写包版本表。
	row, err := a.svc.SubmitUploadedPackage(c.Request.Context(), req, bundle.ScanReport)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// readAndStoreBundle 读取仿真包上传体并委托服务层完成扫描、hash 和对象存储。
func (a *API) readAndStoreBundle(c *gin.Context, tenantID int64, code, version string) (uploadedBundle, error) {
	file, header, err := c.Request.FormFile("bundle")
	if err != nil {
		return uploadedBundle{}, apperr.ErrSimPackageInvalid
	}
	if err := validateSimBundleUploadSize(header.Size, a.upload); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return uploadedBundle{}, apperr.ErrSimBundleReadFail.WithCause(closeErr)
		}
		return uploadedBundle{}, err
	}
	data, err := io.ReadAll(io.LimitReader(file, simBundleReadLimit(a.upload)))
	closeErr := file.Close()
	if err != nil {
		return uploadedBundle{}, apperr.ErrSimBundleReadFail.WithCause(err)
	}
	if closeErr != nil {
		return uploadedBundle{}, apperr.ErrSimBundleReadFail.WithCause(closeErr)
	}
	if a.upload.SimBundleMaxBytes > 0 && int64(len(data)) > a.upload.SimBundleMaxBytes {
		return uploadedBundle{}, apperr.ErrSimBundleTooLarge
	}
	return a.svc.StoreUploadedBundle(c.Request.Context(), tenantID, code, version, header.Filename, header.Header.Get("Content-Type"), data, a.upload)
}

// validateSimBundleUploadSize 把平台上传大小结果映射为 M4 用户向错误码。
func validateSimBundleUploadSize(size int64, uploadCfg config.UploadConfig) error {
	switch upload.CheckSize(size, uploadCfg.SimBundleMaxBytes) {
	case upload.SizeOK:
		return nil
	case upload.SizeEmpty:
		return apperr.ErrSimPackageInvalid
	case upload.SizeTooLarge:
		return apperr.ErrSimBundleTooLarge
	default:
		return apperr.ErrSimPackageInvalid
	}
}

// validatePackageUploadMetadata 在读取和写入 bundle 前校验元数据与提交者身份,避免非法请求污染对象存储。
func validatePackageUploadMetadata(id tenant.Identity, req SubmitPackageRequest) error {
	candidate := req
	if candidate.BundleKey == "" {
		candidate.BundleKey = "pending"
	}
	if candidate.BundleHash == "" {
		candidate.BundleHash = "0000000000000000000000000000000000000000000000000000000000000000"
	}
	if err := validateSubmitPackageRequest(candidate); err != nil {
		return err
	}
	return validatePackageSubmitterAccess(id, candidate)
}

// simBundleReadLimit 返回上传读取上限,未配置时使用 int64 最大值表示不额外截断。
func simBundleReadLimit(upload config.UploadConfig) int64 {
	if upload.SimBundleMaxBytes <= 0 {
		return 1<<63 - 1
	}
	return upload.SimBundleMaxBytes + 1
}

// updateValidationReport 保存受控预览流程回写的审核报告。
func (a *API) updateValidationReport(c *gin.Context) {
	packageID, ok := simRouteID(c, "pkg", apperr.ErrSimPackageInvalid)
	if !ok {
		return
	}
	var req ValidationReportRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimPackageValidationFail) {
		return
	}
	row, err := a.svc.UpdateValidationReport(c.Request.Context(), packageID, req.Report)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// previewPackage 返回审核预览入口所需的包摘要。
func (a *API) previewPackage(c *gin.Context) {
	packageID, ok := simRouteID(c, "pkg", apperr.ErrSimPackageInvalid)
	if !ok {
		return
	}
	out, err := a.svc.GetPackagePreview(c.Request.Context(), packageID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// updatePackage 更新草稿或退回仿真包,新 bundle 同样必须重新扫描后才能替换。
func (a *API) updatePackage(c *gin.Context) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	packageID, ok := simRouteID(c, "pkg", apperr.ErrSimPackageInvalid)
	if !ok {
		return
	}
	// 先读取服务端包元数据,code/version/author 不接受客户端在更新时重传覆盖。
	target, err := a.svc.GetPackagePreview(c.Request.Context(), packageID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	scaleLimit, err := jsonForm(c.PostForm("scale_limit"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	backendConfig, err := jsonForm(c.PostForm("backend_config"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	code, _ := target["code"].(string)
	version, _ := target["version"].(string)
	compute, _ := target["compute"].(string)
	authorID, _ := target["author_id"].(string)
	authorType, _ := target["author_type"].(int16)
	if strings.TrimSpace(code) == "" || strings.TrimSpace(version) == "" {
		response.Fail(c, apperr.ErrSimPackageInvalid)
		return
	}
	req := UpdatePackageRequest{
		Name: c.PostForm("name"), Category: c.PostForm("category"), ScaleLimit: scaleLimit,
		BackendAdapter: c.PostForm("backend_adapter"), BackendConfig: backendConfig,
	}
	if err := validatePackageUploadMetadata(id, SubmitPackageRequest{
		Code: code, Version: version, Name: req.Name, Category: req.Category,
		Compute: compute, ScaleLimit: req.ScaleLimit,
		BundleKey: "pending", BundleHash: "0000000000000000000000000000000000000000000000000000000000000000",
		BackendAdapter: req.BackendAdapter, BackendConfig: req.BackendConfig,
		AuthorType: authorType, AuthorID: authorID,
	}); err != nil {
		response.Fail(c, err)
		return
	}
	// 再读取并保存新包体,服务层只接收扫描后的 bundle_key/hash。
	bundle, err := a.readAndStoreBundle(c, id.TenantID, code, version)
	if err != nil {
		response.Fail(c, err)
		return
	}
	req.BundleKey = bundle.Key
	req.BundleHash = bundle.Hash
	row, err := a.svc.UpdateUploadedPackage(c.Request.Context(), packageID, req, bundle.ScanReport)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// listReviews 查询审核记录。
func (a *API) listReviews(c *gin.Context) {
	rows, err := a.svc.ListReviews(c.Request.Context(), parseReviewResult(c.Query("result")))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// approveReview 审核通过仿真包。
func (a *API) approveReview(c *gin.Context) {
	reviewID, ok := simRouteID(c, "id", apperr.ErrSimReviewNotFound)
	if !ok {
		return
	}
	var req ReviewRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimReviewInvalidState) {
		return
	}
	row, err := a.svc.ApproveReview(c.Request.Context(), reviewID, req.Comment)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// rejectReview 审核退回仿真包。
func (a *API) rejectReview(c *gin.Context) {
	reviewID, ok := simRouteID(c, "id", apperr.ErrSimReviewNotFound)
	if !ok {
		return
	}
	var req ReviewRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimReviewInvalidState) {
		return
	}
	row, err := a.svc.RejectReview(c.Request.Context(), reviewID, req.Comment)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// archivePackage 下架已发布仿真包。
func (a *API) archivePackage(c *gin.Context) {
	packageID, ok := simRouteID(c, "pkg", apperr.ErrSimPackageInvalid)
	if !ok {
		return
	}
	row, err := a.svc.ArchivePackage(c.Request.Context(), packageID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// republishPackage 重新上架已下架仿真包。
func (a *API) republishPackage(c *gin.Context) {
	packageID, ok := simRouteID(c, "pkg", apperr.ErrSimPackageInvalid)
	if !ok {
		return
	}
	row, err := a.svc.RepublishPackage(c.Request.Context(), packageID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// createSession 创建仿真会话。
func (a *API) createSession(c *gin.Context) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	var req CreateSessionRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimSessionInvalid) {
		return
	}
	row, err := a.svc.CreateSession(c.Request.Context(), id.TenantID, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// reportAction 上报仿真用户操作。
func (a *API) reportAction(c *gin.Context) {
	sessionID, ok := simRouteID(c, "id", apperr.ErrSimSessionInvalid)
	if !ok {
		return
	}
	var req ReportActionRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimActionInvalid) {
		return
	}
	row, err := a.svc.ReportAction(c.Request.Context(), sessionID, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// getReplay 查询会话回放数据。
func (a *API) getReplay(c *gin.Context) {
	sessionID, ok := simRouteID(c, "id", apperr.ErrSimSessionInvalid)
	if !ok {
		return
	}
	row, err := a.svc.GetReplay(c.Request.Context(), sessionID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// streamSession 建立后端计算仿真流。
func (a *API) streamSession(c *gin.Context) {
	sessionID, ok := simRouteID(c, "id", apperr.ErrSimSessionInvalid)
	if !ok {
		return
	}
	if err := a.serveBackendStreamWS(c.Writer, c.Request, sessionID); err != nil {
		response.Fail(c, err)
	}
}

// archiveSession 归档单个会话。
func (a *API) archiveSession(c *gin.Context) {
	sessionID, ok := simRouteID(c, "id", apperr.ErrSimSessionInvalid)
	if !ok {
		return
	}
	if err := a.svc.ArchiveSession(c.Request.Context(), sessionID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, map[string]any{"archived": true})
}

// recycleSessions 按 source_ref 归档会话。
func (a *API) recycleSessions(c *gin.Context) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	var req struct {
		SourceRef string `json:"source_ref"`
		Reason    string `json:"reason"`
	}
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimSessionInvalid) {
		return
	}
	if err := a.svc.RecycleBySourceRef(c.Request.Context(), id.TenantID, req.SourceRef, req.Reason); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, map[string]any{"archived": true})
}

// shareSession 创建分享码。
func (a *API) shareSession(c *gin.Context) {
	sessionID, ok := simRouteID(c, "id", apperr.ErrSimSessionInvalid)
	if !ok {
		return
	}
	row, err := a.svc.ShareSession(c.Request.Context(), sessionID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// getSharedReplay 按分享码读取剧本。
func (a *API) getSharedReplay(c *gin.Context) {
	if err := validateShareCode(c.Param("code")); err != nil {
		response.Fail(c, err)
		return
	}
	row, err := a.svc.GetSharedReplay(c.Request.Context(), c.Param("code"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// reportCheckpoint 上报检查点结果快照。
func (a *API) reportCheckpoint(c *gin.Context) {
	sessionID, ok := simRouteID(c, "id", apperr.ErrSimSessionInvalid)
	if !ok {
		return
	}
	var req ReportCheckpointRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSimCheckpointInvalid) {
		return
	}
	if err := a.svc.ReportCheckpoint(c.Request.Context(), sessionID, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, map[string]any{"saved": true})
}

// simRouteID 解析指定路径 ID,调用点传入所属业务域错误码以保持响应精确。
func simRouteID(c *gin.Context, name string, invalid *apperr.Error) (int64, bool) {
	id, ok := ids.Parse(c.Param(name))
	if !ok {
		response.Fail(c, invalid)
		return 0, false
	}
	return id, true
}

// parseStatus 把列表状态筛选参数转为枚举。
func parseStatus(v string) int16 {
	switch v {
	case "draft":
		return PackageStatusDraft
	case "reviewing":
		return PackageStatusReviewing
	case "published", "":
		return PackageStatusPublished
	case "archived":
		return PackageStatusArchived
	case "rejected":
		return PackageStatusRejected
	default:
		n, _ := strconv.Atoi(v)
		return int16(n)
	}
}

// parseReviewResult 把审核结果筛选参数转为枚举。
func parseReviewResult(v string) int16 {
	switch v {
	case "pending", "":
		return ReviewResultPending
	case "approved":
		return ReviewResultApproved
	case "rejected":
		return ReviewResultRejected
	default:
		n, _ := strconv.Atoi(v)
		return int16(n)
	}
}

// jsonForm 解析表单中的 JSON 对象字段,非法输入返回模块错误码避免静默丢配置。
func jsonForm(raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, apperr.ErrSimPackageInvalid.WithCause(err)
	}
	if out == nil {
		return nil, apperr.ErrSimPackageInvalid
	}
	return out, nil
}
