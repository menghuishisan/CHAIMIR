// M1 契约查询数据访问:为 contracts 实现提供账号、统计和审计读取投影。
package identity

import (
	"context"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// getAccountInfo 读取跨模块账号摘要。
func (r *repo) getAccountInfo(ctx context.Context, accountID int64) (AccountInfoSnapshot, error) {
	var out AccountInfoSnapshot
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		acc, err := q.GetAccountByID(ctx, accountID)
		if err != nil {
			return apperr.ErrAccountNotFound
		}
		roles, err := q.ListAccountRoles(ctx, accountID)
		if err != nil {
			return err
		}
		out = AccountInfoSnapshot{
			AccountID:    acc.ID,
			TenantID:     acc.TenantID,
			Name:         acc.Name,
			BaseIdentity: acc.BaseIdentity,
			Roles:        roleCodesOf(roles),
			Status:       acc.Status,
		}
		return nil
	}); err != nil {
		return AccountInfoSnapshot{}, err
	}
	return out, nil
}

// hasAccountRole 读取账号是否具备指定角色。
func (r *repo) hasAccountRole(ctx context.Context, accountID int64, role int16) (bool, error) {
	var has bool
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		has, err = q.HasAccountRole(ctx, sqlcgen.HasAccountRoleParams{AccountID: accountID, Role: role})
		return err
	}); err != nil {
		return false, err
	}
	return has, nil
}

// listAccountViews 读取账号列表及其角色、档案扩展信息。
func (r *repo) listAccountViews(ctx context.Context, filter AccountListFilter, page, size int) ([]AccountViewSnapshot, int64, error) {
	var views []AccountViewSnapshot
	var total int64
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		params := sqlcgen.ListAccountsParams{
			Status:  pgtypex.Int2When(filter.Status, filter.Status != 0),
			Role:    pgtypex.Int2When(filter.Role, filter.Role != 0),
			ClassID: pgtypex.Int8When(filter.ClassID, filter.ClassID != 0),
			Keyword: pgtypex.Text(filter.Keyword),
			Limit:   int32(size),
			Offset:  int32((page - 1) * size),
		}
		cnt, err := q.CountAccounts(ctx, sqlcgen.CountAccountsParams{
			Status: params.Status, BaseIdentity: pgtypex.Int2When(0, false), Role: params.Role, ClassID: params.ClassID, Keyword: params.Keyword,
		})
		if err != nil {
			return err
		}
		total = cnt
		rows, err := q.ListAccounts(ctx, params)
		if err != nil {
			return err
		}
		for _, acc := range rows {
			view, err := accountViewSnapshotFromRow(ctx, q, acc)
			if err != nil {
				return err
			}
			views = append(views, view)
		}
		return nil
	}); err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

// tenantIdentityStats 在指定租户 RLS 上下文内统计本校师生账号。
func (r *repo) tenantIdentityStats(ctx context.Context, tenantID int64) (IdentityStatsSnapshot, error) {
	var stats IdentityStatsSnapshot
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		stats.AccountCount, err = q.CountAccounts(ctx, sqlcgen.CountAccountsParams{
			Status: pgtypex.Int2When(0, false), BaseIdentity: pgtypex.Int2When(0, false), Role: pgtypex.Int2When(0, false), ClassID: pgtypex.Int8When(0, false), Keyword: pgtypex.Text(""),
		})
		if err != nil {
			return err
		}
		stats.TeacherCount, err = q.CountAccounts(ctx, sqlcgen.CountAccountsParams{
			Status: pgtypex.Int2When(0, false), BaseIdentity: pgtypex.Int2When(BaseIdentityTeacher, true), Role: pgtypex.Int2When(0, false), ClassID: pgtypex.Int8When(0, false), Keyword: pgtypex.Text(""),
		})
		if err != nil {
			return err
		}
		stats.StudentCount, err = q.CountAccounts(ctx, sqlcgen.CountAccountsParams{
			Status: pgtypex.Int2When(0, false), BaseIdentity: pgtypex.Int2When(BaseIdentityStudent, true), Role: pgtypex.Int2When(0, false), ClassID: pgtypex.Int8When(0, false), Keyword: pgtypex.Text(""),
		})
		return err
	}); err != nil {
		return IdentityStatsSnapshot{}, err
	}
	return stats, nil
}

// platformIdentityStats 使用特权连接统计全平台账号、租户和待审申请。
func (r *repo) platformIdentityStats(ctx context.Context) (IdentityStatsSnapshot, error) {
	var stats IdentityStatsSnapshot
	if err := r.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		var err error
		stats.TenantCount, err = q.CountTenants(ctx, pgtypex.Int2When(0, false))
		if err != nil {
			return err
		}
		stats.AccountCount, err = q.CountAllAccounts(ctx, pgtypex.Int2When(0, false))
		if err != nil {
			return err
		}
		stats.TeacherCount, err = q.CountAllAccounts(ctx, pgtypex.Int2When(BaseIdentityTeacher, true))
		if err != nil {
			return err
		}
		stats.StudentCount, err = q.CountAllAccounts(ctx, pgtypex.Int2When(BaseIdentityStudent, true))
		if err != nil {
			return err
		}
		stats.PendingApplicationCount, err = q.CountTenantApplications(ctx, pgtypex.Int2When(ApplicationPending, true))
		return err
	}); err != nil {
		return IdentityStatsSnapshot{}, err
	}
	return stats, nil
}

// listAuditLogs 读取平台级或租户级审计日志投影。
func (r *repo) listAuditLogs(ctx context.Context, filter AuditQueryFilter, page, size int, platform bool) ([]AuditLogSnapshot, int64, error) {
	if platform {
		return r.listPlatformAuditLogs(ctx, filter, page, size)
	}
	return r.listTenantAuditLogs(ctx, filter, page, size)
}

// listPlatformAuditLogs 使用特权连接读取平台级和跨租户审计。
func (r *repo) listPlatformAuditLogs(ctx context.Context, filter AuditQueryFilter, page, size int) ([]AuditLogSnapshot, int64, error) {
	var rows []sqlcgen.AuditLog
	var total int64
	err := r.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		params := sqlcgen.ListPlatformAuditLogsParams{
			ActorID: pgtypex.Int8When(filter.ActorID, filter.ActorID != 0), Action: pgtypex.Text(filter.Action), TargetType: pgtypex.Text(filter.TargetType),
			FromTime: filter.FromTime, ToTime: filter.ToTime, Limit: int32(size), Offset: int32((page - 1) * size),
		}
		var err error
		total, err = q.CountPlatformAuditLogs(ctx, sqlcgen.CountPlatformAuditLogsParams{
			ActorID: params.ActorID, Action: params.Action, TargetType: params.TargetType,
			FromTime: params.FromTime, ToTime: params.ToTime,
		})
		if err != nil {
			return err
		}
		rows, err = q.ListPlatformAuditLogs(ctx, params)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	return auditLogSnapshotsFromRows(rows), total, nil
}

// listTenantAuditLogs 在当前租户 RLS 范围内读取审计。
func (r *repo) listTenantAuditLogs(ctx context.Context, filter AuditQueryFilter, page, size int) ([]AuditLogSnapshot, int64, error) {
	var rows []sqlcgen.AuditLog
	var total int64
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		params := sqlcgen.ListAuditLogsParams{
			ActorID: pgtypex.Int8When(filter.ActorID, filter.ActorID != 0), Action: pgtypex.Text(filter.Action), TargetType: pgtypex.Text(filter.TargetType),
			FromTime: filter.FromTime, ToTime: filter.ToTime, Limit: int32(size), Offset: int32((page - 1) * size),
		}
		var err error
		total, err = q.CountAuditLogs(ctx, sqlcgen.CountAuditLogsParams{
			ActorID: params.ActorID, Action: params.Action, TargetType: params.TargetType,
			FromTime: params.FromTime, ToTime: params.ToTime,
		})
		if err != nil {
			return err
		}
		rows, err = q.ListAuditLogs(ctx, params)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	return auditLogSnapshotsFromRows(rows), total, nil
}

// accountViewSnapshotFromRow 把账号表行补齐角色和档案扩展字段。
func accountViewSnapshotFromRow(ctx context.Context, q *sqlcgen.Queries, acc sqlcgen.Account) (AccountViewSnapshot, error) {
	roles, err := q.ListAccountRoles(ctx, acc.ID)
	if err != nil {
		return AccountViewSnapshot{}, err
	}
	view := AccountViewSnapshot{
		ID:            acc.ID,
		Name:          acc.Name,
		PhoneEnc:      acc.PhoneEnc,
		BaseIdentity:  acc.BaseIdentity,
		Status:        acc.Status,
		Roles:         roleCodesOf(roles),
		MustChangePwd: acc.MustChangePwd,
	}
	if prof, err := q.GetAccountProfile(ctx, acc.ID); err == nil {
		view.No = prof.No
		view.OrgID = prof.OrgID
		view.Title = textVal(prof.Title)
		if prof.EnrollmentYear.Valid {
			y := prof.EnrollmentYear.Int16
			view.EnrollmentYear = &y
		}
	}
	return view, nil
}

// auditLogSnapshotsFromRows 批量转换审计日志投影。
func auditLogSnapshotsFromRows(rows []sqlcgen.AuditLog) []AuditLogSnapshot {
	out := make([]AuditLogSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditLogSnapshotFromRow(row))
	}
	return out
}

// auditLogSnapshotFromRow 把审计日志行转换为内部投影。
func auditLogSnapshotFromRow(row sqlcgen.AuditLog) AuditLogSnapshot {
	return AuditLogSnapshot{
		ID:         row.ID,
		TenantID:   pgtypex.Int8Value(row.TenantID),
		ActorID:    row.ActorID,
		ActorRole:  row.ActorRole,
		Action:     row.Action,
		TargetType: row.TargetType,
		TargetID:   pgtypex.Int8Value(row.TargetID),
		Detail:     jsonx.ObjectMap(row.Detail),
		IP:         textVal(row.Ip),
		TraceID:    textVal(row.TraceID),
		CreatedAt:  timex.FromTimestamptz(row.CreatedAt),
	}
}
