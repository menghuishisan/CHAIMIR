// composition 文件实现 M2 唯一的组合编译、依赖展开和摘要生成逻辑。
package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
)

// CompileSandboxComposition 校验声明并锁定全部 runtime 实例、工具和基础设施的镜像闭包。
func (s *Service) CompileSandboxComposition(ctx context.Context, tenantID int64, spec contracts.SandboxCompositionSpec) (contracts.SandboxCompositionSnapshot, error) {
	if tenantID <= 0 {
		return contracts.SandboxCompositionSnapshot{}, apperr.ErrSandboxCreateRequestInvalid
	}
	return CompilePlatformSandboxComposition(ctx, s.store, spec)
}

// CompilePlatformSandboxComposition 供平台治理面冻结判题器等全局组合，不借用任意租户身份。
func (s *Service) CompilePlatformSandboxComposition(ctx context.Context, spec contracts.SandboxCompositionSpec) (contracts.SandboxCompositionSnapshot, error) {
	return CompilePlatformSandboxComposition(ctx, s.store, spec)
}

// CompilePlatformSandboxComposition 让部署装配层复用 M2 唯一编译器冻结平台目录快照。
// 它只读取平台运行时、镜像和工具目录，不创建沙箱或绕开自检门禁。
func CompilePlatformSandboxComposition(ctx context.Context, store Store, spec contracts.SandboxCompositionSpec) (contracts.SandboxCompositionSnapshot, error) {
	if store == nil {
		return contracts.SandboxCompositionSnapshot{}, apperr.ErrSandboxCreateRequestInvalid
	}
	if strings.TrimSpace(spec.ID) == "" || !spec.AccessProfile.Valid() {
		return contracts.SandboxCompositionSnapshot{}, apperr.ErrSandboxCreateRequestInvalid
	}
	if _, err := contracts.CanonicalDigest(spec); err != nil {
		return contracts.SandboxCompositionSnapshot{}, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	var snapshot contracts.SandboxCompositionSnapshot
	err := store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		runtimePlans := make([]RuntimePlan, 0, len(spec.Runtimes))
		runtimeAdapters := make(map[string]AdapterSpec, len(spec.Runtimes))
		runtimeCapabilities := make([]CapabilityConstraints, 0, len(spec.Runtimes))
		for _, ref := range spec.Runtimes {
			instance := strings.TrimSpace(ref.InstanceCode)
			runtime, err := tx.GetRuntimeByCode(ctx, strings.TrimSpace(ref.Code))
			if err != nil {
				return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
			}
			if runtime.Status != RuntimeStatusAvailable || runtime.SelftestStatus != RuntimeSelftestPassed {
				return fmt.Errorf("运行时 %s 尚未通过平台自检", runtime.Code)
			}
			if err := validateComponentParams(ref.Params, runtime.AdapterSpec.Capabilities.ConfigSchema); err != nil {
				return fmt.Errorf("运行时 %s 参数无效: %w", runtime.Code, err)
			}
			image, err := tx.GetRuntimeImageByVersionForShare(ctx, runtime.ID, strings.TrimSpace(ref.ImageVersion))
			if err != nil {
				return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
			}
			runtimePlans = append(runtimePlans, RuntimePlan{InstanceCode: instance, Runtime: runtime, Image: image})
			runtimeAdapters[instance] = runtime.AdapterSpec
			runtimeCapabilities = append(runtimeCapabilities, runtime.AdapterSpec.Capabilities)
		}
		if len(runtimePlans) == 0 {
			return fmt.Errorf("组合至少声明一个 runtime 实例")
		}
		if err := validateRuntimeVolumeDomains(runtimePlans); err != nil {
			return err
		}
		aggregatedCapabilities := aggregateRuntimeCapabilities(runtimeCapabilities)
		tools, err := tx.ListTools(ctx)
		if err != nil {
			return err
		}
		toolByCode := make(map[string]Tool, len(tools))
		for _, item := range tools {
			toolByCode[item.Code] = item
		}
		var selected []contracts.CompositionComponentRef
		selected = append(selected, spec.Infra...)
		selected = append(selected, spec.Tools...)
		explicitInfraCount := len(spec.Infra)
		selectedCodes := map[string]struct{}{}
		for _, ref := range selected {
			code := strings.TrimSpace(ref.Code)
			if code == "" {
				return fmt.Errorf("组合包含空组件编码")
			}
			if _, exists := selectedCodes[code]; exists {
				return fmt.Errorf("组件 %s 重复选择", code)
			}
			selectedCodes[code] = struct{}{}
		}
		for idx := 0; idx < len(selected); idx++ {
			ref := selected[idx]
			code := strings.TrimSpace(ref.Code)
			selection := strings.TrimSpace(ref.Selection)
			if selection != "explicit" && selection != "auto" {
				return fmt.Errorf("组件 %s 的选择来源无效", code)
			}
			if idx < explicitInfraCount+len(spec.Tools) && selection != "explicit" {
				return fmt.Errorf("教师声明的组件 %s 必须标记为 explicit", code)
			}
			tool, ok := toolByCode[code]
			if !ok || tool.Status != ToolStatusAvailable {
				return fmt.Errorf("组件 %s 不可用", code)
			}
			expectedCategory := "infra"
			if idx >= explicitInfraCount && idx < explicitInfraCount+len(spec.Tools) {
				expectedCategory = "tool"
			}
			if tool.Category != expectedCategory {
				return fmt.Errorf("组件 %s 的目录类别与声明位置不一致", code)
			}
			if strings.TrimSpace(tool.ResourceSpec.Capabilities.Placement) != "sandbox" {
				return fmt.Errorf("组件 %s 不能部署到沙箱", code)
			}
			if err := validateToolForRuntime(tool, runtimePlans); err != nil {
				return fmt.Errorf("组件 %s 与运行时不兼容: %w", code, err)
			}
			if err := validateCompositionAccess(spec.AccessProfile, tool.Category, tool.ResourceSpec.Capabilities.StudentAccess); err != nil {
				return fmt.Errorf("组件 %s: %w", code, err)
			}
			if err := validateComponentParams(ref.Params, tool.ResourceSpec.Capabilities.ConfigSchema); err != nil {
				return fmt.Errorf("组件 %s 参数无效: %w", code, err)
			}
			for _, required := range tool.ResourceSpec.Capabilities.Requires {
				if capabilityProvided(aggregatedCapabilities, selected, toolByCode, required) {
					continue
				}
				provider, providerErr := findCapabilityProvider(toolByCode, required, "infra")
				if providerErr != nil {
					return providerErr
				}
				if provider == "" {
					return fmt.Errorf("组件 %s 缺少必需能力 %s", code, required)
				}
				if _, exists := selectedCodes[provider]; exists {
					continue
				}
				selectedCodes[provider] = struct{}{}
				selected = append(selected, contracts.CompositionComponentRef{Code: provider, Selection: "auto"})
			}
		}
		autoInfra := append([]contracts.CompositionComponentRef(nil), selected[explicitInfraCount+len(spec.Tools):]...)
		spec.Infra = append(append([]contracts.CompositionComponentRef(nil), spec.Infra...), autoInfra...)
		spec.Tools = append([]contracts.CompositionComponentRef(nil), spec.Tools...)
		if err := validateCompositionLinks(spec, selected, toolByCode); err != nil {
			return err
		}
		if err := validateCompositionBindings(spec, runtimeCapabilitiesByInstance(runtimePlans), selected, toolByCode); err != nil {
			return err
		}
		if err := validateCompositionConflicts(aggregatedCapabilities, selected, toolByCode); err != nil {
			return err
		}
		if err := validateCapabilityCardinality(aggregatedCapabilities, selected, toolByCode); err != nil {
			return err
		}
		compiledRuntimes, compiledTools, err := compileCompositionLinks(spec, runtimeAdapters, runtimePlans, selected, toolByCode)
		if err != nil {
			return err
		}
		for instance, compiledRuntime := range compiledRuntimes {
			if err := validateNetworkRules(&compiledRuntime); err != nil {
				return fmt.Errorf("运行时 %s 连接拓扑无效: %w", instance, err)
			}
			compiledRuntimes[instance] = compiledRuntime
		}
		for _, ref := range selected {
			tool := compiledTools[ref.Code]
			if err := validateToolMountDomainsForComposition(tool, aggregateRuntimeAdapterSpecsByMap(compiledRuntimes)); err != nil {
				return fmt.Errorf("组件 %s 安全域挂载无效: %w", ref.Code, err)
			}
			if err := validateToolNetworkRules(&tool.ResourceSpec); err != nil {
				return fmt.Errorf("组件 %s 连接拓扑无效: %w", ref.Code, err)
			}
			if err := validateToolNetworkRulesForComposition(tool, compiledRuntimes, compiledTools); err != nil {
				return fmt.Errorf("组件 %s 连接目标无效: %w", ref.Code, err)
			}
		}
		var closure []contracts.ImageClosureItem
		closureSeen := map[string]struct{}{}
		appendClosure := func(category, code string, component workload.ComponentSpec, imageURL, version string, fallbackCommand []string) {
			url := strings.TrimSpace(imageURL)
			if url == "" {
				url = strings.TrimSpace(component.ImageURL)
			}
			if url == "" {
				return
			}
			command := append([]string(nil), component.PrepullCommand...)
			if len(command) == 0 && len(fallbackCommand) > 0 {
				command = append([]string(nil), fallbackCommand...)
			}
			item := contracts.ImageClosureItem{Category: category, Code: code, ImageURL: url, Version: strings.TrimSpace(version), PrepullCommand: command, PrepullHold: component.PrepullHold}
			for _, mount := range component.EphemeralMounts {
				item.EphemeralMounts = append(item.EphemeralMounts, contracts.ImageClosureMount{Name: mount.Name, MountPath: mount.MountPath})
			}
			keyBytes, _ := json.Marshal(struct {
				URL             string
				Command         []string
				Hold            bool
				EphemeralMounts []contracts.ImageClosureMount
			}{URL: item.ImageURL, Command: item.PrepullCommand, Hold: item.PrepullHold, EphemeralMounts: item.EphemeralMounts})
			if _, exists := closureSeen[string(keyBytes)]; exists {
				return
			}
			closureSeen[string(keyBytes)] = struct{}{}
			if item.Version == "" {
				item.Version = immutableImageVersion(url)
			}
			closure = append(closure, item)
		}
		for _, runtimePlan := range runtimePlans {
			compiledRuntime := compiledRuntimes[runtimePlan.InstanceCode]
			appendClosure("runtime", runtimePlan.InstanceCode+"/"+runtimePlan.Runtime.Code, compiledRuntime.RuntimeContainer, runtimePlan.Image.ImageURL, runtimePlan.Image.Version, nil)
			for _, component := range compiledRuntime.InfraSidecars {
				appendClosure("runtime", runtimePlan.InstanceCode+"/"+component.Name, component, "", "", nil)
			}
			for _, pod := range compiledRuntime.Pods {
				for _, component := range pod.Containers {
					appendClosure("runtime", runtimePlan.InstanceCode+"/"+pod.Name+"/"+component.Name, component, "", "", nil)
				}
			}
		}
		components := make([]contracts.CompiledComponentSnapshot, 0, len(selected))
		for _, ref := range selected {
			tool := compiledTools[ref.Code]
			category := tool.Category
			resourceSpec, marshalErr := json.Marshal(tool.ResourceSpec)
			if marshalErr != nil {
				return fmt.Errorf("编码组件 %s 工作负载失败: %w", ref.Code, marshalErr)
			}
			components = append(components, contracts.CompiledComponentSnapshot{
				ComponentID:  tool.ID,
				Category:     category,
				Code:         tool.Code,
				Kind:         tool.Kind,
				ResourceSpec: resourceSpec,
			})
			for _, component := range tool.ResourceSpec.Components {
				appendClosure(category, ref.Code, component, "", "", tool.ResourceSpec.PrepullCommand)
			}
		}
		compiledSnapshots := make([]contracts.CompiledRuntimeSnapshot, 0, len(runtimePlans))
		for _, runtimePlan := range runtimePlans {
			adapterSpec, err := json.Marshal(compiledRuntimes[runtimePlan.InstanceCode])
			if err != nil {
				return fmt.Errorf("编码运行时 %s 适配器失败: %w", runtimePlan.Runtime.Code, err)
			}
			compiledSnapshots = append(compiledSnapshots, contracts.CompiledRuntimeSnapshot{InstanceCode: runtimePlan.InstanceCode, RuntimeID: runtimePlan.Runtime.ID, ImageID: runtimePlan.Image.ID, Code: runtimePlan.Runtime.Code, Eco: runtimePlan.Runtime.Eco, AdapterLevel: runtimePlan.Runtime.AdapterLevel, CapabilityImpl: runtimePlan.Runtime.CapabilityImpl, AdapterSpec: adapterSpec, ImageURL: runtimePlan.Image.ImageURL, ImageVersion: runtimePlan.Image.Version})
		}
		snapshot = contracts.SandboxCompositionSnapshot{
			Spec:         spec,
			Runtimes:     compiledSnapshots,
			Components:   components,
			ImageClosure: closure,
		}
		digest, err := contracts.CanonicalSnapshotDigest(snapshot)
		if err != nil {
			return err
		}
		snapshot.Digest = digest
		rawSnapshot, err := json.Marshal(snapshot)
		if err != nil {
			return fmt.Errorf("编码组合快照失败: %w", err)
		}
		if err := tx.UpsertPublishedCompositionSnapshot(ctx, digest, rawSnapshot); err != nil {
			return fmt.Errorf("登记组合快照失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return contracts.SandboxCompositionSnapshot{}, err
	}
	return snapshot, nil
}

// validateRuntimeVolumeDomains 拒绝同名安全域在不同 runtime 中使用不同契约,避免工具挂载和 PVC 语义依赖数组顺序。
func validateRuntimeVolumeDomains(plans []RuntimePlan) error {
	seen := map[string]VolumeDomainSpec{}
	for _, plan := range plans {
		for _, domain := range plan.Runtime.AdapterSpec.VolumeDomains {
			name := strings.TrimSpace(domain.Name)
			if name == "" {
				continue
			}
			if existing, ok := seen[name]; ok && existing != domain {
				return fmt.Errorf("运行时实例 %s 的安全域 %s 与其他运行时契约冲突", plan.InstanceCode, name)
			}
			seen[name] = domain
		}
	}
	return nil
}

// immutableImageVersion derives a stable display version from the locked digest, never from a mutable tag.
func immutableImageVersion(imageURL string) string {
	const marker = "@sha256:"
	imageURL = strings.TrimSpace(imageURL)
	index := strings.Index(imageURL, marker)
	if index < 0 {
		return "unverified"
	}
	digest := imageURL[index+len(marker):]
	if len(digest) > 16 {
		digest = digest[:16]
	}
	if digest == "" {
		return "unverified"
	}
	return "sha256-" + digest
}

// runtimeCapabilitiesByInstance indexes capability declarations by the stable runtime instance alias.
func runtimeCapabilitiesByInstance(plans []RuntimePlan) map[string]CapabilityConstraints {
	out := make(map[string]CapabilityConstraints, len(plans))
	for _, plan := range plans {
		out[plan.InstanceCode] = plan.Runtime.AdapterSpec.Capabilities
	}
	return out
}

// aggregateRuntimeCapabilities exposes only the union of runtime capabilities to dependency checks.
func aggregateRuntimeCapabilities(items []CapabilityConstraints) CapabilityConstraints {
	out := CapabilityConstraints{}
	seenProvides := map[string]struct{}{}
	seenRequires := map[string]struct{}{}
	seenConflicts := map[string]struct{}{}
	for _, item := range items {
		for _, value := range item.Provides {
			if value = strings.TrimSpace(value); value != "" {
				if _, ok := seenProvides[value]; !ok {
					out.Provides = append(out.Provides, value)
					seenProvides[value] = struct{}{}
				}
			}
		}
		for _, value := range item.Requires {
			if value = strings.TrimSpace(value); value != "" {
				if _, ok := seenRequires[value]; !ok {
					out.Requires = append(out.Requires, value)
					seenRequires[value] = struct{}{}
				}
			}
		}
		for _, value := range item.Conflicts {
			if value = strings.TrimSpace(value); value != "" {
				if _, ok := seenConflicts[value]; !ok {
					out.Conflicts = append(out.Conflicts, value)
					seenConflicts[value] = struct{}{}
				}
			}
		}
	}
	return out
}

// validateToolMountDomainsForComposition 只允许工具挂载当前运行时已声明且与能力匹配的安全域。
func validateToolMountDomainsForComposition(tool Tool, runtime AdapterSpec) error {
	declared := map[string]struct{}{}
	for _, domain := range runtime.VolumeDomains {
		declared[strings.TrimSpace(domain.Name)] = struct{}{}
	}
	requires := map[string]struct{}{}
	for _, capability := range tool.ResourceSpec.Capabilities.Requires {
		requires[strings.TrimSpace(capability)] = struct{}{}
	}
	for _, component := range tool.ResourceSpec.Components {
		for _, raw := range component.MountDomains {
			domain := strings.TrimSpace(raw)
			if _, ok := declared[domain]; !ok {
				return fmt.Errorf("组件 %s 未声明安全域 %s", component.Name, domain)
			}
			if domain == VolumeDomainRuntimeState {
				if _, ok := requires["fabric-crypto-identity"]; !ok {
					if _, generic := requires["runtime-state-reader"]; !generic {
						return fmt.Errorf("组件 %s 没有读取 runtime-state 所需能力", component.Name)
					}
				}
			}
		}
	}
	return nil
}

// validateCompositionBindings 将 manifest 声明的角色绑定与组合 links 一一核对。
// 组件只能通过声明的能力、端点、协议和环境变量接收连接,禁止运行时注入未声明字段。
func validateCompositionBindings(spec contracts.SandboxCompositionSpec, runtimes map[string]CapabilityConstraints, refs []contracts.CompositionComponentRef, tools map[string]Tool) error {
	declared := map[string]map[string]ComponentBinding{}
	for _, ref := range refs {
		tool, ok := tools[ref.Code]
		if !ok {
			return fmt.Errorf("组件 %s 未注册", ref.Code)
		}
		byEnv := map[string]ComponentBinding{}
		for _, binding := range tool.ResourceSpec.Bindings {
			parsed, err := parseCompositionBinding(binding.ConfigBinding)
			if err != nil {
				return fmt.Errorf("组件 %s 绑定 %s 无效: %w", ref.Code, binding.Name, err)
			}
			byEnv[parsed.Name] = binding
		}
		declared[ref.Code] = byEnv
	}
	bound := map[string]map[string]struct{}{}
	for _, link := range spec.Links {
		sourceCode := strings.TrimSpace(link.SourceComponent)
		if _, isRuntime := runtimes[sourceCode]; isRuntime {
			continue
		}
		byEnv, ok := declared[sourceCode]
		if !ok {
			return fmt.Errorf("连接源组件 %s 未注册", sourceCode)
		}
		binding, err := parseCompositionBinding(link.ConfigBinding)
		if err != nil {
			return err
		}
		declaredBinding, ok := byEnv[binding.Name]
		if !ok {
			return fmt.Errorf("组件 %s 未声明连接环境变量 %s", sourceCode, binding.Name)
		}
		if strings.TrimSpace(link.SourceEndpoint) != strings.TrimSpace(declaredBinding.Endpoint) {
			return fmt.Errorf("组件 %s 绑定 %s 的源端点不匹配", sourceCode, declaredBinding.Name)
		}
		if strings.TrimSpace(link.Protocol) != strings.TrimSpace(declaredBinding.Protocol) {
			return fmt.Errorf("组件 %s 绑定 %s 的协议不匹配", sourceCode, declaredBinding.Name)
		}
		if link.RequiredAtStart != declaredBinding.RequiredAtStart {
			return fmt.Errorf("组件 %s 绑定 %s 的启动前置不匹配", sourceCode, declaredBinding.Name)
		}
		if !compositionTargetProvides(link.TargetComponent, declaredBinding.Capability, runtimes, tools) {
			return fmt.Errorf("组件 %s 绑定 %s 的目标未提供能力 %s", sourceCode, declaredBinding.Name, declaredBinding.Capability)
		}
		if bound[sourceCode] == nil {
			bound[sourceCode] = map[string]struct{}{}
		}
		if _, exists := bound[sourceCode][binding.Name]; exists {
			return fmt.Errorf("组件 %s 绑定 %s 重复连接", sourceCode, declaredBinding.Name)
		}
		bound[sourceCode][binding.Name] = struct{}{}
	}
	for componentCode, byEnv := range declared {
		for envName, binding := range byEnv {
			if _, ok := bound[componentCode][envName]; !ok {
				return fmt.Errorf("组件 %s 的绑定 %s 未在组合 links 中连接", componentCode, binding.Name)
			}
		}
	}
	return nil
}

// compositionTargetProvides 确认连接目标的 manifest 能力声明覆盖源组件要求的能力。
// 组合图只允许连接到已声明 runtime 实例或已选组件,不通过组件编码维护硬编码配对表。
func compositionTargetProvides(target, capability string, runtimes map[string]CapabilityConstraints, tools map[string]Tool) bool {
	target = strings.TrimSpace(target)
	capability = strings.TrimSpace(capability)
	if runtime, ok := runtimes[target]; ok {
		for _, provided := range runtime.Provides {
			if strings.TrimSpace(provided) == capability {
				return true
			}
		}
		return false
	}
	tool, ok := tools[target]
	if !ok {
		return false
	}
	for _, provided := range tool.ResourceSpec.Capabilities.Provides {
		if strings.TrimSpace(provided) == capability {
			return true
		}
	}
	return false
}

// validateComponentParams 按 manifest 声明的最小 JSON Schema 子集校验组件参数。
func validateComponentParams(params map[string]any, schema map[string]any) error {
	if len(schema) == 0 {
		if len(params) > 0 {
			return fmt.Errorf("组件未声明可配置参数")
		}
		return nil
	}
	typ, _ := schema["type"].(string)
	if typ != "" && typ != "object" {
		return fmt.Errorf("参数 schema 必须是 object")
	}
	properties, _ := schema["properties"].(map[string]any)
	additional, declared := schema["additionalProperties"]
	for key, value := range params {
		property, ok := properties[key].(map[string]any)
		if !ok {
			if !declared {
				return fmt.Errorf("包含未声明参数 %s", key)
			}
			allowed, isBool := additional.(bool)
			if isBool && !allowed {
				return fmt.Errorf("包含未声明参数 %s", key)
			}
			continue
		}
		if err := validateSchemaValue(key, value, property); err != nil {
			return err
		}
	}
	if required, ok := schema["required"].([]any); ok {
		for _, item := range required {
			name, _ := item.(string)
			if name != "" {
				if _, exists := params[name]; !exists {
					return fmt.Errorf("缺少必填参数 %s", name)
				}
			}
		}
	}
	return nil
}

// applyCompositionParams 编译 manifest 声明的非敏感参数到唯一目标容器环境变量。
// 参数映射来自 JSON Schema 属性上的 x-chaimir-env,平台不按组件编码维护硬编码分支。
func applyCompositionParams(params map[string]any, schema map[string]any, components *[]workload.ComponentSpec) error {
	if len(params) == 0 {
		return nil
	}
	if components == nil || len(*components) == 0 {
		return fmt.Errorf("组件没有可注入参数的容器")
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return fmt.Errorf("参数 schema 缺少 properties 映射")
	}
	for name, value := range params {
		property, ok := properties[name].(map[string]any)
		if !ok {
			return fmt.Errorf("参数 %s 未声明 x-chaimir-env 映射", name)
		}
		binding, ok := property["x-chaimir-env"].(string)
		if !ok || strings.TrimSpace(binding) == "" {
			return fmt.Errorf("参数 %s 未声明 x-chaimir-env 映射", name)
		}
		componentName, envName := "", strings.TrimSpace(binding)
		if parts := strings.SplitN(envName, "/", 2); len(parts) == 2 {
			componentName, envName = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
		if !envNamePattern.MatchString(envName) {
			return fmt.Errorf("参数 %s 的目标环境变量名无效", name)
		}
		componentIndex := -1
		for index := range *components {
			if componentName == "" || (*components)[index].Name == componentName {
				if componentIndex >= 0 {
					return fmt.Errorf("参数 %s 未唯一确定目标容器", name)
				}
				componentIndex = index
			}
		}
		if componentIndex < 0 {
			return fmt.Errorf("参数 %s 的目标容器 %s 不存在", name, componentName)
		}
		component := &(*components)[componentIndex]
		for _, secret := range component.SecretEnv {
			if strings.TrimSpace(secret.Name) == envName {
				return fmt.Errorf("参数 %s 不能写入敏感环境变量 %s", name, envName)
			}
		}
		literal, err := compositionParamString(value)
		if err != nil {
			return fmt.Errorf("参数 %s 值无法编码: %w", name, err)
		}
		for index := range component.Env {
			if component.Env[index].Name != envName {
				continue
			}
			if component.Env[index].Value != "" && component.Env[index].Value != literal {
				return fmt.Errorf("参数 %s 与环境变量 %s 的固定值冲突", name, envName)
			}
			component.Env[index].Value = literal
			goto nextParam
		}
		component.Env = append(component.Env, workload.EnvVarSpec{Name: envName, Value: literal})
	nextParam:
	}
	return nil
}

// compositionParamString 把 JSON 参数转换成环境变量允许的稳定文本表示。
func compositionParamString(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return "", fmt.Errorf("字符串参数不能为空")
		}
		return typed, nil
	case bool, int, int32, int64, float32, float64, json.Number:
		return fmt.Sprint(typed), nil
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	}
}

// validateSchemaValue 校验组件配置的基本 JSON Schema 类型、枚举和边界。
func validateSchemaValue(name string, value any, schema map[string]any) error {
	typ, _ := schema["type"].(string)
	valid := true
	switch typ {
	case "string":
		_, valid = value.(string)
	case "boolean":
		_, valid = value.(bool)
	case "number":
		switch value.(type) {
		case int, int32, int64, float32, float64, json.Number:
		default:
			valid = false
		}
	case "integer":
		switch number := value.(type) {
		case int, int32, int64:
			_ = number
		case float64:
			valid = number == float64(int64(number))
		case json.Number:
			_, err := strconv.ParseInt(string(number), 10, 64)
			valid = err == nil
		default:
			valid = false
		}
	case "array":
		_, valid = value.([]any)
	case "object":
		_, valid = value.(map[string]any)
	case "":
	default:
		return fmt.Errorf("参数 %s 的 schema 类型不支持", name)
	}
	if !valid {
		return fmt.Errorf("参数 %s 的类型不正确", name)
	}
	if enum, ok := schema["enum"].([]any); ok && !containsSchemaValue(enum, value) {
		return fmt.Errorf("参数 %s 的取值不在允许范围内", name)
	}
	return nil
}

func containsSchemaValue(values []any, target any) bool {
	for _, value := range values {
		if fmt.Sprint(value) == fmt.Sprint(target) {
			return true
		}
	}
	return false
}

func validateCapabilityCardinality(runtime CapabilityConstraints, refs []contracts.CompositionComponentRef, tools map[string]Tool) error {
	counts := map[string]int{}
	for _, item := range runtime.Provides {
		counts[strings.TrimSpace(item)]++
	}
	for _, ref := range refs {
		tool := tools[ref.Code]
		if strings.EqualFold(strings.TrimSpace(tool.ResourceSpec.Capabilities.Cardinality), "one") {
			for _, item := range tool.ResourceSpec.Capabilities.Provides {
				capability := strings.TrimSpace(item)
				if capability != "" {
					counts[capability]++
				}
			}
		}
	}
	for capability, count := range counts {
		if count > 1 && capability != "" {
			return fmt.Errorf("能力 %s 只能存在一个提供者", capability)
		}
	}
	return nil
}

func findCapabilityProvider(tools map[string]Tool, required, category string) (string, error) {
	required = strings.TrimSpace(required)
	codes := make([]string, 0, len(tools))
	for code := range tools {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	candidates := make([]string, 0, len(codes))
	for _, code := range codes {
		tool := tools[code]
		if tool.Category != category || strings.TrimSpace(tool.ResourceSpec.Capabilities.Placement) != "sandbox" {
			continue
		}
		for _, provided := range tool.ResourceSpec.Capabilities.Provides {
			if strings.TrimSpace(provided) == required && tool.Status == ToolStatusAvailable {
				candidates = append(candidates, code)
				break
			}
		}
	}
	if len(candidates) == 0 {
		return "", nil
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("能力 %s 存在多个候选提供者 %s,必须显式选择", required, strings.Join(candidates, ","))
	}
	return candidates[0], nil
}

func capabilityProvided(runtime CapabilityConstraints, refs []contracts.CompositionComponentRef, tools map[string]Tool, required string) bool {
	required = strings.TrimSpace(required)
	for _, provided := range runtime.Provides {
		if strings.TrimSpace(provided) == required {
			return true
		}
	}
	for _, ref := range refs {
		for _, provided := range tools[ref.Code].ResourceSpec.Capabilities.Provides {
			if strings.TrimSpace(provided) == required {
				return true
			}
		}
	}
	return false
}

func validateCompositionLinks(spec contracts.SandboxCompositionSpec, refs []contracts.CompositionComponentRef, tools map[string]Tool) error {
	known := map[string]struct{}{}
	for _, runtime := range spec.Runtimes {
		known[strings.TrimSpace(runtime.InstanceCode)] = struct{}{}
	}
	for _, ref := range refs {
		known[ref.Code] = struct{}{}
	}
	for _, link := range spec.Links {
		if _, ok := known[strings.TrimSpace(link.SourceComponent)]; !ok {
			return fmt.Errorf("连接源组件 %s 不存在", link.SourceComponent)
		}
		if _, ok := known[strings.TrimSpace(link.TargetComponent)]; !ok {
			return fmt.Errorf("连接目标组件 %s 不存在", link.TargetComponent)
		}
		if strings.TrimSpace(link.Protocol) == "" || strings.TrimSpace(link.SourceEndpoint) == "" || strings.TrimSpace(link.TargetEndpoint) == "" || strings.TrimSpace(link.AccessScope) == "" || strings.TrimSpace(link.ConfigBinding) == "" {
			return fmt.Errorf("连接声明不完整")
		}
		if _, isRuntime := known[strings.TrimSpace(link.SourceComponent)]; !isRuntime && tools[link.SourceComponent].Code == "" {
			return fmt.Errorf("连接源组件 %s 未注册", link.SourceComponent)
		}
	}
	return nil
}

// compileCompositionLinks 把声明式连接编译成源组件环境变量和最小网络规则。
// 连接只允许注入同一沙箱内已声明 Service 的地址,不接受任意 URL、Secret 或外网出口。
func compileCompositionLinks(spec contracts.SandboxCompositionSpec, runtimes map[string]AdapterSpec, runtimePlans []RuntimePlan, refs []contracts.CompositionComponentRef, tools map[string]Tool) (map[string]AdapterSpec, map[string]Tool, error) {
	compiledRuntimes := make(map[string]AdapterSpec, len(runtimes))
	for instance, runtime := range runtimes {
		cloned, err := cloneAdapterSpec(runtime)
		if err != nil {
			return nil, nil, err
		}
		compiledRuntimes[instance] = cloned
	}
	compiledTools := make(map[string]Tool, len(refs))
	for _, ref := range refs {
		tool, ok := tools[ref.Code]
		if !ok {
			return nil, nil, fmt.Errorf("连接组件 %s 未注册", ref.Code)
		}
		cloned, cloneErr := cloneTool(tool)
		if cloneErr != nil {
			return nil, nil, cloneErr
		}
		compiledTools[ref.Code] = cloned
	}
	for _, runtimePlan := range runtimePlans {
		compiledRuntime := compiledRuntimes[runtimePlan.InstanceCode]
		runtimeComponents := []workload.ComponentSpec{compiledRuntime.RuntimeContainer}
		runtimeComponents = append(runtimeComponents, compiledRuntime.InfraSidecars...)
		for _, pod := range compiledRuntime.Pods {
			runtimeComponents = append(runtimeComponents, pod.Containers...)
		}
		for _, ref := range spec.Runtimes {
			if ref.InstanceCode != runtimePlan.InstanceCode {
				continue
			}
			if err := applyCompositionParams(ref.Params, compiledRuntime.Capabilities.ConfigSchema, &runtimeComponents); err != nil {
				return nil, nil, fmt.Errorf("运行时 %s 参数编译失败: %w", runtimePlan.InstanceCode, err)
			}
		}
		componentIndex := 0
		compiledRuntime.RuntimeContainer = runtimeComponents[componentIndex]
		componentIndex++
		for index := range compiledRuntime.InfraSidecars {
			compiledRuntime.InfraSidecars[index] = runtimeComponents[componentIndex]
			componentIndex++
		}
		for podIndex := range compiledRuntime.Pods {
			for containerIndex := range compiledRuntime.Pods[podIndex].Containers {
				compiledRuntime.Pods[podIndex].Containers[containerIndex] = runtimeComponents[componentIndex]
				componentIndex++
			}
		}
		compiledRuntimes[runtimePlan.InstanceCode] = compiledRuntime
		compiledRuntimes[runtimePlan.InstanceCode] = materializeRuntimeTopology(runtimePlan.InstanceCode, compiledRuntime)
	}
	for _, ref := range refs {
		tool := compiledTools[ref.Code]
		if err := applyCompositionParams(ref.Params, tool.ResourceSpec.Capabilities.ConfigSchema, &tool.ResourceSpec.Components); err != nil {
			return nil, nil, fmt.Errorf("组件 %s 参数编译失败: %w", ref.Code, err)
		}
		compiledTools[ref.Code] = tool
	}
	seenBindings := map[string]string{}
	seenRules := map[string]struct{}{}
	for _, compiledRuntime := range compiledRuntimes {
		for _, rule := range compiledRuntime.NetworkRules {
			seenRules[strings.TrimSpace(rule.Name)] = struct{}{}
		}
	}
	for _, tool := range compiledTools {
		for _, rule := range tool.ResourceSpec.NetworkRules {
			seenRules[strings.TrimSpace(rule.Name)] = struct{}{}
		}
	}
	for _, link := range spec.Links {
		binding, err := parseCompositionBinding(link.ConfigBinding)
		if err != nil {
			return nil, nil, err
		}
		source, err := resolveCompositionEndpoint(link.SourceComponent, link.SourceEndpoint, true, compiledRuntimes, compiledTools)
		if err != nil {
			return nil, nil, err
		}
		target, err := resolveCompositionEndpoint(link.TargetComponent, link.TargetEndpoint, false, compiledRuntimes, compiledTools)
		if err != nil {
			return nil, nil, err
		}
		if err := validateCompositionProtocol(link.Protocol, target.Protocol); err != nil {
			return nil, nil, err
		}
		if binding.Kind == "env" {
			key := source.ComponentCode + "\x00" + source.ComponentName + "\x00" + binding.Name
			value := compositionBindingValue(link.Protocol, target)
			if previous, exists := seenBindings[key]; exists {
				if previous != value {
					return nil, nil, fmt.Errorf("组件 %s 的环境变量 %s 存在冲突连接", source.ComponentCode, binding.Name)
				}
				return nil, nil, fmt.Errorf("组件 %s 的环境变量 %s 存在重复连接", source.ComponentCode, binding.Name)
			}
			seenBindings[key] = value
			if err := setComponentEnv(source.ComponentCode, source.ComponentName, binding.Name, value, compiledRuntimes, compiledTools); err != nil {
				return nil, nil, err
			}
		}
		rule := workload.NetworkRuleSpec{
			Name:  compositionRuleName(link),
			From:  source.Role,
			To:    target.Role,
			Ports: []workload.NetworkPortRef{{Name: target.EndpointName, Port: target.Port}},
		}
		if _, isRuntime := compiledRuntimes[source.ComponentCode]; !isRuntime {
			// 工具自身的规则以组件名表达来源,编排层再转换为该组件 Pod 角色。
			rule.From = source.ComponentName
		}
		if _, exists := seenRules[rule.Name]; exists {
			return nil, nil, fmt.Errorf("连接网络规则重复: %s", rule.Name)
		}
		seenRules[rule.Name] = struct{}{}
		if compiledRuntime, isRuntime := compiledRuntimes[source.ComponentCode]; isRuntime {
			compiledRuntime.NetworkRules = append(compiledRuntime.NetworkRules, rule)
			compiledRuntimes[source.ComponentCode] = compiledRuntime
		} else {
			tool := compiledTools[source.ComponentCode]
			tool.ResourceSpec.NetworkRules = append(tool.ResourceSpec.NetworkRules, rule)
			compiledTools[source.ComponentCode] = tool
		}
	}
	startupOrder, err := compileStartupOrder(spec, refs)
	if err != nil {
		return nil, nil, err
	}
	for instance, compiledRuntime := range compiledRuntimes {
		compiledRuntime.StartupOrder = startupOrder
		compiledRuntimes[instance] = compiledRuntime
	}
	return compiledRuntimes, compiledTools, nil
}

// compileStartupOrder 将 required_at_start 连接转换成确定性的组件启动顺序,并拒绝循环依赖。
func compileStartupOrder(spec contracts.SandboxCompositionSpec, refs []contracts.CompositionComponentRef) ([]string, error) {
	nodes := make([]string, 0, len(spec.Runtimes)+len(refs))
	seen := map[string]struct{}{}
	for _, runtime := range spec.Runtimes {
		instance := strings.TrimSpace(runtime.InstanceCode)
		if instance != "" {
			nodes = append(nodes, instance)
			seen[instance] = struct{}{}
		}
	}
	for _, ref := range refs {
		code := strings.TrimSpace(ref.Code)
		if code == "" {
			continue
		}
		if _, exists := seen[code]; !exists {
			seen[code] = struct{}{}
			nodes = append(nodes, code)
		}
	}
	dependencies := make(map[string]map[string]struct{}, len(nodes))
	dependents := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		dependencies[node] = map[string]struct{}{}
	}
	for _, link := range spec.Links {
		if !link.RequiredAtStart {
			continue
		}
		source := strings.TrimSpace(link.SourceComponent)
		target := strings.TrimSpace(link.TargetComponent)
		if source == "" || target == "" || source == target {
			return nil, fmt.Errorf("required_at_start 连接的源和目标必须是两个已声明组件")
		}
		if _, ok := dependencies[source]; !ok {
			return nil, fmt.Errorf("required_at_start 源组件 %s 不存在", source)
		}
		if _, ok := dependencies[target]; !ok {
			return nil, fmt.Errorf("required_at_start 目标组件 %s 不存在", target)
		}
		if _, exists := dependencies[source][target]; exists {
			continue
		}
		dependencies[source][target] = struct{}{}
		dependents[target] = append(dependents[target], source)
	}
	if len(dependents) == 0 {
		return nil, nil
	}
	for _, values := range dependents {
		sort.Strings(values)
	}
	ready := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if len(dependencies[node]) == 0 {
			ready = append(ready, node)
		}
	}
	sort.Strings(ready)
	order := make([]string, 0, len(nodes))
	for len(ready) > 0 {
		node := ready[0]
		ready = ready[1:]
		order = append(order, node)
		for _, dependent := range dependents[node] {
			delete(dependencies[dependent], node)
			if len(dependencies[dependent]) == 0 {
				ready = append(ready, dependent)
				sort.Strings(ready)
			}
		}
	}
	if len(order) != len(nodes) {
		return nil, fmt.Errorf("required_at_start 连接存在循环依赖")
	}
	return order, nil
}

// compositionBindingValue 按消费协议生成连接地址;TCP/GRPC 消费者通常需要 host:port,HTTP 消费者需要 URL。
func compositionBindingValue(protocol string, endpoint compositionEndpoint) string {
	protocol = strings.ToUpper(strings.TrimSpace(protocol))
	if protocol == "TCP" || protocol == "GRPC" {
		return fmt.Sprintf("%s:%d", endpoint.Service, endpoint.Port)
	}
	return endpoint.URL
}

// validateToolNetworkRulesForComposition 校验编译后工具规则的来源和目标都属于本次冻结组合。
func validateToolNetworkRulesForComposition(tool Tool, runtimes map[string]AdapterSpec, tools map[string]Tool) error {
	componentPorts := componentPortMap(tool.ResourceSpec.Components)
	runtimePorts := map[string]map[string]int32{}
	for instance, runtime := range runtimes {
		for role, ports := range podPortMap(podTopologyForAdapter(runtime)) {
			runtimePorts[instance+"/"+role] = ports
			runtimePorts[role] = ports
		}
	}
	for _, rule := range tool.ResourceSpec.NetworkRules {
		if _, ok := componentPorts[rule.From]; !ok {
			return fmt.Errorf("来源组件 %s 未声明", rule.From)
		}
		var targetPorts map[string]int32
		if ports, ok := runtimePorts[rule.To]; ok {
			targetPorts = ports
		} else if ports, ok := componentPorts[rule.To]; ok {
			targetPorts = ports
		} else {
			for _, candidate := range tools {
				for _, component := range candidate.ResourceSpec.Components {
					if toolComponentPodName(candidate.Code, component.Name) == rule.To {
						targetPorts = componentPortMap([]workload.ComponentSpec{component})[component.Name]
					}
				}
			}
		}
		if len(targetPorts) == 0 {
			return fmt.Errorf("目标 Pod 或组件 %s 未声明", rule.To)
		}
		for _, port := range rule.Ports {
			resolved := port.Port
			if port.Name != "" {
				var ok bool
				resolved, ok = targetPorts[port.Name]
				if !ok {
					return fmt.Errorf("目标 %s 端口 %s 未声明", rule.To, port.Name)
				}
			}
			if !networkPortDeclared(targetPorts, resolved) {
				return fmt.Errorf("目标 %s 端口 %d 未声明", rule.To, resolved)
			}
		}
	}
	return nil
}

type compositionBinding struct {
	Kind string
	Name string
}

type compositionEndpoint struct {
	ComponentCode string
	ComponentName string
	Role          string
	Service       string
	EndpointName  string
	Protocol      string
	Port          int32
	URL           string
}

// parseCompositionBinding 解析唯一的环境变量绑定语法 env:NAME。
func parseCompositionBinding(raw string) (compositionBinding, error) {
	parts := strings.SplitN(strings.TrimSpace(raw), ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) != "env" || !envNamePattern.MatchString(strings.TrimSpace(parts[1])) {
		return compositionBinding{}, fmt.Errorf("连接配置绑定必须使用 env:NAME 语法")
	}
	return compositionBinding{Kind: "env", Name: strings.TrimSpace(parts[1])}, nil
}

// resolveCompositionEndpoint 从运行时 Service 或工具 Service 解析稳定的内网地址。
func resolveCompositionEndpoint(componentCode, endpoint string, source bool, runtimes map[string]AdapterSpec, tools map[string]Tool) (compositionEndpoint, error) {
	componentCode = strings.TrimSpace(componentCode)
	endpoint = strings.TrimSpace(endpoint)
	if runtime, ok := runtimes[componentCode]; ok {
		for _, pod := range podTopologyForAdapter(runtime) {
			for _, component := range pod.Containers {
				for _, port := range component.Ports {
					if port.Name != endpoint {
						continue
					}
					return compositionEndpoint{
						ComponentCode: componentCode,
						ComponentName: component.Name,
						Role:          pod.Name,
						Service:       runtimeServiceName(pod.Name),
						EndpointName:  port.Name,
						Protocol:      port.Protocol,
						Port:          port.ServicePort,
						URL:           endpointURL(runtimeServiceName(pod.Name), port.ServicePort, port.Protocol),
					}, nil
				}
			}
		}
		return compositionEndpoint{}, fmt.Errorf("运行时端点 %s 不存在", endpoint)
	}
	tool, ok := tools[componentCode]
	if !ok {
		return compositionEndpoint{}, fmt.Errorf("组件 %s 不存在", componentCode)
	}
	for _, service := range tool.ResourceSpec.Services {
		for _, servicePort := range service.Ports {
			if servicePort.Name != endpoint {
				continue
			}
			for _, component := range tool.ResourceSpec.Components {
				if component.Name != service.Component {
					continue
				}
				for _, port := range component.Ports {
					if port.Name != servicePort.TargetPort {
						continue
					}
					return compositionEndpoint{
						ComponentCode: componentCode,
						ComponentName: component.Name,
						Role:          toolComponentPodName(componentCode, component.Name),
						Service:       service.Name,
						EndpointName:  servicePort.Name,
						Protocol:      servicePort.Protocol,
						Port:          servicePort.Port,
						URL:           endpointURL(service.Name, servicePort.Port, servicePort.Protocol),
					}, nil
				}
			}
		}
	}
	if source {
		for _, component := range tool.ResourceSpec.Components {
			for _, port := range component.Ports {
				if port.Name == endpoint {
					return compositionEndpoint{ComponentCode: componentCode, ComponentName: component.Name, Role: toolComponentPodName(componentCode, component.Name), EndpointName: port.Name, Protocol: port.Protocol, Port: port.ContainerPort}, nil
				}
			}
		}
	}
	return compositionEndpoint{}, fmt.Errorf("组件 %s 端点 %s 不存在可发现 Service", componentCode, endpoint)
}

// endpointURL 只生成沙箱命名空间内的协议地址,禁止把外部主机写入组合快照。
func endpointURL(service string, port int32, protocol string) string {
	scheme := "http"
	switch strings.ToUpper(strings.TrimSpace(protocol)) {
	case "WS":
		scheme = "ws"
	case "WSS":
		scheme = "wss"
	case "HTTPS":
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, service, port)
}

// validateCompositionProtocol 校验声明协议与目标端口协议一致。
func validateCompositionProtocol(declared, target string) error {
	declared = strings.ToUpper(strings.TrimSpace(declared))
	target = strings.ToUpper(strings.TrimSpace(target))
	if declared == "" || target == "" {
		return fmt.Errorf("连接协议不能为空")
	}
	if declared == target || (declared == "HTTP" && target == "TCP") || (declared == "HTTPS" && target == "TCP") || (declared == "WS" && target == "TCP") || (declared == "WSS" && target == "TCP") {
		return nil
	}
	return fmt.Errorf("连接协议 %s 与目标端口协议 %s 不匹配", declared, target)
}

// setComponentEnv 把编译后的地址写入源组件,拒绝覆盖 manifest 中不同的固定值。
func setComponentEnv(componentCode, componentName, name, value string, runtimes map[string]AdapterSpec, tools map[string]Tool) error {
	if runtime, ok := runtimes[componentCode]; ok {
		component := &runtime.RuntimeContainer
		if component.Name != componentName {
			component = nil
			for index := range runtime.InfraSidecars {
				candidate := &runtime.InfraSidecars[index]
				if candidate.Name == componentName {
					component = candidate
					break
				}
			}
		}
		if component == nil {
			for podIndex := range runtime.Pods {
				for containerIndex := range runtime.Pods[podIndex].Containers {
					candidate := &runtime.Pods[podIndex].Containers[containerIndex]
					if candidate.Name == componentName {
						component = candidate
						break
					}
				}
				if component != nil {
					break
				}
			}
		}
		if component == nil {
			return fmt.Errorf("runtime %s 源容器 %s 不存在", componentCode, componentName)
		}
		for _, secret := range component.SecretEnv {
			if strings.TrimSpace(secret.Name) == name {
				return fmt.Errorf("连接不得写入 runtime %s 敏感环境变量 %s", componentCode, name)
			}
		}
		for index := range component.Env {
			if component.Env[index].Name == name {
				if component.Env[index].Value != "" && component.Env[index].Value != value {
					return fmt.Errorf("runtime %s 的环境变量 %s 已声明不同值", componentCode, name)
				}
				component.Env[index].Value = value
				runtimes[componentCode] = runtime
				return nil
			}
		}
		component.Env = append(component.Env, workload.EnvVarSpec{Name: name, Value: value})
		runtimes[componentCode] = runtime
		return nil
	}
	tool := tools[componentCode]
	for index := range tool.ResourceSpec.Components {
		component := &tool.ResourceSpec.Components[index]
		if component.Name != componentName {
			continue
		}
		for _, secret := range component.SecretEnv {
			if strings.TrimSpace(secret.Name) == name {
				return fmt.Errorf("连接不得写入组件 %s 敏感环境变量 %s", componentCode, name)
			}
		}
		for envIndex := range component.Env {
			if component.Env[envIndex].Name == name {
				if component.Env[envIndex].Value != "" && component.Env[envIndex].Value != value {
					return fmt.Errorf("组件 %s 的环境变量 %s 已声明不同值", componentCode, name)
				}
				component.Env[envIndex].Value = value
				tools[componentCode] = tool
				return nil
			}
		}
		component.Env = append(component.Env, workload.EnvVarSpec{Name: name, Value: value})
		tools[componentCode] = tool
		return nil
	}
	return fmt.Errorf("组件 %s 的源容器 %s 不存在", componentCode, componentName)
}

// materializeRuntimeTopology 为每个运行时实例生成唯一 Pod 角色,保证多链组合中的同名拓扑不会碰撞。
func materializeRuntimeTopology(instance string, adapter AdapterSpec) AdapterSpec {
	instance = strings.TrimSpace(instance)
	if instance == "" {
		return adapter
	}
	podNames := make(map[string]string)
	if len(adapter.Pods) == 0 {
		containers := []workload.ComponentSpec{adapter.RuntimeContainer}
		containers = append(containers, adapter.InfraSidecars...)
		adapter.Pods = []workload.PodSpec{{Name: instance + "-sandbox", Containers: containers}}
		podNames["sandbox"] = instance + "-sandbox"
	} else {
		for index := range adapter.Pods {
			original := strings.TrimSpace(adapter.Pods[index].Name)
			if original == "" {
				original = fmt.Sprintf("pod-%d", index+1)
			}
			prefixed := instance + "-" + original
			adapter.Pods[index].Name = prefixed
			podNames[original] = prefixed
		}
	}
	for index := range adapter.NetworkRules {
		if replacement, ok := podNames[strings.TrimSpace(adapter.NetworkRules[index].From)]; ok {
			adapter.NetworkRules[index].From = replacement
		}
		if replacement, ok := podNames[strings.TrimSpace(adapter.NetworkRules[index].To)]; ok {
			adapter.NetworkRules[index].To = replacement
		}
	}
	for index, item := range adapter.StartupOrder {
		if replacement, ok := podNames[strings.TrimSpace(item)]; ok {
			adapter.StartupOrder[index] = replacement
		}
	}
	adapter.WorkspaceOps.ExecTarget = rewriteRuntimeExecTarget(adapter.WorkspaceOps.ExecTarget, podNames)
	adapter.PrivateArchiveOps.ExecTarget = rewriteRuntimeExecTarget(adapter.PrivateArchiveOps.ExecTarget, podNames)
	adapter.CapabilityCommands.Deploy.ExecTarget = rewriteRuntimeExecTarget(adapter.CapabilityCommands.Deploy.ExecTarget, podNames)
	adapter.CapabilityCommands.Tx.ExecTarget = rewriteRuntimeExecTarget(adapter.CapabilityCommands.Tx.ExecTarget, podNames)
	adapter.CapabilityCommands.Query.ExecTarget = rewriteRuntimeExecTarget(adapter.CapabilityCommands.Query.ExecTarget, podNames)
	adapter.CapabilityCommands.Reset.ExecTarget = rewriteRuntimeExecTarget(adapter.CapabilityCommands.Reset.ExecTarget, podNames)
	return adapter
}

// rewriteRuntimeExecTarget prefixes the Pod portion of a declared pod/container target.
// Container names remain unchanged; an unknown Pod is left intact so later topology validation reports it.
func rewriteRuntimeExecTarget(target string, podNames map[string]string) string {
	parts := strings.SplitN(strings.TrimSpace(target), "/", 2)
	if len(parts) != 2 {
		return target
	}
	if replacement, ok := podNames[strings.TrimSpace(parts[0])]; ok {
		return replacement + "/" + strings.TrimSpace(parts[1])
	}
	return target
}

// compositionRuleName 为连接生成稳定且可审计的网络策略名称。
func compositionRuleName(link contracts.CompositionLink) string {
	value := strings.Join([]string{link.SourceComponent, link.SourceEndpoint, link.TargetComponent, link.TargetEndpoint}, "-")
	value = strings.ToLower(strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, value))
	value = strings.Trim(value, "-")
	if len(value) > 50 {
		value = value[:50]
	}
	return "link-" + value
}

// cloneAdapterSpec 和 cloneTool 通过 JSON 做深复制,保证平台目录对象不会在编译过程中被修改。
func cloneAdapterSpec(input AdapterSpec) (AdapterSpec, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return AdapterSpec{}, fmt.Errorf("复制运行时适配器失败: %w", err)
	}
	var out AdapterSpec
	if err := json.Unmarshal(b, &out); err != nil {
		return AdapterSpec{}, fmt.Errorf("解析运行时适配器副本失败: %w", err)
	}
	return out, nil
}

func cloneTool(input Tool) (Tool, error) {
	b, err := json.Marshal(input.ResourceSpec)
	if err != nil {
		return Tool{}, fmt.Errorf("复制组件 %s 工作负载失败: %w", input.Code, err)
	}
	var spec ToolResourceSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		return Tool{}, fmt.Errorf("解析组件 %s 工作负载副本失败: %w", input.Code, err)
	}
	input.ResourceSpec = spec
	return input, nil
}

func validateCompositionAccess(profile contracts.SandboxAccessProfile, category, access string) error {
	if category == "infra" {
		return nil
	}
	if profile == contracts.SandboxAccessJudgePrivate || profile == contracts.SandboxAccessContestBattle {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(access), "private") {
		return fmt.Errorf("当前访问配置不能挂载私有组件")
	}
	return nil
}

func validateCompositionConflicts(runtime CapabilityConstraints, refs []contracts.CompositionComponentRef, tools map[string]Tool) error {
	capabilities := make(map[string]struct{}, len(runtime.Provides))
	for _, item := range runtime.Provides {
		capabilities[strings.TrimSpace(item)] = struct{}{}
	}
	for _, ref := range refs {
		tool := tools[ref.Code]
		for _, item := range tool.ResourceSpec.Capabilities.Provides {
			capabilities[strings.TrimSpace(item)] = struct{}{}
		}
	}
	for _, item := range runtime.Conflicts {
		if _, ok := capabilities[strings.TrimSpace(item)]; ok {
			return fmt.Errorf("组合能力冲突: %s", item)
		}
	}
	for _, ref := range refs {
		tool := tools[ref.Code]
		for _, conflict := range tool.ResourceSpec.Capabilities.Conflicts {
			if _, ok := capabilities[strings.TrimSpace(conflict)]; ok && strings.TrimSpace(conflict) != "" {
				return fmt.Errorf("组件 %s 与能力 %s 冲突", ref.Code, conflict)
			}
		}
		for _, required := range tool.ResourceSpec.Capabilities.Requires {
			if _, ok := capabilities[strings.TrimSpace(required)]; !ok {
				return fmt.Errorf("组件 %s 缺少必需能力 %s", ref.Code, required)
			}
		}
	}
	return nil
}
