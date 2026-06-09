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

// NewAPI 构造 M5 HTTP 处理器,集中注入内容服务、鉴权管理器和身份只读契约。
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

// listItems 绑定内容检索过滤条件,返回教师可管理的题目/模板分页列表。
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

// createItem 绑定题目/模板创建请求,由服务层生成草稿并隔离答案字段。
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
	out, err := a.svc.SystemImportItem(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// getFace 读取学生可见的题面视角内容,避免经公开接口泄露答案和判题配置。
func (a *API) getFace(c *gin.Context) {
	out, err := a.svc.GetFace(c.Request.Context(), contentItemCode(c), c.Param("version"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// getFull 在教师/管理员或服务签名授权下读取全量内容,用于编辑和引擎内部取用。
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

// updateDraft 更新未发布草稿内容,路径 ID 和请求体都在 HTTP 边界校验。
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

// publish 将草稿发布为可复用版本,状态机校验由服务层统一执行。
func (a *API) publish(c *gin.Context) {
	a.itemStatusAction(c, a.svc.Publish)
}

// deprecate 将内容版本标记为弃用,保留历史引用但阻止继续选用。
func (a *API) deprecate(c *gin.Context) {
	a.itemStatusAction(c, a.svc.Deprecate)
}

// deleteDraft 删除未发布草稿,只返回删除确认且不暴露底层存储细节。
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

// listVersions 查询同一内容编码下的版本历史,供教师选择克隆或发版来源。
func (a *API) listVersions(c *gin.Context) {
	out, err := a.svc.ListVersions(c.Request.Context(), contentItemCode(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// newVersion 基于已有版本创建新草稿,版本号规划和来源校验交给服务层。
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

// clone 将指定版本复制为新内容编码,用于跨课程或跨租户授权复用。
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

// share 将内容加入共享库,服务层负责校验发布状态和可共享范围。
func (a *API) share(c *gin.Context) {
	a.itemStatusAction(c, a.svc.Share)
}

// unshare 取消共享库可见性,不删除原内容和已有引用。
func (a *API) unshare(c *gin.Context) {
	a.itemStatusAction(c, a.svc.Unshare)
}

// listShared 查询可复用共享内容,按类型和关键词过滤后返回给教师选用。
func (a *API) listShared(c *gin.Context) {
	out, err := a.svc.ListShared(c.Request.Context(), httpx.Int16(c.Query("type")), c.Query("keyword"), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// createCategory 创建内容分类节点,父级合法性和环检测由服务层完成。
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

// listCategories 返回当前租户分类树,供题库管理和筛选控件使用。
func (a *API) listCategories(c *gin.Context) {
	out, err := a.svc.ListCategories(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// updateCategory 更新分类名称或父级,在服务层防止自引用和循环父链。
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

// deleteCategory 删除空分类节点,保留服务层对已引用分类的保护。
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

// createPaper 创建固定或随机组卷配置,题目抽取和分值规则由服务层校验。
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

// regeneratePaper 对随机组卷重新抽题,保持试卷 ID 不变并刷新题目明细。
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
	out, err := a.svc.BatchGetFace(c.Request.Context(), req.Items)
	if err != nil {
		response.Fail(c, err)
		return
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
