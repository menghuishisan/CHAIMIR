// identity repo_identity 文件实现租户、账号、会话、短信、导入和审计的数据访问方法。
package identity

import (
	"context"
	"time"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
)

// GetPlatformAdminByUsername 按登录名读取平台管理员账号。
func (t *txStore) GetPlatformAdminByUsername(ctx context.Context, username string) (PlatformAdmin, error) {
	row, err := t.q.GetPlatformAdminByUsername(ctx, username)
	if err != nil {
		return PlatformAdmin{}, err
	}
	return platformAdminFromRow(row), nil
}

// CreateTenantProvisionOutbox 在租户创建事务内保存初始化事件。
func (t *txStore) CreateTenantProvisionOutbox(ctx context.Context, item TenantProvisionOutbox) (TenantProvisionOutbox, error) {
	row, err := t.q.CreateTenantProvisionOutbox(ctx, sqlcgen.CreateTenantProvisionOutboxParams{
		ID: item.ID, TenantID: item.TenantID, DeployMode: item.DeployMode, TraceID: item.TraceID,
		ProvisionedAt: timex.RequiredTimestamptz(item.ProvisionedAt),
	})
	if err != nil {
		return TenantProvisionOutbox{}, err
	}
	return tenantProvisionOutboxFromRow(row), nil
}

// ClaimTenantProvisionOutbox 跨租户领取待发布或超时的初始化事件。
func (t *txStore) ClaimTenantProvisionOutbox(ctx context.Context, limit int32, staleBefore time.Time) ([]TenantProvisionOutbox, error) {
	rows, err := t.q.ClaimTenantProvisionOutbox(ctx, sqlcgen.ClaimTenantProvisionOutboxParams{StaleBefore: timex.RequiredTimestamptz(staleBefore), PageLimit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]TenantProvisionOutbox, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenantProvisionOutboxFromRow(row))
	}
	return out, nil
}

// MarkTenantProvisionOutboxPublished 标记初始化事件已经发布。
func (t *txStore) MarkTenantProvisionOutboxPublished(ctx context.Context, id int64) (TenantProvisionOutbox, error) {
	row, err := t.q.MarkTenantProvisionOutboxPublished(ctx, id)
	if err != nil {
		return TenantProvisionOutbox{}, err
	}
	return tenantProvisionOutboxFromRow(row), nil
}

// MarkTenantProvisionOutboxFailed 记录初始化事件发布失败原因供后续重试。
func (t *txStore) MarkTenantProvisionOutboxFailed(ctx context.Context, id int64, lastError string) (TenantProvisionOutbox, error) {
	row, err := t.q.MarkTenantProvisionOutboxFailed(ctx, sqlcgen.MarkTenantProvisionOutboxFailedParams{ID: id, LastError: pgtypex.Text(lastError)})
	if err != nil {
		return TenantProvisionOutbox{}, err
	}
	return tenantProvisionOutboxFromRow(row), nil
}

// GetPlatformAdminByID 按 ID 读取平台管理员账号。
func (t *txStore) GetPlatformAdminByID(ctx context.Context, id int64) (PlatformAdmin, error) {
	row, err := t.q.GetPlatformAdminByID(ctx, id)
	if err != nil {
		return PlatformAdmin{}, err
	}
	return platformAdminFromRow(row), nil
}

// CreatePlatformAdminIfNotExists 幂等创建 SaaS 首个平台管理员,已存在时不覆盖密码。
func (t *txStore) CreatePlatformAdminIfNotExists(ctx context.Context, input CreatePlatformAdminInput) error {
	return t.q.CreatePlatformAdminIfNotExists(ctx, sqlcgen.CreatePlatformAdminIfNotExistsParams{
		ID:           input.ID,
		Username:     input.Username,
		PasswordHash: input.PasswordHash,
		Name:         input.Name,
		Status:       input.Status,
	})
}

// UpdatePlatformAdminPassword 更新平台管理员密码哈希。
func (t *txStore) UpdatePlatformAdminPassword(ctx context.Context, adminID int64, passwordHash string) error {
	return t.q.UpdatePlatformAdminPassword(ctx, sqlcgen.UpdatePlatformAdminPasswordParams{ID: adminID, PasswordHash: passwordHash})
}

// GetTenantByCode 按学校短码读取租户。
func (t *txStore) GetTenantByCode(ctx context.Context, code string) (Tenant, error) {
	row, err := t.q.GetTenantByCode(ctx, code)
	if err != nil {
		return Tenant{}, err
	}
	return tenantFromRow(row), nil
}

// GetTenantByID 按 ID 读取租户。
func (t *txStore) GetTenantByID(ctx context.Context, id int64) (Tenant, error) {
	row, err := t.q.GetTenantByID(ctx, id)
	if err != nil {
		return Tenant{}, err
	}
	return tenantFromRow(row), nil
}

// ListAllTenants 读取所有租户摘要,仅供聚合层只读汇总使用。
func (t *txStore) ListAllTenants(ctx context.Context) ([]Tenant, error) {
	rows, err := t.q.ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Tenant, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenantFromRow(row))
	}
	return out, nil
}

// ListTenants 分页读取平台租户摘要。
func (t *txStore) ListTenants(ctx context.Context, page, size int) ([]Tenant, int64, error) {
	rows, err := t.q.ListTenantsPaged(ctx, sqlcgen.ListTenantsPagedParams{Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Tenant, 0, len(rows))
	var total int64
	for _, row := range rows {
		total = row.TotalCount
		out = append(out, tenantFromPagedRow(row))
	}
	return out, total, nil
}

// CreateTenant 创建学校租户及内联配置。
func (t *txStore) CreateTenant(ctx context.Context, input CreateTenantInput) (Tenant, error) {
	row, err := t.q.CreateTenant(ctx, sqlcgen.CreateTenantParams{
		ID:                   input.ID,
		Code:                 input.Code,
		Name:                 input.Name,
		Type:                 input.Type,
		Status:               input.Status,
		DeployMode:           input.DeployMode,
		ExpireAt:             pgtypex.TimestamptzPtr(input.ExpireAt),
		LogoUrl:              pgtypex.Text(input.LogoURL),
		DisplayName:          pgtypex.Text(input.DisplayName),
		AuthMode:             input.AuthMode,
		EnableActivationCode: input.EnableActivationCode,
	})
	if err != nil {
		return Tenant{}, err
	}
	return tenantFromRow(row), nil
}

// UpdateTenantConfig 更新租户配置字段。
func (t *txStore) UpdateTenantConfig(ctx context.Context, input UpdateTenantConfigInput) (Tenant, error) {
	row, err := t.q.UpdateTenantConfig(ctx, sqlcgen.UpdateTenantConfigParams{
		ID:                   input.TenantID,
		LogoUrl:              pgtypex.Text(input.LogoURL),
		DisplayName:          pgtypex.Text(input.DisplayName),
		AuthMode:             input.AuthMode,
		EnableActivationCode: input.EnableActivationCode,
	})
	if err != nil {
		return Tenant{}, err
	}
	return tenantFromRow(row), nil
}

// UpdateTenantStatus 更新租户启停和到期时间。
func (t *txStore) UpdateTenantStatus(ctx context.Context, input UpdateTenantStatusInput) (Tenant, error) {
	row, err := t.q.UpdateTenantStatus(ctx, sqlcgen.UpdateTenantStatusParams{
		ID:       input.TenantID,
		Status:   input.Status,
		ExpireAt: pgtypex.TimestamptzPtr(input.ExpireAt),
	})
	if err != nil {
		return Tenant{}, err
	}
	return tenantFromRow(row), nil
}

// CreateTenantApplication 创建学校入驻申请。
func (t *txStore) CreateTenantApplication(ctx context.Context, input CreateApplicationRequest, id int64) (TenantApplication, error) {
	row, err := t.q.CreateTenantApplication(ctx, sqlcgen.CreateTenantApplicationParams{
		ID:           id,
		SchoolName:   input.SchoolName,
		SchoolType:   input.SchoolType,
		ContactName:  input.ContactName,
		ContactPhone: input.ContactPhone,
		ContactEmail: input.ContactEmail,
		Status:       ApplicationStatusPending,
	})
	if err != nil {
		return TenantApplication{}, err
	}
	return applicationFromRow(row), nil
}

// GetTenantApplication 读取单个入驻申请。
func (t *txStore) GetTenantApplication(ctx context.Context, id int64) (TenantApplication, error) {
	row, err := t.q.GetTenantApplication(ctx, id)
	if err != nil {
		return TenantApplication{}, err
	}
	return applicationFromRow(row), nil
}

// ListTenantApplications 按状态读取入驻申请。
func (t *txStore) ListTenantApplications(ctx context.Context, status int16) ([]TenantApplication, error) {
	rows, err := t.q.ListTenantApplications(ctx, status)
	if err != nil {
		return nil, err
	}
	out := make([]TenantApplication, 0, len(rows))
	for _, row := range rows {
		out = append(out, applicationFromRow(row))
	}
	return out, nil
}

// ApproveTenantApplication 把待审核申请推进为已通过并绑定租户。
func (t *txStore) ApproveTenantApplication(ctx context.Context, id, reviewerID, tenantID int64) (TenantApplication, error) {
	row, err := t.q.ApproveTenantApplication(ctx, sqlcgen.ApproveTenantApplicationParams{
		ID:         id,
		Status:     ApplicationStatusApproved,
		ReviewedBy: pgtypex.Int8(reviewerID),
		TenantID:   pgtypex.Int8(tenantID),
	})
	if err != nil {
		return TenantApplication{}, err
	}
	return applicationFromRow(row), nil
}

// RejectTenantApplication 把待审核申请推进为已驳回。
func (t *txStore) RejectTenantApplication(ctx context.Context, id, reviewerID int64, reason string) (TenantApplication, error) {
	row, err := t.q.RejectTenantApplication(ctx, sqlcgen.RejectTenantApplicationParams{
		ID:           id,
		Status:       ApplicationStatusRejected,
		RejectReason: pgtypex.Text(reason),
		ReviewedBy:   pgtypex.Int8(reviewerID),
	})
	if err != nil {
		return TenantApplication{}, err
	}
	return applicationFromRow(row), nil
}

// CreateAccount 创建账号、基础角色、可选学校管理员角色和可选档案。
func (t *txStore) CreateAccount(ctx context.Context, input CreateAccountInput) (Account, error) {
	// 账号主表先落库,后续角色和档案都以同一个 account_id 形成事务内聚合。
	row, err := t.q.CreateAccount(ctx, sqlcgen.CreateAccountParams{
		ID:            input.ID,
		TenantID:      input.TenantID,
		PhoneEnc:      input.PhoneEnc,
		PhoneHash:     input.PhoneHash,
		PasswordHash:  pgtypex.Text(input.PasswordHash),
		Name:          input.Name,
		BaseIdentity:  input.BaseIdentity,
		Status:        input.Status,
		MustChangePwd: input.MustChangePwd,
		ActivatedAt:   pgtypex.TimestamptzPtr(input.ActivatedAt),
	})
	if err != nil {
		return Account{}, err
	}
	// 角色必须在创建账号的同一事务写入,避免出现没有基础身份的半成品账号。
	for _, role := range input.Roles {
		if err := t.q.CreateAccountRole(ctx, sqlcgen.CreateAccountRoleParams{
			ID:        role.ID,
			TenantID:  input.TenantID,
			AccountID: input.ID,
			Role:      role.Role,
		}); err != nil {
			return Account{}, err
		}
	}
	if input.Profile != nil {
		// 首个学校管理员允许暂无档案;普通师生档案由 service 先校验组织归属后再写入。
		if err := t.q.CreateAccountProfile(ctx, sqlcgen.CreateAccountProfileParams{
			AccountID:      input.ID,
			TenantID:       input.TenantID,
			No:             input.Profile.No,
			OrgID:          input.Profile.OrgID,
			EnrollmentYear: pgtypex.Int2(input.Profile.EnrollmentYear),
			Title:          pgtypex.Text(input.Profile.Title),
		}); err != nil {
			return Account{}, err
		}
	}
	// 返回 service 需要的领域快照,不把 sqlc 行类型泄漏到业务层。
	account := Account{
		ID:            row.ID,
		TenantID:      row.TenantID,
		PhoneEnc:      row.PhoneEnc,
		PhoneHash:     row.PhoneHash,
		PasswordHash:  pgtypex.TextValue(row.PasswordHash),
		Name:          row.Name,
		BaseIdentity:  row.BaseIdentity,
		Status:        row.Status,
		MustChangePwd: row.MustChangePwd,
		Roles:         make([]int16, 0, len(input.Roles)),
		CreatedAt:     timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:     timex.FromTimestamptz(row.UpdatedAt),
	}
	if input.Profile != nil {
		account.No = input.Profile.No
		account.OrgID = input.Profile.OrgID
		account.EnrollmentYear = input.Profile.EnrollmentYear
		account.Title = input.Profile.Title
	}
	for _, role := range input.Roles {
		account.Roles = append(account.Roles, role.Role)
	}
	return account, nil
}

// GetAccount 读取单个账号聚合快照。
func (t *txStore) GetAccount(ctx context.Context, id int64) (Account, error) {
	row, err := t.q.GetAccountByID(ctx, id)
	if err != nil {
		return Account{}, err
	}
	return accountFromRow(row), nil
}

// BatchGetAccounts 批量读取账号聚合快照。
func (t *txStore) BatchGetAccounts(ctx context.Context, ids []int64) ([]Account, error) {
	rows, err := t.q.BatchGetAccounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(rows))
	for _, row := range rows {
		out = append(out, accountFromBatchRow(row))
	}
	return out, nil
}

// ListAccountsByPhoneHash 特权读取手机号跨租户候选账号,仅用于预认证定位。
func (t *txStore) ListAccountsByPhoneHash(ctx context.Context, phoneHash string) ([]LoginCandidate, error) {
	rows, err := t.q.ListAccountsByPhoneHashPrivileged(ctx, phoneHash)
	if err != nil {
		return nil, err
	}
	out := make([]LoginCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, LoginCandidate{
			AccountID:      row.ID,
			TenantID:       row.TenantID,
			TenantCode:     row.TenantCode,
			TenantName:     row.TenantName,
			PasswordHash:   pgtypex.TextValue(row.PasswordHash),
			Name:           row.Name,
			BaseIdentity:   row.BaseIdentity,
			Status:         row.Status,
			MustChangePwd:  row.MustChangePwd,
			PwdFailedCount: row.PwdFailedCount,
			LockedUntil:    timex.PtrFromTimestamptz(row.LockedUntil),
		})
	}
	return out, nil
}

// GetAccountByPhoneHash 在已确定租户边界内按手机号哈希读取账号。
func (t *txStore) GetAccountByPhoneHash(ctx context.Context, tenantID int64, phoneHash string) (Account, error) {
	row, err := t.q.GetAccountByPhoneHash(ctx, sqlcgen.GetAccountByPhoneHashParams{TenantID: tenantID, PhoneHash: phoneHash})
	if err != nil {
		return Account{}, err
	}
	return Account{
		ID:             row.ID,
		TenantID:       row.TenantID,
		PhoneEnc:       row.PhoneEnc,
		PhoneHash:      row.PhoneHash,
		PasswordHash:   pgtypex.TextValue(row.PasswordHash),
		Name:           row.Name,
		BaseIdentity:   row.BaseIdentity,
		Status:         row.Status,
		MustChangePwd:  row.MustChangePwd,
		PwdFailedCount: row.PwdFailedCount,
		LockedUntil:    timex.PtrFromTimestamptz(row.LockedUntil),
		ActivatedAt:    timex.PtrFromTimestamptz(row.ActivatedAt),
		No:             pgtypex.TextValue(row.No),
		OrgID:          pgtypex.Int8Value(row.OrgID),
		EnrollmentYear: pgtypex.Int2Value(row.EnrollmentYear),
		Title:          pgtypex.TextValue(row.Title),
		Roles:          row.Roles,
		CreatedAt:      timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:      timex.FromTimestamptz(row.UpdatedAt),
	}, nil
}

// GetAccountByNo 按学号或工号读取账号。
func (t *txStore) GetAccountByNo(ctx context.Context, no string) (Account, error) {
	row, err := t.q.GetAccountByNo(ctx, no)
	if err != nil {
		return Account{}, err
	}
	return Account{
		ID:             row.ID,
		TenantID:       row.TenantID,
		PhoneEnc:       row.PhoneEnc,
		PhoneHash:      row.PhoneHash,
		PasswordHash:   pgtypex.TextValue(row.PasswordHash),
		Name:           row.Name,
		BaseIdentity:   row.BaseIdentity,
		Status:         row.Status,
		MustChangePwd:  row.MustChangePwd,
		PwdFailedCount: row.PwdFailedCount,
		LockedUntil:    timex.PtrFromTimestamptz(row.LockedUntil),
		ActivatedAt:    timex.PtrFromTimestamptz(row.ActivatedAt),
		No:             row.No,
		OrgID:          row.OrgID,
		EnrollmentYear: pgtypex.Int2Value(row.EnrollmentYear),
		Title:          pgtypex.TextValue(row.Title),
		Roles:          row.Roles,
		CreatedAt:      timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:      timex.FromTimestamptz(row.UpdatedAt),
	}, nil
}

// ListAccounts 分页读取学校管理员账号列表。
func (t *txStore) ListAccounts(ctx context.Context, query AccountQuery) ([]Account, int64, error) {
	rows, err := t.q.ListAccounts(ctx, sqlcgen.ListAccountsParams{
		Column1: query.Status,
		Column2: query.BaseIdentity,
		Column3: query.ClassID,
		Column4: query.Keyword,
		Limit:   query.Size,
		Offset:  (query.Page - 1) * query.Size,
	})
	if err != nil {
		return nil, 0, err
	}
	// sqlc 查询通过窗口计数返回 total,repo 在这里拆成列表和总数两个领域值。
	out := make([]Account, 0, len(rows))
	var total int64
	for _, row := range rows {
		total = row.TotalCount
		out = append(out, Account{
			ID:             row.ID,
			TenantID:       row.TenantID,
			PhoneEnc:       row.PhoneEnc,
			PhoneHash:      row.PhoneHash,
			PasswordHash:   pgtypex.TextValue(row.PasswordHash),
			Name:           row.Name,
			BaseIdentity:   row.BaseIdentity,
			Status:         row.Status,
			MustChangePwd:  row.MustChangePwd,
			PwdFailedCount: row.PwdFailedCount,
			LockedUntil:    timex.PtrFromTimestamptz(row.LockedUntil),
			ActivatedAt:    timex.PtrFromTimestamptz(row.ActivatedAt),
			No:             pgtypex.TextValue(row.No),
			OrgID:          pgtypex.Int8Value(row.OrgID),
			EnrollmentYear: pgtypex.Int2Value(row.EnrollmentYear),
			Title:          pgtypex.TextValue(row.Title),
			Roles:          row.Roles,
			CreatedAt:      timex.FromTimestamptz(row.CreatedAt),
			UpdatedAt:      timex.FromTimestamptz(row.UpdatedAt),
		})
	}
	return out, total, nil
}

// UpdateAccountEditable 更新账号可编辑字段并返回最新账号快照。
func (t *txStore) UpdateAccountEditable(ctx context.Context, tenantID, accountID int64, req UpdateAccountRequest) (Account, error) {
	if err := t.q.UpdateAccountBasic(ctx, sqlcgen.UpdateAccountBasicParams{ID: accountID, TenantID: tenantID, Name: req.Name}); err != nil {
		return Account{}, err
	}
	if err := t.q.UpdateAccountProfileEditable(ctx, sqlcgen.UpdateAccountProfileEditableParams{AccountID: accountID, TenantID: tenantID, OrgID: req.OrgID.Int64(), EnrollmentYear: pgtypex.Int2(req.EnrollmentYear), Title: pgtypex.Text(req.Title)}); err != nil {
		return Account{}, err
	}
	return t.GetAccount(ctx, accountID)
}

// UpdateAccountStatus 更新账号状态,注销时写软删除时间。
func (t *txStore) UpdateAccountStatus(ctx context.Context, accountID, tenantID int64, status int16, deleted bool) (Account, error) {
	var deletedAt pgtype.Timestamptz
	if deleted {
		deletedAt = timex.RequiredTimestamptz(timex.Now())
	}
	row, err := t.q.UpdateAccountStatus(ctx, sqlcgen.UpdateAccountStatusParams{
		ID:        accountID,
		TenantID:  tenantID,
		Status:    status,
		DeletedAt: deletedAt,
	})
	if err != nil {
		return Account{}, err
	}
	return Account{ID: row.ID, TenantID: row.TenantID, Name: row.Name, BaseIdentity: row.BaseIdentity, Status: row.Status}, nil
}

// UpdateAccountPassword 更新密码哈希和首登状态。
func (t *txStore) UpdateAccountPassword(ctx context.Context, accountID, tenantID int64, passwordHash string, mustChange bool, status int16) (Account, error) {
	now := timex.Now()
	row, err := t.q.UpdateAccountPassword(ctx, sqlcgen.UpdateAccountPasswordParams{
		ID:            accountID,
		TenantID:      tenantID,
		PasswordHash:  pgtypex.Text(passwordHash),
		MustChangePwd: mustChange,
		Status:        status,
		ActivatedAt:   timex.RequiredTimestamptz(now),
	})
	if err != nil {
		return Account{}, err
	}
	return Account{ID: row.ID, TenantID: row.TenantID, Name: row.Name, BaseIdentity: row.BaseIdentity, Status: row.Status}, nil
}

// ActivateSSOAccount 把 SSO 首登命中的待激活账号推进为正常状态。
func (t *txStore) ActivateSSOAccount(ctx context.Context, accountID, tenantID int64) (Account, error) {
	if _, err := t.q.ActivateSSOAccount(ctx, sqlcgen.ActivateSSOAccountParams{ID: accountID, TenantID: tenantID}); err != nil {
		return Account{}, err
	}
	return t.GetAccount(ctx, accountID)
}

// UpdateAccountPhone 更新账号手机号密文和查询哈希。
func (t *txStore) UpdateAccountPhone(ctx context.Context, tenantID, accountID int64, phoneEnc []byte, phoneHash string) (Account, error) {
	if err := t.q.UpdateAccountPhone(ctx, sqlcgen.UpdateAccountPhoneParams{ID: accountID, TenantID: tenantID, PhoneEnc: phoneEnc, PhoneHash: phoneHash}); err != nil {
		return Account{}, err
	}
	return t.GetAccount(ctx, accountID)
}

// RecordPasswordFailure 写入密码失败次数和可选锁定截止时间。
func (t *txStore) RecordPasswordFailure(ctx context.Context, accountID, tenantID int64, count int16, lockedUntil *time.Time) error {
	_, err := t.q.RecordPasswordFailure(ctx, sqlcgen.RecordPasswordFailureParams{
		ID:             accountID,
		TenantID:       tenantID,
		PwdFailedCount: count,
		LockedUntil:    pgtypex.TimestamptzPtr(lockedUntil),
	})
	return err
}

// ClearPasswordFailure 清除账号密码失败计数。
func (t *txStore) ClearPasswordFailure(ctx context.Context, accountID, tenantID int64) error {
	return t.q.ClearPasswordFailure(ctx, sqlcgen.ClearPasswordFailureParams{ID: accountID, TenantID: tenantID})
}

// GrantRole 授予账号角色。
func (t *txStore) GrantRole(ctx context.Context, tenantID, accountID int64, role int16, roleID int64) error {
	return t.q.CreateAccountRole(ctx, sqlcgen.CreateAccountRoleParams{ID: roleID, TenantID: tenantID, AccountID: accountID, Role: role})
}

// RevokeRole 撤销账号角色。
func (t *txStore) RevokeRole(ctx context.Context, tenantID, accountID int64, role int16) error {
	return t.q.DeleteAccountRole(ctx, sqlcgen.DeleteAccountRoleParams{TenantID: tenantID, AccountID: accountID, Role: role})
}

// CountActiveRoleAccounts 统计租户内具备指定角色且账号正常的账号数。
func (t *txStore) CountActiveRoleAccounts(ctx context.Context, tenantID int64, role int16) (int64, error) {
	return t.q.CountActiveRoleAccounts(ctx, sqlcgen.CountActiveRoleAccountsParams{TenantID: tenantID, Role: role})
}

// RevokeAccountSessions 吊销租户账号的全部有效会话。
func (t *txStore) RevokeAccountSessions(ctx context.Context, tenantID, accountID int64) error {
	return t.q.RevokeAccountSessions(ctx, sqlcgen.RevokeAccountSessionsParams{TenantID: tenantID, AccountID: accountID})
}

// RevokeOtherAccountSessions 吊销租户账号除当前会话外的全部有效会话。
func (t *txStore) RevokeOtherAccountSessions(ctx context.Context, tenantID, accountID, keepSessionID int64) error {
	return t.q.RevokeOtherAccountSessions(ctx, sqlcgen.RevokeOtherAccountSessionsParams{TenantID: tenantID, AccountID: accountID, ID: keepSessionID})
}

// CreateAuthSession 创建租户 Refresh 会话。
func (t *txStore) CreateAuthSession(ctx context.Context, input CreateSessionInput) (AuthSession, error) {
	row, err := t.q.CreateAuthSession(ctx, sqlcgen.CreateAuthSessionParams{
		ID:               input.ID,
		TenantID:         input.TenantID,
		AccountID:        input.AccountID,
		RefreshTokenHash: input.RefreshTokenHash,
		DeviceInfo:       pgtypex.Text(input.DeviceInfo),
		Ip:               pgtypex.Text(input.IP),
		Status:           SessionStatusActive,
		ExpireAt:         timex.RequiredTimestamptz(input.ExpireAt),
	})
	if err != nil {
		return AuthSession{}, err
	}
	return authSessionFromRow(row), nil
}

// GetAuthSessionByRefreshHash 读取租户 Refresh 会话。
func (t *txStore) GetAuthSessionByRefreshHash(ctx context.Context, hash string) (AuthSession, error) {
	row, err := t.q.GetAuthSessionByRefreshHashPrivileged(ctx, hash)
	if err != nil {
		return AuthSession{}, err
	}
	return authSessionFromRow(row), nil
}

// GetAuthSessionByID 按服务端会话 ID 读取租户 Refresh 会话。
func (t *txStore) GetAuthSessionByID(ctx context.Context, tenantID, sessionID int64) (AuthSession, error) {
	row, err := t.q.GetAuthSessionByID(ctx, sqlcgen.GetAuthSessionByIDParams{TenantID: tenantID, ID: sessionID})
	if err != nil {
		return AuthSession{}, err
	}
	return authSessionFromRow(row), nil
}

// ListAuthSessionsByAccount 读取当前账号全部 Refresh 会话供个人中心展示。
func (t *txStore) ListAuthSessionsByAccount(ctx context.Context, tenantID, accountID int64) ([]AuthSession, error) {
	rows, err := t.q.ListAuthSessionsByAccount(ctx, sqlcgen.ListAuthSessionsByAccountParams{TenantID: tenantID, AccountID: accountID})
	if err != nil {
		return nil, err
	}
	out := make([]AuthSession, 0, len(rows))
	for _, row := range rows {
		out = append(out, authSessionFromRow(row))
	}
	return out, nil
}

// RevokeAuthSession 吊销单个租户 Refresh 会话。
func (t *txStore) RevokeAuthSession(ctx context.Context, tenantID, sessionID int64) error {
	affected, err := t.q.RevokeAuthSessionByID(ctx, sqlcgen.RevokeAuthSessionByIDParams{TenantID: tenantID, ID: sessionID})
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperr.ErrIdentitySessionInvalid
	}
	return nil
}

// CreatePlatformAuthSession 创建平台管理员 Refresh 会话。
func (t *txStore) CreatePlatformAuthSession(ctx context.Context, input CreatePlatformSessionInput) (PlatformAuthSession, error) {
	row, err := t.q.CreatePlatformAuthSession(ctx, sqlcgen.CreatePlatformAuthSessionParams{
		ID:               input.ID,
		PlatformAdminID:  input.PlatformAdminID,
		RefreshTokenHash: input.RefreshTokenHash,
		DeviceInfo:       pgtypex.Text(input.DeviceInfo),
		Ip:               pgtypex.Text(input.IP),
		Status:           SessionStatusActive,
		ExpireAt:         timex.RequiredTimestamptz(input.ExpireAt),
	})
	if err != nil {
		return PlatformAuthSession{}, err
	}
	return PlatformAuthSession{ID: row.ID, PlatformAdminID: row.PlatformAdminID, RefreshTokenHash: row.RefreshTokenHash, Status: row.Status, ExpireAt: timex.FromTimestamptz(row.ExpireAt)}, nil
}

// GetPlatformAuthSessionByRefreshHash 读取平台管理员 Refresh 会话。
func (t *txStore) GetPlatformAuthSessionByRefreshHash(ctx context.Context, hash string) (PlatformAuthSession, error) {
	row, err := t.q.GetPlatformAuthSessionByRefreshHash(ctx, hash)
	if err != nil {
		return PlatformAuthSession{}, err
	}
	return PlatformAuthSession{ID: row.ID, PlatformAdminID: row.PlatformAdminID, RefreshTokenHash: row.RefreshTokenHash, Status: row.Status, ExpireAt: timex.FromTimestamptz(row.ExpireAt)}, nil
}

// GetPlatformAuthSessionByID 按服务端会话 ID 读取平台管理员 Refresh 会话。
func (t *txStore) GetPlatformAuthSessionByID(ctx context.Context, sessionID int64) (PlatformAuthSession, error) {
	row, err := t.q.GetPlatformAuthSessionByID(ctx, sessionID)
	if err != nil {
		return PlatformAuthSession{}, err
	}
	return PlatformAuthSession{ID: row.ID, PlatformAdminID: row.PlatformAdminID, RefreshTokenHash: row.RefreshTokenHash, DeviceInfo: pgtypex.TextValue(row.DeviceInfo), IP: pgtypex.TextValue(row.Ip), Status: row.Status, ExpireAt: timex.FromTimestamptz(row.ExpireAt), CreatedAt: timex.FromTimestamptz(row.CreatedAt)}, nil
}

// ListPlatformAuthSessionsByAdmin 读取平台管理员全部服务端 Refresh 会话。
func (t *txStore) ListPlatformAuthSessionsByAdmin(ctx context.Context, platformAdminID int64) ([]PlatformAuthSession, error) {
	rows, err := t.q.ListPlatformAuthSessionsByAdmin(ctx, platformAdminID)
	if err != nil {
		return nil, err
	}
	out := make([]PlatformAuthSession, 0, len(rows))
	for _, row := range rows {
		out = append(out, PlatformAuthSession{ID: row.ID, PlatformAdminID: row.PlatformAdminID, RefreshTokenHash: row.RefreshTokenHash, DeviceInfo: pgtypex.TextValue(row.DeviceInfo), IP: pgtypex.TextValue(row.Ip), Status: row.Status, ExpireAt: timex.FromTimestamptz(row.ExpireAt), CreatedAt: timex.FromTimestamptz(row.CreatedAt)})
	}
	return out, nil
}

// RevokePlatformSessions 吊销平台管理员全部有效会话。
func (t *txStore) RevokePlatformSessions(ctx context.Context, platformAdminID int64) error {
	return t.q.RevokePlatformSessions(ctx, platformAdminID)
}

// RevokeOtherPlatformSessions 吊销平台管理员除当前会话外的全部有效会话。
func (t *txStore) RevokeOtherPlatformSessions(ctx context.Context, platformAdminID, keepSessionID int64) error {
	return t.q.RevokeOtherPlatformSessions(ctx, sqlcgen.RevokeOtherPlatformSessionsParams{PlatformAdminID: platformAdminID, ID: keepSessionID})
}

// RevokePlatformAuthSession 吊销单个平台管理员会话。
func (t *txStore) RevokePlatformAuthSession(ctx context.Context, sessionID int64) error {
	affected, err := t.q.RevokePlatformAuthSessionByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperr.ErrIdentitySessionInvalid
	}
	return nil
}

// CreateSMSCode 写入短信验证码哈希。
func (t *txStore) CreateSMSCode(ctx context.Context, input CreateSMSCodeInput) (SMSCode, error) {
	row, err := t.q.CreateSMSCode(ctx, sqlcgen.CreateSMSCodeParams{
		ID:        input.ID,
		TenantID:  input.TenantID,
		PhoneHash: input.PhoneHash,
		CodeHash:  input.CodeHash,
		Scene:     input.Scene,
		ExpireAt:  timex.RequiredTimestamptz(input.ExpireAt),
	})
	if err != nil {
		return SMSCode{}, err
	}
	return SMSCode{ID: row.ID, TenantID: row.TenantID, PhoneHash: row.PhoneHash, CodeHash: row.CodeHash, Scene: row.Scene, ExpireAt: timex.FromTimestamptz(row.ExpireAt), VerifyAttempts: row.VerifyAttempts, Used: row.Used, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}, nil
}

// GetLatestSMSCode 读取最近一条验证码记录。
func (t *txStore) GetLatestSMSCode(ctx context.Context, tenantID int64, phoneHash string, scene int16) (SMSCode, error) {
	row, err := t.q.GetLatestSMSCode(ctx, sqlcgen.GetLatestSMSCodeParams{TenantID: tenantID, PhoneHash: phoneHash, Scene: scene})
	if err != nil {
		return SMSCode{}, err
	}
	return SMSCode{ID: row.ID, TenantID: row.TenantID, PhoneHash: row.PhoneHash, CodeHash: row.CodeHash, Scene: row.Scene, ExpireAt: timex.FromTimestamptz(row.ExpireAt), VerifyAttempts: row.VerifyAttempts, Used: row.Used, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}, nil
}

// MarkSMSCodeUsed 标记短信验证码已使用。
func (t *txStore) MarkSMSCodeUsed(ctx context.Context, tenantID, id int64) error {
	return t.q.MarkSMSCodeUsed(ctx, sqlcgen.MarkSMSCodeUsedParams{ID: id, TenantID: tenantID})
}

// IncrementSMSVerifyAttempts 增加短信验证码失败尝试次数。
func (t *txStore) IncrementSMSVerifyAttempts(ctx context.Context, tenantID, id int64) error {
	return t.q.IncrementSMSVerifyAttempts(ctx, sqlcgen.IncrementSMSVerifyAttemptsParams{ID: id, TenantID: tenantID})
}

// CreateActivationCode 写入账号激活码哈希。
func (t *txStore) CreateActivationCode(ctx context.Context, input CreateActivationInput) (ActivationCode, error) {
	row, err := t.q.CreateActivationCode(ctx, sqlcgen.CreateActivationCodeParams{
		ID:        input.ID,
		TenantID:  input.TenantID,
		AccountID: input.AccountID,
		CodeHash:  input.CodeHash,
		Status:    ActivationStatusActive,
		ExpireAt:  timex.RequiredTimestamptz(input.ExpireAt),
		CreatedBy: pgtypex.Int8(input.CreatedBy),
	})
	if err != nil {
		return ActivationCode{}, err
	}
	return ActivationCode{ID: row.ID, TenantID: row.TenantID, AccountID: row.AccountID, CodeHash: row.CodeHash, Status: row.Status, ExpireAt: timex.FromTimestamptz(row.ExpireAt)}, nil
}

// GetActivationCodeByHash 读取激活码哈希对应记录。
func (t *txStore) GetActivationCodeByHash(ctx context.Context, codeHash string) (ActivationCode, error) {
	row, err := t.q.GetActivationCodeByHashPrivileged(ctx, codeHash)
	if err != nil {
		return ActivationCode{}, err
	}
	return ActivationCode{ID: row.ID, TenantID: row.TenantID, AccountID: row.AccountID, CodeHash: row.CodeHash, Status: row.Status, ExpireAt: timex.FromTimestamptz(row.ExpireAt)}, nil
}

// UseActivationCode 标记激活码已使用。
func (t *txStore) UseActivationCode(ctx context.Context, tenantID, id int64) error {
	return t.q.UseActivationCode(ctx, sqlcgen.UseActivationCodeParams{ID: id, TenantID: tenantID})
}

// UpsertSSOConfig 创建或更新租户 SSO 配置。
func (t *txStore) UpsertSSOConfig(ctx context.Context, input UpsertSSOInput) (SSOConfig, error) {
	row, err := t.q.UpsertSSOConfig(ctx, sqlcgen.UpsertSSOConfigParams{
		ID:         input.ID,
		TenantID:   input.TenantID,
		Type:       input.Type,
		Config:     input.Config,
		MatchField: input.MatchField,
		Enabled:    input.Enabled,
	})
	if err != nil {
		return SSOConfig{}, err
	}
	return SSOConfig{ID: row.ID, TenantID: row.TenantID, Type: row.Type, Config: row.Config, MatchField: row.MatchField, Enabled: row.Enabled}, nil
}

// GetSSOConfig 读取指定类型 SSO 配置。
func (t *txStore) GetSSOConfig(ctx context.Context, tenantID int64, typ int16) (SSOConfig, error) {
	row, err := t.q.GetSSOConfig(ctx, sqlcgen.GetSSOConfigParams{TenantID: tenantID, Type: typ})
	if err != nil {
		return SSOConfig{}, err
	}
	return SSOConfig{ID: row.ID, TenantID: row.TenantID, Type: row.Type, Config: row.Config, MatchField: row.MatchField, Enabled: row.Enabled}, nil
}

// ListSSOConfigs 读取租户全部 SSO 配置。
func (t *txStore) ListSSOConfigs(ctx context.Context, tenantID int64) ([]SSOConfig, error) {
	rows, err := t.q.ListSSOConfigs(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]SSOConfig, 0, len(rows))
	for _, row := range rows {
		out = append(out, SSOConfig{ID: row.ID, TenantID: row.TenantID, Type: row.Type, Config: row.Config, MatchField: row.MatchField, Enabled: row.Enabled})
	}
	return out, nil
}

// CreateImportPreview 持久化导入预览中间态。
func (t *txStore) CreateImportPreview(ctx context.Context, input CreateImportPreviewInput) (ImportPreview, error) {
	row, err := t.q.CreateImportPreview(ctx, sqlcgen.CreateImportPreviewParams{
		ID:            input.ID,
		TenantID:      input.TenantID,
		OperatorID:    input.OperatorID,
		TargetType:    input.TargetType,
		FileName:      input.FileName,
		Rows:          input.Rows,
		PreviewResult: input.PreviewResult,
		Status:        ImportPreviewPending,
		ExpireAt:      timex.RequiredTimestamptz(input.ExpireAt),
	})
	if err != nil {
		return ImportPreview{}, err
	}
	return ImportPreview{ID: row.ID, TenantID: row.TenantID, OperatorID: row.OperatorID, TargetType: row.TargetType, FileName: row.FileName, Rows: row.Rows, PreviewResult: row.PreviewResult, Status: row.Status, ExpireAt: timex.FromTimestamptz(row.ExpireAt)}, nil
}

// GetImportPreview 按操作人读取导入预览中间态,防止同租户其他管理员提交不属于自己的预览。
func (t *txStore) GetImportPreview(ctx context.Context, tenantID, operatorID, id int64) (ImportPreview, error) {
	row, err := t.q.GetImportPreview(ctx, sqlcgen.GetImportPreviewParams{ID: id, TenantID: tenantID, OperatorID: operatorID})
	if err != nil {
		return ImportPreview{}, err
	}
	return ImportPreview{ID: row.ID, TenantID: row.TenantID, OperatorID: row.OperatorID, TargetType: row.TargetType, FileName: row.FileName, Rows: row.Rows, PreviewResult: row.PreviewResult, Status: row.Status, ExpireAt: timex.FromTimestamptz(row.ExpireAt)}, nil
}

// MarkImportPreviewSubmitted 标记当前操作人的导入预览已提交,防止重复提交和横向消费。
func (t *txStore) MarkImportPreviewSubmitted(ctx context.Context, tenantID, operatorID, id int64) error {
	affected, err := t.q.MarkImportPreviewSubmitted(ctx, sqlcgen.MarkImportPreviewSubmittedParams{ID: id, TenantID: tenantID, OperatorID: operatorID})
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperr.ErrIdentityImportPreviewExpired
	}
	return nil
}

// CreateImportBatch 写入导入批次结果。
func (t *txStore) CreateImportBatch(ctx context.Context, input CreateImportBatchInput) (ImportBatch, error) {
	row, err := t.q.CreateImportBatch(ctx, sqlcgen.CreateImportBatchParams{
		ID:          input.ID,
		TenantID:    input.TenantID,
		OperatorID:  input.OperatorID,
		TargetType:  input.TargetType,
		FileName:    input.FileName,
		Total:       input.Total,
		Success:     input.Success,
		Failed:      input.Failed,
		ErrorDetail: input.ErrorDetail,
		Status:      input.Status,
	})
	if err != nil {
		return ImportBatch{}, err
	}
	return ImportBatch{ID: row.ID, TenantID: row.TenantID, OperatorID: row.OperatorID, TargetType: row.TargetType, FileName: row.FileName, Total: row.Total, Success: row.Success, Failed: row.Failed, Status: row.Status, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}, nil
}

// ListImportBatches 读取导入批次历史。
func (t *txStore) ListImportBatches(ctx context.Context, tenantID int64) ([]ImportBatch, error) {
	rows, err := t.q.ListImportBatches(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]ImportBatch, 0, len(rows))
	for _, row := range rows {
		out = append(out, ImportBatch{ID: row.ID, TenantID: row.TenantID, OperatorID: row.OperatorID, TargetType: row.TargetType, FileName: row.FileName, Total: row.Total, Success: row.Success, Failed: row.Failed, Status: row.Status, CreatedAt: timex.FromTimestamptz(row.CreatedAt)})
	}
	return out, nil
}

// WriteAudit 写入全平台唯一 audit_log。
func (t *txStore) WriteAudit(ctx context.Context, input WriteAuditInput) error {
	_, err := t.q.CreateAuditLog(ctx, sqlcgen.CreateAuditLogParams{
		ID:         input.ID,
		TenantID:   pgtypex.Int8(input.TenantID),
		ActorID:    input.ActorID,
		ActorRole:  input.ActorRole,
		Action:     input.Action,
		TargetType: input.TargetType,
		TargetID:   pgtypex.Int8(input.TargetID),
		Detail:     input.Detail,
		Ip:         pgtypex.Text(input.IP),
		TraceID:    pgtypex.Text(input.TraceID),
	})
	return err
}

// QueryAuditLogs 按条件分页查询共享审计日志。
func (t *txStore) QueryAuditLogs(ctx context.Context, query AuditQueryInput) ([]AuditLogRow, int64, error) {
	rows, err := t.q.QueryAuditLogs(ctx, sqlcgen.QueryAuditLogsParams{
		Column1: query.TenantID,
		Column2: query.ActorID,
		Column3: query.Action,
		Column4: query.TargetType,
		Column5: timex.Timestamptz(query.From),
		Column6: timex.Timestamptz(query.To),
		Limit:   query.Size,
		Offset:  (query.Page - 1) * query.Size,
	})
	if err != nil {
		return nil, 0, err
	}
	// audit_log 的 tenant_id/target_id 允许平台级空值,转换时保留零值语义给 service 判定范围。
	out := make([]AuditLogRow, 0, len(rows))
	var total int64
	for _, row := range rows {
		total = row.TotalCount
		out = append(out, AuditLogRow{
			ID:         row.ID,
			TenantID:   pgtypex.Int8Value(row.TenantID),
			ActorID:    row.ActorID,
			ActorRole:  row.ActorRole,
			Action:     row.Action,
			TargetType: row.TargetType,
			TargetID:   pgtypex.Int8Value(row.TargetID),
			Detail:     string(row.Detail),
			IP:         pgtypex.TextValue(row.Ip),
			TraceID:    pgtypex.TextValue(row.TraceID),
			CreatedAt:  timex.FromTimestamptz(row.CreatedAt),
		})
	}
	return out, total, nil
}

// PlatformStats 读取平台身份统计。
func (t *txStore) PlatformStats(ctx context.Context) (StatsRow, error) {
	row, err := t.q.PlatformStats(ctx)
	if err != nil {
		return StatsRow{}, err
	}
	return StatsRow{
		TenantCount:          row.TenantCount,
		AccountCount:         row.AccountCount,
		TeacherCount:         row.TeacherCount,
		StudentCount:         row.StudentCount,
		SchoolAdminCount:     row.SchoolAdminCount,
		PlatformAdminCount:   row.PlatformAdminCount,
		ActiveAccountCount:   row.ActiveAccountCount,
		ActiveTenantCount:    row.ActiveTenantCount,
		PendingApplyCount:    row.PendingApplyCount,
		DisabledAccountCount: row.DisabledAccountCount,
	}, nil
}

// TenantStats 读取单租户身份统计。
func (t *txStore) TenantStats(ctx context.Context, tenantID int64) (StatsRow, error) {
	row, err := t.q.TenantStats(ctx, tenantID)
	if err != nil {
		return StatsRow{}, err
	}
	return StatsRow{
		AccountCount:         row.AccountCount,
		TeacherCount:         row.TeacherCount,
		StudentCount:         row.StudentCount,
		ActiveAccountCount:   row.ActiveAccountCount,
		DisabledAccountCount: row.DisabledAccountCount,
	}, nil
}
