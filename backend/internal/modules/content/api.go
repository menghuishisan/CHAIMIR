// M5 HTTP 接口层:注册 /api/v1/content 下的内容、版本、共享、分类、组卷与内部取用接口。
package content

import (
	"context"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// API 是 M5 的 HTTP 处理器。
type API struct {
	svc      *Service
	authMgr  *auth.Manager
	identity contracts.IdentityService
}

// NewAPI 构造 M5 API。
func NewAPI(svc *Service, authMgr *auth.Manager, identity contracts.IdentityService) *API {
	return &API{svc: svc, authMgr: authMgr, identity: identity}
}

// Register 注册 M5 路由,并按教师/内部边界挂载鉴权。
func (a *API) Register(rg *gin.RouterGroup) {
	g := rg.Group("/content")
	{
		user := a.authMgr.Middleware()
		teacher := []gin.HandlerFunc{user, a.requireTeacher()}
		g.GET("/items", append(teacher, a.listItems)...)
		g.POST("/items", append(teacher, a.createItem)...)
		g.GET("/items/:item/:version", append(teacher, a.getFace)...)
		g.GET("/items/:item/:version/full", a.requireFullReader(), a.getFull)
		g.PATCH("/items/:item", append(teacher, a.updateDraft)...)
		g.POST("/items/:item/publish", append(teacher, a.publish)...)
		g.POST("/items/:item/deprecate", append(teacher, a.deprecate)...)
		g.DELETE("/items/:item", append(teacher, a.deleteDraft)...)
		g.GET("/items/:item/versions", append(teacher, a.listVersions)...)
		g.POST("/items/:item/new-version", append(teacher, a.newVersion)...)
		g.POST("/items/:item/:version/clone", append(teacher, a.clone)...)
		g.POST("/items/:item/share", append(teacher, a.share)...)
		g.POST("/items/:item/unshare", append(teacher, a.unshare)...)
		g.GET("/shared", append(teacher, a.listShared)...)
		g.GET("/categories", append(teacher, a.listCategories)...)
		g.POST("/categories", append(teacher, a.createCategory)...)
		g.PATCH("/categories/:id", append(teacher, a.updateCategory)...)
		g.DELETE("/categories/:id", append(teacher, a.deleteCategory)...)
		g.GET("/papers", append(teacher, a.listPapers)...)
		g.POST("/papers", append(teacher, a.createPaper)...)
		g.GET("/papers/:id", append(teacher, a.getPaper)...)
		g.POST("/papers/:id/regenerate", append(teacher, a.regeneratePaper)...)
	}
	internal := rg.Group("/content", a.authMgr.ServiceMiddleware())
	{
		internal.POST("/items/system-import", a.systemImport)
		internal.POST("/items/batch", a.batchGetFace)
		internal.POST("/items/:item/:version/usage", a.incrementUsage)
	}
}

// requireFullReader 允许教师/管理员 JWT 或内部服务 HMAC 读取 full 内容。
func (a *API) requireFullReader() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(c.GetHeader(auth.ServiceNameHeader)) != "" || strings.TrimSpace(c.GetHeader(auth.ServiceSignatureHeader)) != "" {
			a.authMgr.ServiceMiddleware()(c)
			return
		}
		a.authMgr.Middleware()(c)
		if c.IsAborted() {
			return
		}
		a.requireTeacher()(c)
	}
}

// requireTeacher 要求当前账号具备教师或学校管理员角色。
func (a *API) requireTeacher() gin.HandlerFunc {
	return auth.RequirePlatformOrAnyRole(a.identity, contracts.RoleTeacher, contracts.RoleSchoolAdmin)
}

// listItems 查询内容列表。
func (a *API) listItems(c *gin.Context) {
	items, total, err := a.svc.ListItems(c.Request.Context(), ListItemsRequest{
		Type: httpx.Int16(c.Query("type")), CategoryID: c.Query("category"), Difficulty: httpx.Int16(c.Query("difficulty")),
		Tag: c.Query("tag"), KP: c.Query("kp"), Keyword: c.Query("keyword"), Visibility: httpx.Int16(c.Query("visibility")),
		Status: httpx.Int16(c.Query("status")), Page: httpx.Int(c.Query("page")), Size: httpx.Int(c.Query("size")),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	response.OKPage(c, items, total, page, size)
}

// createItem 创建内容草稿。
func (a *API) createItem(c *gin.Context) {
	var req CreateItemRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentRequestInvalid) {
		return
	}
	out, err := a.svc.CreateItem(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// systemImport 处理系统/外部源自动建题入口。
func (a *API) systemImport(c *gin.Context) {
	var req CreateItemRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentRequestInvalid) {
		return
	}
	if req.AuthorType == 0 {
		req.AuthorType = AuthorTypeSystem
	}
	out, err := a.svc.CreateItem(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// getFace 读取题面视角内容。
func (a *API) getFace(c *gin.Context) {
	out, err := a.svc.GetFace(c.Request.Context(), contentItemCode(c), c.Param("version"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// getFull 读取全量内容。
func (a *API) getFull(c *gin.Context) {
	if _, ok := auth.ServiceSourceRefFromContext(c.Request.Context()); ok {
		a.getFullInternal(c)
		return
	}
	out, err := a.svc.GetFull(c.Request.Context(), contentItemCode(c), c.Param("version"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// getFullInternal 读取内部服务签名授权的全量内容,仅使用服务端注入租户边界。
func (a *API) getFullInternal(c *gin.Context) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	if a.svc == nil {
		response.Fail(c, apperr.ErrContentQueryFailed)
		return
	}
	out, err := a.svc.getContentFullInTenant(c.Request.Context(), id.TenantID, contentItemCode(c), c.Param("version"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// updateDraft 更新草稿。
func (a *API) updateDraft(c *gin.Context) {
	id, ok := contentPathID(c, apperr.ErrContentIDInvalid)
	if !ok {
		return
	}
	var req UpdateItemRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentRequestInvalid) {
		return
	}
	out, err := a.svc.UpdateDraft(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// publish 发布内容。
func (a *API) publish(c *gin.Context) {
	a.itemStatusAction(c, a.svc.Publish)
}

// deprecate 弃用内容。
func (a *API) deprecate(c *gin.Context) {
	a.itemStatusAction(c, a.svc.Deprecate)
}

// deleteDraft 删除草稿。
func (a *API) deleteDraft(c *gin.Context) {
	id, ok := contentPathID(c, apperr.ErrContentIDInvalid)
	if !ok {
		return
	}
	if err := a.svc.DeleteDraft(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, map[string]any{"deleted": true})
}

// listVersions 查询版本列表。
func (a *API) listVersions(c *gin.Context) {
	out, err := a.svc.ListVersions(c.Request.Context(), contentItemCode(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// newVersion 创建新版本草稿。
func (a *API) newVersion(c *gin.Context) {
	var req NewVersionRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentVersionRequestInvalid) {
		return
	}
	out, err := a.svc.NewVersion(c.Request.Context(), contentItemCode(c), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// clone 克隆内容。
func (a *API) clone(c *gin.Context) {
	var req CloneRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentCloneRequestInvalid) {
		return
	}
	out, err := a.svc.Clone(c.Request.Context(), contentItemCode(c), c.Param("version"), req.Code)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// share 共享内容。
func (a *API) share(c *gin.Context) {
	a.itemStatusAction(c, a.svc.Share)
}

// unshare 取消共享。
func (a *API) unshare(c *gin.Context) {
	a.itemStatusAction(c, a.svc.Unshare)
}

// listShared 查询共享库。
func (a *API) listShared(c *gin.Context) {
	out, err := a.svc.ListShared(c.Request.Context(), httpx.Int16(c.Query("type")), c.Query("keyword"), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// createCategory 创建分类。
func (a *API) createCategory(c *gin.Context) {
	var req CategoryRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentCategoryInvalid) {
		return
	}
	out, err := a.svc.CreateCategory(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// listCategories 查询分类。
func (a *API) listCategories(c *gin.Context) {
	out, err := a.svc.ListCategories(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// updateCategory 更新分类。
func (a *API) updateCategory(c *gin.Context) {
	id, ok := contentPathID(c, apperr.ErrContentCategoryInvalid)
	if !ok {
		return
	}
	var req CategoryRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentCategoryInvalid) {
		return
	}
	out, err := a.svc.UpdateCategory(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// deleteCategory 删除分类。
func (a *API) deleteCategory(c *gin.Context) {
	id, ok := contentPathID(c, apperr.ErrContentCategoryInvalid)
	if !ok {
		return
	}
	if err := a.svc.DeleteCategory(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, map[string]any{"deleted": true})
}

// createPaper 创建试卷。
func (a *API) createPaper(c *gin.Context) {
	var req PaperRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrPaperRequestInvalid) {
		return
	}
	out, err := a.svc.CreatePaper(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// listPapers 查询试卷列表。
func (a *API) listPapers(c *gin.Context) {
	out, err := a.svc.ListPapers(c.Request.Context(), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// getPaper 查询试卷详情。
func (a *API) getPaper(c *gin.Context) {
	id, ok := contentPathID(c, apperr.ErrPaperIDInvalid)
	if !ok {
		return
	}
	out, err := a.svc.GetPaper(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// regeneratePaper 重新抽题。
func (a *API) regeneratePaper(c *gin.Context) {
	id, ok := contentPathID(c, apperr.ErrPaperIDInvalid)
	if !ok {
		return
	}
	var req PaperRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrPaperRequestInvalid) {
		return
	}
	out, err := a.svc.RegeneratePaper(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// batchGetFace 批量读取题面。
func (a *API) batchGetFace(c *gin.Context) {
	var req BatchGetRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentRequestInvalid) {
		return
	}
	out := make([]ItemDTO, 0, len(req.Items))
	for _, ref := range req.Items {
		item, err := a.svc.GetFace(c.Request.Context(), ref.Code, ref.Version)
		if err != nil {
			response.Fail(c, err)
			return
		}
		out = append(out, item)
	}
	response.OK(c, out)
}

// incrementUsage 记录引用计数。
func (a *API) incrementUsage(c *gin.Context) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	if err := a.svc.IncrementUsage(c.Request.Context(), id.TenantID, contentItemCode(c), c.Param("version")); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, map[string]any{"counted": true})
}

// itemStatusAction 执行只需路径 ID 的内容操作。
func (a *API) itemStatusAction(c *gin.Context, fn func(context.Context, int64) (ItemDTO, error)) {
	id, ok := contentPathID(c, apperr.ErrContentIDInvalid)
	if !ok {
		return
	}
	out, err := fn(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// contentPathID 解析路径 ID,失败时写调用方指定的 M5 业务错误码。
func contentPathID(c *gin.Context, invalid *apperr.Error) (int64, bool) {
	id, ok := ids.Parse(contentItemID(c))
	if !ok {
		response.Fail(c, invalid)
		return 0, false
	}
	return id, true
}

// contentItemID 读取文档定义为 item ID 的路径参数;调用方必须继续按数字 ID 校验。
func contentItemID(c *gin.Context) string {
	return c.Param("item")
}

// contentItemCode 读取文档定义为内容 code 的路径参数,不参与 ID 操作。
func contentItemCode(c *gin.Context) string {
	return c.Param("item")
}
