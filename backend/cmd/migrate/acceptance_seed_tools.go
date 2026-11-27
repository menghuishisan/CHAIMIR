// acceptance_seed_tools 从镜像 manifest 生成沙箱工具种子数据。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"path/filepath"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/sandbox"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/workload"

	"sigs.k8s.io/yaml"
)

// platformStableID 根据逻辑编码生成跨迁移运行稳定的正数 ID,不依赖目录遍历顺序。
func platformStableID(namespace, code string) int64 {
	h := fnv.New64a()
	h.Write([]byte(namespace + "\x00" + strings.TrimSpace(code)))
	return 900000000000000000 + int64(h.Sum64()%99999999999999999)
}

// platformCatalogRuntimeID 按运行时编码派生稳定编号,重复执行种子时同一运行时落到同一行。
func platformCatalogRuntimeID(code string) int64 { return platformStableID("runtime", code) }

// platformCatalogRuntimeImageID 按运行时编码派生其镜像版本的稳定编号。
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
	StudentAccess      toolStudentAccess           `json:"student_access"`
	Resources          toolManifestResources       `json:"resources"`
	Build              map[string]any              `json:"build"`
	Selftest           map[string]any              `json:"selftest"`
	SupplyChain        map[string]any              `json:"supply_chain"`
	SecretsRequired    []manifestSecretRequirement `json:"secrets_required"`
	EnvKeys            map[string]any              `json:"env_keys"`
	Labels             map[string]string           `json:"labels"`
	Capabilities       manifestCapabilities        `json:"capabilities"`
	Toolchain          manifestToolchain           `json:"toolchain"`
}

// manifestToolchain 描述镜像内可离线选择的固定语言工具链,供严格 manifest 解析和验收使用。
// 版本选择必须落到明确的可执行路径,不能由迁移层推断默认版本或创建无版本别名。
type manifestToolchain struct {
	Language         string                    `json:"language"`
	Manager          string                    `json:"manager"`
	Selection        string                    `json:"selection"`
	SelectionEnv     string                    `json:"selection_env"`
	Offline          bool                      `json:"offline"`
	CompilerVersions []manifestCompilerVersion `json:"compiler_versions"`
}

type manifestCompilerVersion struct {
	Version           string `json:"version"`
	Executable        string `json:"executable"`
	ManagedExecutable string `json:"managed_executable"`
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

// toolStudentAccess 是工具学生入口权限的强类型声明,必须与实际 WorkloadSpec 挂载一致。
type toolStudentAccess struct {
	Enabled       bool     `json:"enabled"`
	Shell         string   `json:"shell"`
	AllowedMounts []string `json:"allowed_mounts"`
}

// UnmarshalJSON 统一解析对象式和基础镜像使用的列表式 capabilities,并拒绝未知能力字段。
func (c *manifestCapabilities) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*c = manifestCapabilities{}
		return nil
	}
	if trimmed[0] == '[' {
		var provides []string
		if err := json.Unmarshal(trimmed, &provides); err != nil {
			return fmt.Errorf("capabilities 列表格式无效: %w", err)
		}
		*c = manifestCapabilities{Provides: provides}
		return nil
	}
	type plain manifestCapabilities
	var value plain
	if err := jsonx.DecodeStrictKnownFields(trimmed, &value); err != nil {
		return fmt.Errorf("capabilities 字段格式无效: %w", err)
	}
	*c = manifestCapabilities(value)
	return nil
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

// manifestBinding 直接复用 M2 契约，避免目录转换丢失绑定字段。
type manifestBinding = sandbox.ComponentBinding

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
	if manifest.StudentAccess.Enabled != (studentAccess == "public") {
		return toolManifest{}, fmt.Errorf("工具 manifest 能力访问级别与 student_access.enabled 不一致: %s", path)
	}
	if err := validateToolStudentAccess(manifest); err != nil {
		return toolManifest{}, err
	}
	return manifest, nil
}

// validateToolStudentAccess 校验学生权限声明与工具组件将要使用的安全域一致。
func validateToolStudentAccess(manifest toolManifest) error {
	allowed := map[string]struct{}{}
	for _, raw := range manifest.StudentAccess.AllowedMounts {
		name := strings.TrimSpace(raw)
		if name != sandbox.VolumeDomainWorkspace && name != sandbox.VolumeDomainPublicAssets {
			return fmt.Errorf("工具 %s 的 student_access.allowed_mounts 包含不允许的安全域 %s", manifest.Name, name)
		}
		if _, exists := allowed[name]; exists {
			return fmt.Errorf("工具 %s 的 student_access.allowed_mounts 重复声明 %s", manifest.Name, name)
		}
		allowed[name] = struct{}{}
	}
	if !manifest.StudentAccess.Enabled {
		if len(allowed) != 0 || strings.TrimSpace(manifest.StudentAccess.Shell) != "" && strings.TrimSpace(manifest.StudentAccess.Shell) != "none" {
			return fmt.Errorf("非学生工具 %s 不得声明学生挂载或 shell 权限", manifest.Name)
		}
		return nil
	}
	workspaceAllowed := false
	if _, ok := allowed[sandbox.VolumeDomainWorkspace]; ok {
		workspaceAllowed = true
	}
	if manifest.Tool.MountWorkspace != workspaceAllowed {
		return fmt.Errorf("工具 %s 的 mount_workspace 与 student_access.allowed_mounts.workspace 不一致", manifest.Name)
	}
	if strings.TrimSpace(manifest.StudentAccess.Shell) == "" || strings.TrimSpace(manifest.StudentAccess.Shell) == "none" {
		return fmt.Errorf("学生工具 %s 必须声明 shell 权限", manifest.Name)
	}
	return nil
}

// toolStatusFromManifest 返回已通过供应链与 WorkloadSpec 校验的目录状态。
// 运行时配置由组合编译器根据 config_schema 和 links 生成,不能再作为永久停用理由。
func toolStatusFromManifest(manifest toolManifest) int16 {
	return sandbox.ToolStatusAvailable
}

// toolResourceSpecFromManifest 读取工具显式 WorkloadSpec;未声明时仅为单组件工具生成默认规格。
func toolResourceSpecFromManifest(manifest toolManifest, imageURL string, kind int16) (map[string]any, error) {
	var spec map[string]any
	if len(manifest.Tool.ResourceSpec) > 0 {
		var err error
		spec, err = normalizeExplicitToolResourceSpec(manifest.Tool.ResourceSpec, imageURL, manifest.Selftest, manifest.SecretsRequired)
		if err != nil {
			return nil, err
		}
	} else {
		component, err := toolComponentFromManifest(manifest, imageURL, kind)
		if err != nil {
			return nil, err
		}
		spec = map[string]any{"components": []workload.ComponentSpec{component}}
		if kind == contracts.SandboxToolKindWebEmbed {
			serviceName := "tool-" + manifest.Name + "-web"
			port := component.Ports[0]
			spec["services"] = []workload.ServiceSpec{{
				Name: serviceName, Component: component.Name,
				Ports: []workload.ServicePortSpec{{Name: "http", Port: port.ServicePort, TargetPort: "http", Protocol: port.Protocol}},
			}}
			spec["routes"] = []workload.RouteSpec{{PathPrefix: "/", Service: serviceName, Port: "http"}}
		}
		if kind == contracts.SandboxToolKindCommand {
			spec["command_policy"] = manifest.Tool.CommandPolicy
		}
	}
	spec["bindings"] = manifest.Tool.Bindings
	if err := applyToolStudentMounts(spec, manifest); err != nil {
		return nil, err
	}
	applyManifestCapabilities(spec, manifest)
	if err := validateGeneratedToolResourceSpec(spec, kind); err != nil {
		return nil, err
	}
	return spec, nil
}

// applyToolStudentMounts 把公开工具的学生安全域权限落实为容器挂载,并保留经能力声明授权的系统只读挂载。
func applyToolStudentMounts(spec map[string]any, manifest toolManifest) error {
	if !manifest.StudentAccess.Enabled {
		return nil
	}
	components, ok := spec["components"].([]workload.ComponentSpec)
	if !ok || len(components) == 0 {
		return fmt.Errorf("工具 %s 缺少可执行组件", manifest.Name)
	}
	allowed := make([]string, 0, len(manifest.StudentAccess.AllowedMounts))
	for _, raw := range manifest.StudentAccess.AllowedMounts {
		name := strings.TrimSpace(raw)
		if name != sandbox.VolumeDomainWorkspace {
			allowed = append(allowed, name)
		}
	}
	for index := range components {
		component := &components[index]
		if component.MountWorkspace == nil || *component.MountWorkspace != manifest.Tool.MountWorkspace {
			return fmt.Errorf("工具 %s 的组件 %s 与 mount_workspace 不一致", manifest.Name, component.Name)
		}
		seen := map[string]struct{}{}
		mounts := make([]string, 0, len(component.MountDomains)+len(allowed))
		for _, raw := range component.MountDomains {
			name := strings.TrimSpace(raw)
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			if !containsString(allowed, name) && !manifestAllowsSystemMount(manifest, name) {
				return fmt.Errorf("工具 %s 的组件 %s 请求未授权安全域 %s", manifest.Name, component.Name, name)
			}
			mounts = append(mounts, name)
		}
		for _, name := range allowed {
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			mounts = append(mounts, name)
		}
		component.MountDomains = mounts
		component.WritableMountDomains = nil
	}
	spec["components"] = components
	if len(allowed) > 0 && strings.TrimSpace(anyString(spec["volume_source"])) == "" {
		spec["volume_source"] = "workspace"
	}
	return nil
}

// manifestAllowsSystemMount 只按显式能力授予工具读取平台运行态的权限,不把学生挂载白名单扩大为系统域。
func manifestAllowsSystemMount(manifest toolManifest, domain string) bool {
	if strings.TrimSpace(domain) != sandbox.VolumeDomainRuntimeState {
		return false
	}
	for _, required := range manifest.Capabilities.Requires {
		switch strings.TrimSpace(required) {
		case "fabric-crypto-identity", "runtime-state-reader":
			return true
		}
	}
	return false
}

// containsString 判断已规范化的安全域是否已存在,避免同一组件重复挂载。
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// appendManifestSecretEnv 将显式 WorkloadSpec 的敏感环境变量绑定到声明的实际组件。
func appendManifestSecretEnv(spec map[string]any, requirements []manifestSecretRequirement) error {
	if len(requirements) == 0 {
		return nil
	}
	components, ok := spec["components"].([]any)
	if !ok || len(components) == 0 {
		return fmt.Errorf("显式 WorkloadSpec 缺少 Secret 注入目标组件")
	}
	byName := make(map[string]map[string]any, len(components))
	for _, raw := range components {
		component, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("显式 WorkloadSpec component 格式非法")
		}
		name := strings.TrimSpace(anyString(component["name"]))
		if name == "" {
			return fmt.Errorf("显式 WorkloadSpec component 缺少 name")
		}
		if _, exists := byName[name]; exists {
			return fmt.Errorf("显式 WorkloadSpec component 名称重复: %s", name)
		}
		byName[name] = component
	}
	secretEnv := make(map[string][]any, len(byName))
	seen := make(map[string]map[string]struct{}, len(byName))
	for _, requirement := range requirements {
		componentName := strings.TrimSpace(requirement.Component)
		if componentName == "" {
			return fmt.Errorf("secret %s 缺少 component", strings.TrimSpace(requirement.Env))
		}
		if _, exists := byName[componentName]; !exists {
			return fmt.Errorf("secret %s 声明的 component %s 不存在", strings.TrimSpace(requirement.Env), componentName)
		}
		name := strings.TrimSpace(requirement.Env)
		secretKey := strings.TrimSpace(requirement.SecretKey)
		if name == "" || secretKey == "" {
			return fmt.Errorf("secret 引用必须同时声明 env 和 secret_key")
		}
		if seen[componentName] == nil {
			seen[componentName] = map[string]struct{}{}
		}
		if _, exists := seen[componentName][name]; exists {
			return fmt.Errorf("组件 %s 的 secret 环境变量重复声明: %s", componentName, name)
		}
		seen[componentName][name] = struct{}{}
		secretEnv[componentName] = append(secretEnv[componentName], map[string]any{"name": name, "secret_name": "chaimir-secret", "secret_key": secretKey})
	}
	for componentName, env := range secretEnv {
		component := byName[componentName]
		if existing, exists := component["secret_env"]; exists && existing != nil {
			return fmt.Errorf("组件 %s 不得同时声明 secret_env 和 secrets_required", componentName)
		}
		component["secret_env"] = env
	}
	return nil
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

// normalizeExplicitToolResourceSpec 解析组件镜像引用、自检声明和 Secret 绑定。
func normalizeExplicitToolResourceSpec(input map[string]any, imageURL string, selftest map[string]any, secrets []manifestSecretRequirement) (map[string]any, error) {
	spec := deepCopyMap(input).(map[string]any)
	components, ok := spec["components"].([]any)
	if !ok || len(components) == 0 {
		return nil, fmt.Errorf("显式工作负载必须声明组件")
	}
	for _, item := range components {
		component, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("工作负载组件格式无效")
		}
		name := strings.TrimSpace(anyString(component["name"]))
		if name == "" {
			return nil, fmt.Errorf("工作负载组件缺少名称")
		}
		if err := resolveManifestComponentImage(component, imageURL, selftest); err != nil {
			return nil, fmt.Errorf("解析组件 %s 的镜像和自检声明: %w", name, err)
		}
	}
	if err := appendManifestSecretEnv(spec, secrets); err != nil {
		return nil, err
	}
	data, err := json.Marshal(components)
	if err != nil {
		return nil, fmt.Errorf("编码工作负载组件失败: %w", err)
	}
	var typed []workload.ComponentSpec
	if err := jsonx.DecodeStrictKnownFields(data, &typed); err != nil {
		return nil, fmt.Errorf("工作负载组件包含未知字段或类型错误: %w", err)
	}
	spec["components"] = typed
	return spec, nil
}

// resolveManifestComponentImage 从占位符指向的镜像清单解析自检；直接引用保留组件显式声明。
func resolveManifestComponentImage(component map[string]any, imageURL string, selftest map[string]any) error {
	raw := strings.TrimSpace(anyString(component["image_url"]))
	switch {
	case raw == "":
		return fmt.Errorf("缺少镜像引用")
	case raw == "@self":
		component["image_url"] = imageURL
	case strings.HasPrefix(raw, "@image:"):
		image := strings.TrimSpace(strings.TrimPrefix(raw, "@image:"))
		category, name, ok := strings.Cut(image, "/")
		if !ok || category == "" || name == "" || strings.Contains(name, "/") {
			return fmt.Errorf("镜像占位符无效: %s", raw)
		}
		ref, err := verifiedImageURL(image)
		if err != nil {
			return err
		}
		selftest, err = referencedImageSelftest(image, category, name)
		if err != nil {
			return err
		}
		component["image_url"] = ref
	default:
		return nil
	}
	if _, declared := component["prepull_command"]; declared {
		return fmt.Errorf("镜像占位符的自检命令必须来自镜像清单")
	}
	command, err := platformManifestSelftestCommand(selftest)
	if err != nil {
		return err
	}
	if len(command) == 0 {
		return fmt.Errorf("镜像缺少 selftest.commands")
	}
	component["prepull_command"] = command
	return nil
}

// referencedImageSelftest 只读取镜像标识和自检声明，完整清单由镜像发布门禁校验。
func referencedImageSelftest(image, category, name string) (map[string]any, error) {
	if image == "" || filepath.ToSlash(filepath.Clean(filepath.FromSlash(image))) != image || strings.Contains(image, "\\") || category == "." || category == ".." || name == "." || name == ".." {
		return nil, fmt.Errorf("镜像路径无效: %s", image)
	}
	root, err := platformImagesRoot()
	if err != nil {
		return nil, err
	}
	raw, err := readSeedFile(filepath.Join(root, category, name, "manifest.yaml"))
	if err != nil {
		return nil, fmt.Errorf("读取镜像清单 %s 失败: %w", image, err)
	}
	data, err := yaml.YAMLToJSONStrict(raw)
	if err != nil {
		return nil, fmt.Errorf("解析镜像清单 %s 失败: %w", image, err)
	}
	var manifest struct {
		Category string         `json:"category"`
		Name     string         `json:"name"`
		Image    string         `json:"image"`
		Selftest map[string]any `json:"selftest"`
	}
	if err := jsonx.DecodeStrict(data, &manifest); err != nil {
		return nil, fmt.Errorf("解析镜像清单 %s 的标识和自检声明失败: %w", image, err)
	}
	if manifest.Category != category || manifest.Name != name || manifest.Image != image {
		return nil, fmt.Errorf("镜像清单标识与引用不一致: %s", image)
	}
	return manifest.Selftest, nil
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

// toolComponentFromManifest 构造工具容器声明。
func toolComponentFromManifest(manifest toolManifest, imageURL string, kind int16) (workload.ComponentSpec, error) {
	mountWorkspace := manifest.Tool.MountWorkspace
	prepullCommand, err := platformManifestSelftestCommand(manifest.Selftest)
	if err != nil {
		return workload.ComponentSpec{}, fmt.Errorf("解析工具 selftest.commands 失败: %w", err)
	}
	secretEnv, err := secretEnvFromManifest(manifest.SecretsRequired, toolComponentName(kind))
	if err != nil {
		return workload.ComponentSpec{}, err
	}
	component := workload.ComponentSpec{
		Name:                   toolComponentName(kind),
		ImageURL:               imageURL,
		Command:                manifest.Tool.Command,
		Args:                   manifest.Tool.Args,
		Env:                    manifest.Tool.Env,
		SecretEnv:              secretEnv,
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
