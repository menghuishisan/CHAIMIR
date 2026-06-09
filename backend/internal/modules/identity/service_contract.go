// identity service_contract 文件实现 internal/contracts 中 identity 对外只读契约。
package identity

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/pagex"
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
		account = row
		return nil
	}); err != nil {
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
		accounts = rows
		return nil
	}); err != nil {
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
		rows, err := tx.ListTenants(ctx)
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

// ListTenantApplications 读取入驻申请供平台聚合入口展示。
func (s *Service) ListTenantApplications(ctx context.Context, query contracts.TenantApplicationQuery) ([]contracts.TenantApplicationSummary, error) {
	var apps []TenantApplication
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		rows, err := tx.ListTenantApplications(ctx, query.Status)
		if err != nil {
			return err
		}
		apps = rows
		return nil
	}); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	out := make([]contracts.TenantApplicationSummary, 0, len(apps))
	for _, app := range apps {
		out = append(out, contracts.TenantApplicationSummary{ApplicationID: app.ID, SchoolName: app.SchoolName, SchoolType: app.SchoolType, ContactName: app.ContactName, ContactPhone: app.ContactPhone, ContactEmail: app.ContactEmail, Status: app.Status, SubmittedAt: app.CreatedAt})
	}
	return out, nil
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
	query.Page = int32(page)
	query.Size = int32(size)
	var rows []AuditLogRow
	var total int64
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		list, n, err := tx.QueryAuditLogs(ctx, AuditQueryInput{TenantID: query.TenantID, ActorID: query.ActorID, Action: query.Action, TargetType: query.TargetType, From: query.From, To: query.To, Page: query.Page, Size: query.Size})
		if err != nil {
			return err
		}
		rows, total = list, n
		return nil
	}); err != nil {
		return contracts.AuditQueryResult{}, apperr.ErrInternal.WithCause(err)
	}
	out := make([]contracts.AuditLogEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, contracts.AuditLogEntry{ID: row.ID, TenantID: row.TenantID, ActorID: row.ActorID, ActorRole: row.ActorRole, Action: row.Action, TargetType: row.TargetType, TargetID: row.TargetID, Detail: row.Detail, IP: row.IP, TraceID: row.TraceID, CreatedAt: row.CreatedAt})
	}
	return contracts.AuditQueryResult{List: out, Total: total, Page: query.Page, Size: query.Size}, nil
}
