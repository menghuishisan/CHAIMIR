// acceptance_seed_tools 从镜像 manifest 生成沙箱工具种子数据。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/sandbox"
	"chaimir/internal/platform/workload"

	"sigs.k8s.io/yaml"
)

// acceptanceToolDefinition 是写入 tool 表所需的规范化工具定义。
type acceptanceToolDefinition struct {
	ID           int64
	Code         string
	Name         string
	Kind         int16
	EcoTags      []string
	ResourceSpec map[string]any
	Status       int16
}

const acceptanceTerminalToolID int64 = 910000000000001099

type toolManifest struct {
	SchemaVersion      int                   `json:"schema_version"`
	Category           string                `json:"category"`
	Name               string                `json:"name"`
	Image              string                `json:"image"`
	Description        string                `json:"description"`
	Source             map[string]any        `json:"source"`
	Upstream           map[string]any        `json:"upstream"`
	DataDriven         bool                  `json:"data_driven"`
	Tool               toolManifestTool      `json:"tool"`
	Ports              []toolManifestPort    `json:"ports"`
	LocalDev           map[string]any        `json:"local_dev"`
	Auth               map[string]any        `json:"auth"`
	Security           toolManifestSecurity  `json:"security"`
	SecurityExceptions []map[string]any      `json:"security_exceptions"`
	StudentAccess      map[string]any        `json:"student_access"`
	Resources          toolManifestResources `json:"resources"`
	Build              map[string]any        `json:"build"`
	Selftest           map[string]any        `json:"selftest"`
	SupplyChain        map[string]any        `json:"supply_chain"`
	EnvKeys            map[string]any        `json:"env_keys"`
	Labels             map[string]string     `json:"labels"`
	Capabilities       []string              `json:"capabilities"`
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
	EphemeralMounts       []workload.EphemeralMountSpec `json:"ephemeral_mounts"`
	ReadinessPath         string                        `json:"readiness_path"`
	KeepaliveCommand      []string                      `json:"keepalive_command"`
	CommandPolicy         map[string]any                `json:"command_policy"`
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
}

type toolManifestSelftestCommand struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
}

// acceptanceToolDefinitions 读取全部 tool manifest 并转换为 tool 表种子数据。
func acceptanceToolDefinitions() ([]acceptanceToolDefinition, error) {
	root, err := acceptanceImagesRoot()
	if err != nil {
		return nil, err
	}
	toolRoot := filepath.Join(root, "tool")
	entries, err := os.ReadDir(toolRoot)
	if err != nil {
		return nil, fmt.Errorf("读取工具镜像目录失败: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	defs := make([]acceptanceToolDefinition, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifest, err := readToolManifest(filepath.Join(toolRoot, entry.Name(), "manifest.yaml"))
		if err != nil {
			return nil, err
		}
		if !toolManifestDeployable(manifest) {
			continue
		}
		def, err := toolDefinitionFromManifest(len(defs), manifest)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	if len(defs) == 0 {
		return nil, fmt.Errorf("未发现工具 manifest")
	}
	return defs, nil
}

// acceptanceSeedToolDefinitions 返回验收 seed 使用的完整工具集合,包含镜像工具和平台终端工具。
func acceptanceSeedToolDefinitions() ([]acceptanceToolDefinition, error) {
	defs, err := acceptanceToolDefinitions()
	if err != nil {
		return nil, err
	}
	defs = append(defs, acceptanceToolDefinition{
		ID:           acceptanceTerminalToolID,
		Code:         "terminal",
		Name:         "受控终端",
		Kind:         contracts.SandboxToolKindTerminal,
		EcoTags:      []string{"*"},
		ResourceSpec: map[string]any{},
		Status:       1,
	})
	return defs, nil
}

// acceptanceImagesRoot 从当前工作目录向上定位仓库 images 目录。
func acceptanceImagesRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("读取当前工作目录失败: %w", err)
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "images")
		if info, err := os.Stat(filepath.Join(candidate, "tool")); err == nil && info.IsDir() {
			return candidate, nil
		}
		if parent := filepath.Dir(dir); parent == dir {
			break
		}
	}
	return "", fmt.Errorf("未找到 images 目录")
}

// readToolManifest 严格读取单个工具 manifest。
func readToolManifest(path string) (toolManifest, error) {
	raw, err := os.ReadFile(path)
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
	return manifest, nil
}

// toolDefinitionFromManifest 把镜像 manifest 转换为 M2 tool.resource_spec。
func toolDefinitionFromManifest(index int, manifest toolManifest) (acceptanceToolDefinition, error) {
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
		ID:           acceptanceToolID(manifest.Name, index),
		Code:         manifest.Name,
		Name:         manifestDisplayName(manifest),
		Kind:         kind,
		EcoTags:      manifest.Tool.EcoTags,
		ResourceSpec: spec,
		Status:       toolStatusFromManifest(manifest),
	}, nil
}

// toolStatusFromManifest 避免把仍需运行时/实验注入私有配置的工具误标为可调度。
func toolStatusFromManifest(manifest toolManifest) int16 {
	if manifest.Tool.RuntimeConfigRequired {
		return sandbox.ToolStatusDisabled
	}
	return sandbox.ToolStatusAvailable
}

// toolResourceSpecFromManifest 读取工具显式 WorkloadSpec;未声明时仅为单组件工具生成默认规格。
func toolResourceSpecFromManifest(manifest toolManifest, imageURL string, kind int16) (map[string]any, error) {
	if len(manifest.Tool.ResourceSpec) > 0 {
		spec, err := normalizeExplicitToolResourceSpec(manifest.Tool.ResourceSpec, imageURL, kind)
		if err != nil {
			return nil, err
		}
		command := toolPrepullCommandFromManifest(manifest)
		if len(command) == 0 {
			return nil, fmt.Errorf("显式工具 WorkloadSpec 必须声明 selftest.commands 作为预拉取自检命令: %s", manifest.Name)
		}
		spec["prepull_command"] = command
		if err := validateGeneratedToolResourceSpec(spec, kind); err != nil {
			return nil, err
		}
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
	if command := toolPrepullCommandFromManifest(manifest); len(command) > 0 {
		spec["prepull_command"] = command
	}
	if err := validateGeneratedToolResourceSpec(spec, kind); err != nil {
		return nil, err
	}
	return spec, nil
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
	if err := validateGeneratedToolResourceSpec(spec, kind); err != nil {
		return nil, err
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
	if _, ok := component["prepull_command"]; ok {
		return nil
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

// toolPrepullCommandFromManifest 选择镜像声明的首个自检命令作为预拉取启动命令。
func toolPrepullCommandFromManifest(manifest toolManifest) []string {
	raw, ok := manifest.Selftest["commands"]
	if !ok {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var commands []toolManifestSelftestCommand
	if err := json.Unmarshal(data, &commands); err != nil || len(commands) == 0 {
		return nil
	}
	command := commands[0].Command
	if len(command) == 0 {
		return nil
	}
	out := make([]string, 0, len(command))
	for _, part := range command {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
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
	component := workload.ComponentSpec{
		Name:                   toolComponentName(kind),
		ImageURL:               imageURL,
		Command:                manifest.Tool.Command,
		Args:                   manifest.Tool.Args,
		Env:                    manifest.Tool.Env,
		Resources:              workload.ResourceSpec{Requests: map[string]string{"cpu": manifest.Resources.CPURequest, "memory": manifest.Resources.MemoryRequest}, Limits: map[string]string{"cpu": manifest.Resources.CPULimit, "memory": manifest.Resources.MemoryLimit}},
		ReadOnlyRootFilesystem: &manifest.Security.ReadOnlyRootFilesystem,
		MountWorkspace:         &mountWorkspace,
		EphemeralMounts:        manifest.Tool.EphemeralMounts,
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

// acceptanceToolID 返回工具确定性 ID。
func acceptanceToolID(name string, index int) int64 {
	return 910000000000001100 + int64(index)
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
