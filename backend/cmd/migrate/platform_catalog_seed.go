// platform_catalog_seed 负责同步平台级镜像目录；验收业务夹具不参与该流程。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/judge"
	"chaimir/internal/modules/sandbox"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/workload"
	"github.com/jackc/pgx/v5"
	"sigs.k8s.io/yaml"
)

type platformRuntimeManifest struct {
	SchemaVersion                      int `json:"schema_version"`
	Category, Name, Image, Description string
	Labels                             map[string]string `json:"labels"`
	Source                             map[string]any    `json:"source"`
	Upstream                           map[string]any    `json:"upstream"`
	DataDriven                         bool              `json:"data_driven"`
	EnvKeys                            map[string]any    `json:"env_keys"`
	RpcExposure                        map[string]any    `json:"rpc_exposure"`
	SecurityExceptions                 []map[string]any  `json:"security_exceptions"`
	StudentAccess                      map[string]any    `json:"student_access"`
	Build                              map[string]any    `json:"build"`
	Toolchain                          manifestToolchain `json:"toolchain"`
	GenesisBaked                       bool              `json:"genesis_baked"`
	Infra                              map[string]any    `json:"infra"`
	Runtime                            struct {
		Eco                   string         `json:"eco"`
		AdapterLevel          int16          `json:"adapter_level"`
		RuntimeConfigRequired bool           `json:"runtime_config_required"`
		CommandSchema         map[string]any `json:"command_schema"`
		Workload              struct {
			Env            []workload.EnvVarSpec    `json:"env"`
			InitContainers []workload.ComponentSpec `json:"init_containers"`
		} `json:"workload"`
		InitAssets []sandbox.InitAssetSpec `json:"init_assets"`
		Native     struct {
			Helper        string              `json:"helper"`
			Profile       string              `json:"profile"`
			ExecTarget    string              `json:"exec_target"`
			ResetStrategy string              `json:"reset_strategy"`
			Actions       map[string][]string `json:"actions"`
			Methods       map[string]string   `json:"methods"`
		} `json:"native"`
	} `json:"runtime"`
	SupplyChain        map[string]any              `json:"supply_chain"`
	SecretsRequired    []manifestSecretRequirement `json:"secrets_required"`
	Capabilities       manifestCapabilities        `json:"capabilities"`
	Ports              []toolManifestPort          `json:"ports"`
	Security           toolManifestSecurity        `json:"security"`
	Resources          toolManifestResources       `json:"resources"`
	Selftest           map[string]any              `json:"selftest"`
	CapabilitySelftest map[string]any              `json:"capability_selftest"`
}
type platformJudgerManifest struct {
	SchemaVersion                      int `json:"schema_version"`
	Category, Name, Image, Description string
	Labels                             map[string]string    `json:"labels"`
	Source                             map[string]any       `json:"source"`
	Upstream                           map[string]any       `json:"upstream"`
	DataDriven                         bool                 `json:"data_driven"`
	Toolchain                          manifestToolchain    `json:"toolchain"`
	Capabilities                       manifestCapabilities `json:"capabilities"`
	Ports                              []toolManifestPort   `json:"ports"`
	SecurityExceptions                 []map[string]any     `json:"security_exceptions"`
	StudentAccess                      map[string]any       `json:"student_access"`
	Build                              map[string]any       `json:"build"`
	SupplyChain                        map[string]any       `json:"supply_chain"`
	Judger                             struct {
		Type                string                `json:"type"`
		RuntimeCode         string                `json:"runtime_code"`
		GenesisRef          string                `json:"genesis_ref"`
		Command             []string              `json:"command"`
		ExecTarget          string                `json:"exec_target"`
		Env                 []workload.EnvVarSpec `json:"env"`
		SuiteArchiveName    string                `json:"suite_archive_name"`
		Modes               []string              `json:"modes"`
		OutputContract      map[string]any        `json:"output_contract"`
		LeaksExpectedValues bool                  `json:"leaks_expected_values"`
	} `json:"judger"`
	Security  toolManifestSecurity  `json:"security"`
	Selftest  map[string]any        `json:"selftest"`
	Resources toolManifestResources `json:"resources"`
}

type platformInfraManifest struct {
	SchemaVersion                      int `json:"schema_version"`
	Category, Name, Image, Description string
	Labels                             map[string]string    `json:"labels"`
	Source                             map[string]any       `json:"source"`
	Upstream                           map[string]any       `json:"upstream"`
	DataDriven                         bool                 `json:"data_driven"`
	EnvKeys                            map[string]any       `json:"env_keys"`
	SecurityExceptions                 []map[string]any     `json:"security_exceptions"`
	StudentAccess                      map[string]any       `json:"student_access"`
	Build                              map[string]any       `json:"build"`
	PrivateDelivery                    map[string]any       `json:"private_delivery"`
	DataVolumes                        []map[string]any     `json:"data_volumes"`
	Backup                             map[string]any       `json:"backup"`
	Network                            map[string]any       `json:"network"`
	Capabilities                       manifestCapabilities `json:"capabilities"`
	Infra                              struct {
		EcoTags               []string          `json:"eco_tags"`
		Bindings              []manifestBinding `json:"bindings"`
		Roles                 []string          `json:"roles"`
		RuntimeConfigRequired bool              `json:"runtime_config_required"`
		CommandSchema         map[string]any    `json:"command_schema"`
		ResourceSpec          map[string]any    `json:"resource_spec"`
	} `json:"infra"`
	Ports           []toolManifestPort          `json:"ports"`
	Security        toolManifestSecurity        `json:"security"`
	Resources       toolManifestResources       `json:"resources"`
	Selftest        map[string]any              `json:"selftest"`
	SupplyChain     map[string]any              `json:"supply_chain"`
	SecretsRequired []manifestSecretRequirement `json:"secrets_required"`
}

// manifestSecretRequirement 描述 manifest 对平台 Secret 键的最小运行期引用。
type manifestSecretRequirement struct {
	Component    string `json:"component"`
	Env          string `json:"env"`
	SecretKey    string `json:"secret_key"`
	Source       string `json:"source"`
	RuntimeValue string `json:"runtime_value"`
}

// seedPlatformCatalog 是正式迁移初始化的一部分,与验收业务夹具完全分离。
func seedPlatformCatalog(ctx context.Context, database *db.DB) error {
	root, err := platformImagesRoot()
	if err != nil {
		return err
	}
	if err := database.WithPrivilegedTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := syncPlatformRuntime(ctx, tx, root); err != nil {
			return err
		}
		if err := syncPlatformTools(ctx, tx, root); err != nil {
			return err
		}
		if err := syncPlatformInfra(ctx, tx, root); err != nil {
			return err
		}
		return invalidatePlatformCompositionPrepull(ctx, tx)
	}); err != nil {
		return err
	}
	return syncPlatformJudgers(ctx, database, root)
}

// platformImagesRoot 从当前工作目录向上定位仓库 images 根目录。
func platformImagesRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("读取当前工作目录失败: %w", err)
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "images")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", fmt.Errorf("未找到 images 目录")
}

// readPlatformManifest 读取并解析平台镜像 manifest。
func readPlatformManifest(path string, out any) error {
	raw, err := readSeedFile(path)
	if err != nil {
		return fmt.Errorf("读取 manifest 失败 %s: %w", path, err)
	}
	if err := yaml.UnmarshalStrict(raw, out); err != nil {
		return fmt.Errorf("解析 manifest 失败 %s: %w", path, err)
	}
	return nil
}

// platformProof 只接受当前供应链证明解析出的不可变摘要地址。
func platformProof(image string) (string, bool) {
	url, err := verifiedImageURL(image)
	return url, err == nil && strings.Contains(url, "@sha256:")
}

// manifestDeployable 读取供应链 manifest 的发布门禁。
func manifestDeployable(supply map[string]any) bool {
	value, ok := supply["deployable"]
	if !ok {
		return true
	}
	flag, ok := value.(bool)
	return !ok || flag
}

// manifestBlockReason 读取 manifest 声明的不可部署原因。
func manifestBlockReason(supply map[string]any) string {
	value, ok := supply["block_reason"]
	if !ok {
		return ""
	}
	reason, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(reason)
}

// syncPlatformRuntime 同步全部运行时目录。基础镜像/工作负载门禁与原生链动作门禁分离；只有 manifest 明确声明不可部署时才停用。
func syncPlatformRuntime(ctx context.Context, tx pgx.Tx, root string) error {
	entries, err := os.ReadDir(filepath.Join(root, "runtime"))
	if err != nil {
		return fmt.Errorf("读取运行时目录失败: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, "runtime", entry.Name(), "manifest.yaml")
		var m platformRuntimeManifest
		if err := readPlatformManifest(path, &m); err != nil {
			return err
		}
		if m.Category != "runtime" || m.Name != entry.Name() || m.Image != "runtime/"+entry.Name() || m.Runtime.Eco == "" {
			return fmt.Errorf("运行时 manifest 标识不一致: %s", path)
		}
		imageURL, proven := platformProof(m.Image)
		status := int16(2)
		selftestStatus := int16(1)
		reason := "运行时镜像尚未完成平台自检"
		adapterSpec := map[string]any{"disabled_reason": reason}
		if !manifestDeployable(m.SupplyChain) {
			status = 3
			selftestStatus = 3
			reason = manifestBlockReason(m.SupplyChain)
			if reason == "" {
				reason = "镜像 manifest 已声明不可部署"
			}
			adapterSpec = map[string]any{"disabled_reason": reason}
		} else if !proven {
			status = 2
			reason = "运行时镜像缺少当前环境不可变证明"
			adapterSpec = map[string]any{"disabled_reason": reason}
		} else {
			workspaceImage, workspaceProven := platformProof("tool/workspace")
			archiveImage, archiveProven := platformProof("base/judge-min")
			if !workspaceProven {
				reason = "平台工作区能力镜像缺少不可变证明"
				adapterSpec = map[string]any{"disabled_reason": reason}
			} else if !archiveProven {
				reason = "平台私有归档能力镜像缺少不可变证明"
				adapterSpec = map[string]any{"disabled_reason": reason}
			} else {
				adapterSpec, err = genericRuntimeAdapterSpec(m, imageURL, workspaceImage, archiveImage)
				if err != nil {
					return fmt.Errorf("生成运行时 %s 适配器失败: %w", m.Name, err)
				}
				if m.Runtime.AdapterLevel >= sandbox.RuntimeAdapterLevelStandard {
					if capabilityReason := runtimeManifestCapabilityReason(m); capabilityReason != "" {
						reason = capabilityReason
					}
				}
			}
		}
		spec, err := json.Marshal(adapterSpec)
		if err != nil {
			return fmt.Errorf("编码运行时适配器状态失败: %w", err)
		}
		result := "pending"
		if status == 3 {
			result = "blocked"
		}
		detail, err := json.Marshal(map[string]any{"result": result, "reason": reason})
		if err != nil {
			return fmt.Errorf("编码运行时接入详情失败: %w", err)
		}
		runtimeID := platformCatalogRuntimeID(m.Name)
		imageID := platformCatalogRuntimeImageID(m.Name)
		capabilityImpl := runtimeCapabilityImpl(m)
		if err := execJSON(ctx, tx, `INSERT INTO runtime (id,code,name,eco,adapter_level,adapter_spec,capability_impl,selftest_status,selftest_detail,status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name,eco=EXCLUDED.eco,adapter_level=EXCLUDED.adapter_level,adapter_spec=EXCLUDED.adapter_spec,capability_impl=EXCLUDED.capability_impl,selftest_status=CASE WHEN runtime.adapter_spec IS DISTINCT FROM EXCLUDED.adapter_spec OR runtime.adapter_level IS DISTINCT FROM EXCLUDED.adapter_level OR runtime.capability_impl IS DISTINCT FROM EXCLUDED.capability_impl THEN EXCLUDED.selftest_status ELSE runtime.selftest_status END,selftest_detail=CASE WHEN runtime.adapter_spec IS DISTINCT FROM EXCLUDED.adapter_spec OR runtime.adapter_level IS DISTINCT FROM EXCLUDED.adapter_level OR runtime.capability_impl IS DISTINCT FROM EXCLUDED.capability_impl THEN EXCLUDED.selftest_detail ELSE runtime.selftest_detail END,status=CASE WHEN runtime.adapter_spec IS DISTINCT FROM EXCLUDED.adapter_spec OR runtime.adapter_level IS DISTINCT FROM EXCLUDED.adapter_level OR runtime.capability_impl IS DISTINCT FROM EXCLUDED.capability_impl THEN EXCLUDED.status ELSE runtime.status END,updated_at=now()`, runtimeID, m.Name, m.Description, m.Runtime.Eco, m.Runtime.AdapterLevel, spec, capabilityImpl, selftestStatus, detail, status); err != nil {
			return fmt.Errorf("同步运行时 %s 失败: %w", m.Name, err)
		}
		if !proven {
			if err := execJSON(ctx, tx, `DELETE FROM runtime_image WHERE id=$1 AND runtime_id=$2`, imageID, runtimeID); err != nil {
				return fmt.Errorf("删除未证明的运行时镜像 %s 失败: %w", m.Name, err)
			}
			continue
		}
		imageStatus := int16(1)
		if status == 3 {
			imageStatus = 2
		}
		genesisBaked := m.GenesisBaked
		// 镜像 digest 更新会改变 version,稳定主键冲突时必须整体更新原记录。
		if err := execJSON(ctx, tx, `INSERT INTO runtime_image (id,runtime_id,image_url,version,status,genesis_baked) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO UPDATE SET runtime_id=EXCLUDED.runtime_id,image_url=EXCLUDED.image_url,version=EXCLUDED.version,status=EXCLUDED.status,genesis_baked=EXCLUDED.genesis_baked`, imageID, runtimeID, imageURL, imageVersionFromURL(imageURL), imageStatus, genesisBaked); err != nil {
			return fmt.Errorf("同步运行时镜像 %s 失败: %w", m.Name, err)
		}
	}
	return nil
}

// syncPlatformTools 复用已有 manifest 规范化逻辑，证明或契约缺失时明确停用。
func syncPlatformTools(ctx context.Context, tx pgx.Tx, root string) error {
	entries, err := os.ReadDir(filepath.Join(root, "tool"))
	if err != nil {
		return fmt.Errorf("读取工具目录失败: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m, err := readToolManifest(filepath.Join(root, "tool", entry.Name(), "manifest.yaml"))
		if err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(m.Labels["chaimir.io/exposure"]), "platform-private") {
			continue
		}
		status := int16(2)
		reason := manifestBlockReason(m.SupplyChain)
		if reason == "" {
			reason = "镜像证明尚未通过平台门禁"
		}
		resource := map[string]any{"disabled_reason": reason}
		if imageURL, proven := platformProof(m.Image); proven && manifestDeployable(m.SupplyChain) {
			kind, kindErr := toolKindFromManifest(m.Tool.Kind)
			if kindErr == nil {
				if generated, generateErr := toolResourceSpecFromManifest(m, imageURL, kind); generateErr == nil {
					resource, status = generated, toolStatusFromManifest(m)
				} else {
					resource = map[string]any{"disabled_reason": "工具工作负载声明不符合当前契约"}
				}
			}
		}
		resourceJSON, err := json.Marshal(resource)
		if err != nil {
			return err
		}
		if err := execJSON(ctx, tx, `INSERT INTO tool (id,code,name,category,kind,eco_tags,resource_spec,status) VALUES ($1,$2,$3,'tool',$4,$5,$6,$7) ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name,category=EXCLUDED.category,kind=EXCLUDED.kind,eco_tags=EXCLUDED.eco_tags,resource_spec=EXCLUDED.resource_spec,status=EXCLUDED.status,updated_at=now()`, platformCatalogToolID(m.Name), m.Name, manifestDisplayName(m), toolKindOrDisabled(m), strings.Join(m.Tool.EcoTags, ","), resourceJSON, status); err != nil {
			return fmt.Errorf("同步工具 %s 失败: %w", m.Name, err)
		}
	}
	return nil
}

// syncPlatformInfra 将 sandbox 范围的基础设施 manifest 转换为现有 tool 表中的 infra 组件。
// cluster 范围的控制面组件仍由部署目录管理,不得进入教师沙箱编排目录。
func syncPlatformInfra(ctx context.Context, tx pgx.Tx, root string) error {
	entries, err := os.ReadDir(filepath.Join(root, "infra"))
	if err != nil {
		return fmt.Errorf("读取基础设施目录失败: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, "infra", entry.Name(), "manifest.yaml")
		var manifest platformInfraManifest
		if err := readPlatformManifest(path, &manifest); err != nil {
			return err
		}
		if manifest.Category != "infra" || manifest.Name != entry.Name() || manifest.Image != "infra/"+entry.Name() {
			return fmt.Errorf("基础设施 manifest 标识不一致: %s", path)
		}
		if strings.TrimSpace(manifest.Labels["chaimir.io/scope"]) == "cluster" {
			continue
		}
		imageURL, proven := platformProof(manifest.Image)
		status := int16(2)
		reason := manifestBlockReason(manifest.SupplyChain)
		if reason == "" {
			reason = "基础设施镜像尚未通过平台门禁"
		}
		resource := map[string]any{"disabled_reason": reason}
		if proven && manifestDeployable(manifest.SupplyChain) {
			generated, generateErr := infraResourceSpecFromManifest(manifest, imageURL)
			if generateErr != nil {
				return fmt.Errorf("基础设施 %s 工作负载声明无效: %w", manifest.Name, generateErr)
			}
			resource = generated
			status = 1
		}
		resourceJSON, err := json.Marshal(resource)
		if err != nil {
			return fmt.Errorf("编码基础设施 %s 工作负载失败: %w", manifest.Name, err)
		}
		if err := execJSON(ctx, tx, `INSERT INTO tool (id,code,name,category,kind,eco_tags,resource_spec,status) VALUES ($1,$2,$3,'infra',$4,$5,$6,$7) ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name,category=EXCLUDED.category,kind=EXCLUDED.kind,eco_tags=EXCLUDED.eco_tags,resource_spec=EXCLUDED.resource_spec,status=EXCLUDED.status,updated_at=now()`, platformCatalogInfraID(manifest.Name), manifest.Name, manifest.Description, contracts.SandboxToolKindInfra, strings.Join(manifest.Infra.EcoTags, ","), resourceJSON, status); err != nil {
			return fmt.Errorf("同步基础设施 %s 失败: %w", manifest.Name, err)
		}
	}
	return nil
}

// infraResourceSpecFromManifest 规范化 manifest 明确声明的基础设施 WorkloadSpec。
// 基础设施启动命令、环境、挂载和探针都属于镜像契约，不能由 seed 隐式合成。
func infraResourceSpecFromManifest(manifest platformInfraManifest, imageURL string) (map[string]any, error) {
	if len(manifest.Infra.ResourceSpec) == 0 {
		return nil, fmt.Errorf("基础设施必须声明 infra.resource_spec")
	}
	spec, err := normalizeExplicitToolResourceSpec(manifest.Infra.ResourceSpec, imageURL, contracts.SandboxToolKindInfra)
	if err != nil {
		return nil, err
	}
	command, err := platformManifestSelftestCommand(manifest.Selftest)
	if err != nil {
		return nil, err
	}
	if len(command) == 0 {
		return nil, fmt.Errorf("基础设施必须声明 selftest.commands")
	}
	if err := appendManifestSecretEnv(spec, manifest.SecretsRequired); err != nil {
		return nil, err
	}
	componentData, err := json.Marshal(spec["components"])
	if err != nil {
		return nil, fmt.Errorf("编码基础设施组件失败: %w", err)
	}
	var components []workload.ComponentSpec
	if err := json.Unmarshal(componentData, &components); err != nil || len(components) == 0 {
		return nil, fmt.Errorf("基础设施 WorkloadSpec 缺少组件: %s", manifest.Name)
	}
	for index := range components {
		if len(components[index].PrepullCommand) == 0 {
			components[index].PrepullCommand = append([]string(nil), command...)
		}
	}
	spec["components"] = components
	spec["prepull_command"] = command
	spec["bindings"] = append([]manifestBinding(nil), manifest.Infra.Bindings...)
	spec["capabilities"] = map[string]any{
		"provides":       append([]string(nil), manifest.Capabilities.Provides...),
		"requires":       append([]string(nil), manifest.Capabilities.Requires...),
		"conflicts":      append([]string(nil), manifest.Capabilities.Conflicts...),
		"cardinality":    manifest.Capabilities.Cardinality,
		"placement":      manifest.Capabilities.Placement,
		"config_schema":  manifest.Capabilities.ConfigSchema,
		"student_access": manifest.Capabilities.StudentAccess,
	}
	if err := validateGeneratedToolResourceSpec(spec, contracts.SandboxToolKindInfra); err != nil {
		return nil, err
	}
	return spec, nil
}

func platformManifestSelftestCommand(raw map[string]any) ([]string, error) {
	commandsRaw, ok := raw["commands"]
	if !ok {
		return nil, nil
	}
	data, err := json.Marshal(commandsRaw)
	if err != nil {
		return nil, fmt.Errorf("编码 selftest.commands 失败: %w", err)
	}
	var commands []toolManifestSelftestCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		return nil, fmt.Errorf("selftest.commands 结构非法: %w", err)
	}
	if len(commands) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(commands[0].Command))
	for _, part := range commands[0].Command {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("selftest.commands 首条命令包含空参数")
		}
		out = append(out, part)
	}
	return out, nil
}

// secretEnvFromManifest 将 manifest 的密钥需求转换为受控 SecretKeyRef 声明。
func secretEnvFromManifest(requirements []manifestSecretRequirement, component string) ([]workload.SecretEnvVarSpec, error) {
	component = strings.TrimSpace(component)
	if component == "" {
		return nil, fmt.Errorf("Secret 注入目标组件不能为空")
	}
	out := make([]workload.SecretEnvVarSpec, 0, len(requirements))
	seen := map[string]struct{}{}
	for _, requirement := range requirements {
		if declared := strings.TrimSpace(requirement.Component); declared == "" || declared != component {
			return nil, fmt.Errorf("Secret %s 必须显式归属组件 %s", strings.TrimSpace(requirement.Env), component)
		}
		name := strings.TrimSpace(requirement.Env)
		secretKey := strings.TrimSpace(requirement.SecretKey)
		if name == "" || secretKey == "" {
			return nil, fmt.Errorf("Secret 引用必须同时声明 env 和 secret_key")
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("Secret 环境变量重复声明: %s", name)
		}
		seen[name] = struct{}{}
		out = append(out, workload.SecretEnvVarSpec{Name: name, SecretName: "chaimir-secret", SecretKey: secretKey})
	}
	return out, nil
}

func toolKindOrDisabled(m toolManifest) int16 {
	kind, err := toolKindFromManifest(m.Tool.Kind)
	if err != nil {
		return contracts.SandboxToolKindCommand
	}
	return kind
}

// invalidatePlatformCompositionPrepull 仅撤销与已发布快照闭包不一致的组合证明。
func invalidatePlatformCompositionPrepull(ctx context.Context, tx pgx.Tx) error {
	detail, err := json.Marshal(map[string]any{"stage": "invalidated", "reason": "platform_catalog_changed", "subject": "platform-catalog-seed"})
	if err != nil {
		return fmt.Errorf("编码组合预拉取失效原因失败: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE sandbox_composition_prepull AS prepull
SET status=1, detail=$1, completed_at=NULL, updated_at=now()
WHERE prepull.status<>1
  AND NOT EXISTS (
    SELECT 1
    FROM sandbox_composition AS composition
    WHERE composition.composition_digest=prepull.composition_digest
      AND composition.status=1
      AND composition.snapshot->'image_closure' = prepull.image_closure
  )`, detail); err != nil {
		return fmt.Errorf("撤销组合预拉取证明失败: %w", err)
	}
	return nil
}

// splitPlatformCSV 解析数据库中的生态标签列表并忽略空项。
func splitPlatformCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// syncPlatformJudgers 通过 M2 唯一编译器冻结平台判题器环境，绝不在迁移中手写快照。
func syncPlatformJudgers(ctx context.Context, database *db.DB, root string) error {
	entries, err := os.ReadDir(filepath.Join(root, "judger"))
	if err != nil {
		return fmt.Errorf("读取判题器目录失败: %w", err)
	}
	type catalogJudger struct {
		id             int64
		code           string
		name           string
		typ            int16
		executorRef    string
		runtimeNeeded  bool
		defaultTimeout int32
		resource       judge.JudgerResourceSpec
	}
	store := sandbox.NewStore(database)
	var items []catalogJudger
	genesisRefs := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		var m platformJudgerManifest
		path := filepath.Join(root, "judger", entry.Name(), "manifest.yaml")
		if err := readPlatformManifest(path, &m); err != nil {
			return err
		}
		if m.Category != "judger" || m.Name != entry.Name() || m.Image != "judger/"+entry.Name() {
			return fmt.Errorf("判题器 manifest 标识不一致: %s", path)
		}
		genesisRefs[m.Name] = strings.TrimSpace(m.Judger.GenesisRef)
		imageURL, proven := platformProof(m.Image)
		if !manifestDeployable(m.SupplyChain) {
			return fmt.Errorf("判题器 %s 已声明不可部署,必须先完成安全替代实现", m.Name)
		}
		resource := judge.JudgerResourceSpec{}
		composition, execution, resourceErr := platformJudgerResourceSpec(m, imageURL)
		if resourceErr != nil {
			return fmt.Errorf("生成判题器 %s 执行契约失败: %w", m.Name, resourceErr)
		}
		// Persist the manifest execution contract even while its runtime is not ready.
		// The missing composition is then explicit and fail-closed instead of looking like
		// a successfully compiled but empty snapshot.
		resource = execution
		if proven {
			ready, readyErr := platformRuntimeReadyForJudger(ctx, database, composition.Runtimes[0].Code)
			if readyErr != nil {
				return fmt.Errorf("读取判题器 %s 运行时状态失败: %w", m.Name, readyErr)
			}
			if ready {
				snapshot, compileErr := sandbox.CompilePlatformSandboxComposition(ctx, store, composition)
				if compileErr != nil {
					return fmt.Errorf("编译判题器 %s 组合快照失败: %w", m.Name, compileErr)
				}
				snapshot.PrivateSidecars = platformPrivateSidecars(execution.ExecutionSidecars)
				if err := contracts.AppendPrivateSidecarImageClosure(&snapshot); err != nil {
					return fmt.Errorf("补齐判题器 %s 私有 sidecar 镜像闭包失败: %w", m.Name, err)
				}
				digest, digestErr := contracts.CanonicalSnapshotDigest(snapshot)
				if digestErr != nil {
					return fmt.Errorf("计算判题器 %s 私有 sidecar 组合摘要失败: %w", m.Name, digestErr)
				}
				snapshot.Digest = digest
				execution.CompositionSnapshot = snapshot
				resource = execution
			} else {
				resource.DisabledReason = fmt.Sprintf("运行时 %s 尚未完成平台自检", composition.Runtimes[0].Code)
			}
		} else {
			resource.DisabledReason = "判题器镜像缺少当前环境不可变证明"
		}
		items = append(items, catalogJudger{id: platformJudgerID(m.Name), code: m.Name, name: m.Description, typ: platformJudgerType(m.Judger.Type), executorRef: m.Name, runtimeNeeded: strings.TrimSpace(m.Judger.RuntimeCode) != "", defaultTimeout: m.Resources.TimeoutSeconds, resource: resource})
	}
	onchainResource := judge.JudgerResourceSpec{}
	onchainComposition := contracts.SandboxCompositionSpec{ID: "judge:onchain-assert", Runtimes: []contracts.CompositionRuntimeRef{{InstanceCode: "source-chain", Code: "evm-foundry", ImageVersion: platformRuntimeImageVersion("evm-foundry")}}, WorkspaceRuntimeInstance: "source-chain", AccessProfile: contracts.SandboxAccessJudgePrivate}
	ready, err := platformRuntimeReadyForJudger(ctx, database, onchainComposition.Runtimes[0].Code)
	if err != nil {
		return fmt.Errorf("读取链上断言运行时状态失败: %w", err)
	}
	if ready {
		snapshot, compileErr := sandbox.CompilePlatformSandboxComposition(ctx, store, onchainComposition)
		if compileErr != nil {
			return fmt.Errorf("编译链上断言组合快照失败: %w", compileErr)
		}
		genesisRef := strings.TrimSpace(genesisRefs["testcase-evm"])
		if genesisRef == "" {
			return fmt.Errorf("onchain-assert 缺少 testcase-evm manifest 的 genesis_ref")
		}
		onchainResource = judge.JudgerResourceSpec{CompositionSnapshot: snapshot, GenesisRef: genesisRef, TimeoutSec: 60, MaxRetries: 1}
	}
	items = append(items, catalogJudger{id: platformJudgerID("onchain-assert"), code: "onchain-assert", name: "链上状态断言判题器", typ: 2, executorRef: "m3-backend-strategy", runtimeNeeded: true, defaultTimeout: 60, resource: onchainResource})
	return database.WithPrivilegedTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		for _, item := range items {
			resourceJSON, marshalErr := json.Marshal(item.resource)
			if marshalErr != nil {
				return fmt.Errorf("编码判题器 %s 执行事实失败: %w", item.code, marshalErr)
			}
			if err := execJSON(ctx, tx, `INSERT INTO judger (id,code,name,type,executor_ref,runtime_required,default_timeout_sec,resource_spec,selftest_status,status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,1,2) ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name,type=EXCLUDED.type,executor_ref=EXCLUDED.executor_ref,runtime_required=EXCLUDED.runtime_required,default_timeout_sec=EXCLUDED.default_timeout_sec,resource_spec=EXCLUDED.resource_spec,selftest_status=CASE WHEN judger.resource_spec IS DISTINCT FROM EXCLUDED.resource_spec THEN 1 ELSE judger.selftest_status END,status=CASE WHEN judger.resource_spec IS DISTINCT FROM EXCLUDED.resource_spec THEN 2 ELSE judger.status END,updated_at=now()`, item.id, item.code, item.name, item.typ, item.executorRef, item.runtimeNeeded, item.defaultTimeout, resourceJSON); err != nil {
				return fmt.Errorf("同步判题器 %s 失败: %w", item.code, err)
			}
		}
		return nil
	})
}

// platformJudgerManifestForCode 从镜像 manifest 读取指定判题器,保证业务种子复用同一材料契约。
func platformJudgerManifestForCode(root, code string) (platformJudgerManifest, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return platformJudgerManifest{}, fmt.Errorf("判题器 code 不能为空")
	}
	path := filepath.Join(root, "judger", code, "manifest.yaml")
	var manifest platformJudgerManifest
	if err := readPlatformManifest(path, &manifest); err != nil {
		return platformJudgerManifest{}, err
	}
	if manifest.Category != "judger" || manifest.Name != code || manifest.Image != "judger/"+code {
		return platformJudgerManifest{}, fmt.Errorf("判题器 manifest 标识不一致: %s", path)
	}
	if strings.TrimSpace(manifest.Judger.GenesisRef) == "" {
		return platformJudgerManifest{}, fmt.Errorf("判题器 %s 未声明 genesis_ref", code)
	}
	return manifest, nil
}

// platformRuntimeReadyForJudger 判断运行时是否已完成真实平台自检，未通过时不得冻结判题器快照。
func platformRuntimeReadyForJudger(ctx context.Context, database *db.DB, code string) (bool, error) {
	var status, selftestStatus int16
	err := database.WithPrivilegedTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT status,selftest_status FROM runtime WHERE code=$1`, strings.TrimSpace(code)).Scan(&status, &selftestStatus)
	})
	if err != nil {
		return false, err
	}
	return status == 1 && selftestStatus == 2, nil
}

// platformJudgerResourceSpec 把判题器 manifest 转成待编译的组合声明和受控执行策略。
func platformJudgerResourceSpec(manifest platformJudgerManifest, imageURL string) (contracts.SandboxCompositionSpec, judge.JudgerResourceSpec, error) {
	if strings.TrimSpace(manifest.Judger.Type) == "" {
		return contracts.SandboxCompositionSpec{}, judge.JudgerResourceSpec{}, fmt.Errorf("manifest 未声明 judger.type")
	}
	runtimeCode := strings.TrimSpace(manifest.Judger.RuntimeCode)
	if runtimeCode == "" {
		return contracts.SandboxCompositionSpec{}, judge.JudgerResourceSpec{}, fmt.Errorf("manifest 未声明 judger.runtime_code")
	}
	if !workload.ValidNonShellCommand(manifest.Judger.Command) {
		return contracts.SandboxCompositionSpec{}, judge.JudgerResourceSpec{}, fmt.Errorf("manifest 未声明有效 judger.command")
	}
	genesisRef := strings.TrimSpace(manifest.Judger.GenesisRef)
	if genesisRef == "" {
		return contracts.SandboxCompositionSpec{}, judge.JudgerResourceSpec{}, fmt.Errorf("manifest 未声明 judger.genesis_ref")
	}
	execTarget := strings.TrimSpace(manifest.Judger.ExecTarget)
	if execTarget == "" {
		return contracts.SandboxCompositionSpec{}, judge.JudgerResourceSpec{}, fmt.Errorf("manifest 未声明 judger.exec_target")
	}
	if manifest.Resources.TimeoutSeconds <= 0 {
		return contracts.SandboxCompositionSpec{}, judge.JudgerResourceSpec{}, fmt.Errorf("manifest 未声明有效 resources.timeout_seconds")
	}
	suiteName := strings.TrimSpace(manifest.Judger.SuiteArchiveName)
	if suiteName == "" {
		return contracts.SandboxCompositionSpec{}, judge.JudgerResourceSpec{}, fmt.Errorf("manifest 未声明 judger.suite_archive_name")
	}
	env := append([]workload.EnvVarSpec(nil), manifest.Judger.Env...)
	readOnly := manifest.Security.ReadOnlyRootFilesystem
	component := workload.ComponentSpec{
		Name:                   manifest.Name,
		ImageURL:               imageURL,
		Command:                []string{"sleep", "2147483647"},
		Env:                    env,
		Workdir:                "/judge",
		Resources:              workload.ResourceSpec{Requests: map[string]string{"cpu": manifest.Resources.CPURequest, "memory": manifest.Resources.MemoryRequest}, Limits: map[string]string{"cpu": manifest.Resources.CPULimit, "memory": manifest.Resources.MemoryLimit}},
		ReadOnlyRootFilesystem: &readOnly,
		Labels:                 map[string]string{"chaimir.io/student-access": "false", "chaimir.io/sensitivity": "judge-private"},
		MountWorkspace:         func() *bool { value := true; return &value }(),
		MountDomains:           []string{sandbox.VolumeDomainJudgePrivate},
		EphemeralMounts:        []workload.EphemeralMountSpec{{Name: "judge-workdir", MountPath: "/judge"}, {Name: "judge-tmp", MountPath: "/tmp"}},
		PrepullCommand:         []string{"sleep", "1"},
	}
	return contracts.SandboxCompositionSpec{ID: "judge:" + manifest.Name, Runtimes: []contracts.CompositionRuntimeRef{{InstanceCode: "source-chain", Code: runtimeCode, ImageVersion: platformRuntimeImageVersion(runtimeCode)}}, WorkspaceRuntimeInstance: "source-chain", AccessProfile: contracts.SandboxAccessJudgePrivate}, judge.JudgerResourceSpec{
		GenesisRef:        genesisRef,
		Command:           append([]string(nil), manifest.Judger.Command...),
		ExecTarget:        execTarget,
		ExecutionSidecars: []workload.ComponentSpec{component},
		TimeoutSec:        manifest.Resources.TimeoutSeconds,
		MaxRetries:        1,
		SuiteArchiveName:  suiteName,
		Selftest:          manifest.Selftest,
	}, nil
}

// platformPrivateSidecars 将迁移生成的判题 workload 固化到组合快照,使镜像闭包和运行时创建使用同一事实源。
func platformPrivateSidecars(items []workload.ComponentSpec) []contracts.SandboxPrivateSidecarSpec {
	out := make([]contracts.SandboxPrivateSidecarSpec, 0, len(items))
	for _, item := range items {
		env := make([]contracts.SandboxEnvVarSpec, 0, len(item.Env))
		for _, value := range item.Env {
			env = append(env, contracts.SandboxEnvVarSpec{Name: value.Name, Value: value.Value})
		}
		mounts := make([]contracts.SandboxEphemeralMountSpec, 0, len(item.EphemeralMounts))
		for _, mount := range item.EphemeralMounts {
			mounts = append(mounts, contracts.SandboxEphemeralMountSpec{Name: mount.Name, MountPath: mount.MountPath})
		}
		out = append(out, contracts.SandboxPrivateSidecarSpec{
			Name: item.Name, ImageURL: item.ImageURL, Command: append([]string(nil), item.Command...), Args: append([]string(nil), item.Args...),
			PrepullCommand: append([]string(nil), item.PrepullCommand...), PrepullHold: item.PrepullHold, Env: env,
			Resources: contracts.SandboxResourceSpec{Requests: clonePlatformStringMap(item.Resources.Requests), Limits: clonePlatformStringMap(item.Resources.Limits)},
			Workdir:   item.Workdir, ReadOnlyRootFilesystem: item.ReadOnlyRootFilesystem, Labels: clonePlatformStringMap(item.Labels),
			MountWorkspace: item.MountWorkspace, MountDomains: append([]string(nil), item.MountDomains...), EphemeralMounts: mounts,
		})
	}
	return out
}

func clonePlatformStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func platformJudgerType(raw string) int16 {
	if strings.EqualFold(strings.TrimSpace(raw), "static-scan") {
		return 4
	}
	return 1
}

func platformJudgerID(code string) int64 {
	return platformStableID("judger", code)
}
