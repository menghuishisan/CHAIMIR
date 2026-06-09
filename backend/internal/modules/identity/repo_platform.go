// M1 平台数据访问:集中处理入驻申请、租户管理和 SSO 配置持久化。
package identity

import (
	"context"
	"time"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
)

// createTenantApplication 写入访客入驻申请。
func (r *repo) createTenantApplication(ctx context.Context, id int64, req CreateApplicationRequest) error {
	return r.inApp(ctx, func(q *sqlcgen.Queries) error {
		_, err := q.CreateTenantApplication(ctx, sqlcgen.CreateTenantApplicationParams{
			ID: id, SchoolName: req.SchoolName, SchoolType: req.SchoolType,
			ContactName: req.ContactName, ContactPhone: req.ContactPhone, ContactEmail: req.ContactEmail,
		})
		return err
	})
}

// listTenantApplications 分页读取平台入驻申请。
func (r *repo) listTenantApplications(ctx context.Context, status int16, page, size int) ([]TenantApplicationSnapshot, int64, error) {
	var rows []sqlcgen.TenantApplication
	var total int64
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		count, err := q.CountTenantApplications(ctx, pgtypex.Int2When(status, status != 0))
		if err != nil {
			return err
		}
		total = count
		rows, err = q.ListTenantApplications(ctx, sqlcgen.ListTenantApplicationsParams{
			Status: pgtypex.Int2When(status, status != 0), Limit: int32(size), Offset: int32((page - 1) * size),
		})
		return err
	}); err != nil {
		return nil, 0, err
	}
	out := make([]TenantApplicationSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenantApplicationSnapshot(row))
	}
	return out, total, nil
}

// approveApplication 原子完成申请通过、租户创建、首个管理员和激活码写入。
func (r *repo) approveApplication(ctx context.Context, appID, reviewerID, tenantID, adminID int64, tenantCode, adminName string, phoneEnc []byte, phoneHash string, activationCodeHash string, activationExpireAt time.Time, nextID func() int64) error {
	return r.inAppTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		app, err := q.GetTenantApplicationByID(ctx, appID)
		if err != nil {
			return apperr.ErrApplicationNotFound
		}
		if app.Status != ApplicationPending {
			return apperr.ErrApplicationHandled
		}
		// 第一步创建 SaaS 租户,唯一约束冲突要保留为租户短码语义错误。
		if _, err := q.CreateTenant(ctx, sqlcgen.CreateTenantParams{
			ID: tenantID, Code: tenantCode, Name: app.SchoolName, Type: app.SchoolType,
			Status: TenantActive, DeployMode: DeployModeSaaS,
			ExpireAt: pgtype.Timestamptz{}, AuthMode: AuthModeLocal, EnableActivationCode: true,
		}); err != nil {
			if isUniqueViolation(err) {
				return apperr.ErrTenantCodeExists
			}
			return err
		}
		// 第二步回填申请状态,使申请与租户 ID 绑定在同一事务内可追溯。
		if _, err := q.ApproveTenantApplication(ctx, sqlcgen.ApproveTenantApplicationParams{
			ID: appID, ReviewedBy: pgtypex.Int8When(reviewerID, true), TenantID: pgtypex.Int8When(tenantID, true),
		}); err != nil {
			return err
		}
		// 第三步在新租户 RLS 上下文内创建首个学校管理员账号和角色,避免半成品租户。
		if _, err := q.CreateAccount(ctx, sqlcgen.CreateAccountParams{
			ID: adminID, TenantID: tenantID, PhoneEnc: phoneEnc, PhoneHash: phoneHash,
			PasswordHash: pgtype.Text{}, Name: adminName, BaseIdentity: BaseIdentityTeacher,
			Status: AccountPending, MustChangePwd: false,
		}); err != nil {
			return err
		}
		for _, role := range []int16{RoleTeacher, RoleSchoolAdmin} {
			if err := q.AddAccountRole(ctx, sqlcgen.AddAccountRoleParams{
				ID: nextID(), TenantID: tenantID, AccountID: adminID, Role: role,
			}); err != nil {
				return err
			}
		}
		return r.createActivationCodeInTx(ctx, q, nextID(), tenantID, adminID, activationCodeHash, activationExpireAt, reviewerID)
	})
}

// rejectApplication 驳回入驻申请。
func (r *repo) rejectApplication(ctx context.Context, appID, reviewerID int64, reason string) error {
	return r.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, err := q.RejectTenantApplication(ctx, sqlcgen.RejectTenantApplicationParams{
			ID: appID, ReviewedBy: pgtypex.Int8When(reviewerID, true), RejectReason: pgtypex.Text(reason),
		})
		if err != nil {
			return apperr.ErrTenantMutationFailed.WithCause(err)
		}
		if row.ID == 0 {
			return apperr.ErrApplicationHandled
		}
		return nil
	})
}

// listTenants 分页读取租户列表。
func (r *repo) listTenants(ctx context.Context, status int16, page, size int) ([]TenantSnapshot, int64, error) {
	var rows []sqlcgen.Tenant
	var total int64
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		count, err := q.CountTenants(ctx, pgtypex.Int2When(status, status != 0))
		if err != nil {
			return err
		}
		total = count
		rows, err = q.ListTenants(ctx, sqlcgen.ListTenantsParams{
			Status: pgtypex.Int2When(status, status != 0), Limit: int32(size), Offset: int32((page - 1) * size),
		})
		return err
	}); err != nil {
		return nil, 0, err
	}
	out := make([]TenantSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenantSnapshot(row))
	}
	return out, total, nil
}

// getTenant 读取单个租户详情。
func (r *repo) getTenant(ctx context.Context, id int64) (TenantSnapshot, error) {
	var row sqlcgen.Tenant
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		tenant, err := q.GetTenantByID(ctx, id)
		if err != nil {
			return apperr.ErrTenantNotFound
		}
		row = tenant
		return nil
	}); err != nil {
		return TenantSnapshot{}, err
	}
	return tenantSnapshot(row), nil
}

// updateTenantStatus 先确认租户存在再更新状态和到期时间。
func (r *repo) updateTenantStatus(ctx context.Context, id int64, status int16, expireAt *time.Time) error {
	return r.inApp(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetTenantByID(ctx, id); err != nil {
			return apperr.ErrTenantNotFound
		}
		var expire pgtype.Timestamptz
		if expireAt != nil {
			expire = timex.RequiredTimestamptz(*expireAt)
		}
		if _, err := q.UpdateTenantStatus(ctx, sqlcgen.UpdateTenantStatusParams{ID: id, Status: status, ExpireAt: expire}); err != nil {
			return apperr.ErrTenantMutationFailed.WithCause(err)
		}
		return nil
	})
}

// updateTenantConfig 先确认租户存在再更新学校可配置项。
func (r *repo) updateTenantConfig(ctx context.Context, tenantID int64, req TenantConfigRequest, flags []byte) error {
	return r.inApp(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetTenantByID(ctx, tenantID); err != nil {
			return apperr.ErrTenantNotFound
		}
		if _, err := q.UpdateTenantConfig(ctx, sqlcgen.UpdateTenantConfigParams{
			ID: tenantID, LogoUrl: pgtypex.Text(req.LogoURL), DisplayName: pgtypex.Text(req.DisplayName),
			FeatureFlags: flags, AuthMode: req.AuthMode, EnableActivationCode: req.EnableActivationCode,
		}); err != nil {
			return apperr.ErrTenantMutationFailed.WithCause(err)
		}
		return nil
	})
}

// getSsoConfigForView 读取 SSO 配置;没有启用配置时返回 found=false。
func (r *repo) getSsoConfigForView(ctx context.Context, tenantID int64) (SsoConfigSnapshot, bool, error) {
	var row sqlcgen.SsoConfig
	found := true
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		cfg, err := q.GetSsoConfig(ctx, tenantID)
		if err != nil {
			if db.IsNoRows(err) {
				found = false
				return nil
			}
			return err
		}
		row = cfg
		return nil
	}); err != nil {
		return SsoConfigSnapshot{}, false, err
	}
	if !found {
		return SsoConfigSnapshot{}, false, nil
	}
	return ssoConfigSnapshot(row), true, nil
}

// upsertSsoConfigWithAudit 写入租户 SSO 配置并在同一事务内写审计。
func (r *repo) upsertSsoConfigWithAudit(ctx context.Context, tenantID, id int64, req SsoConfigRequest, cfgBytes []byte, auditLog func(rowID int64) (AuditLogCreate, error)) (SsoConfigSnapshot, error) {
	var row sqlcgen.SsoConfig
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		// 配置与审计必须同事务提交,否则敏感认证入口可能变更后缺少追踪记录。
		saved, err := q.UpsertSsoConfig(ctx, sqlcgen.UpsertSsoConfigParams{
			ID: id, TenantID: tenantID, Type: req.Type,
			Config: cfgBytes, MatchField: req.MatchField, Enabled: req.Enabled,
		})
		if err != nil {
			return err
		}
		row = saved
		auditRow, err := auditLog(saved.ID)
		if err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditRow))
	}); err != nil {
		return SsoConfigSnapshot{}, err
	}
	return ssoConfigSnapshot(row), nil
}

// tenantApplicationSnapshot 转换入驻申请行为平台列表投影。
func tenantApplicationSnapshot(row sqlcgen.TenantApplication) TenantApplicationSnapshot {
	return TenantApplicationSnapshot{
		ID: row.ID, SchoolName: row.SchoolName, SchoolType: row.SchoolType,
		ContactName: row.ContactName, ContactPhone: row.ContactPhone, ContactEmail: row.ContactEmail,
		Status: row.Status, RejectReason: textVal(row.RejectReason), CreatedAt: timex.FromTimestamptz(row.CreatedAt),
	}
}

// tenantSnapshot 转换租户行为平台与学校配置投影。
func tenantSnapshot(row sqlcgen.Tenant) TenantSnapshot {
	return TenantSnapshot{
		ID: row.ID, Code: row.Code, Name: row.Name, Type: row.Type, Status: row.Status, DeployMode: row.DeployMode,
		LogoURL: textVal(row.LogoUrl), DisplayName: textVal(row.DisplayName),
		AuthMode: row.AuthMode, EnableActivationCode: row.EnableActivationCode,
		ExpireAt: timex.FromTimestamptz(row.ExpireAt), HasExpireAt: row.ExpireAt.Valid,
	}
}

// ssoConfigSnapshot 转换 SSO 配置行为服务视图投影。
func ssoConfigSnapshot(row sqlcgen.SsoConfig) SsoConfigSnapshot {
	return SsoConfigSnapshot{
		ID: row.ID, Type: row.Type, Config: row.Config, MatchField: row.MatchField, Enabled: row.Enabled,
	}
}
