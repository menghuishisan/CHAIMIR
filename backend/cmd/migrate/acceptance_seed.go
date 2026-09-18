// acceptance_seed 提供受控验收测试数据初始化,覆盖核心业务闭环但不作为生产资料。
package main

import (
	"archive/tar"
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
	"chaimir/internal/platform/upload"
	"chaimir/pkg/crypto"

	"github.com/jackc/pgx/v5"
)

// acceptanceSeedIDs 统一管理验收数据固定 ID,保证重复运行更新同一批数据。
type acceptanceSeedIDs struct {
	TenantID               int64
	TenantIsolation        int64
	DepartmentCS           int64
	DepartmentSec          int64
	DepartmentIsolation    int64
	MajorChain             int64
	MajorSecurity          int64
	MajorIsolation         int64
	ClassChain             int64
	ClassSecurity          int64
	ClassIsolation         int64
	SchoolAdmin            int64
	TeacherMain            int64
	TeacherAssist          int64
	TeacherIsolation       int64
	StudentA               int64
	StudentB               int64
	StudentC               int64
	StudentIsolation       int64
	AuthSession            int64
	Runtime                int64
	RuntimeImage           int64
	Sandbox                int64
	SandboxTool            int64
	SandboxEvent           int64
	SandboxContestSolve    int64
	SandboxBattle          int64
	Judger                 int64
	JudgerOnchain          int64
	JudgeTask              int64
	JudgeResult            int64
	ContestJudgeTask       int64
	ContestJudgeResult     int64
	BattleJudgeTask        int64
	BattleJudgeResult      int64
	ContentCat             int64
	ContentLab             int64
	ContentContest         int64
	ContentBattle          int64
	ContentVuln            int64
	ContentTheory          int64
	Paper                  int64
	Course                 int64
	ChapterIntro           int64
	ChapterLab             int64
	LessonIntro            int64
	LessonLab              int64
	Assignment             int64
	AssignmentItem         int64
	SubmissionA            int64
	DraftB                 int64
	ProgressA              int64
	Discussion             int64
	CourseNotice           int64
	CourseReview           int64
	GradeWeight            int64
	CourseGradeA           int64
	Experiment             int64
	ExperimentGroup        int64
	GroupMemberA           int64
	GroupMemberB           int64
	ExperimentInstance     int64
	CheckpointResult       int64
	ExperimentReport       int64
	SimSession             int64
	SimAction              int64
	SimCheckpoint          int64
	SimShare               int64
	Contest                int64
	ContestProblem         int64
	TeamA                  int64
	TeamAMember            int64
	SolveSubmission        int64
	LadderRank             int64
	ResultSnapshot         int64
	VulnSource             int64
	VulnProblem            int64
	SystemAnnouncement     int64
	NotificationA          int64
	PreferenceA            int64
	AnnouncementReadA      int64
	GradeLevel             int64
	Semester               int64
	GradeReview            int64
	GradeAppeal            int64
	AcademicWarning        int64
	Transcript             int64
	SystemConfig           int64
	AlertRule              int64
	AlertEvent             int64
	Statistics             int64
	BackupRecord           int64
	TransferTask           int64
	AuditEntry             int64
	ApplicationPending     int64
	ApplicationRejected    int64
	ApplicationApproved    int64
	BattleContest          int64
	BattleContestProblem   int64
	BattleTeamA            int64
	BattleTeamAMember      int64
	BattleTeamB            int64
	BattleTeamBMember      int64
	BattleEntryA           int64
	BattleEntryB           int64
	BattleMatch            int64
	BattleLadderRankA      int64
	BattleLadderRankB      int64
	ExperimentJudgeTaskA   int64
	ExperimentJudgeResultA int64
	ExperimentJudgeTaskB   int64
	ExperimentJudgeResultB int64
	ExperimentScoreOutbox  int64
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
	SandboxContestSolve: 910000000000001031, SandboxBattle: 910000000000001041,
	Judger: platformJudgerID("testcase-evm"), JudgerOnchain: platformJudgerID("onchain-assert"), JudgeTask: 910000000000002011, JudgeResult: 910000000000002012,
	ContestJudgeTask: 910000000000002015, ContestJudgeResult: 910000000000002016,
	BattleJudgeTask: 910000000000002013, BattleJudgeResult: 910000000000002014,
	ContentCat: 910000000000004001, ContentLab: 910000000000004011, ContentContest: 910000000000004012, ContentBattle: 910000000000004014, ContentVuln: 910000000000004015, ContentTheory: 910000000000004013, Paper: 910000000000004021,
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
	ExperimentJudgeTaskA: 910000000000006031, ExperimentJudgeResultA: 910000000000006032,
	ExperimentJudgeTaskB: 910000000000006033, ExperimentJudgeResultB: 910000000000006034,
	ExperimentScoreOutbox: 910000000000006035,
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

// seedAcceptance 写入受控验收测试所需的真实业务夹具数据。
func seedAcceptance(ctx context.Context, cfg *config.Config) error {
	if err := ensureAcceptanceSeedAllowed(cfg); err != nil {
		return err
	}
	database, err := db.New(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer database.Close()
	objectStore, err := storage.New(ctx, cfg.ObjectStorage)
	if err != nil {
		return err
	}
	if err := objectStore.EnsureBuckets(ctx); err != nil {
		return err
	}
	materialRefs, err := acceptanceMaterialRefsForBucket(cfg.ObjectStorage.BucketCode)
	if err != nil {
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
	if err := seedAcceptanceExperimentAssets(ctx, objectStore, cfg.ObjectStorage.BucketCode, materialRefs); err != nil {
		return err
	}
	if err := seedAcceptanceExperimentReportAsset(ctx, objectStore, cfg.ObjectStorage.BucketReport); err != nil {
		return err
	}
	if err := seedAcceptanceTransferAsset(ctx, objectStore, cfg.ObjectStorage.BucketReport); err != nil {
		return err
	}
	if err := seedAcceptanceTranscriptAsset(ctx, objectStore, cfg.ObjectStorage.BucketReport); err != nil {
		return err
	}
	if err := seedAcceptanceContestAssets(ctx, objectStore, cfg.ObjectStorage.BucketCode, materialRefs); err != nil {
		return err
	}
	replayRef, replayKey, err := seedAcceptanceReplayObject(ctx, objectStore, cfg.ObjectStorage.BucketReport, materialRefs)
	if err != nil {
		return err
	}
	if err := seedAcceptanceBusiness(ctx, database, includeIsolationTenant, replayRef, cfg.ObjectStorage.BucketReport, materialRefs); err != nil {
		if cleanupErr := objectStore.Delete(ctx, cfg.ObjectStorage.BucketReport, replayKey); cleanupErr != nil {
			return fmt.Errorf("写入验收业务数据失败: %w; 清理回放对象失败: %v", err, cleanupErr)
		}
		return err
	}
	return nil
}

// acceptanceSeedObject 描述验收流程实际读取的项目自有对象,统一由种子写入代码桶。
type acceptanceSeedObject struct {
	ObjectRef   string
	Content     []byte
	ContentType string
}

// acceptanceEVMGenesisRef 返回验收判题使用的受控创世材料引用,唯一来源是 judger manifest。
func acceptanceEVMGenesisRef() (string, error) {
	root, err := platformImagesRoot()
	if err != nil {
		return "", err
	}
	manifest, err := platformJudgerManifestForCode(root, "testcase-evm")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(manifest.Judger.GenesisRef), nil
}

// acceptanceMaterialRefs 统一管理验收流程实际使用的对象引用,所有引用都由项目对象键规则生成。
type acceptanceMaterialRefs struct {
	InitCodeRef             string
	InitScriptRef           string
	JudgeSuiteRef           string
	TeachingSubmissionRef   string
	TeachingSanitizedRef    string
	ContestSubmissionRef    string
	ContestSanitizedRef     string
	ExperimentSanitizedARef string
	ExperimentSanitizedBRef string
	BattleEntryARef         string
	BattleEntryBRef         string
}

// acceptanceMaterialRefsForBucket 按正式对象存储桶和租户边界生成验收材料引用。
func acceptanceMaterialRefsForBucket(codeBucket string) (acceptanceMaterialRefs, error) {
	objectRef := func(module, resourceType string, parts ...string) (string, error) {
		key, err := storage.ObjectKey(acceptanceIDs.TenantID, module, resourceType, parts...)
		if err != nil {
			return "", err
		}
		return storage.ObjectRefString(codeBucket, key)
	}
	initCodeRef, err := objectRef("sandbox", "init", "lab-reentrancy-foundry", "workspace.tar")
	if err != nil {
		return acceptanceMaterialRefs{}, fmt.Errorf("生成验收初始化归档引用失败: %w", err)
	}
	initScriptRef, err := objectRef("sandbox", "init", "lab-reentrancy-foundry", "init.sh")
	if err != nil {
		return acceptanceMaterialRefs{}, fmt.Errorf("生成验收初始化脚本引用失败: %w", err)
	}
	judgeSuiteRef, err := objectRef("judge", "suites", "ctf-reentrancy-vault", "public-regression.tar")
	if err != nil {
		return acceptanceMaterialRefs{}, fmt.Errorf("生成验收判题套件引用失败: %w", err)
	}
	teachingSubmissionRef, err := objectRef("teaching", "submission", ids.Format(acceptanceIDs.SubmissionA), "submission.tar")
	if err != nil {
		return acceptanceMaterialRefs{}, fmt.Errorf("生成验收教学提交引用失败: %w", err)
	}
	teachingSanitizedRef, err := objectRef("judge", "input", ids.Format(acceptanceIDs.JudgeTask), "submission.tar")
	if err != nil {
		return acceptanceMaterialRefs{}, fmt.Errorf("生成验收教学脱敏归档引用失败: %w", err)
	}
	contestSubmissionRef, err := objectRef("contest", "submission", ids.Format(acceptanceIDs.SolveSubmission), "submission.tar")
	if err != nil {
		return acceptanceMaterialRefs{}, fmt.Errorf("生成验收竞赛提交引用失败: %w", err)
	}
	contestSanitizedRef, err := objectRef("judge", "input", ids.Format(acceptanceIDs.ContestJudgeTask), "submission.tar")
	if err != nil {
		return acceptanceMaterialRefs{}, fmt.Errorf("生成验收竞赛脱敏归档引用失败: %w", err)
	}
	experimentSanitizedARef, err := objectRef("judge", "input", ids.Format(acceptanceIDs.ExperimentJudgeTaskA), "submission.tar")
	if err != nil {
		return acceptanceMaterialRefs{}, fmt.Errorf("生成验收实验检查点 A 脱敏归档引用失败: %w", err)
	}
	experimentSanitizedBRef, err := objectRef("judge", "input", ids.Format(acceptanceIDs.ExperimentJudgeTaskB), "submission.tar")
	if err != nil {
		return acceptanceMaterialRefs{}, fmt.Errorf("生成验收实验检查点 B 脱敏归档引用失败: %w", err)
	}
	battleEntryARef, err := objectRef("contest", "battle", ids.Format(acceptanceIDs.BattleEntryA), "artifact.tar")
	if err != nil {
		return acceptanceMaterialRefs{}, fmt.Errorf("生成验收攻方参战物引用失败: %w", err)
	}
	battleEntryBRef, err := objectRef("contest", "battle", ids.Format(acceptanceIDs.BattleEntryB), "artifact.tar")
	if err != nil {
		return acceptanceMaterialRefs{}, fmt.Errorf("生成验收守方参战物引用失败: %w", err)
	}
	return acceptanceMaterialRefs{
		InitCodeRef:             initCodeRef,
		InitScriptRef:           initScriptRef,
		JudgeSuiteRef:           judgeSuiteRef,
		TeachingSubmissionRef:   teachingSubmissionRef,
		TeachingSanitizedRef:    teachingSanitizedRef,
		ContestSubmissionRef:    contestSubmissionRef,
		ContestSanitizedRef:     contestSanitizedRef,
		ExperimentSanitizedARef: experimentSanitizedARef,
		ExperimentSanitizedBRef: experimentSanitizedBRef,
		BattleEntryARef:         battleEntryARef,
		BattleEntryBRef:         battleEntryBRef,
	}, nil
}

// seedAcceptanceContestAssets 写入竞赛解题、判题套件和攻防参战物的受控归档。
// 这些对象不是镜像内置材料,而是 M8/M3/M2 通过统一对象存储引用的运行时输入。
func seedAcceptanceContestAssets(ctx context.Context, objects *storage.Storage, codeBucket string, refs acceptanceMaterialRefs) error {
	assets, err := acceptanceContestAssets(refs)
	if err != nil {
		return err
	}
	for _, asset := range assets {
		ref, err := storage.ParseObjectRef(asset.ObjectRef)
		if err != nil {
			return fmt.Errorf("解析竞赛验收对象引用失败: %w", err)
		}
		if ref.Bucket != codeBucket {
			return fmt.Errorf("竞赛验收对象必须位于代码桶: ref=%s bucket=%s", asset.ObjectRef, codeBucket)
		}
		if err := objects.Put(ctx, ref.Bucket, ref.Key, bytes.NewReader(asset.Content), int64(len(asset.Content)), asset.ContentType); err != nil {
			return fmt.Errorf("写入竞赛验收对象失败: ref=%s: %w", asset.ObjectRef, err)
		}
	}
	return nil
}

// acceptanceContestAssetHash 返回与对象存储中内容一致的原始归档 SHA-256。
func acceptanceContestAssetHash(objectRef string, refs acceptanceMaterialRefs) (string, error) {
	assets, err := acceptanceContestAssets(refs)
	if err != nil {
		return "", err
	}
	for _, asset := range assets {
		if asset.ObjectRef == objectRef {
			return crypto.SHA256Hex(asset.Content), nil
		}
	}
	return "", fmt.Errorf("未找到竞赛验收对象: %s", objectRef)
}

// acceptanceContestAssets 构造竞赛各阶段共用的确定性材料,避免数据库行引用不存在的对象。
func acceptanceContestAssets(refs acceptanceMaterialRefs) ([]acceptanceSeedObject, error) {
	suite, err := acceptanceJudgeSuiteArchive()
	if err != nil {
		return nil, err
	}
	submission, err := acceptanceContestSubmissionArchive()
	if err != nil {
		return nil, err
	}
	sanitizedSubmission, err := acceptanceSanitizedArchive("submission.tar", submission, "竞赛提交脱敏归档")
	if err != nil {
		return nil, err
	}
	attack, err := acceptanceBattleEntryArchive("attack")
	if err != nil {
		return nil, err
	}
	defense, err := acceptanceBattleEntryArchive("defense")
	if err != nil {
		return nil, err
	}
	return []acceptanceSeedObject{
		{ObjectRef: refs.JudgeSuiteRef, Content: suite, ContentType: "application/x-tar"},
		{ObjectRef: refs.TeachingSubmissionRef, Content: submission, ContentType: "application/x-tar"},
		{ObjectRef: refs.TeachingSanitizedRef, Content: sanitizedSubmission, ContentType: "application/x-tar"},
		{ObjectRef: refs.ContestSubmissionRef, Content: submission, ContentType: "application/x-tar"},
		{ObjectRef: refs.ContestSanitizedRef, Content: sanitizedSubmission, ContentType: "application/x-tar"},
		{ObjectRef: refs.BattleEntryARef, Content: attack, ContentType: "application/x-tar"},
		{ObjectRef: refs.BattleEntryBRef, Content: defense, ContentType: "application/x-tar"},
	}, nil
}

// acceptanceSanitizedArchive 使用判题模块相同的安全归档原语生成脱敏输入,不把原始提交字节冒充为脱敏产物。
func acceptanceSanitizedArchive(name string, raw []byte, purpose string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%s不能为空", purpose)
	}
	// 验收归档是仓库内确定性材料,上限只用于复用正式安全校验;正式运行仍由配置提供最终限制。
	sanitized, err := upload.SafeArchiveTar(name, raw, upload.ArchiveLimits{
		MaxFiles:         64,
		MaxUnpackedBytes: int64(len(raw)),
	})
	if err != nil {
		return nil, fmt.Errorf("生成%s失败: %w", purpose, err)
	}
	return sanitized, nil
}

// acceptanceJudgeSuiteArchive 构造真正读取参赛工程的 Foundry 公开测试套件。
func acceptanceJudgeSuiteArchive() ([]byte, error) {
	return acceptanceTarArchive([]acceptanceArchiveFile{{
		Name: "test/ReentrancyVault.t.sol",
		Body: `// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

interface Vm {
    function deal(address who, uint256 newBalance) external;
}

import "../src/ReentrancyVault.sol";

contract ReentrancyProbe {
    ReentrancyVault private immutable vault;
    uint256 public reentryAttempts;
    uint256 public reentryFailures;

    constructor(ReentrancyVault target) {
        vault = target;
    }

    function attack() external payable {
        vault.deposit{value: msg.value}();
        vault.withdraw();
    }

    receive() external payable {
        reentryAttempts++;
        (bool ok,) = address(vault).call(abi.encodeWithSignature("withdraw()"));
        if (!ok) {
            reentryFailures++;
        }
    }
}

contract ReentrancyVaultPublicTest {
    Vm private constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    function test_depositAndWithdraw() public {
        vm.deal(address(this), 1 ether);
        ReentrancyVault vault = new ReentrancyVault();
        vault.deposit{value: 1 ether}();
        require(vault.balances(address(this)) == 1 ether, "deposit was not recorded");
        vault.withdraw();
        require(vault.balances(address(this)) == 0, "balance was not cleared");
        require(address(vault).balance == 0, "vault retained funds");
    }

    function test_reentry_is_rejected_and_existing_funds_are_preserved() public {
        vm.deal(address(this), 3 ether);
        ReentrancyVault vault = new ReentrancyVault();
        vault.deposit{value: 2 ether}();
        ReentrancyProbe probe = new ReentrancyProbe(vault);

        probe.attack{value: 1 ether}();

        require(probe.reentryAttempts() == 1, "probe did not receive withdrawal callback");
        require(probe.reentryFailures() == 1, "reentrant withdrawal was not rejected");
        require(vault.balances(address(probe)) == 0, "probe balance was not cleared");
        require(vault.balances(address(this)) == 2 ether, "existing balance was changed");
        require(address(vault).balance == 2 ether, "vault funds were drained");

        vault.withdraw();
        require(vault.balances(address(this)) == 0, "owner balance was not cleared");
        require(address(vault).balance == 0, "vault retained funds after normal withdrawal");
    }

    receive() external payable {}
}
`,
	}}, "竞赛公开判题套件")
}

// acceptanceContestSubmissionArchive 构造可被公开套件实际编译和执行的提交归档。
func acceptanceContestSubmissionArchive() ([]byte, error) {
	return acceptanceTarArchive([]acceptanceArchiveFile{
		{Name: "foundry.toml", Body: "[profile.default]\nsrc = \"src\"\ntest = \"test\"\nout = \"out\"\nlibs = []\nsolc = \"/usr/local/bin/solc-0.8.20\"\noffline = true\n"},
		{Name: "src/ReentrancyVault.sol", Body: `// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract ReentrancyVault {
    mapping(address => uint256) public balances;

    function deposit() external payable {
        balances[msg.sender] += msg.value;
    }

    function withdraw() external {
        uint256 amount = balances[msg.sender];
        require(amount > 0, "no balance");
        balances[msg.sender] = 0;
        (bool sent,) = payable(msg.sender).call{value: amount}("");
        require(sent, "transfer failed");
    }
}
`},
		{Name: "README.md", Body: "# 可重入漏洞修复提交\n\n该归档提供按题目约束修复后的金库合约。公开套件在受控 EVM runtime 中验证正常存取款、拒绝重入和既有资金不被改变。\n"},
	}, "竞赛解题提交归档")
}

// acceptanceBattleEntryArchive 构造可被 M2 安全恢复的攻防参战物归档。
func acceptanceBattleEntryArchive(role string) ([]byte, error) {
	files := []acceptanceArchiveFile{
		{Name: "README.md", Body: "# 攻防参战物\n\n该归档只在对局沙箱的对应角色目录中使用。\n"},
		{Name: "foundry.toml", Body: `[profile.default]
src = "src"
out = "out"
libs = []
solc = "/usr/local/bin/solc-0.8.20"
offline = true
`},
	}
	switch role {
	case "attack":
		files = append(files, acceptanceArchiveFile{Name: "src/ReentrancyAttack.sol", Body: `// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface IReentrancyVault {
    function deposit() external payable;
    function withdraw() external;
}

contract ReentrancyAttack {
    IReentrancyVault public immutable target;
    uint256 public reentryCount;

    constructor(address targetAddress) {
        target = IReentrancyVault(targetAddress);
    }

    function attack() external payable {
        require(msg.value > 0, "funding required");
        target.deposit{value: msg.value}();
        target.withdraw();
    }

    receive() external payable {
        reentryCount += 1;
        if (address(target).balance >= msg.value && reentryCount < 2) {
            target.withdraw();
        }
    }
}
`})
	case "defense":
		files = append(files, acceptanceArchiveFile{Name: "src/VaultDefense.sol", Body: `// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract VaultDefense {
    mapping(address => uint256) public balances;

    function deposit() external payable {
        balances[msg.sender] += msg.value;
    }

    function withdraw() external {
        uint256 amount = balances[msg.sender];
        require(amount > 0, "no balance");
        balances[msg.sender] = 0;
        (bool sent,) = payable(msg.sender).call{value: amount}("");
        require(sent, "transfer failed");
    }
}
`})
	default:
		return nil, fmt.Errorf("竞赛参战物角色无效: %s", role)
	}
	return acceptanceTarArchive(files, "攻防参战物")
}

type acceptanceArchiveFile struct {
	Name string
	Body string
}

// acceptanceTarArchive 统一构造只含普通文件的确定性 TAR,与平台 SafeArchiveTar 输入契约一致。
func acceptanceTarArchive(files []acceptanceArchiveFile, purpose string) ([]byte, error) {
	var out bytes.Buffer
	writer := tar.NewWriter(&out)
	for _, file := range files {
		if strings.TrimSpace(file.Name) == "" {
			return nil, fmt.Errorf("%s 包含空归档成员名", purpose)
		}
		header := &tar.Header{Name: file.Name, Mode: 0o644, Size: int64(len(file.Body)), Typeflag: tar.TypeReg}
		if err := writer.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("写入%s归档头失败: %w", purpose, err)
		}
		if _, err := writer.Write([]byte(file.Body)); err != nil {
			return nil, fmt.Errorf("写入%s归档内容失败: %w", purpose, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭%s归档失败: %w", purpose, err)
	}
	return out.Bytes(), nil
}

// seedAcceptanceExperimentAssets 写入实验启动实际会读取的工作区归档与初始化脚本。
func seedAcceptanceExperimentAssets(ctx context.Context, objects *storage.Storage, codeBucket string, refs acceptanceMaterialRefs) error {
	archive, err := acceptanceExperimentArchive()
	if err != nil {
		return err
	}
	sanitizedArchive, err := acceptanceSanitizedArchive("workspace.tar", archive, "实验检查点脱敏归档")
	if err != nil {
		return err
	}
	workspaceRef, err := acceptanceSandboxWorkspaceRef(codeBucket)
	if err != nil {
		return err
	}
	assets := []struct {
		objectRef   string
		content     []byte
		contentType string
	}{
		{objectRef: refs.InitCodeRef, content: archive, contentType: "application/x-tar"},
		{objectRef: workspaceRef, content: archive, contentType: "application/x-tar"},
		{objectRef: refs.ExperimentSanitizedARef, content: sanitizedArchive, contentType: "application/x-tar"},
		{objectRef: refs.ExperimentSanitizedBRef, content: sanitizedArchive, contentType: "application/x-tar"},
		// 初始化阶段只在 workspace helper 中校验已注入材料;编译必须通过 runtime 原生 deploy helper 执行。
		{objectRef: refs.InitScriptRef, content: acceptanceExperimentInitScript(), contentType: "text/x-shellscript"},
	}
	for _, asset := range assets {
		ref, err := storage.ParseObjectRef(asset.objectRef)
		if err != nil {
			return fmt.Errorf("解析验收实验初始化对象引用失败: %w", err)
		}
		if ref.Bucket != codeBucket {
			return fmt.Errorf("验收实验初始化对象必须位于代码桶: ref=%s bucket=%s", asset.objectRef, codeBucket)
		}
		if err := objects.Put(ctx, ref.Bucket, ref.Key, bytes.NewReader(asset.content), int64(len(asset.content)), asset.contentType); err != nil {
			return fmt.Errorf("写入验收实验初始化对象失败: ref=%s: %w", asset.objectRef, err)
		}
	}
	return nil
}

// acceptanceSandboxWorkspaceKey 返回历史沙箱保存工作区使用的规范对象键。
func acceptanceSandboxWorkspaceKey() (string, error) {
	return storage.ObjectKey(acceptanceIDs.TenantID, "sandbox", "code", ids.Format(acceptanceIDs.Sandbox), "workspace.tar")
}

// acceptanceSandboxWorkspaceRef 返回历史沙箱保存工作区使用的规范对象引用。
func acceptanceSandboxWorkspaceRef(codeBucket string) (string, error) {
	key, err := acceptanceSandboxWorkspaceKey()
	if err != nil {
		return "", fmt.Errorf("生成验收沙箱工作区对象 key 失败: %w", err)
	}
	ref, err := storage.ObjectRefString(codeBucket, key)
	if err != nil {
		return "", fmt.Errorf("生成验收沙箱工作区对象引用失败: %w", err)
	}
	return ref, nil
}

// seedAcceptanceExperimentReportAsset 写入报告审核链路实际读取的受控报告对象。
func seedAcceptanceExperimentReportAsset(ctx context.Context, objects *storage.Storage, reportBucket string) error {
	ref, err := acceptanceExperimentReportRef(reportBucket)
	if err != nil {
		return err
	}
	parsed, err := storage.ParseObjectRef(ref)
	if err != nil {
		return fmt.Errorf("解析验收实验报告对象引用失败: %w", err)
	}
	content := []byte("# 可重入漏洞实验报告\n\n## 结论\n\n报告记录了 withdraw 外部调用顺序、攻击路径和 checks-effects-interactions 修复结果。\n\n## 证据\n\n- 漏洞复现: 受控 Foundry 链上的重入计数为 2。\n- 修复验证: 余额更新发生在外部调用之前。\n")
	if err := objects.Put(ctx, parsed.Bucket, parsed.Key, bytes.NewReader(content), int64(len(content)), "text/markdown; charset=utf-8"); err != nil {
		return fmt.Errorf("写入验收实验报告对象失败: %w", err)
	}
	return nil
}

// acceptanceExperimentReportRef 返回实验报告使用的规范对象引用,数据库与对象存储共用这一值。
func acceptanceExperimentReportRef(reportBucket string) (string, error) {
	key, err := storage.ObjectKey(acceptanceIDs.TenantID, "experiment", "report", ids.Format(acceptanceIDs.ExperimentInstance), ids.Format(acceptanceIDs.StudentA), "acceptance-report.md")
	if err != nil {
		return "", fmt.Errorf("生成验收实验报告对象 key 失败: %w", err)
	}
	ref, err := storage.ObjectRefString(reportBucket, key)
	if err != nil {
		return "", fmt.Errorf("生成验收实验报告对象引用失败: %w", err)
	}
	return ref, nil
}

// seedAcceptanceTransferAsset 写入与成功 transfer_task 对应的真实 CSV 产物。
func seedAcceptanceTransferAsset(ctx context.Context, objects *storage.Storage, reportBucket string) error {
	ref, err := acceptanceTransferArtifactRef(reportBucket)
	if err != nil {
		return err
	}
	parsed, err := storage.ParseObjectRef(ref)
	if err != nil {
		return fmt.Errorf("解析验收转存对象引用失败: %w", err)
	}
	content := acceptanceTransferArtifact()
	if err := objects.Put(ctx, parsed.Bucket, parsed.Key, bytes.NewReader(content), int64(len(content)), "text/csv; charset=utf-8"); err != nil {
		return fmt.Errorf("写入验收转存对象失败: %w", err)
	}
	return nil
}

// acceptanceTransferArtifactRef 返回 transfer_task 与对象存储共用的统一引用。
func acceptanceTransferArtifactRef(reportBucket string) (string, error) {
	key, err := storage.ObjectKey(acceptanceIDs.TenantID, "transfer", "export", ids.Format(acceptanceIDs.TransferTask), "audit-log-acceptance.csv")
	if err != nil {
		return "", fmt.Errorf("生成验收转存对象 key 失败: %w", err)
	}
	ref, err := storage.ObjectRefString(reportBucket, key)
	if err != nil {
		return "", fmt.Errorf("生成验收转存对象引用失败: %w", err)
	}
	return ref, nil
}

// acceptanceTransferArtifact 返回与 seedAuditRows 中固定审计记录对应的 CSV 内容。
func acceptanceTransferArtifact() []byte {
	return []byte("id,tenant_id,actor_id,action,target_type,target_id,trace_id,created_at\n" +
		"910000000000099001,910000000000000001,910000000000000101,acceptance.seed.apply,identity.tenant,910000000000000001,trace-acceptance-seed-20260619,2026-06-19T12:00:00Z\n")
}

// seedAcceptanceTranscriptAsset 写入成绩单下载链路实际读取的受控 PDF。
func seedAcceptanceTranscriptAsset(ctx context.Context, objects *storage.Storage, reportBucket string) error {
	ref, err := acceptanceTranscriptRef(reportBucket)
	if err != nil {
		return err
	}
	parsed, err := storage.ParseObjectRef(ref)
	if err != nil {
		return fmt.Errorf("解析验收成绩单对象引用失败: %w", err)
	}
	content := acceptanceTranscriptPDF()
	if err := objects.Put(ctx, parsed.Bucket, parsed.Key, bytes.NewReader(content), int64(len(content)), "application/pdf"); err != nil {
		return fmt.Errorf("写入验收成绩单对象失败: %w", err)
	}
	return nil
}

// acceptanceTranscriptRef 返回成绩单服务使用的统一对象引用。
func acceptanceTranscriptRef(reportBucket string) (string, error) {
	key, err := storage.ObjectKey(acceptanceIDs.TenantID, "grade", "transcript", ids.Format(acceptanceIDs.StudentA), "acceptance-transcript.pdf")
	if err != nil {
		return "", fmt.Errorf("生成验收成绩单对象 key 失败: %w", err)
	}
	ref, err := storage.ObjectRefString(reportBucket, key)
	if err != nil {
		return "", fmt.Errorf("生成验收成绩单对象引用失败: %w", err)
	}
	return ref, nil
}

// acceptanceTranscriptPDF 构造一个可被 PDF 下载链路读取的确定性成绩单文件。
func acceptanceTranscriptPDF() []byte {
	content := "BT /F1 12 Tf 72 720 Td (Chaimir acceptance transcript) Tj ET\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for index, object := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xrefOffset := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)
	return out.Bytes()
}

// acceptanceExperimentInitScript 返回只依赖 workspace helper 的验收初始化脚本。
func acceptanceExperimentInitScript() []byte {
	return []byte("#!/bin/sh\nset -eu\ncd /workspace\ntest -f foundry.toml\ntest -f src/ReentrancyVault.sol\n")
}

// acceptanceExperimentArchive 构造可在隔离运行时离线编译的最小 Foundry 教学工程。
func acceptanceExperimentArchive() ([]byte, error) {
	files := []struct {
		name string
		body string
	}{
		{name: "foundry.toml", body: "[profile.default]\nsrc = \"src\"\nout = \"out\"\nlibs = []\nsolc = \"/usr/local/bin/solc-0.8.20\"\noffline = true\noptimizer = true\noptimizer_runs = 200\n"},
		{name: "src/ReentrancyVault.sol", body: `// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract ReentrancyVault {
    mapping(address => uint256) public balances;
    uint256 public reentryCount;

    function deposit() external payable {
        balances[msg.sender] += msg.value;
    }

    function seed() external payable {
        require(msg.value > 0, "seed required");
        balances[address(this)] += msg.value;
    }

    function attack() external payable {
        require(msg.value > 0, "funding required");
        balances[address(this)] += msg.value;
        this.withdraw();
    }

    function withdraw() public {
        uint256 amount = balances[msg.sender];
        require(amount > 0, "no balance");
        (bool sent,) = payable(msg.sender).call{value: amount}("");
        require(sent, "transfer failed");
        balances[msg.sender] = 0;
    }

    receive() external payable {
        reentryCount += 1;
        if (address(this).balance >= msg.value && reentryCount < 2) {
            this.withdraw();
        }
    }
}
`},
		{name: "src/ReentrancyProbe.sol", body: `// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface IReentrancyVault {
    function deposit() external payable;
    function withdraw() external;
}

contract ReentrancyProbe {
    IReentrancyVault private immutable vault;
    uint256 public reentryCount;

    constructor(address target) {
        vault = IReentrancyVault(target);
    }

    function attack() external payable {
        require(msg.value > 0, "funding required");
        vault.deposit{value: msg.value}();
        vault.withdraw();
    }

    receive() external payable {
        reentryCount += 1;
        if (reentryCount < 2) {
            (bool ok,) = address(vault).call(abi.encodeWithSignature("withdraw()"));
            require(ok, "reentry failed");
        }
    }
}
`},
		{name: "README.md", body: "# 可重入漏洞攻防实验\n\n审计 `src/ReentrancyVault.sol`,使用 `src/ReentrancyProbe.sol` 复现真实重入路径,再按检查-生效-交互顺序完成修复。归档中的 Foundry 工程由 EVM runtime 原生编译,初始化脚本只校验材料存在。\n"},
	}
	var out bytes.Buffer
	writer := tar.NewWriter(&out)
	for _, file := range files {
		header := &tar.Header{Name: file.name, Mode: 0o644, Size: int64(len(file.body))}
		if err := writer.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("写入验收实验归档头失败: %w", err)
		}
		if _, err := writer.Write([]byte(file.body)); err != nil {
			return nil, fmt.Errorf("写入验收实验归档内容失败: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭验收实验归档失败: %w", err)
	}
	return out.Bytes(), nil
}

// seedAcceptanceReplayObject 写入一份结构完整的验收回放归档,供浏览器验证授权与下载链路。
// 归档是历史事实夹具,不代表运行时镜像已可用,也不触发沙箱或判题执行。
func seedAcceptanceReplayObject(ctx context.Context, objects *storage.Storage, bucket string, refs acceptanceMaterialRefs) (string, string, error) {
	key, err := storage.ObjectKey(acceptanceIDs.TenantID, "contest", "replay", ids.Format(acceptanceIDs.BattleMatch), "acceptance-battle-replay.json")
	if err != nil {
		return "", "", err
	}
	ref, err := storage.ObjectRefString(bucket, key)
	if err != nil {
		return "", "", err
	}
	artifactAHash, err := acceptanceContestAssetHash(refs.BattleEntryARef, refs)
	if err != nil {
		return "", "", err
	}
	artifactBHash, err := acceptanceContestAssetHash(refs.BattleEntryBRef, refs)
	if err != nil {
		return "", "", err
	}
	archive := acceptanceBattleReplayArchive(artifactAHash, artifactBHash)
	raw, err := json.Marshal(archive)
	if err != nil {
		return "", "", err
	}
	if err := objects.Put(ctx, bucket, key, bytes.NewReader(raw), int64(len(raw)), "application/json"); err != nil {
		return "", "", err
	}
	return ref, key, nil
}

// acceptanceBattleReplayReceipt 保存受控验收链实际返回的公开回执字段,不写入私钥或答案材料。
func acceptanceBattleReplayReceipt(status, transactionHash, blockHash, blockNumber, gasUsed, from, to, contractAddress string) map[string]any {
	receipt := map[string]any{
		"type":            "0x2",
		"status":          status,
		"transactionHash": transactionHash,
		"blockHash":       blockHash,
		"blockNumber":     blockNumber,
		"gasUsed":         gasUsed,
		"from":            from,
		"to":              nil,
		"contractAddress": nil,
	}
	if strings.TrimSpace(to) != "" {
		receipt["to"] = to
	}
	if strings.TrimSpace(contractAddress) != "" {
		receipt["contractAddress"] = contractAddress
	}
	return receipt
}

// acceptanceBattleReplayActions 构造与 M8 正式归档协议一致的受控真实对局回放。
func acceptanceBattleReplayActions() []contracts.JudgeReplayAction {
	const from = "0xfdaf0ea969c98d0a8410d76bac4ac8838d13c9d4"
	defenseAddress := "0x48572b63121b25ca72f3dc1bde0e2a180e3870a1"
	attackAddress := "0xf968bb3408246cef11a43cdcf94573dbf20f9a69"
	return []contracts.JudgeReplayAction{
		{
			Seq: 1, Op: "deploy",
			Payload: map[string]any{"artifact": "VaultDefense", "role": "defense", "project_dir": "/workspace/battle/entry_b", "artifact_path": "out/VaultDefense.sol/VaultDefense.json"},
			Output: map[string]any{
				"contract_ref":     "defense",
				"contract_address": defenseAddress,
				"receipt":          acceptanceBattleReplayReceipt("0x1", "0x9a54da3570cef27e7eabbe7c8585e06cf7add3adcff19e299c5258fee149ed21", "0xc1447302b13f32ea728ac5739708d8e3ab87bc7a8edc0ea98c56a7383a4d46ac", "0x1", "0x4e465", from, "", defenseAddress),
			},
		},
		{
			Seq: 2, Op: "tx",
			Payload: map[string]any{"target_ref": "defense", "method": "deposit", "value": "1000000000000000000"},
			Output: map[string]any{
				"receipt": acceptanceBattleReplayReceipt("0x1", "0xae8120232e746f8053812bd82305bd00fd5a5ab1ce1853ff933f918e85ad59ec", "0xb62c3d956047da03f8e9801e6ba3c49f6d2df290b133207b606d90f515744791", "0x2", "0xaa52", from, defenseAddress, ""),
			},
		},
		{
			Seq: 3, Op: "deploy",
			Payload: map[string]any{"artifact": "ReentrancyAttack", "role": "attack", "target_ref": "defense", "project_dir": "/workspace/battle/entry_a", "artifact_path": "out/ReentrancyAttack.sol/ReentrancyAttack.json"},
			Output: map[string]any{
				"contract_ref":     "attack",
				"contract_address": attackAddress,
				"receipt":          acceptanceBattleReplayReceipt("0x1", "0xc533b0ebff3f30b40d70f3ef8a630a4509d12e9ae67bee3e3567a0ea4dcb4f92", "0x071555ab2eb6f68233514468e19d8809ebda8d96428b6d9fea03a3ae119d55ed", "0x3", "0x4d566", from, "", attackAddress),
			},
		},
		{
			Seq: 4, Op: "tx",
			Payload: map[string]any{"target_ref": "attack", "method": "attack", "value": "1000000000000000000"},
			Output: map[string]any{
				"receipt": acceptanceBattleReplayReceipt("0x1", "0x201233826ed86b297a46e3c0f896161d44f8ab974335eefd976b9b62946851fd", "0x36806f85d032e371f8d6949ec9e4dfffaeaf4623229d3e431af1e33720a9d98f", "0x4", "0x14d2c", from, attackAddress, ""),
			},
		},
		{
			Seq: 5, Op: "tx",
			Payload: map[string]any{"target_ref": "defense", "method": "withdraw"},
			Output: map[string]any{
				"receipt": acceptanceBattleReplayReceipt("0x1", "0x527718439c478573ec6337dccdaa7efeb6478526251652883815146aa06a646d", "0xe39b6c23569c7e2d9ddb351fdf5e13527ce7c334e7336afeb2ec21eb114f7a9f", "0x5", "0x703b", from, defenseAddress, ""),
			},
		},
	}
}

// acceptanceBattleReplayArchive 构造与 M8 正式归档协议一致的历史对局回放。

func acceptanceBattleReplayArchive(artifactAHash, artifactBHash string) map[string]any {
	return map[string]any{
		"version":    1,
		"match_id":   ids.Format(acceptanceIDs.BattleMatch),
		"task_id":    ids.Format(acceptanceIDs.BattleJudgeTask),
		"source_ref": "contest:2026:battle:acceptance-001",
		"initial_state": map[string]any{
			"contest_id":  ids.Format(acceptanceIDs.BattleContest),
			"problem_id":  ids.Format(acceptanceIDs.BattleContestProblem),
			"battle_rule": 1,
			"entry_a":     map[string]any{"role": 2, "version_no": 1, "artifact_hash": artifactAHash},
			"entry_b":     map[string]any{"role": 1, "version_no": 1, "artifact_hash": artifactBHash},
		},
		"actions": acceptanceBattleReplayActions(),
		"result": map[string]any{
			"passed":    true,
			"score":     1,
			"max_score": 1,
			"details": []map[string]any{{
				"case":           "防御合约阻止重复提款",
				"passed":         true,
				"expected_label": "攻击仅触发一次回调,守方仍可正常提款",
				"actual":         "defense_win",
				"hint":           "攻击交易已完成但未发生第二次提款,守方获胜",
			}},
		},
		"finished_at": timex.RFC3339OrEmpty(timex.Now()),
	}
}

// ensureAcceptanceSeedAllowed 防止验收夹具被误写入生产库。
func ensureAcceptanceSeedAllowed(cfg *config.Config) error {
	if !config.IsAcceptanceEnvironment(cfg.Server.AppEnv) {
		return fmt.Errorf("seed-acceptance 仅允许 APP_ENV=test,当前 APP_ENV=%s", cfg.Server.AppEnv)
	}
	if err := identity.ValidatePassword(cfg.Bootstrap.AdminPassword); err != nil {
		return fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD 必须配置为符合验收账号密码强度的初始密码: %w", err)
	}
	return nil
}

// seedAcceptanceTenant 创建验收租户,不依赖生产 bootstrap 租户;隔离租户仅用于 SaaS 多租户验收。
//
// 刻意不写 logo_ref:校徽与课程封面存的是对象引用,而种子只写库不往 ObjectStorage 放字节。
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
func seedAcceptanceBusiness(ctx context.Context, database *db.DB, includeIsolation bool, replayRef, reportBucket string, refs acceptanceMaterialRefs) error {
	if includeIsolation {
		if err := database.WithPrivilegedTx(ctx, seedApplicationRows); err != nil {
			return err
		}
	}
	if err := database.WithTenantTxID(ctx, acceptanceIDs.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		for _, fn := range []func(context.Context, pgx.Tx) error{
			func(ctx context.Context, tx pgx.Tx) error { return seedContentRows(ctx, tx, refs) },
			func(ctx context.Context, tx pgx.Tx) error { return seedAcceptanceBusinessRows(ctx, tx, refs) },
			func(ctx context.Context, tx pgx.Tx) error { return seedTeachingRows(ctx, tx, refs) },
			func(ctx context.Context, tx pgx.Tx) error { return seedExperimentRows(ctx, tx, refs, reportBucket) },
			seedSimRows,
			func(ctx context.Context, tx pgx.Tx) error { return seedContestRows(ctx, tx, refs) },
			seedNotifyRows,
			func(ctx context.Context, tx pgx.Tx) error { return seedGradeRows(ctx, tx, reportBucket) },
			seedAdminRows,
			func(ctx context.Context, tx pgx.Tx) error { return seedTransferRows(ctx, tx, reportBucket) },
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

// seedAcceptanceBusinessRows 写入依赖平台目录的租户业务夹具,不创建或修改平台能力定义。
func seedAcceptanceBusinessRows(ctx context.Context, tx pgx.Tx, refs acceptanceMaterialRefs) error {
	if err := seedTenantQuotaRow(ctx, tx); err != nil {
		return err
	}
	if err := seedSandboxRows(ctx, tx, refs); err != nil {
		return err
	}
	return seedJudgeRows(ctx, tx, refs)
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
