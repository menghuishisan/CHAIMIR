// identity repo 文件定义模块持久化接口和数据库事务边界,是 service 访问数据库的唯一入口。
package identity

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

// Store 定义 service 所需的 identity 持久化能力,不暴露 sqlc 行类型。
type Store interface {
	PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error
	TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error
	PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error
}

// TxStore 定义单个事务内可调用的 identity 数据访问能力。
type TxStore interface {
	UseTenant(ctx context.Context, tenantID int64) error
	GetPlatformAdminByUsername(ctx context.Context, username string) (PlatformAdmin, error)
	GetPlatformAdminByID(ctx context.Context, id int64) (PlatformAdmin, error)
	CreatePlatformAdminIfNotExists(ctx context.Context, input CreatePlatformAdminInput) error
	UpdatePlatformAdminPassword(ctx context.Context, adminID int64, passwordHash string) error
	GetTenantByCode(ctx context.Context, code string) (Tenant, error)
	GetTenantByID(ctx context.Context, id int64) (Tenant, error)
	ListTenants(ctx context.Context) ([]Tenant, error)
	CreateTenant(ctx context.Context, input CreateTenantInput) (Tenant, error)
	UpdateTenantConfig(ctx context.Context, input UpdateTenantConfigInput) (Tenant, error)
	UpdateTenantStatus(ctx context.Context, input UpdateTenantStatusInput) (Tenant, error)
	CreateTenantApplication(ctx context.Context, input CreateApplicationRequest, id int64) (TenantApplication, error)
	GetTenantApplication(ctx context.Context, id int64) (TenantApplication, error)
	ListTenantApplications(ctx context.Context, status int16) ([]TenantApplication, error)
	ApproveTenantApplication(ctx context.Context, id, reviewerID, tenantID int64) (TenantApplication, error)
	RejectTenantApplication(ctx context.Context, id, reviewerID int64, reason string) (TenantApplication, error)
	CreateAccount(ctx context.Context, input CreateAccountInput) (Account, error)
	GetAccount(ctx context.Context, id int64) (Account, error)
	BatchGetAccounts(ctx context.Context, ids []int64) ([]Account, error)
	ListAccountsByPhoneHash(ctx context.Context, phoneHash string) ([]LoginCandidate, error)
	GetAccountByPhoneHash(ctx context.Context, tenantID int64, phoneHash string) (Account, error)
	GetAccountByNo(ctx context.Context, no string) (Account, error)
	ListAccounts(ctx context.Context, query AccountQuery) ([]Account, int64, error)
	UpdateAccountEditable(ctx context.Context, tenantID, accountID int64, req UpdateAccountRequest) (Account, error)
	UpdateAccountStatus(ctx context.Context, accountID, tenantID int64, status int16, deleted bool) (Account, error)
	UpdateAccountPassword(ctx context.Context, accountID, tenantID int64, passwordHash string, mustChange bool, status int16) (Account, error)
	ActivateSSOAccount(ctx context.Context, accountID, tenantID int64) (Account, error)
	UpdateAccountPhone(ctx context.Context, tenantID, accountID int64, phoneEnc []byte, phoneHash string) (Account, error)
	RecordPasswordFailure(ctx context.Context, accountID, tenantID int64, count int16, lockedUntil *time.Time) error
	ClearPasswordFailure(ctx context.Context, accountID, tenantID int64) error
	GrantRole(ctx context.Context, tenantID, accountID int64, role int16, roleID int64) error
	RevokeRole(ctx context.Context, tenantID, accountID int64, role int16) error
	CountActiveRoleAccounts(ctx context.Context, tenantID int64, role int16) (int64, error)
	RevokeAccountSessions(ctx context.Context, tenantID, accountID int64) error
	RevokeOtherAccountSessions(ctx context.Context, tenantID, accountID, keepSessionID int64) error
	CreateAuthSession(ctx context.Context, input CreateSessionInput) (AuthSession, error)
	GetAuthSessionByRefreshHash(ctx context.Context, hash string) (AuthSession, error)
	GetAuthSessionByID(ctx context.Context, tenantID, sessionID int64) (AuthSession, error)
	ListAuthSessionsByAccount(ctx context.Context, tenantID, accountID int64) ([]AuthSession, error)
	RevokeAuthSession(ctx context.Context, tenantID, sessionID int64) error
	CreatePlatformAuthSession(ctx context.Context, input CreatePlatformSessionInput) (PlatformAuthSession, error)
	GetPlatformAuthSessionByRefreshHash(ctx context.Context, hash string) (PlatformAuthSession, error)
	GetPlatformAuthSessionByID(ctx context.Context, sessionID int64) (PlatformAuthSession, error)
	ListPlatformAuthSessionsByAdmin(ctx context.Context, platformAdminID int64) ([]PlatformAuthSession, error)
	RevokePlatformSessions(ctx context.Context, platformAdminID int64) error
	RevokeOtherPlatformSessions(ctx context.Context, platformAdminID, keepSessionID int64) error
	RevokePlatformAuthSession(ctx context.Context, sessionID int64) error
	CreateSMSCode(ctx context.Context, input CreateSMSCodeInput) (SMSCode, error)
	GetLatestSMSCode(ctx context.Context, tenantID int64, phoneHash string, scene int16) (SMSCode, error)
	MarkSMSCodeUsed(ctx context.Context, tenantID, id int64) error
	IncrementSMSVerifyAttempts(ctx context.Context, tenantID, id int64) error
	CreateActivationCode(ctx context.Context, input CreateActivationInput) (ActivationCode, error)
	GetActivationCodeByHash(ctx context.Context, codeHash string) (ActivationCode, error)
	UseActivationCode(ctx context.Context, tenantID, id int64) error
	UpsertSSOConfig(ctx context.Context, input UpsertSSOInput) (SSOConfig, error)
	GetSSOConfig(ctx context.Context, tenantID int64, typ int16) (SSOConfig, error)
	ListSSOConfigs(ctx context.Context, tenantID int64) ([]SSOConfig, error)
	CreateImportPreview(ctx context.Context, input CreateImportPreviewInput) (ImportPreview, error)
	GetImportPreview(ctx context.Context, tenantID, operatorID, id int64) (ImportPreview, error)
	MarkImportPreviewSubmitted(ctx context.Context, tenantID, operatorID, id int64) error
	CreateImportBatch(ctx context.Context, input CreateImportBatchInput) (ImportBatch, error)
	ListImportBatches(ctx context.Context, tenantID int64) ([]ImportBatch, error)
	WriteAudit(ctx context.Context, input WriteAuditInput) error
	QueryAuditLogs(ctx context.Context, query AuditQueryInput) ([]AuditLogRow, int64, error)
	PlatformStats(ctx context.Context) (StatsRow, error)
	TenantStats(ctx context.Context, tenantID int64) (StatsRow, error)
	CreateDepartment(ctx context.Context, tenantID, id int64, req DepartmentRequest) (Department, error)
	ListDepartments(ctx context.Context) ([]Department, error)
	DepartmentExists(ctx context.Context, tenantID, id int64) (bool, error)
	UpdateDepartment(ctx context.Context, tenantID, id int64, req DepartmentRequest) (Department, error)
	DeleteDepartment(ctx context.Context, tenantID, id int64) error
	CreateMajor(ctx context.Context, tenantID, id int64, req MajorRequest) (Major, error)
	ListMajors(ctx context.Context, departmentID int64) ([]Major, error)
	MajorExists(ctx context.Context, tenantID, id int64) (bool, error)
	UpdateMajor(ctx context.Context, tenantID, id int64, req MajorRequest) (Major, error)
	DeleteMajor(ctx context.Context, tenantID, id int64) error
	CreateClass(ctx context.Context, tenantID, id int64, req ClassRequest) (Class, error)
	ListClasses(ctx context.Context, majorID int64) ([]Class, error)
	ClassExists(ctx context.Context, tenantID, id int64) (bool, error)
	UpdateClass(ctx context.Context, tenantID, id int64, req ClassRequest) (Class, error)
	DeleteClass(ctx context.Context, tenantID, id int64) error
	ArchiveClassesByEnrollmentYear(ctx context.Context, tenantID int64, enrollmentYear int16) error
	ArchiveStudentAccountsByEnrollmentYear(ctx context.Context, tenantID int64, enrollmentYear int16) error
	RevokeStudentSessionsByEnrollmentYear(ctx context.Context, tenantID int64, enrollmentYear int16) error
	PromoteClasses(ctx context.Context, tenantID int64) error
}

// store 使用 platform/db 统一事务入口实现 identity 自有表访问。
type store struct {
	database *db.DB
}

// txStore 封装单事务 sqlc 查询对象。
type txStore struct {
	q  *sqlcgen.Queries
	tx pgx.Tx
}

// NewStore 创建 identity 模块持久化入口,仅装配层应调用。
func NewStore(database *db.DB) Store {
	return &store{database: database}
}

// UseTenant 在当前事务中注入 RLS 租户变量,用于平台审核创建租户后的同事务首管账号写入。
func (t *txStore) UseTenant(ctx context.Context, tenantID int64) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("identity tx store 未初始化")
	}
	if tenantID <= 0 {
		return fmt.Errorf("tenant_id 必须大于 0")
	}
	if _, err := t.tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", strconv.FormatInt(tenantID, 10)); err != nil {
		return fmt.Errorf("注入 app.tenant_id 失败: %w", err)
	}
	return nil
}

// PlatformTx 在应用连接中访问平台级表和平台管理员路径。
func (s *store) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("identity store 未初始化")
	}
	return s.database.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx), tx: tx})
	})
}

// TenantTx 在注入 RLS 租户变量后访问租户内自有表。
func (s *store) TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("identity store 未初始化")
	}
	return s.database.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx), tx: tx})
	})
}

// PrivilegedTx 用特权连接处理预认证定位和平台统计,不得作为普通业务路径使用。
func (s *store) PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("identity store 未初始化")
	}
	return s.database.WithPrivilegedModuleTx(ctx, "identity", func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx), tx: tx})
	})
}
