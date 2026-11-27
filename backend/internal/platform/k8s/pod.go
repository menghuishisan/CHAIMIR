package k8s

import (
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

// SamePodExecutionSpec 在副本上核对完整执行规格，仅忽略等价默认值和服务器分配字段。
func SamePodExecutionSpec(current, desired corev1.PodSpec) bool {
	current = NormalizePodExecutionSpec(current)
	desired = NormalizePodExecutionSpec(desired)
	return apiequality.Semantic.DeepEqual(current, desired)
}

// NormalizePodExecutionSpec 去除 Kubernetes 在持久化 Pod 时补齐的等价默认值,
// 保留镜像、安全上下文、资源、挂载和探针等属于执行契约的字段。
func NormalizePodExecutionSpec(spec corev1.PodSpec) corev1.PodSpec {
	spec = *spec.DeepCopy()
	// 节点由调度器绑定，开销由已指定的 RuntimeClass 注入。
	spec.NodeName, spec.DeprecatedServiceAccount, spec.Overhead = "", "", nil
	if spec.DNSPolicy == "" {
		spec.DNSPolicy = corev1.DNSClusterFirst
	}
	if spec.SchedulerName == "" {
		spec.SchedulerName = corev1.DefaultSchedulerName
	}
	if spec.TerminationGracePeriodSeconds == nil {
		seconds := int64(corev1.DefaultTerminationGracePeriodSeconds)
		spec.TerminationGracePeriodSeconds = &seconds
	}
	if spec.Priority == nil {
		priority := int32(0)
		spec.Priority = &priority
	}
	if spec.PreemptionPolicy == nil {
		policy := corev1.PreemptLowerPriority
		spec.PreemptionPolicy = &policy
	}
	for index := range spec.InitContainers {
		normalizePodContainer(&spec.InitContainers[index])
	}
	for index := range spec.Containers {
		normalizePodContainer(&spec.Containers[index])
	}
	spec.Tolerations = normalizePodTolerations(spec.Tolerations)
	return spec
}

// normalizePodTolerations 去除 Kubernetes 自动补齐的默认节点不可用容忍,
// 保留平台显式声明的其他调度约束,避免二阶段幂等核对把同一 Pod 误判为规格漂移。
func normalizePodTolerations(items []corev1.Toleration) []corev1.Toleration {
	if len(items) == 0 {
		return nil
	}
	out := make([]corev1.Toleration, 0, len(items))
	for _, item := range items {
		if isDefaultPodToleration(item) {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// isDefaultPodToleration 识别 API Server 为未配置 Pod 自动加入的两条默认容忍。
func isDefaultPodToleration(item corev1.Toleration) bool {
	if item.Operator != corev1.TolerationOpExists || item.Effect != corev1.TaintEffectNoExecute || item.Value != "" || item.TolerationSeconds == nil || *item.TolerationSeconds != 300 {
		return false
	}
	return item.Key == "node.kubernetes.io/not-ready" || item.Key == "node.kubernetes.io/unreachable"
}

// normalizePodContainer 补齐等价资源和探针默认值，去除等价终止消息与挂载默认值。
func normalizePodContainer(container *corev1.Container) {
	for key, limit := range container.Resources.Limits {
		if _, exists := container.Resources.Requests[key]; !exists {
			if container.Resources.Requests == nil {
				container.Resources.Requests = make(corev1.ResourceList)
			}
			container.Resources.Requests[key] = limit.DeepCopy()
		}
	}
	for _, probe := range []*corev1.Probe{container.StartupProbe, container.ReadinessProbe, container.LivenessProbe} {
		if probe == nil {
			continue
		}
		if probe.TimeoutSeconds == 0 {
			probe.TimeoutSeconds = 1
		}
		if probe.PeriodSeconds == 0 {
			probe.PeriodSeconds = 10
		}
		if probe.SuccessThreshold == 0 {
			probe.SuccessThreshold = 1
		}
		if probe.FailureThreshold == 0 {
			probe.FailureThreshold = 3
		}
		if probe.HTTPGet != nil {
			if probe.HTTPGet.Path == "" {
				probe.HTTPGet.Path = "/"
			}
			if probe.HTTPGet.Scheme == "" {
				probe.HTTPGet.Scheme = corev1.URISchemeHTTP
			}
		}
	}
	if container.TerminationMessagePath == corev1.TerminationMessagePathDefault {
		container.TerminationMessagePath = ""
	}
	if container.TerminationMessagePolicy == corev1.TerminationMessageReadFile {
		container.TerminationMessagePolicy = ""
	}
	for index := range container.VolumeMounts {
		mount := &container.VolumeMounts[index]
		if mount.RecursiveReadOnly != nil && *mount.RecursiveReadOnly == corev1.RecursiveReadOnlyDisabled {
			mount.RecursiveReadOnly = nil
		}
		if mount.MountPropagation != nil && *mount.MountPropagation == corev1.MountPropagationNone {
			mount.MountPropagation = nil
		}
	}
}
