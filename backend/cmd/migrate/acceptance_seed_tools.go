// acceptance_seed_tools 从镜像 manifest 生成沙箱工具种子数据。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"chaimir/internal/contracts"
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
	Command               []string                      `json:"command"`
	Args                  []string                      `json:"args"`
	Env                   []workload.EnvVarSpec         `json:"env"`
	EphemeralMounts       []workload.EphemeralMountSpec `json:"ephemeral_mounts"`
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
	component, err := toolComponentFromManifest(manifest, imageURL, kind)
	if err != nil {
		return acceptanceToolDefinition{}, err
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
	status := int16(1)
	if manifest.Tool.RuntimeConfigRequired {
		status = 2
	}
	return acceptanceToolDefinition{
		ID:           acceptanceToolID(manifest.Name, index),
		Code:         manifest.Name,
		Name:         manifestDisplayName(manifest),
		Kind:         kind,
		EcoTags:      manifest.Tool.EcoTags,
		ResourceSpec: spec,
		Status:       status,
	}, nil
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
		component.ReadinessProbe = workload.ProbeSpec{Type: "http", Path: "/", Port: "http", PeriodSeconds: 2, FailureThreshold: 30}
		return component, nil
	}
	if len(manifest.Ports) != 0 || len(manifest.Tool.KeepaliveCommand) == 0 {
		return workload.ComponentSpec{}, fmt.Errorf("命令工具必须无端口并声明 keepalive_command: %s", manifest.Name)
	}
	component.Command = manifest.Tool.KeepaliveCommand
	return component, nil
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
