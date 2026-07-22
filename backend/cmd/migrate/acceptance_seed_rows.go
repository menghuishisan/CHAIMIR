// acceptance_seed_rows 按模块写入验收测试所需的业务夹具数据。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/crypto"

	"github.com/jackc/pgx/v5"
	"sigs.k8s.io/yaml"
)

type acceptanceImageAttestation struct {
	ImageURL       string `json:"image_url"`
	Digest         string `json:"digest"`
	CosignVerified bool   `json:"cosign_verified"`
	TrivyStatus    string `json:"trivy_status"`
}

const (
	acceptanceInitCodeRef   = "minio://chaimir-code/910000000000000001/sandbox/init/lab-reentrancy-foundry/workspace.tar"
	acceptanceInitScriptRef = "minio://chaimir-code/910000000000000001/sandbox/init/lab-reentrancy-foundry/init.sh"
)

// acceptanceImageURL 从受控镜像证明清单选择不可变 digest 地址,保证验收种子和沙箱安全规则使用同一来源。
func acceptanceImageURL(image string) (string, error) {
	registry := strings.TrimRight(osEnv("IMAGE_REGISTRY"), "/")
	if registry == "" {
		registry = "harbor.chaimir"
	}
	prefix := registry + "/" + strings.TrimLeft(image, "/") + "@sha256:"
	raw := strings.TrimSpace(osEnv("SANDBOX_IMAGE_ATTESTATIONS_JSON"))
	if raw == "" {
		return "", fmt.Errorf("SANDBOX_IMAGE_ATTESTATIONS_JSON 缺少 %s 的镜像证明", image)
	}
	var items []acceptanceImageAttestation
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return "", fmt.Errorf("SANDBOX_IMAGE_ATTESTATIONS_JSON 解析失败: %w", err)
	}
	for _, item := range items {
		imageURL := strings.TrimSpace(item.ImageURL)
		digest := acceptanceImageDigest(imageURL)
		if strings.HasPrefix(imageURL, prefix) &&
			digest != "" &&
			digest == strings.TrimSpace(item.Digest) &&
			item.CosignVerified &&
			strings.EqualFold(strings.TrimSpace(item.TrivyStatus), "passed") {
			return imageURL, nil
		}
	}
	return "", fmt.Errorf("SANDBOX_IMAGE_ATTESTATIONS_JSON 未包含通过校验的 %s digest 镜像证明", image)
}

// acceptanceImageDigest 提取 image@sha256:... 中的不可变 digest。
func acceptanceImageDigest(imageURL string) string {
	parts := strings.Split(strings.TrimSpace(imageURL), "@")
	if len(parts) != 2 || !strings.HasPrefix(parts[1], "sha256:") {
		return ""
	}
	return parts[1]
}

// seedRuntimeRows 写入沙箱运行时、镜像、工具和判题器基础能力。
func seedRuntimeRows(ctx context.Context, tx pgx.Tx) error {
	runtimeImageURL, err := acceptanceImageURL("runtime/evm-foundry")
	if err != nil {
		return err
	}
	runtimeSpec := acceptanceRuntimeAdapterSpec(runtimeImageURL)
	if err := execJSON(ctx, tx, `
INSERT INTO runtime (id, code, name, eco, adapter_level, adapter_spec, capability_impl, selftest_status, selftest_detail, status)
VALUES ($1,'evm-foundry','EVM Foundry 教学运行时','evm',2,$2,'sandbox-exec',2,'{"checked_by":"acceptance-seed"}'::jsonb,1)
ON CONFLICT (id) DO UPDATE SET code=EXCLUDED.code, name=EXCLUDED.name, eco=EXCLUDED.eco, adapter_level=EXCLUDED.adapter_level, adapter_spec=EXCLUDED.adapter_spec, capability_impl=EXCLUDED.capability_impl, selftest_status=EXCLUDED.selftest_status, selftest_detail=EXCLUDED.selftest_detail, status=EXCLUDED.status, updated_at=now()`,
		acceptanceIDs.Runtime, runtimeSpec); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO runtime_image (id, runtime_id, image_url, version, status, prepulled, prepull_status, prepull_detail, prepulled_at, genesis_baked, is_default)
VALUES ($1,$2,$3,'2026.06',1,false,1,'{"source":"acceptance-seed","prepulled":false}'::jsonb,NULL,true,true)
ON CONFLICT (runtime_id, version) DO UPDATE SET image_url=EXCLUDED.image_url, status=EXCLUDED.status, prepulled=EXCLUDED.prepulled, prepull_status=EXCLUDED.prepull_status, prepull_detail=EXCLUDED.prepull_detail, prepulled_at=EXCLUDED.prepulled_at, genesis_baked=EXCLUDED.genesis_baked, is_default=EXCLUDED.is_default`,
		acceptanceIDs.RuntimeImage, acceptanceIDs.Runtime, runtimeImageURL); err != nil {
		return err
	}
	if err := seedToolRows(ctx, tx); err != nil {
		return err
	}
	judgerImageURL, err := acceptanceImageURL("judger/testcase-evm")
	if err != nil {
		return err
	}
	judgeSpec := map[string]any{
		"runtime_code":          "evm-foundry",
		"runtime_image_version": "2026.06",
		"genesis_ref":           "genesis/evm-foundry/acceptance.json",
		"tool_codes":            []string{"code-server"},
		"command":               []string{"run-evm-tests"},
		"exec_target":           "sandbox/testcase-evm",
		"execution_sidecars":    []workload.ComponentSpec{acceptanceEVMJudgerSidecar(judgerImageURL)},
		"timeout_sec":           60,
		"max_retries":           1,
		"suite_archive_name":    "public-regression.tar.gz",
		"selftest":              map[string]any{"case": "public-regression"},
	}
	if err := execJSON(ctx, tx, `
INSERT INTO judger (id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, status)
VALUES ($1,'solidity-unit','Solidity 单元测试判题器',1,$3,true,60,$2,2,1)
ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name, type=EXCLUDED.type, executor_ref=EXCLUDED.executor_ref, runtime_required=EXCLUDED.runtime_required, default_timeout_sec=EXCLUDED.default_timeout_sec, resource_spec=EXCLUDED.resource_spec, selftest_status=EXCLUDED.selftest_status, status=EXCLUDED.status, updated_at=now()`,
		acceptanceIDs.Judger, judgeSpec, judgerImageURL); err != nil {
		return err
	}
	onchainSpec := map[string]any{
		"runtime_code":          "evm-foundry",
		"runtime_image_version": "2026.06",
		"genesis_ref":           "genesis/evm-foundry/acceptance.json",
		"tool_codes":            []string{"code-server"},
		"timeout_sec":           60,
		"max_retries":           1,
		"selftest": map[string]any{
			"tenant_id":    acceptanceIDs.TenantID,
			"submitter_id": acceptanceIDs.TeacherMain,
			"source_ref":   "judge:2026:selftest:onchain-assert",
			"max_score":    100,
			"expectation": map[string]any{
				"assertions": []map[string]any{
					{"label": "判题器自检链 ID", "target": "chainId", "field": "chain_id", "op": "eq", "value": 31337, "expected_label": "链 ID 应为验收本地链"},
				},
			},
		},
	}
	if err := execJSON(ctx, tx, `
INSERT INTO judger (id, code, name, type, executor_ref, runtime_required, default_timeout_sec, resource_spec, selftest_status, status)
VALUES ($1,'onchain-assert','链上状态断言判题器',2,'m3-backend-strategy',true,60,$2,2,1)
ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name, type=EXCLUDED.type, executor_ref=EXCLUDED.executor_ref, runtime_required=EXCLUDED.runtime_required, default_timeout_sec=EXCLUDED.default_timeout_sec, resource_spec=EXCLUDED.resource_spec, selftest_status=EXCLUDED.selftest_status, status=EXCLUDED.status, updated_at=now()`,
		acceptanceIDs.JudgerOnchain, onchainSpec); err != nil {
		return err
	}
	if err := seedTenantQuotaRow(ctx, tx); err != nil {
		return err
	}
	if err := seedSandboxRows(ctx, tx); err != nil {
		return err
	}
	return seedJudgeRows(ctx, tx)
}

// acceptanceEVMJudgerSidecar 生成 EVM testcase 判题器私有执行容器声明。
func acceptanceEVMJudgerSidecar(imageURL string) workload.ComponentSpec {
	readOnlyRoot := true
	mountWorkspace := true
	return workload.ComponentSpec{
		Name:     "testcase-evm",
		ImageURL: imageURL,
		Command:  []string{"sleep", "2147483647"},
		Env: []workload.EnvVarSpec{
			{Name: "CHAIMIR_SUBMISSION_DIR", Value: "/judge-private"},
		},
		Resources: workload.ResourceSpec{
			Requests: map[string]string{"cpu": "500m", "memory": "1Gi"},
			Limits:   map[string]string{"cpu": "2", "memory": "4Gi"},
		},
		Workdir:                "/workspace",
		ReadOnlyRootFilesystem: &readOnlyRoot,
		Labels:                 map[string]string{"chaimir.io/student-access": "false", "chaimir.io/sensitivity": "judge-private"},
		MountWorkspace:         &mountWorkspace,
		EphemeralMounts: []workload.EphemeralMountSpec{
			{Name: "judge-workdir", MountPath: "/judge"},
			{Name: "judge-tmp", MountPath: "/tmp"},
		},
	}
}

// acceptanceRuntimeAdapterSpec 构造验收运行时声明,只放运行时必需组件;工具私有依赖必须留在工具 WorkloadSpec 内。
func acceptanceRuntimeAdapterSpec(runtimeImageURL string) map[string]any {
	return map[string]any{
		"workspace_dir": "/workspace",
		"volume_domains": []map[string]any{
			{"name": "workspace", "mount_path": "/workspace", "student_access": "read_write", "persistence": "minio_code", "snapshot_scope": "always"},
			{"name": "runtime-state", "mount_path": "/runtime-state", "student_access": "none", "persistence": "ephemeral", "snapshot_scope": "snapshot_enabled"},
			{"name": "judge-private", "mount_path": "/judge-private", "student_access": "none", "persistence": "ephemeral", "snapshot_scope": "never"},
			{"name": "runtime-tmp", "mount_path": "/tmp", "student_access": "none", "persistence": "ephemeral", "snapshot_scope": "never"},
		},
		"runtime_container": map[string]any{
			"name": "foundry",
			"ports": []map[string]any{
				{"name": "rpc", "container_port": 8545, "service_port": 8545, "protocol": "TCP"},
			},
			"resources": map[string]any{
				"requests": map[string]string{"cpu": "250m", "memory": "512Mi"},
				"limits":   map[string]string{"cpu": "2", "memory": "2Gi"},
			},
			"readiness_probe": map[string]any{"type": "tcp", "port": "rpc", "period_seconds": 2, "failure_threshold": 30},
			"labels":          map[string]string{"chaimir.io/student-access": "false"},
		},
		"infra_sidecars": []map[string]any{
			{
				"name":      "student-shell",
				"image_url": runtimeImageURL,
				"command":   []string{"sleep", "2147483647"},
				"resources": map[string]any{
					"requests": map[string]string{"cpu": "50m", "memory": "64Mi"},
					"limits":   map[string]string{"cpu": "250m", "memory": "256Mi"},
				},
				"read_only_root_filesystem": true,
				"labels":                    map[string]string{"chaimir.io/student-access": "true"},
				"prepull_command":           []string{"sleep", "2147483647"},
				"prepull_hold":              true,
			},
		},
		"default_tool_codes": []string{"code-server", "terminal"},
		"workspace_ops": map[string]any{
			"read_file":  []string{"/usr/local/bin/chaimir-workspace", "read", "{{workspace}}", "{{path}}"},
			"write_file": []string{"/usr/local/bin/chaimir-workspace", "write", "{{workspace}}", "{{path}}"},
			"list_files": []string{"/usr/local/bin/chaimir-workspace", "list", "{{workspace}}", "{{path}}"},
			"pack_tar":   []string{"/usr/local/bin/chaimir-workspace", "pack", "{{workspace}}", "{{path}}"},
			"unpack_tar": []string{"/usr/local/bin/chaimir-workspace", "unpack", "{{workspace}}", "{{path}}"},
			"run_script": []string{"/usr/local/bin/chaimir-workspace", "run", "{{workspace}}", "{{workspace}}", "{{script}}"},
			"terminal":   []string{"/usr/local/bin/chaimir-workspace", "terminal", "{{workspace}}"},
			"selftest":   []string{"/usr/local/bin/chaimir-workspace", "selftest"},
		},
		"capability_commands": map[string]any{
			"deploy": map[string]any{"command": []string{"/usr/local/bin/chaimir-chain", "deploy"}, "timeout_seconds": 60},
			"tx":     map[string]any{"command": []string{"/usr/local/bin/chaimir-chain", "tx"}, "timeout_seconds": 60},
			"query":  map[string]any{"command": []string{"/usr/local/bin/chaimir-chain", "query"}, "timeout_seconds": 30},
			"reset":  map[string]any{"command": []string{"/usr/local/bin/chaimir-chain", "reset"}, "timeout_seconds": 30},
		},
		"selftest": map[string]any{
			"deploy_payload": map[string]any{"bytecode": "0x6080604052348015600f57600080fd5b50600080f3"},
			"query_target":   "chainId",
		},
	}
}

type acceptanceImageUnitManifest struct {
	SchemaVersion      int                   `json:"schema_version"`
	Category           string                `json:"category"`
	Name               string                `json:"name"`
	Image              string                `json:"image"`
	Description        string                `json:"description"`
	Source             map[string]any        `json:"source"`
	Upstream           map[string]any        `json:"upstream"`
	DataDriven         bool                  `json:"data_driven"`
	Infra              map[string]any        `json:"infra"`
	Ports              []toolManifestPort    `json:"ports"`
	LocalDev           map[string]any        `json:"local_dev"`
	Security           toolManifestSecurity  `json:"security"`
	SecurityExceptions []map[string]any      `json:"security_exceptions"`
	StudentAccess      map[string]any        `json:"student_access"`
	Resources          toolManifestResources `json:"resources"`
	Build              map[string]any        `json:"build"`
	Selftest           map[string]any        `json:"selftest"`
	SupplyChain        map[string]any        `json:"supply_chain"`
	Labels             map[string]string     `json:"labels"`
	Capabilities       []string              `json:"capabilities"`
}

// acceptanceImageUnitManifestFor 严格读取指定镜像单元 manifest,并校验目录分类与镜像名前缀一致。
func acceptanceImageUnitManifestFor(image, category string) (acceptanceImageUnitManifest, error) {
	root, err := acceptanceImagesRoot()
	if err != nil {
		return acceptanceImageUnitManifest{}, err
	}
	path := filepath.Join(root, filepath.FromSlash(image), "manifest.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return acceptanceImageUnitManifest{}, fmt.Errorf("读取镜像 manifest 失败 %s: %w", image, err)
	}
	var manifest acceptanceImageUnitManifest
	if err := yaml.UnmarshalStrict(raw, &manifest); err != nil {
		return acceptanceImageUnitManifest{}, fmt.Errorf("解析镜像 manifest 失败 %s: %w", image, err)
	}
	if manifest.Category != category || manifest.Image != image || strings.TrimSpace(manifest.Name) == "" {
		return acceptanceImageUnitManifest{}, fmt.Errorf("镜像 manifest 分类或镜像名不一致: %s", image)
	}
	return manifest, nil
}

// acceptanceManifestSelftestCommand 选择镜像 manifest 首个自检命令作为预拉取自检命令。
func acceptanceManifestSelftestCommand(manifest acceptanceImageUnitManifest) ([]string, error) {
	raw, ok := manifest.Selftest["commands"]
	if !ok {
		return nil, fmt.Errorf("%s 缺少 selftest.commands", manifest.Image)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("编码 %s selftest.commands 失败: %w", manifest.Image, err)
	}
	var commands []toolManifestSelftestCommand
	if err := json.Unmarshal(data, &commands); err != nil || len(commands) == 0 {
		return nil, fmt.Errorf("解析 %s selftest.commands 失败: %w", manifest.Image, err)
	}
	command := acceptanceCompactCommand(commands[0].Command)
	if len(command) == 0 {
		return nil, fmt.Errorf("%s selftest.commands 为空", manifest.Image)
	}
	return command, nil
}

// acceptanceCompactCommand 清理 manifest 命令数组中的空白参数,保持声明顺序。
func acceptanceCompactCommand(command []string) []string {
	out := make([]string, 0, len(command))
	for _, part := range command {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// seedToolRows 从 images/tool manifest 重建工具表,避免 seed 内保留工具专用分支。
func seedToolRows(ctx context.Context, tx pgx.Tx) error {
	defs, err := acceptanceSeedToolDefinitions()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sandbox_tool`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM tool`); err != nil {
		return err
	}
	for _, def := range defs {
		if err := execJSON(ctx, tx, `
INSERT INTO tool (id, code, name, kind, eco_tags, resource_spec, status)
VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			def.ID, def.Code, def.Name, def.Kind, strings.Join(def.EcoTags, ","), def.ResourceSpec, def.Status); err != nil {
			return err
		}
	}
	return nil
}

// seedTenantQuotaRow 写入租户沙箱配额,确保沙箱统计和创建流程使用真实配额表。
func seedTenantQuotaRow(ctx context.Context, tx pgx.Tx) error {
	return execJSON(ctx, tx, `
INSERT INTO tenant_quota (
	tenant_id, max_concurrent_sandbox, max_cpu, max_memory_mb,
	idle_timeout_min, max_lifetime_min, max_keepalive_min, max_snapshot_retention_min
) VALUES (
	$1,30,64,131072,45,240,120,10080
)
ON CONFLICT (tenant_id) DO UPDATE SET
	max_concurrent_sandbox=EXCLUDED.max_concurrent_sandbox,
	max_cpu=EXCLUDED.max_cpu,
	max_memory_mb=EXCLUDED.max_memory_mb,
	idle_timeout_min=EXCLUDED.idle_timeout_min,
	max_lifetime_min=EXCLUDED.max_lifetime_min,
	max_keepalive_min=EXCLUDED.max_keepalive_min,
	max_snapshot_retention_min=EXCLUDED.max_snapshot_retention_min,
	updated_at=now()`,
		acceptanceIDs.TenantID)
}

// seedSandboxRows 写入历史沙箱和工具行,用于沙箱详情、鉴权和历史记录查询。
func seedSandboxRows(ctx context.Context, tx pgx.Tx) error {
	if err := execJSON(ctx, tx, `
INSERT INTO sandbox (
	id, tenant_id, runtime_id, image_id, namespace, source_ref, owner_account_id, phase, status,
	keep_alive, snapshot_enabled, code_storage_key, code_hash, init_code_ref, init_script_ref, snapshot_ref, snapshot_domains,
	snapshot_created_at, snapshot_expire_at, keep_alive_until, expire_at
) VALUES (
	$1,$2,$3,$4,'chaimir-acceptance-sandbox-a','sandbox:acceptance:reentrancy-a',$5,$6,$7,
	false,true,'910000000000000001/sandbox/code/910000000000001021/workspace.tar','6d0f2d2a4f7a7b7b6b0e0e9f7c8a1c2d3e4f506172839405162738495a6b7c8d',$8,$9,'snapshots/acceptance/reentrancy-a.tar','["workspace"]'::jsonb,
	now(),now() + interval '7 days',NULL,now() + interval '2 hours'
)
ON CONFLICT (id) DO UPDATE SET runtime_id=EXCLUDED.runtime_id, image_id=EXCLUDED.image_id, namespace=EXCLUDED.namespace, source_ref=EXCLUDED.source_ref, owner_account_id=EXCLUDED.owner_account_id, phase=EXCLUDED.phase, status=EXCLUDED.status, keep_alive=EXCLUDED.keep_alive, snapshot_enabled=EXCLUDED.snapshot_enabled, code_storage_key=EXCLUDED.code_storage_key, code_hash=EXCLUDED.code_hash, init_code_ref=EXCLUDED.init_code_ref, init_script_ref=EXCLUDED.init_script_ref, snapshot_ref=EXCLUDED.snapshot_ref, snapshot_domains=EXCLUDED.snapshot_domains, snapshot_created_at=EXCLUDED.snapshot_created_at, snapshot_expire_at=EXCLUDED.snapshot_expire_at, keep_alive_until=EXCLUDED.keep_alive_until, expire_at=EXCLUDED.expire_at, updated_at=now()`,
		acceptanceIDs.Sandbox, acceptanceIDs.TenantID, acceptanceIDs.Runtime, acceptanceIDs.RuntimeImage, acceptanceIDs.StudentA, contracts.SandboxPhaseFullyReady, contracts.SandboxStatusDestroyed, acceptanceInitCodeRef, acceptanceInitScriptRef); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO sandbox_tool (id, tenant_id, sandbox_id, tool_id, access_endpoint, status)
VALUES ($1,$2,$3,(SELECT id FROM tool WHERE code='code-server'),'/api/v1/sandbox/sandboxes/910000000000001021/tools/code-server/',1)
ON CONFLICT (tenant_id, sandbox_id, tool_id) DO UPDATE SET access_endpoint=EXCLUDED.access_endpoint, status=EXCLUDED.status`,
		acceptanceIDs.SandboxTool, acceptanceIDs.TenantID, acceptanceIDs.Sandbox); err != nil {
		return err
	}
	return execJSON(ctx, tx, `
INSERT INTO sandbox_event (id, tenant_id, sandbox_id, event_type, detail)
VALUES ($1,$2,$3,'create','{"seed":"acceptance","status":"ready"}'::jsonb)
ON CONFLICT (id) DO UPDATE SET event_type=EXCLUDED.event_type, detail=EXCLUDED.detail`,
		acceptanceIDs.SandboxEvent, acceptanceIDs.TenantID, acceptanceIDs.Sandbox)
}

// seedJudgeRows 写入一个已完成判题任务和脱敏结果,用于判题详情和重判测试。
func seedJudgeRows(ctx context.Context, tx pgx.Tx) error {
	judgerImageURL, err := acceptanceImageURL("judger/testcase-evm")
	if err != nil {
		return err
	}
	snapshot := map[string]any{
		"item_code":                   "ctf-reentrancy-vault",
		"item_version":                "1.0.0",
		"trace_id":                    "trace-acceptance-judge",
		"judger_code":                 "solidity-unit",
		"judger_type":                 1,
		"judger_version":              judgerImageURL,
		"suite_ref":                   "minio://chaimir-code/910000000000000001/judge/suites/ctf-reentrancy-vault/public-regression.tar.gz",
		"suite_archive_name":          "public-regression.tar.gz",
		"version_hash":                "acceptance-version-hash",
		"runtime_code":                "evm-foundry",
		"runtime_image_version":       "2026.06",
		"genesis_ref":                 "genesis/evm-foundry/acceptance.json",
		"tool_codes":                  []string{"code-server"},
		"command":                     []string{"run-evm-tests"},
		"exec_target":                 "sandbox/testcase-evm",
		"execution_sidecars":          []workload.ComponentSpec{acceptanceEVMJudgerSidecar(judgerImageURL)},
		"timeout_sec":                 60,
		"max_retries":                 1,
		"max_score":                   100,
		"expectation":                 map[string]any{"public": true},
		"sanitized_code_archive_name": "submission.zip",
		"sanitized_code_archive_ref":  "minio://chaimir-code/acceptance/submissions/S20260001/reentrancy-fixed.zip",
	}
	details := []map[string]any{{"case": "public-visible-tests", "passed": true, "actual": "全部公开断言通过"}}
	if err := execJSON(ctx, tx, `
INSERT INTO judge_task (
	id, tenant_id, judger_id, source_ref, source_owner_id, source_course_id, source_scope,
	submitter_id, problem_ref, code_storage_key, code_hash, input_snapshot, sandbox_mode,
	target_sandbox_ref, priority, status, retry_count, max_retries
) VALUES (
	$1,$2,$3,'teaching:2026:submission-item:910000000000005041-910000000000005032',$4,$5,'teaching',$6,'ctf-reentrancy-vault:1.0.0',
	'minio://chaimir-code/acceptance/submissions/S20260001/reentrancy-fixed.zip','6d0f2d2a4f7a7b7b6b0e0e9f7c8a1c2d3e4f506172839405162738495a6b7c8d',
	$7,2,'sandbox:acceptance:reentrancy-a',5,$8,0,1
)
ON CONFLICT (tenant_id, source_ref, problem_ref) DO UPDATE SET judger_id=EXCLUDED.judger_id, source_owner_id=EXCLUDED.source_owner_id, source_course_id=EXCLUDED.source_course_id, source_scope=EXCLUDED.source_scope, submitter_id=EXCLUDED.submitter_id, code_storage_key=EXCLUDED.code_storage_key, code_hash=EXCLUDED.code_hash, input_snapshot=EXCLUDED.input_snapshot, sandbox_mode=EXCLUDED.sandbox_mode, target_sandbox_ref=EXCLUDED.target_sandbox_ref, priority=EXCLUDED.priority, status=EXCLUDED.status, retry_count=EXCLUDED.retry_count, max_retries=EXCLUDED.max_retries, updated_at=now()`,
		acceptanceIDs.JudgeTask, acceptanceIDs.TenantID, acceptanceIDs.Judger, acceptanceIDs.TeacherMain, acceptanceIDs.Course, acceptanceIDs.StudentA, snapshot, contracts.JudgeTaskStatusDone); err != nil {
		return err
	}
	return execJSON(ctx, tx, `
INSERT INTO judge_result (id, task_id, tenant_id, version, passed, score, max_score, details, judge_sandbox_ref, judged_at, is_rejudge)
VALUES ($1,$2,$3,1,true,92,100,$4,'sandbox:acceptance:reentrancy-a',now(),false)
ON CONFLICT (tenant_id, task_id, version) DO UPDATE SET passed=EXCLUDED.passed, score=EXCLUDED.score, max_score=EXCLUDED.max_score, details=EXCLUDED.details, judge_sandbox_ref=EXCLUDED.judge_sandbox_ref, judged_at=EXCLUDED.judged_at, is_rejudge=EXCLUDED.is_rejudge`,
		acceptanceIDs.JudgeResult, acceptanceIDs.JudgeTask, acceptanceIDs.TenantID, details)
}

// seedContentRows 写入内容库、题库和试卷数据。
func seedContentRows(ctx context.Context, tx pgx.Tx) error {
	bodyLab := map[string]any{
		"runtime_code": "evm-foundry", "tools": []string{"code-server"}, "init_code_ref": "", "sim_package_ref": "",
		"description": "使用 Foundry 复现可重入漏洞：审计 withdraw 调用顺序，编写攻击合约，并应用 checks-effects-interactions 完成修复。", "init_script": "",
		"judge_config": map[string]any{"judger_code": "solidity-unit", "suite_ref": "minio://chaimir-code/910000000000000001/judge/suites/ctf-reentrancy-vault/public-regression.tar.gz", "max_score": 100, "expectation": map[string]any{"public": true}},
	}
	bodyContest := map[string]any{
		"statement":      "给定简化金库合约，提交能够触发资金重复提取的最小攻击代码。链上余额断言通过后计分。",
		"init_contracts": []string{},
		"judge_config": map[string]any{
			"judger_code": "solidity-unit",
			"suite_ref":   "minio://chaimir-code/910000000000000001/judge/suites/ctf-reentrancy-vault/public-regression.tar.gz",
			"max_score":   100,
			"expectation": map[string]any{"public": true},
		},
	}
	bodyBattle := map[string]any{
		"statement":      "攻方提交攻击归档，守方提交防御归档；系统在隔离对局沙箱恢复双方参战物并用链上断言判定是否攻破。",
		"init_contracts": []string{},
		"ad_config":      map[string]any{"runtime_code": "evm-foundry", "runtime_image_version": "2026.06", "tool_codes": []string{"code-server"}},
		"judge_config": map[string]any{
			"judger_code": "onchain-assert",
			"max_score":   100,
			"expectation": map[string]any{
				"assertions": []map[string]any{
					{"label": "本地链已进入可判定状态", "target": "chainId", "field": "chain_id", "op": "eq", "value": 31337, "expected_label": "链 ID 应为验收本地链"},
				},
			},
		},
	}
	bodyTheory := map[string]any{
		"statement": "拜占庭容错共识中的安全性主要指什么？", "q_type": "single_choice",
		"options": []string{"只提高出块速度", "在部分节点作恶时仍保持一致性", "取消交易签名", "跳过网络传播"},
		"answer":  "在部分节点作恶时仍保持一致性", "explanation": "安全性要求系统即使面对规定范围内的恶意节点，也不会确认相互冲突的状态。",
	}
	if err := execJSON(ctx, tx, `
INSERT INTO content_category (id, tenant_id, parent_id, name, sort)
VALUES ($1,$2,NULL,'智能合约安全',10)
ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, sort=EXCLUDED.sort, deleted_at=NULL, updated_at=now()`,
		acceptanceIDs.ContentCat, acceptanceIDs.TenantID); err != nil {
		return err
	}
	if err := upsertContentItem(ctx, tx, acceptanceIDs.ContentLab, "lab-reentrancy-foundry", "1.0.0", 1, "Foundry 可重入漏洞复现实验", 2, bodyLab, []string{}); err != nil {
		return err
	}
	if err := upsertContentItem(ctx, tx, acceptanceIDs.ContentContest, "ctf-reentrancy-vault", "1.0.0", 2, "Reentrancy Vault 攻击题", 3, bodyContest, []string{"judge_config"}); err != nil {
		return err
	}
	if err := upsertContentItem(ctx, tx, acceptanceIDs.ContentBattle, "battle-reentrancy-duel", "1.0.0", 2, "Reentrancy Vault 攻防对局题", 3, bodyBattle, []string{"judge_config"}); err != nil {
		return err
	}
	if err := upsertContentItem(ctx, tx, acceptanceIDs.ContentTheory, "quiz-bft-safety-liveness", "1.0.0", 3, "BFT 安全性与活性理解题", 2, bodyTheory, []string{}); err != nil {
		return err
	}
	criteria := map[string]any{"source": "manual", "coverage": []string{"solidity", "bft"}}
	if err := execJSON(ctx, tx, `
INSERT INTO paper (id, tenant_id, name, author_id, gen_mode, gen_criteria)
VALUES ($1,$2,'区块链系统安全阶段测验',$3,1,$4)
ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, author_id=EXCLUDED.author_id, gen_mode=EXCLUDED.gen_mode, gen_criteria=EXCLUDED.gen_criteria, deleted_at=NULL, updated_at=now()`,
		acceptanceIDs.Paper, acceptanceIDs.TenantID, acceptanceIDs.TeacherMain, criteria); err != nil {
		return err
	}
	for _, item := range []struct {
		id      int64
		code    string
		version string
		score   int
		seq     int
	}{
		{acceptanceIDs.Paper + 1, "ctf-reentrancy-vault", "1.0.0", 60, 1},
		{acceptanceIDs.Paper + 2, "quiz-bft-safety-liveness", "1.0.0", 40, 2},
	} {
		if err := execJSON(ctx, tx, `
INSERT INTO paper_item (id, tenant_id, paper_id, item_code, item_version, score, seq)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (tenant_id, paper_id, seq) DO UPDATE SET item_code=EXCLUDED.item_code, item_version=EXCLUDED.item_version, score=EXCLUDED.score`,
			item.id, acceptanceIDs.TenantID, acceptanceIDs.Paper, item.code, item.version, item.score, item.seq); err != nil {
			return err
		}
	}
	return nil
}

// upsertContentItem 幂等写入内容条目及正文。
func upsertContentItem(ctx context.Context, tx pgx.Tx, id int64, code, version string, itemType int16, title string, difficulty int16, body any, sensitive []string) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	versionHash := crypto.SHA256Hex(raw)
	if err := execJSON(ctx, tx, `
INSERT INTO content_item (id, tenant_id, code, version, type, title, category_id, difficulty, tags, knowledge_points, author_id, author_type, visibility, status, usage_count, version_hash)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,ARRAY['solidity','security'],ARRAY['reentrancy','smart-contract'],$9,1,2,2,1,$10)
ON CONFLICT (id) DO UPDATE SET code=EXCLUDED.code, version=EXCLUDED.version, type=EXCLUDED.type, title=EXCLUDED.title, category_id=EXCLUDED.category_id, difficulty=EXCLUDED.difficulty, tags=EXCLUDED.tags, knowledge_points=EXCLUDED.knowledge_points, author_id=EXCLUDED.author_id, author_type=EXCLUDED.author_type, visibility=EXCLUDED.visibility, status=EXCLUDED.status, usage_count=EXCLUDED.usage_count, version_hash=EXCLUDED.version_hash, deleted_at=NULL, updated_at=now()`,
		id, acceptanceIDs.TenantID, code, version, itemType, title, acceptanceIDs.ContentCat, difficulty, acceptanceIDs.TeacherMain, versionHash); err != nil {
		return err
	}
	return execJSON(ctx, tx, `
INSERT INTO content_body (item_id, tenant_id, body, sensitive_fields)
VALUES ($1,$2,$3,$4)
ON CONFLICT (item_id) DO UPDATE SET body=EXCLUDED.body, sensitive_fields=EXCLUDED.sensitive_fields, updated_at=now()`,
		id, acceptanceIDs.TenantID, body, sensitive)
}

// seedTeachingRows 写入课程、课节、作业、提交、讨论和课程成绩。
func seedTeachingRows(ctx context.Context, tx pgx.Tx) error {
	schedule := map[string]any{"items": []map[string]any{{"weekday": 2, "time": "13:30-15:05", "room": "链安实验室 A302"}}}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2027, 1, 15, 23, 59, 59, 0, time.UTC)
	if err := execJSON(ctx, tx, `
INSERT INTO course (id, tenant_id, teacher_id, name, description, type, difficulty, semester, credits, schedule, start_at, end_at, invite_code, status, visibility)
VALUES ($1,$2,$3,'区块链系统设计与安全实践','面向区块链工程专业的智能合约安全、链上验证与实验环境实践课程。',3,2,'2026-2027-1',3.0,$4,$5,$6,'CHAIN26A',3,1)
ON CONFLICT (id) DO UPDATE SET teacher_id=EXCLUDED.teacher_id, name=EXCLUDED.name, description=EXCLUDED.description, type=EXCLUDED.type, difficulty=EXCLUDED.difficulty, semester=EXCLUDED.semester, credits=EXCLUDED.credits, schedule=EXCLUDED.schedule, start_at=EXCLUDED.start_at, end_at=EXCLUDED.end_at, invite_code=EXCLUDED.invite_code, status=EXCLUDED.status, visibility=EXCLUDED.visibility, deleted_at=NULL, updated_at=now()`,
		acceptanceIDs.Course, acceptanceIDs.TenantID, acceptanceIDs.TeacherMain, schedule, start, end); err != nil {
		return err
	}
	for _, chapter := range []struct {
		id    int64
		title string
		sort  int
	}{{acceptanceIDs.ChapterIntro, "区块链运行机制与智能合约安全基础", 1}, {acceptanceIDs.ChapterLab, "可重入漏洞复现、利用与修复", 2}} {
		if err := execJSON(ctx, tx, `
INSERT INTO chapter (id, tenant_id, course_id, title, sort)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (id) DO UPDATE SET course_id=EXCLUDED.course_id, title=EXCLUDED.title, sort=EXCLUDED.sort, deleted_at=NULL, updated_at=now()`,
			chapter.id, acceptanceIDs.TenantID, acceptanceIDs.Course, chapter.title, chapter.sort); err != nil {
			return err
		}
	}
	lessonIntro := map[string]any{"markdown": "本节梳理交易、区块、状态机与合约调用的关系。"}
	lessonLab := map[string]any{"experiment_id": ids.Format(acceptanceIDs.Experiment)}
	if err := upsertLesson(ctx, tx, acceptanceIDs.LessonIntro, acceptanceIDs.ChapterIntro, "交易生命周期与状态转换", 2, lessonIntro, 1); err != nil {
		return err
	}
	if err := upsertLesson(ctx, tx, acceptanceIDs.LessonLab, acceptanceIDs.ChapterLab, "Foundry 可重入漏洞实验", 4, lessonLab, 1); err != nil {
		return err
	}
	for _, member := range []struct {
		id        int64
		studentID int64
	}{{acceptanceIDs.Course + 101, acceptanceIDs.StudentA}, {acceptanceIDs.Course + 102, acceptanceIDs.StudentB}, {acceptanceIDs.Course + 103, acceptanceIDs.StudentC}} {
		if err := execJSON(ctx, tx, `
INSERT INTO course_member (id, tenant_id, course_id, student_id, join_mode)
VALUES ($1,$2,$3,$4,1)
ON CONFLICT (tenant_id, course_id, student_id) DO UPDATE SET join_mode=EXCLUDED.join_mode`,
			member.id, acceptanceIDs.TenantID, acceptanceIDs.Course, member.studentID); err != nil {
			return err
		}
	}
	latePolicy := map[string]any{"type": "daily_percent", "value": 10, "max_percent": 50}
	due := time.Date(2026, 10, 20, 23, 59, 59, 0, time.UTC)
	if err := execJSON(ctx, tx, `
INSERT INTO assignment (id, tenant_id, course_id, title, chapter_id, due_at, max_attempts, late_policy, late_penalty, status)
VALUES ($1,$2,$3,'作业一: 可重入漏洞攻击与修复报告',$4,$5,3,2,$6,2)
ON CONFLICT (id) DO UPDATE SET course_id=EXCLUDED.course_id, title=EXCLUDED.title, chapter_id=EXCLUDED.chapter_id, due_at=EXCLUDED.due_at, max_attempts=EXCLUDED.max_attempts, late_policy=EXCLUDED.late_policy, late_penalty=EXCLUDED.late_penalty, status=EXCLUDED.status, deleted_at=NULL, updated_at=now()`,
		acceptanceIDs.Assignment, acceptanceIDs.TenantID, acceptanceIDs.Course, acceptanceIDs.ChapterLab, due, latePolicy); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO assignment_item (id, tenant_id, assignment_id, item_code, item_version, score, seq, grading_mode, judger_code)
VALUES ($1,$2,$3,'ctf-reentrancy-vault','1.0.0',100,1,1,'solidity-unit')
ON CONFLICT (tenant_id, assignment_id, seq) DO UPDATE SET item_code=EXCLUDED.item_code, item_version=EXCLUDED.item_version, score=EXCLUDED.score, grading_mode=EXCLUDED.grading_mode, judger_code=EXCLUDED.judger_code`,
		acceptanceIDs.AssignmentItem, acceptanceIDs.TenantID, acceptanceIDs.Assignment); err != nil {
		return err
	}
	submissionRef := map[string]any{"bucket": "chaimir-code", "key": "acceptance/submissions/S20260001/reentrancy-fixed.zip"}
	if err := execJSON(ctx, tx, `
INSERT INTO submission (id, tenant_id, assignment_id, student_id, attempt_no, content_ref, auto_score, final_score, comment, is_late, status)
VALUES ($1,$2,$3,$4,1,$5,92,92,'修复思路完整,测试覆盖了重复提款路径。',false,3)
ON CONFLICT (tenant_id, assignment_id, student_id, attempt_no) DO UPDATE SET content_ref=EXCLUDED.content_ref, auto_score=EXCLUDED.auto_score, final_score=EXCLUDED.final_score, comment=EXCLUDED.comment, is_late=EXCLUDED.is_late, status=EXCLUDED.status`,
		acceptanceIDs.SubmissionA, acceptanceIDs.TenantID, acceptanceIDs.Assignment, acceptanceIDs.StudentA, submissionRef); err != nil {
		return err
	}
	draft := map[string]any{"note": "攻击合约已完成,正在补充修复说明。"}
	if err := execJSON(ctx, tx, `
INSERT INTO submission_draft (id, tenant_id, assignment_id, student_id, content)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (tenant_id, assignment_id, student_id) DO UPDATE SET content=EXCLUDED.content, updated_at=now()`,
		acceptanceIDs.DraftB, acceptanceIDs.TenantID, acceptanceIDs.Assignment, acceptanceIDs.StudentB, draft); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO lesson_progress (id, tenant_id, lesson_id, student_id, status, duration_sec)
VALUES ($1,$2,$3,$4,3,1800)
ON CONFLICT (tenant_id, lesson_id, student_id) DO UPDATE SET status=EXCLUDED.status, duration_sec=EXCLUDED.duration_sec, updated_at=now()`,
		acceptanceIDs.ProgressA, acceptanceIDs.TenantID, acceptanceIDs.LessonIntro, acceptanceIDs.StudentA); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO discussion_post (id, tenant_id, course_id, author_id, content, is_pinned)
VALUES ($1,$2,$3,$4,'重入攻击复现时请先确认本地链快照已重置,避免旧状态影响断言。',true)
ON CONFLICT (id) DO UPDATE SET content=EXCLUDED.content, is_pinned=EXCLUDED.is_pinned, deleted_at=NULL`,
		acceptanceIDs.Discussion, acceptanceIDs.TenantID, acceptanceIDs.Course, acceptanceIDs.TeacherAssist); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO announcement (id, tenant_id, course_id, title, content, is_pinned)
VALUES ($1,$2,$3,'实验环境维护窗口','本周五 22:00 至 23:00 将更新 Foundry 基础镜像,已启动环境不受影响。',true)
ON CONFLICT (id) DO UPDATE SET title=EXCLUDED.title, content=EXCLUDED.content, is_pinned=EXCLUDED.is_pinned, deleted_at=NULL`,
		acceptanceIDs.CourseNotice, acceptanceIDs.TenantID, acceptanceIDs.Course); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO course_review (id, tenant_id, course_id, student_id, rating, comment)
VALUES ($1,$2,$3,$4,5,'实验步骤和链上验证目标清晰,适合复盘安全漏洞。')
ON CONFLICT (tenant_id, course_id, student_id) DO UPDATE SET rating=EXCLUDED.rating, comment=EXCLUDED.comment`,
		acceptanceIDs.CourseReview, acceptanceIDs.TenantID, acceptanceIDs.Course, acceptanceIDs.StudentA); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO grade_weight (id, tenant_id, course_id, source_type, source_ref, weight)
VALUES ($1,$2,$3,1,$4,100.00)
ON CONFLICT (tenant_id, course_id, source_type, source_ref) DO UPDATE SET weight=EXCLUDED.weight, updated_at=now()`,
		acceptanceIDs.GradeWeight, acceptanceIDs.TenantID, acceptanceIDs.Course, "assignment:910000000000005031"); err != nil {
		return err
	}
	return execJSON(ctx, tx, `
INSERT INTO course_grade (id, tenant_id, course_id, student_id, auto_total, override_total, is_overridden, is_locked)
VALUES ($1,$2,$3,$4,92.00,NULL,false,false)
ON CONFLICT (tenant_id, course_id, student_id) DO UPDATE SET auto_total=EXCLUDED.auto_total, override_total=EXCLUDED.override_total, is_overridden=EXCLUDED.is_overridden, is_locked=EXCLUDED.is_locked, updated_at=now()`,
		acceptanceIDs.CourseGradeA, acceptanceIDs.TenantID, acceptanceIDs.Course, acceptanceIDs.StudentA)
}

// upsertLesson 幂等写入课程课节。
func upsertLesson(ctx context.Context, tx pgx.Tx, id, chapterID int64, title string, contentType int16, contentRef any, sort int) error {
	return execJSON(ctx, tx, `
INSERT INTO lesson (id, tenant_id, chapter_id, title, content_type, content_ref, sort)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (id) DO UPDATE SET chapter_id=EXCLUDED.chapter_id, title=EXCLUDED.title, content_type=EXCLUDED.content_type, content_ref=EXCLUDED.content_ref, sort=EXCLUDED.sort, deleted_at=NULL, updated_at=now()`,
		id, acceptanceIDs.TenantID, chapterID, title, contentType, contentRef, sort)
}

// seedExperimentRows 写入实验定义、分组、实例、检查点和报告。
func seedExperimentRows(ctx context.Context, tx pgx.Tx) error {
	components := map[string]any{
		"envs": []map[string]any{{
			"id":                         "lab-foundry",
			"runtime_code":               "evm-foundry",
			"runtime_image_version":      "2026.06",
			"tools":                      []string{"code-server"},
			"init_code_ref":              acceptanceInitCodeRef,
			"init_script_ref":            acceptanceInitScriptRef,
			"snapshot_enabled":           false,
			"snapshot_retention_minutes": 0,
			"keep_alive":                 true,
			"keep_alive_minutes":         60,
		}},
		"checkpoints": []map[string]any{
			{"id": "withdraw-guard", "judger": "solidity-unit", "item_code": "ctf-reentrancy-vault", "item_version": "1.0.0", "score": 60, "mode": "fresh", "env_id": "lab-foundry"},
			{"id": "attack-regression", "judger": "solidity-unit", "item_code": "ctf-reentrancy-vault", "item_version": "1.0.0", "score": 40, "mode": "fresh", "env_id": "lab-foundry"},
		},
		"stages": []map[string]any{
			{"stage": 1, "title": "漏洞复现与修复", "description": "使用 Foundry 复现可重入攻击并完成修复。", "components": map[string]any{"envs": []string{"lab-foundry"}}},
		},
	}
	if err := execJSON(ctx, tx, `
INSERT INTO experiment (id, tenant_id, course_id, author_id, template_ref, template_version, name, description, components, collab_mode, require_report, wizard_step, status)
VALUES ($1,$2,$3,$4,'lab-reentrancy-foundry','1.0.0','可重入漏洞攻防实验','学生需要复现攻击、完成修复并提交报告。',$5,2,true,6,2)
ON CONFLICT (id) DO UPDATE SET course_id=EXCLUDED.course_id, author_id=EXCLUDED.author_id, template_ref=EXCLUDED.template_ref, template_version=EXCLUDED.template_version, name=EXCLUDED.name, description=EXCLUDED.description, components=EXCLUDED.components, collab_mode=EXCLUDED.collab_mode, require_report=EXCLUDED.require_report, wizard_step=EXCLUDED.wizard_step, status=EXCLUDED.status, deleted_at=NULL, updated_at=now()`,
		acceptanceIDs.Experiment, acceptanceIDs.TenantID, acceptanceIDs.Course, acceptanceIDs.TeacherMain, components); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO experiment_group (id, tenant_id, experiment_id, name)
VALUES ($1,$2,$3,'第一小组: Vault 审计')
ON CONFLICT (id) DO UPDATE SET experiment_id=EXCLUDED.experiment_id, name=EXCLUDED.name`,
		acceptanceIDs.ExperimentGroup, acceptanceIDs.TenantID, acceptanceIDs.Experiment); err != nil {
		return err
	}
	for _, member := range []struct {
		id        int64
		studentID int64
		role      string
	}{{acceptanceIDs.GroupMemberA, acceptanceIDs.StudentA, "leader"}, {acceptanceIDs.GroupMemberB, acceptanceIDs.StudentB, "member"}} {
		if err := execJSON(ctx, tx, `
INSERT INTO group_member (id, tenant_id, group_id, student_id, role)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (tenant_id, group_id, student_id) DO UPDATE SET role=EXCLUDED.role`,
			member.id, acceptanceIDs.TenantID, acceptanceIDs.ExperimentGroup, member.studentID, member.role); err != nil {
			return err
		}
	}
	sandboxRefs := []map[string]any{{
		"component_id": "lab-foundry",
		"stage":        1,
		"sandbox_id":   acceptanceIDs.Sandbox,
		"runtime_code": "evm-foundry",
		"tools": []map[string]any{{
			"code":     "code-server",
			"kind":     3,
			"endpoint": "/api/v1/sandbox/sandboxes/910000000000001021/tools/code-server/",
			"status":   1,
		}},
	}}
	simRefs := []map[string]any{{
		"component_id": "gas-metering",
		"stage":        1,
		"session_id":   acceptanceIDs.SimSession,
		"package_code": "builtin__runtime-gas-metering",
		"version":      "1.0.0",
		"bundle_ref":   "builtin://sim-sdk/builtin__runtime-gas-metering@1.0.0",
	}}
	if err := execJSON(ctx, tx, `
INSERT INTO experiment_instance (id, tenant_id, experiment_id, owner_account_id, group_id, source_ref, sandbox_refs, sim_session_refs, status, score, finished_at)
VALUES ($1,$2,$3,$4,$5,'experiment:2026:reentrancy:instance-a',$6,$7,4,88.50,now())
ON CONFLICT (id) DO UPDATE SET experiment_id=EXCLUDED.experiment_id, owner_account_id=EXCLUDED.owner_account_id, group_id=EXCLUDED.group_id, source_ref=EXCLUDED.source_ref, sandbox_refs=EXCLUDED.sandbox_refs, sim_session_refs=EXCLUDED.sim_session_refs, status=EXCLUDED.status, score=EXCLUDED.score, finished_at=EXCLUDED.finished_at, last_active_at=now()`,
		acceptanceIDs.ExperimentInstance, acceptanceIDs.TenantID, acceptanceIDs.Experiment, acceptanceIDs.StudentA, acceptanceIDs.ExperimentGroup, sandboxRefs, simRefs); err != nil {
		return err
	}
	checkpointDetail := map[string]any{"assertion": "withdraw balance cannot be drained twice", "passed_cases": 7, "total_cases": 8}
	if err := execJSON(ctx, tx, `
INSERT INTO checkpoint_result (id, tenant_id, instance_id, checkpoint_id, judge_task_ref, passed, score, detail_ref, binding_output)
VALUES ($1,$2,$3,'withdraw-guard','judge:acceptance:withdraw-guard',true,60.00,'reports/acceptance/checkpoints/withdraw-guard.json',$4)
ON CONFLICT (tenant_id, instance_id, checkpoint_id) DO UPDATE SET judge_task_ref=EXCLUDED.judge_task_ref, passed=EXCLUDED.passed, score=EXCLUDED.score, detail_ref=EXCLUDED.detail_ref, binding_output=EXCLUDED.binding_output, judged_at=now()`,
		acceptanceIDs.CheckpointResult, acceptanceIDs.TenantID, acceptanceIDs.ExperimentInstance, checkpointDetail); err != nil {
		return err
	}
	return execJSON(ctx, tx, `
INSERT INTO experiment_report (id, tenant_id, instance_id, student_id, content_ref, manual_score, comment, status)
VALUES ($1,$2,$3,$4,'reports/acceptance/S20260001/reentrancy-report.pdf',28.50,'报告对调用栈和余额变化分析完整。',2)
ON CONFLICT (tenant_id, instance_id, student_id) DO UPDATE SET content_ref=EXCLUDED.content_ref, manual_score=EXCLUDED.manual_score, comment=EXCLUDED.comment, status=EXCLUDED.status, submitted_at=now()`,
		acceptanceIDs.ExperimentReport, acceptanceIDs.TenantID, acceptanceIDs.ExperimentInstance, acceptanceIDs.StudentA)
}

type acceptanceBuiltinSimPackage struct {
	Code     string
	Name     string
	Category string
	Version  string
}

var acceptanceBuiltinSimPackages = []acceptanceBuiltinSimPackage{
	{Code: "builtin__runtime-gas-metering", Name: "Gas 计量与回滚推演", Category: "transaction-runtime", Version: "1.0.0"},
	{Code: "builtin__cross-bridge-validation", Name: "跨链桥证明验证推演", Category: "cross-chain-system", Version: "1.0.0"},
	{Code: "builtin__cross-finality-confirmation", Name: "跨链最终性确认推演", Category: "cross-chain-system", Version: "1.0.0"},
	{Code: "builtin__cross-message-lifecycle", Name: "跨链消息生命周期推演", Category: "cross-chain-system", Version: "1.0.0"},
	{Code: "builtin__cross-multisig-committee", Name: "跨链多签委员会推演", Category: "cross-chain-system", Version: "1.0.0"},
	{Code: "builtin__cross-replay-protection", Name: "跨链消息重放防护推演", Category: "cross-chain-system", Version: "1.0.0"},
	{Code: "builtin__crypto-digital-signature", Name: "数字签名与重放防护推演", Category: "cryptography", Version: "1.0.0"},
	{Code: "builtin__crypto-hash-chain", Name: "哈希链篡改扩散推演", Category: "cryptography", Version: "1.0.0"},
	{Code: "builtin__crypto-merkle-proof", Name: "Merkle 证明路径推演", Category: "cryptography", Version: "1.0.0"},
	{Code: "builtin__crypto-threshold-signature", Name: "门限签名聚合推演", Category: "cryptography", Version: "1.0.0"},
	{Code: "builtin__crypto-zk-proof", Name: "零知识证明交互流程推演", Category: "cryptography", Version: "1.0.0"},
	{Code: "builtin__data-blockchain-link", Name: "区块链父哈希结构推演", Category: "data-structure", Version: "1.0.0"},
	{Code: "builtin__data-merkle-tree-structure", Name: "Merkle Tree 构建更新推演", Category: "data-structure", Version: "1.0.0"},
	{Code: "builtin__data-patricia-trie", Name: "Patricia Trie 状态树推演", Category: "data-structure", Version: "1.0.0"},
	{Code: "builtin__data-state-snapshot", Name: "状态快照与回滚推演", Category: "data-structure", Version: "1.0.0"},
	{Code: "builtin__data-utxo-set", Name: "UTXO 集合更新推演", Category: "data-structure", Version: "1.0.0"},
	{Code: "builtin__hotstuff-chained-bft", Name: "HotStuff 链式 BFT 推演", Category: "consensus", Version: "1.0.0"},
	{Code: "builtin__network-dht-routing", Name: "DHT 异或路由推演", Category: "network", Version: "1.0.0"},
	{Code: "builtin__network-gossip-propagation", Name: "Gossip 消息传播推演", Category: "network", Version: "1.0.0"},
	{Code: "builtin__network-latency-loss", Name: "延迟丢包与重传推演", Category: "network", Version: "1.0.0"},
	{Code: "builtin__network-p2p-discovery", Name: "P2P 节点发现推演", Category: "network", Version: "1.0.0"},
	{Code: "builtin__network-partition-recovery", Name: "网络分区与恢复推演", Category: "network", Version: "1.0.0"},
	{Code: "builtin__pbft-consensus", Name: "PBFT 三阶段共识推演", Category: "consensus", Version: "1.0.0"},
	{Code: "builtin__pos-finality", Name: "PoS 权益证明与最终性推演", Category: "consensus", Version: "1.0.0"},
	{Code: "builtin__pow-longest-chain", Name: "PoW 最长链共识推演", Category: "consensus", Version: "1.0.0"},
	{Code: "builtin__raft-log-replication", Name: "Raft 选举与日志复制推演", Category: "consensus", Version: "1.0.0"},
	{Code: "builtin__runtime-block-validation", Name: "区块验证与拒绝推演", Category: "transaction-runtime", Version: "1.0.0"},
	{Code: "builtin__runtime-evm-call-stack", Name: "EVM 调用栈与 revert 推演", Category: "transaction-runtime", Version: "1.0.0"},
	{Code: "builtin__runtime-nonce-ordering", Name: "Nonce 顺序与替换交易推演", Category: "transaction-runtime", Version: "1.0.0"},
	{Code: "builtin__runtime-transaction-lifecycle", Name: "交易生命周期推演", Category: "transaction-runtime", Version: "1.0.0"},
	{Code: "builtin__security-access-control", Name: "授权缺陷与最小权限推演", Category: "contract-security", Version: "1.0.0"},
	{Code: "builtin__security-flash-loan", Name: "闪电贷组合攻击推演", Category: "contract-security", Version: "1.0.0"},
	{Code: "builtin__security-integer-boundary", Name: "整数边界与 checked 运算推演", Category: "contract-security", Version: "1.0.0"},
	{Code: "builtin__security-oracle-manipulation", Name: "预言机操纵防护推演", Category: "contract-security", Version: "1.0.0"},
	{Code: "builtin__security-reentrancy", Name: "重入攻击与防护推演", Category: "contract-security", Version: "1.0.0"},
}

// seedSimRows 写入仿真包、会话、动作、检查点和分享码。
func seedSimRows(ctx context.Context, tx pgx.Tx) error {
	scale := map[string]any{"max_nodes": 96, "max_ticks": 140, "max_events": 240}
	schema := map[string]any{"events": map[string]any{
		"select":  map[string]any{"interaction_id": "select", "kind": "select-element", "target": "element", "params": []map[string]any{}},
		"advance": map[string]any{"interaction_id": "advance", "kind": "button", "target": "global", "params": []map[string]any{}},
		"attack":  map[string]any{"interaction_id": "attack", "kind": "button", "target": "global", "params": []map[string]any{}},
		"recover": map[string]any{"interaction_id": "recover", "kind": "button", "target": "global", "params": []map[string]any{}},
	}}
	for index, pkg := range acceptanceBuiltinSimPackages {
		packageID := acceptanceIDs.SimPackage + int64(index)
		bundleKey := fmt.Sprintf("builtin://sim-sdk/%s@%s", pkg.Code, pkg.Version)
		bundleHash := fmt.Sprintf("%064x", packageID)
		if err := execJSON(ctx, tx, `
INSERT INTO sim_package (id, code, version, name, category, compute, scale_limit, bundle_key, bundle_hash, interaction_schema, author_type, status)
VALUES ($1,$2,$3,$4,$5,1,$6,$7,$8,$9,1,3)
ON CONFLICT (code, version) DO UPDATE SET name=EXCLUDED.name, category=EXCLUDED.category, compute=EXCLUDED.compute, scale_limit=EXCLUDED.scale_limit, bundle_key=EXCLUDED.bundle_key, bundle_hash=EXCLUDED.bundle_hash, interaction_schema=EXCLUDED.interaction_schema, author_type=EXCLUDED.author_type, status=EXCLUDED.status, updated_at=now()`,
			packageID, pkg.Code, pkg.Version, pkg.Name, pkg.Category, scale, bundleKey, bundleHash, schema); err != nil {
			return err
		}
	}
	params := map[string]any{"gas_limit": 42_000, "scenario": "classroom-demo"}
	if err := execJSON(ctx, tx, `
INSERT INTO sim_session (id, tenant_id, package_id, source_ref, owner_account_id, seed, init_params, compute, status)
VALUES ($1,$2,$3,'sim:2026:gas-metering:session-a',$4,2026061901,$5,1,4)
ON CONFLICT (id) DO UPDATE SET package_id=EXCLUDED.package_id, source_ref=EXCLUDED.source_ref, owner_account_id=EXCLUDED.owner_account_id, seed=EXCLUDED.seed, init_params=EXCLUDED.init_params, compute=EXCLUDED.compute, status=EXCLUDED.status, updated_at=now()`,
		acceptanceIDs.SimSession, acceptanceIDs.TenantID, acceptanceIDs.SimPackage, acceptanceIDs.StudentA, params); err != nil {
		return err
	}
	payload := map[string]any{}
	if err := execJSON(ctx, tx, `
INSERT INTO sim_action_log (id, tenant_id, session_id, seq, at_tick, event_type, payload)
VALUES ($1,$2,$3,1,1,'advance',$4)
ON CONFLICT (tenant_id, session_id, seq) DO UPDATE SET at_tick=EXCLUDED.at_tick, event_type=EXCLUDED.event_type, payload=EXCLUDED.payload`,
		acceptanceIDs.SimAction, acceptanceIDs.TenantID, acceptanceIDs.SimSession, payload); err != nil {
		return err
	}
	answer := map[string]any{"phase": "gas-deducted", "rollback_observed": true}
	if err := execJSON(ctx, tx, `
INSERT INTO sim_checkpoint (id, tenant_id, session_id, checkpoint_id, answer, achieved)
VALUES ($1,$2,$3,'gas-metering-rollback',$4,true)
ON CONFLICT (tenant_id, session_id, checkpoint_id) DO UPDATE SET answer=EXCLUDED.answer, achieved=EXCLUDED.achieved`,
		acceptanceIDs.SimCheckpoint, acceptanceIDs.TenantID, acceptanceIDs.SimSession, answer); err != nil {
		return err
	}
	return execJSON(ctx, tx, `
INSERT INTO sim_share (id, tenant_id, session_id, code, created_by, status, expire_at)
VALUES ($1,$2,$3,'GASMETER26',$4,1,now() + interval '30 days')
ON CONFLICT (code) DO UPDATE SET session_id=EXCLUDED.session_id, created_by=EXCLUDED.created_by, status=EXCLUDED.status, expire_at=EXCLUDED.expire_at, updated_at=now()`,
		acceptanceIDs.SimShare, acceptanceIDs.TenantID, acceptanceIDs.SimSession, acceptanceIDs.StudentA)
}

// seedContestRows 写入解题赛、队伍、提交、榜单和漏洞题素材。
func seedContestRows(ctx context.Context, tx pgx.Tx) error {
	rules := map[string]any{"scoring": "static", "allowed_languages": []string{"solidity"}, "appeal_minutes": 20}
	signupStart := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	signupEnd := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	start := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 10, 17, 0, 0, 0, time.UTC)
	if err := execJSON(ctx, tx, `
INSERT INTO contest (id, tenant_id, organizer_id, name, mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status)
VALUES ($1,$2,$3,'2026 链安新生攻防挑战赛',1,1,$4,$5,$6,$7,30,$8,3)
ON CONFLICT (id) DO UPDATE SET organizer_id=EXCLUDED.organizer_id, name=EXCLUDED.name, mode=EXCLUDED.mode, team_mode=EXCLUDED.team_mode, signup_start=EXCLUDED.signup_start, signup_end=EXCLUDED.signup_end, start_at=EXCLUDED.start_at, end_at=EXCLUDED.end_at, freeze_minutes=EXCLUDED.freeze_minutes, rules=EXCLUDED.rules, status=EXCLUDED.status, deleted_at=NULL, updated_at=now()`,
		acceptanceIDs.Contest, acceptanceIDs.TenantID, acceptanceIDs.TeacherMain, signupStart, signupEnd, start, end, rules); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO contest_problem (id, tenant_id, contest_id, item_code, item_version, score, seq)
VALUES ($1,$2,$3,'ctf-reentrancy-vault','1.0.0',500,1)
ON CONFLICT (tenant_id, contest_id, item_code, item_version) DO UPDATE SET score=EXCLUDED.score, seq=EXCLUDED.seq`,
		acceptanceIDs.ContestProblem, acceptanceIDs.TenantID, acceptanceIDs.Contest); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO team (id, tenant_id, contest_id, name, invite_code, status)
VALUES ($1,$2,$3,'赵一航个人队','ZA2026',2)
ON CONFLICT (id) DO UPDATE SET contest_id=EXCLUDED.contest_id, name=EXCLUDED.name, invite_code=EXCLUDED.invite_code, status=EXCLUDED.status`,
		acceptanceIDs.TeamA, acceptanceIDs.TenantID, acceptanceIDs.Contest); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO team_member (id, tenant_id, team_id, account_id, member_tenant_id, is_leader)
VALUES ($1,$2,$3,$4,$2,true)
ON CONFLICT (tenant_id, team_id, member_tenant_id, account_id) DO UPDATE SET is_leader=EXCLUDED.is_leader`,
		acceptanceIDs.TeamAMember, acceptanceIDs.TenantID, acceptanceIDs.TeamA, acceptanceIDs.StudentA); err != nil {
		return err
	}
	contentRef := map[string]any{"answer": ""}
	if err := execJSON(ctx, tx, `
INSERT INTO solve_submission (id, tenant_id, contest_id, problem_id, team_id, submitter_id, content_ref, source_ref, passed, score, sandbox_ref)
VALUES ($1,$2,$3,$4,$5,$6,$7,'contest:2026:solve:ZA2026-001',true,500,'sandbox:contest:ZA2026-001')
ON CONFLICT (tenant_id, source_ref) DO UPDATE SET content_ref=EXCLUDED.content_ref, passed=EXCLUDED.passed, score=EXCLUDED.score, sandbox_ref=EXCLUDED.sandbox_ref`,
		acceptanceIDs.SolveSubmission, acceptanceIDs.TenantID, acceptanceIDs.Contest, acceptanceIDs.ContestProblem, acceptanceIDs.TeamA, acceptanceIDs.StudentA, contentRef); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO ladder_rank (id, tenant_id, contest_id, team_id, score, solved_count, last_solve_at, rank)
VALUES ($1,$2,$3,$4,500.00,1,now(),1)
ON CONFLICT (tenant_id, contest_id, team_id) DO UPDATE SET score=EXCLUDED.score, solved_count=EXCLUDED.solved_count, last_solve_at=EXCLUDED.last_solve_at, rank=EXCLUDED.rank, updated_at=now()`,
		acceptanceIDs.LadderRank, acceptanceIDs.TenantID, acceptanceIDs.Contest, acceptanceIDs.TeamA); err != nil {
		return err
	}
	snapshotAt := end
	ranking := []map[string]any{{"rank": 1, "team_id": fmt.Sprintf("%d", acceptanceIDs.TeamA), "score": 500, "solved_count": 1, "last_solve_at": snapshotAt, "updated_at": snapshotAt}}
	if err := execJSON(ctx, tx, `
INSERT INTO contest_ladder_snapshot (id, tenant_id, contest_id, snapshot_status, ranking)
VALUES ($1,$2,$3,6,$4)
ON CONFLICT (tenant_id, contest_id, snapshot_status) DO UPDATE SET ranking=EXCLUDED.ranking, generated_at=now()`,
		acceptanceIDs.ResultSnapshot, acceptanceIDs.TenantID, acceptanceIDs.Contest, ranking); err != nil {
		return err
	}
	sourceConfig := map[string]any{"source": "teacher-curated", "license": "internal-training"}
	if err := execJSON(ctx, tx, `
INSERT INTO vuln_source (id, tenant_id, type, name, config, default_level, enabled, last_sync_at)
VALUES ($1,$2,3,'校内智能合约漏洞案例库',$3,2,true,now())
ON CONFLICT (tenant_id, id) DO UPDATE SET type=EXCLUDED.type, name=EXCLUDED.name, config=EXCLUDED.config, default_level=EXCLUDED.default_level, enabled=EXCLUDED.enabled, last_sync_at=EXCLUDED.last_sync_at, updated_at=now()`,
		acceptanceIDs.VulnSource, acceptanceIDs.TenantID, sourceConfig); err != nil {
		return err
	}
	draftBody := map[string]any{
		"statement":      "复现 Vault withdraw 的可重入漏洞，提交能够在余额更新前重复提取资金的最小利用步骤。",
		"init_contracts": []string{"minio://chaimir-code/910000000000000001/vuln/CL-REENTRANCY-2026-001/Vault.sol"},
		"judge_config":   map[string]any{"judger_code": "onchain-assert", "max_score": 100},
		"init_steps": []map[string]any{
			{"op": "deploy", "payload": map[string]any{"contract_ref": "minio://chaimir-code/910000000000000001/vuln/CL-REENTRANCY-2026-001/Vault.sol"}},
		},
		"positive_steps": []map[string]any{
			{"op": "tx", "payload": map[string]any{"method": "attack", "value": "1 ether"}},
		},
		"assertions": []map[string]any{
			{"label": "攻击合约余额增加", "target": "contract:attacker", "field": "balance", "op": "gt", "value": 0, "expected_label": "攻击合约应获得额外余额"},
		},
	}
	return execJSON(ctx, tx, `
INSERT INTO vuln_problem (id, tenant_id, source_id, external_ref, title, level, runtime_mode, draft_body, prevalidate_status, prevalidate_detail, content_item_code, content_item_version, status)
VALUES ($1,$2,$3,'CL-REENTRANCY-2026-001','Vault withdraw 可重入漏洞',1,1,$4,2,'{"positive":"passed","negative":"passed"}'::jsonb,'ctf-reentrancy-vault','1.0.0',2)
ON CONFLICT (id) DO UPDATE SET source_id=EXCLUDED.source_id, external_ref=EXCLUDED.external_ref, title=EXCLUDED.title, level=EXCLUDED.level, runtime_mode=EXCLUDED.runtime_mode, draft_body=EXCLUDED.draft_body, prevalidate_status=EXCLUDED.prevalidate_status, prevalidate_detail=EXCLUDED.prevalidate_detail, content_item_code=EXCLUDED.content_item_code, content_item_version=EXCLUDED.content_item_version, status=EXCLUDED.status, updated_at=now()`,
		acceptanceIDs.VulnProblem, acceptanceIDs.TenantID, acceptanceIDs.VulnSource, draftBody)
}

// seedNotifyRows 写入公告、站内信、偏好和已读状态。
func seedNotifyRows(ctx context.Context, tx pgx.Tx) error {
	if err := execJSON(ctx, tx, `
INSERT INTO system_announcement (id, tenant_id, title, content, scope, target_roles, publisher_id, expire_at)
VALUES ($1,$2,'链安实验周安排','本周实验重点为可重入漏洞复现、链上断言和修复报告提交。',2,NULL,$3,now() + interval '45 days')
ON CONFLICT (id) DO UPDATE SET title=EXCLUDED.title, content=EXCLUDED.content, scope=EXCLUDED.scope, target_roles=EXCLUDED.target_roles, publisher_id=EXCLUDED.publisher_id, expire_at=EXCLUDED.expire_at`,
		acceptanceIDs.SystemAnnouncement, acceptanceIDs.TenantID, acceptanceIDs.SchoolAdmin); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO notification (id, tenant_id, receiver_id, type, title, content, link, is_read)
VALUES ($1,$2,$3,'assignment.published','新作业已发布','区块链系统设计与安全实践发布了作业一,请在截止前完成。','/student/courses/910000000000005001/assignments/910000000000005031',false)
ON CONFLICT (id) DO UPDATE SET receiver_id=EXCLUDED.receiver_id, type=EXCLUDED.type, title=EXCLUDED.title, content=EXCLUDED.content, link=EXCLUDED.link, is_read=EXCLUDED.is_read, deleted_at=NULL`,
		acceptanceIDs.NotificationA, acceptanceIDs.TenantID, acceptanceIDs.StudentA); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO notification_preference (id, tenant_id, account_id, type, enabled)
VALUES ($1,$2,$3,'assignment.due',true)
ON CONFLICT (tenant_id, account_id, type) DO UPDATE SET enabled=EXCLUDED.enabled`,
		acceptanceIDs.PreferenceA, acceptanceIDs.TenantID, acceptanceIDs.StudentA); err != nil {
		return err
	}
	return execJSON(ctx, tx, `
INSERT INTO announcement_read (id, tenant_id, announcement_id, account_id)
VALUES ($1,$2,$3,$4)
ON CONFLICT (tenant_id, announcement_id, account_id) DO UPDATE SET read_at=now()`,
		acceptanceIDs.AnnouncementReadA, acceptanceIDs.TenantID, acceptanceIDs.SystemAnnouncement, acceptanceIDs.StudentA)
}

// seedGradeRows 写入成绩中心等级、学期、审核、申诉、预警和成绩单。
func seedGradeRows(ctx context.Context, tx pgx.Tx) error {
	mapping := []map[string]any{{"min": 90, "grade": "A", "gpa": 4.0}, {"min": 80, "grade": "B", "gpa": 3.0}, {"min": 60, "grade": "C", "gpa": 2.0}, {"min": 0, "grade": "F", "gpa": 0.0}}
	warningRules := map[string]any{"min_gpa": 2.0, "fail_count": 1}
	if err := execJSON(ctx, tx, `
INSERT INTO grade_level_config (id, tenant_id, name, mapping, warning_rules, is_default)
VALUES ($1,$2,'四分制等级换算',$3,$4,true)
ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, mapping=EXCLUDED.mapping, warning_rules=EXCLUDED.warning_rules, is_default=EXCLUDED.is_default, updated_at=now()`,
		acceptanceIDs.GradeLevel, acceptanceIDs.TenantID, mapping, warningRules); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO semester (id, tenant_id, name, start_date, end_date, is_current)
VALUES ($1,$2,'2026-2027-1','2026-09-01','2027-01-15',true)
ON CONFLICT (tenant_id, name) DO UPDATE SET start_date=EXCLUDED.start_date, end_date=EXCLUDED.end_date, is_current=EXCLUDED.is_current`,
		acceptanceIDs.Semester, acceptanceIDs.TenantID); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO grade_review (id, tenant_id, course_id, semester_id, submitter_id, reviewer_id, status, is_locked, comment, reviewed_at)
VALUES ($1,$2,$3,$4,$5,$6,2,true,'验收课程成绩已审核锁定。',now())
ON CONFLICT (id) DO UPDATE SET semester_id=EXCLUDED.semester_id, submitter_id=EXCLUDED.submitter_id, reviewer_id=EXCLUDED.reviewer_id, status=EXCLUDED.status, is_locked=EXCLUDED.is_locked, comment=EXCLUDED.comment, reviewed_at=EXCLUDED.reviewed_at`,
		acceptanceIDs.GradeReview, acceptanceIDs.TenantID, acceptanceIDs.Course, acceptanceIDs.Semester, acceptanceIDs.TeacherMain, acceptanceIDs.SchoolAdmin); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO student_semester_grade (id, tenant_id, student_id, semester_id, total_credits, gpa, cumulative_gpa)
VALUES ($1,$2,$3,$4,3.0,3.650,3.650)
ON CONFLICT (tenant_id, student_id, semester_id) DO UPDATE SET total_credits=EXCLUDED.total_credits, gpa=EXCLUDED.gpa, cumulative_gpa=EXCLUDED.cumulative_gpa, computed_at=now()`,
		acceptanceIDs.GradeReview+100, acceptanceIDs.TenantID, acceptanceIDs.StudentA, acceptanceIDs.Semester); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO grade_appeal (id, tenant_id, student_id, course_id, reason, status, handler_id, result_comment, handled_at)
VALUES ($1,$2,$3,$4,'申请复核报告人工评分中对攻击路径描述部分的扣分。',3,$5,'复核后维持原分,教师已补充评语。',now())
ON CONFLICT (id) DO UPDATE SET reason=EXCLUDED.reason, status=EXCLUDED.status, handler_id=EXCLUDED.handler_id, result_comment=EXCLUDED.result_comment, handled_at=EXCLUDED.handled_at`,
		acceptanceIDs.GradeAppeal, acceptanceIDs.TenantID, acceptanceIDs.StudentA, acceptanceIDs.Course, acceptanceIDs.TeacherMain); err != nil {
		return err
	}
	detail := map[string]any{"gpa": 1.95, "suggestion": "建议预约导师并完成补强练习"}
	if err := execJSON(ctx, tx, `
INSERT INTO academic_warning (id, tenant_id, student_id, semester_id, type, detail, status)
VALUES ($1,$2,$3,$4,2,$5,1)
ON CONFLICT (id) DO UPDATE SET type=EXCLUDED.type, detail=EXCLUDED.detail, status=EXCLUDED.status`,
		acceptanceIDs.AcademicWarning, acceptanceIDs.TenantID, acceptanceIDs.StudentC, acceptanceIDs.Semester, detail); err != nil {
		return err
	}
	return execJSON(ctx, tx, `
INSERT INTO transcript_record (id, tenant_id, student_id, scope, semester_id, pdf_ref)
VALUES ($1,$2,$3,1,$4,'reports/acceptance/transcripts/S20260001-2026-1.pdf')
ON CONFLICT (id) DO UPDATE SET scope=EXCLUDED.scope, semester_id=EXCLUDED.semester_id, pdf_ref=EXCLUDED.pdf_ref, generated_at=now()`,
		acceptanceIDs.Transcript, acceptanceIDs.TenantID, acceptanceIDs.StudentA, acceptanceIDs.Semester)
}

// seedAdminRows 写入管理后台配置、告警、统计和备份记录。
func seedAdminRows(ctx context.Context, tx pgx.Tx) error {
	value := map[string]any{"max_concurrent_sandbox": 30, "idle_timeout_min": 45}
	if err := execJSON(ctx, tx, `
INSERT INTO system_config (id, scope, tenant_id, key, value, version, updated_by)
VALUES ($1,2,$2,'sandbox.quota.default',$3,1,$4)
ON CONFLICT (scope, tenant_id, key) WHERE tenant_id IS NOT NULL DO UPDATE SET value=EXCLUDED.value, version=system_config.version+1, updated_by=EXCLUDED.updated_by, updated_at=now()`,
		acceptanceIDs.SystemConfig, acceptanceIDs.TenantID, value, acceptanceIDs.SchoolAdmin); err != nil {
		return err
	}
	condition := map[string]any{"metric": "sandbox_pending_seconds", "op": ">", "value": 180}
	if err := execJSON(ctx, tx, `
INSERT INTO alert_rule (id, scope, tenant_id, name, metric, condition, level, enabled)
VALUES ($1,2,$2,'实验环境等待时间过长','sandbox_pending_seconds',$3,2,true)
ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, metric=EXCLUDED.metric, condition=EXCLUDED.condition, level=EXCLUDED.level, enabled=EXCLUDED.enabled, updated_at=now()`,
		acceptanceIDs.AlertRule, acceptanceIDs.TenantID, condition); err != nil {
		return err
	}
	if err := execJSON(ctx, tx, `
INSERT INTO alert_event (id, rule_id, tenant_id, level, message, status, handler_id, handled_at)
VALUES ($1,$2,$3,2,'验收环境曾出现沙箱排队超过 180 秒,已扩容本地工作节点。',2,$4,now())
ON CONFLICT (id) DO UPDATE SET level=EXCLUDED.level, message=EXCLUDED.message, status=EXCLUDED.status, handler_id=EXCLUDED.handler_id, handled_at=EXCLUDED.handled_at`,
		acceptanceIDs.AlertEvent, acceptanceIDs.AlertRule, acceptanceIDs.TenantID, acceptanceIDs.SchoolAdmin); err != nil {
		return err
	}
	metrics := map[string]any{"active_students": 3, "published_courses": 1, "running_contests": 1, "completed_experiments": 1}
	if err := execJSON(ctx, tx, `
INSERT INTO platform_statistics (id, scope, tenant_id, stat_date, metrics)
VALUES ($1,2,$2,'2026-06-19',$3)
ON CONFLICT (scope, tenant_id, stat_date) WHERE tenant_id IS NOT NULL DO UPDATE SET metrics=EXCLUDED.metrics, created_at=now()`,
		acceptanceIDs.Statistics, acceptanceIDs.TenantID, metrics); err != nil {
		return err
	}
	return execJSON(ctx, tx, `
INSERT INTO backup_record (id, type, storage_ref, size_bytes, status, started_at, finished_at)
VALUES ($1,1,'backups/local/chaimir-acceptance-20260619.dump',73400320,2,now() - interval '1 hour',now() - interval '58 minutes')
ON CONFLICT (id) DO UPDATE SET storage_ref=EXCLUDED.storage_ref, size_bytes=EXCLUDED.size_bytes, status=EXCLUDED.status, started_at=EXCLUDED.started_at, finished_at=EXCLUDED.finished_at`,
		acceptanceIDs.BackupRecord)
}

// seedTransferRows 写入一个成功导出任务,用于统一 transfer API 和下载授权测试。
func seedTransferRows(ctx context.Context, tx pgx.Tx) error {
	return execJSON(ctx, tx, `
INSERT INTO transfer_task (
	id, tenant_id, account_id, channel, subject, status, content_type, file_name,
	attempt_count, max_attempts, last_error, artifact_ref, artifact_size,
	artifact_content_type, artifact_file_name, completed_at
) VALUES (
	$1,$2,$3,'export','audit-log-export','succeeded','text/csv','audit-log-acceptance.csv',
	1,3,'','minio://chaimir-report/910000000000000001/transfer/export/910000000000013001/audit-log-acceptance.csv',2048,
	'text/csv','audit-log-acceptance.csv',now()
)
ON CONFLICT (id) DO UPDATE SET account_id=EXCLUDED.account_id, channel=EXCLUDED.channel, subject=EXCLUDED.subject, status=EXCLUDED.status, content_type=EXCLUDED.content_type, file_name=EXCLUDED.file_name, attempt_count=EXCLUDED.attempt_count, max_attempts=EXCLUDED.max_attempts, last_error=EXCLUDED.last_error, artifact_ref=EXCLUDED.artifact_ref, artifact_size=EXCLUDED.artifact_size, artifact_content_type=EXCLUDED.artifact_content_type, artifact_file_name=EXCLUDED.artifact_file_name, completed_at=EXCLUDED.completed_at, updated_at=now()`,
		acceptanceIDs.TransferTask, acceptanceIDs.TenantID, acceptanceIDs.SchoolAdmin)
}

// seedAuditRows 写入一条系统审计记录,便于审计列表接口有可核对数据。
func seedAuditRows(ctx context.Context, tx pgx.Tx) error {
	detail := map[string]any{"seed": "acceptance", "tenant_code": "acceptance-chainlab"}
	return execJSON(ctx, tx, `
INSERT INTO audit_log (id, tenant_id, actor_id, actor_role, action, target_type, target_id, detail, ip, trace_id)
VALUES ($1,$2,$3,2,'acceptance.seed.apply','identity.tenant',$2,$4,'127.0.0.1','trace-acceptance-seed-20260619')
ON CONFLICT (id) DO UPDATE SET actor_id=EXCLUDED.actor_id, actor_role=EXCLUDED.actor_role, action=EXCLUDED.action, target_type=EXCLUDED.target_type, target_id=EXCLUDED.target_id, detail=EXCLUDED.detail, ip=EXCLUDED.ip, trace_id=EXCLUDED.trace_id`,
		acceptanceIDs.AuditEntry, acceptanceIDs.TenantID, acceptanceIDs.SchoolAdmin, detail)
}
