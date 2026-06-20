// sandbox service_k8s_orchestrator 文件负责把 M2 编排计划转换为受限 Kubernetes 资源。
package sandbox

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/config"
	platformk8s "chaimir/internal/platform/k8s"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/logging"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

var errMetricsUnavailable = errors.New("kubernetes metrics api unavailable")

const sandboxWorkloadServiceAccount = "sandbox-workload"

// K8sOrchestrator 使用 client-go 创建、回收和预拉取沙箱资源。
type K8sOrchestrator struct {
	client *platformk8s.Client
	cfg    config.SandboxConfig
}

// NewK8sOrchestrator 构造 K8s 编排器。
func NewK8sOrchestrator(client *platformk8s.Client, cfg config.SandboxConfig) *K8sOrchestrator {
	return &K8sOrchestrator{client: client, cfg: cfg}
}

// CreateSandboxResources 创建 Namespace、资源限制、默认拒绝网络、PVC、Pod 和工具 Service。
func (o *K8sOrchestrator) CreateSandboxResources(ctx context.Context, plan CreateSandboxPlan) error {
	cs := o.client.Clientset()
	ns := namespaceObject(plan.Sandbox.Namespace, plan.Sandbox)
	if _, err := cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("创建沙箱 Namespace 失败: %w", err)
	}
	if err := o.applySandboxServiceAccount(ctx, cs, plan); err != nil {
		return err
	}
	if err := o.applyResourceQuota(ctx, cs, plan); err != nil {
		return err
	}
	if err := o.applyLimitRange(ctx, cs, plan); err != nil {
		return err
	}
	if err := o.applyNetworkPolicies(ctx, cs, plan); err != nil {
		return err
	}
	if err := o.applyWorkspacePVC(ctx, cs, plan); err != nil {
		return err
	}
	if err := o.applyRuntimeServices(ctx, cs, plan); err != nil {
		return err
	}
	for _, pod := range o.podsForPlan(plan) {
		if _, err := cs.CoreV1().Pods(plan.Sandbox.Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("创建沙箱 Pod 失败: %w", err)
		}
	}
	for _, tool := range plan.Tools {
		if tool.Kind == SandboxToolKindWebEmbed {
			if err := o.applyToolService(ctx, cs, plan.Sandbox, tool); err != nil {
				return err
			}
		}
	}
	if err := o.waitSandboxPodReady(ctx, cs, plan); err != nil {
		return err
	}
	return nil
}

// DestroySandboxResources 删除普通沙箱资源。
func (o *K8sOrchestrator) DestroySandboxResources(ctx context.Context, sb Sandbox) error {
	policy := metav1.DeletePropagationForeground
	if err := o.client.Clientset().CoreV1().Namespaces().Delete(ctx, sb.Namespace, metav1.DeleteOptions{PropagationPolicy: &policy}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除沙箱 Namespace 失败: %w", err)
	}
	return waitNamespaceDeleted(ctx, o.client.Clientset(), sb.Namespace, o.cfg.ReadyPollIntervalSeconds)
}

// StopComputeKeepSnapshot 释放计算工作负载但保留快照命名空间和 PVC。
func (o *K8sOrchestrator) StopComputeKeepSnapshot(ctx context.Context, sb Sandbox) error {
	cs := o.client.Clientset()
	selector := fmt.Sprintf("chaimir.io/sandbox-id=%d", sb.ID)
	if err := cs.CoreV1().Pods(sb.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: selector}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除快照沙箱计算 Pod 失败: %w", err)
	}
	return o.cleanupSnapshotRetainedNamespace(ctx, cs, sb)
}

// CreateSnapshot 创建 CSI VolumeSnapshot 并返回 namespaced 引用。
func (o *K8sOrchestrator) CreateSnapshot(ctx context.Context, plan CreateSandboxPlan, retention time.Duration) (SnapshotResult, error) {
	domains := snapshotDomainsForPlan(plan)
	if len(domains) == 0 {
		return SnapshotResult{}, fmt.Errorf("沙箱没有可快照卷域")
	}
	gvr := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	for _, domain := range domains {
		name := volumeSnapshotName(plan.Sandbox, domain)
		obj := volumeSnapshotObject(plan.Sandbox, domain, name, retention, o.cfg.VolumeSnapshotClassName)
		if _, err := o.client.Dynamic().Resource(gvr).Namespace(plan.Sandbox.Namespace).Create(ctx, obj, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return SnapshotResult{}, fmt.Errorf("创建 VolumeSnapshot 失败: %w", err)
		}
		if err := o.waitVolumeSnapshotReady(ctx, gvr, plan.Sandbox.Namespace, name); err != nil {
			return SnapshotResult{}, err
		}
	}
	return SnapshotResult{Ref: plan.Sandbox.Namespace + "/" + snapshotGroupName(plan.Sandbox), Domains: domains}, nil
}

// CleanupSnapshotResources 清理快照保留到期后的 Namespace/PVC/VolumeSnapshot。
func (o *K8sOrchestrator) CleanupSnapshotResources(ctx context.Context, sb Sandbox) error {
	return o.DestroySandboxResources(ctx, sb)
}

// waitVolumeSnapshotReady 等待 CSI 快照 readyToUse,避免记录尚不可恢复的快照引用。
func (o *K8sOrchestrator) waitVolumeSnapshotReady(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	if o.cfg.ReadyPollIntervalSeconds <= 0 {
		return fmt.Errorf("SANDBOX_READY_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	contentGVR := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotcontents"}
	ticker := time.NewTicker(time.Duration(o.cfg.ReadyPollIntervalSeconds) * time.Second)
	defer ticker.Stop()
	for {
		current, err := o.client.Dynamic().Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("查询 VolumeSnapshot 状态失败: %w", err)
		}
		contentName, _, _ := unstructured.NestedString(current.Object, "status", "boundVolumeSnapshotContentName")
		if contentName != "" {
			content, err := o.client.Dynamic().Resource(contentGVR).Get(ctx, contentName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("查询 VolumeSnapshotContent 状态失败: %w", err)
			}
			if volumeSnapshotReadyAndBound(current, content, namespace, name) {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待 VolumeSnapshot Ready 超时: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// volumeSnapshotReadyAndBound 校验快照 ready 且 VolumeSnapshotContent 双向反指当前快照。
func volumeSnapshotReadyAndBound(snapshot, content *unstructured.Unstructured, namespace, name string) bool {
	if snapshot == nil || content == nil {
		return false
	}
	ready, _, _ := unstructured.NestedBool(snapshot.Object, "status", "readyToUse")
	if !ready {
		return false
	}
	contentName, _, _ := unstructured.NestedString(snapshot.Object, "status", "boundVolumeSnapshotContentName")
	if strings.TrimSpace(contentName) == "" || content.GetName() != contentName {
		return false
	}
	refName, _, _ := unstructured.NestedString(content.Object, "spec", "volumeSnapshotRef", "name")
	refNamespace, _, _ := unstructured.NestedString(content.Object, "spec", "volumeSnapshotRef", "namespace")
	return refName == name && refNamespace == namespace
}

// RestoreSnapshotResources 基于保留 PVC 或同命名空间 VolumeSnapshot 恢复沙箱运行资源。
func (o *K8sOrchestrator) RestoreSnapshotResources(ctx context.Context, plan CreateSandboxPlan) error {
	cs := o.client.Clientset()
	ns := namespaceObject(plan.Sandbox.Namespace, plan.Sandbox)
	if _, err := cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("创建快照恢复 Namespace 失败: %w", err)
	}
	if err := o.applySandboxServiceAccount(ctx, cs, plan); err != nil {
		return err
	}
	if err := o.applyResourceQuota(ctx, cs, plan); err != nil {
		return err
	}
	if err := o.applyLimitRange(ctx, cs, plan); err != nil {
		return err
	}
	if err := o.applyNetworkPolicies(ctx, cs, plan); err != nil {
		return err
	}
	if err := o.applyWorkspacePVCFromSnapshot(ctx, cs, plan); err != nil {
		return err
	}
	if err := o.applyRuntimeServices(ctx, cs, plan); err != nil {
		return err
	}
	for _, pod := range o.podsForPlan(plan) {
		if _, err := cs.CoreV1().Pods(plan.Sandbox.Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("创建快照恢复 Pod 失败: %w", err)
		}
	}
	for _, tool := range plan.Tools {
		if tool.Kind == SandboxToolKindWebEmbed {
			if err := o.applyToolService(ctx, cs, plan.Sandbox, tool); err != nil {
				return err
			}
		}
	}
	return o.waitSandboxPodReady(ctx, cs, plan)
}

// ResourceUsage 汇总沙箱 Pod 与工作区 PVC 的资源申请和限制。
func (o *K8sOrchestrator) ResourceUsage(ctx context.Context, sb Sandbox) (contracts.SandboxResourceUsage, error) {
	cs := o.client.Clientset()
	usage := contracts.SandboxResourceUsage{}
	metrics, err := o.client.Metrics().MetricsV1beta1().PodMetricses(sb.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("chaimir.io/sandbox-id=%d", sb.ID),
	})
	if err != nil {
		if unavailable := metricsUnavailableError(err); unavailable != nil {
			return usage, unavailable
		}
		return usage, fmt.Errorf("查询沙箱实时资源用量失败: %w", err)
	}
	addPodMetricsUsage(&usage, metrics.Items)
	pods, err := cs.CoreV1().Pods(sb.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("chaimir.io/sandbox-id=%d", sb.ID),
	})
	if apierrors.IsNotFound(err) {
		return usage, nil
	}
	if err != nil {
		return usage, fmt.Errorf("查询沙箱 Pod 资源失败: %w", err)
	}
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			addResourceUsage(&usage, container.Resources)
		}
	}
	pvcs, err := cs.CoreV1().PersistentVolumeClaims(sb.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("chaimir.io/sandbox-id=%d", sb.ID),
	})
	if apierrors.IsNotFound(err) {
		return usage, nil
	}
	if err != nil {
		return usage, fmt.Errorf("查询沙箱 PVC 失败: %w", err)
	}
	for _, pvc := range pvcs.Items {
		if storage, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			usage.StorageBytes += storage.Value()
		}
	}
	return usage, nil
}

// metricsUnavailableError 识别 metrics.k8s.io 不可用,避免把缺失实时用量伪装成 0。
func metricsUnavailableError(err error) error {
	if err == nil {
		return nil
	}
	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) {
		details := statusErr.ErrStatus.Details
		if details != nil && strings.EqualFold(details.Group, "metrics.k8s.io") {
			return fmt.Errorf("%w: %w", errMetricsUnavailable, err)
		}
	}
	if strings.Contains(err.Error(), "metrics.k8s.io") || strings.Contains(err.Error(), "the server could not find the requested resource") {
		return fmt.Errorf("%w: %w", errMetricsUnavailable, err)
	}
	return nil
}

// Exec 在沙箱容器中执行受控命令。
func (o *K8sOrchestrator) Exec(ctx context.Context, namespace, container string, command []string, stdin []byte, tty bool) ([]byte, []byte, error) {
	podName, containerName := splitExecTarget(container)
	var stdout, stderr bytes.Buffer
	err := o.client.Exec(ctx, namespace, podName, containerName, command, execInputReader(stdin), &stdout, &stderr, tty)
	return stdout.Bytes(), stderr.Bytes(), err
}

// execInputReader 把可选 stdin 转为真正的 io.Reader,避免把带类型的 nil 传给 client-go exec。
func execInputReader(stdin []byte) io.Reader {
	if stdin == nil {
		return nil
	}
	return bytes.NewReader(stdin)
}

// ExecStream 在沙箱容器中执行交互式命令并透传标准流。
func (o *K8sOrchestrator) ExecStream(ctx context.Context, namespace, container string, command []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, tty bool) error {
	podName, containerName := splitExecTarget(container)
	return o.client.ExecStream(ctx, namespace, podName, containerName, command, stdin, stdout, stderr, tty)
}

// PrepullImage 创建或更新预拉取 DaemonSet 并等待工作负载镜像集合在真实节点 Ready。
func (o *K8sOrchestrator) PrepullImage(ctx context.Context, image RuntimeImage, imageURLs []string) (PrepullResult, error) {
	ds := o.prepullDaemonSet(image, imageURLs)
	cs := o.client.Clientset()
	client := cs.AppsV1().DaemonSets(o.cfg.PrepullNamespace)
	existing, err := client.Get(ctx, ds.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, err := client.Create(ctx, ds, metav1.CreateOptions{}); err != nil {
			return PrepullResult{DaemonSet: ds.Name}, fmt.Errorf("创建预拉取 DaemonSet 失败: %w", err)
		}
	} else if err != nil {
		return PrepullResult{DaemonSet: ds.Name}, fmt.Errorf("查询预拉取 DaemonSet 失败: %w", err)
	} else {
		ds.ResourceVersion = existing.ResourceVersion
		if _, err := client.Update(ctx, ds, metav1.UpdateOptions{}); err != nil {
			return PrepullResult{DaemonSet: ds.Name}, fmt.Errorf("更新预拉取 DaemonSet 失败: %w", err)
		}
	}
	ticker := time.NewTicker(time.Duration(o.cfg.PrepullPollIntervalSeconds) * time.Second)
	defer ticker.Stop()
	timeout := time.NewTimer(time.Duration(o.cfg.PrepullTimeoutSeconds) * time.Second)
	defer timeout.Stop()
	for {
		current, err := client.Get(ctx, ds.Name, metav1.GetOptions{})
		if err != nil {
			return PrepullResult{DaemonSet: ds.Name}, fmt.Errorf("查询预拉取状态失败: %w", err)
		}
		detail, err := jsonBytes(map[string]any{"desired_nodes": current.Status.DesiredNumberScheduled, "ready_nodes": current.Status.NumberReady, "daemonset": ds.Name, "image_count": len(imageURLs), "images": imageURLs})
		if err != nil {
			return PrepullResult{DaemonSet: ds.Name}, fmt.Errorf("编码预拉取状态失败: %w", err)
		}
		result := PrepullResult{
			DesiredNodes: current.Status.DesiredNumberScheduled,
			ReadyNodes:   current.Status.NumberReady,
			DaemonSet:    ds.Name,
			Detail:       detail,
		}
		pods, err := cs.CoreV1().Pods(o.cfg.PrepullNamespace).List(ctx, metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(ds.Spec.Selector)})
		if err != nil {
			return result, fmt.Errorf("查询预拉取 Pod 状态失败: %w", err)
		}
		if err := imagePullFailureFromPods(pods.Items); err != nil {
			detail, encodeErr := jsonBytes(map[string]any{"error": logging.SanitizeError(err.Error()), "daemonset": ds.Name})
			if encodeErr != nil {
				return result, fmt.Errorf("编码预拉取失败详情失败: %w", encodeErr)
			}
			result.Detail = detail
			return result, err
		}
		if current.Status.DesiredNumberScheduled > 0 &&
			current.Status.DesiredNumberScheduled == current.Status.NumberReady &&
			current.Status.DesiredNumberScheduled == current.Status.UpdatedNumberScheduled {
			return result, nil
		}
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-timeout.C:
			return result, fmt.Errorf("预拉取等待超时")
		case <-ticker.C:
		}
	}
}

// DeletePrepullDaemonSet 删除镜像预拉取 DaemonSet,NotFound 视为幂等成功。
func (o *K8sOrchestrator) DeletePrepullDaemonSet(ctx context.Context, image RuntimeImage) error {
	ds := o.prepullDaemonSet(image, []string{image.ImageURL})
	err := o.client.Clientset().AppsV1().DaemonSets(o.cfg.PrepullNamespace).Delete(ctx, ds.Name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("删除预拉取 DaemonSet 失败: %w", err)
	}
	return nil
}

// imagePullFailureFromPods 从预拉取 Pod 状态中识别真实镜像拉取失败,避免等待超时才暴露错误。
func imagePullFailureFromPods(pods []corev1.Pod) error {
	failureReasons := map[string]struct{}{
		"ImagePullBackOff": {},
		"ErrImagePull":     {},
		"InvalidImageName": {},
	}
	for _, pod := range pods {
		for _, status := range pod.Status.ContainerStatuses {
			if status.State.Waiting == nil {
				continue
			}
			reason := strings.TrimSpace(status.State.Waiting.Reason)
			if _, failed := failureReasons[reason]; !failed {
				continue
			}
			message := strings.TrimSpace(status.State.Waiting.Message)
			if message == "" {
				return fmt.Errorf("预拉取 Pod %s 容器 %s 镜像拉取失败: %s", pod.Name, status.Name, reason)
			}
			return fmt.Errorf("预拉取 Pod %s 容器 %s 镜像拉取失败: %s: %s", pod.Name, status.Name, reason, message)
		}
	}
	return nil
}

// waitSandboxPodReady 等待沙箱主 Pod 达到 Ready,避免资源已创建但运行时尚不可用时提前放行用户。
func (o *K8sOrchestrator) waitSandboxPodReady(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan) error {
	if o.cfg.ReadyPollIntervalSeconds <= 0 {
		return fmt.Errorf("SANDBOX_READY_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	podNames := runtimePodNames(plan)
	ticker := time.NewTicker(time.Duration(o.cfg.ReadyPollIntervalSeconds) * time.Second)
	defer ticker.Stop()
	for {
		ready := true
		for _, podName := range podNames {
			pod, err := cs.CoreV1().Pods(plan.Sandbox.Namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("查询沙箱 Pod 状态失败: %w", err)
			}
			if err != nil || !sandboxPodReady(pod) {
				ready = false
				break
			}
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待沙箱 Pod Ready 超时: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// ToolReady 校验 Web 工具的所有声明组件都已通过 Kubernetes Ready 条件。
func (o *K8sOrchestrator) ToolReady(ctx context.Context, sb Sandbox, tool Tool) error {
	if len(tool.ResourceSpec.Components) == 0 {
		return fmt.Errorf("工具组件未声明")
	}
	for _, component := range tool.ResourceSpec.Components {
		pod, err := o.client.Clientset().CoreV1().Pods(sb.Namespace).Get(ctx, toolComponentPodName(tool.Code, component.Name), metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("查询工具 Pod 状态失败: %w", err)
		}
		found := false
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == component.Name {
				found = true
				if !status.Ready {
					return fmt.Errorf("工具组件尚未就绪")
				}
				break
			}
		}
		if !found {
			return fmt.Errorf("工具组件不存在")
		}
	}
	return nil
}

// sandboxPodReady 判断 Kubernetes Pod 是否达到 Ready 条件。
func sandboxPodReady(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// SnapshotSupported 返回当前集群是否安装并启用 CSI 快照能力。
func (o *K8sOrchestrator) SnapshotSupported(ctx context.Context) (bool, error) {
	if strings.TrimSpace(o.cfg.StorageClassName) == "" || strings.TrimSpace(o.cfg.VolumeSnapshotClassName) == "" {
		return false, nil
	}
	if _, err := o.client.Clientset().StorageV1().StorageClasses().Get(ctx, o.cfg.StorageClassName, metav1.GetOptions{}); apierrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	gvr := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	_, err := o.client.Dynamic().Resource(gvr).Namespace(o.cfg.ControlNamespace).List(ctx, metav1.ListOptions{Limit: 1})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil && strings.Contains(err.Error(), "the server could not find the requested resource") {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	classGVR := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotclasses"}
	class, err := o.client.Dynamic().Resource(classGVR).Get(ctx, o.cfg.VolumeSnapshotClassName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return volumeSnapshotClassAllowsDeletion(class), nil
}

// volumeSnapshotClassAllowsDeletion 要求快照类删除策略为 Delete,避免到期清理后底层快照继续占用存储。
func volumeSnapshotClassAllowsDeletion(class *unstructured.Unstructured) bool {
	if class == nil {
		return false
	}
	policy, _, _ := unstructured.NestedString(class.Object, "deletionPolicy")
	return strings.EqualFold(strings.TrimSpace(policy), "Delete")
}

// waitNamespaceDeleted 等待动态沙箱 Namespace 真正消失,避免数据库状态先行标记 destroyed 后存储仍残留。
func waitNamespaceDeleted(ctx context.Context, cs kubernetes.Interface, namespace string, pollSeconds int) error {
	if pollSeconds <= 0 {
		return fmt.Errorf("SANDBOX_READY_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	ticker := time.NewTicker(time.Duration(pollSeconds) * time.Second)
	defer ticker.Stop()
	for {
		if _, err := cs.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{}); apierrors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return fmt.Errorf("查询沙箱 Namespace 删除状态失败: %w", err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待沙箱 Namespace 删除超时: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// cleanupSnapshotRetainedNamespace 只保留快照恢复所需的 PVC 与 VolumeSnapshot,其余运行资源立即释放。
func (o *K8sOrchestrator) cleanupSnapshotRetainedNamespace(ctx context.Context, cs kubernetes.Interface, sb Sandbox) error {
	if err := deleteServices(ctx, cs, sb.Namespace); err != nil {
		return err
	}
	if err := cs.CoreV1().ConfigMaps(sb.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: "app=chaimir,module=sandbox"}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除快照沙箱 ConfigMap 失败: %w", err)
	}
	if err := cs.CoreV1().Secrets(sb.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: "app=chaimir,module=sandbox"}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除快照沙箱 Secret 失败: %w", err)
	}
	if err := cs.NetworkingV1().NetworkPolicies(sb.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除快照沙箱 NetworkPolicy 失败: %w", err)
	}
	if err := cs.CoreV1().ResourceQuotas(sb.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除快照沙箱 ResourceQuota 失败: %w", err)
	}
	if err := cs.CoreV1().LimitRanges(sb.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除快照沙箱 LimitRange 失败: %w", err)
	}
	if err := cs.CoreV1().ServiceAccounts(sb.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: "app=chaimir,module=sandbox"}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除快照沙箱 ServiceAccount 失败: %w", err)
	}
	return nil
}

// deleteServices 显式逐个删除 Service;client-go 的 ServiceInterface 不提供 DeleteCollection。
func deleteServices(ctx context.Context, cs kubernetes.Interface, namespace string) error {
	services, err := cs.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("列出快照沙箱 Service 失败: %w", err)
	}
	for _, svc := range services.Items {
		if err := cs.CoreV1().Services(namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("删除快照沙箱 Service 失败: %w", err)
		}
	}
	return nil
}

// namespaceObject 构造带平台所有权标签的动态沙箱 Namespace。
func namespaceObject(name string, sb Sandbox) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: name,
		Labels: map[string]string{
			"app":                                "chaimir",
			"app.kubernetes.io/part-of":          "chaimir",
			"module":                             "sandbox",
			"chaimir.io/sandbox":                 "true",
			"chaimir.io/managed-by":              "chaimir-backend",
			"chaimir.io/tenant-id":               fmt.Sprintf("%d", sb.TenantID),
			"chaimir.io/sandbox-id":              fmt.Sprintf("%d", sb.ID),
			"pod-security.kubernetes.io/enforce": "restricted",
		},
	}}
}

// applySandboxServiceAccount 创建无权限工作负载账号,避免依赖 Kubernetes 默认 SA 异步生成。
func (o *K8sOrchestrator) applySandboxServiceAccount(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan) error {
	automount := false
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sandboxWorkloadServiceAccount,
			Namespace: plan.Sandbox.Namespace,
			Labels: map[string]string{
				"app":                   "chaimir",
				"module":                "sandbox",
				"chaimir.io/sandbox-id": fmt.Sprintf("%d", plan.Sandbox.ID),
			},
		},
		AutomountServiceAccountToken: &automount,
	}
	if _, err := cs.CoreV1().ServiceAccounts(plan.Sandbox.Namespace).Create(ctx, sa, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("创建沙箱 ServiceAccount 失败: %w", err)
	}
	return nil
}

// applyResourceQuota 创建 Namespace 级资源硬限。
func (o *K8sOrchestrator) applyResourceQuota(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan) error {
	rq := o.resourceQuotaForPlan(plan)
	if _, err := cs.CoreV1().ResourceQuotas(plan.Sandbox.Namespace).Create(ctx, rq, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("创建 ResourceQuota 失败: %w", err)
	}
	return nil
}

// resourceQuotaForPlan 构造 Namespace 级资源硬限,避免工具 sidecar 被单容器默认 request 误拒。
func (o *K8sOrchestrator) resourceQuotaForPlan(plan CreateSandboxPlan) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-quota", Namespace: plan.Sandbox.Namespace},
		Spec: corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{
			corev1.ResourceRequestsCPU:    resource.MustParse(o.cfg.MaxCPU),
			corev1.ResourceRequestsMemory: resource.MustParse(o.cfg.MaxMemory),
			corev1.ResourceLimitsCPU:      resource.MustParse(o.cfg.MaxCPU),
			corev1.ResourceLimitsMemory:   resource.MustParse(o.cfg.MaxMemory),
			corev1.ResourcePods:           resource.MustParse(o.cfg.MaxPods),
		}},
	}
}

// applyLimitRange 创建容器默认 requests/limits。
func (o *K8sOrchestrator) applyLimitRange(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan) error {
	lr := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-limits", Namespace: plan.Sandbox.Namespace},
		Spec: corev1.LimitRangeSpec{Limits: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Default: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(o.cfg.DefaultCPU),
				corev1.ResourceMemory: resource.MustParse(o.cfg.DefaultMemory),
			},
			DefaultRequest: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(o.cfg.DefaultReqCPU),
				corev1.ResourceMemory: resource.MustParse(o.cfg.DefaultReqMemory),
			},
		}}},
	}
	if _, err := cs.CoreV1().LimitRanges(plan.Sandbox.Namespace).Create(ctx, lr, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("创建 LimitRange 失败: %w", err)
	}
	return nil
}

// applyNetworkPolicies 创建默认拒绝、控制面入口和清单声明的同沙箱 Pod 互通策略。
func (o *K8sOrchestrator) applyNetworkPolicies(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan) error {
	deny := &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-deny-all", Namespace: plan.Sandbox.Namespace, Labels: sandboxPolicyLabels()},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeIngress, netv1.PolicyTypeEgress},
		},
	}
	if _, err := cs.NetworkingV1().NetworkPolicies(plan.Sandbox.Namespace).Create(ctx, deny, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("创建 deny-all NetworkPolicy 失败: %w", err)
	}
	if _, err := cs.NetworkingV1().NetworkPolicies(plan.Sandbox.Namespace).Create(ctx, o.dnsEgressPolicy(plan.Sandbox), metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("创建 DNS NetworkPolicy 失败: %w", err)
	}
	for _, policy := range o.allowControlPlanePolicies(plan) {
		if _, err := cs.NetworkingV1().NetworkPolicies(plan.Sandbox.Namespace).Create(ctx, policy, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("创建控制面 NetworkPolicy 失败: %w", err)
		}
	}
	for _, policy := range o.allowSandboxPodLinkPolicies(plan) {
		if _, err := cs.NetworkingV1().NetworkPolicies(plan.Sandbox.Namespace).Create(ctx, policy, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("创建沙箱内 Pod 网络策略失败: %w", err)
		}
	}
	for _, policy := range o.allowToolPodLinkPolicies(plan) {
		if _, err := cs.NetworkingV1().NetworkPolicies(plan.Sandbox.Namespace).Create(ctx, policy, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("创建工具 Pod 网络策略失败: %w", err)
		}
	}
	return nil
}

// dnsEgressPolicy 允许沙箱内 Pod 解析集群 Service DNS,但不放开其他出站目标。
func (o *K8sOrchestrator) dnsEgressPolicy(sb Sandbox) *netv1.NetworkPolicy {
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sandbox-allow-dns-egress",
			Namespace: sb.Namespace,
			Labels:    sandboxPolicyLabels(),
		},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeEgress},
			Egress: []netv1.NetworkPolicyEgressRule{{
				To: []netv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": o.cfg.DNSNamespace}},
				}},
				Ports: dnsNetworkPolicyPorts(o.cfg.DNSPort),
			}},
		},
	}
}

// sandboxPolicyLabels 与部署层动态沙箱 NetworkPolicy 审计模板保持一致。
func sandboxPolicyLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/part-of": "chaimir",
		"chaimir.io/sandbox":        "true",
	}
}

// allowControlPlanePolicies 只允许后端控制面按 Pod 角色访问运行时和工具各自声明的端口。
func (o *K8sOrchestrator) allowControlPlanePolicies(plan CreateSandboxPlan) []*netv1.NetworkPolicy {
	policies := []*netv1.NetworkPolicy{}
	for _, pod := range podGroupForPlan(plan) {
		ports := networkPolicyPortsForSinglePod(pod)
		if len(ports) == 0 {
			continue
		}
		policies = append(policies, o.controlPlaneIngressPolicy(plan.Sandbox, "sandbox-allow-control-plane-"+pod.Name, pod.Name, ports))
	}
	for _, tool := range plan.Tools {
		if tool.Kind != SandboxToolKindWebEmbed {
			continue
		}
		for role, ports := range toolControlPlanePorts(tool) {
			if len(ports) == 0 {
				continue
			}
			policies = append(policies, o.controlPlaneIngressPolicy(plan.Sandbox, "sandbox-allow-control-plane-"+role, role, ports))
		}
	}
	return policies
}

// controlPlaneIngressPolicy 构造控制面到单个 Pod 角色的最小入口策略。
func (o *K8sOrchestrator) controlPlaneIngressPolicy(sb Sandbox, name, role string, ports []netv1.NetworkPolicyPort) *netv1.NetworkPolicy {
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: sb.Namespace},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: sandboxPodRoleLabels(sb, role)},
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeIngress},
			Ingress: []netv1.NetworkPolicyIngressRule{{
				From: []netv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": o.cfg.ControlNamespace}},
					PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{o.cfg.ControlPodLabelKey: o.cfg.ControlPodLabelValue}},
				}},
				Ports: ports,
			}},
		},
	}
}

// allowSandboxPodLinkPolicies 仅为 adapter_spec.network_rules 声明的 Pod 访问生成 ingress/egress 放行。
func (o *K8sOrchestrator) allowSandboxPodLinkPolicies(plan CreateSandboxPlan) []*netv1.NetworkPolicy {
	policies := make([]*netv1.NetworkPolicy, 0, len(plan.Runtime.AdapterSpec.NetworkRules)*2)
	for _, rule := range plan.Runtime.AdapterSpec.NetworkRules {
		ports := networkPolicyPortsForRefs(rule.Ports)
		// deny-all 同时限制入站和出站,因此显式互通需要为源 Pod 出站和目标 Pod 入站各建一条策略。
		policies = append(policies,
			sandboxPodIngressPolicy(plan, rule, ports),
			sandboxPodEgressPolicy(plan, rule, ports),
		)
	}
	return policies
}

// sandboxPodIngressPolicy 允许来源 Pod 访问目标 Pod 的声明端口。
func sandboxPodIngressPolicy(plan CreateSandboxPlan, rule workload.NetworkRuleSpec, ports []netv1.NetworkPolicyPort) *netv1.NetworkPolicy {
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-allow-" + rule.Name + "-ingress", Namespace: plan.Sandbox.Namespace},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: sandboxPodRoleLabels(plan.Sandbox, rule.To)},
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeIngress},
			Ingress: []netv1.NetworkPolicyIngressRule{{
				From:  []netv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: sandboxPodRoleLabels(plan.Sandbox, rule.From)}}},
				Ports: ports,
			}},
		},
	}
}

// sandboxPodEgressPolicy 允许来源 Pod 出站访问目标 Pod 的声明端口。
func sandboxPodEgressPolicy(plan CreateSandboxPlan, rule workload.NetworkRuleSpec, ports []netv1.NetworkPolicyPort) *netv1.NetworkPolicy {
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-allow-" + rule.Name + "-egress", Namespace: plan.Sandbox.Namespace},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: sandboxPodRoleLabels(plan.Sandbox, rule.From)},
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeEgress},
			Egress: []netv1.NetworkPolicyEgressRule{{
				To:    []netv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: sandboxPodRoleLabels(plan.Sandbox, rule.To)}}},
				Ports: ports,
			}},
		},
	}
}

// allowToolPodLinkPolicies 仅为 web-embed 工具声明的运行时访问生成 NetworkPolicy。
func (o *K8sOrchestrator) allowToolPodLinkPolicies(plan CreateSandboxPlan) []*netv1.NetworkPolicy {
	policies := []*netv1.NetworkPolicy{}
	for _, tool := range plan.Tools {
		if tool.Kind != SandboxToolKindWebEmbed {
			continue
		}
		for _, rule := range tool.ResourceSpec.NetworkRules {
			ports := networkPolicyPortsForRefs(rule.Ports)
			policies = append(policies,
				toolPodIngressPolicy(plan, tool, rule, ports),
				toolPodEgressPolicy(plan, tool, rule, ports),
			)
		}
	}
	return policies
}

// toolPodIngressPolicy 允许工具 Pod 访问目标运行时 Pod 的声明端口。
func toolPodIngressPolicy(plan CreateSandboxPlan, tool Tool, rule workload.NetworkRuleSpec, ports []netv1.NetworkPolicyPort) *netv1.NetworkPolicy {
	role := toolComponentPodName(tool.Code, rule.From)
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-allow-tool-" + rule.Name + "-ingress", Namespace: plan.Sandbox.Namespace},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: sandboxPodRoleLabels(plan.Sandbox, rule.To)},
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeIngress},
			Ingress: []netv1.NetworkPolicyIngressRule{{
				From:  []netv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: sandboxPodRoleLabels(plan.Sandbox, role)}}},
				Ports: ports,
			}},
		},
	}
}

// toolPodEgressPolicy 允许工具 Pod 出站访问目标运行时 Pod 的声明端口。
func toolPodEgressPolicy(plan CreateSandboxPlan, tool Tool, rule workload.NetworkRuleSpec, ports []netv1.NetworkPolicyPort) *netv1.NetworkPolicy {
	role := toolComponentPodName(tool.Code, rule.From)
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-allow-tool-" + rule.Name + "-egress", Namespace: plan.Sandbox.Namespace},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: sandboxPodRoleLabels(plan.Sandbox, role)},
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeEgress},
			Egress: []netv1.NetworkPolicyEgressRule{{
				To:    []netv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: sandboxPodRoleLabels(plan.Sandbox, rule.To)}}},
				Ports: ports,
			}},
		},
	}
}

// applyWorkspacePVC 为所有声明为持久化的卷安全域创建 PVC。
func (o *K8sOrchestrator) applyWorkspacePVC(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan) error {
	return o.createPersistentVolumeClaims(ctx, cs, plan, nil)
}

// applyWorkspacePVCFromSnapshot 从同命名空间 VolumeSnapshot 恢复可快照卷域 PVC,已有 PVC 时直接复用。
func (o *K8sOrchestrator) applyWorkspacePVCFromSnapshot(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan) error {
	if err := validateSnapshotRefForNamespace(plan.Sandbox); err != nil {
		return err
	}
	return o.createPersistentVolumeClaims(ctx, cs, plan, snapshotSourceForDomain)
}

// createPersistentVolumeClaims 创建 adapter 声明的持久化卷域 PVC,可选按卷域从 VolumeSnapshot 恢复。
func (o *K8sOrchestrator) createPersistentVolumeClaims(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan, sourceFn func(Sandbox, VolumeDomainSpec) *corev1.TypedLocalObjectReference) error {
	for _, domain := range persistentVolumeDomains(plan.Runtime.AdapterSpec) {
		source := (*corev1.TypedLocalObjectReference)(nil)
		if sourceFn != nil && containsString(plan.Sandbox.SnapshotDomains, domain.Name) {
			source = sourceFn(plan.Sandbox, domain)
		}
		if err := o.createVolumeDomainPVC(ctx, cs, plan, domain, source); err != nil {
			return err
		}
	}
	return nil
}

// createVolumeDomainPVC 创建单个卷安全域 PVC,名称与 volume_domains.name 保持一致。
func (o *K8sOrchestrator) createVolumeDomainPVC(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan, domain VolumeDomainSpec, source *corev1.TypedLocalObjectReference) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      domain.Name,
			Namespace: plan.Sandbox.Namespace,
			Labels: map[string]string{
				"app":                   "chaimir",
				"module":                "sandbox",
				"chaimir.io/sandbox-id": fmt.Sprintf("%d", plan.Sandbox.ID),
				"chaimir.io/volume":     domain.Name,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(o.cfg.WorkspaceStorage)}},
		},
	}
	if strings.TrimSpace(o.cfg.StorageClassName) != "" {
		className := strings.TrimSpace(o.cfg.StorageClassName)
		pvc.Spec.StorageClassName = &className
	}
	if source != nil {
		pvc.Spec.DataSource = source
	}
	if _, err := cs.CoreV1().PersistentVolumeClaims(plan.Sandbox.Namespace).Create(ctx, pvc, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("创建沙箱卷 PVC 失败: %w", err)
	}
	return nil
}

// validateSnapshotRefForNamespace 校验快照组引用只能指向当前沙箱命名空间。
func validateSnapshotRefForNamespace(sb Sandbox) error {
	parts := strings.Split(strings.TrimSpace(sb.SnapshotRef), "/")
	if len(parts) != 2 || parts[0] != sb.Namespace || parts[1] != snapshotGroupName(sb) {
		return fmt.Errorf("快照引用不属于当前沙箱")
	}
	return nil
}

// podForPlan 构造主运行时容器和工具 sidecar Pod。
func (o *K8sOrchestrator) podsForPlan(plan CreateSandboxPlan) []*corev1.Pod {
	pods := podGroupForPlan(plan)
	out := make([]*corev1.Pod, 0, len(pods)+len(plan.Tools))
	for _, pod := range pods {
		out = append(out, o.podFromSpec(plan, pod))
	}
	for _, tool := range plan.Tools {
		if tool.Kind == SandboxToolKindWebEmbed || tool.Kind == SandboxToolKindCommand {
			for _, component := range tool.ResourceSpec.Components {
				out = append(out, o.toolPodForPlan(plan, tool, component))
			}
		}
	}
	return out
}

// podGroupForPlan 返回运行时声明的 Pod 组;未声明时按 runtime_container + infra_sidecars 生成单 Pod 拓扑。
func podGroupForPlan(plan CreateSandboxPlan) []workload.PodSpec {
	if len(plan.Runtime.AdapterSpec.Pods) > 0 {
		return plan.Runtime.AdapterSpec.Pods
	}
	specContainers := []workload.ComponentSpec{plan.Runtime.AdapterSpec.RuntimeContainer}
	specContainers[0].ImageURL = plan.Image.ImageURL
	for _, sidecar := range plan.Runtime.AdapterSpec.InfraSidecars {
		specContainers = append(specContainers, sidecar)
	}
	return []workload.PodSpec{{Name: "sandbox", Containers: specContainers}}
}

// podFromSpec 把运行时 Pod 拓扑转换为受限 Kubernetes Pod。
func (o *K8sOrchestrator) podFromSpec(plan CreateSandboxPlan, spec workload.PodSpec) *corev1.Pod {
	containers := make([]corev1.Container, 0, len(spec.Containers))
	for _, container := range spec.Containers {
		image := container.ImageURL
		if container.Name == plan.Runtime.AdapterSpec.RuntimeContainer.Name && image == "" {
			image = plan.Image.ImageURL
		}
		containers = append(containers, o.containerFromRuntime(container, image, plan.Runtime.AdapterSpec))
	}
	automount := false
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: plan.Sandbox.Namespace,
			Labels: map[string]string{
				"app":                   "chaimir",
				"module":                "sandbox",
				"chaimir.io/sandbox-id": fmt.Sprintf("%d", plan.Sandbox.ID),
				"chaimir.io/pod-role":   spec.Name,
			},
		},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: &automount,
			ServiceAccountName:           sandboxWorkloadServiceAccount,
			RestartPolicy:                corev1.RestartPolicyNever,
			SecurityContext:              podSecurityContext(),
			Containers:                   containers,
			NodeSelector:                 copyStringMap(o.cfg.SandboxNodeSelector),
			Tolerations:                  sandboxTolerations(o.cfg.SandboxNodeTolerations),
			Volumes:                      podVolumesForPlan(plan),
		},
	}
}

// toolPodForPlan 为 web-embed 工具组件创建独立 Pod,避免动态工具影响运行时 Pod 组拓扑。
func (o *K8sOrchestrator) toolPodForPlan(plan CreateSandboxPlan, tool Tool, component workload.ComponentSpec) *corev1.Pod {
	automount := false
	podName := toolComponentPodName(tool.Code, component.Name)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: plan.Sandbox.Namespace,
			Labels: map[string]string{
				"app":                   "chaimir",
				"module":                "sandbox",
				"chaimir.io/sandbox-id": fmt.Sprintf("%d", plan.Sandbox.ID),
				"chaimir.io/tool-code":  tool.Code,
				"chaimir.io/component":  component.Name,
				"chaimir.io/pod-role":   podName,
			},
		},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: &automount,
			ServiceAccountName:           sandboxWorkloadServiceAccount,
			RestartPolicy:                corev1.RestartPolicyNever,
			SecurityContext:              podSecurityContext(),
			Containers:                   []corev1.Container{o.containerFromTool(tool, component, plan.Runtime.AdapterSpec)},
			NodeSelector:                 copyStringMap(o.cfg.SandboxNodeSelector),
			Tolerations:                  sandboxTolerations(o.cfg.SandboxNodeTolerations),
			Volumes:                      podVolumesForTool(plan, tool, component),
		},
	}
}

// containerFromRuntime 构造运行时或 infra sidecar 容器。
func (o *K8sOrchestrator) containerFromRuntime(spec workload.ComponentSpec, image string, adapter AdapterSpec) corev1.Container {
	if image == "" {
		image = spec.ImageURL
	}
	return corev1.Container{
		Name:            spec.Name,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         spec.Command,
		Args:            spec.Args,
		WorkingDir:      spec.Workdir,
		Env:             envVars(spec.Env),
		Ports:           containerPorts(spec.Ports),
		Resources:       resources(spec.Resources, o.cfg),
		SecurityContext: containerSecurityContext(readOnlyRootFilesystem(spec.ReadOnlyRootFilesystem)),
		VolumeMounts:    volumeMountsForContainer(adapter, spec),
		ReadinessProbe:  probe(spec.ReadinessProbe),
		LivenessProbe:   probe(spec.LivenessProbe),
	}
}

// containerFromTool 构造 web-embed 工具组件容器。
func (o *K8sOrchestrator) containerFromTool(tool Tool, component workload.ComponentSpec, adapter AdapterSpec) corev1.Container {
	mounts := []corev1.VolumeMount{}
	if shouldMountWorkspace(component) {
		mounts = append(mounts, corev1.VolumeMount{Name: VolumeDomainWorkspace, MountPath: adapter.WorkspaceDir})
	}
	mounts = append(mounts, toolEphemeralVolumeMounts(tool, component)...)
	return corev1.Container{
		Name:            component.Name,
		Image:           component.ImageURL,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         component.Command,
		Args:            component.Args,
		WorkingDir:      component.Workdir,
		Env:             envVars(component.Env),
		Ports:           containerPorts(component.Ports),
		Resources:       resources(component.Resources, o.cfg),
		SecurityContext: containerSecurityContext(readOnlyRootFilesystem(component.ReadOnlyRootFilesystem)),
		VolumeMounts:    mounts,
		ReadinessProbe:  probe(component.ReadinessProbe),
		LivenessProbe:   probe(component.LivenessProbe),
	}
}

// podVolumesForPlan 汇总运行时卷域与工具临时卷,保证工具缓存不复用学生工作区或私有域。
func podVolumesForPlan(plan CreateSandboxPlan) []corev1.Volume {
	volumes := volumeDomains(plan.Runtime.AdapterSpec)
	return volumes
}

// podVolumesForTool 汇总工具需要的工作区 PVC 和私有临时卷。
func podVolumesForTool(plan CreateSandboxPlan, tool Tool, component workload.ComponentSpec) []corev1.Volume {
	volumes := []corev1.Volume{}
	if shouldMountWorkspace(component) {
		volumes = append(volumes, corev1.Volume{
			Name:         VolumeDomainWorkspace,
			VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: VolumeDomainWorkspace}},
		})
	}
	for _, mount := range component.EphemeralMounts {
		volumes = append(volumes, corev1.Volume{
			Name:         toolEphemeralVolumeName(tool.Code, component.Name, mount.Name),
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		})
	}
	return volumes
}

// toolEphemeralVolumeMounts 将工具声明的临时目录转换为只挂给该工具容器的 emptyDir。
func toolEphemeralVolumeMounts(tool Tool, component workload.ComponentSpec) []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0, len(component.EphemeralMounts))
	for _, mount := range component.EphemeralMounts {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      toolEphemeralVolumeName(tool.Code, component.Name, mount.Name),
			MountPath: path.Clean(mount.MountPath),
		})
	}
	return mounts
}

// toolEphemeralVolumeName 生成工具私有临时卷名,避免不同工具的缓存目录发生命名碰撞。
func toolEphemeralVolumeName(toolCode, componentName, mountName string) string {
	name := "tool-" + toolCode + "-" + componentName + "-" + mountName
	if len(name) <= 63 {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	suffix := hex.EncodeToString(sum[:4])
	base := "tool-" + toolCode
	maxBase := 63 - len(suffix) - 1
	if len(base) > maxBase {
		base = base[:maxBase]
	}
	return strings.TrimRight(base, "-") + "-" + suffix
}

// volumeDomains 为 adapter 声明的安全域创建 Pod 卷,运行态默认用临时卷防止进入学生代码持久化。
func volumeDomains(adapter AdapterSpec) []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(adapter.VolumeDomains))
	for _, domain := range adapter.VolumeDomains {
		volumes = append(volumes, volumeForDomain(domain))
	}
	return volumes
}

// volumeForDomain 按安全域持久化策略构造 Kubernetes 卷。
func volumeForDomain(domain VolumeDomainSpec) corev1.Volume {
	volume := corev1.Volume{Name: domain.Name}
	if domain.Name == VolumeDomainWorkspace || domain.Persistence == VolumePersistenceMinioCode || domain.Persistence == VolumePersistenceSnapshot {
		volume.VolumeSource = corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: domain.Name}}
		return volume
	}
	volume.VolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}
	return volume
}

// volumeMountsForContainer 按容器职责决定挂载域,防止终端或协同容器绕过文件 API 读取私有资产。
func volumeMountsForContainer(adapter AdapterSpec, spec workload.ComponentSpec) []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0, len(adapter.VolumeDomains))
	for _, domain := range adapter.VolumeDomains {
		if studentAccessibleContainer(spec) && domain.StudentAccess == VolumeAccessNone {
			continue
		}
		if domain.Name == VolumeDomainJudgePrivate && spec.Name != adapter.RuntimeContainer.Name {
			continue
		}
		mounts = append(mounts, volumeMountForDomain(domain))
	}
	return mounts
}

// volumeMountForDomain 根据卷域访问级别构造挂载,公开素材等只读域必须在 K8s 层强制只读。
func volumeMountForDomain(domain VolumeDomainSpec) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      domain.Name,
		MountPath: domain.MountPath,
		ReadOnly:  domain.StudentAccess == VolumeAccessReadOnly,
	}
}

// studentAccessibleContainer 判断容器是否声明允许学生通过终端进入。
func studentAccessibleContainer(spec workload.ComponentSpec) bool {
	return strings.EqualFold(strings.TrimSpace(spec.Labels[studentAccessLabel]), "true")
}

// persistentVolumeDomains 选出必须由 PVC 承载的卷域,确保 Pod 挂载和 PVC 创建口径一致。
func persistentVolumeDomains(adapter AdapterSpec) []VolumeDomainSpec {
	domains := make([]VolumeDomainSpec, 0, len(adapter.VolumeDomains))
	for _, domain := range adapter.VolumeDomains {
		if domain.Name == VolumeDomainWorkspace || domain.Persistence == VolumePersistenceMinioCode || domain.Persistence == VolumePersistenceSnapshot {
			domains = append(domains, domain)
		}
	}
	return domains
}

// snapshotDomainsForPlan 计算本次快照真实覆盖的 PVC 卷域,排除私有判题域和临时卷。
func snapshotDomainsForPlan(plan CreateSandboxPlan) []string {
	out := []string{}
	for _, domain := range persistentVolumeDomains(plan.Runtime.AdapterSpec) {
		if domain.Name == VolumeDomainJudgePrivate || domain.SnapshotScope == VolumeSnapshotNever {
			continue
		}
		if domain.SnapshotScope == VolumeSnapshotAlways || (plan.Sandbox.SnapshotEnabled && domain.SnapshotScope == VolumeSnapshotEnabled) {
			out = append(out, domain.Name)
		}
	}
	return out
}

// volumeSnapshotObject 构造单个卷域的 CSI VolumeSnapshot 对象。
func volumeSnapshotObject(sb Sandbox, domain, name string, retention time.Duration, snapshotClassName string) *unstructured.Unstructured {
	spec := map[string]any{
		"source": map[string]any{"persistentVolumeClaimName": domain},
	}
	if strings.TrimSpace(snapshotClassName) != "" {
		spec["volumeSnapshotClassName"] = strings.TrimSpace(snapshotClassName)
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "snapshot.storage.k8s.io/v1",
		"kind":       "VolumeSnapshot",
		"metadata": map[string]any{
			"name":      name,
			"namespace": sb.Namespace,
			"labels": map[string]any{
				"app":                   "chaimir",
				"module":                "sandbox",
				"chaimir.io/sandbox-id": fmt.Sprintf("%d", sb.ID),
				"chaimir.io/volume":     domain,
			},
			"annotations": map[string]any{
				"chaimir.io/retention-seconds": fmt.Sprintf("%.0f", retention.Seconds()),
			},
		},
		"spec": spec,
	}}
}

// snapshotGroupName 返回沙箱快照组引用名,具体卷域快照由同一前缀派生。
func snapshotGroupName(sb Sandbox) string {
	return fmt.Sprintf("snapshot-%d", sb.ID)
}

// volumeSnapshotName 返回单个卷域的 VolumeSnapshot 名称。
func volumeSnapshotName(sb Sandbox, domain string) string {
	return snapshotGroupName(sb) + "-" + domain
}

// snapshotSourceForDomain 构造 PVC 从同域 VolumeSnapshot 恢复的数据源引用。
func snapshotSourceForDomain(sb Sandbox, domain VolumeDomainSpec) *corev1.TypedLocalObjectReference {
	apiGroup := "snapshot.storage.k8s.io"
	return &corev1.TypedLocalObjectReference{APIGroup: &apiGroup, Kind: "VolumeSnapshot", Name: volumeSnapshotName(sb, domain.Name)}
}

// containsString 判断字符串列表是否包含目标值,用于快照域恢复白名单判断。
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// applyToolService 按工具 WorkloadSpec 创建仅集群内访问的 Service。
func (o *K8sOrchestrator) applyToolService(ctx context.Context, cs kubernetes.Interface, sb Sandbox, tool Tool) error {
	for _, service := range tool.ResourceSpec.Services {
		svc := o.toolServiceFor(sb, tool, service)
		if _, err := cs.CoreV1().Services(sb.Namespace).Create(ctx, svc, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("创建工具 Service 失败: %w", err)
		}
	}
	return nil
}

// applyRuntimeServices 为运行时 Pod 创建内部 ClusterIP,供声明式工具 NetworkPolicy 访问。
func (o *K8sOrchestrator) applyRuntimeServices(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan) error {
	for _, pod := range podGroupForPlan(plan) {
		ports := servicePortsForPod(pod)
		if len(ports) == 0 {
			continue
		}
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: runtimeServiceName(pod.Name), Namespace: plan.Sandbox.Namespace},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: sandboxPodRoleLabels(plan.Sandbox, pod.Name),
				Ports:    ports,
			},
		}
		if _, err := cs.CoreV1().Services(plan.Sandbox.Namespace).Create(ctx, svc, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("创建运行时 Service 失败: %w", err)
		}
	}
	return nil
}

// toolServiceFor 构造选择指定工具组件 Pod 的 ClusterIP Service。
func (o *K8sOrchestrator) toolServiceFor(sb Sandbox, tool Tool, service workload.ServiceSpec) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: service.Name, Namespace: sb.Namespace},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: sandboxPodRoleLabels(sb, toolComponentPodName(tool.Code, service.Component)),
			Ports:    servicePortsForSpec(service.Ports),
		},
	}
}

// sandboxPodRoleLabels 返回 NetworkPolicy 和 Service 共同使用的受控 Pod 选择标签。
func sandboxPodRoleLabels(sb Sandbox, role string) map[string]string {
	return map[string]string{
		"app":                   "chaimir",
		"module":                "sandbox",
		"chaimir.io/sandbox-id": fmt.Sprintf("%d", sb.ID),
		"chaimir.io/pod-role":   role,
	}
}

// servicePortsForPod 汇总运行时 Pod 中显式声明的端口,用于内部服务发现。
func servicePortsForPod(pod workload.PodSpec) []corev1.ServicePort {
	ports := []corev1.ServicePort{}
	seen := map[string]struct{}{}
	for _, container := range pod.Containers {
		for _, port := range container.Ports {
			if port.Name == "" || port.ContainerPort <= 0 {
				continue
			}
			if _, ok := seen[port.Name]; ok {
				continue
			}
			seen[port.Name] = struct{}{}
			ports = append(ports, corev1.ServicePort{Name: port.Name, Port: port.ContainerPort, TargetPort: intstr.FromString(port.Name)})
		}
	}
	return ports
}

// servicePortsForSpec 把 WorkloadSpec Service 端口转换为 Kubernetes Service 端口。
func servicePortsForSpec(items []workload.ServicePortSpec) []corev1.ServicePort {
	ports := make([]corev1.ServicePort, 0, len(items))
	for _, item := range items {
		protocol := corev1.ProtocolTCP
		if strings.EqualFold(item.Protocol, "UDP") {
			protocol = corev1.ProtocolUDP
		}
		ports = append(ports, corev1.ServicePort{
			Name:       item.Name,
			Port:       item.Port,
			TargetPort: intstr.FromString(item.TargetPort),
			Protocol:   protocol,
		})
	}
	return ports
}

// runtimeServiceName 生成运行时 Pod 的内部服务名,与工具 env 中的服务发现口径保持一致。
func runtimeServiceName(podName string) string {
	return "runtime-" + strings.TrimSpace(podName)
}

// toolControlPlanePorts 汇总平台代理路由实际需要访问的工具组件端口。
func toolControlPlanePorts(tool Tool) map[string][]netv1.NetworkPolicyPort {
	componentPorts := componentPortMap(tool.ResourceSpec.Components)
	serviceIndex := map[string]workload.ServiceSpec{}
	for _, service := range tool.ResourceSpec.Services {
		serviceIndex[service.Name] = service
	}
	rolePorts := map[string][]netv1.NetworkPolicyPort{}
	seen := map[string]map[int32]struct{}{}
	for _, route := range tool.ResourceSpec.Routes {
		service, ok := serviceIndex[route.Service]
		if !ok {
			continue
		}
		declared, ok := componentPorts[service.Component]
		if !ok {
			continue
		}
		for _, servicePort := range service.Ports {
			if servicePort.Name != route.Port {
				continue
			}
			port, ok := declared[servicePort.TargetPort]
			if !ok || port <= 0 {
				continue
			}
			role := toolComponentPodName(tool.Code, service.Component)
			if seen[role] == nil {
				seen[role] = map[int32]struct{}{}
			}
			if _, exists := seen[role][port]; exists {
				continue
			}
			seen[role][port] = struct{}{}
			rolePorts[role] = append(rolePorts[role], networkPolicyPort(port))
		}
	}
	return rolePorts
}

// toolComponentPodName 生成工具组件 Pod 名,避免回退到旧的单工具 Pod 命名。
func toolComponentPodName(toolCode, componentName string) string {
	name := "tool-" + strings.TrimSpace(toolCode) + "-" + strings.TrimSpace(componentName)
	if len(name) <= 63 {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	suffix := hex.EncodeToString(sum[:4])
	base := "tool-" + strings.TrimSpace(toolCode)
	maxBase := 63 - len(suffix) - 1
	if len(base) > maxBase {
		base = base[:maxBase]
	}
	return strings.TrimRight(base, "-") + "-" + suffix
}

// runtimePodNames 返回阶段一必须 Ready 的运行时 Pod 列表,不把动态 Web 工具失败混作链节点失败。
func runtimePodNames(plan CreateSandboxPlan) []string {
	pods := podGroupForPlan(plan)
	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Name)
	}
	return names
}

// splitExecTarget 把内部执行目标拆成 pod/container,未带 Pod 时使用单 Pod 拓扑名称。
func splitExecTarget(target string) (string, string) {
	parts := strings.Split(strings.TrimSpace(target), "/")
	if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return "sandbox", strings.TrimSpace(target)
}

// prepullDaemonSet 构造镜像预拉取 DaemonSet。
func (o *K8sOrchestrator) prepullDaemonSet(image RuntimeImage, imageURLs []string) *appsv1.DaemonSet {
	labels := map[string]string{"app": "chaimir", "module": "sandbox", "runtime_image_id": fmt.Sprintf("%d", image.ID)}
	automount := false
	containers := make([]corev1.Container, 0, len(imageURLs))
	for idx, imageURL := range imageURLs {
		containers = append(containers, corev1.Container{
			Name:            prepullContainerName(idx, imageURL),
			Image:           imageURL,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"sleep", fmt.Sprintf("%d", o.cfg.PrepullHoldSeconds)},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(o.cfg.PrepullRequestCPU), corev1.ResourceMemory: resource.MustParse(o.cfg.PrepullRequestMemory)},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(o.cfg.PrepullLimitCPU), corev1.ResourceMemory: resource.MustParse(o.cfg.PrepullLimitMemory)},
			},
			SecurityContext: containerSecurityContext(true),
		})
	}
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("chaimir-prepull-%d", image.ID), Namespace: o.cfg.PrepullNamespace, Labels: labels},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: &automount,
					RestartPolicy:                corev1.RestartPolicyAlways,
					NodeSelector:                 copyStringMap(o.cfg.SandboxNodeSelector),
					Tolerations:                  sandboxTolerations(o.cfg.SandboxNodeTolerations),
					Containers:                   containers,
					SecurityContext:              podSecurityContext(),
				},
			},
		},
	}
}

// prepullContainerName 基于镜像位置生成稳定容器名,避免多镜像预拉取时名称冲突。
func prepullContainerName(index int, imageURL string) string {
	sum := sha256.Sum256([]byte(imageURL))
	name := strings.NewReplacer("/", "-", ":", "-", "@", "-").Replace(strings.TrimSpace(imageURL))
	name = strings.ToLower(name)
	if len(name) > 36 {
		name = name[len(name)-36:]
	}
	name = strings.Trim(name, "-.")
	if name == "" {
		name = "image"
	}
	return fmt.Sprintf("prepull-%02d-%s-%s", index, name, hex.EncodeToString(sum[:3]))
}

// copyStringMap 复制调度标签映射,避免调用方持有的配置被 Kubernetes 对象修改。
func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

// sandboxTolerations 把平台配置转换为 Kubernetes toleration。
func sandboxTolerations(items []config.SandboxToleration) []corev1.Toleration {
	if len(items) == 0 {
		return nil
	}
	out := make([]corev1.Toleration, 0, len(items))
	for _, item := range items {
		operator := corev1.TolerationOpEqual
		if item.Operator == "Exists" {
			operator = corev1.TolerationOpExists
		}
		out = append(out, corev1.Toleration{
			Key:               item.Key,
			Operator:          operator,
			Value:             item.Value,
			Effect:            corev1.TaintEffect(item.Effect),
			TolerationSeconds: item.TolerationSeconds,
		})
	}
	return out
}

// podSecurityContext 构造 Pod Security Restricted 需要的 Pod 级安全上下文。
func podSecurityContext() *corev1.PodSecurityContext {
	runAsNonRoot := true
	runAsUser := int64(1000)
	runAsGroup := int64(1000)
	fsGroup := int64(1000)
	fsGroupPolicy := corev1.FSGroupChangeOnRootMismatch
	return &corev1.PodSecurityContext{
		RunAsNonRoot:        &runAsNonRoot,
		RunAsUser:           &runAsUser,
		RunAsGroup:          &runAsGroup,
		FSGroup:             &fsGroup,
		FSGroupChangePolicy: &fsGroupPolicy,
		SeccompProfile:      &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}

// containerSecurityContext 构造容器最小权限安全上下文。
func containerSecurityContext(readOnlyRoot bool) *corev1.SecurityContext {
	allow := false
	privileged := false
	return &corev1.SecurityContext{
		Privileged:               &privileged,
		AllowPrivilegeEscalation: &allow,
		ReadOnlyRootFilesystem:   &readOnlyRoot,
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}
}

// readOnlyRootFilesystem 默认保持只读根文件系统,仅当声明式镜像规格明确关闭时放开。
func readOnlyRootFilesystem(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}

// envVars 转换非敏感字面量环境变量。
func envVars(items []workload.EnvVarSpec) []corev1.EnvVar {
	out := make([]corev1.EnvVar, 0, len(items))
	for _, item := range items {
		out = append(out, corev1.EnvVar{Name: item.Name, Value: item.Value})
	}
	return out
}

// containerPorts 转换容器端口声明。
func containerPorts(items []workload.PortSpec) []corev1.ContainerPort {
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

// resources 转换容器资源 requests/limits,缺省时使用平台默认值。
func resources(spec workload.ResourceSpec, cfg config.SandboxConfig) corev1.ResourceRequirements {
	reqCPU := valueOrDefault(spec.Requests["cpu"], cfg.DefaultReqCPU)
	reqMem := valueOrDefault(spec.Requests["memory"], cfg.DefaultReqMemory)
	limCPU := valueOrDefault(spec.Limits["cpu"], cfg.DefaultCPU)
	limMem := valueOrDefault(spec.Limits["memory"], cfg.DefaultMemory)
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(reqCPU), corev1.ResourceMemory: resource.MustParse(reqMem)},
		Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(limCPU), corev1.ResourceMemory: resource.MustParse(limMem)},
	}
}

// addResourceUsage 把单容器资源声明累加到沙箱资源摘要。
func addResourceUsage(usage *contracts.SandboxResourceUsage, resources corev1.ResourceRequirements) {
	if cpu, ok := resources.Requests[corev1.ResourceCPU]; ok {
		usage.CPURequestMilli += cpu.MilliValue()
	}
	if cpu, ok := resources.Limits[corev1.ResourceCPU]; ok {
		usage.CPULimitMilli += cpu.MilliValue()
	}
	if memory, ok := resources.Requests[corev1.ResourceMemory]; ok {
		usage.MemoryRequestMiB += bytesToMiB(memory.Value())
	}
	if memory, ok := resources.Limits[corev1.ResourceMemory]; ok {
		usage.MemoryLimitMiB += bytesToMiB(memory.Value())
	}
}

// addPodMetricsUsage 把 metrics-server 返回的实时容器用量累加到沙箱资源摘要。
func addPodMetricsUsage(usage *contracts.SandboxResourceUsage, metrics []metricsv1beta1.PodMetrics) {
	for _, pod := range metrics {
		for _, container := range pod.Containers {
			if cpu, ok := container.Usage[corev1.ResourceCPU]; ok {
				usage.CPUUsageMilli += cpu.MilliValue()
			}
			if memory, ok := container.Usage[corev1.ResourceMemory]; ok {
				usage.MemoryUsageMiB += bytesToMiB(memory.Value())
			}
		}
	}
}

// bytesToMiB 将 Kubernetes 字节数转换为 MiB,便于前端展示和配额对比。
func bytesToMiB(value int64) int64 {
	const mib = 1024 * 1024
	return value / mib
}

// probe 转换声明式探针。
func probe(spec workload.ProbeSpec) *corev1.Probe {
	if spec.Type == "" {
		return nil
	}
	p := &corev1.Probe{PeriodSeconds: spec.PeriodSeconds, FailureThreshold: spec.FailureThreshold}
	switch spec.Type {
	case "tcp":
		p.ProbeHandler = corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(spec.Port)}}
	case "http":
		p.ProbeHandler = corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: spec.Path, Port: intstr.FromString(spec.Port)}}
	case "exec":
		p.ProbeHandler = corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: spec.Command}}
	}
	return p
}

// networkPolicyPort 构造 TCP 网络策略端口。
func networkPolicyPort(port int32) netv1.NetworkPolicyPort {
	protocol := corev1.ProtocolTCP
	return netv1.NetworkPolicyPort{Protocol: &protocol, Port: &intstr.IntOrString{Type: intstr.Int, IntVal: port}}
}

// dnsNetworkPolicyPorts 返回 Kubernetes DNS 所需的 UDP/TCP 端口集合。
func dnsNetworkPolicyPorts(port int32) []netv1.NetworkPolicyPort {
	udp := corev1.ProtocolUDP
	tcp := corev1.ProtocolTCP
	return []netv1.NetworkPolicyPort{
		{Protocol: &udp, Port: &intstr.IntOrString{Type: intstr.Int, IntVal: port}},
		{Protocol: &tcp, Port: &intstr.IntOrString{Type: intstr.Int, IntVal: port}},
	}
}

// networkPolicyPortsForPodGroup 汇总运行时 Pod 组声明端口,用于控制面精确访问。
func networkPolicyPortsForPodGroup(pods []workload.PodSpec) []netv1.NetworkPolicyPort {
	ports := []netv1.NetworkPolicyPort{}
	seen := map[int32]struct{}{}
	for _, pod := range pods {
		for _, container := range pod.Containers {
			for _, port := range container.Ports {
				if _, ok := seen[port.ContainerPort]; ok {
					continue
				}
				seen[port.ContainerPort] = struct{}{}
				ports = append(ports, networkPolicyPort(port.ContainerPort))
			}
		}
	}
	return ports
}

// networkPolicyPortsForSinglePod 复用 Pod 组端口汇总逻辑,用于控制面对单个运行时 Pod 的精确放行。
func networkPolicyPortsForSinglePod(pod workload.PodSpec) []netv1.NetworkPolicyPort {
	return networkPolicyPortsForPodGroup([]workload.PodSpec{pod})
}

// networkPolicyPortsForRefs 把 adapter 显式网络规则端口转换为 K8s NetworkPolicy 端口。
func networkPolicyPortsForRefs(refs []workload.NetworkPortRef) []netv1.NetworkPolicyPort {
	ports := make([]netv1.NetworkPolicyPort, 0, len(refs))
	for _, ref := range refs {
		ports = append(ports, networkPolicyPort(ref.Port))
	}
	return ports
}

// valueOrDefault 返回非空配置值,未声明资源时使用平台统一默认值。
func valueOrDefault(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
