// acceptance_seed 提供本地验收测试数据初始化,覆盖核心业务闭环但不作为生产资料。
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/identity"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/pkg/crypto"

	"github.com/jackc/pgx/v5"
)

// acceptanceSeedIDs 统一管理验收数据固定 ID,保证重复运行更新同一批数据。
type acceptanceSeedIDs struct {
	TenantID           int64
	DepartmentCS       int64
	DepartmentSec      int64
	MajorChain         int64
	MajorSecurity      int64
	ClassChain         int64
	ClassSecurity      int64
	SchoolAdmin        int64
	TeacherMain        int64
	TeacherAssist      int64
	StudentA           int64
	StudentB           int64
	StudentC           int64
	AuthSession        int64
	Runtime            int64
	RuntimeImage       int64
	Sandbox            int64
	SandboxTool        int64
	SandboxEvent       int64
	Judger             int64
	JudgeTask          int64
	JudgeResult        int64
	SimPackage         int64
	ContentCat         int64
	ContentLab         int64
	ContentContest     int64
	ContentTheory      int64
	Paper              int64
	Course             int64
	ChapterIntro       int64
	ChapterLab         int64
	LessonIntro        int64
	LessonLab          int64
	Assignment         int64
	AssignmentItem     int64
	SubmissionA        int64
	DraftB             int64
	ProgressA          int64
	Discussion         int64
	CourseNotice       int64
	CourseReview       int64
	GradeWeight        int64
	CourseGradeA       int64
	Experiment         int64
	ExperimentGroup    int64
	GroupMemberA       int64
	GroupMemberB       int64
	ExperimentInstance int64
	CheckpointResult   int64
	ExperimentReport   int64
	SimSession         int64
	SimAction          int64
	SimCheckpoint      int64
	SimShare           int64
	Contest            int64
	ContestProblem     int64
	TeamA              int64
	TeamAMember        int64
	SolveSubmission    int64
	LadderRank         int64
	ResultSnapshot     int64
	VulnSource         int64
	VulnProblem        int64
	SystemAnnouncement int64
	NotificationA      int64
	PreferenceA        int64
	AnnouncementReadA  int64
	GradeLevel         int64
	Semester           int64
	GradeReview        int64
	GradeAppeal        int64
	AcademicWarning    int64
	Transcript         int64
	SystemConfig       int64
	AlertRule          int64
	AlertEvent         int64
	Statistics         int64
	BackupRecord       int64
	TransferTask       int64
	AuditEntry         int64
}

var acceptanceIDs = acceptanceSeedIDs{
	TenantID: 910000000000000001, DepartmentCS: 910000000000000011, DepartmentSec: 910000000000000012,
	MajorChain: 910000000000000021, MajorSecurity: 910000000000000022, ClassChain: 910000000000000031, ClassSecurity: 910000000000000032,
	SchoolAdmin: 910000000000000101, TeacherMain: 910000000000000102, TeacherAssist: 910000000000000103,
	StudentA: 910000000000000201, StudentB: 910000000000000202, StudentC: 910000000000000203, AuthSession: 910000000000000301,
	Runtime: 910000000000001001, RuntimeImage: 910000000000001002,
	Sandbox: 910000000000001021, SandboxTool: 910000000000001022, SandboxEvent: 910000000000001023,
	Judger: 910000000000002001, JudgeTask: 910000000000002011, JudgeResult: 910000000000002012, SimPackage: 910000000000003001,
	ContentCat: 910000000000004001, ContentLab: 910000000000004011, ContentContest: 910000000000004012, ContentTheory: 910000000000004013, Paper: 910000000000004021,
	Course: 910000000000005001, ChapterIntro: 910000000000005011, ChapterLab: 910000000000005012, LessonIntro: 910000000000005021, LessonLab: 910000000000005022,
	Assignment: 910000000000005031, AssignmentItem: 910000000000005032, SubmissionA: 910000000000005041, DraftB: 910000000000005042, ProgressA: 910000000000005043,
	Discussion: 910000000000005044, CourseNotice: 910000000000005045, CourseReview: 910000000000005046, GradeWeight: 910000000000005047, CourseGradeA: 910000000000005048,
	Experiment: 910000000000006001, ExperimentGroup: 910000000000006011, GroupMemberA: 910000000000006012, GroupMemberB: 910000000000006013,
	ExperimentInstance: 910000000000006021, CheckpointResult: 910000000000006022, ExperimentReport: 910000000000006023,
	SimSession: 910000000000007001, SimAction: 910000000000007002, SimCheckpoint: 910000000000007003, SimShare: 910000000000007004,
	Contest: 910000000000008001, ContestProblem: 910000000000008002, TeamA: 910000000000008011, TeamAMember: 910000000000008012, SolveSubmission: 910000000000008021,
	LadderRank: 910000000000008031, ResultSnapshot: 910000000000008032, VulnSource: 910000000000008041, VulnProblem: 910000000000008042,
	SystemAnnouncement: 910000000000010001, NotificationA: 910000000000010011, PreferenceA: 910000000000010012, AnnouncementReadA: 910000000000010013,
	GradeLevel: 910000000000011001, Semester: 910000000000011002, GradeReview: 910000000000011003, GradeAppeal: 910000000000011004,
	AcademicWarning: 910000000000011005, Transcript: 910000000000011006,
	SystemConfig: 910000000000012001, AlertRule: 910000000000012002, AlertEvent: 910000000000012003, Statistics: 910000000000012004, BackupRecord: 910000000000012005,
	TransferTask: 910000000000013001,
	AuditEntry:   910000000000099001,
}

type acceptanceAccount struct {
	ID             int64
	Phone          string
	Name           string
	No             string
	BaseIdentity   int16
	OrgID          int64
	EnrollmentYear int16
	Title          string
	Roles          []int16
}

// seedAcceptance 写入本地验收测试所需的真实业务夹具数据。
func seedAcceptance(ctx context.Context, cfg *config.Config) error {
	if err := ensureAcceptanceSeedAllowed(cfg); err != nil {
		return err
	}
	database, err := db.New(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := seedAcceptanceTenant(ctx, database); err != nil {
		return err
	}
	if err := seedAcceptanceOrg(ctx, database); err != nil {
		return err
	}
	if err := seedAcceptanceAccounts(ctx, database, cfg.Bootstrap.AdminPassword); err != nil {
		return err
	}
	if err := seedAcceptanceBusiness(ctx, database); err != nil {
		return err
	}
	return nil
}

// ensureAcceptanceSeedAllowed 防止验收夹具被误写入生产库。
func ensureAcceptanceSeedAllowed(cfg *config.Config) error {
	appEnv := strings.ToLower(strings.TrimSpace(cfg.Server.AppEnv))
	mode := strings.ToLower(strings.TrimSpace(cfg.Deploy.Mode))
	if appEnv != "local" && appEnv != "dev" && appEnv != "development" && mode != "local" && mode != "dev" {
		return fmt.Errorf("seed-acceptance 仅允许 APP_ENV/DEPLOY_MODE 为 local/dev/development,当前 APP_ENV=%s DEPLOY_MODE=%s", cfg.Server.AppEnv, cfg.Deploy.Mode)
	}
	if err := identity.ValidatePassword(cfg.Bootstrap.AdminPassword); err != nil {
		return fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD 必须配置为符合本地密码强度的验收账号初始密码: %w", err)
	}
	return nil
}

// seedAcceptanceTenant 创建一所完整的验收租户,不依赖生产 bootstrap 租户。
func seedAcceptanceTenant(ctx context.Context, database *db.DB) error {
	return database.WithPrivilegedTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
INSERT INTO tenant (id, code, name, type, status, deploy_mode, display_name, feature_flags, auth_mode, enable_activation_code)
VALUES ($1, 'acceptance-chainlab', '华东链安实验学院', 3, 1, 2, '华东链安实验学院', '{"modules":["teaching","experiment","contest"]}'::jsonb, 1, false)
ON CONFLICT (id) DO UPDATE SET
	code = EXCLUDED.code,
	name = EXCLUDED.name,
	type = EXCLUDED.type,
	status = EXCLUDED.status,
	deploy_mode = EXCLUDED.deploy_mode,
	display_name = EXCLUDED.display_name,
	feature_flags = EXCLUDED.feature_flags,
	auth_mode = EXCLUDED.auth_mode,
	enable_activation_code = EXCLUDED.enable_activation_code,
	updated_at = now()`, acceptanceIDs.TenantID)
		return err
	})
}

// seedAcceptanceOrg 创建院系、专业和班级,供账号档案和课程数据引用。
func seedAcceptanceOrg(ctx context.Context, database *db.DB) error {
	return database.WithTenantTxID(ctx, acceptanceIDs.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		statements := []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO department (id, tenant_id, name, code) VALUES ($1,$2,'计算机科学与技术学院','CS') ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, code=EXCLUDED.code, deleted_at=NULL`, []any{acceptanceIDs.DepartmentCS, acceptanceIDs.TenantID}},
			{`INSERT INTO department (id, tenant_id, name, code) VALUES ($1,$2,'网络空间安全学院','SEC') ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, code=EXCLUDED.code, deleted_at=NULL`, []any{acceptanceIDs.DepartmentSec, acceptanceIDs.TenantID}},
			{`INSERT INTO major (id, tenant_id, department_id, name) VALUES ($1,$2,$3,'区块链工程') ON CONFLICT (id) DO UPDATE SET department_id=EXCLUDED.department_id, name=EXCLUDED.name, deleted_at=NULL`, []any{acceptanceIDs.MajorChain, acceptanceIDs.TenantID, acceptanceIDs.DepartmentCS}},
			{`INSERT INTO major (id, tenant_id, department_id, name) VALUES ($1,$2,$3,'网络空间安全') ON CONFLICT (id) DO UPDATE SET department_id=EXCLUDED.department_id, name=EXCLUDED.name, deleted_at=NULL`, []any{acceptanceIDs.MajorSecurity, acceptanceIDs.TenantID, acceptanceIDs.DepartmentSec}},
			{`INSERT INTO class (id, tenant_id, major_id, name, enrollment_year, status) VALUES ($1,$2,$3,'区块链工程 2026-1 班',2026,1) ON CONFLICT (id) DO UPDATE SET major_id=EXCLUDED.major_id, name=EXCLUDED.name, enrollment_year=EXCLUDED.enrollment_year, status=EXCLUDED.status, deleted_at=NULL`, []any{acceptanceIDs.ClassChain, acceptanceIDs.TenantID, acceptanceIDs.MajorChain}},
			{`INSERT INTO class (id, tenant_id, major_id, name, enrollment_year, status) VALUES ($1,$2,$3,'网络空间安全 2026-1 班',2026,1) ON CONFLICT (id) DO UPDATE SET major_id=EXCLUDED.major_id, name=EXCLUDED.name, enrollment_year=EXCLUDED.enrollment_year, status=EXCLUDED.status, deleted_at=NULL`, []any{acceptanceIDs.ClassSecurity, acceptanceIDs.TenantID, acceptanceIDs.MajorSecurity}},
		}
		for _, stmt := range statements {
			if _, err := tx.Exec(ctx, stmt.sql, stmt.args...); err != nil {
				return err
			}
		}
		return nil
	})
}

// seedAcceptanceAccounts 写入固定验收账号,复用正式密码哈希、手机号加密和数据库档案校验。
func seedAcceptanceAccounts(ctx context.Context, database *db.DB, initialPassword string) error {
	for _, account := range []acceptanceAccount{
		{ID: acceptanceIDs.SchoolAdmin, Phone: "13900001001", Name: "林明远", No: "T20260001", BaseIdentity: identity.BaseIdentityTeacher, OrgID: acceptanceIDs.DepartmentCS, Title: "教学副院长", Roles: []int16{contracts.RoleNumTeacher, contracts.RoleNumSchoolAdmin}},
		{ID: acceptanceIDs.TeacherMain, Phone: "13900001002", Name: "周子衡", No: "T20260002", BaseIdentity: identity.BaseIdentityTeacher, OrgID: acceptanceIDs.DepartmentCS, Title: "副教授", Roles: []int16{contracts.RoleNumTeacher}},
		{ID: acceptanceIDs.TeacherAssist, Phone: "13900001003", Name: "陈若水", No: "T20260003", BaseIdentity: identity.BaseIdentityTeacher, OrgID: acceptanceIDs.DepartmentSec, Title: "讲师", Roles: []int16{contracts.RoleNumTeacher}},
		{ID: acceptanceIDs.StudentA, Phone: "13900002001", Name: "赵一航", No: "S20260001", BaseIdentity: identity.BaseIdentityStudent, OrgID: acceptanceIDs.ClassChain, EnrollmentYear: 2026, Roles: []int16{contracts.RoleNumStudent}},
		{ID: acceptanceIDs.StudentB, Phone: "13900002002", Name: "钱思源", No: "S20260002", BaseIdentity: identity.BaseIdentityStudent, OrgID: acceptanceIDs.ClassChain, EnrollmentYear: 2026, Roles: []int16{contracts.RoleNumStudent}},
		{ID: acceptanceIDs.StudentC, Phone: "13900002003", Name: "孙明珂", No: "S20260003", BaseIdentity: identity.BaseIdentityStudent, OrgID: acceptanceIDs.ClassSecurity, EnrollmentYear: 2026, Roles: []int16{contracts.RoleNumStudent}},
	} {
		if err := ensureAcceptanceAccount(ctx, database, account, initialPassword); err != nil {
			return err
		}
	}
	return seedAcceptanceAuthSession(ctx, database)
}

// ensureAcceptanceAccount 幂等写入单个账号、角色和组织档案。
func ensureAcceptanceAccount(ctx context.Context, database *db.DB, account acceptanceAccount, initialPassword string) error {
	if err := identity.ValidatePhone(account.Phone); err != nil {
		return err
	}
	if err := identity.ValidatePassword(initialPassword); err != nil {
		return err
	}
	phoneEnc, phoneHash, err := protectedPhone(account.Phone)
	if err != nil {
		return err
	}
	passwordHash, err := crypto.HashPassword(initialPassword)
	if err != nil {
		return err
	}
	return database.WithTenantTxID(ctx, acceptanceIDs.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
INSERT INTO account (id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd, activated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,2,false,now())
ON CONFLICT (id) DO UPDATE SET phone_enc=EXCLUDED.phone_enc, phone_hash=EXCLUDED.phone_hash, password_hash=EXCLUDED.password_hash, name=EXCLUDED.name, base_identity=EXCLUDED.base_identity, status=EXCLUDED.status, must_change_pwd=EXCLUDED.must_change_pwd, activated_at=EXCLUDED.activated_at, deleted_at=NULL, updated_at=now()`,
			account.ID, acceptanceIDs.TenantID, phoneEnc, phoneHash, passwordHash, account.Name, account.BaseIdentity); err != nil {
			return err
		}
		for i, role := range account.Roles {
			if err := upsertAccountRole(ctx, tx, account.ID, role, acceptanceRoleID(account.ID, i)); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `
INSERT INTO account_profile (account_id, tenant_id, no, org_id, enrollment_year, title)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (account_id) DO UPDATE SET no=EXCLUDED.no, org_id=EXCLUDED.org_id, enrollment_year=EXCLUDED.enrollment_year, title=EXCLUDED.title`,
			account.ID, acceptanceIDs.TenantID, account.No, account.OrgID, nullInt16(account.EnrollmentYear), account.Title)
		return err
	})
}

// seedAcceptanceAuthSession 写入一条已吊销会话,用于会话列表和非法 Refresh 测试。
func seedAcceptanceAuthSession(ctx context.Context, database *db.DB) error {
	refreshHash, err := crypto.HMACHash([]byte(osEnv("APP_HMAC_KEY")), "acceptance-revoked-refresh-token")
	if err != nil {
		return err
	}
	return database.WithTenantTxID(ctx, acceptanceIDs.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
INSERT INTO auth_session (id, tenant_id, account_id, refresh_token_hash, device_info, ip, status, expire_at)
VALUES ($1,$2,$3,$4,'acceptance-seed-revoked','127.0.0.1',2,now() - interval '1 hour')
ON CONFLICT (id) DO UPDATE SET refresh_token_hash=EXCLUDED.refresh_token_hash, device_info=EXCLUDED.device_info, ip=EXCLUDED.ip, status=EXCLUDED.status, expire_at=EXCLUDED.expire_at`,
			acceptanceIDs.AuthSession, acceptanceIDs.TenantID, acceptanceIDs.StudentA, refreshHash)
		return err
	})
}

// acceptanceRoleID 按账号固定 ID 派生角色行 ID,避免相邻账号之间发生主键碰撞。
func acceptanceRoleID(accountID int64, index int) int64 {
	return accountID*10 + int64(index+1)
}

// seedAcceptanceBusiness 写入跨模块验收业务数据。
func seedAcceptanceBusiness(ctx context.Context, database *db.DB) error {
	return database.WithTenantTxID(ctx, acceptanceIDs.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		for _, fn := range []func(context.Context, pgx.Tx) error{
			seedRuntimeRows,
			seedContentRows,
			seedTeachingRows,
			seedExperimentRows,
			seedSimRows,
			seedContestRows,
			seedNotifyRows,
			seedGradeRows,
			seedAdminRows,
			seedTransferRows,
			seedAuditRows,
		} {
			if err := fn(ctx, tx); err != nil {
				return err
			}
		}
		return nil
	})
}

// protectedPhone 复用生产加密与 HMAC 算法生成手机号持久化字段。
func protectedPhone(phone string) ([]byte, string, error) {
	keyRaw := osEnv("APP_ENCRYPTION_KEY")
	key, err := base64.StdEncoding.DecodeString(keyRaw)
	if err != nil {
		return nil, "", fmt.Errorf("解析 APP_ENCRYPTION_KEY 失败: %w", err)
	}
	cipher, err := crypto.NewCipher(key)
	if err != nil {
		return nil, "", err
	}
	phoneEnc, err := cipher.Encrypt([]byte(phone))
	if err != nil {
		return nil, "", err
	}
	phoneHash, err := crypto.HMACHash([]byte(osEnv("APP_HMAC_KEY")), phone)
	if err != nil {
		return nil, "", err
	}
	return phoneEnc, phoneHash, nil
}

// osEnv 读取已由 config 校验过的环境变量,便于 seed 内复用密钥。
func osEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// upsertAccountRole 幂等写入账号角色。
func upsertAccountRole(ctx context.Context, tx pgx.Tx, accountID int64, role int16, roleID int64) error {
	_, err := tx.Exec(ctx, `
INSERT INTO account_role (id, tenant_id, account_id, role)
VALUES ($1,$2,$3,$4)
ON CONFLICT (tenant_id, account_id, role) DO NOTHING`, roleID, acceptanceIDs.TenantID, accountID, role)
	return err
}

// nullInt16 把可选 int16 转为数据库 NULL。
func nullInt16(value int16) any {
	if value == 0 {
		return nil
	}
	return value
}

// jsonb 把结构化 seed 数据编码为 JSONB 入参。
func jsonb(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// execJSON 执行带 JSONB 参数的 SQL,集中包装编码错误。
func execJSON(ctx context.Context, tx pgx.Tx, sqlText string, args ...any) error {
	_, err := tx.Exec(ctx, sqlText, args...)
	return err
}
