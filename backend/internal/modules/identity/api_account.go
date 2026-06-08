// M1 账号管理 + 个人中心 HTTP 处理器。
package identity

import (
	"io"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// listAccounts 分页查账号(学校管理员)。
func (a *API) listAccounts(c *gin.Context) {
	page, size := pagex.Normalize(httpx.QueryInt(c, "page"), httpx.QueryInt(c, "size"))
	filter, err := buildAccountListFilter(c.Query("role"), c.Query("class_id"), c.Query("status"), c.Query("keyword"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	views, total, err := a.svc.ListAccounts(c.Request.Context(), filter, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OKPage(c, views, total, page, size)
}

// createAccount 单个建账号。
func (a *API) createAccount(c *gin.Context) {
	var req CreateAccountRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrAccountCreateInvalid) {
		return
	}
	res, err := a.svc.CreateAccount(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// updateAccount 更新账号(姓名)。
func (a *API) updateAccount(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req UpdateAccountRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrAccountUpdateInvalid) {
		return
	}
	if err := a.svc.UpdateAccount(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"updated": true})
}

// disableAccount 将账号置为停用,保留数据但禁止继续登录。
func (a *API) disableAccount(c *gin.Context) { a.setStatus(c, AccountDisabled) }

// enableAccount 将账号恢复为正常状态,用于撤销停用。
func (a *API) enableAccount(c *gin.Context) { a.setStatus(c, AccountActive) }

// archiveAccount 将离校或毕业账号归档,便于保留历史数据。
func (a *API) archiveAccount(c *gin.Context) { a.setStatus(c, AccountArchived) }

// restoreAccount 将归档账号恢复为正常状态。
func (a *API) restoreAccount(c *gin.Context) { a.setStatus(c, AccountActive) }

// cancelAccount 将账号注销到终态软删状态,避免后续再次启用。
func (a *API) cancelAccount(c *gin.Context) { a.setStatus(c, AccountCancelled) }

// setStatus 通用状态迁移处理。
func (a *API) setStatus(c *gin.Context, target int16) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.SetAccountStatus(c.Request.Context(), id, target); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"status": target})
}

// forceLogout 踢人(吊销账号全部会话)。
func (a *API) forceLogout(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.ForceLogout(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"forced_logout": true})
}

// resetAccountPassword 管理员重置他人密码(返回临时密码)。
func (a *API) resetAccountPassword(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	temp, err := a.svc.ResetAccountPassword(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"init_password": temp})
}

// grantAdmin 授予学校管理员。
func (a *API) grantAdmin(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.GrantAdmin(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"granted": true})
}

// revokeAdmin 撤销学校管理员。
func (a *API) revokeAdmin(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if err := a.svc.RevokeAdmin(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"revoked": true})
}

// importTemplate 下载教师/学生导入模板。
func (a *API) importTemplate(c *gin.Context) {
	targetType, ok := importTargetFromQuery(c.Query("type"))
	if !ok {
		response.Fail(c, apperr.ErrImportTargetInvalid)
		return
	}
	tpl, err := BuildImportTemplate(targetType, c.Query("format"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+tpl.FileName+`"`)
	c.Data(200, tpl.ContentType, tpl.Content)
}

// listImportBatches 查询导入批次历史。
func (a *API) listImportBatches(c *gin.Context) {
	page, size := pagex.Normalize(httpx.QueryInt(c, "page"), httpx.QueryInt(c, "size"))
	rows, total, err := a.svc.ListImportBatches(c.Request.Context(), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OKPage(c, rows, total, page, size)
}

// importPreview 导入预览:上传 CSV/XLSX,服务端解析并持久化预览状态。
func (a *API) importPreview(c *gin.Context) {
	id, ok := currentID(c)
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	targetType, ok := importTargetFromQuery(c.PostForm("type"))
	if !ok {
		response.Fail(c, apperr.ErrImportTargetInvalid)
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, apperr.ErrImportUploadMissing)
		return
	}
	if err := ensureImportUploadSize(fileHeader.Size, a.uploadCfg.ImportMaxBytes); err != nil {
		response.Fail(c, err)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, apperr.ErrImportFormat.WithCause(err))
		return
	}
	content, err := io.ReadAll(io.LimitReader(file, importReadLimit(a.uploadCfg.ImportMaxBytes)))
	closeErr := file.Close()
	if err != nil {
		response.Fail(c, apperr.ErrImportFormat.WithCause(err))
		return
	}
	if closeErr != nil {
		response.Fail(c, apperr.ErrImportFormat.WithCause(closeErr))
		return
	}
	if err := ensureImportUploadSize(int64(len(content)), a.uploadCfg.ImportMaxBytes); err != nil {
		response.Fail(c, err)
		return
	}
	if err := ensureImportUploadType(fileHeader.Filename, fileHeader.Header.Get("Content-Type"), content); err != nil {
		response.Fail(c, err)
		return
	}
	rows, err := ParseImportFile(fileHeader.Filename, content)
	if err != nil {
		response.Fail(c, err)
		return
	}
	res, err := a.svc.CreateImportPreview(c.Request.Context(), id.AccountID, ImportRequest{
		TargetType: targetType,
		FileName:   fileHeader.Filename,
		Rows:       rows,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// importReadLimit 返回导入文件读取上限,未配置时使用 int64 最大值。
func importReadLimit(maxBytes int64) int64 {
	if maxBytes <= 0 {
		return 1<<63 - 1
	}
	return maxBytes + 1
}

// importCommit 导入提交(仅写通过行)。
func (a *API) importCommit(c *gin.Context) {
	id, ok := currentID(c)
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	var req ImportCommitRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrImportCommitInvalid) {
		return
	}
	res, err := a.svc.CommitImportPreview(c.Request.Context(), id.AccountID, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// batchDisableAccounts 批量停用账号。
func (a *API) batchDisableAccounts(c *gin.Context) {
	a.batchSetAccountStatus(c, AccountDisabled)
}

// batchArchiveAccounts 按入学年份批量归档学生账号。
func (a *API) batchArchiveAccounts(c *gin.Context) {
	var req BatchArchiveAccountsRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrBatchAccountArchiveInvalid) {
		return
	}
	res, err := a.svc.BatchArchiveAccounts(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// batchRestoreAccounts 批量恢复账号。
func (a *API) batchRestoreAccounts(c *gin.Context) {
	a.batchSetAccountStatus(c, AccountActive)
}

// batchSetAccountStatus 执行批量账号状态迁移。
func (a *API) batchSetAccountStatus(c *gin.Context, target int16) {
	var req BatchAccountStatusRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrAccountStatusInvalid) {
		return
	}
	res, err := a.svc.BatchSetAccountStatus(c.Request.Context(), req.AccountIDs, target)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// ---- 个人中心 ----

// getMe 取个人信息。
func (a *API) getMe(c *gin.Context) {
	id, ok := currentID(c)
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	me, err := a.svc.GetMe(c.Request.Context(), id.AccountID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, me)
}

// changeMyPassword 本人改密。
func (a *API) changeMyPassword(c *gin.Context) {
	id, ok := currentID(c)
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	var req ChangePasswordRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrPasswordChangeInvalid) {
		return
	}
	if err := a.svc.ChangeMyPassword(c.Request.Context(), id.AccountID, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"changed": true})
}

// changeMyPhone 本人换绑手机。
func (a *API) changeMyPhone(c *gin.Context) {
	id, ok := currentID(c)
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	var req ChangePhoneRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrPhoneChangeInvalid) {
		return
	}
	if err := a.svc.ChangeMyPhone(c.Request.Context(), id.TenantID, id.AccountID, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"changed": true})
}

// listMySessions 查询当前账号有效会话。
func (a *API) listMySessions(c *gin.Context) {
	id, ok := currentID(c)
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	rows, err := a.svc.ListMySessions(c.Request.Context(), id.AccountID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// listAudit 审计查询。
func (a *API) listAudit(c *gin.Context) {
	page, size := pagex.Normalize(httpx.QueryInt(c, "page"), httpx.QueryInt(c, "size"))
	filter, err := buildAuditQueryFilter(c.Query("actor_id"), c.Query("action"), c.Query("from"), c.Query("to"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	rows, total, err := a.svc.ListAuditLogs(c.Request.Context(), filter, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OKPage(c, rows, total, page, size)
}

// importTargetFromQuery 把模板类型查询参数转换为导入目标枚举。
func importTargetFromQuery(v string) (int16, bool) {
	switch v {
	case contracts.RoleTeacher:
		return ImportTargetTeacher, true
	case contracts.RoleStudent:
		return ImportTargetStudent, true
	default:
		return 0, false
	}
}
