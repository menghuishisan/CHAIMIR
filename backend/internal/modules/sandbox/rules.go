// sandbox rules 文件定义输入校验、状态机校验和安全规则,不访问 repo/db/contracts。
package sandbox

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"

	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	codePattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)
	envNamePattern   = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,63}$`)
	mountNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	portNamePattern  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,14}$`)
	shellCommands    = map[string]struct{}{"sh": {}, "bash": {}, "dash": {}, "ash": {}, "zsh": {}, "ksh": {}, "csh": {}, "cmd": {}, "cmd.exe": {}, "powershell": {}, "powershell.exe": {}, "pwsh": {}, "pwsh.exe": {}}
)

// validateCreateRequest 校验内部创建沙箱请求的租户、来源和资源开关。
func validateCreateRequest(req CreateSandboxInputModel) error {
	if req.TenantID <= 0 || strings.TrimSpace(req.RuntimeCode) == "" || strings.TrimSpace(req.SourceRef) == "" {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	if req.OwnerAccountID <= 0 {
		return apperr.ErrSandboxOwnerInvalid
	}
	if !validSourceRef(req.SourceRef) {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	if req.KeepAliveMinutes < 0 || req.SnapshotRetentionMinutes < 0 {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	if !req.KeepAlive && req.KeepAliveMinutes > 0 {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	if !req.SnapshotEnabled && req.SnapshotRetentionMinutes > 0 {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	return nil
}

// validSourceRef 复用平台服务鉴权的来源标识规则,避免 HTTP 与 contract 调用出现两套口径。
func validSourceRef(sourceRef string) bool {
	return auth.ValidSourceRef(sourceRef)
}

// validateRuntimeRequest 校验运行时声明式清单满足控制面可执行边界。
func validateRuntimeRequest(req RuntimeRequest, cfg config.SandboxConfig) (AdapterSpec, error) {
	if !codePattern.MatchString(req.Code) || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Eco) == "" {
		return AdapterSpec{}, apperr.ErrSandboxRuntimeCreateInvalid
	}
	if req.AdapterLevel < 1 || req.AdapterLevel > 3 || len(req.AdapterSpec) == 0 {
		return AdapterSpec{}, apperr.ErrSandboxAdapterSpecInvalid
	}
	if req.Status != 0 && req.Status != RuntimeStatusOnboarding && req.Status != RuntimeStatusDisabled {
		return AdapterSpec{}, apperr.ErrSandboxRuntimeCreateInvalid
	}
	var spec AdapterSpec
	if err := jsonx.DecodeStrict(req.AdapterSpec, &spec); err != nil {
		return AdapterSpec{}, apperr.ErrSandboxAdapterSpecInvalid.WithCause(err)
	}
	if err := normalizeAndValidateAdapterSpec(&spec, cfg); err != nil {
		return AdapterSpec{}, err
	}
	if req.AdapterLevel >= 2 &&
		strings.TrimSpace(req.CapabilityImpl) == "" &&
		strings.TrimSpace(req.PluginRef) == "" &&
		!hasCapabilityCommands(spec.CapabilityCommands) {
		return AdapterSpec{}, apperr.ErrSandboxCapabilityUnavailable
	}
	return spec, nil
}

// validateToolRequest 校验工具定义,确保不同工具类型只走各自唯一声明口径。
func validateToolRequest(req ToolRequest, cfg config.SandboxConfig) (ToolResourceSpec, error) {
	if !codePattern.MatchString(req.Code) || strings.TrimSpace(req.Name) == "" {
		return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
	}
	if req.Kind < SandboxToolKindBuiltin || req.Kind > SandboxToolKindCommand {
		return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
	}
	if req.Status != 0 && req.Status != ToolStatusAvailable && req.Status != ToolStatusDisabled {
		return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
	}
	var spec ToolResourceSpec
	if len(req.ResourceSpec) > 0 {
		if err := jsonx.DecodeStrict(req.ResourceSpec, &spec); err != nil {
			return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid.WithCause(err)
		}
	}
	if req.Kind == SandboxToolKindWebEmbed {
		if commandPolicyConfigured(spec.CommandPolicy) {
			return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
		}
		if len(spec.Components) == 0 || len(spec.Services) == 0 || len(spec.Routes) == 0 {
			return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
		}
		for i := range spec.Components {
			component := &spec.Components[i]
			if err := validateContainerSpec(component, cfg); err != nil {
				return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid.WithCause(err)
			}
			if err := validateToolEphemeralMounts(component.EphemeralMounts); err != nil {
				return ToolResourceSpec{}, err
			}
			if !imageAttested(cfg, component.ImageURL, digestFromImageURL(component.ImageURL)) {
				return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
			}
			if component.MountWorkspace == nil {
				return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
			}
			if len(component.Command) > 0 && !safeNonShellCommand(component.Command) {
				return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
			}
		}
	}
	if req.Kind == SandboxToolKindCommand {
		if err := validateCommandToolSpec(&spec, cfg); err != nil {
			return ToolResourceSpec{}, err
		}
	}
	if req.Kind == SandboxToolKindBuiltin {
		if commandPolicyConfigured(spec.CommandPolicy) {
			return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
		}
		if !validBuiltinEndpointTemplate(spec.BuiltinEndpoint) {
			return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
		}
	}
	if req.Kind != SandboxToolKindWebEmbed && req.Kind != SandboxToolKindCommand && toolHasContainerSpec(spec) {
		return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
	}
	if req.Kind != SandboxToolKindWebEmbed && len(spec.NetworkRules) > 0 {
		return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
	}
	if err := validateToolServicesAndRoutes(&spec); err != nil {
		return ToolResourceSpec{}, err
	}
	if err := validateToolNetworkRules(&spec); err != nil {
		return ToolResourceSpec{}, err
	}
	return spec, nil
}

// validateCommandToolSpec 校验命令工具只能声明无端口执行容器,不得暴露平台代理入口。
func validateCommandToolSpec(spec *ToolResourceSpec, cfg config.SandboxConfig) error {
	if spec == nil {
		return apperr.ErrSandboxToolCreateInvalid
	}
	if len(spec.Components) != 1 || len(spec.Services) > 0 || len(spec.Routes) > 0 || len(spec.NetworkRules) > 0 {
		return apperr.ErrSandboxToolCreateInvalid
	}
	if err := validateCommandToolPolicy(&spec.CommandPolicy); err != nil {
		return err
	}
	if err := validateCommandContainerSpec(&spec.Components[0], cfg); err != nil {
		return apperr.ErrSandboxToolCreateInvalid.WithCause(err)
	}
	return nil
}

// validateCommandToolPolicy 校验命令工具只允许声明固定入口命令和正数超时。
func validateCommandToolPolicy(policy *CommandToolPolicy) error {
	if policy == nil || len(policy.AllowedCommands) == 0 || policy.DefaultTimeoutSeconds <= 0 || policy.MaxTimeoutSeconds <= 0 || policy.DefaultTimeoutSeconds > policy.MaxTimeoutSeconds {
		return apperr.ErrSandboxToolCreateInvalid
	}
	seen := map[string]struct{}{}
	for i := range policy.AllowedCommands {
		command := strings.TrimSpace(policy.AllowedCommands[i])
		if command == "" || strings.ContainsAny(command, `/\`) {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if _, exists := seen[command]; exists {
			return apperr.ErrSandboxToolCreateInvalid
		}
		seen[command] = struct{}{}
		policy.AllowedCommands[i] = command
	}
	return nil
}

// commandPolicyConfigured 判断非命令工具是否误带命令策略。
func commandPolicyConfigured(policy CommandToolPolicy) bool {
	return len(policy.AllowedCommands) > 0 || policy.DefaultTimeoutSeconds != 0 || policy.MaxTimeoutSeconds != 0
}

// validateCommandContainerSpec 校验命令工具容器长期待命且不暴露网络端口。
func validateCommandContainerSpec(spec *workload.ComponentSpec, cfg config.SandboxConfig) error {
	spec.Name = strings.TrimSpace(spec.Name)
	if !mountNamePattern.MatchString(spec.Name) || len(spec.Ports) != 0 {
		return apperr.ErrSandboxContainerSpecInvalid
	}
	if strings.TrimSpace(spec.ImageURL) == "" || digestFromImageURL(spec.ImageURL) == "" {
		return apperr.ErrSandboxContainerSpecInvalid
	}
	if !imageAttested(cfg, spec.ImageURL, digestFromImageURL(spec.ImageURL)) {
		return apperr.ErrSandboxToolCreateInvalid
	}
	if err := validateLiteralEnv(spec.Env); err != nil {
		return err
	}
	if err := validateResourceSpec(spec.Resources); err != nil {
		return err
	}
	normalizeProbe(&spec.ReadinessProbe, cfg)
	normalizeProbe(&spec.LivenessProbe, cfg)
	for _, probe := range []*workload.ProbeSpec{&spec.ReadinessProbe, &spec.LivenessProbe} {
		if err := validateProbeSpec(probe, map[string]struct{}{}); err != nil {
			return err
		}
	}
	if !safeNonShellCommand(spec.Command) {
		return apperr.ErrSandboxContainerSpecInvalid
	}
	if spec.MountWorkspace == nil {
		return apperr.ErrSandboxContainerSpecInvalid
	}
	if err := validateToolEphemeralMounts(spec.EphemeralMounts); err != nil {
		return err
	}
	return nil
}

// validateQuota 校验租户沙箱配额均为显式正数,快照和保活上限允许为零表示禁用。
func validateQuota(q TenantQuota) error {
	if q.TenantID <= 0 || q.MaxConcurrentSandbox <= 0 || q.MaxCPU <= 0 || q.MaxMemoryMB <= 0 ||
		q.IdleTimeoutMin <= 0 || q.MaxLifetimeMin <= 0 || q.MaxKeepaliveMin < 0 || q.MaxSnapshotRetentionMin < 0 {
		return apperr.ErrSandboxQuotaInvalid
	}
	return nil
}

// validateToolServicesAndRoutes 校验 Web 工具的 Service 与代理路由都引用已声明组件和端口。
func validateToolServicesAndRoutes(spec *ToolResourceSpec) error {
	if spec == nil || len(spec.Components) == 0 {
		if len(spec.Services) > 0 || len(spec.Routes) > 0 {
			return apperr.ErrSandboxToolCreateInvalid
		}
		return nil
	}
	componentPorts := componentPortMap(spec.Components)
	servicePorts := map[string]map[string]struct{}{}
	for i := range spec.Services {
		service := &spec.Services[i]
		service.Name = strings.TrimSpace(service.Name)
		service.Component = strings.TrimSpace(service.Component)
		if !mountNamePattern.MatchString(service.Name) || len(service.Ports) == 0 {
			return apperr.ErrSandboxToolCreateInvalid
		}
		ports, ok := componentPorts[service.Component]
		if !ok {
			return apperr.ErrSandboxToolCreateInvalid
		}
		declared := map[string]struct{}{}
		for j := range service.Ports {
			port := &service.Ports[j]
			port.Name = strings.TrimSpace(port.Name)
			port.TargetPort = strings.TrimSpace(port.TargetPort)
			port.Protocol = strings.ToUpper(strings.TrimSpace(port.Protocol))
			if port.Protocol == "" {
				port.Protocol = "TCP"
			}
			if !portNamePattern.MatchString(port.Name) || port.Port <= 0 || port.Port > 65535 ||
				port.TargetPort == "" || (port.Protocol != "TCP" && port.Protocol != "UDP") {
				return apperr.ErrSandboxToolCreateInvalid
			}
			if _, ok := ports[port.TargetPort]; !ok {
				return apperr.ErrSandboxToolCreateInvalid
			}
			if _, exists := declared[port.Name]; exists {
				return apperr.ErrSandboxToolCreateInvalid
			}
			declared[port.Name] = struct{}{}
		}
		if _, exists := servicePorts[service.Name]; exists {
			return apperr.ErrSandboxToolCreateInvalid
		}
		servicePorts[service.Name] = declared
	}
	for i := range spec.Routes {
		route := &spec.Routes[i]
		route.PathPrefix = strings.TrimSpace(route.PathPrefix)
		route.Service = strings.TrimSpace(route.Service)
		route.Port = strings.TrimSpace(route.Port)
		if !validToolRoutePrefix(route.PathPrefix) {
			return apperr.ErrSandboxToolCreateInvalid
		}
		ports, ok := servicePorts[route.Service]
		if !ok {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if _, ok := ports[route.Port]; !ok {
			return apperr.ErrSandboxToolCreateInvalid
		}
	}
	return nil
}

// componentPortMap 汇总组件名称到端口名称的索引。
func componentPortMap(components []workload.ComponentSpec) map[string]map[string]int32 {
	out := map[string]map[string]int32{}
	for _, component := range components {
		ports := map[string]int32{}
		for _, port := range component.Ports {
			ports[port.Name] = port.ContainerPort
		}
		out[component.Name] = ports
	}
	return out
}

// validToolRoutePrefix 校验工具代理前缀必须是绝对路径且不能包含路径穿越。
func validToolRoutePrefix(prefix string) bool {
	if prefix == "" || !strings.HasPrefix(prefix, "/") || strings.Contains(prefix, "\\") {
		return false
	}
	clean := path.Clean(prefix)
	return clean == prefix && !strings.Contains(prefix, "/../")
}

// validateQuotaForCreate 校验本次创建是否超过单沙箱资源、租户并发、保活和快照上限。
func validateQuotaForCreate(req CreateSandboxInputModel, quota TenantQuota, active int64, cfg config.SandboxConfig, adapter AdapterSpec, tools []Tool) error {
	if active >= int64(quota.MaxConcurrentSandbox) {
		return apperr.ErrSandboxQuotaExceeded
	}
	if err := validateSingleSandboxResourceLimit(adapter, tools, cfg); err != nil {
		return err
	}
	if err := validateTenantResourceCapacity(quota, active+1, cfg); err != nil {
		return err
	}
	if req.KeepAlive {
		if quota.MaxKeepaliveMin <= 0 || req.KeepAliveMinutes <= 0 || req.KeepAliveMinutes > quota.MaxKeepaliveMin {
			return apperr.ErrSandboxKeepaliveQuotaExceeded
		}
	}
	if req.SnapshotEnabled {
		if quota.MaxSnapshotRetentionMin <= 0 || req.SnapshotRetentionMinutes <= 0 || req.SnapshotRetentionMinutes > quota.MaxSnapshotRetentionMin {
			return apperr.ErrSandboxSnapshotQuotaExceeded
		}
	}
	return nil
}

// validateSingleSandboxResourceLimit 按 runtime/tool 声明式规格汇总资源,在写入 K8s 前拒绝超过 Namespace 上限的组合。
func validateSingleSandboxResourceLimit(adapter AdapterSpec, tools []Tool, cfg config.SandboxConfig) error {
	usage, err := sandboxDeclaredResourceUsage(adapter, tools, cfg)
	if err != nil {
		return apperr.ErrSandboxQuotaInvalid.WithCause(err)
	}
	maxCPU, err := resource.ParseQuantity(cfg.MaxCPU)
	if err != nil {
		return apperr.ErrSandboxQuotaInvalid.WithCause(err)
	}
	maxMemory, err := resource.ParseQuantity(cfg.MaxMemory)
	if err != nil {
		return apperr.ErrSandboxQuotaInvalid.WithCause(err)
	}
	maxPods, err := resource.ParseQuantity(cfg.MaxPods)
	if err != nil {
		return apperr.ErrSandboxQuotaInvalid.WithCause(err)
	}
	if usage.RequestCPUMilli > maxCPU.MilliValue() || usage.LimitCPUMilli > maxCPU.MilliValue() {
		return apperr.ErrSandboxResourceQuotaExceeded
	}
	if usage.RequestMemoryBytes > maxMemory.Value() || usage.LimitMemoryBytes > maxMemory.Value() {
		return apperr.ErrSandboxResourceQuotaExceeded
	}
	if usage.PodCount > maxPods.Value() {
		return apperr.ErrSandboxResourceQuotaExceeded
	}
	return nil
}

type declaredResourceUsage struct {
	RequestCPUMilli    int64
	LimitCPUMilli      int64
	RequestMemoryBytes int64
	LimitMemoryBytes   int64
	PodCount           int64
}

// sandboxDeclaredResourceUsage 汇总默认拓扑、显式 Pod 组和会创建 Pod 的工具声明式资源。
func sandboxDeclaredResourceUsage(adapter AdapterSpec, tools []Tool, cfg config.SandboxConfig) (declaredResourceUsage, error) {
	usage := declaredResourceUsage{PodCount: int64(len(podTopologyForAdapter(adapter)))}
	for _, pod := range podTopologyForAdapter(adapter) {
		for _, container := range pod.Containers {
			if err := addDeclaredContainerResources(&usage, container.Resources, cfg); err != nil {
				return declaredResourceUsage{}, err
			}
		}
	}
	for _, tool := range tools {
		if tool.Kind != SandboxToolKindWebEmbed && tool.Kind != SandboxToolKindCommand {
			continue
		}
		usage.PodCount += int64(len(tool.ResourceSpec.Components))
		for _, component := range tool.ResourceSpec.Components {
			if err := addDeclaredContainerResources(&usage, component.Resources, cfg); err != nil {
				return declaredResourceUsage{}, err
			}
		}
	}
	return usage, nil
}

// addDeclaredContainerResources 按与编排器一致的默认值口径累加单容器 requests/limits。
func addDeclaredContainerResources(usage *declaredResourceUsage, spec workload.ResourceSpec, cfg config.SandboxConfig) error {
	reqCPU, err := resource.ParseQuantity(valueOrDefault(spec.Requests["cpu"], cfg.DefaultReqCPU))
	if err != nil {
		return err
	}
	reqMemory, err := resource.ParseQuantity(valueOrDefault(spec.Requests["memory"], cfg.DefaultReqMemory))
	if err != nil {
		return err
	}
	limitCPU, err := resource.ParseQuantity(valueOrDefault(spec.Limits["cpu"], cfg.DefaultCPU))
	if err != nil {
		return err
	}
	limitMemory, err := resource.ParseQuantity(valueOrDefault(spec.Limits["memory"], cfg.DefaultMemory))
	if err != nil {
		return err
	}
	usage.RequestCPUMilli += reqCPU.MilliValue()
	usage.RequestMemoryBytes += reqMemory.Value()
	usage.LimitCPUMilli += limitCPU.MilliValue()
	usage.LimitMemoryBytes += limitMemory.Value()
	return nil
}

// validateTenantResourceCapacity 用单沙箱 Namespace 硬上限估算最坏占用,防止租户绕过 CPU/内存总配额。
func validateTenantResourceCapacity(quota TenantQuota, sandboxCount int64, cfg config.SandboxConfig) error {
	cpu, err := resource.ParseQuantity(cfg.MaxCPU)
	if err != nil {
		return apperr.ErrSandboxQuotaInvalid.WithCause(err)
	}
	memory, err := resource.ParseQuantity(cfg.MaxMemory)
	if err != nil {
		return apperr.ErrSandboxQuotaInvalid.WithCause(err)
	}
	if cpu.MilliValue()*sandboxCount > int64(quota.MaxCPU)*1000 {
		return apperr.ErrSandboxResourceQuotaExceeded
	}
	memoryMiB := memory.Value() / (1024 * 1024)
	if memoryMiB*sandboxCount > int64(quota.MaxMemoryMB) {
		return apperr.ErrSandboxResourceQuotaExceeded
	}
	return nil
}

// validateStateTransition 校验沙箱生命周期状态机不允许越级或回退到不安全状态。
func validateStateTransition(from, to int16) error {
	if from == to {
		return nil
	}
	allowed := map[int16]map[int16]struct{}{
		SandboxStatusCreating: {
			SandboxStatusReady:     {},
			SandboxStatusRecycling: {},
			SandboxStatusFailed:    {},
		},
		SandboxStatusReady: {
			SandboxStatusRunning:   {},
			SandboxStatusPaused:    {},
			SandboxStatusRecycling: {},
			SandboxStatusFailed:    {},
		},
		SandboxStatusRunning: {
			SandboxStatusIdle:      {},
			SandboxStatusPaused:    {},
			SandboxStatusRecycling: {},
			SandboxStatusFailed:    {},
		},
		SandboxStatusIdle: {
			SandboxStatusRunning:   {},
			SandboxStatusPaused:    {},
			SandboxStatusRecycling: {},
			SandboxStatusFailed:    {},
		},
		SandboxStatusPaused: {
			SandboxStatusRunning:   {},
			SandboxStatusRecycling: {},
		},
		SandboxStatusFailed: {
			SandboxStatusRecycling: {},
		},
		SandboxStatusRecycling: {
			SandboxStatusDestroyed: {},
			SandboxStatusFailed:    {},
		},
	}
	if _, ok := allowed[from][to]; !ok {
		return apperr.ErrSandboxStateInvalid
	}
	return nil
}

// validateWorkspacePath 校验文件路径限定在 workspace 内,拒绝绝对路径和路径穿越。
func validateWorkspacePath(relativePath string) (string, error) {
	cleanInput := strings.TrimSpace(relativePath)
	if cleanInput == "" {
		return "", apperr.ErrSandboxFileInvalid
	}
	if filepath.IsAbs(cleanInput) || strings.Contains(cleanInput, "\\") {
		return "", apperr.ErrSandboxFileInvalid
	}
	for _, segment := range strings.Split(cleanInput, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", apperr.ErrSandboxFileInvalid
		}
	}
	out := path.Clean(cleanInput)
	if out == "." || out == ".." || strings.HasPrefix(out, "../") || strings.Contains(out, "/../") {
		return "", apperr.ErrSandboxFileInvalid
	}
	return out, nil
}

// validateWorkspaceListPath 校验目录列表路径,允许空路径明确表示工作区根目录。
func validateWorkspaceListPath(relativePath string) (string, error) {
	if strings.TrimSpace(relativePath) == "" {
		return ".", nil
	}
	return validateWorkspacePath(relativePath)
}

// normalizeAndValidateAdapterSpec 为探针补配置默认值并校验容器、端口、环境变量安全边界。
func normalizeAndValidateAdapterSpec(spec *AdapterSpec, cfg config.SandboxConfig) error {
	if spec == nil || strings.TrimSpace(spec.WorkspaceDir) == "" || !strings.HasPrefix(spec.WorkspaceDir, "/") {
		return apperr.ErrSandboxAdapterSpecInvalid
	}
	if err := normalizeAndValidateVolumeDomains(spec); err != nil {
		return err
	}
	if err := validateWorkspaceOps(spec.WorkspaceOps); err != nil {
		return err
	}
	if err := validateCapabilityCommands(spec, cfg); err != nil {
		return err
	}
	if err := validateContainerSpec(&spec.RuntimeContainer, cfg); err != nil {
		return err
	}
	if err := validatePrivateArchiveExecutionTarget(spec); err != nil {
		return err
	}
	ports := map[string]struct{}{}
	for _, port := range spec.RuntimeContainer.Ports {
		if _, exists := ports[port.Name]; exists {
			return apperr.ErrSandboxContainerSpecInvalid
		}
		ports[port.Name] = struct{}{}
	}
	for i := range spec.InfraSidecars {
		if err := validateContainerSpec(&spec.InfraSidecars[i], cfg); err != nil {
			return err
		}
		if err := validateInfraSidecarImage(spec.InfraSidecars[i], cfg); err != nil {
			return err
		}
		for _, port := range spec.InfraSidecars[i].Ports {
			if _, exists := ports[port.Name]; exists {
				return apperr.ErrSandboxContainerSpecInvalid
			}
			ports[port.Name] = struct{}{}
		}
	}
	if err := validateRuntimeContainerNames(spec); err != nil {
		return err
	}
	if err := validatePodTopology(spec, cfg); err != nil {
		return err
	}
	if err := validateNetworkRules(spec); err != nil {
		return err
	}
	return nil
}

// validateRuntimeContainerNames 保证运行时清单内所有容器名全局唯一,避免 Pod 引用表覆盖错误容器。
func validateRuntimeContainerNames(spec *AdapterSpec) error {
	seen := map[string]struct{}{}
	containers := append([]workload.ComponentSpec{spec.RuntimeContainer}, spec.InfraSidecars...)
	for _, container := range containers {
		if !mountNamePattern.MatchString(container.Name) {
			return apperr.ErrSandboxContainerSpecInvalid
		}
		if _, exists := seen[container.Name]; exists {
			return apperr.ErrSandboxContainerSpecInvalid
		}
		seen[container.Name] = struct{}{}
	}
	return nil
}

// validatePodTopology 校验显式 Pod 组拓扑,缺省时使用单 Pod 多容器口径。
func validatePodTopology(spec *AdapterSpec, cfg config.SandboxConfig) error {
	if len(spec.Pods) == 0 {
		return nil
	}
	containerDefs := declaredContainerMap(*spec)
	seenPods := map[string]struct{}{}
	runtimeContainerSeen := false
	for i := range spec.Pods {
		pod := &spec.Pods[i]
		pod.Name = strings.TrimSpace(pod.Name)
		if !mountNamePattern.MatchString(pod.Name) || len(pod.Containers) == 0 {
			return apperr.ErrSandboxPodTopologyInvalid
		}
		if _, exists := seenPods[pod.Name]; exists {
			return apperr.ErrSandboxPodTopologyInvalid
		}
		seenPods[pod.Name] = struct{}{}
		containerNames := map[string]struct{}{}
		ports := map[int32]struct{}{}
		for j := range pod.Containers {
			container := &pod.Containers[j]
			container.Name = strings.TrimSpace(container.Name)
			if !podContainerIsReferenceOnly(*container) {
				return apperr.ErrSandboxPodTopologyInvalid
			}
			declared, ok := containerDefs[container.Name]
			if !ok {
				return apperr.ErrSandboxPodTopologyInvalid
			}
			if declared.Name == spec.RuntimeContainer.Name {
				runtimeContainerSeen = true
				if hasVolumeDomain(*spec, VolumeDomainJudgePrivate) && studentAccessibleContainer(declared) {
					return apperr.ErrSandboxPrivateDomainInvalid
				}
			}
			if _, exists := containerNames[declared.Name]; exists {
				return apperr.ErrSandboxPodTopologyInvalid
			}
			containerNames[declared.Name] = struct{}{}
			for _, port := range declared.Ports {
				if _, exists := ports[port.ContainerPort]; exists {
					return apperr.ErrSandboxPodTopologyInvalid
				}
				ports[port.ContainerPort] = struct{}{}
			}
			pod.Containers[j] = declared
		}
	}
	if !runtimeContainerSeen {
		return apperr.ErrSandboxPodTopologyInvalid
	}
	return nil
}

// declaredContainerMap 建立运行时主容器和 infra sidecar 的唯一声明表,显式 Pod 组只能引用这里的容器。
func declaredContainerMap(spec AdapterSpec) map[string]workload.ComponentSpec {
	out := map[string]workload.ComponentSpec{spec.RuntimeContainer.Name: spec.RuntimeContainer}
	for _, container := range spec.InfraSidecars {
		out[container.Name] = container
	}
	return out
}

// podContainerIsReferenceOnly 保证 pods[].containers 只是引用,避免同一容器在 adapter_spec 中出现两套声明。
func podContainerIsReferenceOnly(spec workload.ComponentSpec) bool {
	return spec.Name != "" &&
		strings.TrimSpace(spec.ImageURL) == "" &&
		len(spec.Command) == 0 &&
		len(spec.Args) == 0 &&
		len(spec.Env) == 0 &&
		len(spec.Ports) == 0 &&
		len(spec.Resources.Requests) == 0 &&
		len(spec.Resources.Limits) == 0 &&
		strings.TrimSpace(spec.ReadinessProbe.Type) == "" &&
		strings.TrimSpace(spec.LivenessProbe.Type) == "" &&
		strings.TrimSpace(spec.Workdir) == "" &&
		spec.ReadOnlyRootFilesystem == nil &&
		len(spec.Labels) == 0 &&
		spec.MountWorkspace == nil &&
		len(spec.EphemeralMounts) == 0
}

// validateNetworkRules 校验同沙箱 Pod 互通必须显式引用已声明的目标端口。
func validateNetworkRules(spec *AdapterSpec) error {
	if len(spec.NetworkRules) == 0 {
		return nil
	}
	pods := podTopologyForAdapter(*spec)
	podPorts := podPortMap(pods)
	seenRules := map[string]struct{}{}
	for i := range spec.NetworkRules {
		rule := &spec.NetworkRules[i]
		rule.Name = strings.TrimSpace(rule.Name)
		rule.From = strings.TrimSpace(rule.From)
		rule.To = strings.TrimSpace(rule.To)
		if !mountNamePattern.MatchString(rule.Name) || rule.From == "" || rule.To == "" || len(rule.Ports) == 0 {
			return apperr.ErrSandboxNetworkPolicyInvalid
		}
		if _, exists := seenRules[rule.Name]; exists {
			return apperr.ErrSandboxNetworkPolicyInvalid
		}
		seenRules[rule.Name] = struct{}{}
		if _, ok := podPorts[rule.From]; !ok {
			return apperr.ErrSandboxNetworkPolicyInvalid
		}
		targetPorts, ok := podPorts[rule.To]
		if !ok {
			return apperr.ErrSandboxNetworkPolicyInvalid
		}
		// 每条规则只放行目标 Pod 已公开的端口,禁止用空端口表达宽泛互通。
		seenPorts := map[int32]struct{}{}
		for j := range rule.Ports {
			ref := &rule.Ports[j]
			ref.Name = strings.TrimSpace(ref.Name)
			if ref.Name != "" {
				resolved, ok := targetPorts[ref.Name]
				if !ok {
					return apperr.ErrSandboxNetworkPolicyInvalid
				}
				ref.Port = resolved
			}
			if ref.Port <= 0 {
				return apperr.ErrSandboxNetworkPolicyInvalid
			}
			if !networkPortDeclared(targetPorts, ref.Port) {
				return apperr.ErrSandboxNetworkPolicyInvalid
			}
			if _, ok := seenPorts[ref.Port]; ok {
				return apperr.ErrSandboxNetworkPolicyInvalid
			}
			seenPorts[ref.Port] = struct{}{}
		}
	}
	return nil
}

// validateToolNetworkRules 校验工具网络规则自身格式,目标 Pod 引用在沙箱创建时结合运行时拓扑校验。
func validateToolNetworkRules(spec *ToolResourceSpec) error {
	seenRules := map[string]struct{}{}
	componentPorts := componentPortMap(spec.Components)
	for i := range spec.NetworkRules {
		rule := &spec.NetworkRules[i]
		rule.Name = strings.TrimSpace(rule.Name)
		rule.From = strings.TrimSpace(rule.From)
		rule.To = strings.TrimSpace(rule.To)
		if !mountNamePattern.MatchString(rule.Name) || rule.From == "" || rule.To == "" || len(rule.Ports) == 0 {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if _, exists := seenRules[rule.Name]; exists {
			return apperr.ErrSandboxToolCreateInvalid
		}
		seenRules[rule.Name] = struct{}{}
		if _, ok := componentPorts[rule.From]; !ok {
			return apperr.ErrSandboxToolCreateInvalid
		}
		for j := range rule.Ports {
			ref := &rule.Ports[j]
			ref.Name = strings.TrimSpace(ref.Name)
			if ref.Name == "" && ref.Port <= 0 {
				return apperr.ErrSandboxToolCreateInvalid
			}
		}
	}
	return nil
}

// toolHasContainerSpec 判断非 web 工具是否误带容器运行配置,终端和平台内置工具不创建 sidecar。
func toolHasContainerSpec(spec ToolResourceSpec) bool {
	return len(spec.Components) > 0 ||
		len(spec.Services) > 0 ||
		len(spec.Routes) > 0
}

// validateToolNetworkRulesForRuntime 校验工具网络规则只能访问运行时拓扑中已声明的目标端口。
func validateToolNetworkRulesForRuntime(tool Tool, adapter AdapterSpec) error {
	podPorts := podPortMap(podTopologyForAdapter(adapter))
	for i := range tool.ResourceSpec.NetworkRules {
		rule := &tool.ResourceSpec.NetworkRules[i]
		targetPorts, ok := podPorts[rule.To]
		if !ok {
			return apperr.ErrSandboxToolIncompatible
		}
		for j := range rule.Ports {
			ref := &rule.Ports[j]
			if ref.Name != "" {
				resolved, ok := targetPorts[ref.Name]
				if !ok {
					return apperr.ErrSandboxToolIncompatible
				}
				ref.Port = resolved
			}
			if !networkPortDeclared(targetPorts, ref.Port) {
				return apperr.ErrSandboxToolIncompatible
			}
		}
	}
	return nil
}

// podPortMap 汇总 Pod 名称到该 Pod 已声明端口的索引。
func podPortMap(pods []workload.PodSpec) map[string]map[string]int32 {
	podPorts := map[string]map[string]int32{}
	for _, pod := range pods {
		ports := map[string]int32{}
		for _, container := range pod.Containers {
			for _, port := range container.Ports {
				ports[port.Name] = port.ContainerPort
			}
		}
		podPorts[pod.Name] = ports
	}
	return podPorts
}

// networkPortDeclared 判断数字端口是否属于目标 Pod 已声明端口,避免规则放行未公开端口。
func networkPortDeclared(ports map[string]int32, port int32) bool {
	for _, declared := range ports {
		if declared == port {
			return true
		}
	}
	return false
}

// podTopologyForAdapter 返回 adapter 归一化后的运行时 Pod 组,供校验与服务解析复用。
func podTopologyForAdapter(spec AdapterSpec) []workload.PodSpec {
	if len(spec.Pods) > 0 {
		return spec.Pods
	}
	containers := []workload.ComponentSpec{spec.RuntimeContainer}
	containers = append(containers, spec.InfraSidecars...)
	return []workload.PodSpec{{Name: "sandbox", Containers: containers}}
}

// validatePrivateArchiveExecutionTarget 保证隐藏判题域只会进入非学生入口的执行容器。
func validatePrivateArchiveExecutionTarget(spec *AdapterSpec) error {
	if !hasVolumeDomain(*spec, VolumeDomainJudgePrivate) {
		return nil
	}
	if studentAccessibleContainer(spec.RuntimeContainer) {
		return apperr.ErrSandboxPrivateDomainInvalid
	}
	return nil
}

// hasVolumeDomain 判断适配器是否声明指定卷安全域。
func hasVolumeDomain(spec AdapterSpec, name string) bool {
	for _, domain := range spec.VolumeDomains {
		if domain.Name == name {
			return true
		}
	}
	return false
}

// normalizeAndValidateVolumeDomains 校验沙箱卷安全域,防止运行态或私有数据落入学生工作区。
func normalizeAndValidateVolumeDomains(spec *AdapterSpec) error {
	if len(spec.VolumeDomains) == 0 {
		spec.VolumeDomains = []VolumeDomainSpec{
			{Name: VolumeDomainWorkspace, MountPath: spec.WorkspaceDir, StudentAccess: VolumeAccessReadWrite, Persistence: VolumePersistenceMinioCode, SnapshotScope: VolumeSnapshotAlways},
			{Name: VolumeDomainRuntimeState, MountPath: "/runtime-state", StudentAccess: VolumeAccessNone, Persistence: VolumePersistenceEphemeral, SnapshotScope: VolumeSnapshotEnabled},
		}
	}
	seen := map[string]VolumeDomainSpec{}
	workspaceSeen := false
	for i := range spec.VolumeDomains {
		domain := &spec.VolumeDomains[i]
		domain.Name = strings.TrimSpace(domain.Name)
		domain.MountPath = path.Clean(strings.TrimSpace(domain.MountPath))
		domain.StudentAccess = strings.TrimSpace(domain.StudentAccess)
		domain.Persistence = strings.TrimSpace(domain.Persistence)
		domain.SnapshotScope = strings.TrimSpace(domain.SnapshotScope)
		if domain.Name == "" || !strings.HasPrefix(domain.MountPath, "/") || domain.MountPath == "/" {
			return apperr.ErrSandboxVolumeDomainInvalid
		}
		if _, ok := seen[domain.Name]; ok {
			return apperr.ErrSandboxVolumeDomainInvalid
		}
		if !validVolumeAccess(domain.StudentAccess) || !validVolumePersistence(domain.Persistence) || !validVolumeSnapshotScope(domain.SnapshotScope) {
			return apperr.ErrSandboxVolumeDomainInvalid
		}
		if domain.Name == VolumeDomainWorkspace {
			workspaceSeen = true
			if domain.MountPath != path.Clean(spec.WorkspaceDir) || domain.StudentAccess != VolumeAccessReadWrite || domain.Persistence != VolumePersistenceMinioCode {
				return apperr.ErrSandboxVolumeDomainInvalid
			}
		}
		if domain.Name == VolumeDomainRuntimeState && domain.StudentAccess != VolumeAccessNone {
			return apperr.ErrSandboxVolumeDomainInvalid
		}
		if domain.Name == VolumeDomainJudgePrivate && (domain.StudentAccess != VolumeAccessNone || domain.SnapshotScope != VolumeSnapshotNever) {
			return apperr.ErrSandboxPrivateDomainInvalid
		}
		for _, existing := range seen {
			if volumePathOverlaps(domain.MountPath, existing.MountPath) {
				return apperr.ErrSandboxVolumeDomainInvalid
			}
		}
		seen[domain.Name] = *domain
	}
	if !workspaceSeen {
		return apperr.ErrSandboxVolumeDomainInvalid
	}
	return nil
}

// validVolumeAccess 校验卷域学生访问级别枚举。
func validVolumeAccess(access string) bool {
	return access == VolumeAccessNone || access == VolumeAccessReadOnly || access == VolumeAccessReadWrite
}

// validVolumePersistence 校验卷域持久化策略枚举。
func validVolumePersistence(persistence string) bool {
	return persistence == VolumePersistenceMinioCode || persistence == VolumePersistenceEphemeral || persistence == VolumePersistenceSnapshot
}

// validVolumeSnapshotScope 校验卷域快照策略枚举。
func validVolumeSnapshotScope(scope string) bool {
	return scope == VolumeSnapshotNever || scope == VolumeSnapshotAlways || scope == VolumeSnapshotEnabled
}

// volumePathOverlaps 判断两个挂载路径是否存在父子或相同关系。
func volumePathOverlaps(left, right string) bool {
	left = path.Clean(left)
	right = path.Clean(right)
	return left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}

// validateInfraSidecarImage 校验运行时协同容器镜像命中受控证明清单。
func validateInfraSidecarImage(spec workload.ComponentSpec, cfg config.SandboxConfig) error {
	imageURL := strings.TrimSpace(spec.ImageURL)
	digest := digestFromImageURL(imageURL)
	if imageURL == "" || digest == "" || !imageAttested(cfg, imageURL, digest) {
		return apperr.ErrSandboxSidecarImageInvalid
	}
	return nil
}

// digestFromImageURL 从 image@sha256:... 提取不可变 digest。
func digestFromImageURL(imageURL string) string {
	parts := strings.Split(strings.TrimSpace(imageURL), "@")
	if len(parts) != 2 || !strings.HasPrefix(parts[1], "sha256:") {
		return ""
	}
	return parts[1]
}

// validateWorkspaceOps 校验运行时声明了文件、归档、脚本、自检和终端所需的受控命令。
func validateWorkspaceOps(ops WorkspaceOps) error {
	helperCommands := [][]string{
		ops.ReadFile,
		ops.WriteFile,
		ops.ListFiles,
		ops.PackTar,
		ops.UnpackTar,
		ops.RunScript,
		ops.Selftest,
	}
	for _, command := range helperCommands {
		if !safeNonShellCommand(command) {
			return apperr.ErrSandboxWorkspaceOpsInvalid
		}
	}
	if !safeCommand(ops.Terminal) {
		return apperr.ErrSandboxWorkspaceOpsInvalid
	}
	return nil
}

// validateCapabilityCommands 校验 L2/L3 标准链能力有真实执行入口,避免只登记字段却无法 deploy/tx/query/reset。
func validateCapabilityCommands(spec *AdapterSpec, cfg config.SandboxConfig) error {
	if !hasCapabilityCommands(spec.CapabilityCommands) {
		return nil
	}
	commands := []*CapabilityCommandSpec{
		&spec.CapabilityCommands.Deploy,
		&spec.CapabilityCommands.Tx,
		&spec.CapabilityCommands.Query,
		&spec.CapabilityCommands.Reset,
	}
	for _, command := range commands {
		if !safeNonShellCommand(command.Command) {
			return apperr.ErrSandboxCapabilityCommandInvalid
		}
		if command.TimeoutSeconds < 0 {
			return apperr.ErrSandboxCapabilityCommandInvalid
		}
		if command.TimeoutSeconds == 0 {
			command.TimeoutSeconds = int32(cfg.ChainRPCTimeoutSeconds)
		}
	}
	return nil
}

// hasCapabilityCommands 判断运行时是否声明完整 L2 能力命令集合。
func hasCapabilityCommands(commands CapabilityCommandSet) bool {
	return len(commands.Deploy.Command) > 0 &&
		len(commands.Tx.Command) > 0 &&
		len(commands.Query.Command) > 0 &&
		len(commands.Reset.Command) > 0
}

// safeCommand 校验声明式命令为 argv 数组且不含控制字符,终端入口可在受限容器中启动 shell。
func safeCommand(command []string) bool {
	if len(command) == 0 {
		return false
	}
	for _, part := range command {
		if strings.TrimSpace(part) == "" {
			return false
		}
		if strings.ContainsAny(part, "\x00\r\n") {
			return false
		}
	}
	return true
}

// safeNonShellCommand 禁止内部 helper、判题和链能力命令以 shell 解释器作为入口,避免字符串脚本成为注入面。
func safeNonShellCommand(command []string) bool {
	if !safeCommand(command) {
		return false
	}
	executable := strings.ToLower(path.Base(strings.TrimSpace(command[0])))
	_, blocked := shellCommands[executable]
	return !blocked
}

// validateContainerSpec 校验单个容器声明不会绕过安全上下文或硬编码无效探针。
func validateContainerSpec(spec *workload.ComponentSpec, cfg config.SandboxConfig) error {
	spec.Name = strings.TrimSpace(spec.Name)
	if !mountNamePattern.MatchString(spec.Name) || len(spec.Ports) == 0 {
		return apperr.ErrSandboxContainerSpecInvalid
	}
	if err := validateLiteralEnv(spec.Env); err != nil {
		return err
	}
	if err := validateResourceSpec(spec.Resources); err != nil {
		return err
	}
	if err := validatePortSpecs(spec.Ports); err != nil {
		return err
	}
	normalizeProbe(&spec.ReadinessProbe, cfg)
	normalizeProbe(&spec.LivenessProbe, cfg)
	portNames := declaredPortNames(spec.Ports)
	for _, probe := range []*workload.ProbeSpec{&spec.ReadinessProbe, &spec.LivenessProbe} {
		if err := validateProbeSpec(probe, portNames); err != nil {
			return err
		}
	}
	return nil
}

// validatePortSpecs 校验容器端口声明可被探针、NetworkPolicy 和工具规则稳定引用。
func validatePortSpecs(ports []workload.PortSpec) error {
	seen := map[string]struct{}{}
	for i := range ports {
		port := &ports[i]
		port.Name = strings.TrimSpace(port.Name)
		port.Protocol = strings.ToUpper(strings.TrimSpace(port.Protocol))
		if port.Protocol == "" {
			port.Protocol = "TCP"
		}
		if !portNamePattern.MatchString(port.Name) || port.ContainerPort <= 0 || port.ContainerPort > 65535 ||
			port.ServicePort < 0 || port.ServicePort > 65535 || (port.Protocol != "TCP" && port.Protocol != "UDP") {
			return apperr.ErrSandboxContainerSpecInvalid
		}
		if _, exists := seen[port.Name]; exists {
			return apperr.ErrSandboxContainerSpecInvalid
		}
		seen[port.Name] = struct{}{}
	}
	return nil
}

// declaredPortNames 建立端口名索引,供探针和网络规则引用校验。
func declaredPortNames(ports []workload.PortSpec) map[string]struct{} {
	out := map[string]struct{}{}
	for _, port := range ports {
		out[port.Name] = struct{}{}
	}
	return out
}

// validateProbeSpec 校验探针类型与目标,exec 探针也必须走 argv 且不能以 shell 字符串入口执行。
func validateProbeSpec(probe *workload.ProbeSpec, portNames map[string]struct{}) error {
	if probe == nil || strings.TrimSpace(probe.Type) == "" {
		return nil
	}
	probe.Type = strings.ToLower(strings.TrimSpace(probe.Type))
	probe.Port = strings.TrimSpace(probe.Port)
	probe.Path = strings.TrimSpace(probe.Path)
	switch probe.Type {
	case "tcp":
		if _, ok := portNames[probe.Port]; !ok {
			return apperr.ErrSandboxProbeSpecInvalid
		}
	case "http":
		if _, ok := portNames[probe.Port]; !ok || !strings.HasPrefix(probe.Path, "/") {
			return apperr.ErrSandboxProbeSpecInvalid
		}
	case "exec":
		if !safeNonShellCommand(probe.Command) {
			return apperr.ErrSandboxProbeSpecInvalid
		}
	default:
		return apperr.ErrSandboxProbeSpecInvalid
	}
	return nil
}

// validateResourceSpec 校验显式资源配置必须同时声明 requests 与 limits,并能被 K8s quantity 解析。
func validateResourceSpec(spec workload.ResourceSpec) error {
	if len(spec.Requests) == 0 && len(spec.Limits) == 0 {
		return nil
	}
	for _, resources := range []map[string]string{spec.Requests, spec.Limits} {
		if strings.TrimSpace(resources["cpu"]) == "" || strings.TrimSpace(resources["memory"]) == "" {
			return apperr.ErrSandboxContainerSpecInvalid
		}
		if _, err := resource.ParseQuantity(resources["cpu"]); err != nil {
			return apperr.ErrSandboxContainerSpecInvalid.WithCause(err)
		}
		if _, err := resource.ParseQuantity(resources["memory"]); err != nil {
			return apperr.ErrSandboxContainerSpecInvalid.WithCause(err)
		}
	}
	return nil
}

// normalizeProbe 从配置补齐探针默认值,避免编排代码硬编码周期和失败阈值。
func normalizeProbe(probe *workload.ProbeSpec, cfg config.SandboxConfig) {
	if probe.PeriodSeconds <= 0 {
		probe.PeriodSeconds = cfg.ProbeDefaultPeriodSeconds
	}
	if probe.FailureThreshold <= 0 {
		probe.FailureThreshold = cfg.ProbeDefaultFailureThreshold
	}
}

// validateLiteralEnv 拒绝密钥式字段进入声明式配置,敏感值必须走平台 Secret。
func validateLiteralEnv(env []workload.EnvVarSpec) error {
	for _, item := range env {
		name := strings.TrimSpace(item.Name)
		if !envNamePattern.MatchString(name) {
			return apperr.ErrSandboxRuntimeEnvInvalid
		}
		upper := strings.ToUpper(name)
		if strings.Contains(upper, "SECRET") || strings.Contains(upper, "TOKEN") || strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "PRIVATE_KEY") {
			return apperr.ErrSandboxRuntimeSecretEnvInvalid
		}
	}
	return nil
}

// validateToolEphemeralMounts 校验工具临时写目录,防止只读 rootfs 例外覆盖工作区或敏感系统目录。
func validateToolEphemeralMounts(mounts []workload.EphemeralMountSpec) error {
	seen := map[string]struct{}{}
	paths := []string{}
	for _, mount := range mounts {
		name := strings.TrimSpace(mount.Name)
		mountPath := path.Clean(strings.TrimSpace(mount.MountPath))
		if !mountNamePattern.MatchString(name) || !strings.HasPrefix(mountPath, "/") || mountPath == "/" {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if _, ok := seen[name]; ok {
			return apperr.ErrSandboxToolCreateInvalid
		}
		// 这些位置属于平台、工作区或系统敏感面,工具缓存卷不得覆盖。
		for _, blocked := range []string{"/workspace", "/runtime-state", "/judge-private", "/var/run", "/etc", "/proc", "/sys", "/dev"} {
			if volumePathOverlaps(mountPath, blocked) {
				return apperr.ErrSandboxToolCreateInvalid
			}
		}
		for _, existing := range paths {
			if volumePathOverlaps(mountPath, existing) {
				return apperr.ErrSandboxToolCreateInvalid
			}
		}
		seen[name] = struct{}{}
		paths = append(paths, mountPath)
	}
	return nil
}

// validBuiltinEndpointTemplate 校验平台内置工具只能指向本模块已有用户路由,并显式绑定 sandbox_id。
func validBuiltinEndpointTemplate(endpoint string) bool {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" || strings.Contains(endpoint, "://") || strings.Contains(endpoint, "\\") {
		return false
	}
	if !strings.Contains(endpoint, "{sandbox_id}") {
		return false
	}
	return strings.HasPrefix(endpoint, "/api/v1/sandbox/sandboxes/{sandbox_id}")
}

// imageAttested 校验镜像 URL 与 digest 命中受控证明清单,请求体不能直接声明扫描结论。
func imageAttested(cfg config.SandboxConfig, imageURL, digest string) bool {
	imageURL = strings.TrimSpace(imageURL)
	digest = strings.TrimSpace(digest)
	if !imageUnderRegistry(imageURL, cfg.ImageRegistry) || digestFromImageURL(imageURL) != digest {
		return false
	}
	for _, item := range cfg.ImageAttestations {
		if item.ImageURL == imageURL && item.Digest == digest && item.CosignVerified && strings.EqualFold(item.TrivyStatus, "passed") {
			return true
		}
	}
	return false
}

// imageUnderRegistry 校验镜像来自平台配置的私有 Harbor 前缀,避免证明清单误配放行外部镜像。
func imageUnderRegistry(imageURL, registry string) bool {
	registry = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(registry, "https://"), "http://"))
	imageURL = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(imageURL, "https://"), "http://"))
	return registry != "" && strings.HasPrefix(imageURL, registry+"/")
}

// shouldMountWorkspace 判定工具组件是否挂载工作区,工具必须显式声明。
func shouldMountWorkspace(spec workload.ComponentSpec) bool {
	return spec.MountWorkspace != nil && *spec.MountWorkspace
}
