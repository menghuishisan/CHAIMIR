package sandbox

import (
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation"
	podresource "k8s.io/component-helpers/resource"
)

var (
	codePattern        = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)
	envNamePattern     = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,63}$`)
	bindingNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,62}$`)
	mountNamePattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	portNamePattern    = regexp.MustCompile(`^[a-z][a-z0-9-]{0,14}$`)
)

func normalizeSandboxCreateRequest(req contracts.SandboxCreateRequest) contracts.SandboxCreateRequest {
	req.InitCodeRef = strings.TrimSpace(req.InitCodeRef)
	req.InitScriptRef = strings.TrimSpace(req.InitScriptRef)
	req.SourceRef = strings.TrimSpace(req.SourceRef)
	req.ScopeRef = strings.TrimSpace(req.ScopeRef)
	return req
}

// validateCreateRequest 校验内部创建沙箱请求的租户、来源和资源开关。
func validateCreateRequest(req contracts.SandboxCreateRequest) error {
	if req.TenantID <= 0 || strings.TrimSpace(req.CompositionSnapshot.Digest) == "" ||
		len(req.CompositionSnapshot.Runtimes) == 0 || len(req.CompositionSnapshot.Spec.Runtimes) == 0 ||
		strings.TrimSpace(req.SourceRef) == "" || strings.TrimSpace(req.ScopeRef) == "" {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	specRuntimes := make(map[string]contracts.CompositionRuntimeRef, len(req.CompositionSnapshot.Spec.Runtimes))
	for _, runtime := range req.CompositionSnapshot.Spec.Runtimes {
		if strings.TrimSpace(runtime.InstanceCode) == "" || strings.TrimSpace(runtime.Code) == "" || strings.TrimSpace(runtime.ImageVersion) == "" {
			return apperr.ErrSandboxCreateRequestInvalid
		}
		specRuntimes[runtime.InstanceCode] = runtime
	}
	if len(specRuntimes) != len(req.CompositionSnapshot.Spec.Runtimes) || len(specRuntimes) != len(req.CompositionSnapshot.Runtimes) {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	for _, runtime := range req.CompositionSnapshot.Runtimes {
		declared, ok := specRuntimes[runtime.InstanceCode]
		if !ok || runtime.RuntimeID <= 0 || runtime.ImageID <= 0 || runtime.Code != declared.Code || runtime.ImageVersion != declared.ImageVersion || len(runtime.AdapterSpec) == 0 {
			return apperr.ErrSandboxCreateRequestInvalid
		}
	}
	for _, component := range req.CompositionSnapshot.Components {
		if component.ComponentID <= 0 || strings.TrimSpace(component.Code) == "" ||
			(component.Category != "infra" && component.Category != "tool") || len(component.ResourceSpec) == 0 {
			return apperr.ErrSandboxCreateRequestInvalid
		}
	}
	if req.OwnerAccountID <= 0 {
		return apperr.ErrSandboxOwnerInvalid
	}
	if _, err := normalizeSandboxSharedAccountIDs(req.OwnerAccountID, req.AuthorizedAccountIDs); err != nil {
		return err
	}
	if !validSourceRef(req.SourceRef) {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	if !auth.ValidScopeRef(req.ScopeRef) {
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

// normalizeSandboxSharedAccountIDs 生成创建事务要写入的唯一非 owner 共享账号集合。
func normalizeSandboxSharedAccountIDs(ownerAccountID int64, authorizedAccountIDs []int64) ([]int64, error) {
	if ownerAccountID <= 0 {
		return nil, apperr.ErrSandboxOwnerInvalid
	}
	seen := map[int64]struct{}{ownerAccountID: {}}
	out := make([]int64, 0, len(authorizedAccountIDs))
	for _, accountID := range authorizedAccountIDs {
		if accountID <= 0 {
			return nil, apperr.ErrSandboxOwnerInvalid
		}
		if _, exists := seen[accountID]; exists {
			continue
		}
		seen[accountID] = struct{}{}
		out = append(out, accountID)
	}
	return out, nil
}

// sandboxAccountAuthorized 判断账号是否为沙箱 owner 或创建时明确记录的共享成员。
func sandboxAccountAuthorized(sb Sandbox, accountID int64) bool {
	if accountID <= 0 {
		return false
	}
	if sb.OwnerAccountID == accountID {
		return true
	}
	for _, sharedAccountID := range sb.SharedAccountIDs {
		if sharedAccountID == accountID {
			return true
		}
	}
	return false
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
	if req.AdapterLevel < RuntimeAdapterLevelHosted || req.AdapterLevel > RuntimeAdapterLevelPlugin || len(req.AdapterSpec) == 0 {
		return AdapterSpec{}, apperr.ErrSandboxAdapterSpecInvalid
	}
	if req.Status != 0 && req.Status != RuntimeStatusOnboarding && req.Status != RuntimeStatusDisabled {
		return AdapterSpec{}, apperr.ErrSandboxRuntimeCreateInvalid
	}
	var spec AdapterSpec
	if err := jsonx.DecodeStrictKnownFields(req.AdapterSpec, &spec); err != nil {
		return AdapterSpec{}, apperr.ErrSandboxAdapterSpecInvalid.WithCause(err)
	}
	if strings.TrimSpace(spec.DisabledReason) != "" {
		if spec.WorkspaceDir != "" || len(spec.VolumeDomains) > 0 || spec.RuntimeContainer.Name != "" ||
			len(spec.InfraSidecars) > 0 || len(spec.Pods) > 0 || len(spec.Services) > 0 || len(spec.Routes) > 0 ||
			len(spec.NetworkRules) > 0 || len(spec.InitAssets) > 0 ||
			len(spec.WorkspaceOps.ReadFile) > 0 || len(spec.CapabilityCommands.Deploy.Command) > 0 {
			return AdapterSpec{}, apperr.ErrSandboxRuntimeCreateInvalid
		}
		return spec, nil
	}
	if err := normalizeAndValidateAdapterSpec(&spec, cfg); err != nil {
		return AdapterSpec{}, err
	}
	if req.AdapterLevel >= RuntimeAdapterLevelStandard {
		key := runtimeCapabilityKey(req.AdapterLevel, req.CapabilityImpl, req.PluginRef)
		if (req.AdapterLevel == RuntimeAdapterLevelPlugin && key == "") ||
			((key == "" || key == BuiltinExecCapability) && !hasCapabilityCommands(spec.CapabilityCommands)) {
			return AdapterSpec{}, apperr.ErrSandboxCapabilityUnavailable
		}
	}
	return spec, nil
}

// validateToolRequest 校验工具定义,确保不同工具类型只走各自唯一声明口径。
func validateToolRequest(req ToolRequest, cfg config.SandboxConfig) (ToolResourceSpec, error) {
	if !codePattern.MatchString(req.Code) || strings.TrimSpace(req.Name) == "" {
		return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
	}
	if req.Kind < SandboxToolKindBuiltin || req.Kind > SandboxToolKindInfra {
		return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
	}
	if req.Status != 0 && req.Status != ToolStatusAvailable && req.Status != ToolStatusDisabled {
		return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
	}
	var spec ToolResourceSpec
	if len(req.ResourceSpec) > 0 {
		if err := jsonx.DecodeStrictKnownFields(req.ResourceSpec, &spec); err != nil {
			return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid.WithCause(err)
		}
	}
	if err := validateToolResourceSpecShape(&spec, req.Kind); err != nil {
		return ToolResourceSpec{}, err
	}
	for i := range spec.Components {
		component := &spec.Components[i]
		if err := validateRuntimeContainerSpec(component, cfg, req.Kind == SandboxToolKindWebEmbed); err != nil {
			return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid.WithCause(err)
		}
		if err := validateToolEphemeralMounts(component.EphemeralMounts); err != nil {
			return ToolResourceSpec{}, err
		}
		if !imageRefAttested(cfg, component.ImageURL) || (req.Kind != SandboxToolKindInfra && component.MountWorkspace == nil) {
			return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
		}
		if req.Kind == SandboxToolKindCommand && (len(component.Ports) != 0 || !workload.ValidNonShellCommand(component.Command)) {
			return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
		}
	}
	return spec, nil
}

// ValidateToolResourceSpecDefinition 校验工具 WorkloadSpec 的类型形态,供运行期注册和迁移 seed 共用同一套领域规则。
func ValidateToolResourceSpecDefinition(raw []byte, kind int16) error {
	var spec ToolResourceSpec
	if len(raw) > 0 {
		if err := jsonx.DecodeStrictKnownFields(raw, &spec); err != nil {
			return apperr.ErrSandboxToolCreateInvalid.WithCause(err)
		}
	}
	return validateToolResourceSpecShape(&spec, kind)
}

// validateToolResourceSpecShape 校验工具类型与组件、服务、路由、网络规则和命令策略之间的唯一合法组合。
func validateToolResourceSpecShape(spec *ToolResourceSpec, kind int16) error {
	if spec == nil {
		return apperr.ErrSandboxToolCreateInvalid
	}
	if strings.TrimSpace(spec.DisabledReason) != "" {
		if len(spec.Components) > 0 || len(spec.Services) > 0 || len(spec.Routes) > 0 ||
			len(spec.NetworkRules) > 0 || commandPolicyConfigured(spec.CommandPolicy) ||
			strings.TrimSpace(spec.BuiltinEndpoint) != "" || strings.TrimSpace(spec.VolumeSource) != "" {
			return apperr.ErrSandboxToolCreateInvalid
		}
		return nil
	}
	switch kind {
	case SandboxToolKindInfra:
		if commandPolicyConfigured(spec.CommandPolicy) || len(spec.Components) == 0 || len(spec.Routes) > 0 || strings.TrimSpace(spec.BuiltinEndpoint) != "" {
			return apperr.ErrSandboxToolCreateInvalid
		}
	case SandboxToolKindWebEmbed:
		if commandPolicyConfigured(spec.CommandPolicy) || len(spec.Components) == 0 || len(spec.Services) == 0 || len(spec.Routes) == 0 {
			return apperr.ErrSandboxToolCreateInvalid
		}
	case SandboxToolKindCommand:
		if len(spec.Components) != 1 || len(spec.Services) > 0 || len(spec.Routes) > 0 || len(spec.NetworkRules) > 0 {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if err := validateCommandToolPolicy(&spec.CommandPolicy); err != nil {
			return err
		}
	case SandboxToolKindBuiltin:
		if commandPolicyConfigured(spec.CommandPolicy) || toolHasContainerSpec(*spec) || len(spec.NetworkRules) > 0 || !validBuiltinEndpointTemplate(spec.BuiltinEndpoint) {
			return apperr.ErrSandboxToolCreateInvalid
		}
	case SandboxToolKindTerminal:
		if commandPolicyConfigured(spec.CommandPolicy) || toolHasContainerSpec(*spec) || len(spec.NetworkRules) > 0 || strings.TrimSpace(spec.BuiltinEndpoint) != "" {
			return apperr.ErrSandboxToolCreateInvalid
		}
	default:
		return apperr.ErrSandboxToolCreateInvalid
	}
	if err := validateToolServicesAndRoutes(spec); err != nil {
		return err
	}
	if err := validateToolNetworkRules(spec); err != nil {
		return err
	}
	if err := validateComponentBindings(spec.Bindings, spec.Components); err != nil {
		return err
	}
	if err := validateToolVolumeSource(*spec); err != nil {
		return apperr.ErrSandboxToolCreateInvalid.WithCause(err)
	}
	for i := range spec.Components {
		component := &spec.Components[i]
		if kind != SandboxToolKindInfra && len(component.WritableMountDomains) > 0 {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if err := validateComponentMountDomains(*component); err != nil {
			return err
		}
		// DaemonSet 由运行时唯一保持容器维持 Ready,工具镜像只作为 initContainer 拉取并自检。
		if component.PrepullHold {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if len(component.PrepullCommand) == 0 || validatePrepullCommand(component.PrepullCommand) != nil {
			return apperr.ErrSandboxToolCreateInvalid
		}
	}
	return nil
}

// validateComponentBindings 校验组件绑定的角色、实际源组件、端点、协议和环境变量契约。
func validateComponentBindings(bindings []ComponentBinding, components []workload.ComponentSpec) error {
	seen := map[string]struct{}{}
	environments := map[string]struct{}{}
	componentNames := map[string]struct{}{}
	for _, component := range components {
		name := strings.TrimSpace(component.Name)
		if name == "" || !mountNamePattern.MatchString(name) {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if _, exists := componentNames[name]; exists {
			return apperr.ErrSandboxToolCreateInvalid
		}
		componentNames[name] = struct{}{}
	}
	for _, binding := range bindings {
		name := strings.TrimSpace(binding.Name)
		sourceComponent := strings.TrimSpace(binding.SourceComponent)
		if name == "" || !bindingNamePattern.MatchString(name) || sourceComponent == "" || !mountNamePattern.MatchString(sourceComponent) || strings.TrimSpace(binding.Capability) == "" || strings.TrimSpace(binding.Endpoint) == "" {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if _, exists := componentNames[sourceComponent]; !exists {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if source := strings.TrimSpace(binding.SourceEndpoint); source != "" && source != "component" {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if binding.Protocol != "TCP" && binding.Protocol != "HTTP" && binding.Protocol != "HTTPS" && binding.Protocol != "WS" && binding.Protocol != "WSS" && binding.Protocol != "GRPC" {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if _, exists := seen[name]; exists {
			return apperr.ErrSandboxToolCreateInvalid
		}
		seen[name] = struct{}{}
		env, err := parseCompositionBinding(binding.ConfigBinding)
		if err != nil {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if _, duplicate := environments[env]; duplicate {
			return apperr.ErrSandboxToolCreateInvalid
		}
		environments[env] = struct{}{}
	}
	return nil
}

func validateCommandToolPolicy(policy *CommandToolPolicy) error {
	if policy == nil || len(policy.AllowedArgv) == 0 || policy.DefaultTimeoutSeconds <= 0 || policy.MaxTimeoutSeconds <= 0 || policy.DefaultTimeoutSeconds > policy.MaxTimeoutSeconds {
		return apperr.ErrSandboxToolCreateInvalid
	}
	seen := map[string]struct{}{}
	for _, argv := range policy.AllowedArgv {
		if !workload.ValidNonShellCommand(argv) {
			return apperr.ErrSandboxToolCreateInvalid
		}
		key := strings.Join(argv, "\x00")
		if _, exists := seen[key]; exists {
			return apperr.ErrSandboxToolCreateInvalid
		}
		seen[key] = struct{}{}
	}
	return nil
}

// commandPolicyConfigured 判断非命令工具是否误带命令策略。
func commandPolicyConfigured(policy CommandToolPolicy) bool {
	return len(policy.AllowedArgv) > 0 || policy.DefaultTimeoutSeconds != 0 || policy.MaxTimeoutSeconds != 0
}

func validatePrepullCommand(command []string) error {
	if len(command) == 0 {
		return nil
	}
	if !workload.ValidCommand(command) {
		return apperr.ErrSandboxToolCreateInvalid
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
	servicePorts := map[string]map[string]string{}
	for i := range spec.Services {
		service := &spec.Services[i]
		service.Name = strings.TrimSpace(service.Name)
		service.Component = strings.TrimSpace(service.Component)
		if len(validation.IsDNS1035Label(service.Name)) != 0 || len(service.Ports) == 0 {
			return apperr.ErrSandboxToolCreateInvalid
		}
		componentIndex := slices.IndexFunc(spec.Components, func(component workload.ComponentSpec) bool { return component.Name == service.Component })
		if componentIndex < 0 {
			return apperr.ErrSandboxToolCreateInvalid
		}
		ports := spec.Components[componentIndex].Ports
		declared := map[string]string{}
		declaredServicePorts := map[string]struct{}{}
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
			targetIndex := slices.IndexFunc(ports, func(target workload.PortSpec) bool { return target.Name == port.TargetPort })
			if targetIndex < 0 {
				return apperr.ErrSandboxToolCreateInvalid
			}
			protocol := strings.ToUpper(strings.TrimSpace(ports[targetIndex].Protocol))
			if protocol == "" {
				protocol = "TCP"
			}
			if protocol != port.Protocol {
				return apperr.ErrSandboxToolCreateInvalid
			}
			if _, exists := declared[port.Name]; exists {
				return apperr.ErrSandboxToolCreateInvalid
			}
			servicePortKey := fmt.Sprintf("%d/%s", port.Port, port.Protocol)
			if _, exists := declaredServicePorts[servicePortKey]; exists {
				return apperr.ErrSandboxToolCreateInvalid
			}
			declared[port.Name] = port.Protocol
			declaredServicePorts[servicePortKey] = struct{}{}
		}
		if _, exists := servicePorts[service.Name]; exists {
			return apperr.ErrSandboxToolCreateInvalid
		}
		servicePorts[service.Name] = declared
	}
	seenRoutes := map[string]bool{}
	for i := range spec.Routes {
		route := &spec.Routes[i]
		route.PathPrefix = strings.TrimSpace(route.PathPrefix)
		route.Service = strings.TrimSpace(route.Service)
		route.Port = strings.TrimSpace(route.Port)
		if !validToolRoutePrefix(route.PathPrefix) || seenRoutes[route.PathPrefix] {
			return apperr.ErrSandboxToolCreateInvalid
		}
		seenRoutes[route.PathPrefix] = true
		ports, ok := servicePorts[route.Service]
		if !ok {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if protocol, ok := ports[route.Port]; !ok || protocol != "TCP" {
			return apperr.ErrSandboxToolCreateInvalid
		}
	}
	return nil
}

func componentPortMap(components []workload.ComponentSpec) map[string]map[string]workload.PortSpec {
	out := map[string]map[string]workload.PortSpec{}
	for _, component := range components {
		ports := map[string]workload.PortSpec{}
		for _, port := range component.Ports {
			ports[port.Name] = port
		}
		out[component.Name] = ports
	}
	return out
}

// validToolRoutePrefix 校验工具代理前缀必须是绝对路径且不能包含路径穿越。
func validToolRoutePrefix(prefix string) bool {
	if prefix == "" || !strings.HasPrefix(prefix, "/") || strings.ContainsAny(prefix, "\\?#") {
		return false
	}
	clean := path.Clean(prefix)
	return clean == prefix && !strings.Contains(prefix, "/../")
}

// validateQuotaForCreate 校验本次创建是否超过单沙箱资源、租户并发、保活和快照上限。
func validateQuotaForCreate(req contracts.SandboxCreateRequest, quota TenantQuota, active int64, cfg config.SandboxConfig, plan CreateSandboxPlan) error {
	if err := validateTenantSandboxCapacity(quota, active+1, cfg); err != nil {
		return err
	}
	if err := validateSingleSandboxResourceLimit(plan, cfg); err != nil {
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

func validateTenantSandboxCapacity(quota TenantQuota, count int64, cfg config.SandboxConfig) error {
	if count > int64(quota.MaxConcurrentSandbox) {
		return apperr.ErrSandboxQuotaExceeded
	}
	return validateTenantResourceCapacity(quota, count, cfg)
}

func validateSingleSandboxResourceLimit(plan CreateSandboxPlan, cfg config.SandboxConfig) error {
	plan.DeferTools = false
	pods, err := (&K8sOrchestrator{cfg: cfg}).podsForPlan(plan)
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
	requests, limits := corev1.ResourceList{}, corev1.ResourceList{}
	for _, pod := range pods {
		for _, usage := range []struct{ total, resources corev1.ResourceList }{
			{requests, podresource.PodRequests(pod, podresource.PodResourcesOptions{})},
			{limits, podresource.PodLimits(pod, podresource.PodResourcesOptions{})},
		} {
			for name, quantity := range usage.resources {
				sum := usage.total[name]
				sum.Add(quantity)
				usage.total[name] = sum
			}
		}
	}
	if requests.Cpu().Cmp(maxCPU) > 0 || limits.Cpu().Cmp(maxCPU) > 0 {
		return apperr.ErrSandboxResourceQuotaExceeded
	}
	if requests.Memory().Cmp(maxMemory) > 0 || limits.Memory().Cmp(maxMemory) > 0 {
		return apperr.ErrSandboxResourceQuotaExceeded
	}
	if int64(len(pods)) > maxPods.Value() {
		return apperr.ErrSandboxResourceQuotaExceeded
	}
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
	// 先按 quantity 累加再比较，避免单位取整及 int64 乘法溢出。
	cpu.Mul(sandboxCount)
	if cpu.Cmp(*resource.NewQuantity(int64(quota.MaxCPU), resource.DecimalSI)) > 0 {
		return apperr.ErrSandboxResourceQuotaExceeded
	}
	memory.Mul(sandboxCount)
	if memory.Cmp(*resource.NewQuantity(int64(quota.MaxMemoryMB)*1024*1024, resource.BinarySI)) > 0 {
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
	if out == "." || out == ".." || strings.HasPrefix(out, "../") || strings.Contains(out, "/../") || workload.IsPrivateWorkspacePath(out) {
		return "", apperr.ErrSandboxFileInvalid
	}
	return out, nil
}

// validateWorkspaceListPath 校验目录列表路径,允许空路径明确表示工作区根目录。
func validateWorkspaceListPath(relativePath string) (string, error) {
	if strings.TrimSpace(relativePath) == "" || strings.TrimSpace(relativePath) == "." {
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
	if err := validatePrivateArchiveOps(spec.PrivateArchiveOps); err != nil {
		return err
	}
	if len(spec.InitAssets) > 0 && validatePrivateArchiveOps(spec.InitAssetOps) != nil {
		return apperr.ErrSandboxInitAssetConfigInvalid
	}
	if err := validateCapabilityCommands(spec, cfg); err != nil {
		return err
	}
	if err := validateAdapterComponentMountDomains(*spec); err != nil {
		return err
	}
	if err := validateRuntimeContainerSpec(&spec.RuntimeContainer, cfg, true); err != nil {
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
		if err := validateRuntimeContainerSpec(&spec.InfraSidecars[i], cfg, false); err != nil {
			return err
		}
		if err := validateInfraSidecarImage(spec.InfraSidecars[i], cfg); err != nil {
			return err
		}
		if err := validatePrepullCommand(spec.InfraSidecars[i].PrepullCommand); err != nil || len(spec.InfraSidecars[i].PrepullCommand) == 0 {
			return apperr.ErrSandboxAdapterSpecInvalid
		}
		for _, port := range spec.InfraSidecars[i].Ports {
			if _, exists := ports[port.Name]; exists {
				return apperr.ErrSandboxContainerSpecInvalid
			}
			ports[port.Name] = struct{}{}
		}
	}
	if err := validateRuntimePrepullHold(spec.InfraSidecars); err != nil {
		return err
	}
	if err := validateRuntimeContainerNames(spec); err != nil {
		return err
	}
	if err := validatePodTopology(spec); err != nil {
		return err
	}
	if err := validateRuntimePodInitContainers(spec, cfg); err != nil {
		return err
	}
	if err := validateAdapterExecTargets(spec); err != nil {
		return err
	}
	if err := validateInitAssets(*spec); err != nil {
		return err
	}
	if err := validateNetworkRules(spec); err != nil {
		return err
	}
	return nil
}

// validateInitAssets 校验运行时清单中的初始化资产只会由专用非学生 sidecar
// 写入受控 runtime-state/public-assets 安全域,并拒绝重复名称与未校验哈希。
func validateInitAssets(spec AdapterSpec) error {
	if len(spec.InitAssets) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(spec.InitAssets))
	for _, asset := range spec.InitAssets {
		name := strings.TrimSpace(asset.Name)
		if name == "" || strings.TrimSpace(asset.SourceRef) == "" || strings.TrimSpace(asset.TargetDomain) == "" {
			return apperr.ErrSandboxInitAssetConfigInvalid
		}
		if _, exists := seen[name]; exists {
			return apperr.ErrSandboxInitAssetConfigInvalid
		}
		seen[name] = struct{}{}
		domain, ok := volumeDomainByName(spec, asset.TargetDomain)
		if !ok || (domain.Name != VolumeDomainRuntimeState && domain.Name != VolumeDomainPublicAssets) || domain.StudentAccess == VolumeAccessReadWrite {
			return apperr.ErrSandboxInitAssetConfigInvalid
		}
		if !pkgcrypto.ValidSHA256Hex(asset.ExpectedSHA256) {
			return apperr.ErrSandboxInitAssetConfigInvalid
		}
		if !componentCanWriteDomain(spec, spec.InitAssetOps.ExecTarget, domain.Name) {
			return apperr.ErrSandboxInitAssetConfigInvalid
		}
	}
	return nil
}

// validateRuntimePrepullHold 保证每个运行时闭包只有一个常驻容器,其余镜像均以 initContainer 自检。
func validateRuntimePrepullHold(sidecars []workload.ComponentSpec) error {
	holdCount := 0
	for _, sidecar := range sidecars {
		if sidecar.PrepullHold {
			holdCount++
		}
	}
	if holdCount != 1 {
		return apperr.ErrSandboxAdapterSpecInvalid
	}
	return nil
}

// validateRuntimeContainerSpec 校验运行时组件声明;主容器必须有服务端口,执行类 sidecar 可无端口但仍受同一安全边界约束。
func validateRuntimeContainerSpec(spec *workload.ComponentSpec, cfg config.SandboxConfig, requirePort bool) error {
	spec.Name = strings.TrimSpace(spec.Name)
	if !mountNamePattern.MatchString(spec.Name) || (requirePort && len(spec.Ports) == 0) {
		return apperr.ErrSandboxContainerSpecInvalid
	}
	if err := validateComponentMountDomains(*spec); err != nil {
		return err
	}
	if err := validateComponentEnv(*spec); err != nil {
		return err
	}
	if err := validateContainerIdentity(spec); err != nil {
		return err
	}
	if err := validateResourceSpec(spec.Resources); err != nil {
		return err
	}
	if len(spec.Ports) > 0 {
		if err := validatePortSpecs(spec.Ports); err != nil {
			return err
		}
	}
	normalizeProbe(&spec.ReadinessProbe, cfg)
	normalizeProbe(&spec.LivenessProbe, cfg)
	portNames := declaredPortNames(spec.Ports)
	for _, probe := range []*workload.ProbeSpec{&spec.ReadinessProbe, &spec.LivenessProbe} {
		if err := validateProbeSpec(probe, portNames); err != nil {
			return err
		}
	}
	if len(spec.Command) > 0 && !workload.ValidNonShellCommand(spec.Command) {
		return apperr.ErrSandboxContainerSpecInvalid
	}
	return nil
}

func validateRuntimePodInitContainers(spec *AdapterSpec, cfg config.SandboxConfig) error {
	for podIndex := range spec.Pods {
		pod := &spec.Pods[podIndex]
		containerNames := make(map[string]struct{}, len(pod.Containers)+len(pod.InitContainers))
		for _, container := range pod.Containers {
			containerNames[container.Name] = struct{}{}
		}
		for containerIndex := range pod.InitContainers {
			container := &pod.InitContainers[containerIndex]
			if err := validateRuntimeContainerSpec(container, cfg, false); err != nil {
				return err
			}
			if _, exists := containerNames[container.Name]; exists {
				return apperr.ErrSandboxPodTopologyInvalid
			}
			containerNames[container.Name] = struct{}{}
			if err := validateInfraSidecarImage(*container, cfg); err != nil {
				return err
			}
			if len(container.PrepullCommand) == 0 || validatePrepullCommand(container.PrepullCommand) != nil {
				return apperr.ErrSandboxAdapterSpecInvalid
			}
		}
	}
	return nil
}

// validateContainerIdentity 只允许平台规定的非 root 工作负载身份,并禁止容器自行取得更高权限。
func validateContainerIdentity(spec *workload.ComponentSpec) error {
	if spec.RunAsUser == nil && spec.RunAsGroup == nil {
		return nil
	}
	if spec.RunAsUser == nil || spec.RunAsGroup == nil || *spec.RunAsUser != 1000 || *spec.RunAsGroup != 1000 {
		return apperr.ErrSandboxContainerSpecInvalid
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
func validatePodTopology(spec *AdapterSpec) error {
	if len(spec.Pods) == 0 {
		return validatePodPortConflicts(podTopologyForAdapter(*spec))
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
		}
	}
	if !runtimeContainerSeen {
		return apperr.ErrSandboxPodTopologyInvalid
	}
	return validatePodPortConflicts(podTopologyForAdapter(*spec))
}

// 同 Pod 内端口号与协议共同标识监听地址。
func validatePodPortConflicts(pods []workload.PodSpec) error {
	for _, pod := range pods {
		ports := map[workload.NetworkPortRef]bool{}
		for _, container := range pod.Containers {
			for _, port := range container.Ports {
				identity := workload.NetworkPortRef{Port: port.ContainerPort, Protocol: valueOrDefault(port.Protocol, "TCP")}
				if ports[identity] {
					return apperr.ErrSandboxPodTopologyInvalid
				}
				ports[identity] = true
			}
		}
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
	return spec.Name != "" && apiequality.Semantic.DeepEqual(spec, workload.ComponentSpec{Name: spec.Name})
}

// validateNetworkRules 校验同沙箱 Pod 互通必须显式引用已声明的目标端口。
func validateNetworkRules(spec *AdapterSpec) error {
	return validateCompositionNetworkRules(map[string]AdapterSpec{"": *spec}, nil)
}

// validateCompositionNetworkRules 在本次完整拓扑上核对所有运行时和工具的网络规则。
func validateCompositionNetworkRules(runtimes map[string]AdapterSpec, tools map[string]Tool) error {
	owners := make([]string, 0, len(runtimes))
	for owner := range runtimes {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	portsByRole, err := compositionPodPorts(runtimes, tools)
	if err != nil {
		return err
	}
	seenRules := map[string]struct{}{}
	for _, owner := range owners {
		ownedPorts := podPortMap(podTopologyForAdapter(runtimes[owner]))
		rules := runtimes[owner].NetworkRules
		for i := range rules {
			rule := &rules[i]
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
			if _, ok := ownedPorts[rule.From]; !ok {
				return apperr.ErrSandboxNetworkPolicyInvalid
			}
			targetPorts, ok := portsByRole[rule.To]
			if !ok {
				return apperr.ErrSandboxNetworkPolicyInvalid
			}
			if err := resolveNetworkRulePorts(rule.Ports, targetPorts); err != nil {
				return err
			}
		}
	}
	for _, code := range slices.Sorted(maps.Keys(tools)) {
		if err := validateToolNetworkTargets(tools[code], portsByRole); err != nil {
			return apperr.ErrSandboxToolIncompatible.WithCause(fmt.Errorf("组件 %s 连接目标无效: %w", code, err))
		}
	}
	return nil
}

func compositionPodPorts(runtimes map[string]AdapterSpec, tools map[string]Tool) (map[string]map[string]workload.PortSpec, error) {
	portsByRole := make(map[string]map[string]workload.PortSpec)
	for _, runtime := range runtimes {
		for role, ports := range podPortMap(podTopologyForAdapter(runtime)) {
			if _, exists := portsByRole[role]; exists {
				return nil, apperr.ErrSandboxNetworkPolicyInvalid
			}
			portsByRole[role] = ports
		}
	}
	for _, tool := range tools {
		for name, ports := range componentPortMap(tool.ResourceSpec.Components) {
			role := toolComponentPodName(tool.Code, name)
			if _, exists := portsByRole[role]; exists {
				return nil, apperr.ErrSandboxNetworkPolicyInvalid
			}
			portsByRole[role] = ports
		}
	}
	return portsByRole, nil
}

func validateToolNetworkTargets(tool Tool, portsByRole map[string]map[string]workload.PortSpec) error {
	componentPorts := componentPortMap(tool.ResourceSpec.Components)
	for _, rule := range tool.ResourceSpec.NetworkRules {
		if _, ok := componentPorts[rule.From]; !ok {
			return fmt.Errorf("来源组件 %s 未声明", rule.From)
		}
		targetPorts, ok := componentPorts[rule.To]
		if !ok {
			targetPorts = portsByRole[rule.To]
		}
		if len(targetPorts) == 0 {
			return fmt.Errorf("目标 Pod 或组件 %s 未声明", rule.To)
		}
		if err := resolveNetworkRulePorts(rule.Ports, targetPorts); err != nil {
			return fmt.Errorf("规则 %s 的目标 %s 端口无效: %w", rule.Name, rule.To, err)
		}
	}
	return nil
}

// 冻结实际端口与协议；仅数字端口跨协议重名时拒绝猜测。
func resolveNetworkRulePorts(refs []workload.NetworkPortRef, targetPorts map[string]workload.PortSpec) error {
	if len(refs) == 0 {
		return apperr.ErrSandboxNetworkPolicyInvalid
	}
	seen := make(map[workload.NetworkPortRef]bool, len(refs))
	for i := range refs {
		ref := &refs[i]
		ref.Name = strings.TrimSpace(ref.Name)
		ref.Protocol = strings.ToUpper(strings.TrimSpace(ref.Protocol))
		var resolved workload.PortSpec
		found := false
		if ref.Name != "" {
			port, ok := targetPorts[ref.Name]
			if !ok || ref.Protocol != "" && ref.Protocol != valueOrDefault(port.Protocol, "TCP") {
				return apperr.ErrSandboxNetworkPolicyInvalid
			}
			resolved, found = port, true
		} else {
			for _, port := range targetPorts {
				port.Protocol = valueOrDefault(port.Protocol, "TCP")
				if port.ContainerPort != ref.Port || ref.Protocol != "" && ref.Protocol != port.Protocol {
					continue
				}
				if found && resolved.Protocol != port.Protocol {
					return apperr.ErrSandboxNetworkPolicyInvalid
				}
				resolved, found = port, true
			}
		}
		if !found || resolved.ContainerPort <= 0 || resolved.ContainerPort > 65535 {
			return apperr.ErrSandboxNetworkPolicyInvalid
		}
		ref.Port, ref.Protocol = resolved.ContainerPort, valueOrDefault(resolved.Protocol, "TCP")
		if ref.Protocol != "TCP" && ref.Protocol != "UDP" {
			return apperr.ErrSandboxNetworkPolicyInvalid
		}
		identity := workload.NetworkPortRef{Port: ref.Port, Protocol: ref.Protocol}
		if seen[identity] {
			return apperr.ErrSandboxNetworkPolicyInvalid
		}
		seen[identity] = true
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

func podPortMap(pods []workload.PodSpec) map[string]map[string]workload.PortSpec {
	podPorts := map[string]map[string]workload.PortSpec{}
	for _, pod := range pods {
		ports := map[string]workload.PortSpec{}
		for _, container := range pod.Containers {
			for _, port := range container.Ports {
				ports[port.Name] = port
			}
		}
		podPorts[pod.Name] = ports
	}
	return podPorts
}

// podTopologyForAdapter 返回 adapter 归一化后的运行时 Pod 组,供校验与服务解析复用。
func podTopologyForAdapter(spec AdapterSpec) []workload.PodSpec {
	if len(spec.Pods) > 0 {
		pods := slices.Clone(spec.Pods)
		declared := declaredContainerMap(spec)
		seen := map[string]struct{}{}
		for podIndex := range pods {
			pods[podIndex].Containers = slices.Clone(pods[podIndex].Containers)
			for containerIndex := range pods[podIndex].Containers {
				container := &pods[podIndex].Containers[containerIndex]
				if definition, ok := declared[container.Name]; ok && podContainerIsReferenceOnly(*container) {
					*container = definition
				}
				seen[container.Name] = struct{}{}
			}
		}
		for _, sidecar := range spec.InfraSidecars {
			if _, exists := seen[sidecar.Name]; exists {
				continue
			}
			for index := range pods {
				for _, container := range pods[index].Containers {
					if container.Name == spec.RuntimeContainer.Name {
						pods[index].Containers = append(pods[index].Containers, sidecar)
						seen[sidecar.Name] = struct{}{}
						break
					}
				}
				if _, added := seen[sidecar.Name]; added {
					break
				}
			}
		}
		return pods
	}
	containers := []workload.ComponentSpec{spec.RuntimeContainer}
	containers = append(containers, spec.InfraSidecars...)
	return []workload.PodSpec{{Name: "sandbox", Containers: containers}}
}

// validatePrivateArchiveOps 保证隐藏判题归档只会进入声明了私有卷且禁止学生进入的容器。
func validatePrivateArchiveOps(ops PrivateArchiveOps) error {
	if !explicitExecTarget(ops.ExecTarget) || !workload.ValidNonShellCommand(ops.UnpackTar) {
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
			{Name: VolumeDomainWorkspace, MountPath: spec.WorkspaceDir, StudentAccess: VolumeAccessReadWrite, Persistence: VolumePersistenceObjectStorageCode, SnapshotScope: VolumeSnapshotAlways},
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
		if (domain.Persistence == VolumePersistenceSnapshot && domain.SnapshotScope == VolumeSnapshotNever) ||
			(domain.Persistence == VolumePersistenceObjectStorageCode && domain.Name != VolumeDomainWorkspace) {
			return apperr.ErrSandboxVolumeDomainInvalid
		}
		if domain.Name == VolumeDomainWorkspace {
			workspaceSeen = true
			if domain.MountPath != path.Clean(spec.WorkspaceDir) || domain.StudentAccess != VolumeAccessReadWrite || domain.Persistence != VolumePersistenceObjectStorageCode {
				return apperr.ErrSandboxVolumeDomainInvalid
			}
		}
		if domain.Name == VolumeDomainRuntimeState && domain.StudentAccess != VolumeAccessNone {
			return apperr.ErrSandboxVolumeDomainInvalid
		}
		if domain.Name == VolumeDomainPublicAssets && domain.StudentAccess != VolumeAccessReadOnly {
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

// validateAdapterComponentMountDomains 校验运行时所有实际组件只使用已声明安全域,并显式声明写权限。
func validateAdapterComponentMountDomains(spec AdapterSpec) error {
	declared := make(map[string]VolumeDomainSpec, len(spec.VolumeDomains))
	for _, domain := range spec.VolumeDomains {
		declared[strings.TrimSpace(domain.Name)] = domain
	}
	components := []workload.ComponentSpec{spec.RuntimeContainer}
	components = append(components, spec.InfraSidecars...)
	for _, pod := range spec.Pods {
		components = append(components, pod.InitContainers...)
		for _, component := range pod.Containers {
			if !podContainerIsReferenceOnly(component) {
				components = append(components, component)
			}
		}
	}
	for _, component := range components {
		mounted := map[string]struct{}{}
		for _, raw := range component.MountDomains {
			name := strings.TrimSpace(raw)
			domain, ok := declared[name]
			if !ok || domain.Name == VolumeDomainWorkspace || (domain.Name == VolumeDomainJudgePrivate && studentAccessibleContainer(component)) {
				return apperr.ErrSandboxVolumeDomainInvalid
			}
			mounted[name] = struct{}{}
		}
		for _, raw := range component.WritableMountDomains {
			name := strings.TrimSpace(raw)
			domain, ok := declared[name]
			if !ok || domain.Name == VolumeDomainWorkspace || (domain.Name == VolumeDomainJudgePrivate && studentAccessibleContainer(component)) || domain.StudentAccess == VolumeAccessReadWrite {
				return apperr.ErrSandboxPrivateDomainInvalid
			}
			if _, ok := mounted[name]; !ok {
				return apperr.ErrSandboxVolumeDomainInvalid
			}
		}
	}
	return nil
}

// validVolumeAccess 校验卷域学生访问级别枚举。
func validVolumeAccess(access string) bool {
	return access == VolumeAccessNone || access == VolumeAccessReadOnly || access == VolumeAccessReadWrite
}

// validVolumePersistence 校验卷域持久化策略枚举。
func validVolumePersistence(persistence string) bool {
	return persistence == VolumePersistenceObjectStorageCode || persistence == VolumePersistenceEphemeral || persistence == VolumePersistenceSnapshot
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
	if !imageRefAttested(cfg, spec.ImageURL) {
		return apperr.ErrSandboxSidecarImageInvalid
	}
	return nil
}

// validatePrivateSidecars 校验内部执行 sidecar,避免把判题器等私有容器暴露为学生终端或绕过镜像证明。
func validatePrivateSidecars(items []workload.ComponentSpec, cfg config.SandboxConfig, adapter AdapterSpec) error {
	seen := declaredContainerMap(adapter)
	for i := range items {
		item := &items[i]
		if err := validateRuntimeContainerSpec(item, cfg, false); err != nil {
			return err
		}
		if studentAccessibleContainer(*item) {
			return apperr.ErrSandboxPrivateDomainInvalid
		}
		for _, raw := range item.MountDomains {
			if strings.TrimSpace(raw) != VolumeDomainJudgePrivate {
				return apperr.ErrSandboxPrivateDomainInvalid
			}
		}
		if len(item.WritableMountDomains) > 0 {
			return apperr.ErrSandboxPrivateDomainInvalid
		}
		if _, exists := seen[item.Name]; exists {
			return apperr.ErrSandboxContainerSpecInvalid
		}
		seen[item.Name] = *item
		if err := validateInfraSidecarImage(*item, cfg); err != nil {
			return err
		}
	}
	return nil
}

// validatePlanImagesCurrentlyAdmitted 在启动或恢复前用当前证明校验已入库镜像引用。
func validatePlanImagesCurrentlyAdmitted(cfg config.SandboxConfig, runtime Runtime, image RuntimeImage, tools []Tool, privateSidecars []workload.ComponentSpec) error {
	if !imageRefAttested(cfg, image.ImageURL) {
		return apperr.ErrSandboxImageAttestationInvalid
	}
	for _, component := range runtime.AdapterSpec.InfraSidecars {
		if err := validateInfraSidecarImage(component, cfg); err != nil {
			return err
		}
	}
	for _, pod := range runtime.AdapterSpec.Pods {
		for _, component := range pod.InitContainers {
			if err := validateInfraSidecarImage(component, cfg); err != nil {
				return err
			}
		}
		for _, component := range pod.Containers {
			if component.ImageURL == "" || component.Name == runtime.AdapterSpec.RuntimeContainer.Name {
				continue
			}
			if err := validateInfraSidecarImage(component, cfg); err != nil {
				return err
			}
		}
	}
	for _, tool := range tools {
		for _, component := range tool.ResourceSpec.Components {
			if !imageRefAttested(cfg, component.ImageURL) {
				return apperr.ErrSandboxImageAttestationInvalid
			}
		}
	}
	for _, component := range privateSidecars {
		if !imageRefAttested(cfg, component.ImageURL) {
			return apperr.ErrSandboxImageAttestationInvalid
		}
	}
	return nil
}

// imageRefAttested 判断镜像引用是否是当前配置允许的 Harbor digest 证明。
func imageRefAttested(cfg config.SandboxConfig, imageURL string) bool {
	imageURL = strings.TrimSpace(imageURL)
	digest := digestFromImageURL(imageURL)
	return imageURL != "" && digest != "" && imageAttested(cfg, imageURL, digest)
}

// digestFromImageURL 从 image@sha256:... 提取不可变 digest。
func digestFromImageURL(imageURL string) string {
	parts := strings.Split(strings.TrimSpace(imageURL), "@")
	if len(parts) != 2 || !strings.HasPrefix(parts[1], "sha256:") {
		return ""
	}
	return parts[1]
}

// validateWorkspaceOps 校验文件、归档、私有脚本、开放前清理、自检和终端的受控命令。
func validateWorkspaceOps(ops WorkspaceOps) error {
	if !explicitExecTarget(ops.ExecTarget) || !explicitExecTarget(ops.ScriptExecTarget) {
		return apperr.ErrSandboxWorkspaceOpsInvalid
	}
	helperCommands := [][]string{
		ops.CleanWorkspace,
		ops.ReadFile,
		ops.WriteFile,
		ops.ListFiles,
		ops.PackTar,
		ops.UnpackTar,
		ops.RunScript,
		ops.Selftest,
	}
	for _, command := range helperCommands {
		if !workload.ValidNonShellCommand(command) {
			return apperr.ErrSandboxWorkspaceOpsInvalid
		}
	}
	if !workload.ValidCommand(ops.Terminal) {
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
	}
	for _, command := range commands {
		if !explicitExecTarget(command.ExecTarget) {
			return apperr.ErrSandboxCapabilityCommandInvalid
		}
		if !workload.ValidNonShellCommand(command.Command) {
			return apperr.ErrSandboxCapabilityCommandInvalid
		}
		if command.TimeoutSeconds < 0 {
			return apperr.ErrSandboxCapabilityCommandInvalid
		}
		if command.TimeoutSeconds == 0 {
			defaultTimeout, ok := intx.Int32(cfg.ChainRPCTimeoutSeconds)
			if !ok || defaultTimeout <= 0 {
				return apperr.ErrSandboxCapabilityCommandInvalid
			}
			command.TimeoutSeconds = defaultTimeout
		}
	}
	strategy := strings.TrimSpace(spec.CapabilityCommands.ResetStrategy)
	if strategy == "recreate_runtime" {
		if len(spec.CapabilityCommands.Reset.Command) > 0 {
			return apperr.ErrSandboxCapabilityCommandInvalid
		}
		return nil
	}
	if strategy != "" {
		return apperr.ErrSandboxCapabilityCommandInvalid
	}
	reset := &spec.CapabilityCommands.Reset
	if !explicitExecTarget(reset.ExecTarget) || !workload.ValidNonShellCommand(reset.Command) {
		return apperr.ErrSandboxCapabilityCommandInvalid
	}
	if reset.TimeoutSeconds < 0 {
		return apperr.ErrSandboxCapabilityCommandInvalid
	}
	if reset.TimeoutSeconds == 0 {
		defaultTimeout, ok := intx.Int32(cfg.ChainRPCTimeoutSeconds)
		if !ok || defaultTimeout <= 0 {
			return apperr.ErrSandboxCapabilityCommandInvalid
		}
		reset.TimeoutSeconds = defaultTimeout
	}
	return nil
}

// validateAdapterExecTargets 校验受控命令只进入已声明且权限匹配的容器。
func validateAdapterExecTargets(spec *AdapterSpec) error {
	containers := map[string]workload.ComponentSpec{}
	for _, pod := range podTopologyForAdapter(*spec) {
		for _, container := range pod.Containers {
			containers[pod.Name+"/"+container.Name] = container
		}
	}
	workspaceContainer, ok := containers[strings.TrimSpace(spec.WorkspaceOps.ExecTarget)]
	if !ok || !studentAccessibleContainer(workspaceContainer) || !shouldMountWorkspace(workspaceContainer) {
		return apperr.ErrSandboxWorkspaceOpsInvalid
	}
	if _, err := initScriptExecTarget(*spec); err != nil {
		return err
	}
	commands := []CapabilityCommandSpec{
		spec.CapabilityCommands.Deploy,
		spec.CapabilityCommands.Tx,
		spec.CapabilityCommands.Query,
		spec.CapabilityCommands.Reset,
	}
	for _, command := range commands {
		if len(command.Command) == 0 {
			continue
		}
		container, exists := containers[strings.TrimSpace(command.ExecTarget)]
		if !exists || studentAccessibleContainer(container) {
			return apperr.ErrSandboxCapabilityCommandInvalid
		}
	}
	privateContainer, exists := containers[strings.TrimSpace(spec.PrivateArchiveOps.ExecTarget)]
	if !exists || studentAccessibleContainer(privateContainer) {
		return apperr.ErrSandboxPrivateDomainInvalid
	}
	if len(spec.InitAssets) > 0 {
		initContainer, initExists := containers[strings.TrimSpace(spec.InitAssetOps.ExecTarget)]
		if !initExists || studentAccessibleContainer(initContainer) {
			return apperr.ErrSandboxInitAssetConfigInvalid
		}
	}
	return nil
}

// explicitExecTarget 要求内部受控命令明确使用 pod/container,禁止依赖隐式单 Pod 回退。
func explicitExecTarget(target string) bool {
	target = strings.TrimSpace(target)
	return strings.Count(target, "/") == 1 && safeExecTarget(target)
}

// hasCapabilityCommands 判断运行时是否声明完整 L2 能力命令集合。
func hasCapabilityCommands(commands CapabilityCommandSet) bool {
	return len(commands.Deploy.Command) > 0 &&
		len(commands.Tx.Command) > 0 &&
		len(commands.Query.Command) > 0 &&
		(strings.TrimSpace(commands.ResetStrategy) == "recreate_runtime" || len(commands.Reset.Command) > 0)
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
		if !workload.ValidNonShellCommand(probe.Command) {
			return apperr.ErrSandboxProbeSpecInvalid
		}
	default:
		return apperr.ErrSandboxProbeSpecInvalid
	}
	return nil
}

// validateResourceSpec 校验显式 CPU/内存非负，且 requests 不超过 limits。
func validateResourceSpec(spec workload.ResourceSpec) error {
	if len(spec.Requests) == 0 && len(spec.Limits) == 0 {
		return nil
	}
	for _, name := range []string{"cpu", "memory"} {
		request, err := resource.ParseQuantity(spec.Requests[name])
		if err != nil {
			return apperr.ErrSandboxContainerSpecInvalid.WithCause(err)
		}
		limit, err := resource.ParseQuantity(spec.Limits[name])
		if err != nil {
			return apperr.ErrSandboxContainerSpecInvalid.WithCause(err)
		}
		if request.Sign() < 0 || limit.Sign() < 0 || request.Cmp(limit) > 0 {
			return apperr.ErrSandboxContainerSpecInvalid
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

// validateComponentEnv 统一校验两种来源的名称、唯一性及 Secret 引用。
func validateComponentEnv(spec workload.ComponentSpec) error {
	seen := make(map[string]struct{}, len(spec.Env)+len(spec.SecretEnv))
	checkName := func(name string) error {
		if !envNamePattern.MatchString(name) {
			return apperr.ErrSandboxRuntimeEnvInvalid
		}
		if _, exists := seen[name]; exists {
			return apperr.ErrSandboxRuntimeEnvInvalid
		}
		seen[name] = struct{}{}
		return nil
	}
	for _, item := range spec.Env {
		name := item.Name
		if err := checkName(name); err != nil {
			return err
		}
		upper := strings.ToUpper(name)
		if strings.Contains(upper, "SECRET") || strings.Contains(upper, "TOKEN") || strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "PRIVATE_KEY") {
			return apperr.ErrSandboxRuntimeSecretEnvInvalid
		}
	}
	for _, item := range spec.SecretEnv {
		if err := checkName(item.Name); err != nil {
			return err
		}
		if item.SecretName != "chaimir-secret" || len(validation.IsConfigMapKey(item.SecretKey)) != 0 {
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

// validateComponentMountDomains 校验组件只能显式引用安全域,且不允许工具绕过学生与判题隔离。
func validateComponentMountDomains(spec workload.ComponentSpec) error {
	seen := map[string]struct{}{}
	for _, raw := range spec.MountDomains {
		domain := strings.TrimSpace(raw)
		if !mountNamePattern.MatchString(domain) || domain == VolumeDomainWorkspace || (domain == VolumeDomainJudgePrivate && studentAccessibleContainer(spec)) {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if _, exists := seen[domain]; exists {
			return apperr.ErrSandboxToolCreateInvalid
		}
		seen[domain] = struct{}{}
	}
	for _, raw := range spec.WritableMountDomains {
		domain := strings.TrimSpace(raw)
		if !mountNamePattern.MatchString(domain) || (domain == VolumeDomainJudgePrivate && studentAccessibleContainer(spec)) {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if _, exists := seen[domain]; !exists {
			return apperr.ErrSandboxToolCreateInvalid
		}
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
	for _, item := range cfg.PlatformImageAttestations {
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

// validateSandboxControlRequest 校验内部生命周期控制请求必须绑定租户、沙箱和来源。
func validateSandboxControlRequest(req contracts.SandboxControlRequest) error {
	if req.TenantID <= 0 || req.SandboxID <= 0 || !validSourceRef(req.SourceRef) {
		return apperr.ErrSandboxContractRequestInvalid
	}
	return nil
}
