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
	Category, Name, Image, Description string
	Runtime                            struct {
		Eco          string `json:"eco"`
		AdapterLevel int16  `json:"adapter_level"`
		Native       struct {
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
	Category, Name, Image, Description string
	SupplyChain                        map[string]any `json:"supply_chain"`
	Judger                             struct {
		Type             string                `json:"type"`
		RuntimeCode      string                `json:"runtime_code"`
		GenesisRef       string                `json:"genesis_ref"`
		Command          []string              `json:"command"`
		ExecTarget       string                `json:"exec_target"`
		Env              []workload.EnvVarSpec `json:"env"`
		SuiteArchiveName string                `json:"suite_archive_name"`
	} `json:"judger"`
	Security  toolManifestSecurity  `json:"security"`
	Selftest  map[string]any        `json:"selftest"`
	Resources toolManifestResources `json:"resources"`
}

type platformInfraManifest struct {
	Category, Name, Image, Description string
	Labels                             map[string]string    `json:"labels"`
	Capabilities                       manifestCapabilities `json:"capabilities"`
	Infra                              struct {
		EcoTags          []string `json:"eco_tags"`
		RequiredBindings []string `json:"required_bindings"`
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
	Env          string `json:"env"`
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
	if err := yaml.Unmarshal(raw, out); err != nil {
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
			if !workspaceProven {
				reason = "平台工作区能力镜像缺少不可变证明"
				adapterSpec = map[string]any{"disabled_reason": reason}
			} else {
				adapterSpec, err = genericRuntimeAdapterSpec(m, imageURL, workspaceImage)
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
		if err := execJSON(ctx, tx, `INSERT INTO runtime (id,code,name,eco,adapter_level,adapter_spec,capability_impl,selftest_status,selftest_detail,status) VALUES ($1,$2,$3,$4,$5,$6,'sandbox-exec',$7,$8,$9) ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name,eco=EXCLUDED.eco,adapter_level=EXCLUDED.adapter_level,adapter_spec=EXCLUDED.adapter_spec,selftest_status=CASE WHEN runtime.adapter_spec IS DISTINCT FROM EXCLUDED.adapter_spec OR runtime.adapter_level IS DISTINCT FROM EXCLUDED.adapter_level THEN EXCLUDED.selftest_status ELSE runtime.selftest_status END,selftest_detail=CASE WHEN runtime.adapter_spec IS DISTINCT FROM EXCLUDED.adapter_spec OR runtime.adapter_level IS DISTINCT FROM EXCLUDED.adapter_level THEN EXCLUDED.selftest_detail ELSE runtime.selftest_detail END,status=CASE WHEN runtime.adapter_spec IS DISTINCT FROM EXCLUDED.adapter_spec OR runtime.adapter_level IS DISTINCT FROM EXCLUDED.adapter_level THEN EXCLUDED.status ELSE runtime.status END,updated_at=now()`, runtimeID, m.Name, m.Description, m.Runtime.Eco, m.Runtime.AdapterLevel, spec, selftestStatus, detail, status); err != nil {
			return fmt.Errorf("同步运行时 %s 失败: %w", m.Name, err)
		}
		if strings.TrimSpace(imageURL) == "" {
			imageURL = "unverified://" + m.Image
		}
		imageStatus := int16(1)
		if status == 3 || !proven {
			imageStatus = 2
		}
		if err := execJSON(ctx, tx, `INSERT INTO runtime_image (id,runtime_id,image_url,version,status,genesis_baked) VALUES ($1,$2,$3,$4,$5,false) ON CONFLICT (runtime_id,version) DO UPDATE SET image_url=EXCLUDED.image_url,status=EXCLUDED.status,genesis_baked=false`, imageID, runtimeID, imageURL, imageVersionFromURL(imageURL), imageStatus); err != nil {
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

// infraResourceSpecFromManifest 生成基础设施组件的唯一 WorkloadSpec。
func infraResourceSpecFromManifest(manifest platformInfraManifest, imageURL string) (map[string]any, error) {
	command, err := platformManifestSelftestCommand(manifest.Selftest)
	if err != nil {
		return nil, err
	}
	if len(command) == 0 {
		return nil, fmt.Errorf("基础设施必须声明 selftest.commands")
	}
	readOnly := manifest.Security.ReadOnlyRootFilesystem
	component := workload.ComponentSpec{
		Name:                   "infra",
		ImageURL:               imageURL,
		Resources:              workload.ResourceSpec{Requests: map[string]string{"cpu": manifest.Resources.CPURequest, "memory": manifest.Resources.MemoryRequest}, Limits: map[string]string{"cpu": manifest.Resources.CPULimit, "memory": manifest.Resources.MemoryLimit}},
		ReadOnlyRootFilesystem: &readOnly,
		MountWorkspace:         func() *bool { value := false; return &value }(),
		PrepullCommand:         command,
		SecretEnv:              secretEnvFromManifest(manifest.SecretsRequired),
	}
	for _, port := range manifest.Ports {
		if port.ContainerPort <= 0 || strings.TrimSpace(port.Name) == "" {
			return nil, fmt.Errorf("基础设施端口声明无效")
		}
		component.Ports = append(component.Ports, workload.PortSpec{Name: port.Name, ContainerPort: port.ContainerPort, ServicePort: port.ContainerPort, Protocol: defaultProtocol(port.Protocol)})
	}
	for _, writable := range infraWritableMounts(manifest) {
		component.EphemeralMounts = append(component.EphemeralMounts, workload.EphemeralMountSpec{Name: writable, MountPath: "/" + writable})
	}
	spec := map[string]any{
		"components":        []workload.ComponentSpec{component},
		"prepull_command":   command,
		"required_bindings": append([]string(nil), manifest.Infra.RequiredBindings...),
		"capabilities": map[string]any{
			"provides":       append([]string(nil), manifest.Capabilities.Provides...),
			"requires":       append([]string(nil), manifest.Capabilities.Requires...),
			"conflicts":      append([]string(nil), manifest.Capabilities.Conflicts...),
			"cardinality":    manifest.Capabilities.Cardinality,
			"placement":      manifest.Capabilities.Placement,
			"config_schema":  manifest.Capabilities.ConfigSchema,
			"student_access": manifest.Capabilities.StudentAccess,
		},
	}
	services := make([]workload.ServiceSpec, 0, len(component.Ports))
	for _, port := range component.Ports {
		services = append(services, workload.ServiceSpec{Name: "infra-" + manifest.Name + "-" + port.Name, Component: component.Name, Ports: []workload.ServicePortSpec{{Name: port.Name, Port: port.ServicePort, TargetPort: port.Name, Protocol: port.Protocol}}})
	}
	if len(services) > 0 {
		spec["services"] = services
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

func infraWritableMounts(manifest platformInfraManifest) []string {
	if manifest.Security.ReadOnlyRootFilesystem {
		return []string{"runtime-state"}
	}
	return []string{"runtime-state"}
}

// secretEnvFromManifest 将 manifest 的密钥需求转换为受控 SecretKeyRef 声明。
func secretEnvFromManifest(requirements []manifestSecretRequirement) []workload.SecretEnvVarSpec {
	out := make([]workload.SecretEnvVarSpec, 0, len(requirements))
	seen := map[string]struct{}{}
	for _, requirement := range requirements {
		name := strings.TrimSpace(requirement.Env)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, workload.SecretEnvVarSpec{Name: name, SecretName: "chaimir-secret", SecretKey: name})
	}
	return out
}

func toolKindOrDisabled(m toolManifest) int16 {
	kind, err := toolKindFromManifest(m.Tool.Kind)
	if err != nil {
		return contracts.SandboxToolKindCommand
	}
	return kind
}

// invalidatePlatformCompositionPrepull 目录重同步后撤销旧组合证明,避免已变更组件继续被启动。
func invalidatePlatformCompositionPrepull(ctx context.Context, tx pgx.Tx) error {
	detail, err := json.Marshal(map[string]any{"stage": "invalidated", "reason": "platform_catalog_changed", "subject": "platform-catalog-seed"})
	if err != nil {
		return fmt.Errorf("编码组合预拉取失效原因失败: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE sandbox_composition_prepull SET status=1,detail=$1,completed_at=NULL,updated_at=now() WHERE status<>1`, detail); err != nil {
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
	items := make([]catalogJudger, 0, len(entries)+1)
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
		imageURL, proven := platformProof(m.Image)
		if !manifestDeployable(m.SupplyChain) {
			return fmt.Errorf("判题器 %s 已声明不可部署,必须先完成安全替代实现", m.Name)
		}
		resource := judge.JudgerResourceSpec{}
		composition, execution, resourceErr := platformJudgerResourceSpec(m, imageURL)
		if resourceErr != nil {
			return fmt.Errorf("生成判题器 %s 执行契约失败: %w", m.Name, resourceErr)
		}
		if proven {
			ready, readyErr := platformRuntimeReadyForJudger(ctx, database, composition.PrimaryRuntime.Code)
			if readyErr != nil {
				return fmt.Errorf("读取判题器 %s 运行时状态失败: %w", m.Name, readyErr)
			}
			if ready {
				snapshot, compileErr := sandbox.CompilePlatformSandboxComposition(ctx, store, composition)
				if compileErr != nil {
					return fmt.Errorf("编译判题器 %s 组合快照失败: %w", m.Name, compileErr)
				}
				execution.CompositionSnapshot = snapshot
				resource = execution
			}
		}
		items = append(items, catalogJudger{id: platformJudgerID(m.Name), code: m.Name, name: m.Description, typ: platformJudgerType(m.Judger.Type), executorRef: m.Name, runtimeNeeded: strings.TrimSpace(m.Judger.RuntimeCode) != "", defaultTimeout: m.Resources.TimeoutSeconds, resource: resource})
	}
	onchainResource := judge.JudgerResourceSpec{}
	onchainComposition := contracts.SandboxCompositionSpec{ID: "judge:onchain-assert", PrimaryRuntime: contracts.CompositionRuntimeRef{Code: "evm-foundry", ImageVersion: platformRuntimeImageVersion("evm-foundry")}, AccessProfile: contracts.SandboxAccessJudgePrivate}
	ready, err := platformRuntimeReadyForJudger(ctx, database, onchainComposition.PrimaryRuntime.Code)
	if err != nil {
		return fmt.Errorf("读取链上断言运行时状态失败: %w", err)
	}
	if ready {
		snapshot, compileErr := sandbox.CompilePlatformSandboxComposition(ctx, store, onchainComposition)
		if compileErr != nil {
			return fmt.Errorf("编译链上断言组合快照失败: %w", compileErr)
		}
		onchainResource = judge.JudgerResourceSpec{CompositionSnapshot: snapshot, GenesisRef: "genesis/evm-foundry/acceptance.json", TimeoutSec: 60, MaxRetries: 1}
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
		Workdir:                "/judge-private",
		Resources:              workload.ResourceSpec{Requests: map[string]string{"cpu": manifest.Resources.CPURequest, "memory": manifest.Resources.MemoryRequest}, Limits: map[string]string{"cpu": manifest.Resources.CPULimit, "memory": manifest.Resources.MemoryLimit}},
		ReadOnlyRootFilesystem: &readOnly,
		Labels:                 map[string]string{"chaimir.io/student-access": "false", "chaimir.io/sensitivity": "judge-private"},
		MountWorkspace:         func() *bool { value := true; return &value }(),
		EphemeralMounts:        []workload.EphemeralMountSpec{{Name: "judge-workdir", MountPath: "/judge-private"}, {Name: "judge-tmp", MountPath: "/tmp"}},
	}
	return contracts.SandboxCompositionSpec{ID: "judge:" + manifest.Name, PrimaryRuntime: contracts.CompositionRuntimeRef{Code: runtimeCode, ImageVersion: platformRuntimeImageVersion(runtimeCode)}, AccessProfile: contracts.SandboxAccessJudgePrivate}, judge.JudgerResourceSpec{
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

func platformJudgerType(raw string) int16 {
	if strings.EqualFold(strings.TrimSpace(raw), "static-scan") {
		return 4
	}
	return 1
}

func platformJudgerID(code string) int64 {
	return platformStableID("judger", code)
}
