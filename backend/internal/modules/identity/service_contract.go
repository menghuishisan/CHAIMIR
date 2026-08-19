// identity service_contract 文件实现 internal/contracts 中 identity 对外只读契约。
package identity

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// GetAccount 读取账号摘要,供审计、鉴权和聚合只读调用。
func (s *Service) GetAccount(ctx context.Context, accountID int64) (contracts.AccountInfo, error) {
	var account Account
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetAccount(ctx, accountID)
		if err != nil {
			return err
		}
		if err := ensureAccountVisibleInContext(ctx, row.TenantID); err != nil {
			return err
		}
		account = row
		return nil
	}); err != nil {
		if appErr, ok := apperr.As(err); ok {
			return contracts.AccountInfo{}, appErr
		}
		return contracts.AccountInfo{}, apperr.ErrNotFound.WithCause(err)
	}
	phone, err := s.decryptPhone(account.PhoneEnc)
	if err != nil {
		return contracts.AccountInfo{}, apperr.ErrInternal.WithCause(err)
	}
	return ToContractAccount(account, phone), nil
}

// BatchGetAccounts 批量读取账号摘要,避免高层模块形成 N+1 查询。
func (s *Service) BatchGetAccounts(ctx context.Context, accountIDs []int64) ([]contracts.AccountInfo, error) {
	var accounts []Account
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		rows, err := tx.BatchGetAccounts(ctx, accountIDs)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err := ensureAccountVisibleInContext(ctx, row.TenantID); err != nil {
				return err
			}
		}
		accounts = rows
		return nil
	}); err != nil {
		if appErr, ok := apperr.As(err); ok {
			return nil, appErr
		}
		return nil, apperr.ErrInternal.WithCause(err)
	}
	out := make([]contracts.AccountInfo, 0, len(accounts))
	for _, account := range accounts {
		phone, err := s.decryptPhone(account.PhoneEnc)
		if err != nil {
			return nil, apperr.ErrInternal.WithCause(err)
		}
		out = append(out, ToContractAccount(account, phone))
	}
	return out, nil
}

// ListClassStudents 读取指定班级的在校学生摘要,实现 M6 按班级批量选课所需的契约。
// 走租户事务(RLS 生效)并显式传 tenant_id:调用方是同租户业务模块,不需要跨租户视角。
func (s *Service) ListClassStudents(ctx context.Context, tenantID, classID int64, page, size int) ([]contracts.AccountInfo, error) {
	if tenantID <= 0 || classID <= 0 {
		return nil, apperr.ErrIdentityOrgInvalidInput
	}
	var students []Account
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		// 先确认班级确属本租户,再取学生 —— 否则拿到空列表分不清「班级不存在」和「班里没人」。
		// 班级归档本身不需要在这里判:归档班级是整届毕业动作,同事务已把该学年学生账号一并
		// 归档(ArchiveClassesByAdmin),而查询只回 status=在用 的学生,已覆盖这一情形。
		exists, err := tx.ClassExists(ctx, tenantID, classID)
		if err != nil {
			return err
		}
		if !exists {
			return apperr.ErrIdentityOrgInvalidInput
		}
		students, _, err = tx.ListClassStudents(ctx, tenantID, classID, page, size)
		return err
	}); err != nil {
		if appErr, ok := apperr.As(err); ok {
			return nil, appErr
		}
		return nil, apperr.ErrInternal.WithCause(err)
	}
	out := make([]contracts.AccountInfo, 0, len(students))
	for _, student := range students {
		// 本契约不下发手机号:选课只需要姓名与学号,查询也没有取回手机号密文。
		out = append(out, ToContractAccount(student, ""))
	}
	return out, nil
}

// HasRole 判断账号是否具备指定角色。
func (s *Service) HasRole(ctx context.Context, accountID int64, role string) (bool, error) {
	want, ok := contracts.RoleNumber(role)
	if !ok {
		return false, nil
	}
	info, err := s.GetAccount(ctx, accountID)
	if err != nil {
		return false, err
	}
	for _, got := range info.Roles {
		n, ok := contracts.RoleNumber(got)
		if ok && n == want {
			return true, nil
		}
	}
	return false, nil
}

// ListTenants 读取租户列表供聚合层只读使用。
func (s *Service) ListTenants(ctx context.Context) ([]contracts.TenantSummary, error) {
	var tenants []Tenant
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		rows, err := tx.ListAllTenants(ctx)
		if err != nil {
			return err
		}
		tenants = rows
		return nil
	}); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	out := make([]contracts.TenantSummary, 0, len(tenants))
	for _, t := range tenants {
		out = append(out, contracts.TenantSummary{TenantID: t.ID, Code: t.Code, Name: t.Name, Type: t.Type, Status: t.Status, DeployMode: t.DeployMode, ExpireAt: t.ExpireAt, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt})
	}
	return out, nil
}

// GetTenant 读取单个租户摘要供聚合层只读使用。
func (s *Service) GetTenant(ctx context.Context, tenantID int64) (contracts.TenantSummary, error) {
	var t Tenant
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetTenantByID(ctx, tenantID)
		if err != nil {
			return err
		}
		t = row
		return nil
	}); err != nil {
		return contracts.TenantSummary{}, apperr.ErrNotFound.WithCause(err)
	}
	return contracts.TenantSummary{TenantID: t.ID, Code: t.Code, Name: t.Name, Type: t.Type, Status: t.Status, DeployMode: t.DeployMode, ExpireAt: t.ExpireAt, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}, nil
}

// PlatformStats 返回平台身份统计。
func (s *Service) PlatformStats(ctx context.Context) (contracts.IdentityStats, error) {
	var stats StatsRow
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		row, err := tx.PlatformStats(ctx)
		if err != nil {
			return err
		}
		stats = row
		return nil
	}); err != nil {
		return contracts.IdentityStats{}, apperr.ErrInternal.WithCause(err)
	}
	return contracts.IdentityStats{TenantCount: stats.TenantCount, AccountCount: stats.AccountCount, TeacherCount: stats.TeacherCount, StudentCount: stats.StudentCount, SchoolAdminCount: stats.SchoolAdminCount, PlatformAdminCount: stats.PlatformAdminCount, ActiveAccountCount: stats.ActiveAccountCount, ActiveTenantCount: stats.ActiveTenantCount, PendingApplyCount: stats.PendingApplyCount, DisabledAccountCount: stats.DisabledAccountCount}, nil
}

// TenantStats 返回单租户身份统计。
func (s *Service) TenantStats(ctx context.Context, tenantID int64) (contracts.IdentityStats, error) {
	var stats StatsRow
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		row, err := tx.TenantStats(ctx, tenantID)
		if err != nil {
			return err
		}
		stats = row
		return nil
	}); err != nil {
		return contracts.IdentityStats{}, apperr.ErrInternal.WithCause(err)
	}
	return contracts.IdentityStats{TenantID: tenantID, AccountCount: stats.AccountCount, TeacherCount: stats.TeacherCount, StudentCount: stats.StudentCount, ActiveAccountCount: stats.ActiveAccountCount, DisabledAccountCount: stats.DisabledAccountCount}, nil
}

// QueryAuditLogs 按权限范围查询共享审计日志。
func (s *Service) QueryAuditLogs(ctx context.Context, query contracts.AuditQuery) (contracts.AuditQueryResult, error) {
	page, size := pagex.Normalize(int(query.Page), int(query.Size))
	query.Page, query.Size = pagex.Int32(page, size)
	var rows []AuditLogRow
	var total int64
	read := func(ctx context.Context, tx TxStore) error {
		list, n, err := tx.QueryAuditLogs(ctx, AuditQueryInput{TenantID: auditTenantFilter(query), ActorID: query.ActorID, Action: query.Action, TargetType: query.TargetType, From: query.From, To: query.To, Page: query.Page, Size: query.Size})
		if err != nil {
			return err
		}
		rows, total = list, n
		return nil
	}
	var err error
	if query.IncludePlatform {
		err = s.store.PrivilegedTx(ctx, read)
	} else {
		err = s.store.TenantTx(ctx, query.TenantID, read)
	}
	if err != nil {
		return contracts.AuditQueryResult{}, apperr.ErrInternal.WithCause(err)
	}
	out := make([]contracts.AuditLogEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, contracts.AuditLogEntry{ID: row.ID, TenantID: row.TenantID, ActorID: row.ActorID, ActorRole: row.ActorRole, Action: row.Action, TargetType: row.TargetType, TargetID: row.TargetID, Detail: row.Detail, IP: row.IP, TraceID: row.TraceID, CreatedAt: row.CreatedAt})
	}
	return contracts.AuditQueryResult{List: out, Total: total, Page: query.Page, Size: query.Size}, nil
}

// ensureAccountVisibleInContext 用调用方上下文收敛账号摘要读取范围,避免跨租户 ID 枚举。
func ensureAccountVisibleInContext(ctx context.Context, accountTenantID int64) error {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.IsPlatform {
		return nil
	}
	if id.TenantID <= 0 || id.TenantID != accountTenantID {
		return apperr.ErrCrossTenant
	}
	return nil
}

// auditTenantFilter 把平台级审计范围转换为 repo 层全量查询,租户路径固定收敛到本租户。
func auditTenantFilter(query contracts.AuditQuery) int64 {
	if query.IncludePlatform {
		return -1
	}
	return query.TenantID
}
