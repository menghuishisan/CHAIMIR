// sandbox rules 文件定义输入校验、状态机校验和安全规则,不访问 repo/db/contracts。
package sandbox

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"

	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	codePattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)
	envNamePattern   = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,63}$`)
	mountNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
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

// validateToolRequest 校验工具定义,确保 web-embed 镜像、探针默认值和工作区挂载语义明确。
func validateToolRequest(req ToolRequest, cfg config.SandboxConfig) (ToolResourceSpec, error) {
	if !codePattern.MatchString(req.Code) || strings.TrimSpace(req.Name) == "" {
		return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
	}
	if req.Kind < SandboxToolKindBuiltin || req.Kind > SandboxToolKindWebEmbed {
		return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
	}
	var spec ToolResourceSpec
	if len(req.ResourceSpec) > 0 {
		if err := jsonx.DecodeStrict(req.ResourceSpec, &spec); err != nil {
			return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid.WithCause(err)
		}
	}
	if req.Kind == SandboxToolKindWebEmbed {
		if strings.TrimSpace(req.ImageURL) == "" || req.Port <= 0 || strings.TrimSpace(spec.ReadinessProbe.Type) == "" {
			return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
		}
		normalizeProbe(&spec.ReadinessProbe, cfg)
	}
	if req.Kind == SandboxToolKindBuiltin {
		if !validBuiltinEndpointTemplate(spec.BuiltinEndpoint) {
			return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
		}
	}
	if err := validateLiteralEnv(spec.Env); err != nil {
		return ToolResourceSpec{}, err
	}
	if err := validateToolEphemeralMounts(spec.EphemeralMounts); err != nil {
		return ToolResourceSpec{}, err
	}
	if req.Kind != SandboxToolKindWebEmbed && len(spec.NetworkRules) > 0 {
		return ToolResourceSpec{}, apperr.ErrSandboxToolCreateInvalid
	}
	if err := validateToolNetworkRules(&spec); err != nil {
		return ToolResourceSpec{}, err
	}
	return spec, nil
}

// validateQuota 校验租户沙箱配额均为显式正数,快照和保活上限允许为零表示禁用。
func validateQuota(q TenantQuota) error {
	if q.TenantID <= 0 || q.MaxConcurrentSandbox <= 0 || q.MaxCPU <= 0 || q.MaxMemoryMB <= 0 ||
		q.IdleTimeoutMin <= 0 || q.MaxLifetimeMin <= 0 || q.MaxKeepaliveMin < 0 || q.MaxSnapshotRetentionMin < 0 {
		return apperr.ErrSandboxQuotaInvalid
	}
	return nil
}

// validateQuotaForCreate 校验本次创建是否超过租户并发、保活和快照上限。
func validateQuotaForCreate(req CreateSandboxInputModel, quota TenantQuota, active int64, cfg config.SandboxConfig) error {
	if active >= int64(quota.MaxConcurrentSandbox) {
		return apperr.ErrSandboxQuotaExceeded
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
			SandboxStatusRunning:   {},
			SandboxStatusRecycling: {},
			SandboxStatusFailed:    {},
		},
		SandboxStatusRunning: {
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
	if err := validatePodTopology(spec, cfg); err != nil {
		return err
	}
	if err := validateNetworkRules(spec); err != nil {
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
	return nil
}

// validatePodTopology 校验显式 Pod 组拓扑,缺省时使用单 Pod 多容器口径。
func validatePodTopology(spec *AdapterSpec, cfg config.SandboxConfig) error {
	if len(spec.Pods) == 0 {
		return nil
	}
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
			if err := validateContainerSpec(container, cfg); err != nil {
				return err
			}
			if container.Name == spec.RuntimeContainer.Name {
				runtimeContainerSeen = true
				if strings.TrimSpace(container.Image) != "" {
					return apperr.ErrSandboxPodTopologyInvalid
				}
				if hasVolumeDomain(*spec, VolumeDomainJudgePrivate) && studentAccessibleContainer(*container) {
					return apperr.ErrSandboxPrivateDomainInvalid
				}
			}
			if container.Name != spec.RuntimeContainer.Name && !imageAttested(cfg, container.Image, digestFromImageURL(container.Image)) {
				return apperr.ErrSandboxSidecarImageInvalid
			}
			if _, exists := containerNames[container.Name]; exists {
				return apperr.ErrSandboxPodTopologyInvalid
			}
			containerNames[container.Name] = struct{}{}
			for _, port := range container.Ports {
				if _, exists := ports[port.ContainerPort]; exists {
					return apperr.ErrSandboxPodTopologyInvalid
				}
				ports[port.ContainerPort] = struct{}{}
			}
		}
	}
	if !runtimeContainerSeen {
		return apperr.ErrSandboxPodTopologyInvalid
	}
	return nil
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
		rule.FromPod = strings.TrimSpace(rule.FromPod)
		rule.ToPod = strings.TrimSpace(rule.ToPod)
		if !mountNamePattern.MatchString(rule.Name) || rule.FromPod == "" || rule.ToPod == "" || len(rule.Ports) == 0 {
			return apperr.ErrSandboxNetworkPolicyInvalid
		}
		if _, exists := seenRules[rule.Name]; exists {
			return apperr.ErrSandboxNetworkPolicyInvalid
		}
		seenRules[rule.Name] = struct{}{}
		if _, ok := podPorts[rule.FromPod]; !ok {
			return apperr.ErrSandboxNetworkPolicyInvalid
		}
		targetPorts, ok := podPorts[rule.ToPod]
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
	for i := range spec.NetworkRules {
		rule := &spec.NetworkRules[i]
		rule.Name = strings.TrimSpace(rule.Name)
		rule.ToPod = strings.TrimSpace(rule.ToPod)
		if !mountNamePattern.MatchString(rule.Name) || rule.ToPod == "" || len(rule.Ports) == 0 {
			return apperr.ErrSandboxToolCreateInvalid
		}
		if _, exists := seenRules[rule.Name]; exists {
			return apperr.ErrSandboxToolCreateInvalid
		}
		seenRules[rule.Name] = struct{}{}
		seenPorts := map[string]struct{}{}
		for j := range rule.Ports {
			ref := &rule.Ports[j]
			ref.Name = strings.TrimSpace(ref.Name)
			if ref.Name == "" && ref.Port <= 0 {
				return apperr.ErrSandboxToolCreateInvalid
			}
			key := ref.Name
			if key == "" {
				key = fmt.Sprintf("%d", ref.Port)
			}
			if _, ok := seenPorts[key]; ok {
				return apperr.ErrSandboxToolCreateInvalid
			}
			seenPorts[key] = struct{}{}
		}
	}
	return nil
}

// validateToolNetworkRulesForRuntime 校验工具网络规则只能访问运行时拓扑中已声明的目标端口。
func validateToolNetworkRulesForRuntime(tool Tool, adapter AdapterSpec) error {
	podPorts := podPortMap(podTopologyForAdapter(adapter))
	for i := range tool.ResourceSpec.NetworkRules {
		rule := &tool.ResourceSpec.NetworkRules[i]
		targetPorts, ok := podPorts[rule.ToPod]
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
func podPortMap(pods []PodSpec) map[string]map[string]int32 {
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
func podTopologyForAdapter(spec AdapterSpec) []PodSpec {
	if len(spec.Pods) > 0 {
		return spec.Pods
	}
	containers := []ContainerSpec{spec.RuntimeContainer}
	containers = append(containers, spec.InfraSidecars...)
	return []PodSpec{{Name: "sandbox", Containers: containers}}
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
func validateInfraSidecarImage(spec ContainerSpec, cfg config.SandboxConfig) error {
	imageURL := strings.TrimSpace(spec.Image)
	if imageURL == "" && spec.Labels != nil {
		imageURL = strings.TrimSpace(spec.Labels["image"])
	}
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
	required := [][]string{
		ops.ReadFile,
		ops.WriteFile,
		ops.ListFiles,
		ops.PackTar,
		ops.UnpackTar,
		ops.RunScript,
		ops.Terminal,
		ops.Selftest,
	}
	for _, command := range required {
		if !safeCommand(command) {
			return apperr.ErrSandboxWorkspaceOpsInvalid
		}
	}
	return nil
}

// validateCapabilityCommands 校验 L2/L3 标准链能力有真实执行入口,避免只登记字段却无法 deploy/tx/query/reset。
func validateCapabilityCommands(spec *AdapterSpec, cfg config.SandboxConfig) error {
	if !hasCapabilityCommands(spec.CapabilityCommands) {
		return nil
	}
	commands := []CapabilityCommandSpec{
		spec.CapabilityCommands.Deploy,
		spec.CapabilityCommands.Tx,
		spec.CapabilityCommands.Query,
		spec.CapabilityCommands.Reset,
	}
	for _, command := range commands {
		if !safeCommand(command.Command) {
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

// safeCommand 校验声明式命令不为空、不经 shell 字符串拼接,避免注册期放入隐式脚本。
func safeCommand(command []string) bool {
	if len(command) == 0 {
		return false
	}
	for _, part := range command {
		if strings.TrimSpace(part) == "" {
			return false
		}
	}
	return true
}

// validateContainerSpec 校验单个容器声明不会绕过安全上下文或硬编码无效探针。
func validateContainerSpec(spec *ContainerSpec, cfg config.SandboxConfig) error {
	if strings.TrimSpace(spec.Name) == "" || len(spec.Ports) == 0 {
		return apperr.ErrSandboxContainerSpecInvalid
	}
	if err := validateLiteralEnv(spec.Env); err != nil {
		return err
	}
	normalizeProbe(&spec.ReadinessProbe, cfg)
	normalizeProbe(&spec.LivenessProbe, cfg)
	for _, probe := range []ProbeSpec{spec.ReadinessProbe, spec.LivenessProbe} {
		if probe.Type != "" && probe.Type != "tcp" && probe.Type != "http" && probe.Type != "exec" {
			return apperr.ErrSandboxProbeSpecInvalid
		}
	}
	return nil
}

// normalizeProbe 从配置补齐探针默认值,避免编排代码硬编码周期和失败阈值。
func normalizeProbe(probe *ProbeSpec, cfg config.SandboxConfig) {
	if probe.PeriodSeconds <= 0 {
		probe.PeriodSeconds = cfg.ProbeDefaultPeriodSeconds
	}
	if probe.FailureThreshold <= 0 {
		probe.FailureThreshold = cfg.ProbeDefaultFailureThreshold
	}
}

// validateLiteralEnv 拒绝密钥式字段进入声明式配置,敏感值必须走平台 Secret。
func validateLiteralEnv(env []EnvVarSpec) error {
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
func validateToolEphemeralMounts(mounts []ToolEphemeralMountSpec) error {
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

// shouldMountWorkspace 判定工具容器是否挂载工作区:运行时默认挂载,工具必须显式声明。
func shouldMountWorkspace(spec ToolResourceSpec) bool {
	return spec.MountWorkspace != nil && *spec.MountWorkspace
}
