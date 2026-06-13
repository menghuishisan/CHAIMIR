// identity api_org 文件承接组织架构 CRUD、班级归档和升级 HTTP 请求。
package identity

import (
	"io"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// orgAPI 封装组织架构 HTTP handler 依赖。
type orgAPI struct {
	svc *Service
}

// registerOrgRoutes 注册学校管理员组织架构维护路由。
func registerOrgRoutes(r gin.IRouter, svc *Service, authn *auth.Manager) {
	api := orgAPI{svc: svc}
	g := r.Group("/org", authn.Middleware(), auth.RequireTenantAnyRole(svc, contracts.RoleSchoolAdmin))
	api.register(g)
}

// register 把组织架构各资源路由绑定到具名 handler。
func (a orgAPI) register(g gin.IRouter) {
	g.GET("/departments", a.listDepartments)
	g.POST("/departments", a.createDepartment)
	g.PATCH("/departments/:id", a.updateDepartment)
	g.DELETE("/departments/:id", a.deleteDepartment)
	g.GET("/majors", a.listMajors)
	g.POST("/majors", a.createMajor)
	g.PATCH("/majors/:id", a.updateMajor)
	g.DELETE("/majors/:id", a.deleteMajor)
	g.GET("/classes", a.listClasses)
	g.POST("/classes", a.createClass)
	g.PATCH("/classes/:id", a.updateClass)
	g.DELETE("/classes/:id", a.deleteClass)
	g.POST("/import/preview", a.importOrgPreview)
	g.POST("/import/commit", a.importOrgCommit)
	g.GET("/import/template", a.importOrgTemplate)
	g.POST("/classes/archive", a.archiveClasses)
	g.POST("/classes/promote", a.promoteClasses)
}

// listDepartments 返回当前租户院系列表。
func (a orgAPI) listDepartments(c *gin.Context) {
	out, err := a.svc.ListDepartmentsByAdmin(c.Request.Context())
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// createDepartment 绑定创建院系请求。
func (a orgAPI) createDepartment(c *gin.Context) {
	var req DepartmentRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.CreateDepartmentByAdmin(c.Request.Context(), req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// updateDepartment 绑定更新院系请求。
func (a orgAPI) updateDepartment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req DepartmentRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.UpdateDepartmentByAdmin(c.Request.Context(), id, req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// deleteDepartment 绑定删除院系请求。
func (a orgAPI) deleteDepartment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.DeleteDepartmentByAdmin(c.Request.Context(), id); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// listMajors 返回专业列表,支持 department_id 查询过滤。
func (a orgAPI) listMajors(c *gin.Context) {
	departmentID, ok := httpx.QueryInt(c, "department_id", httpx.QueryIntRule{BitSize: 64, Min: 0})
	if !ok {
		return
	}
	out, err := a.svc.ListMajorsByAdmin(c.Request.Context(), departmentID)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// createMajor 绑定创建专业请求。
func (a orgAPI) createMajor(c *gin.Context) {
	var req MajorRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.CreateMajorByAdmin(c.Request.Context(), req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// updateMajor 绑定更新专业请求。
func (a orgAPI) updateMajor(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req MajorRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.UpdateMajorByAdmin(c.Request.Context(), id, req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// deleteMajor 绑定删除专业请求。
func (a orgAPI) deleteMajor(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.DeleteMajorByAdmin(c.Request.Context(), id); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// listClasses 返回班级列表,支持 major_id 查询过滤。
func (a orgAPI) listClasses(c *gin.Context) {
	majorID, ok := httpx.QueryInt(c, "major_id", httpx.QueryIntRule{BitSize: 64, Min: 0})
	if !ok {
		return
	}
	out, err := a.svc.ListClassesByAdmin(c.Request.Context(), majorID)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// createClass 绑定创建班级请求。
func (a orgAPI) createClass(c *gin.Context) {
	var req ClassRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.CreateClassByAdmin(c.Request.Context(), req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// updateClass 绑定更新班级请求。
func (a orgAPI) updateClass(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ClassRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.UpdateClassByAdmin(c.Request.Context(), id, req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// deleteClass 绑定删除班级请求。
func (a orgAPI) deleteClass(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.DeleteClassByAdmin(c.Request.Context(), id); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// importOrgPreview 绑定组织架构导入预览 multipart 请求。
func (a orgAPI) importOrgPreview(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityImportContentInvalid.WithCause(err))
		return
	}
	if maxBytes := a.svc.importMaxBytes(); maxBytes > 0 && fileHeader.Size > maxBytes {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityImportFileTooLarge)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityImportContentInvalid.WithCause(err))
		return
	}
	content, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityImportContentInvalid.WithCause(readErr))
		return
	}
	if closeErr != nil {
		httpx.Write(c, gin.H{}, apperr.ErrInternal.WithCause(closeErr))
		return
	}
	out, err := a.svc.PreviewOrgImportByAdmin(c.Request.Context(), ImportPreviewRequest{
		TargetType:  ImportTargetOrg,
		FileName:    fileHeader.Filename,
		ContentType: fileHeader.Header.Get("Content-Type"),
		Content:     content,
	})
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// importOrgCommit 绑定组织架构导入提交请求。
func (a orgAPI) importOrgCommit(c *gin.Context) {
	var req ImportCommitRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.CommitOrgImportByAdmin(c.Request.Context(), req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// importOrgTemplate 绑定组织架构导入模板下载请求。
func (a orgAPI) importOrgTemplate(c *gin.Context) {
	tpl, err := a.svc.OrgImportTemplate(c.Query("format"))
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.WriteAttachment(c, tpl.FileName, tpl.ContentType, tpl.Content)
}

// archiveClasses 绑定按入学年份归档班级请求。
func (a orgAPI) archiveClasses(c *gin.Context) {
	var req ArchiveClassesRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := a.svc.ArchiveClassesByAdmin(c.Request.Context(), req); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// promoteClasses 绑定班级批量升级请求。
func (a orgAPI) promoteClasses(c *gin.Context) {
	if err := a.svc.PromoteClassesByAdmin(c.Request.Context()); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}
