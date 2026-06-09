// M1 对外契约实现:contracts.IdentityService + audit.Writer + 账号查询/审计查询。
// 依据 docs/总-工程目录设计.md §3.2:模块经 contracts 接口对外;identity 提供审计写入实现。
package identity

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
)

// 编译期断言:Service 实现 contracts.IdentityService 与 audit.Writer。
var (
	_ contracts.IdentityService      = (*Service)(nil)
	_ contracts.IdentityAdminService = (*Service)(nil)
	_ audit.Writer                   = (*Service)(nil)
)

// GetAccount 取账号摘要(跨模块只读)。
func (s *Service) GetAccount(ctx context.Context, accountID int64) (contracts.AccountInfo, error) {
	info, err := s.repo.getAccountInfo(ctx, accountID)
	if err != nil {
		return contracts.AccountInfo{}, toAppErr(err)
	}
	return accountInfoSnapshotToContract(info), nil
}

// BatchGetAccounts 批量取账号摘要(逐个查;数量通常有限)。
func (s *Service) BatchGetAccounts(ctx context.Context, accountIDs []int64) ([]contracts.AccountInfo, error) {
	return collectBatchAccounts(ctx, accountIDs, s.GetAccount)
}

// accountLookupFunc 是批量账号查询使用的单账号读取函数。
type accountLookupFunc func(ctx context.Context, accountID int64) (contracts.AccountInfo, error)

// collectBatchAccounts 逐个读取账号摘要;任一账号失败即返回错误,避免跨模块调用方收到不完整数据。
func collectBatchAccounts(ctx context.Context, accountIDs []int64, lookup accountLookupFunc) ([]contracts.AccountInfo, error) {
	out := make([]contracts.AccountInfo, 0, len(accountIDs))
	for _, id := range accountIDs {
		info, err := lookup(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("batch get account %d: %w", id, err)
		}
		out = append(out, info)
	}
	return out, nil
}

// HasRole 判断账号是否具备某角色编码。
func (s *Service) HasRole(ctx context.Context, accountID int64, role string) (bool, error) {
	roleNum, ok := roleNumOf(role)
	if !ok {
		return false, nil
	}
	has, err := s.repo.hasAccountRole(ctx, accountID, roleNum)
	if err != nil {
		return false, toAppErr(err)
	}
	return has, nil
}

// ListAccounts 学校管理员分页查账号(含角色、班级、关键字过滤)。
func (s *Service) ListAccounts(ctx context.Context, filter AccountListFilter, page, size int) ([]AccountView, int64, error) {
	rows, total, err := s.repo.listAccountViews(ctx, filter, page, size)
	if err != nil {
		return nil, 0, toAppErr(err)
	}
	views := make([]AccountView, 0, len(rows))
	for _, row := range rows {
		view, err := s.accountViewFromSnapshot(row)
		if err != nil {
			return nil, 0, err
		}
		views = append(views, view)
	}
	return views, total, nil
}

// Stats 读取平台级或租户级身份统计,供 M9 看板只读聚合。
func (s *Service) Stats(ctx context.Context, tenantID int64) (contracts.IdentityStats, error) {
	if tenantID > 0 {
		return s.tenantStats(ctx, tenantID)
	}
	return s.platformStats(ctx)
}

// tenantStats 在指定租户 RLS 上下文内统计本校师生账号。
func (s *Service) tenantStats(ctx context.Context, tenantID int64) (contracts.IdentityStats, error) {
	stats, err := s.repo.tenantIdentityStats(ctx, tenantID)
	if err != nil {
		return contracts.IdentityStats{}, apperr.ErrIdentityStatsQueryFailed.WithCause(err)
	}
	return identityStatsSnapshotToContract(stats), nil
}

// platformStats 使用特权连接统计全平台账号、租户和待审申请。
func (s *Service) platformStats(ctx context.Context) (contracts.IdentityStats, error) {
	if !s.repo.hasPrivileged() {
		return contracts.IdentityStats{}, apperr.ErrIdentityPrivilegedRequired.WithCause(fmt.Errorf("平台身份统计需要特权连接"))
	}
	stats, err := s.repo.platformIdentityStats(ctx)
	if err != nil {
		return contracts.IdentityStats{}, apperr.ErrIdentityStatsQueryFailed.WithCause(err)
	}
	return identityStatsSnapshotToContract(stats), nil
}

// AdminListTenants 返回 M9 平台租户列表所需的类型化摘要。
func (s *Service) AdminListTenants(ctx context.Context, status int16, page, size int) ([]contracts.TenantSummary, int64, error) {
	rows, total, err := s.ListTenants(ctx, status, page, size)
	if err != nil {
		return nil, 0, err
	}
	out := make([]contracts.TenantSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenantSummaryFromMap(row))
	}
	return out, total, nil
}

// AdminListApplications 返回 M9 学校入驻申请入口所需的类型化摘要。
func (s *Service) AdminListApplications(ctx context.Context, status int16, page, size int) ([]contracts.ApplicationSummary, int64, error) {
	rows, total, err := s.ListApplications(ctx, status, page, size)
	if err != nil {
		return nil, 0, err
	}
	out := make([]contracts.ApplicationSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, applicationSummaryFromMap(row))
	}
	return out, total, nil
}

// AdminApproveApplication 转发 M9 审核通过入口到 M1 业务实现。
func (s *Service) AdminApproveApplication(ctx context.Context, req contracts.ApplicationApproval) (contracts.ApplicationApprovalResult, error) {
	result, err := s.ApproveApplication(ctx, req.ApplicationID, req.ReviewerID, req.TenantCode, req.AdminPhone, req.AdminName)
	if err != nil {
		return contracts.ApplicationApprovalResult{}, err
	}
	tenantID, _ := strconv.ParseInt(result.TenantID, 10, 64)
	return contracts.ApplicationApprovalResult{
		TenantID: tenantID, TenantCode: result.TenantCode, AdminPhone: result.AdminPhone,
		ActivationCode: result.ActivationCode, ActivationHint: result.ActivationHint,
	}, nil
}

// AdminRejectApplication 转发 M9 审核驳回入口到 M1 业务实现。
func (s *Service) AdminRejectApplication(ctx context.Context, applicationID, reviewerID int64, reason string) error {
	return s.RejectApplication(ctx, applicationID, reviewerID, reason)
}

// ListAuditLogs 审计查询(学校管理员查本租户;平台管理员查平台级),返回数据库总数供分页使用。
func (s *Service) ListAuditLogs(ctx context.Context, filter AuditQueryFilter, page, size int) ([]map[string]any, int64, error) {
	// 第一步根据当前身份确定审计查询范围,平台查询必须显式走特权连接。
	scope, err := auditQueryScopeFromContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	// 平台级审计跨租户读取是受控例外,没有特权连接时必须失败而不是退化为租户查询。
	if scope.platform && !s.repo.hasPrivileged() {
		return nil, 0, apperr.ErrIdentityPrivilegedRequired.WithCause(fmt.Errorf("平台级审计查询需要特权连接"))
	}
	rows, total, err := s.repo.listAuditLogs(ctx, filter, page, size, scope.platform)
	if err != nil {
		return nil, 0, toAppErr(err)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditLogToMap(row))
	}
	return out, total, nil
}

// ListAuditRecords 返回 M9 审计中心使用的类型化审计记录与总数。
func (s *Service) ListAuditRecords(ctx context.Context, query contracts.AuditQuery, page, size int) ([]contracts.AuditRecord, int64, error) {
	scope, err := auditQueryScopeFromContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	from := optionalContractTime(query.From)
	to := optionalContractTime(query.To)
	page, size = pagex.Normalize(page, size)

	if scope.platform {
		if !s.repo.hasPrivileged() {
			return nil, 0, apperr.ErrIdentityPrivilegedRequired.WithCause(fmt.Errorf("平台级审计查询需要特权连接"))
		}
	}
	filter := AuditQueryFilter{
		ActorID: query.ActorID, Action: query.Action, TargetType: query.TargetType,
		FromTime: from, ToTime: to,
	}
	rows, total, err := s.repo.listAuditLogs(ctx, filter, page, size, scope.platform)
	if err != nil {
		return nil, 0, toAppErr(err)
	}
	return auditRecordsFromSnapshots(rows), total, nil
}

// auditQueryScope 描述审计查询使用平台级还是租户级数据路径。
type auditQueryScope struct {
	platform bool
}

// auditQueryScopeFromContext 根据服务端鉴权身份决定审计查询范围。
func auditQueryScopeFromContext(ctx context.Context) (auditQueryScope, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return auditQueryScope{}, apperr.ErrUnauthorized
	}
	return auditQueryScope{platform: id.IsPlatform}, nil
}

// auditLogToMap 转换审计日志投影为对外视图。
func auditLogToMap(r AuditLogSnapshot) map[string]any {
	return map[string]any{
		"id": ids.Format(r.ID), "actor_id": ids.Format(r.ActorID), "actor_role": r.ActorRole,
		"action": r.Action, "target_type": r.TargetType, "created_at": r.CreatedAt,
	}
}

// auditRecordsFromSnapshots 转换审计投影到跨模块 DTO。
func auditRecordsFromSnapshots(rows []AuditLogSnapshot) []contracts.AuditRecord {
	out := make([]contracts.AuditRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditRecordFromSnapshot(row))
	}
	return out
}

// auditRecordFromSnapshot 转换单条审计投影。
func auditRecordFromSnapshot(row AuditLogSnapshot) contracts.AuditRecord {
	return contracts.AuditRecord{
		ID: row.ID, TenantID: row.TenantID, ActorID: row.ActorID, ActorRole: row.ActorRole,
		Action: row.Action, TargetType: row.TargetType, TargetID: row.TargetID,
		Detail: row.Detail, IP: row.IP, TraceID: row.TraceID, CreatedAt: row.CreatedAt,
	}
}

// optionalContractTime 转换 contracts 可空时间过滤条件。
func optionalContractTime(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return timex.RequiredTimestamptz(*t)
}

// tenantSummaryFromMap 将现有平台租户 map 视图转为 contracts DTO。
func tenantSummaryFromMap(row map[string]any) contracts.TenantSummary {
	return contracts.TenantSummary{
		ID: intFromAny(row["id"]), Code: stringFromAny(row["code"]), Name: stringFromAny(row["name"]),
		Type: int16FromAny(row["type"]), Status: int16FromAny(row["status"]), DeployMode: int16FromAny(row["deploy_mode"]),
		LogoURL: stringFromAny(row["logo_url"]), DisplayName: stringFromAny(row["display_name"]),
		AuthMode: int16FromAny(row["auth_mode"]), EnableActivationCode: boolFromAny(row["enable_activation_code"]),
		ExpireAt: timePtrFromAny(row["expire_at"]),
	}
}

// applicationSummaryFromMap 将现有申请 map 视图转为 contracts DTO。
func applicationSummaryFromMap(row map[string]any) contracts.ApplicationSummary {
	return contracts.ApplicationSummary{
		ID: intFromAny(row["id"]), SchoolName: stringFromAny(row["school_name"]), SchoolType: int16FromAny(row["school_type"]),
		ContactName: stringFromAny(row["contact_name"]), ContactPhone: stringFromAny(row["contact_phone"]),
		ContactEmail: stringFromAny(row["contact_email"]), Status: int16FromAny(row["status"]),
		RejectReason: stringFromAny(row["reject_reason"]), CreatedAt: timeFromAny(row["created_at"]),
	}
}

// intFromAny 安全转换 map 视图中的 ID 字段。
func intFromAny(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	default:
		return 0
	}
}

// int16FromAny 安全转换 map 视图中的 smallint 字段。
func int16FromAny(v any) int16 {
	switch x := v.(type) {
	case int16:
		return x
	case int32:
		return int16(x)
	case int:
		return int16(x)
	default:
		return 0
	}
}

// stringFromAny 安全转换 map 视图中的字符串字段。
func stringFromAny(v any) string {
	s, _ := v.(string)
	return s
}

// boolFromAny 安全转换 map 视图中的布尔字段。
func boolFromAny(v any) bool {
	b, _ := v.(bool)
	return b
}

// timeFromAny 安全转换 map 视图中的时间字段。
func timeFromAny(v any) time.Time {
	t, _ := v.(time.Time)
	return timex.UTC(t)
}

// timePtrFromAny 安全转换 map 视图中的可空时间字段。
func timePtrFromAny(v any) *time.Time {
	t, ok := v.(time.Time)
	if !ok || t.IsZero() {
		return nil
	}
	t = timex.UTC(t)
	return &t
}

// accountInfoSnapshotToContract 转换账号摘要投影为跨模块 DTO。
func accountInfoSnapshotToContract(row AccountInfoSnapshot) contracts.AccountInfo {
	return contracts.AccountInfo{
		AccountID:    row.AccountID,
		TenantID:     row.TenantID,
		Name:         row.Name,
		BaseIdentity: row.BaseIdentity,
		Roles:        row.Roles,
		Status:       row.Status,
	}
}

// identityStatsSnapshotToContract 转换身份统计投影为跨模块 DTO。
func identityStatsSnapshotToContract(row IdentityStatsSnapshot) contracts.IdentityStats {
	return contracts.IdentityStats{
		TenantCount:             row.TenantCount,
		AccountCount:            row.AccountCount,
		TeacherCount:            row.TeacherCount,
		StudentCount:            row.StudentCount,
		PendingApplicationCount: row.PendingApplicationCount,
	}
}

// accountViewFromSnapshot 把账号投影中的手机号解密脱敏后组装为对外视图。
func (s *Service) accountViewFromSnapshot(row AccountViewSnapshot) (AccountView, error) {
	phone, err := s.decryptPhone(row.PhoneEnc)
	if err != nil {
		return AccountView{}, apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	return AccountView{
		ID:             ids.Format(row.ID),
		Name:           row.Name,
		Phone:          maskPhone(phone),
		BaseIdentity:   row.BaseIdentity,
		Status:         row.Status,
		Roles:          row.Roles,
		No:             row.No,
		OrgID:          ids.Format(row.OrgID),
		EnrollmentYear: row.EnrollmentYear,
		Title:          row.Title,
		MustChangePwd:  row.MustChangePwd,
	}, nil
}

// roleNumOf 角色编码 → 枚举。
func roleNumOf(code string) (int16, bool) {
	return contracts.RoleNumber(code)
}

// detailJSON 把审计 detail 字符串转 JSONB 字节(空则 {})。
func detailJSON(detail string) []byte {
	if detail == "" {
		return []byte("{}")
	}
	return []byte(detail)
}

// CurrentIdentity 从 ctx 取当前鉴权身份(供 api 层取 accountID/tenantID)。
func CurrentIdentity(ctx context.Context) (tenant.Identity, bool) {
	return tenant.FromContext(ctx)
}
