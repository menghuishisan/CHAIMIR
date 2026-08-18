// acceptance_seed 提供本地验收测试数据初始化,覆盖核心业务闭环但不作为生产资料。
package main

import (
	"bytes"
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
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/crypto"

	"github.com/jackc/pgx/v5"
)

// acceptanceSeedIDs 统一管理验收数据固定 ID,保证重复运行更新同一批数据。
type acceptanceSeedIDs struct {
	TenantID             int64
	TenantIsolation      int64
	DepartmentCS         int64
	DepartmentSec        int64
	DepartmentIsolation  int64
	MajorChain           int64
	MajorSecurity        int64
	MajorIsolation       int64
	ClassChain           int64
	ClassSecurity        int64
	ClassIsolation       int64
	SchoolAdmin          int64
	TeacherMain          int64
	TeacherAssist        int64
	TeacherIsolation     int64
	StudentA             int64
	StudentB             int64
	StudentC             int64
	StudentIsolation     int64
	AuthSession          int64
	Runtime              int64
	RuntimeImage         int64
	Sandbox              int64
	SandboxTool          int64
	SandboxEvent         int64
	Judger               int64
	JudgerOnchain        int64
	JudgeTask            int64
	JudgeResult          int64
	ContentCat           int64
	ContentLab           int64
	ContentContest       int64
	ContentBattle        int64
	ContentTheory        int64
	Paper                int64
	Course               int64
	ChapterIntro         int64
	ChapterLab           int64
	LessonIntro          int64
	LessonLab            int64
	Assignment           int64
	AssignmentItem       int64
	SubmissionA          int64
	DraftB               int64
	ProgressA            int64
	Discussion           int64
	CourseNotice         int64
	CourseReview         int64
	GradeWeight          int64
	CourseGradeA         int64
	Experiment           int64
	ExperimentGroup      int64
	GroupMemberA         int64
	GroupMemberB         int64
	ExperimentInstance   int64
	CheckpointResult     int64
	ExperimentReport     int64
	SimSession           int64
	SimAction            int64
	SimCheckpoint        int64
	SimShare             int64
	Contest              int64
	ContestProblem       int64
	TeamA                int64
	TeamAMember          int64
	SolveSubmission      int64
	LadderRank           int64
	ResultSnapshot       int64
	VulnSource           int64
	VulnProblem          int64
	SystemAnnouncement   int64
	NotificationA        int64
	PreferenceA          int64
	AnnouncementReadA    int64
	GradeLevel           int64
	Semester             int64
	GradeReview          int64
	GradeAppeal          int64
	AcademicWarning      int64
	Transcript           int64
	SystemConfig         int64
	AlertRule            int64
	AlertEvent           int64
	Statistics           int64
	BackupRecord         int64
	TransferTask         int64
	AuditEntry           int64
	ApplicationPending   int64
	ApplicationRejected  int64
	ApplicationApproved  int64
	BattleContest        int64
	BattleContestProblem int64
	BattleTeamA          int64
	BattleTeamAMember    int64
	BattleTeamB          int64
	BattleTeamBMember    int64
	BattleEntryA         int64
	BattleEntryB         int64
	BattleMatch          int64
	BattleLadderRankA    int64
	BattleLadderRankB    int64
}

var acceptanceIDs = acceptanceSeedIDs{
	TenantID: 910000000000000001, TenantIsolation: 910000000000000002,
	DepartmentCS: 910000000000000011, DepartmentSec: 910000000000000012, DepartmentIsolation: 910000000000000013,
	MajorChain: 910000000000000021, MajorSecurity: 910000000000000022, MajorIsolation: 910000000000000023,
	ClassChain: 910000000000000031, ClassSecurity: 910000000000000032, ClassIsolation: 910000000000000033,
	SchoolAdmin: 910000000000000101, TeacherMain: 910000000000000102, TeacherAssist: 910000000000000103, TeacherIsolation: 910000000000000104,
	StudentA: 910000000000000201, StudentB: 910000000000000202, StudentC: 910000000000000203, StudentIsolation: 910000000000000204, AuthSession: 910000000000000301,
	Runtime: 910000000000001001, RuntimeImage: 910000000000001002,
	Sandbox: 910000000000001021, SandboxTool: 910000000000001022, SandboxEvent: 910000000000001023,
	Judger: 910000000000002001, JudgerOnchain: 910000000000002002, JudgeTask: 910000000000002011, JudgeResult: 910000000000002012,
	ContentCat: 910000000000004001, ContentLab: 910000000000004011, ContentContest: 910000000000004012, ContentBattle: 910000000000004014, ContentTheory: 910000000000004013, Paper: 910000000000004021,
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
	TransferTask:       910000000000013001,
	AuditEntry:         910000000000099001,
	ApplicationPending: 910000000000014001, ApplicationRejected: 910000000000014002, ApplicationApproved: 910000000000014003,
	BattleContest: 910000000000008101, BattleContestProblem: 910000000000008102,
	BattleTeamA: 910000000000008111, BattleTeamAMember: 910000000000008112,
	BattleTeamB: 910000000000008113, BattleTeamBMember: 910000000000008114,
	BattleEntryA: 910000000000008121, BattleEntryB: 910000000000008122, BattleMatch: 910000000000008131,
	BattleLadderRankA: 910000000000008141, BattleLadderRankB: 910000000000008142,
}

type acceptanceAccount struct {
	TenantID       int64
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
	objectStore, err := storage.New(ctx, cfg.MinIO)
	if err != nil {
		return err
	}
	if err := objectStore.EnsureBuckets(ctx); err != nil {
		return err
	}
	includeIsolationTenant := cfg.Deploy.PlatformEnabled
	tenantDeployMode := identity.DeployModeSaaS
	if cfg.Deploy.IsSchool() {
		tenantDeployMode = identity.DeployModeSchool
		// 私有化形态下「唯一那所学校」由 SCHOOL_TENANT_ID 指定,运行期的免鉴权品牌读取口
		// (GET /tenant/brand)就按它取租户。夹具租户 id 是固定常量,两者不一致时
		// 种子会造出一个运行期永远读不到的租户 —— 登录页没有校徽却查不出原因,
		// 故在这里直接失败,不把配置矛盾留到运行期。
		if cfg.Deploy.SchoolTenantID != acceptanceIDs.TenantID {
			return fmt.Errorf("DEPLOY_MODE=school 时 SCHOOL_TENANT_ID 必须为验收夹具租户 %d,当前=%d",
				acceptanceIDs.TenantID, cfg.Deploy.SchoolTenantID)
		}
	}
	if err := seedAcceptanceTenant(ctx, database, includeIsolationTenant, tenantDeployMode); err != nil {
		return err
	}
	if err := seedAcceptanceOrg(ctx, database, includeIsolationTenant); err != nil {
		return err
	}
	if err := seedAcceptanceAccounts(ctx, database, cfg.Bootstrap.AdminPassword, includeIsolationTenant); err != nil {
		return err
	}
	replayRef, replayKey, err := seedAcceptanceReplayObject(ctx, objectStore, cfg.MinIO.BucketReport)
	if err != nil {
		return err
	}
	if err := seedAcceptanceBusiness(ctx, database, includeIsolationTenant, replayRef); err != nil {
		if cleanupErr := objectStore.Delete(ctx, cfg.MinIO.BucketReport, replayKey); cleanupErr != nil {
			return fmt.Errorf("写入验收业务数据失败: %w; 清理回放对象失败: %v", err, cleanupErr)
		}
		return err
	}
	return nil
}

// seedAcceptanceReplayObject 写入一份结构完整的验收回放归档,供浏览器验证授权与下载链路。
// 归档是历史事实夹具,不代表运行时镜像已可用,也不触发沙箱或判题执行。
func seedAcceptanceReplayObject(ctx context.Context, objects *storage.Storage, bucket string) (string, string, error) {
	key, err := storage.ObjectKey(acceptanceIDs.TenantID, "contest", "replay", ids.Format(acceptanceIDs.BattleMatch), "acceptance-battle-replay.json")
	if err != nil {
		return "", "", err
	}
	ref, err := storage.ObjectRefString(bucket, key)
	if err != nil {
		return "", "", err
	}
	archive := map[string]any{
		"version":    1,
		"match_id":   ids.Format(acceptanceIDs.BattleMatch),
		"task_id":    "acceptance-battle-task-001",
		"source_ref": "contest:2026:battle:acceptance-001",
		"initial_state": map[string]any{
			"contest_id":  acceptanceIDs.BattleContest,
			"problem_id":  acceptanceIDs.BattleContestProblem,
			"battle_rule": 1,
			"entry_a":     map[string]any{"role": 2, "version_no": 1, "artifact_hash": strings.Repeat("a", 64)},
			"entry_b":     map[string]any{"role": 1, "version_no": 1, "artifact_hash": strings.Repeat("b", 64)},
		},
		"actions":     []map[string]any{{"seq": 1, "at_tick": 1, "event_type": "assertion", "payload": map[string]any{"passed": true}}},
		"result":      map[string]any{"passed": true, "score": 1, "max_score": 1, "details": map[string]any{"winner": "entry_a"}},
		"finished_at": timex.RFC3339OrEmpty(timex.Now()),
	}
	raw, err := json.Marshal(archive)
	if err != nil {
		return "", "", err
	}
	if err := objects.Put(ctx, bucket, key, bytes.NewReader(raw), int64(len(raw)), "application/json"); err != nil {
		return "", "", err
	}
	return ref, key, nil
}

// ensureAcceptanceSeedAllowed 防止验收夹具被误写入生产库。
func ensureAcceptanceSeedAllowed(cfg *config.Config) error {
	if !config.IsLocalLikeEnvironment(cfg.Server.AppEnv) {
		return fmt.Errorf("seed-acceptance 仅允许 APP_ENV 为 local/dev/development/test,当前 APP_ENV=%s", cfg.Server.AppEnv)
	}
	if err := identity.ValidatePassword(cfg.Bootstrap.AdminPassword); err != nil {
		return fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD 必须配置为符合本地密码强度的验收账号初始密码: %w", err)
	}
	return nil
}

// seedAcceptanceTenant 创建验收租户,不依赖生产 bootstrap 租户;隔离租户仅用于 SaaS 多租户验收。
//
// 刻意不写 logo_ref:校徽与课程封面存的是对象引用,而种子只写库不往 MinIO 放字节。
// 编一个引用出来会让验收环境「有校徽但取不到」——比没有校徽更难排查;留空则走设计好的
// 回落(徽记位显示校名首字、封面用平台纸材质),校徽/封面本身在验收时经上传入口真实产生。
// 同理 course.cover_ref 也不写(见 acceptance_seed_rows.go 的 course 插入)。
func seedAcceptanceTenant(ctx context.Context, database *db.DB, includeIsolation bool, deployMode int16) error {
	return database.WithPrivilegedTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tenants := []struct {
			id          int64
			code        string
			name        string
			displayName string
		}{
			{acceptanceIDs.TenantID, "acceptance-chainlab", "华东链安实验学院", "华东链安实验学院"},
		}
		if includeIsolation {
			tenants = append(tenants, struct {
				id          int64
				code        string
				name        string
				displayName string
			}{acceptanceIDs.TenantIsolation, "acceptance-isolation", "华南链安实验学院", "华南链安实验学院"})
		}
		for _, tenant := range tenants {
			if _, err := tx.Exec(ctx, `
INSERT INTO tenant (id, code, name, type, status, deploy_mode, display_name, feature_flags, auth_mode, enable_activation_code)
VALUES ($1, $2, $3, 3, 1, $5, $4, '{"modules":["teaching","experiment","contest"]}'::jsonb, 1, false)
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
			updated_at = now()`, tenant.id, tenant.code, tenant.name, tenant.displayName, deployMode); err != nil {
				return err
			}
		}
		return nil
	})
}

// seedAcceptanceOrg 创建院系、专业和班级,供账号档案和课程数据引用。
func seedAcceptanceOrg(ctx context.Context, database *db.DB, includeIsolation bool) error {
	orgs := []struct {
		tenantID, departmentID, majorID, classID     int64
		department, departmentCode, major, className string
	}{
		{acceptanceIDs.TenantID, acceptanceIDs.DepartmentCS, acceptanceIDs.MajorChain, acceptanceIDs.ClassChain, "计算机科学与技术学院", "CS", "区块链工程", "区块链工程 2026-1 班"},
		{acceptanceIDs.TenantID, acceptanceIDs.DepartmentSec, acceptanceIDs.MajorSecurity, acceptanceIDs.ClassSecurity, "网络空间安全学院", "SEC", "网络空间安全", "网络空间安全 2026-1 班"},
	}
	if includeIsolation {
		orgs = append(orgs, struct {
			tenantID, departmentID, majorID, classID     int64
			department, departmentCode, major, className string
		}{acceptanceIDs.TenantIsolation, acceptanceIDs.DepartmentIsolation, acceptanceIDs.MajorIsolation, acceptanceIDs.ClassIsolation, "信息工程学院", "IE", "软件工程", "软件工程 2026-1 班"})
	}
	for _, org := range orgs {
		if err := database.WithTenantTxID(ctx, org.tenantID, func(ctx context.Context, tx pgx.Tx) error {
			statements := []struct {
				sql  string
				args []any
			}{
				{`INSERT INTO department (id, tenant_id, name, code) VALUES ($1,$2,$3,$4) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, code=EXCLUDED.code, deleted_at=NULL`, []any{org.departmentID, org.tenantID, org.department, org.departmentCode}},
				{`INSERT INTO major (id, tenant_id, department_id, name) VALUES ($1,$2,$3,$4) ON CONFLICT (id) DO UPDATE SET department_id=EXCLUDED.department_id, name=EXCLUDED.name, deleted_at=NULL`, []any{org.majorID, org.tenantID, org.departmentID, org.major}},
				{`INSERT INTO class (id, tenant_id, major_id, name, enrollment_year, status) VALUES ($1,$2,$3,$4,2026,1) ON CONFLICT (id) DO UPDATE SET major_id=EXCLUDED.major_id, name=EXCLUDED.name, enrollment_year=EXCLUDED.enrollment_year, status=EXCLUDED.status, deleted_at=NULL`, []any{org.classID, org.tenantID, org.majorID, org.className}},
			}
			for _, stmt := range statements {
				if _, err := tx.Exec(ctx, stmt.sql, stmt.args...); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// seedAcceptanceAccounts 写入固定验收账号,复用正式密码哈希、手机号加密和数据库档案校验。
func seedAcceptanceAccounts(ctx context.Context, database *db.DB, initialPassword string, includeIsolation bool) error {
	accounts := []acceptanceAccount{
		{TenantID: acceptanceIDs.TenantID, ID: acceptanceIDs.SchoolAdmin, Phone: "13900001001", Name: "林明远", No: "T20260001", BaseIdentity: identity.BaseIdentityTeacher, OrgID: acceptanceIDs.DepartmentCS, Title: "教学副院长", Roles: []int16{contracts.RoleNumTeacher, contracts.RoleNumSchoolAdmin}},
		{TenantID: acceptanceIDs.TenantID, ID: acceptanceIDs.TeacherMain, Phone: "13900001002", Name: "周子衡", No: "T20260002", BaseIdentity: identity.BaseIdentityTeacher, OrgID: acceptanceIDs.DepartmentCS, Title: "副教授", Roles: []int16{contracts.RoleNumTeacher}},
		{TenantID: acceptanceIDs.TenantID, ID: acceptanceIDs.TeacherAssist, Phone: "13900001003", Name: "陈若水", No: "T20260003", BaseIdentity: identity.BaseIdentityTeacher, OrgID: acceptanceIDs.DepartmentSec, Title: "讲师", Roles: []int16{contracts.RoleNumTeacher}},
		{TenantID: acceptanceIDs.TenantID, ID: acceptanceIDs.StudentA, Phone: "13900002001", Name: "赵一航", No: "S20260001", BaseIdentity: identity.BaseIdentityStudent, OrgID: acceptanceIDs.ClassChain, EnrollmentYear: 2026, Roles: []int16{contracts.RoleNumStudent}},
		{TenantID: acceptanceIDs.TenantID, ID: acceptanceIDs.StudentB, Phone: "13900002002", Name: "钱思源", No: "S20260002", BaseIdentity: identity.BaseIdentityStudent, OrgID: acceptanceIDs.ClassChain, EnrollmentYear: 2026, Roles: []int16{contracts.RoleNumStudent}},
		{TenantID: acceptanceIDs.TenantID, ID: acceptanceIDs.StudentC, Phone: "13900002003", Name: "孙明珂", No: "S20260003", BaseIdentity: identity.BaseIdentityStudent, OrgID: acceptanceIDs.ClassSecurity, EnrollmentYear: 2026, Roles: []int16{contracts.RoleNumStudent}},
	}
	if includeIsolation {
		accounts = append(accounts,
			acceptanceAccount{TenantID: acceptanceIDs.TenantIsolation, ID: acceptanceIDs.TeacherIsolation, Phone: "13900001002", Name: "周子衡（隔离租户）", No: "T20260002-I", BaseIdentity: identity.BaseIdentityTeacher, OrgID: acceptanceIDs.DepartmentIsolation, Title: "副教授", Roles: []int16{contracts.RoleNumTeacher}},
			acceptanceAccount{TenantID: acceptanceIDs.TenantIsolation, ID: acceptanceIDs.StudentIsolation, Phone: "13900002001", Name: "赵一航（隔离租户）", No: "S20260001-I", BaseIdentity: identity.BaseIdentityStudent, OrgID: acceptanceIDs.ClassIsolation, EnrollmentYear: 2026, Roles: []int16{contracts.RoleNumStudent}},
		)
	}
	for _, account := range accounts {
		if err := ensureAcceptanceAccount(ctx, database, account.TenantID, account, initialPassword); err != nil {
			return err
		}
	}
	return seedAcceptanceAuthSession(ctx, database)
}

// ensureAcceptanceAccount 幂等写入单个账号、角色和组织档案。
func ensureAcceptanceAccount(ctx context.Context, database *db.DB, tenantID int64, account acceptanceAccount, initialPassword string) error {
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
	return database.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
INSERT INTO account (id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd, activated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,2,false,now())
ON CONFLICT (id) DO UPDATE SET phone_enc=EXCLUDED.phone_enc, phone_hash=EXCLUDED.phone_hash, password_hash=EXCLUDED.password_hash, name=EXCLUDED.name, base_identity=EXCLUDED.base_identity, status=EXCLUDED.status, must_change_pwd=EXCLUDED.must_change_pwd, activated_at=EXCLUDED.activated_at, deleted_at=NULL, updated_at=now()`,
			account.ID, tenantID, phoneEnc, phoneHash, passwordHash, account.Name, account.BaseIdentity); err != nil {
			return err
		}
		for i, role := range account.Roles {
			if err := upsertAccountRole(ctx, tx, tenantID, account.ID, role, acceptanceRoleID(account.ID, i)); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `
INSERT INTO account_profile (account_id, tenant_id, no, org_id, enrollment_year, title)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (account_id) DO UPDATE SET no=EXCLUDED.no, org_id=EXCLUDED.org_id, enrollment_year=EXCLUDED.enrollment_year, title=EXCLUDED.title`,
			account.ID, tenantID, account.No, account.OrgID, nullInt16(account.EnrollmentYear), account.Title)
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
func seedAcceptanceBusiness(ctx context.Context, database *db.DB, includeIsolation bool, replayRef string) error {
	if includeIsolation {
		if err := database.WithPrivilegedTx(ctx, seedApplicationRows); err != nil {
			return err
		}
	}
	if err := database.WithTenantTxID(ctx, acceptanceIDs.TenantID, func(ctx context.Context, tx pgx.Tx) error {
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
		if err := seedContestReplayRows(ctx, tx, replayRef); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if !includeIsolation {
		return nil
	}
	// 隔离租户保持业务空态，但必须拥有可计算成绩的默认配置，否则学生成绩中心会进入配置错误态。
	return database.WithTenantTxID(ctx, acceptanceIDs.TenantIsolation, seedIsolationGradeRows)
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
	// 复用生产同一套加密 + HMAC 组合,种子与 identity 共享唯一实现,避免手机号保护算法漂移。
	return crypto.ProtectPhone(cipher, []byte(osEnv("APP_HMAC_KEY")), phone)
}

// osEnv 读取已由 config 校验过的环境变量,便于 seed 内复用密钥。
func osEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// upsertAccountRole 幂等写入账号角色。
func upsertAccountRole(ctx context.Context, tx pgx.Tx, tenantID, accountID int64, role int16, roleID int64) error {
	_, err := tx.Exec(ctx, `
INSERT INTO account_role (id, tenant_id, account_id, role)
VALUES ($1,$2,$3,$4)
ON CONFLICT (tenant_id, account_id, role) DO NOTHING`, roleID, tenantID, accountID, role)
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
