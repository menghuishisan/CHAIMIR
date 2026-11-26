package sandbox

import (
	"fmt"
	"slices"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/workload"
	corev1 "k8s.io/api/core/v1"
)

type planVolumeDomain struct {
	Owner string
	Spec  VolumeDomainSpec
}

func volumeClaimName(owner, domain string) string {
	if domain == VolumeDomainWorkspace {
		return domain
	}
	return boundedSandboxName(owner+"-"+domain, 63)
}

func (d planVolumeDomain) claimName() string { return volumeClaimName(d.Owner, d.Spec.Name) }

func toolVolumeSource(plan CreateSandboxPlan, tool Tool) (string, AdapterSpec, error) {
	source := tool.ResourceSpec.VolumeSource
	if source == "self" {
		return "tool-" + tool.Code, toolStateAdapter(), nil
	}
	if instance, ok := strings.CutPrefix(source, "runtime:"); ok {
		if runtime, exists := plan.RuntimeInstance(instance); exists {
			return "runtime-" + runtime.InstanceCode, runtime.Runtime.AdapterSpec, nil
		}
	}
	return "", AdapterSpec{}, fmt.Errorf("组件 %s 未声明有效的卷来源", tool.Code)
}

func toolStateAdapter() AdapterSpec {
	return AdapterSpec{VolumeDomains: []VolumeDomainSpec{{
		Name: VolumeDomainRuntimeState, MountPath: "/runtime-state", StudentAccess: VolumeAccessNone,
		Persistence: VolumePersistenceEphemeral, SnapshotScope: VolumeSnapshotEnabled,
	}}}
}

func volumeDomainsForPlan(plan CreateSandboxPlan) []corev1.Volume {
	persistent := map[string]bool{}
	for _, domain := range persistentVolumeDomains(plan) {
		persistent[domain.claimName()] = true
	}
	var volumes []corev1.Volume
	for _, domain := range allVolumeDomainsForPlan(plan) {
		name := domain.claimName()
		volume := corev1.Volume{Name: name}
		if persistent[name] {
			volume.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{ClaimName: name}
		} else {
			volume.EmptyDir = &corev1.EmptyDirVolumeSource{}
		}
		volumes = append(volumes, volume)
	}
	return volumes
}

func allVolumeDomainsForPlan(plan CreateSandboxPlan) []planVolumeDomain {
	var domains []planVolumeDomain
	seen := map[string]bool{}
	appendDomains := func(owner string, spec AdapterSpec) {
		for _, domain := range spec.VolumeDomains {
			item := planVolumeDomain{Owner: owner, Spec: domain}
			if !seen[item.claimName()] {
				seen[item.claimName()] = true
				domains = append(domains, item)
			}
		}
	}
	for _, runtime := range plan.Runtimes {
		appendDomains("runtime-"+runtime.InstanceCode, runtime.Runtime.AdapterSpec)
	}
	for _, tool := range plan.Tools {
		if tool.ResourceSpec.VolumeSource == "self" {
			appendDomains("tool-"+tool.Code, toolStateAdapter())
		}
	}
	return domains
}

func persistentVolumeDomains(plan CreateSandboxPlan) []planVolumeDomain {
	shared := sharedVolumeDomainsForPlan(plan)
	for _, runtime := range plan.Runtimes {
		for _, asset := range runtime.Runtime.AdapterSpec.InitAssets {
			shared[volumeClaimName("runtime-"+runtime.InstanceCode, asset.TargetDomain)] = true
		}
	}
	var domains []planVolumeDomain
	for _, domain := range allVolumeDomainsForPlan(plan) {
		spec := domain.Spec
		snapshot := plan.Sandbox.SnapshotEnabled && spec.Name != VolumeDomainJudgePrivate && (spec.SnapshotScope == VolumeSnapshotAlways || spec.SnapshotScope == VolumeSnapshotEnabled)
		if shared[domain.claimName()] || snapshot || spec.Name == VolumeDomainWorkspace || spec.Persistence == VolumePersistenceObjectStorageCode || spec.Persistence == VolumePersistenceSnapshot {
			domains = append(domains, domain)
		}
	}
	return domains
}

func sharedVolumeDomainsForPlan(plan CreateSandboxPlan) map[string]bool {
	shared := map[string]bool{}
	previousPod := map[string]string{}
	add := func(owner, domain, podName string) {
		name := volumeClaimName(owner, domain)
		if previous, exists := previousPod[name]; exists && previous != podName {
			shared[name] = true
		}
		previousPod[name] = podName
	}
	for _, pod := range podGroupForPlan(plan) {
		runtime, ok := runtimePlanForPod(plan, pod.Name)
		if !ok {
			continue
		}
		for _, component := range slices.Concat(pod.InitContainers, pod.Containers) {
			for _, mount := range volumeMountsForContainer(runtime.Runtime.AdapterSpec, component) {
				if _, declared := volumeDomainByName(runtime.Runtime.AdapterSpec, mount.Name); declared {
					add("runtime-"+runtime.InstanceCode, mount.Name, pod.Name)
				}
			}
		}
	}
	for _, tool := range plan.Tools {
		owner := "runtime-" + strings.TrimPrefix(tool.ResourceSpec.VolumeSource, "runtime:")
		if tool.ResourceSpec.VolumeSource == "self" {
			owner = "tool-" + tool.Code
		}
		for _, component := range tool.ResourceSpec.Components {
			for _, raw := range component.MountDomains {
				name := strings.TrimSpace(raw)
				add(owner, name, toolComponentPodName(tool.Code, component.Name))
			}
		}
	}
	return shared
}

func snapshotDomainsForPlan(plan CreateSandboxPlan) []string {
	var domains []string
	for _, domain := range persistentVolumeDomains(plan) {
		if domain.Spec.Name == VolumeDomainJudgePrivate || domain.Spec.SnapshotScope == VolumeSnapshotNever {
			continue
		}
		if domain.Spec.SnapshotScope == VolumeSnapshotAlways || plan.Sandbox.SnapshotEnabled && domain.Spec.SnapshotScope == VolumeSnapshotEnabled {
			domains = append(domains, domain.claimName())
		}
	}
	return domains
}

func hasDomainMounts(components []workload.ComponentSpec) bool {
	return slices.ContainsFunc(components, func(component workload.ComponentSpec) bool { return len(component.MountDomains) > 0 })
}

func compileToolVolumeSource(spec contracts.SandboxCompositionSpec, tool *Tool, runtimes map[string]AdapterSpec) error {
	if err := validateToolVolumeSource(tool.ResourceSpec); err != nil {
		return fmt.Errorf("组件 %s 卷来源无效: %w", tool.Code, err)
	}
	source := strings.TrimSpace(tool.ResourceSpec.VolumeSource)
	tool.ResourceSpec.VolumeSource = source
	if !hasDomainMounts(tool.ResourceSpec.Components) {
		return nil
	}
	if source == "self" {
		return nil
	}
	if source == "workspace" {
		if _, exists := runtimes[spec.WorkspaceRuntimeInstance]; !exists {
			return fmt.Errorf("组件 %s 的公开材料缺少工作区运行时", tool.Code)
		}
		tool.ResourceSpec.VolumeSource = "runtime:" + spec.WorkspaceRuntimeInstance
		return nil
	}
	bindingName, ok := strings.CutPrefix(source, "binding:")
	if !ok || bindingName == "" {
		return fmt.Errorf("组件 %s 必须声明有效的卷来源", tool.Code)
	}
	for _, binding := range tool.ResourceSpec.Bindings {
		if binding.Name != bindingName {
			continue
		}
		for _, link := range spec.Links {
			if link.SourceComponent == tool.Code && link.ConfigBinding == binding.ConfigBinding {
				if _, exists := runtimes[link.TargetComponent]; !exists {
					return fmt.Errorf("组件 %s 的卷来源必须连接具体运行时", tool.Code)
				}
				tool.ResourceSpec.VolumeSource = "runtime:" + link.TargetComponent
				return nil
			}
		}
	}
	return fmt.Errorf("组件 %s 的卷来源绑定 %s 未连接", tool.Code, bindingName)
}

func validateToolVolumeSource(spec ToolResourceSpec) error {
	source := strings.TrimSpace(spec.VolumeSource)
	if !hasDomainMounts(spec.Components) {
		if source != "" {
			return fmt.Errorf("未挂载安全域的组件不应声明卷来源")
		}
		return nil
	}
	if source == "self" {
		return nil
	}
	if source == "workspace" {
		for _, component := range spec.Components {
			if len(component.WritableMountDomains) > 0 || slices.ContainsFunc(component.MountDomains, func(domain string) bool {
				return strings.TrimSpace(domain) != VolumeDomainPublicAssets
			}) {
				return fmt.Errorf("工作区来源只允许读取公开材料")
			}
		}
		return nil
	}
	if bindingName, ok := strings.CutPrefix(source, "binding:"); ok && bindingName != "" {
		for _, binding := range spec.Bindings {
			if binding.Name == bindingName {
				if binding.Multiple || binding.Optional {
					return fmt.Errorf("多目标或可选连接不能作为卷来源")
				}
				return nil
			}
		}
	}
	return fmt.Errorf("卷来源必须是 self、workspace 或已声明的具名连接")
}
