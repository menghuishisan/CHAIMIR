// M1 对外契约实现:contracts.IdentityService + audit.Writer + 账号查询/审计查询。
// 依据 docs/总-工程目录设计.md §3.2:模块经 contracts 接口对外;identity 提供审计写入实现。
package identity

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
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
	var info contracts.AccountInfo
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		acc, e := q.GetAccountByID(ctx, accountID)
		if e != nil {
			return apperr.ErrAccountNotFound
		}
		roles, e := q.ListAccountRoles(ctx, accountID)
		if e != nil {
			return e
		}
		info = contracts.AccountInfo{
			AccountID: acc.ID, TenantID: acc.TenantID, Name: acc.Name,
			BaseIdentity: acc.BaseIdentity, Roles: roleCodesOf(roles), Status: acc.Status,
		}
		return nil
	}); err != nil {
		return contracts.AccountInfo{}, toAppErr(err)
	}
	return info, nil
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
	var has bool
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		h, e := q.HasAccountRole(ctx, sqlcgen.HasAccountRoleParams{AccountID: accountID, Role: roleNum})
		if e != nil {
			return e
		}
		has = h
		return nil
	}); err != nil {
		return false, toAppErr(err)
	}
	return has, nil
}

// Write 写审计日志(实现 audit.Writer)。平台级操作(tenant_id=0)走 app 事务;租户级走租户事务。
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

// ListAccounts 学校管理员分页查账号(含角色、班级、关键字过滤)。
func (s *Service) ListAccounts(ctx context.Context, filter AccountListFilter, page, size int) ([]AccountView, int64, error) {
	var views []AccountView
	var total int64
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		params := sqlcgen.ListAccountsParams{
			Status:  pgInt2(filter.Status, filter.Status != 0),
			Role:    pgInt2(filter.Role, filter.Role != 0),
			ClassID: pgInt8(filter.ClassID, filter.ClassID != 0),
			Keyword: pgText(filter.Keyword),
			Limit:   int32(size),
			Offset:  int32((page - 1) * size),
		}
		cnt, e := q.CountAccounts(ctx, sqlcgen.CountAccountsParams{
			Status: params.Status, BaseIdentity: pgInt2(0, false), Role: params.Role, ClassID: params.ClassID, Keyword: params.Keyword,
		})
		if e != nil {
			return e
		}
		total = cnt
		rows, e := q.ListAccounts(ctx, params)
		if e != nil {
			return e
		}
		for _, acc := range rows {
			v, e := s.accountToView(ctx, q, acc)
			if e != nil {
				return e
			}
			views = append(views, v)
		}
		return nil
	}); err != nil {
		return nil, 0, toAppErr(err)
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
	var stats contracts.IdentityStats
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		stats.AccountCount, err = q.CountAccounts(ctx, sqlcgen.CountAccountsParams{
			Status: pgInt2(0, false), BaseIdentity: pgInt2(0, false), Role: pgInt2(0, false), ClassID: pgInt8(0, false), Keyword: pgText(""),
		})
		if err != nil {
			return err
		}
		stats.TeacherCount, err = q.CountAccounts(ctx, sqlcgen.CountAccountsParams{
			Status: pgInt2(0, false), BaseIdentity: pgInt2(BaseIdentityTeacher, true), Role: pgInt2(0, false), ClassID: pgInt8(0, false), Keyword: pgText(""),
		})
		if err != nil {
			return err
		}
		stats.StudentCount, err = q.CountAccounts(ctx, sqlcgen.CountAccountsParams{
			Status: pgInt2(0, false), BaseIdentity: pgInt2(BaseIdentityStudent, true), Role: pgInt2(0, false), ClassID: pgInt8(0, false), Keyword: pgText(""),
		})
		return err
	}); err != nil {
		return contracts.IdentityStats{}, apperr.ErrIdentityStatsQueryFailed.WithCause(err)
	}
	return stats, nil
}

// platformStats 使用特权连接统计全平台账号、租户和待审申请。
func (s *Service) platformStats(ctx context.Context) (contracts.IdentityStats, error) {
	if !s.repo.hasPrivileged() {
		return contracts.IdentityStats{}, apperr.ErrIdentityPrivilegedRequired.WithCause(fmt.Errorf("平台身份统计需要特权连接"))
	}
	var stats contracts.IdentityStats
	if err := s.repo.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		var err error
		stats.TenantCount, err = q.CountTenants(ctx, pgInt2(0, false))
		if err != nil {
			return err
		}
		stats.AccountCount, err = q.CountAllAccounts(ctx, pgInt2(0, false))
		if err != nil {
			return err
		}
		stats.TeacherCount, err = q.CountAllAccounts(ctx, pgInt2(BaseIdentityTeacher, true))
		if err != nil {
			return err
		}
		stats.StudentCount, err = q.CountAllAccounts(ctx, pgInt2(BaseIdentityStudent, true))
		if err != nil {
			return err
		}
		stats.PendingApplicationCount, err = q.CountTenantApplications(ctx, pgInt2(ApplicationPending, true))
		return err
	}); err != nil {
		return contracts.IdentityStats{}, apperr.ErrIdentityStatsQueryFailed.WithCause(err)
	}
	return stats, nil
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
	var out []map[string]any
	var total int64
	scope, err := auditQueryScopeFromContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	params := sqlcgen.ListAuditLogsParams{
		ActorID: pgInt8(filter.ActorID, filter.ActorID != 0), Action: pgText(filter.Action), TargetType: pgText(filter.TargetType),
		FromTime: filter.FromTime, ToTime: filter.ToTime,
		Limit: int32(size), Offset: int32((page - 1) * size),
	}
	appendRows := func(rows []sqlcgen.AuditLog) {
		for _, r := range rows {
			out = append(out, auditLogToMap(r))
		}
	}

	if scope.platform {
		if !s.repo.hasPrivileged() {
			return nil, 0, apperr.ErrIdentityPrivilegedRequired.WithCause(fmt.Errorf("平台级审计查询需要特权连接"))
		}
		err = s.repo.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
			cnt, e := q.CountPlatformAuditLogs(ctx, sqlcgen.CountPlatformAuditLogsParams{
				ActorID: params.ActorID, Action: params.Action, TargetType: params.TargetType,
				FromTime: params.FromTime, ToTime: params.ToTime,
			})
			if e != nil {
				return e
			}
			total = cnt
			rows, e := q.ListPlatformAuditLogs(ctx, sqlcgen.ListPlatformAuditLogsParams(params))
			if e != nil {
				return e
			}
			appendRows(rows)
			return nil
		})
		if err != nil {
			return nil, 0, toAppErr(err)
		}
		return out, total, nil
	}

	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		cnt, e := q.CountAuditLogs(ctx, sqlcgen.CountAuditLogsParams{
			ActorID: params.ActorID, Action: params.Action, TargetType: params.TargetType,
			FromTime: params.FromTime, ToTime: params.ToTime,
		})
		if e != nil {
			return e
		}
		total = cnt
		rows, e := q.ListAuditLogs(ctx, params)
		if e != nil {
			return e
		}
		appendRows(rows)
		return nil
	}); err != nil {
		return nil, 0, toAppErr(err)
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
		return s.listPlatformAuditRecords(ctx, query, from, to, page, size)
	}
	return s.listTenantAuditRecords(ctx, query, from, to, page, size)
}

// listPlatformAuditRecords 使用特权连接读取平台级与全校审计。
func (s *Service) listPlatformAuditRecords(ctx context.Context, query contracts.AuditQuery, from, to pgtype.Timestamptz, page, size int) ([]contracts.AuditRecord, int64, error) {
	if !s.repo.hasPrivileged() {
		return nil, 0, apperr.ErrIdentityPrivilegedRequired.WithCause(fmt.Errorf("平台级审计查询需要特权连接"))
	}
	var rows []sqlcgen.AuditLog
	var total int64
	err := s.repo.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		params := sqlcgen.ListPlatformAuditLogsParams{
			ActorID: pgInt8(query.ActorID, query.ActorID != 0), Action: pgText(query.Action), TargetType: pgText(query.TargetType),
			FromTime: from, ToTime: to, Limit: int32(size), Offset: int32((page - 1) * size),
		}
		var e error
		total, e = q.CountPlatformAuditLogs(ctx, sqlcgen.CountPlatformAuditLogsParams{
			ActorID: params.ActorID, Action: params.Action, TargetType: params.TargetType, FromTime: from, ToTime: to,
		})
		if e != nil {
			return e
		}
		rows, e = q.ListPlatformAuditLogs(ctx, params)
		return e
	})
	if err != nil {
		return nil, 0, toAppErr(err)
	}
	return auditRecordsFromRows(rows), total, nil
}

// listTenantAuditRecords 在当前租户 RLS 范围内读取审计。
func (s *Service) listTenantAuditRecords(ctx context.Context, query contracts.AuditQuery, from, to pgtype.Timestamptz, page, size int) ([]contracts.AuditRecord, int64, error) {
	var rows []sqlcgen.AuditLog
	var total int64
	err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		params := sqlcgen.ListAuditLogsParams{
			ActorID: pgInt8(query.ActorID, query.ActorID != 0), Action: pgText(query.Action), TargetType: pgText(query.TargetType),
			FromTime: from, ToTime: to, Limit: int32(size), Offset: int32((page - 1) * size),
		}
		var e error
		total, e = q.CountAuditLogs(ctx, sqlcgen.CountAuditLogsParams{
			ActorID: params.ActorID, Action: params.Action, TargetType: params.TargetType, FromTime: from, ToTime: to,
		})
		if e != nil {
			return e
		}
		rows, e = q.ListAuditLogs(ctx, params)
		return e
	})
	if err != nil {
		return nil, 0, toAppErr(err)
	}
	return auditRecordsFromRows(rows), total, nil
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

// auditLogToMap 转换审计日志对外视图。
func auditLogToMap(r sqlcgen.AuditLog) map[string]any {
	return map[string]any{
		"id": ids.Format(r.ID), "actor_id": ids.Format(r.ActorID), "actor_role": r.ActorRole,
		"action": r.Action, "target_type": r.TargetType, "created_at": timex.FromTimestamptz(r.CreatedAt),
	}
}

// auditRecordsFromRows 转换 sqlc 审计行到跨模块 DTO。
func auditRecordsFromRows(rows []sqlcgen.AuditLog) []contracts.AuditRecord {
	out := make([]contracts.AuditRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditRecordFromRow(row))
	}
	return out
}

// auditRecordFromRow 转换单条审计日志并解析 detail JSON。
func auditRecordFromRow(row sqlcgen.AuditLog) contracts.AuditRecord {
	return contracts.AuditRecord{
		ID: row.ID, TenantID: int8Value(row.TenantID), ActorID: row.ActorID, ActorRole: row.ActorRole,
		Action: row.Action, TargetType: row.TargetType, TargetID: int8Value(row.TargetID),
		Detail: jsonx.ObjectMap(row.Detail), IP: textVal(row.Ip), TraceID: textVal(row.TraceID), CreatedAt: timex.FromTimestamptz(row.CreatedAt),
	}
}

// optionalContractTime 转换 contracts 可空时间过滤条件。
func optionalContractTime(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return timex.RequiredTimestamptz(*t)
}

// int8Value 读取可空 int8,空值按平台级/未设置语义返回 0。
func int8Value(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
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

// accountToView 把账号行 + 扩展/角色/手机号组装为对外视图。
func (s *Service) accountToView(ctx context.Context, q *sqlcgen.Queries, acc sqlcgen.Account) (AccountView, error) {
	roles, e := q.ListAccountRoles(ctx, acc.ID)
	if e != nil {
		return AccountView{}, e
	}
	phone, e := s.decryptPhone(acc.PhoneEnc)
	if e != nil {
		return AccountView{}, apperr.ErrAccountCredentialFailed.WithCause(e)
	}
	v := AccountView{
		ID: ids.Format(acc.ID), Name: acc.Name, Phone: maskPhone(phone),
		BaseIdentity: acc.BaseIdentity, Status: acc.Status, Roles: roleCodesOf(roles),
		MustChangePwd: acc.MustChangePwd,
	}
	// 扩展信息(可能不存在,如新建管理员未完善)。
	if prof, e := q.GetAccountProfile(ctx, acc.ID); e == nil {
		v.No = prof.No
		v.OrgID = ids.Format(prof.OrgID)
		v.Title = textVal(prof.Title)
		if prof.EnrollmentYear.Valid {
			y := prof.EnrollmentYear.Int16
			v.EnrollmentYear = &y
		}
	}
	return v, nil
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
