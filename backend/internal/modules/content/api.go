// content api 文件负责注册 M5 HTTP 路由、绑定请求和组合鉴权,不承载题库业务逻辑。
package content

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册题库与模板中心 HTTP API。
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
	api := contentAPI{svc: svc, roles: roles}
	g := r.Group("/api/v1/content")
	api.registerTeacherRoutes(g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleTeacher, contracts.RoleSchoolAdmin)))
	api.registerInternalRoutes(g.Group("", authn.ServiceMiddleware()))
	g.GET("/items/:item/:version/full", authn.ServiceOrTenantAnyRoleMiddleware(roles, contracts.RoleTeacher, contracts.RoleSchoolAdmin), api.getFull)
	return nil
}

type contentAPI struct {
	svc   *Service
	roles contracts.IdentityService
}

// registerTeacherRoutes 注册教师/学校管理员题库管理接口。
func (a contentAPI) registerTeacherRoutes(g gin.IRouter) {
	g.GET("/items", a.listItems)
	g.POST("/items", a.createItem)
	g.GET("/items/:item/:version", a.getFace)
	g.PATCH("/items/:item", a.updateItem)
	g.POST("/items/:item/publish", a.publishItem)
	g.POST("/items/:item/deprecate", a.deprecateItem)
	g.DELETE("/items/:item", a.deleteItem)
	g.GET("/items/:item/versions", a.listVersions)
	g.POST("/items/:item/new-version", a.newVersion)
	g.POST("/items/:item/:version/clone", a.cloneItem)
	g.POST("/items/:item/share", a.shareItem)
	g.POST("/items/:item/unshare", a.unshareItem)
	g.GET("/shared", a.listShared)
	g.GET("/categories", a.listCategories)
	g.POST("/categories", a.createCategory)
	g.PATCH("/categories/:id", a.updateCategory)
	g.DELETE("/categories/:id", a.deleteCategory)
	g.GET("/papers", a.listPapers)
	g.POST("/papers", a.createPaper)
	g.GET("/papers/:id", a.getPaper)
	g.POST("/papers/:id/regenerate", a.regeneratePaper)
}

// registerInternalRoutes 注册内部服务取用接口。
func (a contentAPI) registerInternalRoutes(g gin.IRouter) {
	g.POST("/items/system-import", a.systemImport)
	g.POST("/items/batch", a.batchItems)
	g.POST("/items/:item/:version/usage", a.incrementUsage)
}

// listItems 绑定题库检索参数。
func (a contentAPI) listItems(c *gin.Context) {
	filter, ok := itemListFilterFromQuery(c)
	if !ok {
		return
	}
	items, total, page, size, err := a.svc.ListItems(c.Request.Context(), filter)
	httpx.WritePage(c, items, total, page, size, err)
}

// createItem 绑定教师建题请求。
func (a contentAPI) createItem(c *gin.Context) {
	var req CreateItemRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentInvalid) {
		return
	}
	out, err := a.svc.CreateItem(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// getFace 读取题面视角内容。
func (a contentAPI) getFace(c *gin.Context) {
	out, err := a.svc.GetItemFaceForUser(c.Request.Context(), c.Param("item"), c.Param("version"))
	httpx.Write(c, out, err)
}

// getFull 读取全量内容,同一路由支持内部服务和受控教师路径。
func (a contentAPI) getFull(c *gin.Context) {
	if id, ok := tenant.FromContext(c.Request.Context()); ok && id.IsSystem {
		out, err := a.svc.GetContentFull(c.Request.Context(), id.TenantID, contracts.ContentItemRef{ItemCode: c.Param("item"), ItemVersion: c.Param("version")})
		httpx.Write(c, out, err)
		return
	}
	if _, ok := currentHTTPIdentity(c); !ok {
		return
	}
	out, err := a.svc.GetItemFullForUser(c.Request.Context(), c.Param("item"), c.Param("version"))
	httpx.Write(c, out, err)
}

// updateItem 绑定草稿更新请求。
func (a contentAPI) updateItem(c *gin.Context) {
	id, ok := httpx.PathID(c, "item")
	if !ok {
		return
	}
	var req UpdateItemRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentInvalid) {
		return
	}
	out, err := a.svc.UpdateDraftItem(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// publishItem 发布草稿。
func (a contentAPI) publishItem(c *gin.Context) {
	id, ok := httpx.PathID(c, "item")
	if !ok {
		return
	}
	out, err := a.svc.PublishItem(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// deprecateItem 弃用内容。
func (a contentAPI) deprecateItem(c *gin.Context) {
	id, ok := httpx.PathID(c, "item")
	if !ok {
		return
	}
	out, err := a.svc.DeprecateItem(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// deleteItem 软删无引用草稿。
func (a contentAPI) deleteItem(c *gin.Context) {
	id, ok := httpx.PathID(c, "item")
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.DeleteItem(c.Request.Context(), id))
}

// listVersions 查询同 code 版本列表。
func (a contentAPI) listVersions(c *gin.Context) {
	out, err := a.svc.ListVersions(c.Request.Context(), c.Param("item"))
	httpx.Write(c, out, err)
}

// newVersion 从既有版本复制新草稿。
func (a contentAPI) newVersion(c *gin.Context) {
	var req NewVersionRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentVersionInvalid) {
		return
	}
	out, err := a.svc.CreateNewVersion(c.Request.Context(), c.Param("item"), req)
	httpx.Write(c, out, err)
}

// cloneItem 克隆本租户或共享内容。
func (a contentAPI) cloneItem(c *gin.Context) {
	var req CloneItemRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentCloneInvalid) {
		return
	}
	out, err := a.svc.CloneItem(c.Request.Context(), c.Param("item"), c.Param("version"), req)
	httpx.Write(c, out, err)
}

// shareItem 设置共享库可见。
func (a contentAPI) shareItem(c *gin.Context) {
	id, ok := httpx.PathID(c, "item")
	if !ok {
		return
	}
	out, err := a.svc.ShareItem(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// unshareItem 取消共享。
func (a contentAPI) unshareItem(c *gin.Context) {
	id, ok := httpx.PathID(c, "item")
	if !ok {
		return
	}
	out, err := a.svc.UnshareItem(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// listShared 浏览共享库摘要。
func (a contentAPI) listShared(c *gin.Context) {
	filter, ok := itemListFilterFromQuery(c)
	if !ok {
		return
	}
	items, total, page, size, err := a.svc.ListShared(c.Request.Context(), filter)
	httpx.WritePage(c, items, total, page, size, err)
}

// batchItems 绑定内部批量题面读取。
func (a contentAPI) batchItems(c *gin.Context) {
	var req BatchItemsRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentQueryInvalid) {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	refs, err := refsFromBatchDTO(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	out, err := a.svc.BatchGetContentFace(c.Request.Context(), tenantID, refs)
	httpx.Write(c, out, err)
}

// incrementUsage 绑定内部引用计数上报。
func (a contentAPI) incrementUsage(c *gin.Context) {
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	err := a.svc.IncrementUsage(c.Request.Context(), tenantID, contracts.ContentItemRef{ItemCode: c.Param("item"), ItemVersion: c.Param("version")})
	httpx.Write(c, gin.H{}, err)
}

// systemImport 绑定内部系统建题入口。
func (a contentAPI) systemImport(c *gin.Context) {
	var req SystemImportRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentSystemImportInvalid) {
		return
	}
	out, err := a.svc.SystemImportContentFromHTTP(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// listCategories 查询分类树。
func (a contentAPI) listCategories(c *gin.Context) {
	out, err := a.svc.ListCategories(c.Request.Context())
	httpx.Write(c, out, err)
}

// createCategory 创建分类。
func (a contentAPI) createCategory(c *gin.Context) {
	var req CategoryRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentCategoryInvalid) {
		return
	}
	out, err := a.svc.CreateCategory(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// updateCategory 更新分类。
func (a contentAPI) updateCategory(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req CategoryRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContentCategoryInvalid) {
		return
	}
	out, err := a.svc.UpdateCategory(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// deleteCategory 删除分类。
func (a contentAPI) deleteCategory(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.DeleteCategory(c.Request.Context(), id))
}

// listPapers 查询试卷分页。
func (a contentAPI) listPapers(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	items, total, p, s, err := a.svc.ListPapers(c.Request.Context(), page, size)
	httpx.WritePage(c, items, total, p, s, err)
}

// createPaper 创建试卷。
func (a contentAPI) createPaper(c *gin.Context) {
	var req CreatePaperRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrPaperInvalid) {
		return
	}
	out, err := a.svc.CreatePaper(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// getPaper 查询试卷详情。
func (a contentAPI) getPaper(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetPaperDetail(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// regeneratePaper 重新随机组卷。
func (a contentAPI) regeneratePaper(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.RegeneratePaper(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// itemListFilterFromQuery 解析内容检索分页和过滤条件。
func itemListFilterFromQuery(c *gin.Context) (ItemListFilter, bool) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return ItemListFilter{}, false
	}
	typeValue, ok := httpx.QueryInt(c, "type", httpx.QueryIntRule{Default: 0, Min: 0, Max: 3, HasMax: true})
	if !ok {
		return ItemListFilter{}, false
	}
	category, ok := httpx.QueryInt(c, "category", httpx.QueryIntRule{Default: 0, Min: 0})
	if !ok {
		return ItemListFilter{}, false
	}
	difficulty, ok := httpx.QueryInt(c, "difficulty", httpx.QueryIntRule{Default: 0, Min: 0, Max: 4, HasMax: true})
	if !ok {
		return ItemListFilter{}, false
	}
	visibility, ok := httpx.QueryInt(c, "visibility", httpx.QueryIntRule{Default: 0, Min: 0, Max: 3, HasMax: true})
	if !ok {
		return ItemListFilter{}, false
	}
	status, ok := httpx.QueryInt(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 3, HasMax: true})
	if !ok {
		return ItemListFilter{}, false
	}
	authorID, ok := httpx.QueryInt(c, "author", httpx.QueryIntRule{Default: 0, Min: 0})
	if !ok {
		return ItemListFilter{}, false
	}
	return ItemListFilter{Type: int16(typeValue), CategoryID: category, Difficulty: int16(difficulty), Tag: c.Query("tag"), KnowledgePoint: c.Query("kp"), Keyword: c.Query("keyword"), Visibility: int16(visibility), Status: int16(status), AuthorID: authorID, Page: page, Size: size}, true
}

// currentHTTPIdentity 从上下文读取租户账号身份。
func currentHTTPIdentity(c *gin.Context) (tenant.Identity, bool) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 {
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
