package sandbox

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

type compositionCatalog struct {
	spec            contracts.SandboxCompositionSpec
	runtimePlans    []RuntimePlan
	runtimeAdapters map[string]AdapterSpec
	selected        []contracts.CompositionComponentRef
	tools           map[string]Tool
}

// loadCompositionCatalog 在调用方事务中共用原目录、参数、兼容性和依赖展开规则。
// 场景访问边界由最终编译检查,连接预览不替题库或业务调用者选择访问配置。
func loadCompositionCatalog(ctx context.Context, tx TxStore, spec contracts.SandboxCompositionSpec) (compositionCatalog, error) {
	runtimePlans := make([]RuntimePlan, 0, len(spec.Runtimes))
	runtimeAdapters := make(map[string]AdapterSpec, len(spec.Runtimes))
	runtimeCapabilities := make([]CapabilityConstraints, 0, len(spec.Runtimes))
	for _, ref := range spec.Runtimes {
		instance := ref.InstanceCode
		runtime, err := tx.GetRuntimeByCode(ctx, ref.Code)
		if err != nil {
			return compositionCatalog{}, apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		if runtime.Status != RuntimeStatusAvailable || runtime.SelftestStatus != RuntimeSelftestPassed {
			return compositionCatalog{}, fmt.Errorf("运行时 %s 尚未通过平台自检", runtime.Code)
		}
		if err := validateComponentParams(ref.Params, runtime.AdapterSpec.Capabilities.ConfigSchema); err != nil {
			return compositionCatalog{}, fmt.Errorf("运行时 %s 参数无效: %w", runtime.Code, err)
		}
		image, err := tx.GetRuntimeImageByVersionForShare(ctx, runtime.ID, ref.ImageVersion)
		if err != nil {
			return compositionCatalog{}, apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		runtimePlans = append(runtimePlans, RuntimePlan{InstanceCode: instance, Runtime: runtime, Image: image})
		runtimeAdapters[instance] = runtime.AdapterSpec
		runtimeCapabilities = append(runtimeCapabilities, runtime.AdapterSpec.Capabilities)
	}
	if len(runtimePlans) == 0 {
		return compositionCatalog{}, fmt.Errorf("组合至少声明一个 runtime 实例")
	}
	if err := validateRuntimeIsolation(runtimePlans); err != nil {
		return compositionCatalog{}, err
	}
	aggregatedCapabilities := aggregateRuntimeCapabilities(runtimeCapabilities)
	tools, err := tx.ListTools(ctx)
	if err != nil {
		return compositionCatalog{}, err
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
			return compositionCatalog{}, fmt.Errorf("组合包含空组件编码")
		}
		if _, exists := selectedCodes[code]; exists {
			return compositionCatalog{}, fmt.Errorf("组件 %s 重复选择", code)
		}
		selectedCodes[code] = struct{}{}
	}
	runtimesByInstance := runtimeCapabilitiesByInstance(runtimePlans)
	expandDependencies := func(owner string, required []string) error {
		for _, capability := range required {
			if capabilityProvided(runtimesByInstance, selected, toolByCode, capability, owner) {
				continue
			}
			provider, err := findCapabilityProvider(toolByCode, capability, "infra", owner)
			if err != nil {
				return err
			}
			if provider == "" {
				return fmt.Errorf("组件 %s 缺少必需能力 %s", owner, capability)
			}
			if _, exists := selectedCodes[provider]; exists {
				continue
			}
			selectedCodes[provider] = struct{}{}
			selected = append(selected, contracts.CompositionComponentRef{Code: provider, Selection: "auto"})
		}
		return nil
	}
	for _, runtime := range runtimePlans {
		if err := expandDependencies(runtime.InstanceCode, runtime.Runtime.AdapterSpec.Capabilities.Requires); err != nil {
			return compositionCatalog{}, err
		}
	}
	for idx := 0; idx < len(selected); idx++ {
		ref := selected[idx]
		code := strings.TrimSpace(ref.Code)
		if _, exists := runtimeAdapters[code]; exists {
			return compositionCatalog{}, fmt.Errorf("组件 %s 与运行时实例别名重复", code)
		}
		selection := strings.TrimSpace(ref.Selection)
		if selection != "explicit" && selection != "auto" {
			return compositionCatalog{}, fmt.Errorf("组件 %s 的选择来源无效", code)
		}
		if idx < explicitInfraCount+len(spec.Tools) && selection != "explicit" {
			return compositionCatalog{}, fmt.Errorf("教师声明的组件 %s 必须标记为 explicit", code)
		}
		tool, ok := toolByCode[code]
		if !ok || tool.Status != ToolStatusAvailable {
			return compositionCatalog{}, fmt.Errorf("组件 %s 不可用", code)
		}
		expectedCategory := "infra"
		if idx >= explicitInfraCount && idx < explicitInfraCount+len(spec.Tools) {
			expectedCategory = "tool"
		}
		if tool.Category != expectedCategory {
			return compositionCatalog{}, fmt.Errorf("组件 %s 的目录类别与声明位置不一致", code)
		}
		if strings.TrimSpace(tool.ResourceSpec.Capabilities.Placement) != "sandbox" {
			return compositionCatalog{}, fmt.Errorf("组件 %s 不能部署到沙箱", code)
		}
		if err := validateComponentParams(ref.Params, tool.ResourceSpec.Capabilities.ConfigSchema); err != nil {
			return compositionCatalog{}, fmt.Errorf("组件 %s 参数无效: %w", code, err)
		}
		if err := expandDependencies(code, tool.ResourceSpec.Capabilities.Requires); err != nil {
			return compositionCatalog{}, err
		}
	}
	if err := validateCompositionConflicts(aggregatedCapabilities, selected, toolByCode); err != nil {
		return compositionCatalog{}, err
	}
	if err := validateCapabilityCardinality(aggregatedCapabilities, selected, toolByCode); err != nil {
		return compositionCatalog{}, err
	}
	autoInfra := selected[explicitInfraCount+len(spec.Tools):]
	spec.Infra = append(append([]contracts.CompositionComponentRef(nil), spec.Infra...), autoInfra...)
	spec.Tools = append([]contracts.CompositionComponentRef(nil), spec.Tools...)
	return compositionCatalog{spec: spec, runtimePlans: runtimePlans, runtimeAdapters: runtimeAdapters, selected: selected, tools: toolByCode}, nil
}
