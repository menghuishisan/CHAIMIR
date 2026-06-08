// M2 K8s 编排实现:创建/暂停/恢复/回收每沙箱独占 Namespace 与工作负载。
// 编排职责只处理数据面原语(Pod/Service/NetworkPolicy/Quota/exec),业务状态机由 service 层负责。
package sandbox

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/ids"
	platformk8s "chaimir/internal/platform/k8s"
	"chaimir/internal/platform/timex"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	sandboxWorkloadName = "sandbox-runtime"
	sandboxServiceName  = "sandbox-runtime"
	workspaceVolumeName = "workspace"
	prepullContainer    = "prepull"
	snapshotGroup       = "snapshot.storage.k8s.io"
)

// k8sOrchestrator 使用 platform/k8s 客户端执行沙箱数据面编排。
type k8sOrchestrator struct {
	client *platformk8s.Client
	cfg    config.SandboxConfig
}

// NewK8sOrchestrator 构造 K8s 编排器。
func NewK8sOrchestrator(client *platformk8s.Client, cfg config.SandboxConfig) Orchestrator {
	return &k8sOrchestrator{client: client, cfg: cfg}
}

// PrepullImage 创建或更新镜像预拉取 DaemonSet,并以 DaemonSet 状态作为成功判定。
func (o *k8sOrchestrator) PrepullImage(ctx context.Context, spec ImagePrepullSpec) (ImagePrepullStatus, error) {
	if err := validateK8sSandboxConfig(o.cfg); err != nil {
		return ImagePrepullStatus{}, err
	}
	cs := o.client.Clientset()
	namespace := prepullNamespaceName(o.cfg)
	if _, err := cs.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{}); apierrors.IsNotFound(err) {
		if _, createErr := cs.CoreV1().Namespaces().Create(ctx, prepullNamespaceManifest(o.cfg), metav1.CreateOptions{}); createErr != nil {
			return ImagePrepullStatus{}, fmt.Errorf("创建镜像预拉取 Namespace 失败: %w", createErr)
		}
	} else if err != nil {
		return ImagePrepullStatus{}, fmt.Errorf("查询镜像预拉取 Namespace 失败: %w", err)
	}

	ds := prepullDaemonSetManifest(spec, o.cfg)
	current, err := cs.AppsV1().DaemonSets(namespace).Get(ctx, ds.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, createErr := cs.AppsV1().DaemonSets(namespace).Create(ctx, ds, metav1.CreateOptions{}); createErr != nil {
			return ImagePrepullStatus{}, fmt.Errorf("创建镜像预拉取 DaemonSet 失败: %w", createErr)
		}
	} else if err != nil {
		return ImagePrepullStatus{}, fmt.Errorf("查询镜像预拉取 DaemonSet 失败: %w", err)
	} else {
		// 保留资源版本后更新 spec,用于镜像版本重复触发时刷新节点状态。
		ds.ResourceVersion = current.ResourceVersion
		if _, updateErr := cs.AppsV1().DaemonSets(namespace).Update(ctx, ds, metav1.UpdateOptions{}); updateErr != nil {
			return ImagePrepullStatus{}, fmt.Errorf("更新镜像预拉取 DaemonSet 失败: %w", updateErr)
		}
	}

	status := ImagePrepullStatus{DaemonSet: ds.Name}
	if err := wait.PollUntilContextTimeout(ctx, time.Duration(o.cfg.PrepullPollIntervalSeconds)*time.Second, time.Duration(o.cfg.PrepullTimeoutSeconds)*time.Second, true, func(ctx context.Context) (bool, error) {
		latest, err := cs.AppsV1().DaemonSets(namespace).Get(ctx, ds.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("查询镜像预拉取 DaemonSet 失败: %w", err)
		}
		status.DesiredNodes = latest.Status.DesiredNumberScheduled
		status.ReadyNodes = latest.Status.NumberReady
		status.FailedNodes = latest.Status.NumberUnavailable
		if failure, eventErr := o.prepullFailureFromEvents(ctx, ds); eventErr != nil {
			return false, eventErr
		} else if failure != "" {
			status.Failure = failure
			return false, fmt.Errorf("镜像预拉取出现失败事件: %s", failure)
		}
		status.Completed = status.DesiredNodes > 0 &&
			latest.Status.DesiredNumberScheduled == latest.Status.NumberReady &&
			latest.Status.UpdatedNumberScheduled == latest.Status.DesiredNumberScheduled
		return status.Completed, nil
	}); err != nil {
		if status.Failure == "" {
			status.Failure = err.Error()
		}
		return status, fmt.Errorf("镜像预拉取未完成: %w", err)
	}
	return status, nil
}

// Create 创建沙箱完整数据面:Namespace、网络策略、资源限制、运行时 Pod/Service。
func (o *k8sOrchestrator) Create(ctx context.Context, spec SandboxCreateSpec) error {
	if err := validateK8sSandboxConfig(o.cfg); err != nil {
		return err
	}
	cs := o.client.Clientset()
	if _, err := cs.CoreV1().Namespaces().Create(ctx, namespaceManifest(spec), metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建沙箱 Namespace 失败: %w", err)
	}

	// 先落默认 deny-all,再创建工作负载;即使后续 Pod 创建失败,沙箱也不会暴露开放网络窗口。
	if _, err := cs.NetworkingV1().NetworkPolicies(spec.Namespace).Create(ctx, denyAllNetworkPolicy(spec.Namespace), metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建沙箱默认网络策略失败: %w", err)
	}
	if _, err := cs.NetworkingV1().NetworkPolicies(spec.Namespace).Create(ctx, controlPlaneNetworkPolicy(spec.Namespace, o.cfg, sandboxControlPlanePorts(spec)), metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建沙箱控制面网络策略失败: %w", err)
	}
	if _, err := cs.CoreV1().ResourceQuotas(spec.Namespace).Create(ctx, namespaceQuota(o.cfg), metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建沙箱资源配额失败: %w", err)
	}
	if _, err := cs.CoreV1().LimitRanges(spec.Namespace).Create(ctx, namespaceLimitRange(o.cfg), metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建沙箱资源限制失败: %w", err)
	}
	if _, err := cs.CoreV1().PersistentVolumeClaims(spec.Namespace).Create(ctx, workspacePVCManifest(o.cfg), metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建沙箱工作区 PVC 失败: %w", err)
	}

	workload := runtimeWorkloadManifest(spec, o.cfg)
	if _, err := cs.AppsV1().Deployments(spec.Namespace).Create(ctx, workload, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建沙箱 Deployment 失败: %w", err)
	}
	svc := runtimeServiceManifest(spec)
	if svc != nil {
		if _, err := cs.CoreV1().Services(spec.Namespace).Create(ctx, svc, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("创建沙箱 Service 失败: %w", err)
		}
	}
	return nil
}

// WaitReady 等待沙箱 Deployment 至少一个副本 Ready。
func (o *k8sOrchestrator) WaitReady(ctx context.Context, namespace string) error {
	return wait.PollUntilContextTimeout(ctx, time.Duration(o.cfg.ReadyPollIntervalSeconds)*time.Second, time.Duration(o.cfg.ReadyTimeoutSeconds)*time.Second, true, func(ctx context.Context) (bool, error) {
		deploy, err := o.client.Clientset().AppsV1().Deployments(namespace).Get(ctx, sandboxWorkloadName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("查询沙箱 Deployment 就绪状态失败: %w", err)
		}
		return deploy.Status.ReadyReplicas > 0, nil
	})
}

// SnapshotAvailable 通过 API discovery 确认集群存在 VolumeSnapshot CRD,避免创建后才失败。
func (o *k8sOrchestrator) SnapshotAvailable(ctx context.Context) error {
	resources, err := o.client.Clientset().Discovery().ServerResourcesForGroupVersion(snapshotGroup + "/v1")
	if err != nil {
		return fmt.Errorf("查询 VolumeSnapshot 能力失败: %w", err)
	}
	for _, resource := range resources.APIResources {
		if resource.Name == "volumesnapshots" {
			return nil
		}
	}
	return fmt.Errorf("集群未注册 VolumeSnapshot 资源")
}

// SnapshotWorkspace 创建 CSI VolumeSnapshot;集群未安装 CRD 时会显式失败。
func (o *k8sOrchestrator) SnapshotWorkspace(ctx context.Context, spec SnapshotSpec) (SnapshotResult, error) {
	now := timex.Now()
	name := "snapshot-" + ids.Format(spec.SandboxID) + "-" + strconvTime(now)
	gvr := schema.GroupVersionResource{Group: snapshotGroup, Version: "v1", Resource: "volumesnapshots"}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": snapshotGroup + "/v1",
		"kind":       "VolumeSnapshot",
		"metadata": map[string]any{
			"name":      name,
			"namespace": spec.Namespace,
			"labels": map[string]any{
				"app.kubernetes.io/name": "chaimir-sandbox",
				"chaimir.io/sandbox-id":  ids.Format(spec.SandboxID),
				"chaimir.io/tenant-id":   ids.Format(spec.TenantID),
			},
		},
		"spec": map[string]any{
			"source": map[string]any{
				"persistentVolumeClaimName": workspaceVolumeName,
			},
		},
	}}
	if _, err := o.client.Dynamic().Resource(gvr).Namespace(spec.Namespace).Create(ctx, obj, metav1.CreateOptions{}); err != nil {
		return SnapshotResult{}, fmt.Errorf("创建沙箱 VolumeSnapshot 失败: %w", err)
	}
	return SnapshotResult{Ref: spec.Namespace + "/" + name, CreatedAt: now, ExpiresAt: spec.ExpiresAt}, nil
}

// Pause 把 Deployment 副本缩为 0,保留 PVC/Namespace。
func (o *k8sOrchestrator) Pause(ctx context.Context, binding SandboxRuntimeBinding) error {
	replicas := int32(0)
	if _, err := o.client.Clientset().AppsV1().Deployments(binding.Namespace).UpdateScale(ctx, sandboxWorkloadName, &autoscalingv1.Scale{
		ObjectMeta: metav1.ObjectMeta{Name: sandboxWorkloadName, Namespace: binding.Namespace},
		Spec:       autoscalingv1.ScaleSpec{Replicas: replicas},
	}, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("暂停沙箱 Deployment 失败: %w", err)
	}
	return nil
}

// Resume 把已暂停的 Deployment 副本恢复为 1;不存在时重建完整工作负载。
func (o *k8sOrchestrator) Resume(ctx context.Context, spec SandboxCreateSpec) error {
	replicas := int32(1)
	if _, err := o.client.Clientset().AppsV1().Deployments(spec.Namespace).Get(ctx, sandboxWorkloadName, metav1.GetOptions{}); apierrors.IsNotFound(err) {
		return o.Create(ctx, spec)
	} else if err != nil {
		return fmt.Errorf("读取沙箱 Deployment 失败: %w", err)
	}
	if _, err := o.client.Clientset().AppsV1().Deployments(spec.Namespace).UpdateScale(ctx, sandboxWorkloadName, &autoscalingv1.Scale{
		ObjectMeta: metav1.ObjectMeta{Name: sandboxWorkloadName, Namespace: spec.Namespace},
		Spec:       autoscalingv1.ScaleSpec{Replicas: replicas},
	}, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("恢复沙箱 Deployment 失败: %w", err)
	}
	return nil
}

// Recycle 删除沙箱 Namespace,K8s 会级联清理其内资源。
func (o *k8sOrchestrator) Recycle(ctx context.Context, namespace string) error {
	if err := o.client.Clientset().CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("删除沙箱 Namespace 失败: %w", err)
	}
	return nil
}

// prepullFailureFromEvents 查找预拉取 Pod/DaemonSet 的失败事件,用于阻止失败镜像被标记为已完成。
func (o *k8sOrchestrator) prepullFailureFromEvents(ctx context.Context, ds *appsv1.DaemonSet) (string, error) {
	selector := labels.SelectorFromSet(ds.Spec.Selector.MatchLabels).String()
	namespace := prepullNamespaceName(o.cfg)
	pods, err := o.client.Clientset().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return "", fmt.Errorf("查询镜像预拉取 Pod 失败: %w", err)
	}
	targets := map[string]struct{}{ds.Name: {}}
	for _, pod := range pods.Items {
		targets[pod.Name] = struct{}{}
	}
	events, err := o.client.Clientset().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("查询镜像预拉取事件失败: %w", err)
	}
	for _, event := range events.Items {
		if event.InvolvedObject.Kind != "Pod" && event.InvolvedObject.Kind != "DaemonSet" {
			continue
		}
		if _, ok := targets[event.InvolvedObject.Name]; !ok {
			continue
		}
		if prepullEventFailed(event) {
			return event.Reason + ": " + event.Message, nil
		}
	}
	return "", nil
}

// prepullEventFailed 识别镜像拉取、签名策略和调度失败等会破坏预拉取闭环的事件。
func prepullEventFailed(event corev1.Event) bool {
	reason := strings.ToLower(event.Reason)
	message := strings.ToLower(event.Message)
	if strings.Contains(reason, "imagepullbackoff") || strings.Contains(reason, "errimagepull") ||
		strings.Contains(reason, "failedscheduling") || strings.Contains(reason, "failedcreate") {
		return true
	}
	if reason == "failed" {
		return strings.Contains(message, "pull image") || strings.Contains(message, "imagepull") ||
			strings.Contains(message, "signature") || strings.Contains(message, "cosign") ||
			strings.Contains(message, "schedule")
	}
	return strings.Contains(message, "imagepullbackoff") || strings.Contains(message, "errimagepull")
}

// RuntimeBinding 返回沙箱运行时主容器定位。
func (o *k8sOrchestrator) RuntimeBinding(ctx context.Context, namespace string) (SandboxRuntimeBinding, error) {
	pod, err := o.runtimePod(ctx, namespace)
	if err != nil {
		return SandboxRuntimeBinding{}, err
	}
	var svc *corev1.Service
	current, err := o.client.Clientset().CoreV1().Services(namespace).Get(ctx, sandboxServiceName, metav1.GetOptions{})
	if err == nil {
		svc = current
	} else if !apierrors.IsNotFound(err) {
		return SandboxRuntimeBinding{}, fmt.Errorf("读取沙箱 Service 失败: %w", err)
	}
	return runtimeBindingFromPodAndService(namespace, pod, svc), nil
}

// ToolEndpoint 返回工具 sidecar 的 Service 暴露目标。
func (o *k8sOrchestrator) ToolEndpoint(ctx context.Context, namespace, toolCode string) (SandboxToolEndpoint, error) {
	svc, err := o.client.Clientset().CoreV1().Services(namespace).Get(ctx, sandboxServiceName, metav1.GetOptions{})
	if err != nil {
		return SandboxToolEndpoint{}, fmt.Errorf("读取沙箱 Service 失败: %w", err)
	}
	for _, port := range svc.Spec.Ports {
		if port.Name == toolCode {
			return SandboxToolEndpoint{
				ToolCode:    toolCode,
				ServiceName: svc.Name,
				ServicePort: port.Port,
			}, nil
		}
	}
	return SandboxToolEndpoint{}, fmt.Errorf("工具 %s 未暴露 Service 端口", toolCode)
}

// Exec 在运行时或工具容器内执行命令。
func (o *k8sOrchestrator) Exec(
	ctx context.Context,
	binding SandboxRuntimeBinding,
	command []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
	tty bool,
) error {
	return o.client.Exec(ctx, binding.Namespace, binding.PodName, binding.Container, command, stdin, stdout, stderr, tty)
}

// runtimePod 查询沙箱当前运行的主 Pod。
func (o *k8sOrchestrator) runtimePod(ctx context.Context, namespace string) (*corev1.Pod, error) {
	pods, err := o.client.Clientset().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=chaimir-sandbox-runtime",
	})
	if err != nil {
		return nil, fmt.Errorf("查询沙箱 Pod 失败: %w", err)
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
			return &pod, nil
		}
	}
	return nil, fmt.Errorf("沙箱 Pod 不存在")
}

// namespaceManifest 生成沙箱 Namespace,标签用于 NetworkPolicy/RBAC/审计定位。
func namespaceManifest(spec SandboxCreateSpec) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: spec.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":             "chaimir-sandbox",
				"chaimir.io/sandbox-id":              ids.Format(spec.SandboxID),
				"chaimir.io/tenant-id":               ids.Format(spec.TenantID),
				"pod-security.kubernetes.io/enforce": "restricted",
			},
		},
	}
}

// denyAllNetworkPolicy 默认拒绝所有出入站流量,后续按工具/链能力精确放行。
func denyAllNetworkPolicy(namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "default-deny-all", Namespace: namespace},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
		},
	}
}

// controlPlaneNetworkPolicy 仅允许配置指定的后端控制面 Pod 访问沙箱声明端口。
func controlPlaneNetworkPolicy(namespace string, cfg config.SandboxConfig, ports []networkingv1.NetworkPolicyPort) *networkingv1.NetworkPolicy {
	var ingress []networkingv1.NetworkPolicyIngressRule
	if len(ports) > 0 {
		ingress = []networkingv1.NetworkPolicyIngressRule{{
			From: []networkingv1.NetworkPolicyPeer{{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"kubernetes.io/metadata.name": cfg.ControlNamespace},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{cfg.ControlPodLabelKey: cfg.ControlPodLabelValue},
				},
			}},
			Ports: ports,
		}}
	}
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "allow-control-plane", Namespace: namespace},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app.kubernetes.io/name": "chaimir-sandbox-runtime"},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress:     ingress,
		},
	}
}

// sandboxControlPlanePorts 收集控制面代理需要访问的 runtime、infra sidecar 与 web 工具端口。
func sandboxControlPlanePorts(spec SandboxCreateSpec) []networkingv1.NetworkPolicyPort {
	seen := map[string]struct{}{}
	var ports []networkingv1.NetworkPolicyPort
	add := func(port PortSpec) {
		if port.ContainerPort <= 0 {
			return
		}
		protocol := corev1.ProtocolTCP
		if strings.EqualFold(port.Protocol, "UDP") {
			protocol = corev1.ProtocolUDP
		}
		key := string(protocol) + ":" + fmt.Sprint(port.ContainerPort)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		target := intstr.FromInt32(port.ContainerPort)
		ports = append(ports, networkingv1.NetworkPolicyPort{Protocol: &protocol, Port: &target})
	}
	for _, port := range spec.Runtime.AdapterSpec.RuntimeContainer.Ports {
		add(port)
	}
	for _, sidecar := range spec.Runtime.AdapterSpec.InfraSidecars {
		for _, port := range sidecar.Ports {
			add(port)
		}
	}
	for _, tool := range spec.Tools {
		if tool.Kind != ToolKindWebEmbed || tool.Port <= 0 {
			continue
		}
		add(PortSpec{ContainerPort: tool.Port, Protocol: "TCP"})
	}
	return ports
}

// namespaceQuota 设置单沙箱资源硬限,防止资源炸弹影响邻居。
func namespaceQuota(cfg config.SandboxConfig) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-quota"},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceLimitsCPU:    checkedQuantity(cfg.MaxCPU),
				corev1.ResourceLimitsMemory: checkedQuantity(cfg.MaxMemory),
				corev1.ResourcePods:         checkedQuantity(cfg.MaxPods),
			},
		},
	}
}

// namespaceLimitRange 为容器设置默认 request/limit,避免未声明资源的容器逃逸配额控制。
func namespaceLimitRange(cfg config.SandboxConfig) *corev1.LimitRange {
	return &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-limits"},
		Spec: corev1.LimitRangeSpec{Limits: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Default: corev1.ResourceList{
				corev1.ResourceCPU:    checkedQuantity(cfg.DefaultCPU),
				corev1.ResourceMemory: checkedQuantity(cfg.DefaultMemory),
			},
			DefaultRequest: corev1.ResourceList{
				corev1.ResourceCPU:    checkedQuantity(cfg.DefaultReqCPU),
				corev1.ResourceMemory: checkedQuantity(cfg.DefaultReqMemory),
			},
		}}},
	}
}

// workspacePVCManifest 为代码与链运行态提供可快照工作区。
func workspacePVCManifest(cfg config.SandboxConfig) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: workspaceVolumeName},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: checkedQuantity(cfg.WorkspaceStorage),
				},
			},
		},
	}
}

// runtimeWorkloadManifest 生成包含 runtime 主容器与工具/infra sidecar 的 Deployment。
func runtimeWorkloadManifest(spec SandboxCreateSpec, cfg config.SandboxConfig) *appsv1.Deployment {
	labels := map[string]string{
		"app.kubernetes.io/name": "chaimir-sandbox-runtime",
		"chaimir.io/sandbox-id":  ids.Format(spec.SandboxID),
		"chaimir.io/tenant-id":   ids.Format(spec.TenantID),
	}
	replicas := int32(1)
	containers := []corev1.Container{buildRuntimeContainer(spec, cfg)}
	for _, sidecar := range spec.Runtime.AdapterSpec.InfraSidecars {
		containers = append(containers, buildDeclaredContainer(sidecar, sidecar.ImageURL, spec.Runtime.AdapterSpec.WorkspaceDir, cfg))
	}
	for _, tool := range spec.Tools {
		if tool.Kind != ToolKindWebEmbed {
			continue
		}
		containers = append(containers, buildToolContainer(tool, spec.Runtime.AdapterSpec.WorkspaceDir, cfg))
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: sandboxWorkloadName, Namespace: spec.Namespace, Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: boolPtr(false),
					RestartPolicy:                corev1.RestartPolicyAlways,
					SecurityContext:              restrictedPodSecurityContext(),
					Volumes: []corev1.Volume{{
						Name: workspaceVolumeName,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: workspaceVolumeName,
							},
						},
					}},
					Containers: containers,
				},
			},
		},
	}
}

// prepullNamespaceManifest 生成镜像预拉取受控命名空间。
// prepullNamespaceName 返回配置声明的镜像预拉取受控命名空间。
func prepullNamespaceName(cfg config.SandboxConfig) string {
	return strings.TrimSpace(cfg.PrepullNamespace)
}

// prepullNamespaceManifest 生成镜像预拉取受控命名空间。
func prepullNamespaceManifest(cfg config.SandboxConfig) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: prepullNamespaceName(cfg),
			Labels: map[string]string{
				"app.kubernetes.io/name": "chaimir-prepull",
				"module":                 "sandbox",
			},
		},
	}
}

// prepullDaemonSetManifest 生成全节点镜像预拉取 DaemonSet。
func prepullDaemonSetManifest(spec ImagePrepullSpec, cfg config.SandboxConfig) *appsv1.DaemonSet {
	name := "chaimir-prepull-" + ids.Format(spec.RuntimeImageID)
	labels := map[string]string{
		"app":                       "chaimir",
		"module":                    "sandbox",
		"runtime_image_id":          ids.Format(spec.RuntimeImageID),
		"app.kubernetes.io/name":    "chaimir-prepull",
		"app.kubernetes.io/part-of": "chaimir",
	}
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: prepullNamespaceName(cfg), Labels: labels},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: boolPtr(false),
					SecurityContext:              restrictedPodSecurityContext(),
					Containers: []corev1.Container{{
						Name:            prepullContainer,
						Image:           spec.ImageURL,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"tail", "-f", "/dev/null"},
						SecurityContext: restrictedSecurityContext(),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    checkedQuantity(cfg.PrepullRequestCPU),
								corev1.ResourceMemory: checkedQuantity(cfg.PrepullRequestMemory),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    checkedQuantity(cfg.PrepullLimitCPU),
								corev1.ResourceMemory: checkedQuantity(cfg.PrepullLimitMemory),
							},
						},
					}},
				},
			},
		},
	}
}

// strconvTime 把时间压缩成 K8s 资源名可用的后缀。
func strconvTime(t time.Time) string {
	return t.UTC().Format("20060102150405")
}

// runtimeServiceManifest 为 runtime 与 web 工具 sidecar 暴露 ClusterIP Service。
func runtimeServiceManifest(spec SandboxCreateSpec) *corev1.Service {
	var ports []corev1.ServicePort
	for _, port := range spec.Runtime.AdapterSpec.RuntimeContainer.Ports {
		ports = append(ports, servicePort(port))
	}
	for _, tool := range spec.Tools {
		if tool.Kind != ToolKindWebEmbed || tool.Port <= 0 {
			continue
		}
		ports = append(ports, corev1.ServicePort{
			Name:       tool.Code,
			Port:       tool.Port,
			TargetPort: intstr.FromInt32(tool.Port),
			Protocol:   corev1.ProtocolTCP,
		})
	}
	if len(ports) == 0 {
		return nil
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: sandboxServiceName, Namespace: spec.Namespace},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app.kubernetes.io/name": "chaimir-sandbox-runtime",
				"chaimir.io/sandbox-id":  ids.Format(spec.SandboxID),
			},
			Ports: ports,
		},
	}
}

// buildRuntimeContainer 构造运行时主容器。
func buildRuntimeContainer(spec SandboxCreateSpec, cfg config.SandboxConfig) corev1.Container {
	containerSpec := spec.Runtime.AdapterSpec.RuntimeContainer
	container := buildDeclaredContainer(containerSpec, spec.Image.ImageURL, spec.Runtime.AdapterSpec.WorkspaceDir, cfg)
	container.Name = containerSpec.Name
	if container.Name == "" {
		container.Name = "runtime"
	}
	return container
}

// buildToolContainer 构造 web-embed 工具 sidecar。
func buildToolContainer(tool ToolDefinition, workspaceDir string, cfg config.SandboxConfig) corev1.Container {
	spec := ContainerSpec{
		Name:           tool.Code,
		ImageURL:       tool.ImageURL,
		Command:        tool.Spec.Command,
		Args:           tool.Spec.Args,
		Env:            tool.Spec.Env,
		Ports:          []PortSpec{{Name: tool.Code, ContainerPort: tool.Port, ServicePort: tool.Port, Protocol: "TCP"}},
		Resources:      tool.Spec.Resources,
		ReadinessProbe: tool.Spec.ReadinessProbe,
		LivenessProbe:  tool.Spec.LivenessProbe,
		Workdir:        tool.Spec.Workdir,
		MountWorkspace: tool.Spec.MountWorkspace,
	}
	return buildDeclaredContainer(spec, tool.ImageURL, workspaceDir, cfg)
}

// buildDeclaredContainer 把声明式 spec 映射为 Kubernetes 容器。
func buildDeclaredContainer(spec ContainerSpec, imageURL, workspaceDir string, cfg config.SandboxConfig) corev1.Container {
	container := corev1.Container{
		Name:            spec.Name,
		Image:           imageURL,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         spec.Command,
		Args:            spec.Args,
		Env:             buildEnvVars(spec.Env),
		Ports:           buildContainerPorts(spec.Ports),
		Resources:       buildResources(spec.Resources),
		SecurityContext: restrictedSecurityContext(),
		WorkingDir:      chooseWorkdir(spec.Workdir, workspaceDir),
	}
	if shouldMountWorkspace(spec) {
		container.VolumeMounts = []corev1.VolumeMount{{
			Name:      workspaceVolumeName,
			MountPath: workspaceDir,
		}}
	}
	if probe := buildProbe(spec.ReadinessProbe, cfg); probe != nil {
		container.ReadinessProbe = probe
	}
	if probe := buildProbe(spec.LivenessProbe, cfg); probe != nil {
		container.LivenessProbe = probe
	}
	return container
}

// buildEnvVars 把声明式 env 映射为 Kubernetes EnvVar。
func buildEnvVars(items []EnvVarSpec) []corev1.EnvVar {
	out := make([]corev1.EnvVar, 0, len(items))
	for _, item := range items {
		out = append(out, corev1.EnvVar{Name: item.Name, Value: item.Value})
	}
	return out
}

// buildContainerPorts 把声明式端口映射为 Kubernetes 容器端口。
func buildContainerPorts(items []PortSpec) []corev1.ContainerPort {
	out := make([]corev1.ContainerPort, 0, len(items))
	for _, item := range items {
		protocol := corev1.ProtocolTCP
		if strings.EqualFold(item.Protocol, "UDP") {
			protocol = corev1.ProtocolUDP
		}
		out = append(out, corev1.ContainerPort{Name: item.Name, ContainerPort: item.ContainerPort, Protocol: protocol})
	}
	return out
}

// buildResources 把声明式 requests/limits 转为 ResourceRequirements。
func buildResources(spec ResourceSpec) corev1.ResourceRequirements {
	out := corev1.ResourceRequirements{}
	if spec.Requests.CPU != "" || spec.Requests.Memory != "" {
		out.Requests = corev1.ResourceList{}
		if spec.Requests.CPU != "" {
			out.Requests[corev1.ResourceCPU] = checkedQuantity(spec.Requests.CPU)
		}
		if spec.Requests.Memory != "" {
			out.Requests[corev1.ResourceMemory] = checkedQuantity(spec.Requests.Memory)
		}
	}
	if spec.Limits.CPU != "" || spec.Limits.Memory != "" {
		out.Limits = corev1.ResourceList{}
		if spec.Limits.CPU != "" {
			out.Limits[corev1.ResourceCPU] = checkedQuantity(spec.Limits.CPU)
		}
		if spec.Limits.Memory != "" {
			out.Limits[corev1.ResourceMemory] = checkedQuantity(spec.Limits.Memory)
		}
	}
	return out
}

// validateK8sSandboxConfig 校验配置来源的资源声明,防止错误配置进入 K8s 请求路径。
func validateK8sSandboxConfig(cfg config.SandboxConfig) error {
	values := map[string]string{
		"SANDBOX_DEFAULT_CPU":            cfg.DefaultCPU,
		"SANDBOX_DEFAULT_MEMORY":         cfg.DefaultMemory,
		"SANDBOX_DEFAULT_REQUEST_CPU":    cfg.DefaultReqCPU,
		"SANDBOX_DEFAULT_REQUEST_MEMORY": cfg.DefaultReqMemory,
		"SANDBOX_MAX_CPU":                cfg.MaxCPU,
		"SANDBOX_MAX_MEMORY":             cfg.MaxMemory,
		"SANDBOX_MAX_PODS":               cfg.MaxPods,
		"SANDBOX_WORKSPACE_STORAGE":      cfg.WorkspaceStorage,
		"SANDBOX_PREPULL_REQUEST_CPU":    cfg.PrepullRequestCPU,
		"SANDBOX_PREPULL_REQUEST_MEMORY": cfg.PrepullRequestMemory,
		"SANDBOX_PREPULL_LIMIT_CPU":      cfg.PrepullLimitCPU,
		"SANDBOX_PREPULL_LIMIT_MEMORY":   cfg.PrepullLimitMemory,
	}
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("沙箱 K8s 配置 %s 不能为空", key)
		}
		if _, err := resource.ParseQuantity(value); err != nil {
			return fmt.Errorf("沙箱 K8s 配置 %s 非法: %w", key, err)
		}
	}
	return nil
}

// checkedQuantity 在调用方已校验后转换 Kubernetes quantity;异常时返回零值而非 panic。
func checkedQuantity(value string) resource.Quantity {
	q, err := resource.ParseQuantity(value)
	if err != nil {
		return resource.Quantity{}
	}
	return q
}

// buildProbe 生成统一探针定义。
func buildProbe(spec ProbeSpec, cfg config.SandboxConfig) *corev1.Probe {
	if spec.Type == "" {
		return nil
	}
	probe := &corev1.Probe{
		PeriodSeconds:    defaultInt32(spec.PeriodSeconds, cfg.ProbeDefaultPeriodSeconds),
		FailureThreshold: defaultInt32(spec.FailureThreshold, cfg.ProbeDefaultFailureThreshold),
	}
	switch spec.Type {
	case "tcp":
		probe.ProbeHandler = corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(spec.Port)},
		}
	case "http":
		probe.ProbeHandler = corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{Path: spec.Path, Port: intstr.FromString(spec.Port)},
		}
	case "exec":
		probe.ProbeHandler = corev1.ProbeHandler{
			Exec: &corev1.ExecAction{Command: spec.Command},
		}
	}
	return probe
}

// servicePort 生成 Service 端口。
func servicePort(port PortSpec) corev1.ServicePort {
	protocol := corev1.ProtocolTCP
	if strings.EqualFold(port.Protocol, "UDP") {
		protocol = corev1.ProtocolUDP
	}
	return corev1.ServicePort{
		Name:       port.Name,
		Port:       port.ServicePort,
		TargetPort: intstr.FromInt32(port.ContainerPort),
		Protocol:   protocol,
	}
}

// restrictedPodSecurityContext 设置 Pod 级 non-root、fsGroup 与默认 seccomp profile。
func restrictedPodSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsNonRoot: ptrTrue(),
		FSGroup:      int64Ptr(1000),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// restrictedSecurityContext 生成与文档一致的受限容器安全上下文。
func restrictedSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsNonRoot:             ptrTrue(),
		RunAsUser:                int64Ptr(1000),
		AllowPrivilegeEscalation: boolPtr(false),
		ReadOnlyRootFilesystem:   boolPtr(true),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

// specRuntimeContainerName 返回运行时主容器名。
func specRuntimeContainerName(pod *corev1.Pod) string {
	for _, container := range pod.Spec.Containers {
		if container.Name == "runtime" {
			return "runtime"
		}
	}
	if len(pod.Spec.Containers) > 0 {
		return pod.Spec.Containers[0].Name
	}
	return "runtime"
}

// runtimeBindingFromPodAndService 聚合执行定位与声明式 Service 端口,供 L2 能力按名称取端点。
func runtimeBindingFromPodAndService(namespace string, pod *corev1.Pod, svc *corev1.Service) SandboxRuntimeBinding {
	binding := SandboxRuntimeBinding{
		Namespace:  namespace,
		PodName:    pod.Name,
		Container:  specRuntimeContainerName(pod),
		PortByName: map[string]int32{},
	}
	if svc == nil {
		return binding
	}
	binding.ServiceName = svc.Name
	for _, port := range svc.Spec.Ports {
		binding.PortByName[port.Name] = port.Port
	}
	return binding
}

// chooseWorkdir 优先使用容器声明的工作目录,否则落回运行时统一 workspace。
func chooseWorkdir(specWorkdir, workspaceDir string) string {
	if strings.TrimSpace(specWorkdir) != "" {
		return specWorkdir
	}
	return workspaceDir
}

// shouldMountWorkspace 统一 mount_workspace 语义:
// runtime 主容器默认共享工作区;工具/sidecar 只有显式声明 true 时才挂载。
func shouldMountWorkspace(spec ContainerSpec) bool {
	if spec.Name == "runtime" {
		if spec.MountWorkspace == nil {
			return true
		}
		return *spec.MountWorkspace
	}
	return spec.MountWorkspace != nil && *spec.MountWorkspace
}

// defaultInt32 在声明式字段未设置时使用平台定义的默认值。
func defaultInt32(v, defaultValue int32) int32 {
	if v == 0 {
		return defaultValue
	}
	return v
}

// boolPtr 构造 Kubernetes API 需要的布尔指针字段。
func boolPtr(v bool) *bool { return &v }

// ptrTrue 返回 true 指针,用于显式开启 K8s 安全开关。
func ptrTrue() *bool {
	v := true
	return &v
}

// int64Ptr 构造 Kubernetes API 需要的 int64 指针字段。
func int64Ptr(v int64) *int64 { return &v }
