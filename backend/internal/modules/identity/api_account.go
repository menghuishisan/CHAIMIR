// identity api_account 文件承接学校管理员账号管理和账号导入 HTTP 请求。
package identity

import (
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// accountAPI 封装账号管理 HTTP handler 依赖。
type accountAPI struct {
	svc *Service
}

// registerAccountRoutes 注册账号管理和导入路由。
func registerAccountRoutes(r gin.IRouter, svc *Service, authn *auth.Manager) {
	api := accountAPI{svc: svc}
	g := r.Group("/accounts", authn.Middleware(), auth.RequireTenantAnyRole(svc, contracts.RoleSchoolAdmin))
	api.register(g)
}

// register 绑定账号管理资源路由到具名 handler。
func (a accountAPI) register(g gin.IRouter) {
	g.GET("", a.listAccounts)
	g.POST("", a.createAccount)
	g.PATCH("/:id", a.updateAccount)
	g.POST("/:id/disable", a.disableAccount)
	g.POST("/:id/enable", a.enableAccount)
	g.POST("/:id/archive", a.archiveAccount)
	g.POST("/:id/restore", a.restoreAccount)
	g.POST("/:id/cancel", a.cancelAccount)
	g.POST("/:id/force-logout", a.forceLogout)
	g.POST("/:id/reset-password", a.resetPassword)
	g.POST("/:id/grant-admin", a.grantAdmin)
	g.POST("/:id/revoke-admin", a.revokeAdmin)
	g.POST("/batch/disable", a.batchDisable)
	g.POST("/batch/archive", a.batchArchive)
	g.POST("/batch/restore", a.batchRestore)
	g.POST("/import/preview", a.importPreview)
	g.POST("/import/commit", a.importCommit)
	g.GET("/import/template", a.importTemplate)
	g.GET("/import/batches", a.importBatches)
}

// listAccounts 绑定账号列表查询参数并委托 service 分页读取。
func (a accountAPI) listAccounts(c *gin.Context) {
	query, ok := bindAccountQuery(c)
	if !ok {
		return
	}
	list, total, page, size, err := a.svc.ListAccountsByAdmin(c.Request.Context(), query)
	httpx.WritePage(c, list, total, page, size, err)
}

// createAccount 绑定单个账号创建请求。
func (a accountAPI) createAccount(c *gin.Context) {
	var req CreateAccountRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	dto, activation, err := a.svc.CreateAccountByAdmin(c.Request.Context(), req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{"account": dto, "activation_code": activation}, nil)
}

// updateAccount 绑定账号可编辑字段更新请求。
func (a accountAPI) updateAccount(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req UpdateAccountRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.UpdateAccountByAdmin(c.Request.Context(), id, req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// disableAccount 停用账号并吊销会话。
func (a accountAPI) disableAccount(c *gin.Context) {
	a.updateStatus(c, AccountStatusDisabled)
}

// enableAccount 启用账号。
func (a accountAPI) enableAccount(c *gin.Context) {
	a.updateStatus(c, AccountStatusActive)
}

// archiveAccount 归档账号并吊销会话。
func (a accountAPI) archiveAccount(c *gin.Context) {
	a.updateStatus(c, AccountStatusArchived)
}

// restoreAccount 恢复归档账号为正常状态。
func (a accountAPI) restoreAccount(c *gin.Context) {
	a.updateStatus(c, AccountStatusActive)
}

// cancelAccount 注销账号并写软删除标记。
func (a accountAPI) cancelAccount(c *gin.Context) {
	a.updateStatus(c, AccountStatusCancelled)
}

// updateStatus 统一绑定单账号状态流转请求。
func (a accountAPI) updateStatus(c *gin.Context, status int16) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.UpdateAccountStatusByAdmin(c.Request.Context(), id, status); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// forceLogout 强制吊销指定账号所有会话。
func (a accountAPI) forceLogout(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.ForceLogoutAccountByAdmin(c.Request.Context(), id); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// resetPassword 绑定管理员重置密码请求。
func (a accountAPI) resetPassword(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req AdminResetPasswordRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := a.svc.ResetAccountPasswordByAdmin(c.Request.Context(), id, req); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// grantAdmin 授予教师学校管理员角色。
func (a accountAPI) grantAdmin(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.GrantSchoolAdmin(c.Request.Context(), id); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// revokeAdmin 撤销学校管理员角色。
func (a accountAPI) revokeAdmin(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.RevokeSchoolAdmin(c.Request.Context(), id); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// batchDisable 批量停用账号。
func (a accountAPI) batchDisable(c *gin.Context) {
	a.batchStatus(c, AccountStatusDisabled)
}

// batchArchive 按入学年份批量归档学生账号,同时同步班级归档状态。
func (a accountAPI) batchArchive(c *gin.Context) {
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

// batchRestore 批量恢复账号。
func (a accountAPI) batchRestore(c *gin.Context) {
	a.batchStatus(c, AccountStatusActive)
}

// batchStatus 绑定批量账号状态流转请求。
func (a accountAPI) batchStatus(c *gin.Context, status int16) {
	var req BatchAccountIDsRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := a.svc.BatchUpdateAccountStatusByAdmin(c.Request.Context(), req, status); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// importPreview 绑定账号导入预览请求。
func (a accountAPI) importPreview(c *gin.Context) {
	targetType, ok := parseImportTarget(c.PostForm("type"))
	if !ok {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityImportTypeInvalid)
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityImportContentInvalid.WithCause(err))
		return
	}
	if maxBytes := a.svc.importMaxBytes(); maxBytes > 0 && fileHeader.Size > maxBytes {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityImportFileTooLarge)
		return
	}
	// API 层只做上传边界检查和读取,文件类型与逐行校验交给 service 统一处理。
	file, err := fileHeader.Open()
	if err != nil {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityImportContentInvalid.WithCause(err))
		return
	}
	content, sizeResult, readErr := upload.ReadBounded(file, a.svc.importMaxBytes())
	closeErr := file.Close()
	if readErr != nil {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityImportContentInvalid.WithCause(readErr))
		return
	}
	if sizeResult == upload.SizeTooLarge {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityImportFileTooLarge)
		return
	}
	if sizeResult == upload.SizeEmpty {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityImportContentInvalid)
		return
	}
	if closeErr != nil {
		httpx.Write(c, gin.H{}, apperr.ErrInternal.WithCause(closeErr))
		return
	}
	// 组装最小预览请求,避免 API 层理解导入业务状态机。
	req := ImportPreviewRequest{
		TargetType:  targetType,
		FileName:    fileHeader.Filename,
		ContentType: fileHeader.Header.Get("Content-Type"),
		Content:     content,
	}
	out, err := a.svc.PreviewAccountImport(c.Request.Context(), req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// importTemplate 绑定模板类型和格式查询参数并返回下载文件。
func (a accountAPI) importTemplate(c *gin.Context) {
	targetType, ok := parseImportTarget(c.Query("type"))
	if !ok {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityImportTypeInvalid)
		return
	}
	tpl, err := a.svc.ImportTemplate(targetType, c.Query("format"))
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.WriteAttachment(c, tpl.FileName, tpl.ContentType, tpl.Content)
}

// importCommit 绑定账号导入提交请求。
func (a accountAPI) importCommit(c *gin.Context) {
	var req ImportCommitRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.CommitAccountImport(c.Request.Context(), req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// importBatches 读取账号导入批次历史。
func (a accountAPI) importBatches(c *gin.Context) {
	out, err := a.svc.ListImportBatchesByAdmin(c.Request.Context())
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// bindAccountQuery 解析账号列表过滤和分页查询参数。
func bindAccountQuery(c *gin.Context) (AccountQuery, bool) {
	query := AccountQuery{}
	status, ok := httpx.QueryInt(c, "status", httpx.QueryIntRule{BitSize: 16, Min: 0})
	if !ok {
		return AccountQuery{}, false
	}
	query.Status = int16(status)
	baseIdentity, ok := httpx.QueryInt(c, "role", httpx.QueryIntRule{BitSize: 16, Min: 0})
	if !ok {
		return AccountQuery{}, false
	}
	query.BaseIdentity = int16(baseIdentity)
	classID, ok := httpx.QueryInt(c, "class_id", httpx.QueryIntRule{BitSize: 64, Min: 0})
	if !ok {
		return AccountQuery{}, false
	}
	query.ClassID = classID
	page, size, ok := httpx.Page(c)
	if !ok {
		return AccountQuery{}, false
	}
	query.Page = int32(page)
	query.Size = int32(size)
	query.Keyword = c.Query("keyword")
	return query, true
}

// parseImportTarget 解析文档定义的 student/teacher 导入类型,不接受前端传数值角色。
func parseImportTarget(raw string) (int16, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "teacher":
		return ImportTargetTeacher, true
	case "student":
		return ImportTargetStudent, true
	default:
		return 0, false
	}
}
