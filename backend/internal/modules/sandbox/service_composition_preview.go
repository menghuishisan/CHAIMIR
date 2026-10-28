// composition_preview 复用M2编译器解析连接候选,全程只读且不创建快照或执行资源。
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

// PreviewComposition 在同一目录事务内校验选择、展开依赖并生成可明确选择的连接。
func (s *Service) PreviewComposition(ctx context.Context, req CompositionPreviewRequest) (CompositionPreviewResponse, error) {
	if err := contracts.ValidateCompositionRuntimeSelection(req.Runtimes, req.WorkspaceRuntimeInstance); err != nil {
		return CompositionPreviewResponse{}, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	var out CompositionPreviewResponse
	err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		catalog, err := loadCompositionCatalog(ctx, tx, contracts.SandboxCompositionSpec{
			Runtimes: req.Runtimes, WorkspaceRuntimeInstance: req.WorkspaceRuntimeInstance,
			Infra: req.Infra, Tools: req.Tools, Links: req.Links,
		})
		if err != nil {
			return err
		}
		capabilities := aggregateRuntimeCapabilities(catalog.runtimeCapabilities)
		if err := validateCompositionConflicts(capabilities, catalog.selected, catalog.tools); err != nil {
			return err
		}
		if err := validateCapabilityCardinality(capabilities, catalog.selected, catalog.tools); err != nil {
			return err
		}
		out, err = previewCompositionBindings(catalog)
		return err
	})
	if err != nil {
		return CompositionPreviewResponse{}, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	return out, nil
}

// previewCompositionBindings 只为manifest要求生成候选,保留仍合法的明确选择。
// 唯一候选可以预选;多个或没有候选时保持未完成,不按顺序猜测目标。
func previewCompositionBindings(catalog compositionCatalog) (CompositionPreviewResponse, error) {
	out := CompositionPreviewResponse{
		Infra:    append([]contracts.CompositionComponentRef{}, catalog.spec.Infra...),
		Bindings: []CompositionBindingPreview{}, Links: []contracts.CompositionLink{}, Complete: true,
	}
	runtimeCapabilities := runtimeCapabilitiesByInstance(catalog.runtimePlans)
	declaredBindings := map[string]map[string]struct{}{}
	knownTargets := map[string]struct{}{}
	for _, runtime := range catalog.runtimePlans {
		knownTargets[runtime.InstanceCode] = struct{}{}
	}
	for _, ref := range catalog.selected {
		knownTargets[ref.Code] = struct{}{}
	}
	// 运行时自身的显式连接不属于工具绑定,仍交给同一编译器验证并保留。
	for _, link := range catalog.spec.Links {
		if _, runtimeSource := catalog.runtimeAdapters[link.SourceComponent]; runtimeSource {
			if _, targetExists := knownTargets[link.TargetComponent]; targetExists {
				out.Links = append(out.Links, link)
			}
		}
	}
	for _, ref := range catalog.selected {
		tool := catalog.tools[ref.Code]
		declaredBindings[ref.Code] = map[string]struct{}{}
		for _, binding := range tool.ResourceSpec.Bindings {
			if _, duplicate := declaredBindings[ref.Code][binding.ConfigBinding]; duplicate {
				return CompositionPreviewResponse{}, fmt.Errorf("组件 %s 的绑定 %s 重复声明", ref.Code, binding.Name)
			}
			declaredBindings[ref.Code][binding.ConfigBinding] = struct{}{}
			if _, err := parseCompositionBinding(binding.ConfigBinding); err != nil {
				return CompositionPreviewResponse{}, err
			}
			sourceEndpoint := strings.TrimSpace(binding.SourceEndpoint)
			if sourceEndpoint == "" {
				sourceEndpoint = "component"
			}
			if _, err := resolveCompositionEndpoint(ref.Code, sourceEndpoint, true, binding.SourceComponent, catalog.runtimeAdapters, catalog.tools); err != nil {
				return CompositionPreviewResponse{}, err
			}
			item := CompositionBindingPreview{SourceComponent: ref.Code, SourceName: tool.Name, ConfigBinding: binding.ConfigBinding, Options: []CompositionBindingOption{}}
			// 候选仅来自当前命名实例和依赖闭包,不暴露目录之外的服务。
			for _, runtime := range catalog.runtimePlans {
				if err := appendCompositionBindingOption(&item, binding, sourceEndpoint, runtime.InstanceCode, runtime.Runtime.Name+" · "+runtime.InstanceCode, runtimeCapabilities, catalog); err != nil {
					return CompositionPreviewResponse{}, err
				}
			}
			for _, target := range catalog.selected {
				if target.Code == ref.Code {
					continue
				}
				if err := appendCompositionBindingOption(&item, binding, sourceEndpoint, target.Code, catalog.tools[target.Code].Name, runtimeCapabilities, catalog); err != nil {
					return CompositionPreviewResponse{}, err
				}
			}
			var chosen *contracts.CompositionLink
			matches := 0
			for _, existing := range catalog.spec.Links {
				if existing.SourceComponent != ref.Code || existing.ConfigBinding != binding.ConfigBinding {
					continue
				}
				matches++
				if matches > 1 {
					return CompositionPreviewResponse{}, fmt.Errorf("组件 %s 的连接 %s 重复", ref.Code, binding.Name)
				}
				for _, option := range item.Options {
					if existing == option.Link {
						link := option.Link
						chosen = &link
						break
					}
				}
			}
			if chosen == nil && len(item.Options) == 1 {
				link := item.Options[0].Link
				chosen = &link
			}
			if chosen == nil {
				out.Complete = false
			} else {
				out.Links = append(out.Links, *chosen)
			}
			out.Bindings = append(out.Bindings, item)
		}
	}
	// 已移出依赖闭包的连接随选择失效;仍选中的组件不得夹带未声明绑定。
	for _, link := range catalog.spec.Links {
		if bindings, selected := declaredBindings[link.SourceComponent]; selected {
			if _, declared := bindings[link.ConfigBinding]; !declared {
				return CompositionPreviewResponse{}, fmt.Errorf("组件 %s 的连接绑定未声明", link.SourceComponent)
			}
		}
	}
	if out.Complete {
		spec := catalog.spec
		spec.Links = out.Links
		if err := validateCompositionLinks(spec, catalog.selected, catalog.tools); err != nil {
			return CompositionPreviewResponse{}, err
		}
		if err := validateCompositionBindings(spec, runtimeCapabilities, catalog.selected, catalog.tools); err != nil {
			return CompositionPreviewResponse{}, err
		}
		if _, _, err := compileCompositionLinks(spec, catalog.runtimeAdapters, catalog.runtimePlans, catalog.selected, catalog.tools); err != nil {
			return CompositionPreviewResponse{}, err
		}
	}
	return out, nil
}

// appendCompositionBindingOption 使用编译器的能力、端点及协议校验,不另建配对规则。
func appendCompositionBindingOption(item *CompositionBindingPreview, binding ComponentBinding, sourceEndpoint, target, label string, runtimeCapabilities map[string]CapabilityConstraints, catalog compositionCatalog) error {
	if !compositionTargetProvides(target, binding.Capability, runtimeCapabilities, catalog.tools) {
		return nil
	}
	endpoint, err := resolveCompositionEndpoint(target, binding.Endpoint, false, "", catalog.runtimeAdapters, catalog.tools)
	if errors.Is(err, errCompositionEndpointNotFound) {
		// 同名能力不保证提供绑定要求的端点;该目标不适用,继续检查其它候选。
		return nil
	}
	if err != nil {
		return err
	}
	if err := validateCompositionProtocol(binding.Protocol, endpoint.Protocol); err != nil {
		if errors.Is(err, errCompositionProtocolMismatch) {
			return nil
		}
		return err
	}
	item.Options = append(item.Options, CompositionBindingOption{Label: label, Link: contracts.CompositionLink{
		SourceComponent: item.SourceComponent, SourceEndpoint: sourceEndpoint,
		TargetComponent: target, TargetEndpoint: binding.Endpoint, Protocol: binding.Protocol,
		RequiredAtStart: binding.RequiredAtStart, AccessScope: "sandbox", ConfigBinding: binding.ConfigBinding,
	}})
	return nil
}
