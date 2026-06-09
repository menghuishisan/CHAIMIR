// M1 审计辅助:集中定义身份模块动作码、对象类型与写入 audit_log 的条目构造。
// 目录职责说明:
//   - platform/audit 只放全平台通用 Entry/Writer/请求元数据,不放身份业务动作码。
//   - identity 拥有 audit_log 表,因此在本模块内把 M1 业务语义映射为 audit.Entry。
package identity

import (
	"context"
	"fmt"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// 身份模块审计动作码。
const (
	AuditActionAuthLogin          = "auth.login"
	AuditActionAuthLogout         = "auth.logout"
	AuditActionAccountCreate      = "account.create"
	AuditActionAccountUpdate      = "account.update"
	AuditActionAccountStatus      = "account.status"
	AuditActionAccountForceLogout = "account.force_logout"
	AuditActionAccountResetPwd    = "account.reset_password"
	AuditActionAccountGrantAdmin  = "account.grant_admin"
	AuditActionAccountRevokeAdmin = "account.revoke_admin"
	AuditActionAccountImport      = "account.import"
	AuditActionOrgChange          = "org.change"
	AuditActionOrgImport          = "org.import"
	AuditActionTenantApprove      = "tenant.approve"
	AuditActionTenantReject       = "tenant.reject"
	AuditActionTenantUpdate       = "tenant.update"
	AuditActionTenantBootstrap    = "tenant.bootstrap"
	AuditActionTenantConfig       = "tenant.config"
	AuditActionTenantSSO          = "tenant.sso"
)

// 身份模块审计对象类型。
const (
	AuditTargetAuthSession = "auth_session"
	AuditTargetAccount     = "account"
	AuditTargetImportBatch = "import_batch"
	AuditTargetOrg         = "organization"
	AuditTargetTenant      = "tenant"
	AuditTargetApplication = "tenant_application"
	AuditTargetSSOConfig   = "sso_config"
)

// buildAuditEntry 按当前鉴权上下文构造 M1 审计条目。
func buildAuditEntry(ctx context.Context, actorRole int16, action, targetType string, targetID int64, detail map[string]any) (audit.Entry, error) {
	// actor 只从服务端鉴权上下文取得,不接受客户端传入,避免审计操作者被伪造。
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return audit.Entry{}, apperr.ErrUnauthorized
	}
	return buildAccountAuditEntry(ctx, id.TenantID, id.AccountID, actorRole, action, targetType, targetID, detail)
}

// buildAccountAuditEntry 构造指定账号的审计条目。
func buildAccountAuditEntry(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) (audit.Entry, error) {
	// M1 自己也复用平台统一条目构造,避免 detail/IP/trace_id 映射在模块内再保留一套。
	return audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
}

// buildPlatformAuditEntry 构造平台管理员审计条目。
func buildPlatformAuditEntry(ctx context.Context, actorID int64, action, targetType string, targetID int64, detail map[string]any) (audit.Entry, error) {
	// Entry 用 TenantID=0 表示平台级审计,最终落库时转换为 tenant_id=NULL。
	return buildAccountAuditEntry(ctx, 0, actorID, RolePlatformAdmin, action, targetType, targetID, detail)
}

// auditEntryIsPlatformScoped 判断审计记录是否属于平台级范围。
func auditEntryIsPlatformScoped(e audit.Entry) bool {
	// 平台级审计最终以 tenant_id=NULL 落库,不能混入某个学校租户的 RLS 范围。
	return e.TenantID == 0
}

// writeAudit 构造并写入审计日志。
func (s *Service) writeAudit(ctx context.Context, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	entry, err := buildAuditEntry(ctx, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return err
	}

	// 审计属于安全能力,写入失败必须向上返回;调用方不能让敏感操作成功但丢失留痕。
	if err := s.Write(ctx, entry); err != nil {
		return apperr.ErrIdentityAuditWriteFailed.WithCause(err)
	}
	return nil
}

// writePlatformAudit 写入平台级审计记录。
func (s *Service) writePlatformAudit(ctx context.Context, actorID int64, action, targetType string, targetID int64, detail map[string]any) error {
	entry, err := buildPlatformAuditEntry(ctx, actorID, action, targetType, targetID, detail)
	if err != nil {
		return err
	}

	// 平台管理员没有学校租户上下文,必须走 audit.Writer 的平台级路径写 tenant_id=NULL。
	if err := s.Write(ctx, entry); err != nil {
		return apperr.ErrIdentityAuditWriteFailed.WithCause(err)
	}
	return nil
}

// Write 写审计日志,实现 platform/audit.Writer 供其他模块统一落 identity.audit_log。
func (s *Service) Write(ctx context.Context, e audit.Entry) error {
	params := buildAuditLogParams(s.idgen.Generate(), e)
	exec := func(q *sqlcgen.Queries) error { return q.CreateAuditLog(ctx, params) }
	if !auditEntryIsPlatformScoped(e) {
		return s.repo.inTenantID(ctx, e.TenantID, exec)
	}
	// 平台级审计 tenant_id 为 NULL,普通 app 连接会受 audit_log RLS 限制;必须使用特权连接。
	if !s.repo.hasPrivileged() {
		return fmt.Errorf("平台级审计写入需要特权连接")
	}
	return s.repo.inPrivileged(ctx, exec)
}

// writeAuditInTx 在当前 M1 业务事务内追加审计记录。
func (s *Service) writeAuditInTx(ctx context.Context, q *sqlcgen.Queries, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	entry, err := buildAuditEntry(ctx, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return err
	}

	// 与业务写入复用同一事务,避免业务提交成功但审计日志缺失。
	return q.CreateAuditLog(ctx, buildAuditLogParams(s.idgen.Generate(), entry))
}

// writeAccountAuditInTx 在当前事务内写入指定账号的审计记录。
func (s *Service) writeAccountAuditInTx(ctx context.Context, q *sqlcgen.Queries, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	// 登录、激活、找回密码等预认证流程没有鉴权上下文,但服务端已通过凭据定位账号。
	entry, err := buildAccountAuditEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return err
	}

	return q.CreateAuditLog(ctx, buildAuditLogParams(s.idgen.Generate(), entry))
}

// buildAuditLogParams 把平台审计 Entry 转为 sqlc 入参。
func buildAuditLogParams(id int64, e audit.Entry) sqlcgen.CreateAuditLogParams {
	// 字段映射集中在一处,确保 M1 自己写审计与跨模块 audit.Writer 写审计完全一致。
	return auditLogParamsFromCreate(buildAuditLogCreate(id, e))
}

// buildAuditLogCreate 把平台审计 Entry 转为 repo 可写入的内部投影。
func buildAuditLogCreate(id int64, e audit.Entry) AuditLogCreate {
	return AuditLogCreate{
		ID:         id,
		TenantID:   e.TenantID,
		ActorID:    e.ActorID,
		ActorRole:  e.ActorRole,
		Action:     e.Action,
		TargetType: e.TargetType,
		TargetID:   e.TargetID,
		Detail:     detailJSON(e.Detail),
		IP:         e.IP,
		TraceID:    e.TraceID,
	}
}

// auditLogParamsFromCreate 把内部审计投影转换为 sqlc 写入参数。
func auditLogParamsFromCreate(row AuditLogCreate) sqlcgen.CreateAuditLogParams {
	return sqlcgen.CreateAuditLogParams{
		ID:         row.ID,
		TenantID:   pgtypex.Int8When(row.TenantID, row.TenantID != 0),
		ActorID:    row.ActorID,
		ActorRole:  row.ActorRole,
		Action:     row.Action,
		TargetType: row.TargetType,
		TargetID:   pgtypex.Int8When(row.TargetID, row.TargetID != 0),
		Detail:     row.Detail,
		Ip:         pgtypex.Text(row.IP),
		TraceID:    pgtypex.Text(row.TraceID),
	}
}
