// acceptance_seed_tools 从镜像 manifest 生成沙箱工具种子数据。
package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/sandbox"
	"chaimir/internal/platform/workload"

	"sigs.k8s.io/yaml"
)

// platformStableID 根据逻辑编码生成跨迁移运行稳定的正数 ID,不依赖目录遍历顺序。
func platformStableID(namespace, code string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(namespace + "\x00" + strings.TrimSpace(code)))
	return 900000000000000000 + int64(h.Sum64()%99999999999999999)
}

func platformCatalogRuntimeID(code string) int64      { return platformStableID("runtime", code) }
func platformCatalogRuntimeImageID(code string) int64 { return platformStableID("runtime-image", code) }

// imageVersionFromURL 把不可变 digest 的短前缀作为展示版本,不再维护一份硬编码版本号。
func imageVersionFromURL(imageURL string) string {
	imageURL = strings.TrimSpace(imageURL)
	if at := strings.Index(imageURL, "@sha256:"); at >= 0 {
		digest := imageURL[at+len("@sha256:"):]
		if len(digest) > 16 {
			digest = digest[:16]
		}
		if digest != "" {
			return "sha256-" + digest
		}
	}
	return "unverified"
}

// platformRuntimeImageVersion 从当前供应链证明推导组合引用版本,避免旧的固定版本号漂移。
func platformRuntimeImageVersion(code string) string {
	url, proven := platformProof("runtime/" + strings.TrimSpace(code))
	if !proven {
		return "unverified"
	}
	return imageVersionFromURL(url)
}

// acceptanceToolDefinition 是 manifest 转换阶段的规范化工具定义。
type acceptanceToolDefinition struct {
	ID           int64
	Code         string
	Name         string
	Kind         int16
	EcoTags      []string
	ResourceSpec map[string]any
	Status       int16
}

type toolManifest struct {
	SchemaVersion      int                         `json:"schema_version"`
	Category           string                      `json:"category"`
	Name               string                      `json:"name"`
	Image              string                      `json:"image"`
	Description        string                      `json:"description"`
	Source             map[string]any              `json:"source" yaml:"source"`
	Upstream           map[string]any              `json:"upstream"`
	DataDriven         bool                        `json:"data_driven"`
	Tool               toolManifestTool            `json:"tool"`
	Ports              []toolManifestPort          `json:"ports"`
	Auth               map[string]any              `json:"auth"`
	Security           toolManifestSecurity        `json:"security"`
	SecurityExceptions []map[string]any            `json:"security_exceptions"`
	StudentAccess      map[string]any              `json:"student_access"`
	Resources          toolManifestResources       `json:"resources"`
	Build              map[string]any              `json:"build"`
	Selftest           map[string]any              `json:"selftest"`
	SupplyChain        map[string]any              `json:"supply_chain"`
	SecretsRequired    []manifestSecretRequirement `json:"secrets_required"`
	EnvKeys            map[string]any              `json:"env_keys"`
	Labels             map[string]string           `json:"labels"`
	Capabilities       manifestCapabilities        `json:"capabilities"`
}

type manifestCapabilities struct {
	Provides      []string       `json:"provides"`
	Requires      []string       `json:"requires"`
	Conflicts     []string       `json:"conflicts"`
	Cardinality   string         `json:"cardinality"`
	Placement     string         `json:"placement"`
	ConfigSchema  map[string]any `json:"config_schema"`
	StudentAccess string         `json:"student_access"`
}

type toolManifestTool struct {
	Kind                  string                        `json:"kind"`
	EcoTags               []string                      `json:"eco_tags"`
	MountWorkspace        bool                          `json:"mount_workspace"`
	RuntimeConfigRequired bool                          `json:"runtime_config_required"`
	ResourceSpec          map[string]any                `json:"resource_spec"`
	Command               []string                      `json:"command"`
	Args                  []string                      `json:"args"`
	Env                   []workload.EnvVarSpec         `json:"env"`
	MountDomains          []string                      `json:"mount_domains"`
	EphemeralMounts       []workload.EphemeralMountSpec `json:"ephemeral_mounts"`
	ReadinessPath         string                        `json:"readiness_path"`
	KeepaliveCommand      []string                      `json:"keepalive_command"`
	Bindings              []manifestBinding             `json:"bindings"`
	CommandPolicy         map[string]any                `json:"command_policy"`
}

// manifestBinding 是 images manifest 与 M2 WorkloadSpec 共用的能力端点绑定。
type manifestBinding struct {
	Name            string `json:"name"`
	Capability      string `json:"capability"`
	Endpoint        string `json:"endpoint"`
	Protocol        string `json:"protocol"`
	RequiredAtStart bool   `json:"required_at_start"`
	ConfigBinding   string `json:"config_binding"`
}

type toolManifestPort struct {
	Name          string `json:"name"`
	ContainerPort int32  `json:"container_port"`
	Protocol      string `json:"protocol"`
	Expose        string `json:"expose"`
	Purpose       string `json:"purpose"`
}

type toolManifestSecurity struct {
	RunAsNonRoot                 bool     `json:"run_as_non_root"`
	ReadOnlyRootFilesystem       bool     `json:"read_only_root_filesystem"`
	AllowPrivilegeEscalation     bool     `json:"allow_privilege_escalation"`
	Privileged                   bool     `json:"privileged"`
	HostNetwork                  bool     `json:"host_network"`
	AutomountServiceAccountToken bool     `json:"automount_service_account_token"`
	DropCapabilities             []string `json:"drop_capabilities"`
	NetworkPolicy                string   `json:"network_policy"`
}

type toolManifestResources struct {
	CPURequest            string `json:"cpu_request"`
	CPULimit              string `json:"cpu_limit"`
	MemoryRequest         string `json:"memory_request"`
	MemoryLimit           string `json:"memory_limit"`
	EphemeralStorageLimit string `json:"ephemeral_storage_limit"`
	TimeoutSeconds        int32  `json:"timeout_seconds"`
}

type toolManifestSelftestCommand struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
}

// acceptanceImagesRoot 从当前工作目录向上定位仓库 images 目录。
func acceptanceImagesRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("读取当前工作目录失败: %w", err)
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "images")
		if info, statErr := os.Stat(filepath.Join(candidate, "tool")); statErr == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", fmt.Errorf("未找到 images 目录")
}

// toolDefinitionFromManifest 把工具 manifest 转换为 M2 工具资源规格。
func toolDefinitionFromManifest(manifest toolManifest) (acceptanceToolDefinition, error) {
	imageURL, err := acceptanceImageURL(manifest.Image)
	if err != nil {
		return acceptanceToolDefinition{}, err
	}
	kind, err := toolKindFromManifest(manifest.Tool.Kind)
	if err != nil {
		return acceptanceToolDefinition{}, err
	}
	spec, err := toolResourceSpecFromManifest(manifest, imageURL, kind)
	if err != nil {
		return acceptanceToolDefinition{}, err
	}
	return acceptanceToolDefinition{
		ID: platformCatalogToolID(manifest.Name), Code: manifest.Name, Name: manifestDisplayName(manifest),
		Kind: kind, EcoTags: manifest.Tool.EcoTags, ResourceSpec: spec, Status: toolStatusFromManifest(manifest),
	}, nil
}

// readToolManifest 严格读取单个工具 manifest。
func readToolManifest(path string) (toolManifest, error) {
	raw, err := readSeedFile(path)
	if err != nil {
		return toolManifest{}, fmt.Errorf("读取工具 manifest 失败: %w", err)
	}
	var manifest toolManifest
	if err := yaml.UnmarshalStrict(raw, &manifest); err != nil {
		return toolManifest{}, fmt.Errorf("解析工具 manifest 失败 %s: %w", path, err)
	}
	if manifest.Category != "tool" || strings.TrimSpace(manifest.Name) == "" || manifest.Image != "tool/"+manifest.Name {
		return toolManifest{}, fmt.Errorf("工具 manifest 分类或镜像名不一致: %s", path)
	}
	studentAccess := strings.TrimSpace(manifest.Capabilities.StudentAccess)
	if studentAccess != "public" && studentAccess != "private" {
		return toolManifest{}, fmt.Errorf("工具 manifest capabilities.student_access 无效: %s", path)
	}
	enabled, ok := manifest.StudentAccess["enabled"].(bool)
	if !ok || (enabled && studentAccess != "public") || (!enabled && studentAccess != "private") {
		return toolManifest{}, fmt.Errorf("工具 manifest 能力访问级别与 student_access.enabled 不一致: %s", path)
	}
	return manifest, nil
}

// toolStatusFromManifest 返回已通过供应链与 WorkloadSpec 校验的目录状态。
// 运行时配置由组合编译器根据 config_schema 和 links 生成,不能再作为永久停用理由。
func toolStatusFromManifest(manifest toolManifest) int16 {
	return sandbox.ToolStatusAvailable
}

// toolResourceSpecFromManifest 读取工具显式 WorkloadSpec;未声明时仅为单组件工具生成默认规格。
func toolResourceSpecFromManifest(manifest toolManifest, imageURL string, kind int16) (map[string]any, error) {
	if len(manifest.Tool.ResourceSpec) > 0 {
		spec, err := normalizeExplicitToolResourceSpec(manifest.Tool.ResourceSpec, imageURL, kind)
		if err != nil {
			return nil, err
		}
		command, err := toolPrepullCommandFromManifest(manifest)
		if err != nil {
			return nil, fmt.Errorf("解析工具 selftest.commands 失败: %w", err)
		}
		if len(command) == 0 {
			return nil, fmt.Errorf("显式工具 WorkloadSpec 必须声明 selftest.commands 作为预拉取自检命令: %s", manifest.Name)
		}
		componentData, err := json.Marshal(spec["components"])
		if err != nil {
			return nil, fmt.Errorf("编码显式工具组件失败: %w", err)
		}
		var components []workload.ComponentSpec
		if err := json.Unmarshal(componentData, &components); err != nil || len(components) == 0 {
			return nil, fmt.Errorf("显式工具 WorkloadSpec 缺少可预拉取组件: %s", manifest.Name)
		}
		for index := range components {
			if len(components[index].PrepullCommand) == 0 {
				components[index].PrepullCommand = append([]string(nil), command...)
			}
		}
		spec["components"] = components
		appendManifestSecretEnv(spec, manifest.SecretsRequired)
		spec["prepull_command"] = command
		spec["bindings"] = append([]manifestBinding(nil), manifest.Tool.Bindings...)
		if err := validateGeneratedToolResourceSpec(spec, kind); err != nil {
			return nil, err
		}
		applyManifestCapabilities(spec, manifest)
		return spec, nil
	}
	component, err := toolComponentFromManifest(manifest, imageURL, kind)
	if err != nil {
		return nil, err
	}
	spec := map[string]any{"components": []workload.ComponentSpec{component}}
	if kind == contracts.SandboxToolKindWebEmbed {
		serviceName := "tool-" + manifest.Name + "-web"
		spec["services"] = []workload.ServiceSpec{{
			Name:      serviceName,
			Component: component.Name,
			Ports:     []workload.ServicePortSpec{{Name: "http", Port: component.Ports[0].ServicePort, TargetPort: "http", Protocol: component.Ports[0].Protocol}},
		}}
		spec["routes"] = []workload.RouteSpec{{PathPrefix: "/", Service: serviceName, Port: "http"}}
	}
	if kind == contracts.SandboxToolKindCommand {
		spec["command_policy"] = manifest.Tool.CommandPolicy
	}
	command, err := toolPrepullCommandFromManifest(manifest)
	if err != nil {
		return nil, fmt.Errorf("解析工具 selftest.commands 失败: %w", err)
	}
	if len(command) > 0 {
		spec["prepull_command"] = command
	}
	spec["bindings"] = append([]manifestBinding(nil), manifest.Tool.Bindings...)
	if err := validateGeneratedToolResourceSpec(spec, kind); err != nil {
		return nil, err
	}
	applyManifestCapabilities(spec, manifest)
	return spec, nil
}

// appendManifestSecretEnv 将显式 WorkloadSpec 的敏感环境变量统一绑定到平台 Secret。
func appendManifestSecretEnv(spec map[string]any, requirements []manifestSecretRequirement) {
	if len(requirements) == 0 {
		return
	}
	components, ok := spec["components"].([]any)
	if !ok || len(components) == 0 {
		return
	}
	component, ok := components[0].(map[string]any)
	if !ok {
		return
	}
	secretEnv := make([]any, 0, len(requirements))
	for _, requirement := range requirements {
		name := strings.TrimSpace(requirement.Env)
		if name == "" {
			continue
		}
		secretEnv = append(secretEnv, map[string]any{"name": name, "secret_name": "chaimir-secret", "secret_key": name})
	}
	if len(secretEnv) > 0 {
		component["secret_env"] = secretEnv
	}
}

// applyManifestCapabilities 将镜像清单的原子能力写入唯一的工具资源声明。
func applyManifestCapabilities(spec map[string]any, manifest toolManifest) {
	provides := make([]string, 0, len(manifest.Capabilities.Provides))
	for _, capability := range manifest.Capabilities.Provides {
		if value := strings.TrimSpace(capability); value != "" {
			provides = append(provides, value)
		}
	}
	studentAccess := strings.TrimSpace(manifest.Capabilities.StudentAccess)
	spec["capabilities"] = map[string]any{
		"provides":       provides,
		"requires":       append([]string(nil), manifest.Capabilities.Requires...),
		"conflicts":      append([]string(nil), manifest.Capabilities.Conflicts...),
		"cardinality":    manifest.Capabilities.Cardinality,
		"placement":      manifest.Capabilities.Placement,
		"config_schema":  manifest.Capabilities.ConfigSchema,
		"student_access": studentAccess,
	}
}

// normalizeExplicitToolResourceSpec 校验显式 WorkloadSpec 的基本形态,并把 @self 替换为本镜像 digest。
func normalizeExplicitToolResourceSpec(input map[string]any, imageURL string, kind int16) (map[string]any, error) {
	spec := deepCopyMap(input).(map[string]any)
	components, ok := spec["components"].([]any)
	if !ok || len(components) == 0 {
		return nil, fmt.Errorf("显式工具 WorkloadSpec 必须声明 components")
	}
	for _, item := range components {
		component, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("显式工具 WorkloadSpec component 格式非法")
		}
		if strings.TrimSpace(anyString(component["name"])) == "" {
			return nil, fmt.Errorf("显式工具 WorkloadSpec component 缺少 name")
		}
		switch strings.TrimSpace(anyString(component["image_url"])) {
		case "":
			return nil, fmt.Errorf("显式工具 WorkloadSpec component 缺少 image_url")
		case "@self":
			component["image_url"] = imageURL
		default:
			if err := normalizeReferencedComponentImage(component); err != nil {
				return nil, err
			}
		}
	}
	if kind == contracts.SandboxToolKindWebEmbed {
		if _, ok := spec["services"].([]any); !ok {
			return nil, fmt.Errorf("web 工具显式 WorkloadSpec 必须声明 services")
		}
		if _, ok := spec["routes"].([]any); !ok {
			return nil, fmt.Errorf("web 工具显式 WorkloadSpec 必须声明 routes")
		}
	}
	if kind == contracts.SandboxToolKindCommand {
		if _, ok := spec["command_policy"].(map[string]any); !ok {
			return nil, fmt.Errorf("命令工具显式 WorkloadSpec 必须声明 command_policy")
		}
	}
	return spec, nil
}

// normalizeReferencedComponentImage 把显式工具组件里的受控镜像占位符替换为已证明的 Harbor digest。
func normalizeReferencedComponentImage(component map[string]any) error {
	raw := strings.TrimSpace(anyString(component["image_url"]))
	if !strings.HasPrefix(raw, "@image:") {
		return nil
	}
	image := strings.TrimSpace(strings.TrimPrefix(raw, "@image:"))
	parts := strings.Split(image, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("显式工具 WorkloadSpec component 镜像占位符非法: %s", raw)
	}
	imageURL, err := acceptanceImageURL(image)
	if err != nil {
		return err
	}
	component["image_url"] = imageURL
	if rawCommand, ok := component["prepull_command"]; ok && rawCommand != nil {
		data, err := json.Marshal(rawCommand)
		if err != nil {
			return fmt.Errorf("编码组件预拉取命令失败: %w", err)
		}
		var existing []string
		if err := json.Unmarshal(data, &existing); err == nil && len(existing) > 0 {
			return nil
		}
	}
	manifest, err := acceptanceImageUnitManifestFor(image, parts[0])
	if err != nil {
		return err
	}
	command, err := acceptanceManifestSelftestCommand(manifest)
	if err != nil {
		return err
	}
	component["prepull_command"] = command
	return nil
}

// toolPrepullCommandFromManifest 选择镜像声明的首个自检命令作为预拉取启动命令,显式传播 manifest 解析错误。
func toolPrepullCommandFromManifest(manifest toolManifest) ([]string, error) {
	raw, ok := manifest.Selftest["commands"]
	if !ok {
		return nil, nil
	}
	data, err := json.Marshal(raw)
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
	command := commands[0].Command
	if len(command) == 0 {
		return nil, fmt.Errorf("selftest.commands 首条命令不能为空")
	}
	out := make([]string, 0, len(command))
	for _, part := range command {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("selftest.commands 首条命令不能为空")
	}
	return out, nil
}

// validateGeneratedToolResourceSpec 复用 M2 规则层校验 seed 产物,避免迁移入口绕过运行期约束。
func validateGeneratedToolResourceSpec(spec map[string]any, kind int16) error {
	raw, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("编码工具 WorkloadSpec 失败: %w", err)
	}
	if err := sandbox.ValidateToolResourceSpecDefinition(raw, kind); err != nil {
		return fmt.Errorf("工具 WorkloadSpec 校验失败: %w", err)
	}
	return nil
}

// deepCopyMap 复制 YAML 解析出的 map/slice,避免规范化过程修改原始 manifest 结构。
func deepCopyMap(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = deepCopyMap(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, deepCopyMap(item))
		}
		return out
	default:
		return typed
	}
}

// anyString 返回 YAML 标量的字符串值,仅用于 manifest 结构校验。
func anyString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

// toolKindFromManifest 把 manifest 的唯一工具类型转换为数据库枚举。
func toolKindFromManifest(kind string) (int16, error) {
	switch strings.TrimSpace(kind) {
	case "web-embed":
		return contracts.SandboxToolKindWebEmbed, nil
	case "command-tool":
		return contracts.SandboxToolKindCommand, nil
	default:
		return 0, fmt.Errorf("不支持的工具类型: %s", kind)
	}
}

// toolManifestDeployable 只让供应链准入的工具进入验收种子。
func toolManifestDeployable(manifest toolManifest) bool {
	value, ok := manifest.SupplyChain["deployable"]
	if !ok {
		return true
	}
	deployable, ok := value.(bool)
	if !ok {
		return true
	}
	return deployable
}

// toolComponentFromManifest 构造工具容器声明。
func toolComponentFromManifest(manifest toolManifest, imageURL string, kind int16) (workload.ComponentSpec, error) {
	mountWorkspace := manifest.Tool.MountWorkspace
	prepullCommand, err := toolPrepullCommandFromManifest(manifest)
	if err != nil {
		return workload.ComponentSpec{}, fmt.Errorf("解析工具 selftest.commands 失败: %w", err)
	}
	component := workload.ComponentSpec{
		Name:                   toolComponentName(kind),
		ImageURL:               imageURL,
		Command:                manifest.Tool.Command,
		Args:                   manifest.Tool.Args,
		Env:                    manifest.Tool.Env,
		SecretEnv:              secretEnvFromManifest(manifest.SecretsRequired),
		Resources:              workload.ResourceSpec{Requests: map[string]string{"cpu": manifest.Resources.CPURequest, "memory": manifest.Resources.MemoryRequest}, Limits: map[string]string{"cpu": manifest.Resources.CPULimit, "memory": manifest.Resources.MemoryLimit}},
		ReadOnlyRootFilesystem: &manifest.Security.ReadOnlyRootFilesystem,
		MountWorkspace:         &mountWorkspace,
		MountDomains:           append([]string(nil), manifest.Tool.MountDomains...),
		EphemeralMounts:        manifest.Tool.EphemeralMounts,
		PrepullCommand:         prepullCommand,
	}
	if kind == contracts.SandboxToolKindWebEmbed {
		if len(manifest.Ports) != 1 || manifest.Ports[0].Expose != "proxy" {
			return workload.ComponentSpec{}, fmt.Errorf("web 工具必须声明唯一平台代理端口: %s", manifest.Name)
		}
		port := manifest.Ports[0]
		component.Ports = []workload.PortSpec{{Name: "http", ContainerPort: port.ContainerPort, ServicePort: port.ContainerPort, Protocol: defaultProtocol(port.Protocol)}}
		readinessPath, err := toolReadinessPath(manifest)
		if err != nil {
			return workload.ComponentSpec{}, err
		}
		component.ReadinessProbe = workload.ProbeSpec{Type: "http", Path: readinessPath, Port: "http", PeriodSeconds: 2, FailureThreshold: 30}
		return component, nil
	}
	if len(manifest.Ports) != 0 || len(manifest.Tool.KeepaliveCommand) == 0 {
		return workload.ComponentSpec{}, fmt.Errorf("命令工具必须无端口并声明 keepalive_command: %s", manifest.Name)
	}
	component.Command = manifest.Tool.KeepaliveCommand
	return component, nil
}

// toolReadinessPath 返回 Web 工具声明的健康检查路径,默认沿用根路径。
func toolReadinessPath(manifest toolManifest) (string, error) {
	path := strings.TrimSpace(manifest.Tool.ReadinessPath)
	if path == "" {
		return "/", nil
	}
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("web 工具 readiness_path 必须以 / 开头: %s", manifest.Name)
	}
	return path, nil
}

// toolComponentName 返回工具类型内唯一组件名。
func toolComponentName(kind int16) string {
	if kind == contracts.SandboxToolKindCommand {
		return "command"
	}
	return "web"
}

// platformCatalogToolID 返回平台工具目录的确定性 ID。
func platformCatalogToolID(name string) int64 {
	return platformStableID("tool", name)
}

// platformCatalogInfraID 返回基础设施目录的确定性 ID,与教师工具区间分离。
func platformCatalogInfraID(name string) int64 {
	return platformStableID("infra", name)
}

// manifestDisplayName 选择工具展示名。
func manifestDisplayName(manifest toolManifest) string {
	if strings.TrimSpace(manifest.Description) != "" {
		return strings.TrimSpace(manifest.Description)
	}
	return manifest.Name
}

// defaultProtocol 补齐端口协议默认值。
func defaultProtocol(protocol string) string {
	protocol = strings.ToUpper(strings.TrimSpace(protocol))
	if protocol == "" {
		return "TCP"
	}
	return protocol
}
