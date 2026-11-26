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
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/ids"
	platformk8s "chaimir/internal/platform/k8s"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/limitio"
	"chaimir/pkg/logging"
	"chaimir/pkg/terminal"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	netv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var errMetricsUnavailable = errors.New("kubernetes metrics api unavailable")

const sandboxWorkloadServiceAccount = "sandbox-workload"

const runtimeSelftestNamespacePrefix = "sbx-selftest-"
const selftestCleanupAfterAnnotation = "chaimir.io/selftest-cleanup-after"

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
func (o *K8sOrchestrator) CreateSandboxResources(ctx context.Context, plan CreateSandboxPlan, archives []RuntimeInitArchive) error {
	orderedPods, err := o.podsForPlan(plan)
	if err != nil {
		return err
	}
	services, err := servicesForPlan(plan)
	if err != nil {
		return err
	}
	assetPods, err := initAssetPodsForPlan(plan, orderedPods, archives)
	if err != nil {
		return err
	}
	cs := o.client.Clientset()
	ns := namespaceObject(plan.Sandbox.Namespace, plan.Sandbox)
	if plan.Sandbox.TenantID == 0 {
		deadline, ok := ctx.Deadline()
		if !ok || o.cfg.ReadyTimeoutSeconds <= 0 || o.cfg.SelftestRecycleTimeoutSeconds <= 0 {
			return fmt.Errorf("自检资源缺少有效执行或清理预算")
		}
		// 资产 Pod 的失败清理可能超出执行窗口，之后还须为整个 Namespace 的清理留出预算。
		cleanupAfter := deadline.Add(timeDurationSeconds(o.cfg.ReadyTimeoutSeconds)).Add(timeDurationSeconds(o.cfg.SelftestRecycleTimeoutSeconds))
		ns.Annotations = map[string]string{selftestCleanupAfterAnnotation: cleanupAfter.UTC().Format(time.RFC3339Nano)}
	}
	if err := createOrVerifyNamespace(ctx, cs, ns); err != nil {
		return fmt.Errorf("创建沙箱 Namespace 失败: %w", err)
	}
	if err := platformk8s.WaitForSandboxBackendRBAC(ctx, cs, plan.Sandbox.Namespace, platformk8s.SandboxBackendServiceAccountNamespace, platformk8s.SandboxBackendServiceAccountName, time.Duration(o.cfg.ReadyTimeoutSeconds)*time.Second, time.Duration(o.cfg.ReadyPollIntervalSeconds)*time.Second); err != nil {
		return err
	}
	if err := o.ensureImagePullSecrets(ctx, plan.Sandbox.Namespace); err != nil {
		return err
	}
	if err := o.ensureRuntimeSecrets(ctx, plan); err != nil {
		return err
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
	if err := o.initializeRuntimeAssets(ctx, cs, assetPods, o.Exec); err != nil {
		return err
	}
	// 依赖消费者启动前，先创建完整组合的服务发现入口。
	for _, service := range services {
		if err := createOrVerifyService(ctx, cs, service); err != nil {
			return fmt.Errorf("创建沙箱 Service %s 失败: %w", service.Name, err)
		}
	}
	if err := createPodsInStartupOrder(plan, orderedPods, func(pod *corev1.Pod) error {
		if err := createOrVerifyPod(ctx, cs, pod); err != nil {
			return fmt.Errorf("创建沙箱 Pod 失败: %w", err)
		}
		return nil
	}, func(names []string) error {
		return o.waitPodsReady(ctx, cs, plan.Sandbox.Namespace, names)
	}); err != nil {
		return err
	}
	return o.waitPodsReady(ctx, cs, plan.Sandbox.Namespace, runtimePodNames(plan, orderedPods))
}

// DestroySandboxResources 删除普通沙箱资源。
func (o *K8sOrchestrator) DestroySandboxResources(ctx context.Context, sb Sandbox) error {
	if err := o.client.Clientset().CoreV1().Namespaces().Delete(ctx, sb.Namespace, namespaceDeleteOptions()); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除沙箱 Namespace 失败: %w", err)
	}
	return waitNamespaceDeleted(ctx, o.client.Clientset(), sb.Namespace, o.cfg.ReadyPollIntervalSeconds)
}

// CleanupExpiredRuntimeSelftests 清理请求中断后遗留的运行时自检 Namespace。
// 仅接受后端创建的 tenant=0 自检命名空间，不能把普通租户沙箱作为补偿目标。
func (o *K8sOrchestrator) CleanupExpiredRuntimeSelftests(ctx context.Context) error {
	return cleanupExpiredRuntimeSelftestNamespaces(ctx, o.client.Clientset(), o.cfg.ReadyPollIntervalSeconds, time.Now())
}

func cleanupExpiredRuntimeSelftestNamespaces(ctx context.Context, cs kubernetes.Interface, pollSeconds int, now time.Time) error {
	if cs == nil || pollSeconds <= 0 {
		return fmt.Errorf("运行时自检补偿清理配置不正确")
	}
	namespaces, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{LabelSelector: "app=chaimir,module=sandbox,chaimir.io/managed-by=chaimir-backend,chaimir.io/sandbox=true,chaimir.io/tenant-id=0"})
	if err != nil {
		return fmt.Errorf("列出运行时自检 Namespace 失败: %w", err)
	}
	var cleanupErrs []error
	for index := range namespaces.Items {
		namespace := &namespaces.Items[index]
		expired, err := runtimeSelftestNamespaceExpired(namespace, now)
		if err != nil {
			cleanupErrs = append(cleanupErrs, err)
			continue
		}
		if !expired {
			continue
		}
		if err := cs.CoreV1().Namespaces().Delete(ctx, namespace.Name, namespaceDeleteOptions()); err != nil && !apierrors.IsNotFound(err) {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("删除过期运行时自检 Namespace(%s)失败: %w", namespace.Name, err))
			continue
		}
		if err := waitNamespaceDeleted(ctx, cs, namespace.Name, pollSeconds); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("等待过期运行时自检 Namespace(%s)删除失败: %w", namespace.Name, err))
		}
	}
	return errors.Join(cleanupErrs...)
}

// runtimeSelftestNamespaceExpired 严格匹配平台自检资源，防止后台任务清理到普通租户沙箱。
func runtimeSelftestNamespaceExpired(namespace *corev1.Namespace, now time.Time) (bool, error) {
	if namespace == nil || namespace.CreationTimestamp.IsZero() || namespace.DeletionTimestamp != nil {
		return false, nil
	}
	name := strings.TrimSpace(namespace.Name)
	if !strings.HasPrefix(name, runtimeSelftestNamespacePrefix) {
		return false, nil
	}
	sandboxID := strings.TrimPrefix(name, runtimeSelftestNamespacePrefix)
	labels := namespace.Labels
	if sandboxID == "" || labels["app"] != "chaimir" || labels["module"] != "sandbox" || labels["chaimir.io/managed-by"] != "chaimir-backend" || labels["chaimir.io/sandbox"] != "true" || labels["chaimir.io/tenant-id"] != "0" || labels["chaimir.io/sandbox-id"] != sandboxID {
		return false, nil
	}
	cleanupAfter, err := time.Parse(time.RFC3339Nano, namespace.Annotations[selftestCleanupAfterAnnotation])
	if err != nil {
		return false, fmt.Errorf("自检 Namespace %s 清理期限缺失或无效: %w", name, err)
	}
	if !cleanupAfter.After(namespace.CreationTimestamp.Time) {
		return false, fmt.Errorf("自检 Namespace %s 清理期限早于资源创建", name)
	}
	return !cleanupAfter.After(now), nil
}

// namespaceDeleteOptions 使用后台级联触发命名空间回收;调用方仍需等待对象消失,保证业务状态不先行。
func namespaceDeleteOptions() metav1.DeleteOptions {
	policy := metav1.DeletePropagationBackground
	return metav1.DeleteOptions{PropagationPolicy: &policy}
}

// StopComputeKeepSnapshot 释放计算工作负载但保留快照命名空间和 PVC。
func (o *K8sOrchestrator) StopComputeKeepSnapshot(ctx context.Context, sb Sandbox) error {
	if err := o.StopSandboxCompute(ctx, sb); err != nil {
		return err
	}
	if sb.SnapshotRef != "" {
		if err := o.cleanupSupersededSnapshots(ctx, sb); err != nil {
			return err
		}
	}
	return o.cleanupSnapshotRetainedNamespace(ctx, o.client.Clientset(), sb)
}

// StopSandboxCompute 等待所有计算 Pod 正常退出后再允许归档或快照。
func (o *K8sOrchestrator) StopSandboxCompute(ctx context.Context, sb Sandbox) error {
	ctx, cancel := context.WithTimeout(ctx, timeDurationSeconds(o.cfg.ReadyTimeoutSeconds))
	defer cancel()
	cs := o.client.Clientset()
	selector := "chaimir.io/sandbox-id=" + ids.Format(sb.ID)
	pods, err := cs.CoreV1().Pods(sb.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("列出快照沙箱计算 Pod 失败: %w", err)
	}
	if err := cs.CoreV1().Pods(sb.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: selector}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除快照沙箱计算 Pod 失败: %w", err)
	}
	for _, pod := range pods.Items {
		if err := waitPodDeleted(ctx, cs, sb.Namespace, pod.Name, o.cfg.ReadyPollIntervalSeconds); err != nil {
			return err
		}
	}
	return nil
}

// CreateSnapshot 创建 CSI VolumeSnapshot 并返回 namespaced 引用。
func (o *K8sOrchestrator) CreateSnapshot(ctx context.Context, plan CreateSandboxPlan, retention time.Duration) (result SnapshotResult, resultErr error) {
	if plan.Sandbox.resourceLeaseToken == "" || retention <= 0 {
		return SnapshotResult{}, fmt.Errorf("快照必须由有效资源执行者创建并设置保留期限")
	}
	attempt := sha256.Sum256([]byte(plan.Sandbox.resourceLeaseToken))
	plan.Sandbox.SnapshotRef = plan.Sandbox.Namespace + "/snapshot-" + ids.Format(plan.Sandbox.ID) + "-" + hex.EncodeToString(attempt[:16])
	domains := snapshotDomainsForPlan(plan)
	if len(domains) == 0 {
		return SnapshotResult{}, fmt.Errorf("沙箱没有可快照卷域")
	}
	gvr := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	client := o.client.Dynamic().Resource(gvr).Namespace(plan.Sandbox.Namespace)
	var created []*unstructured.Unstructured
	defer func() {
		if resultErr == nil || len(created) == 0 {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeDurationSeconds(o.cfg.ReadyTimeoutSeconds))
		defer cancel()
		for _, snapshot := range created {
			uid := snapshot.GetUID()
			if err := client.Delete(cleanupCtx, snapshot.GetName(), metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); err != nil && !apierrors.IsNotFound(err) {
				resultErr = errors.Join(resultErr, fmt.Errorf("清理未完成快照 %s 失败: %w", snapshot.GetName(), err))
			}
		}
	}()
	for _, domain := range domains {
		name := volumeSnapshotName(plan.Sandbox, domain)
		obj := volumeSnapshotObject(plan.Sandbox, domain, name, retention, o.cfg.VolumeSnapshotClassName)
		snapshot, err := createOrVerifyVolumeSnapshot(ctx, client, obj)
		if err != nil {
			return SnapshotResult{}, fmt.Errorf("创建 VolumeSnapshot 失败: %w", err)
		}
		if snapshot != nil {
			created = append(created, snapshot)
		}
		if err := o.waitVolumeSnapshotReady(ctx, gvr, plan.Sandbox.Namespace, name); err != nil {
			return SnapshotResult{}, err
		}
	}
	return SnapshotResult{Ref: plan.Sandbox.Namespace + "/" + snapshotGroupName(plan.Sandbox), Domains: domains}, nil
}

func (o *K8sOrchestrator) cleanupSupersededSnapshots(ctx context.Context, sb Sandbox) error {
	if err := validateSnapshotRefForNamespace(sb); err != nil {
		return err
	}
	gvr := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	client := o.client.Dynamic().Resource(gvr).Namespace(sb.Namespace)
	snapshots, err := client.List(ctx, metav1.ListOptions{LabelSelector: "app=chaimir,module=sandbox,chaimir.io/sandbox-id=" + ids.Format(sb.ID)})
	if err != nil {
		return fmt.Errorf("列出旧快照失败: %w", err)
	}
	for _, snapshot := range snapshots.Items {
		labels := snapshot.GetLabels()
		domain := labels["chaimir.io/volume"]
		if labels["app"] != "chaimir" || labels["module"] != "sandbox" || labels["chaimir.io/sandbox-id"] != ids.Format(sb.ID) || domain == "" {
			continue
		}
		if snapshot.GetName() == volumeSnapshotName(sb, domain) {
			continue
		}
		uid := snapshot.GetUID()
		if err := client.Delete(ctx, snapshot.GetName(), metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("删除旧快照 %s 失败: %w", snapshot.GetName(), err)
		}
	}
	return nil
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
		contentName, _, err := unstructured.NestedString(current.Object, "status", "boundVolumeSnapshotContentName")
		if err != nil {
			return fmt.Errorf("解析 VolumeSnapshot 绑定内容失败: %w", err)
		}
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

// volumeSnapshotReadyAndBound 校验 Ready 与双向绑定，UID 防止复用同名旧快照的内容。
func volumeSnapshotReadyAndBound(snapshot, content *unstructured.Unstructured, namespace, name string) bool {
	if snapshot == nil || content == nil {
		return false
	}
	ready, _, err := unstructured.NestedBool(snapshot.Object, "status", "readyToUse")
	if err != nil {
		return false
	}
	if !ready {
		return false
	}
	contentName, _, err := unstructured.NestedString(snapshot.Object, "status", "boundVolumeSnapshotContentName")
	if err != nil {
		return false
	}
	if strings.TrimSpace(contentName) == "" || content.GetName() != contentName {
		return false
	}
	refName, _, err := unstructured.NestedString(content.Object, "spec", "volumeSnapshotRef", "name")
	if err != nil {
		return false
	}
	refNamespace, _, err := unstructured.NestedString(content.Object, "spec", "volumeSnapshotRef", "namespace")
	if err != nil {
		return false
	}
	refUID, _, err := unstructured.NestedString(content.Object, "spec", "volumeSnapshotRef", "uid")
	if err != nil {
		return false
	}
	return refName == name && refNamespace == namespace && refUID != "" && refUID == string(snapshot.GetUID())
}

// ResourceUsage 汇总沙箱 Pod 与工作区 PVC 的资源申请和限制。
func (o *K8sOrchestrator) ResourceUsage(ctx context.Context, sb Sandbox) (contracts.SandboxResourceUsage, error) {
	cs := o.client.Clientset()
	usage := contracts.SandboxResourceUsage{}
	metrics, err := o.client.Metrics().MetricsV1beta1().PodMetricses(sb.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "chaimir.io/sandbox-id=" + ids.Format(sb.ID),
	})
	if err != nil {
		if unavailable := metricsUnavailableError(err); unavailable != nil {
			return usage, unavailable
		}
		return usage, fmt.Errorf("查询沙箱实时资源用量失败: %w", err)
	}
	pods, err := cs.CoreV1().Pods(sb.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "chaimir.io/sandbox-id=" + ids.Format(sb.ID),
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
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		found := false
		for _, sample := range metrics.Items {
			if sample.Name == pod.Name {
				// 采样必须覆盖所有仍在运行的容器，未采样不能当成真实零用量。
				for _, state := range pod.Status.ContainerStatuses {
					if state.State.Running == nil {
						continue
					}
					matched := false
					for _, container := range sample.Containers {
						if container.Name == state.Name {
							cpu, hasCPU := container.Usage[corev1.ResourceCPU]
							memory, hasMemory := container.Usage[corev1.ResourceMemory]
							if !hasCPU || !hasMemory {
								return contracts.SandboxResourceUsage{}, fmt.Errorf("运行容器资源采样缺少CPU或内存: pod=%s container=%s", pod.Name, state.Name)
							}
							matched = true
							usage.CPUUsageMilli += cpu.MilliValue()
							usage.MemoryUsageMiB += bytesToMiB(memory.Value())
							break
						}
					}
					if !matched {
						return contracts.SandboxResourceUsage{}, fmt.Errorf("运行容器尚无完整资源采样: pod=%s container=%s", pod.Name, state.Name)
					}
				}
				found = true
				break
			}
		}
		if !found {
			return contracts.SandboxResourceUsage{}, fmt.Errorf("运行环境尚无资源采样: pod=%s", pod.Name)
		}
	}
	pvcs, err := cs.CoreV1().PersistentVolumeClaims(sb.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "chaimir.io/sandbox-id=" + ids.Format(sb.ID),
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

// PrepareWorkspaceArchive 不重启业务计算，只用冻结工作区镜像挂载原 PVC。
func (o *K8sOrchestrator) PrepareWorkspaceArchive(ctx context.Context, plan CreateSandboxPlan) error {
	if plan.Sandbox.resourceLeaseToken == "" || !plan.Sandbox.resourceLeaseUntil.After(timex.Now()) {
		return errSandboxResourceLeaseLost
	}
	runtime, ok := plan.RuntimeInstance(plan.WorkspaceRuntimeInstance)
	if !ok {
		return fmt.Errorf("归档工作区运行时未声明")
	}
	podName, containerName := splitExecTarget(workspaceExecTarget(runtime.Runtime))
	var component workload.ComponentSpec
	for _, pod := range podGroupForRuntime(runtime.Runtime) {
		if pod.Name != podName {
			continue
		}
		for _, container := range pod.Containers {
			if container.Name == containerName {
				component = container
			}
		}
	}
	if component.ImageURL == "" || !studentAccessibleContainer(component) || !shouldMountWorkspace(component) {
		return fmt.Errorf("归档工作区镜像或执行目标未声明")
	}
	cs := o.client.Clientset()
	pvc, err := cs.CoreV1().PersistentVolumeClaims(plan.Sandbox.Namespace).Get(ctx, VolumeDomainWorkspace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("读取原工作区卷失败: %w", err)
	}
	expectedPVC := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: VolumeDomainWorkspace, Labels: map[string]string{
		"app": "chaimir", "module": "sandbox", "chaimir.io/sandbox-id": ids.Format(plan.Sandbox.ID), "chaimir.io/volume": VolumeDomainWorkspace,
	}}}
	if err := verifyManagedLabels(pvc, expectedPVC); err != nil {
		return err
	}
	if err := o.StopSandboxCompute(ctx, plan.Sandbox); err != nil {
		return err
	}
	if err := o.ensureImagePullSecrets(ctx, plan.Sandbox.Namespace); err != nil {
		return err
	}
	if err := o.applySandboxServiceAccount(ctx, cs, plan); err != nil {
		return err
	}
	podName, containerName = splitExecTarget(workspaceArchiveExecTarget(plan.Sandbox))
	automount, serviceLinks := false, false
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: plan.Sandbox.Namespace, Labels: map[string]string{
			"app": "chaimir", "module": "sandbox", "chaimir.io/sandbox-id": ids.Format(plan.Sandbox.ID),
		}},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: &automount, EnableServiceLinks: &serviceLinks,
			RuntimeClassName: &o.cfg.RuntimeClass, ServiceAccountName: sandboxWorkloadServiceAccount,
			RestartPolicy: corev1.RestartPolicyNever, SecurityContext: podSecurityContext(),
			NodeSelector: copyStringMap(o.cfg.SandboxNodeSelector), Tolerations: sandboxTolerations(o.cfg.SandboxNodeTolerations),
			Containers: []corev1.Container{{
				Name: containerName, Image: component.ImageURL, ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{"sleep", "2147483647"}, Resources: resources(component.Resources, o.cfg),
				SecurityContext: containerSecurityContext(true),
				VolumeMounts:    []corev1.VolumeMount{{Name: VolumeDomainWorkspace, MountPath: runtime.Runtime.AdapterSpec.WorkspaceDir}},
			}},
			Volumes: []corev1.Volume{{Name: VolumeDomainWorkspace, VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: VolumeDomainWorkspace},
			}}},
		},
	}
	if err := createOrVerifyPod(ctx, cs, pod); err != nil {
		return fmt.Errorf("创建工作区归档 Pod 失败: %w", err)
	}
	return o.waitPodsReady(ctx, cs, plan.Sandbox.Namespace, []string{podName})
}

func workspaceArchiveExecTarget(sb Sandbox) string {
	attempt := sha256.Sum256([]byte(sb.resourceLeaseToken))
	return "archive-" + ids.Format(sb.ID) + "-" + hex.EncodeToString(attempt[:16]) + "/archive"
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

// Exec 在沙箱容器中执行受控命令,stdout/stderr 分别受配置上限约束。
func (o *K8sOrchestrator) Exec(ctx context.Context, namespace, container string, command []string, stdin []byte, tty bool) ([]byte, []byte, error) {
	if o.cfg.ExecOutputMaxBytes <= 0 {
		return nil, nil, fmt.Errorf("SANDBOX_EXEC_OUTPUT_MAX_BYTES 未配置")
	}
	podName, containerName := splitExecTarget(container)
	stdout := limitio.NewBuffer(o.cfg.ExecOutputMaxBytes)
	stderr := limitio.NewBuffer(o.cfg.ExecOutputMaxBytes)
	err := o.client.Exec(ctx, namespace, podName, containerName, command, execInputReader(stdin), stdout, stderr, tty)
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
func (o *K8sOrchestrator) ExecStream(ctx context.Context, namespace, container string, command []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, tty bool, sizes terminal.SizeQueue) error {
	podName, containerName := splitExecTarget(container)
	return o.client.ExecStream(ctx, namespace, podName, containerName, command, stdin, stdout, stderr, tty, sizes)
}

// PrepullImage 创建或更新预拉取 DaemonSet 并等待工作负载镜像集合在真实节点 Ready。
func (o *K8sOrchestrator) PrepullImage(ctx context.Context, image RuntimeImage, compositionDigest string, specs []PrepullImageSpec) (PrepullResult, error) {
	imageURLs := prepullSpecImageURLs(specs)
	ds := o.prepullDaemonSet(image, compositionDigest, specs)
	cs := o.client.Clientset()
	if err := o.ensureImagePullSecrets(ctx, o.cfg.PrepullNamespace); err != nil {
		return PrepullResult{DaemonSet: ds.Name}, err
	}
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
		detail, err := jsonBytes(map[string]any{
			"desired_nodes":       current.Status.DesiredNumberScheduled,
			"ready_nodes":         current.Status.NumberReady,
			"daemonset":           ds.Name,
			"generation":          current.Generation,
			"observed_generation": current.Status.ObservedGeneration,
			"image_count":         len(imageURLs),
			"images":              imageURLs,
		})
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
		if err := containerRuntimeFailureFromPods(pods.Items); err != nil {
			detail, encodeErr := jsonBytes(map[string]any{"error": logging.SanitizeError(err.Error()), "daemonset": ds.Name})
			if encodeErr != nil {
				return result, fmt.Errorf("编码预拉取失败详情失败: %w", encodeErr)
			}
			result.Detail = detail
			return result, err
		}
		if prepullDaemonSetReady(current) {
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

// prepullDaemonSetReady 只接受控制器已观察当前模板且全部目标节点运行当前版本的 DaemonSet。
func prepullDaemonSetReady(ds *appsv1.DaemonSet) bool {
	if ds == nil || ds.Status.ObservedGeneration < ds.Generation || ds.Status.DesiredNumberScheduled <= 0 {
		return false
	}
	desired := ds.Status.DesiredNumberScheduled
	return ds.Status.UpdatedNumberScheduled == desired &&
		ds.Status.NumberReady == desired &&
		ds.Status.NumberAvailable == desired &&
		ds.Status.NumberUnavailable == 0 &&
		ds.Status.NumberMisscheduled == 0
}

// DeletePrepullDaemonSet 删除镜像预拉取 DaemonSet,NotFound 视为幂等成功。
func (o *K8sOrchestrator) DeletePrepullDaemonSet(ctx context.Context, image RuntimeImage) error {
	client := o.client.Clientset().AppsV1().DaemonSets(o.cfg.PrepullNamespace)
	list, err := client.List(ctx, metav1.ListOptions{LabelSelector: "runtime_image_id=" + ids.Format(image.ID)})
	if err != nil {
		return fmt.Errorf("查询镜像预拉取 DaemonSet 失败: %w", err)
	}
	for _, item := range list.Items {
		if err := client.Delete(ctx, item.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("删除预拉取 DaemonSet 失败: %w", err)
		}
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
		if err := imagePullFailureFromContainerStatuses(pod.Name, pod.Status.InitContainerStatuses, failureReasons); err != nil {
			return err
		}
		if err := imagePullFailureFromContainerStatuses(pod.Name, pod.Status.ContainerStatuses, failureReasons); err != nil {
			return err
		}
	}
	return nil
}

// imagePullFailureFromContainerStatuses 从容器状态中提取镜像拉取失败原因。
func imagePullFailureFromContainerStatuses(podName string, statuses []corev1.ContainerStatus, failureReasons map[string]struct{}) error {
	for _, status := range statuses {
		if status.State.Waiting == nil {
			continue
		}
		reason := strings.TrimSpace(status.State.Waiting.Reason)
		if _, failed := failureReasons[reason]; !failed {
			continue
		}
		message := strings.TrimSpace(status.State.Waiting.Message)
		if message == "" {
			return fmt.Errorf("预拉取 Pod %s 容器 %s 镜像拉取失败: %s", podName, status.Name, reason)
		}
		return fmt.Errorf("预拉取 Pod %s 容器 %s 镜像拉取失败: %s: %s", podName, status.Name, reason, message)
	}
	return nil
}

// containerRuntimeFailureFromPods 从预拉取 Pod 状态中识别自检命令或容器启动失败。
func containerRuntimeFailureFromPods(pods []corev1.Pod) error {
	failureReasons := map[string]struct{}{
		"CreateContainerConfigError": {},
		"RunContainerError":          {},
		"CrashLoopBackOff":           {},
	}
	for _, pod := range pods {
		if err := containerRuntimeFailureFromContainerStatuses(pod.Name, pod.Status.InitContainerStatuses, failureReasons); err != nil {
			return err
		}
		if err := containerRuntimeFailureFromContainerStatuses(pod.Name, pod.Status.ContainerStatuses, failureReasons); err != nil {
			return err
		}
	}
	return nil
}

// containerRuntimeFailureFromContainerStatuses 从容器当前状态中提取启动或自检失败原因。
func containerRuntimeFailureFromContainerStatuses(podName string, statuses []corev1.ContainerStatus, failureReasons map[string]struct{}) error {
	for _, status := range statuses {
		if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
			terminated := status.State.Terminated
			reason := strings.TrimSpace(terminated.Reason)
			message := strings.TrimSpace(terminated.Message)
			if message == "" {
				return fmt.Errorf("预拉取 Pod %s 容器 %s 异常退出: %s exit=%d", podName, status.Name, reason, terminated.ExitCode)
			}
			return fmt.Errorf("预拉取 Pod %s 容器 %s 异常退出: %s exit=%d: %s", podName, status.Name, reason, terminated.ExitCode, message)
		}
		if status.State.Waiting == nil {
			continue
		}
		reason := strings.TrimSpace(status.State.Waiting.Reason)
		if _, failed := failureReasons[reason]; !failed {
			continue
		}
		message := strings.TrimSpace(status.State.Waiting.Message)
		if message == "" {
			return fmt.Errorf("预拉取 Pod %s 容器 %s 启动失败: %s", podName, status.Name, reason)
		}
		return fmt.Errorf("预拉取 Pod %s 容器 %s 启动失败: %s: %s", podName, status.Name, reason, message)
	}
	return nil
}

func (o *K8sOrchestrator) waitPodsReady(ctx context.Context, cs kubernetes.Interface, namespace string, podNames []string) error {
	if o.cfg.ReadyPollIntervalSeconds <= 0 {
		return fmt.Errorf("SANDBOX_READY_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	ticker := time.NewTicker(sandboxPodReadyPollInterval(o.cfg.ReadyPollIntervalSeconds))
	defer ticker.Stop()
	for {
		ready := true
		for _, podName := range podNames {
			pod, err := cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("查询沙箱 Pod 状态失败: %w", err)
			}
			if err == nil {
				if failed := podFailureReason(pod); failed != "" {
					return fmt.Errorf("沙箱 Pod %s 失败: %s", podName, failed)
				}
			}
			ready = err == nil && sandboxPodReady(pod) && ready
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

// podFailureReason 抽取启动依赖 Pod 的公开失败原因,避免依赖等待超时才报告根因。
func podFailureReason(pod *corev1.Pod) string {
	if pod == nil {
		return ""
	}
	if pod.Status.Phase == corev1.PodFailed {
		if strings.TrimSpace(pod.Status.Reason) != "" {
			return strings.TrimSpace(pod.Status.Reason)
		}
		return "pod_failed"
	}
	for _, status := range append(append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...), pod.Status.ContainerStatuses...) {
		if status.State.Waiting != nil && strings.TrimSpace(status.State.Waiting.Reason) != "" {
			reason := strings.TrimSpace(status.State.Waiting.Reason)
			if reason == "ImagePullBackOff" || reason == "ErrImagePull" || reason == "CrashLoopBackOff" || reason == "CreateContainerError" {
				return status.Name + ":" + reason
			}
		}
		if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
			return status.Name + ":exit_" + fmt.Sprint(status.State.Terminated.ExitCode)
		}
	}
	return ""
}

// sandboxPodReadyPollInterval 使用统一配置,避免高频轮询挤占共享Kubernetes客户端预算。
func sandboxPodReadyPollInterval(configuredSeconds int) time.Duration {
	if configuredSeconds <= 0 {
		return time.Second
	}
	return time.Duration(configuredSeconds) * time.Second
}

// ToolReady 校验当前工具Pod与Web服务端点,不以容器曾经Ready代替当前可接入状态。
func (o *K8sOrchestrator) ToolReady(ctx context.Context, sb Sandbox, tool Tool) error {
	if len(tool.ResourceSpec.Components) == 0 {
		return fmt.Errorf("工具组件未声明")
	}
	window := int64(o.cfg.ReadyTimeoutSeconds)
	for _, component := range tool.ResourceSpec.Components {
		probe := component.ReadinessProbe
		if probe.Type == "" {
			continue
		}
		period, threshold := probe.PeriodSeconds, probe.FailureThreshold
		if period <= 0 {
			period = o.cfg.ProbeDefaultPeriodSeconds
		}
		if threshold <= 0 {
			threshold = o.cfg.ProbeDefaultFailureThreshold
		}
		window = max(window, int64(period)*int64(threshold))
	}
	if window <= 0 {
		return fmt.Errorf("工具启动就绪预算必须大于 0")
	}
	pods := make(map[string]*corev1.Pod, len(tool.ResourceSpec.Components))
	var observations []error
	for _, component := range tool.ResourceSpec.Components {
		pod, err := o.client.Clientset().CoreV1().Pods(sb.Namespace).Get(ctx, toolComponentPodName(tool.Code, component.Name), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			observations = append(observations, fmt.Errorf("%w: 工具Pod %s 不存在", errToolNotReady, component.Name))
			continue
		}
		if err != nil {
			return fmt.Errorf("查询工具 Pod 状态失败: %w", err)
		}
		pods[component.Name] = pod
		reason := podFailureReason(pod)
		switch {
		case pod.DeletionTimestamp != nil:
			observations = append(observations, fmt.Errorf("%w: 工具Pod %s 正在删除", errToolNotReady, pod.Name))
		case reason != "":
			observations = append(observations, fmt.Errorf("%w: 工具Pod %s: %s", errToolFailed, pod.Name, reason))
		case pod.Status.Phase == corev1.PodSucceeded:
			observations = append(observations, fmt.Errorf("%w: 工具Pod %s 已退出", errToolFailed, pod.Name))
		case pod.Status.Phase != corev1.PodRunning || !sandboxPodReady(pod) || !slices.ContainsFunc(pod.Status.ContainerStatuses, func(status corev1.ContainerStatus) bool {
			return status.Name == component.Name && status.Ready && status.State.Running != nil
		}):
			state := errToolNotReady
			if !pod.CreationTimestamp.IsZero() && time.Since(pod.CreationTimestamp.Time).Seconds() >= float64(window) {
				state = errToolFailed
			}
			observations = append(observations, fmt.Errorf("%w: 工具Pod %s 当前不可接入，启动预算 %ds", state, pod.Name, window))
		}
	}
	if err := errors.Join(observations...); err != nil {
		return err
	}
	if tool.Kind == SandboxToolKindWebEmbed {
		if len(tool.ResourceSpec.Services) == 0 {
			return fmt.Errorf("Web工具未声明Service")
		}
		componentPorts := componentPortMap(tool.ResourceSpec.Components)
		for _, service := range tool.ResourceSpec.Services {
			pod := pods[service.Component]
			if pod == nil {
				return fmt.Errorf("工具Service %s 的组件不存在", service.Name)
			}
			if err := o.toolServiceEndpointsReady(ctx, sb.Namespace, service, pod, componentPorts[service.Component]); err != nil {
				if !errors.Is(err, errToolNotReady) {
					return err
				}
				observations = append(observations, err)
			}
		}
	}
	return errors.Join(observations...)
}

// toolServiceEndpointsReady 同时核对当前 Service、Pod 和目标端口，拒绝同名旧切片。
func (o *K8sOrchestrator) toolServiceEndpointsReady(ctx context.Context, namespace string, service workload.ServiceSpec, pod *corev1.Pod, targetPorts map[string]int32) error {
	current, err := o.client.Clientset().CoreV1().Services(namespace).Get(ctx, service.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return fmt.Errorf("%w: 工具Service %s 不存在", errToolNotReady, service.Name)
	}
	if err != nil {
		return fmt.Errorf("查询工具Service %s 失败: %w", service.Name, err)
	}
	if current.DeletionTimestamp != nil || current.UID == "" {
		return fmt.Errorf("%w: 工具Service %s 当前不可接入", errToolNotReady, service.Name)
	}
	ports, err := servicePortsForSpec(service.Ports)
	if err != nil {
		return err
	}
	endpointSlices, err := o.client.Clientset().DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=" + service.Name,
	})
	if err != nil {
		return fmt.Errorf("查询工具Service %s 端点失败: %w", service.Name, err)
	}
	for _, port := range ports {
		if !slices.ContainsFunc(current.Spec.Ports, func(actual corev1.ServicePort) bool {
			return actual.Name == port.Name && actual.Port == port.Port && actual.TargetPort == port.TargetPort && actual.Protocol == port.Protocol
		}) {
			return fmt.Errorf("%w: Service %s 端口 %s 与冻结声明不符", errToolNotReady, service.Name, port.Name)
		}
		ready := false
		for _, slice := range endpointSlices.Items {
			if slice.DeletionTimestamp != nil || !slices.ContainsFunc(slice.OwnerReferences, func(owner metav1.OwnerReference) bool {
				return owner.APIVersion == "v1" && owner.Kind == "Service" && owner.Name == current.Name && owner.UID == current.UID
			}) {
				continue
			}
			portFound := false
			for _, endpointPort := range slice.Ports {
				protocol := corev1.ProtocolTCP
				if endpointPort.Protocol != nil {
					protocol = *endpointPort.Protocol
				}
				if endpointPort.Name != nil && *endpointPort.Name == port.Name && endpointPort.Port != nil &&
					*endpointPort.Port == targetPorts[port.TargetPort.StrVal] && *endpointPort.Port > 0 && protocol == port.Protocol {
					portFound = true
					break
				}
			}
			if !portFound {
				continue
			}
			for _, endpoint := range slice.Endpoints {
				if endpoint.Conditions.Ready == nil || !*endpoint.Conditions.Ready ||
					(endpoint.Conditions.Serving != nil && !*endpoint.Conditions.Serving) ||
					(endpoint.Conditions.Terminating != nil && *endpoint.Conditions.Terminating) {
					continue
				}
				if endpoint.TargetRef != nil && endpoint.TargetRef.Kind == "Pod" && endpoint.TargetRef.Name == pod.Name &&
					endpoint.TargetRef.UID == pod.UID && len(endpoint.Addresses) > 0 {
					ready = true
					break
				}
			}
			if ready {
				break
			}
		}
		if !ready {
			return fmt.Errorf("%w: Service %s 端口 %s 无当前Pod的可用端点", errToolNotReady, service.Name, port.Name)
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
	storageClass, err := o.client.Clientset().StorageV1().StorageClasses().Get(ctx, o.cfg.StorageClassName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	gvr := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	_, err = o.client.Dynamic().Resource(gvr).Namespace(o.cfg.ControlNamespace).List(ctx, metav1.ListOptions{Limit: 1})
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
	driver, _, err := unstructured.NestedString(class.Object, "driver")
	if err != nil {
		return false, fmt.Errorf("读取快照类驱动失败: %w", err)
	}
	return driver != "" && driver == storageClass.Provisioner && volumeSnapshotClassAllowsDeletion(class), nil
}

// volumeSnapshotClassAllowsDeletion 要求快照类删除策略为 Delete,避免到期清理后底层快照继续占用存储。
func volumeSnapshotClassAllowsDeletion(class *unstructured.Unstructured) bool {
	if class == nil {
		return false
	}
	policy, _, err := unstructured.NestedString(class.Object, "deletionPolicy")
	if err != nil {
		return false
	}
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
			"policy.sigstore.dev/include":        "true",
			"chaimir.io/sandbox":                 "true",
			"chaimir.io/managed-by":              "chaimir-backend",
			"chaimir.io/tenant-id":               ids.Format(sb.TenantID),
			"chaimir.io/sandbox-id":              ids.Format(sb.ID),
			"pod-security.kubernetes.io/enforce": "restricted",
		},
	}}
}

// createOrVerifyNamespace 创建或核对沙箱 Namespace,拒绝复用其他沙箱或旧安全标签的对象。
func createOrVerifyNamespace(ctx context.Context, cs kubernetes.Interface, desired *corev1.Namespace) error {
	client := cs.CoreV1().Namespaces()
	if _, err := client.Create(ctx, desired, metav1.CreateOptions{}); err == nil {
		return nil
	} else if !apierrors.IsAlreadyExists(err) {
		return err
	}
	existing, err := client.Get(ctx, desired.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("读取已存在 Namespace 失败: %w", err)
	}
	return verifyManagedLabels(existing, desired)
}

// createOrVerifyPod 创建或核对 Pod,防止旧 Pod 以相同名称承载不同镜像或挂载配置。
func createOrVerifyPod(ctx context.Context, cs kubernetes.Interface, desired *corev1.Pod) error {
	client := cs.CoreV1().Pods(desired.Namespace)
	if _, err := client.Create(ctx, desired, metav1.CreateOptions{}); err == nil {
		return nil
	} else if !apierrors.IsAlreadyExists(err) {
		return err
	}
	existing, err := client.Get(ctx, desired.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("读取已存在 Pod 失败: %w", err)
	}
	if err := verifyManagedLabels(existing, desired); err != nil {
		return err
	}
	if !sameSandboxPodSpec(existing.Spec, desired.Spec) {
		return fmt.Errorf("已存在 Pod %s 的工作负载规格与当前组合不一致", desired.Name)
	}
	return nil
}

// createOrVerifyResourceQuota 创建或核对 Namespace 资源硬限,不接受旧配额继续承载新组合。
func createOrVerifyResourceQuota(ctx context.Context, cs kubernetes.Interface, desired *corev1.ResourceQuota) error {
	client := cs.CoreV1().ResourceQuotas(desired.Namespace)
	if _, err := client.Create(ctx, desired, metav1.CreateOptions{}); err == nil {
		return nil
	} else if !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("创建 ResourceQuota 失败: %w", err)
	}
	existing, err := client.Get(ctx, desired.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("读取已存在 ResourceQuota 失败: %w", err)
	}
	if err := verifyManagedLabels(existing, desired); err != nil {
		return err
	}
	if !apiequality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		return fmt.Errorf("已存在 ResourceQuota %s 的硬限与当前配置不一致", desired.Name)
	}
	return nil
}

// createOrVerifyLimitRange 创建或核对容器默认资源限制。
func createOrVerifyLimitRange(ctx context.Context, cs kubernetes.Interface, desired *corev1.LimitRange) error {
	client := cs.CoreV1().LimitRanges(desired.Namespace)
	if _, err := client.Create(ctx, desired, metav1.CreateOptions{}); err == nil {
		return nil
	} else if !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("创建 LimitRange 失败: %w", err)
	}
	existing, err := client.Get(ctx, desired.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("读取已存在 LimitRange 失败: %w", err)
	}
	if err := verifyManagedLabels(existing, desired); err != nil {
		return err
	}
	if !apiequality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		return fmt.Errorf("已存在 LimitRange %s 的默认限制与当前配置不一致", desired.Name)
	}
	return nil
}

// createOrVerifyPVC 只在首次分配、快照恢复或显式重置时允许创建缺卷。
func createOrVerifyPVC(ctx context.Context, cs kubernetes.Interface, desired *corev1.PersistentVolumeClaim, allowCreate bool) error {
	client := cs.CoreV1().PersistentVolumeClaims(desired.Namespace)
	existing, err := client.Get(ctx, desired.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) && allowCreate {
		existing, err = client.Create(ctx, desired, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			existing, err = client.Get(ctx, desired.Name, metav1.GetOptions{})
		}
	}
	if err != nil {
		return fmt.Errorf("准备沙箱卷 PVC %s 失败: %w", desired.Name, err)
	}
	if existing.DeletionTimestamp != nil {
		return fmt.Errorf("沙箱卷 PVC %s 正在删除", desired.Name)
	}
	if err := verifyManagedLabels(existing, desired); err != nil {
		return err
	}
	if !sameSandboxPVCSpec(existing.Spec, desired.Spec) {
		return fmt.Errorf("已存在 PVC %s 的卷规格与当前组合不一致", desired.Name)
	}
	return nil
}

// createOrVerifyService 创建或核对服务发现入口,忽略 Kubernetes 自动分配的 ClusterIP 字段。
func createOrVerifyService(ctx context.Context, cs kubernetes.Interface, desired *corev1.Service) error {
	client := cs.CoreV1().Services(desired.Namespace)
	if _, err := client.Create(ctx, desired, metav1.CreateOptions{}); err == nil {
		return nil
	} else if !apierrors.IsAlreadyExists(err) {
		return err
	}
	existing, err := client.Get(ctx, desired.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("读取已存在 Service 失败: %w", err)
	}
	if err := verifyManagedLabels(existing, desired); err != nil {
		return err
	}
	if existing.Spec.Type != desired.Spec.Type || !apiequality.Semantic.DeepEqual(existing.Spec.Selector, desired.Spec.Selector) ||
		!apiequality.Semantic.DeepEqual(existing.Spec.Ports, desired.Spec.Ports) {
		return fmt.Errorf("已存在 Service %s 的选择器或端口与当前组合不一致", desired.Name)
	}
	return nil
}

// createOrVerifyVolumeSnapshot 创建或核对快照对象,避免恢复到错误 PVC 或错误保留策略。
func createOrVerifyVolumeSnapshot(ctx context.Context, client dynamic.ResourceInterface, desired *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if created, err := client.Create(ctx, desired, metav1.CreateOptions{}); err == nil {
		return created, nil
	} else if !apierrors.IsAlreadyExists(err) {
		return nil, err
	}
	existing, err := client.Get(ctx, desired.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("读取已存在 VolumeSnapshot 失败: %w", err)
	}
	if err := verifyManagedLabels(existing, desired); err != nil {
		return nil, err
	}
	desiredSpec, _, desiredErr := unstructured.NestedFieldNoCopy(desired.Object, "spec")
	existingSpec, _, existingErr := unstructured.NestedFieldNoCopy(existing.Object, "spec")
	if desiredErr != nil || existingErr != nil || !apiequality.Semantic.DeepEqual(existingSpec, desiredSpec) {
		return nil, fmt.Errorf("已存在 VolumeSnapshot %s 的来源与当前组合不一致", desired.GetName())
	}
	return nil, nil
}

// verifyManagedLabels 核对平台管理对象的不可变身份标签,避免名称复用跨沙箱资源。
func verifyManagedLabels(existing, desired metav1.Object) error {
	for key, value := range desired.GetLabels() {
		if existing.GetLabels()[key] != value {
			return fmt.Errorf("已存在对象 %s 的管理标签 %s 不一致", desired.GetName(), key)
		}
	}
	return nil
}

// sameSandboxPodSpec 比较沙箱 Pod 的安全、镜像、挂载、网络和调度语义,忽略 API Server 自动填充字段。
func sameSandboxPodSpec(current, desired corev1.PodSpec) bool {
	current = normalizeSandboxPodSpecForComparison(current)
	desired = normalizeSandboxPodSpecForComparison(desired)
	return current.ServiceAccountName == desired.ServiceAccountName &&
		apiequality.Semantic.DeepEqual(current.AutomountServiceAccountToken, desired.AutomountServiceAccountToken) &&
		apiequality.Semantic.DeepEqual(current.EnableServiceLinks, desired.EnableServiceLinks) &&
		current.RestartPolicy == desired.RestartPolicy &&
		apiequality.Semantic.DeepEqual(current.RuntimeClassName, desired.RuntimeClassName) &&
		apiequality.Semantic.DeepEqual(current.SecurityContext, desired.SecurityContext) &&
		apiequality.Semantic.DeepEqual(current.InitContainers, desired.InitContainers) &&
		apiequality.Semantic.DeepEqual(current.Containers, desired.Containers) &&
		apiequality.Semantic.DeepEqual(current.Volumes, desired.Volumes) &&
		apiequality.Semantic.DeepEqual(current.NodeSelector, desired.NodeSelector) &&
		apiequality.Semantic.DeepEqual(current.Tolerations, desired.Tolerations) &&
		apiequality.Semantic.DeepEqual(current.Affinity, desired.Affinity)
}

// normalizeSandboxPodSpecForComparison 去除 Kubernetes 在持久化 Pod 时补齐的等价默认值,
// 保留镜像、安全上下文、资源、挂载和探针等真正属于组合契约的字段。
func normalizeSandboxPodSpecForComparison(spec corev1.PodSpec) corev1.PodSpec {
	for index := range spec.InitContainers {
		normalizeSandboxContainerForComparison(&spec.InitContainers[index])
	}
	for index := range spec.Containers {
		normalizeSandboxContainerForComparison(&spec.Containers[index])
	}
	spec.Tolerations = normalizeSandboxTolerationsForComparison(spec.Tolerations)
	return spec
}

// normalizeSandboxTolerationsForComparison 去除 Kubernetes 自动补齐的默认节点不可用容忍,
// 保留平台显式声明的其他调度约束,避免二阶段幂等核对把同一 Pod 误判为规格漂移。
func normalizeSandboxTolerationsForComparison(items []corev1.Toleration) []corev1.Toleration {
	if len(items) == 0 {
		return nil
	}
	out := make([]corev1.Toleration, 0, len(items))
	for _, item := range items {
		if isKubernetesDefaultSandboxToleration(item) {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// isKubernetesDefaultSandboxToleration 识别 API Server 为未配置 Pod 自动加入的两条默认容忍。
func isKubernetesDefaultSandboxToleration(item corev1.Toleration) bool {
	if item.Operator != corev1.TolerationOpExists || item.Effect != corev1.TaintEffectNoExecute || item.Value != "" || item.TolerationSeconds == nil || *item.TolerationSeconds != 300 {
		return false
	}
	return item.Key == "node.kubernetes.io/not-ready" || item.Key == "node.kubernetes.io/unreachable"
}

// normalizeSandboxContainerForComparison 忽略 API Server 为容器补齐的终止消息字段,
// 以及等价于未设置的递归只读挂载默认值,避免同一组合的二次幂等核对误报规格漂移。
func normalizeSandboxContainerForComparison(container *corev1.Container) {
	container.TerminationMessagePath = ""
	container.TerminationMessagePolicy = ""
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

// sameSandboxPVCSpec 核对可复用规格；快照数据源只记录卷最初的创建来源，不代表当前内容。
func sameSandboxPVCSpec(current, desired corev1.PersistentVolumeClaimSpec) bool {
	if !apiequality.Semantic.DeepEqual(current.AccessModes, desired.AccessModes) ||
		!apiequality.Semantic.DeepEqual(current.Resources.Requests, desired.Resources.Requests) {
		return false
	}
	if desired.StorageClassName != nil && !apiequality.Semantic.DeepEqual(current.StorageClassName, desired.StorageClassName) {
		return false
	}
	return true
}

// applySandboxServiceAccount 创建无权限工作负载账号,避免依赖 Kubernetes 默认 SA 异步生成。
func (o *K8sOrchestrator) applySandboxServiceAccount(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan) error {
	automount := false
	refs := imagePullSecrets(o.cfg.ImagePullSecretNames)
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sandboxWorkloadServiceAccount,
			Namespace: plan.Sandbox.Namespace,
			Labels: map[string]string{
				"app":                   "chaimir",
				"module":                "sandbox",
				"chaimir.io/sandbox-id": ids.Format(plan.Sandbox.ID),
			},
		},
		AutomountServiceAccountToken: &automount,
		ImagePullSecrets:             refs,
	}
	client := cs.CoreV1().ServiceAccounts(plan.Sandbox.Namespace)
	existing, err := client.Get(ctx, sandboxWorkloadServiceAccount, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, err := client.Create(ctx, sa, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("创建沙箱 ServiceAccount 失败: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("查询沙箱 ServiceAccount 失败: %w", err)
	}
	if err := verifyManagedLabels(existing, sa); err != nil {
		return err
	}
	if !slices.Equal(existing.ImagePullSecrets, refs) ||
		existing.AutomountServiceAccountToken == nil ||
		*existing.AutomountServiceAccountToken {
		updated := existing.DeepCopy()
		updated.ImagePullSecrets = refs
		updated.AutomountServiceAccountToken = &automount
		if _, err := client.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("更新沙箱 ServiceAccount 失败: %w", err)
		}
	}
	return nil
}

// ensureImagePullSecrets 复用平台层能力同步动态命名空间所需的最小镜像凭据。
func (o *K8sOrchestrator) ensureImagePullSecrets(ctx context.Context, namespace string) error {
	return o.client.SyncImagePullSecrets(ctx, o.cfg.ControlNamespace, namespace, "sandbox", o.cfg.ImagePullSecretNames)
}

// ensureRuntimeSecrets 将当前组合声明的最小敏感键同步到沙箱命名空间。
func (o *K8sOrchestrator) ensureRuntimeSecrets(ctx context.Context, plan CreateSandboxPlan) error {
	keys := map[string]struct{}{}
	collect := func(component workload.ComponentSpec) error {
		if err := validateComponentEnv(component); err != nil {
			return fmt.Errorf("组件 %s 环境变量无效: %w", component.Name, err)
		}
		for _, secret := range component.SecretEnv {
			name := strings.TrimSpace(secret.Name)
			secretName := strings.TrimSpace(secret.SecretName)
			secretKey := strings.TrimSpace(secret.SecretKey)
			if name == "" || secretName == "" || secretKey == "" {
				return fmt.Errorf("组件 %s 的 Secret 环境声明不完整", component.Name)
			}
			if secretName != "chaimir-secret" {
				return fmt.Errorf("组件 %s 的 Secret 只能引用平台受控 Secret", component.Name)
			}
			keys[secretKey] = struct{}{}
		}
		return nil
	}
	for _, runtimePlan := range plan.Runtimes {
		if err := collect(runtimePlan.Runtime.AdapterSpec.RuntimeContainer); err != nil {
			return err
		}
		for _, component := range runtimePlan.Runtime.AdapterSpec.InfraSidecars {
			if err := collect(component); err != nil {
				return err
			}
		}
		for _, pod := range runtimePlan.Runtime.AdapterSpec.Pods {
			for _, component := range pod.InitContainers {
				if err := collect(component); err != nil {
					return err
				}
			}
			for _, component := range pod.Containers {
				if err := collect(component); err != nil {
					return err
				}
			}
		}
	}
	for _, tool := range plan.Tools {
		for _, component := range tool.ResourceSpec.Components {
			if err := collect(component); err != nil {
				return err
			}
		}
	}
	for _, component := range plan.PrivateSidecars {
		if err := collect(component); err != nil {
			return err
		}
	}
	if len(keys) == 0 {
		return nil
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	return o.client.SyncSecretKeys(ctx, o.cfg.ControlNamespace, "chaimir-secret", plan.Sandbox.Namespace, "chaimir-secret", "sandbox", ordered)
}

// applyResourceQuota 创建 Namespace 级资源硬限。
func (o *K8sOrchestrator) applyResourceQuota(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan) error {
	rq := o.resourceQuotaForPlan(plan)
	return createOrVerifyResourceQuota(ctx, cs, rq)
}

// resourceQuotaForPlan 构造 Namespace 级资源硬限,避免工具 sidecar 被单容器默认 request 误拒。
func (o *K8sOrchestrator) resourceQuotaForPlan(plan CreateSandboxPlan) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-quota", Namespace: plan.Sandbox.Namespace, Labels: sandboxManagedLabels(plan.Sandbox)},
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
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-limits", Namespace: plan.Sandbox.Namespace, Labels: sandboxManagedLabels(plan.Sandbox)},
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
	return createOrVerifyLimitRange(ctx, cs, lr)
}

// applyNetworkPolicies 创建默认拒绝、控制面入口和清单声明的同沙箱 Pod 互通策略。
func (o *K8sOrchestrator) applyNetworkPolicies(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan) error {
	deny := &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-deny-all", Namespace: plan.Sandbox.Namespace, Labels: sandboxPolicyLabels(plan.Sandbox)},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeIngress, netv1.PolicyTypeEgress},
		},
	}
	if err := createOrVerifyNetworkPolicy(ctx, cs, deny); err != nil {
		return fmt.Errorf("创建 deny-all NetworkPolicy 失败: %w", err)
	}
	if err := createOrVerifyNetworkPolicy(ctx, cs, o.dnsEgressPolicy(plan.Sandbox)); err != nil {
		return fmt.Errorf("创建 DNS NetworkPolicy 失败: %w", err)
	}
	for _, policy := range o.allowControlPlanePolicies(plan) {
		if err := createOrVerifyNetworkPolicy(ctx, cs, policy); err != nil {
			return fmt.Errorf("创建控制面 NetworkPolicy 失败: %w", err)
		}
	}
	for _, policy := range o.allowSandboxPodLinkPolicies(plan) {
		if err := createOrVerifyNetworkPolicy(ctx, cs, policy); err != nil {
			return fmt.Errorf("创建沙箱内 Pod 网络策略失败: %w", err)
		}
	}
	for _, policy := range o.allowToolPodLinkPolicies(plan) {
		if err := createOrVerifyNetworkPolicy(ctx, cs, policy); err != nil {
			return fmt.Errorf("创建工具 Pod 网络策略失败: %w", err)
		}
	}
	return nil
}

// createOrVerifyNetworkPolicy 只接受与当前编译结果完全一致的重试对象。
// 已存在的策略可能来自先前的部分创建,必须拒绝其放行范围与本次组合不一致的情况。
func createOrVerifyNetworkPolicy(ctx context.Context, cs kubernetes.Interface, desired *netv1.NetworkPolicy) error {
	if desired == nil || strings.TrimSpace(desired.Namespace) == "" || strings.TrimSpace(desired.Name) == "" {
		return fmt.Errorf("NetworkPolicy 声明不完整")
	}
	policies := cs.NetworkingV1().NetworkPolicies(desired.Namespace)
	if _, err := policies.Create(ctx, desired, metav1.CreateOptions{}); err == nil {
		return nil
	} else if !apierrors.IsAlreadyExists(err) {
		return err
	}
	existing, err := policies.Get(ctx, desired.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("读取已存在 NetworkPolicy 失败: %w", err)
	}
	if err := verifyManagedLabels(existing, desired); err != nil {
		return err
	}
	if !apiequality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		return fmt.Errorf("已存在 NetworkPolicy %s 的规则与当前组合不一致", desired.Name)
	}
	return nil
}

// dnsEgressPolicy 允许沙箱内 Pod 解析集群 Service DNS,但不放开其他出站目标。
func (o *K8sOrchestrator) dnsEgressPolicy(sb Sandbox) *netv1.NetworkPolicy {
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sandbox-allow-dns-egress",
			Namespace: sb.Namespace,
			Labels:    sandboxPolicyLabels(sb),
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

// sandboxManagedLabels 为沙箱命名空间内的受控对象写入不可变所有权标签。
func sandboxManagedLabels(sb Sandbox) map[string]string {
	return map[string]string{
		"app":                       "chaimir",
		"module":                    "sandbox",
		"app.kubernetes.io/part-of": "chaimir",
		"chaimir.io/managed-by":     "chaimir-backend",
		"chaimir.io/sandbox":        "true",
		"chaimir.io/tenant-id":      ids.Format(sb.TenantID),
		"chaimir.io/sandbox-id":     ids.Format(sb.ID),
	}
}

// sandboxPolicyLabels 与部署层动态沙箱 NetworkPolicy 审计模板保持一致,并绑定当前沙箱。
func sandboxPolicyLabels(sb Sandbox) map[string]string {
	return sandboxManagedLabels(sb)
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
		if tool.Category != "infra" && tool.Kind != SandboxToolKindWebEmbed {
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
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: sb.Namespace, Labels: sandboxPolicyLabels(sb)},
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
	var policies []*netv1.NetworkPolicy
	for _, runtimePlan := range plan.Runtimes {
		for _, rule := range runtimePlan.Runtime.AdapterSpec.NetworkRules {
			ports := networkPolicyPortsForRefs(rule.Ports)
			// deny-all 同时限制入站和出站,因此显式互通需要为源 Pod 出站和目标 Pod 入站各建一条策略。
			policies = append(policies,
				sandboxPodIngressPolicy(plan, rule, ports),
				sandboxPodEgressPolicy(plan, rule, ports),
			)
		}
	}
	return policies
}

// sandboxPodIngressPolicy 允许来源 Pod 访问目标 Pod 的声明端口。
func sandboxPodIngressPolicy(plan CreateSandboxPlan, rule workload.NetworkRuleSpec, ports []netv1.NetworkPolicyPort) *netv1.NetworkPolicy {
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: networkPolicyRuleName("sandbox-allow", rule, "ingress"), Namespace: plan.Sandbox.Namespace, Labels: sandboxPolicyLabels(plan.Sandbox)},
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
		ObjectMeta: metav1.ObjectMeta{Name: networkPolicyRuleName("sandbox-allow", rule, "egress"), Namespace: plan.Sandbox.Namespace, Labels: sandboxPolicyLabels(plan.Sandbox)},
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

// allowToolPodLinkPolicies 为全部工具与基础设施的声明连接生成双向 NetworkPolicy。
func (o *K8sOrchestrator) allowToolPodLinkPolicies(plan CreateSandboxPlan) []*netv1.NetworkPolicy {
	policies := []*netv1.NetworkPolicy{}
	for _, tool := range plan.Tools {
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

// toolPodIngressPolicy 允许工具 Pod 访问同工具组件或目标运行时 Pod 的声明端口。
func toolPodIngressPolicy(plan CreateSandboxPlan, tool Tool, rule workload.NetworkRuleSpec, ports []netv1.NetworkPolicyPort) *netv1.NetworkPolicy {
	fromRole := toolComponentPodName(tool.Code, rule.From)
	toRole := toolNetworkRuleTargetRole(tool, rule.To)
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: networkPolicyRuleName("sandbox-allow-tool", workload.NetworkRuleSpec{Name: rule.Name, From: fromRole, To: toRole, Ports: rule.Ports}, "ingress"), Namespace: plan.Sandbox.Namespace, Labels: sandboxPolicyLabels(plan.Sandbox)},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: sandboxPodRoleLabels(plan.Sandbox, toRole)},
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeIngress},
			Ingress: []netv1.NetworkPolicyIngressRule{{
				From:  []netv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: sandboxPodRoleLabels(plan.Sandbox, fromRole)}}},
				Ports: ports,
			}},
		},
	}
}

// toolPodEgressPolicy 允许工具 Pod 出站访问同工具组件或目标运行时 Pod 的声明端口。
func toolPodEgressPolicy(plan CreateSandboxPlan, tool Tool, rule workload.NetworkRuleSpec, ports []netv1.NetworkPolicyPort) *netv1.NetworkPolicy {
	fromRole := toolComponentPodName(tool.Code, rule.From)
	toRole := toolNetworkRuleTargetRole(tool, rule.To)
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: networkPolicyRuleName("sandbox-allow-tool", workload.NetworkRuleSpec{Name: rule.Name, From: fromRole, To: toRole, Ports: rule.Ports}, "egress"), Namespace: plan.Sandbox.Namespace, Labels: sandboxPolicyLabels(plan.Sandbox)},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: sandboxPodRoleLabels(plan.Sandbox, fromRole)},
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeEgress},
			Egress: []netv1.NetworkPolicyEgressRule{{
				To:    []netv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: sandboxPodRoleLabels(plan.Sandbox, toRole)}}},
				Ports: ports,
			}},
		},
	}
}

// networkPolicyRuleName 构造组合实例级唯一的 DNS-1123 NetworkPolicy 名称。
// 同一 manifest 可以在一个组合中实例化多次,因此规则名必须包含实例化后的来源、目标和端口。
func networkPolicyRuleName(prefix string, rule workload.NetworkRuleSpec, direction string) string {
	identity := strings.Join([]string{
		strings.TrimSpace(prefix),
		strings.TrimSpace(rule.Name),
		strings.TrimSpace(rule.From),
		strings.TrimSpace(rule.To),
		strings.TrimSpace(direction),
		networkPolicyPortIdentity(rule.Ports),
	}, "\x00")
	hash := sha256.Sum256([]byte(identity))
	suffix := hex.EncodeToString(hash[:])[:12]
	base := strings.ToLower(strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, strings.TrimSpace(prefix)+"-"+strings.TrimSpace(rule.Name)+"-"+strings.TrimSpace(direction)))
	base = strings.Trim(base, "-")
	maxBase := 63 - len(suffix) - 1
	if len(base) > maxBase {
		base = strings.TrimRight(base[:maxBase], "-")
	}
	if base == "" {
		base = "sandbox-allow"
	}
	return base + "-" + suffix
}

// networkPolicyPortIdentity 将端口引用按声明顺序编码进策略名称哈希,避免同规则名不同端口共用对象。
func networkPolicyPortIdentity(ports []workload.NetworkPortRef) string {
	items := make([]string, 0, len(ports))
	for _, port := range ports {
		items = append(items, strings.TrimSpace(port.Name)+"="+strconv.Itoa(int(port.Port)))
	}
	return strings.Join(items, ",")
}

// toolNetworkRuleTargetRole 把工具内组件目标解析为实际 Pod role;其他目标保持运行时 Pod 名称。
func toolNetworkRuleTargetRole(tool Tool, target string) string {
	target = strings.TrimSpace(target)
	for _, component := range tool.ResourceSpec.Components {
		if strings.TrimSpace(component.Name) == target {
			return toolComponentPodName(tool.Code, target)
		}
	}
	return target
}

func (o *K8sOrchestrator) applyWorkspacePVC(ctx context.Context, cs kubernetes.Interface, plan CreateSandboxPlan) error {
	restore := strings.TrimSpace(plan.Sandbox.SnapshotRef) != ""
	if restore {
		if err := validateSnapshotRefForNamespace(plan.Sandbox); err != nil {
			return err
		}
	}
	var pending []*corev1.PersistentVolumeClaim
	for _, domain := range persistentVolumeDomains(plan) {
		source := (*corev1.TypedLocalObjectReference)(nil)
		if restore && containsString(plan.Sandbox.SnapshotDomains, domain.claimName()) {
			source = snapshotSourceForDomain(plan.Sandbox, VolumeDomainSpec{Name: domain.claimName()})
		}
		pvc := o.volumeDomainPVC(plan, domain.claimName(), source)
		if (plan.Sandbox.Phase != SandboxPhaseAllocating || restore) && source == nil {
			// 先核对无快照可恢复的保留卷，避免留下部分恢复的环境。
			if err := createOrVerifyPVC(ctx, cs, pvc, false); err != nil {
				return err
			}
		} else {
			pending = append(pending, pvc)
		}
	}
	for _, pvc := range pending {
		if err := createOrVerifyPVC(ctx, cs, pvc, true); err != nil {
			return err
		}
	}
	return nil
}

func (o *K8sOrchestrator) volumeDomainPVC(plan CreateSandboxPlan, name string, source *corev1.TypedLocalObjectReference) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: plan.Sandbox.Namespace,
			Labels: map[string]string{
				"app":                   "chaimir",
				"module":                "sandbox",
				"chaimir.io/sandbox-id": ids.Format(plan.Sandbox.ID),
				"chaimir.io/volume":     name,
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
	pvc.Spec.DataSource = source
	return pvc
}

// validateSnapshotRefForNamespace 校验快照组引用只能指向当前沙箱命名空间。
func validateSnapshotRefForNamespace(sb Sandbox) error {
	parts := strings.Split(strings.TrimSpace(sb.SnapshotRef), "/")
	prefix := "snapshot-" + ids.Format(sb.ID) + "-"
	if len(parts) != 2 || parts[0] != sb.Namespace || !strings.HasPrefix(parts[1], prefix) {
		return fmt.Errorf("快照引用不属于当前沙箱")
	}
	attempt, err := hex.DecodeString(strings.TrimPrefix(parts[1], prefix))
	if err != nil || len(attempt) != 16 {
		return fmt.Errorf("快照引用缺少有效代次")
	}
	return nil
}

// podsForPlan 构造本阶段全部运行时与工具 Pod。
func (o *K8sOrchestrator) podsForPlan(plan CreateSandboxPlan) ([]*corev1.Pod, error) {
	if err := validateRuntimeIsolation(plan.Runtimes); err != nil {
		return nil, err
	}
	deferred, err := deferredToolPods(plan)
	if err != nil {
		return nil, err
	}
	pods := podGroupForPlan(plan)
	var out []*corev1.Pod
	for _, pod := range pods {
		compiled, err := o.podFromSpec(plan, pod)
		if err != nil {
			return nil, err
		}
		out = append(out, compiled)
	}
	for _, tool := range plan.Tools {
		if tool.Category == "infra" || tool.Kind == SandboxToolKindWebEmbed || tool.Kind == SandboxToolKindCommand {
			for _, component := range tool.ResourceSpec.Components {
				pod, err := o.toolPodForPlan(plan, tool, component)
				if err != nil {
					return nil, err
				}
				out = append(out, pod)
			}
		}
	}
	seen := make(map[string]bool, len(out))
	for _, pod := range out {
		if problems := validation.IsDNS1123Label(pod.Name); len(problems) != 0 {
			return nil, fmt.Errorf("沙箱 Pod 名称 %s 无效: %s", pod.Name, strings.Join(problems, "; "))
		}
		if seen[pod.Name] {
			return nil, fmt.Errorf("沙箱 Pod 名称 %s 重复", pod.Name)
		}
		seen[pod.Name] = true
	}
	out = slices.DeleteFunc(out, func(pod *corev1.Pod) bool { return deferred[pod.Name] })
	return out, nil
}

func deferredToolPods(plan CreateSandboxPlan) (map[string]bool, error) {
	if !plan.DeferTools || len(plan.Tools) == 0 {
		return nil, nil
	}
	snapshot, err := verifiedCompositionSnapshot(plan.Sandbox)
	if err != nil {
		return nil, err
	}
	required := make(map[string]bool, len(plan.Runtimes))
	for _, runtime := range plan.Runtimes {
		required[runtime.InstanceCode] = true
	}
	// 依赖边从消费者指向提供者；迭代直到完整前置闭包不再增长。
	for changed := true; changed; {
		changed = false
		for _, link := range snapshot.Spec.Links {
			if link.RequiredAtStart && required[link.SourceComponent] && !required[link.TargetComponent] {
				required[link.TargetComponent] = true
				changed = true
			}
		}
	}
	deferred := make(map[string]bool)
	for _, tool := range plan.Tools {
		if !required[tool.Code] {
			for _, component := range tool.ResourceSpec.Components {
				deferred[toolComponentPodName(tool.Code, component.Name)] = true
			}
		}
	}
	return deferred, nil
}

// createPodsInStartupOrder 先创建独立组件，再逐组创建依赖组件；同组 Pod 必须全部创建后才能等待就绪。
func createPodsInStartupOrder(plan CreateSandboxPlan, pods []*corev1.Pod, create func(*corev1.Pod) error, waitReady func([]string) error) error {
	order := []string{""}
	required := make(map[string]bool)
	for _, runtime := range plan.Runtimes {
		for _, code := range runtime.Runtime.AdapterSpec.StartupOrder {
			if !required[code] {
				required[code] = true
				order = append(order, code)
			}
		}
	}
	groups := make(map[string][]*corev1.Pod)
	for _, pod := range pods {
		var code string
		if runtime, ok := runtimePlanForPod(plan, pod.Name); ok {
			code = runtime.InstanceCode
		} else {
			for _, tool := range plan.Tools {
				for _, component := range tool.ResourceSpec.Components {
					if toolComponentPodName(tool.Code, component.Name) == pod.Name {
						code = tool.Code
					}
				}
			}
		}
		if code == "" {
			return fmt.Errorf("沙箱 Pod %s 不属于声明的运行时或工具", pod.Name)
		}
		if !required[code] {
			code = ""
		}
		groups[code] = append(groups[code], pod)
	}
	for _, code := range order {
		var names []string
		for _, pod := range groups[code] {
			if err := create(pod); err != nil {
				return err
			}
			names = append(names, pod.Name)
		}
		if code != "" && len(names) > 0 {
			if err := waitReady(names); err != nil {
				return err
			}
		}
	}
	return nil
}

// podGroupForPlan 返回运行时声明的 Pod 组;未声明时按 runtime_container + infra_sidecars 生成单 Pod 拓扑。
func podGroupForPlan(plan CreateSandboxPlan) []workload.PodSpec {
	var pods []workload.PodSpec
	for _, runtimePlan := range plan.Runtimes {
		items := runtimePodSpecs(runtimePlan)
		for itemIndex := range items {
			for containerIndex := range items[itemIndex].Containers {
				container := &items[itemIndex].Containers[containerIndex]
				if container.Name == runtimePlan.Runtime.AdapterSpec.RuntimeContainer.Name && container.ImageURL == "" {
					container.ImageURL = runtimePlan.Image.ImageURL
				}
			}
		}
		if runtimePlan.InstanceCode == plan.WorkspaceRuntimeInstance {
			items = appendPrivateSidecarsToRuntimePod(items, runtimePlan.Runtime.AdapterSpec.RuntimeContainer.Name, plan.PrivateSidecars)
		}
		pods = append(pods, items...)
	}
	return pods
}

// runtimePodSpecs 返回带实例前缀的运行时 Pod 拓扑,确保未编译的自检计划也不会回退到共享 sandbox 名称。
func runtimePodSpecs(plan RuntimePlan) []workload.PodSpec {
	pods := podTopologyForAdapter(plan.Runtime.AdapterSpec)
	for i := range pods {
		pods[i].Name = runtimePodName(plan.InstanceCode, pods[i].Name)
	}
	return pods
}

// runtimePodName 生成运行时实例唯一的 Pod 名,并使已编译拓扑重复读取时保持幂等。
func runtimePodName(instanceCode, podName string) string {
	instanceCode = strings.TrimSpace(instanceCode)
	podName = strings.TrimSpace(podName)
	// 前缀预留连接符和八位摘要,保证超长实例重复物化时仍能识别自己的 Pod。
	instanceCode = boundedSandboxName(instanceCode, 54)
	if instanceCode == "" || podName == "" || strings.HasPrefix(podName, instanceCode+"-") {
		return podName
	}
	return boundedSandboxName(instanceCode+"-"+podName, 63)
}

// runtimePlanForPod 根据唯一 Pod 角色定位所属运行时实例,避免用第一个运行时解释其他实例容器。
func runtimePlanForPod(plan CreateSandboxPlan, podName string) (RuntimePlan, bool) {
	for _, runtimePlan := range plan.Runtimes {
		for _, pod := range runtimePodSpecs(runtimePlan) {
			if pod.Name == podName {
				return runtimePlan, true
			}
		}
	}
	return RuntimePlan{}, false
}

// appendPrivateSidecarsToRuntimePod 把服务端私有执行容器放入运行时所在 Pod,不创建可代理工具入口。
func appendPrivateSidecarsToRuntimePod(pods []workload.PodSpec, runtimeContainer string, sidecars []workload.ComponentSpec) []workload.PodSpec {
	if len(sidecars) == 0 {
		return pods
	}
	for i := range pods {
		for _, container := range pods[i].Containers {
			if container.Name == runtimeContainer {
				pods[i].Containers = append(pods[i].Containers, sidecars...)
				return pods
			}
		}
	}
	return pods
}

// podFromSpec 把运行时 Pod 拓扑转换为受限 Kubernetes Pod。
func (o *K8sOrchestrator) podFromSpec(plan CreateSandboxPlan, spec workload.PodSpec) (*corev1.Pod, error) {
	runtimePlan, found := runtimePlanForPod(plan, spec.Name)
	if !found {
		return nil, fmt.Errorf("运行时 Pod %q 未绑定到运行时实例", spec.Name)
	}
	containers := make([]corev1.Container, 0, len(spec.Containers))
	for _, container := range spec.Containers {
		image := container.ImageURL
		if container.Name == runtimePlan.Runtime.AdapterSpec.RuntimeContainer.Name && image == "" {
			image = runtimePlan.Image.ImageURL
		}
		containers = append(containers, o.containerFromRuntime(container, image, runtimePlan.Runtime.AdapterSpec))
	}
	initContainers := make([]corev1.Container, 0, len(spec.InitContainers))
	for _, container := range spec.InitContainers {
		initContainers = append(initContainers, o.containerFromRuntime(container, container.ImageURL, runtimePlan.Runtime.AdapterSpec))
	}
	automount := false
	enableServiceLinks := false
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: plan.Sandbox.Namespace,
			Labels:    sandboxPodRoleLabels(plan.Sandbox, spec.Name),
		},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: &automount,
			EnableServiceLinks:           &enableServiceLinks,
			RuntimeClassName:             &o.cfg.RuntimeClass,
			ServiceAccountName:           sandboxWorkloadServiceAccount,
			RestartPolicy:                corev1.RestartPolicyNever,
			SecurityContext:              podSecurityContext(),
			InitContainers:               initContainers,
			Containers:                   containers,
			NodeSelector:                 copyStringMap(o.cfg.SandboxNodeSelector),
			Tolerations:                  sandboxTolerations(o.cfg.SandboxNodeTolerations),
			Volumes:                      podVolumesForPlan(plan, runtimePlan, slices.Concat(initContainers, containers)),
		},
	}
	applySandboxVolumeAffinity(plan.Sandbox, pod)
	return pod, nil
}

// toolPodForPlan 为 web-embed 工具组件创建独立 Pod,避免动态工具影响运行时 Pod 组拓扑。
func (o *K8sOrchestrator) toolPodForPlan(plan CreateSandboxPlan, tool Tool, component workload.ComponentSpec) (*corev1.Pod, error) {
	automount := false
	enableServiceLinks := false
	podName := toolComponentPodName(tool.Code, component.Name)
	container, err := o.containerFromTool(plan, tool, component)
	if err != nil {
		return nil, err
	}
	toolVolumes, err := podVolumesForTool(plan, tool, component)
	if err != nil {
		return nil, err
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: plan.Sandbox.Namespace,
			Labels:    sandboxPodRoleLabels(plan.Sandbox, podName),
		},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: &automount,
			EnableServiceLinks:           &enableServiceLinks,
			RuntimeClassName:             &o.cfg.RuntimeClass,
			ServiceAccountName:           sandboxWorkloadServiceAccount,
			RestartPolicy:                corev1.RestartPolicyNever,
			SecurityContext:              podSecurityContext(),
			Containers:                   []corev1.Container{container},
			NodeSelector:                 copyStringMap(o.cfg.SandboxNodeSelector),
			Tolerations:                  sandboxTolerations(o.cfg.SandboxNodeTolerations),
			Volumes:                      toolVolumes,
		},
	}
	applySandboxVolumeAffinity(plan.Sandbox, pod)
	return pod, nil
}

// applySandboxVolumeAffinity 让 PVC 消费者共节点；首个消费者匹配自身，不等待后续启动依赖。
func applySandboxVolumeAffinity(sb Sandbox, pod *corev1.Pod) {
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		pod.Labels["chaimir.io/pvc-consumer"] = "true"
		pod.Spec.Affinity = &corev1.Affinity{PodAffinity: &corev1.PodAffinity{RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app": "chaimir", "module": "sandbox", "chaimir.io/sandbox-id": ids.Format(sb.ID), "chaimir.io/pvc-consumer": "true",
			}},
			TopologyKey: "kubernetes.io/hostname",
		}}}}
		return
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
		Env:             envVarsWithSecrets(spec.Env, spec.SecretEnv),
		Ports:           containerPorts(spec.Ports),
		Resources:       resources(spec.Resources, o.cfg),
		SecurityContext: containerSecurityContext(readOnlyRootFilesystem(spec.ReadOnlyRootFilesystem)),
		VolumeMounts:    volumeMountsForContainer(adapter, spec),
		ReadinessProbe:  probe(spec.ReadinessProbe),
		LivenessProbe:   probe(spec.LivenessProbe),
	}
}

// containerFromTool 构造 web-embed 工具组件容器。
func (o *K8sOrchestrator) containerFromTool(plan CreateSandboxPlan, tool Tool, component workload.ComponentSpec) (corev1.Container, error) {
	mounts := []corev1.VolumeMount{}
	seenMountPaths := map[string]struct{}{}
	if shouldMountWorkspace(component) {
		mounts = appendUniqueVolumeMount(mounts, seenMountPaths, corev1.VolumeMount{Name: VolumeDomainWorkspace, MountPath: workspaceMountPath(plan)})
	}
	var domainMounts []corev1.VolumeMount
	if len(component.MountDomains) > 0 {
		_, adapter, err := toolVolumeSource(plan, tool)
		if err != nil {
			return corev1.Container{}, err
		}
		domainMounts, err = toolDomainVolumeMounts(adapter, component)
		if err != nil {
			return corev1.Container{}, err
		}
	}
	for _, domain := range domainMounts {
		mounts = appendUniqueVolumeMount(mounts, seenMountPaths, domain)
	}
	for _, mount := range toolEphemeralVolumeMounts(tool, component) {
		mounts = appendUniqueVolumeMount(mounts, seenMountPaths, mount)
	}
	return corev1.Container{
		Name:            component.Name,
		Image:           component.ImageURL,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         component.Command,
		Args:            component.Args,
		WorkingDir:      component.Workdir,
		Env:             envVarsWithSecrets(component.Env, component.SecretEnv),
		Ports:           containerPorts(component.Ports),
		Resources:       resources(component.Resources, o.cfg),
		SecurityContext: containerSecurityContext(readOnlyRootFilesystem(component.ReadOnlyRootFilesystem)),
		VolumeMounts:    mounts,
		ReadinessProbe:  probe(component.ReadinessProbe),
		LivenessProbe:   probe(component.LivenessProbe),
	}, nil
}

// podVolumesForPlan 只声明当前 Pod 实际挂载的域与临时卷，避免附带其他 Pod 的 PVC。
func podVolumesForPlan(plan CreateSandboxPlan, runtime RuntimePlan, containers []corev1.Container) []corev1.Volume {
	domains := map[string]corev1.Volume{}
	for _, volume := range volumeDomainsForPlan(plan) {
		domains[volume.Name] = volume
	}
	var volumes []corev1.Volume
	seen := map[string]struct{}{}
	for _, container := range containers {
		for _, mount := range container.VolumeMounts {
			if _, exists := seen[mount.Name]; exists {
				continue
			}
			seen[mount.Name] = struct{}{}
			volume, declared := domains[volumeClaimName("runtime-"+runtime.InstanceCode, mount.Name)]
			if !declared {
				volume = corev1.Volume{Name: mount.Name, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}
			}
			volume.Name = mount.Name
			volumes = append(volumes, volume)
		}
	}
	return volumes
}

// podVolumesForTool 汇总工具需要的工作区 PVC 和私有临时卷。
func podVolumesForTool(plan CreateSandboxPlan, tool Tool, component workload.ComponentSpec) ([]corev1.Volume, error) {
	volumes := []corev1.Volume{}
	seen := map[string]struct{}{}
	if shouldMountWorkspace(component) {
		volumes = append(volumes, corev1.Volume{
			Name:         VolumeDomainWorkspace,
			VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: VolumeDomainWorkspace}},
		})
		seen[VolumeDomainWorkspace] = struct{}{}
	}
	owner := ""
	adapter := AdapterSpec{}
	if len(component.MountDomains) > 0 {
		var err error
		owner, adapter, err = toolVolumeSource(plan, tool)
		if err != nil {
			return nil, err
		}
	}
	available := map[string]corev1.Volume{}
	for _, volume := range volumeDomainsForPlan(plan) {
		available[volume.Name] = volume
	}
	for _, domain := range component.MountDomains {
		name := strings.TrimSpace(domain)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		if _, ok := volumeDomainByName(adapter, name); !ok {
			return nil, fmt.Errorf("工具组件 %s 引用了未声明的安全域 %s", component.Name, name)
		}
		volume, ok := available[volumeClaimName(owner, name)]
		if !ok {
			return nil, fmt.Errorf("工具组件 %s 的卷来源未进入计划", component.Name)
		}
		volume.Name = name
		volumes = append(volumes, volume)
		seen[name] = struct{}{}
	}
	for _, mount := range component.EphemeralMounts {
		volumes = append(volumes, corev1.Volume{
			Name:         toolEphemeralVolumeName(tool.Code, component.Name, mount.Name),
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		})
	}
	return volumes, nil
}

// toolDomainVolumeMounts 使用来源域的路径，写权限须由组件显式声明。
func toolDomainVolumeMounts(adapter AdapterSpec, component workload.ComponentSpec) ([]corev1.VolumeMount, error) {
	domains := make(map[string]VolumeDomainSpec, len(adapter.VolumeDomains))
	for _, domain := range adapter.VolumeDomains {
		domains[domain.Name] = domain
	}
	mounts := make([]corev1.VolumeMount, 0, len(component.MountDomains))
	writable := make(map[string]struct{}, len(component.WritableMountDomains))
	for _, raw := range component.WritableMountDomains {
		writable[strings.TrimSpace(raw)] = struct{}{}
	}
	for _, raw := range component.MountDomains {
		name := strings.TrimSpace(raw)
		domain, ok := domains[name]
		if !ok {
			return nil, fmt.Errorf("组件 %s 引用了未声明安全域 %s", component.Name, name)
		}
		_, writableDomain := writable[name]
		mounts = append(mounts, corev1.VolumeMount{Name: domain.Name, MountPath: domain.MountPath, ReadOnly: !writableDomain})
	}
	return mounts, nil
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
	return boundedSandboxName("tool-"+toolCode+"-"+componentName+"-"+mountName, 63)
}

func boundedSandboxName(name string, limit int) string {
	if len(name) <= limit {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	suffix := hex.EncodeToString(sum[:4])
	return strings.TrimRight(name[:limit-len(suffix)-1], "-") + "-" + suffix
}

// workspaceMountPath 返回组合共享工作区的唯一挂载路径。
func workspaceMountPath(plan CreateSandboxPlan) string {
	adapter := workspaceRuntimeForPlan(plan).AdapterSpec
	for _, domain := range adapter.VolumeDomains {
		if domain.Name == VolumeDomainWorkspace && domain.MountPath != "" {
			return domain.MountPath
		}
	}
	if adapter.WorkspaceDir != "" {
		return adapter.WorkspaceDir
	}
	return "/workspace"
}

// volumeMountsForContainer 按容器职责决定挂载域,防止终端或协同容器绕过文件 API 读取私有资产。
func volumeMountsForContainer(adapter AdapterSpec, spec workload.ComponentSpec) []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0, len(adapter.VolumeDomains))
	seenMountPaths := map[string]struct{}{}
	requested := make(map[string]struct{}, len(spec.MountDomains))
	for _, raw := range spec.MountDomains {
		requested[strings.TrimSpace(raw)] = struct{}{}
	}
	for _, domain := range adapter.VolumeDomains {
		if domain.Name == VolumeDomainWorkspace {
			if !shouldMountWorkspace(spec) {
				continue
			}
		} else if len(requested) > 0 {
			if _, ok := requested[domain.Name]; !ok {
				continue
			}
		} else {
			continue
		}
		if studentAccessibleContainer(spec) && domain.StudentAccess == VolumeAccessNone {
			continue
		}
		if domain.Name == VolumeDomainJudgePrivate && studentAccessibleContainer(spec) {
			continue
		}
		writable := false
		for _, raw := range spec.WritableMountDomains {
			if strings.TrimSpace(raw) == domain.Name {
				writable = true
				break
			}
		}
		mounts = appendUniqueVolumeMount(mounts, seenMountPaths, volumeMountForDomain(domain, writable))
	}
	for _, mount := range spec.EphemeralMounts {
		mounts = appendUniqueVolumeMount(mounts, seenMountPaths, corev1.VolumeMount{
			Name:      runtimeEphemeralVolumeName(spec.Name, mount.Name),
			MountPath: path.Clean(mount.MountPath),
		})
	}
	return mounts
}

// appendUniqueVolumeMount 以 mountPath 为容器内唯一键,防止 WorkloadSpec 临时卷与运行时卷域重复挂载。
func appendUniqueVolumeMount(mounts []corev1.VolumeMount, seen map[string]struct{}, mount corev1.VolumeMount) []corev1.VolumeMount {
	mount.MountPath = path.Clean(mount.MountPath)
	if _, ok := seen[mount.MountPath]; ok {
		return mounts
	}
	seen[mount.MountPath] = struct{}{}
	return append(mounts, mount)
}

// runtimeEphemeralVolumeName 构造运行时/私有 sidecar 的临时卷名。
func runtimeEphemeralVolumeName(containerName, mountName string) string {
	return strings.TrimRight("rt-"+containerName+"-"+mountName, "-")
}

// volumeMountForDomain 根据卷域访问级别构造挂载,公开素材等只读域必须在 K8s 层强制只读。
func volumeMountForDomain(domain VolumeDomainSpec, writable bool) corev1.VolumeMount {
	readOnly := !writable
	// 工作区是唯一默认允许学生入口读写的域;其余域必须显式声明写权限。
	if domain.Name == VolumeDomainWorkspace && domain.StudentAccess == VolumeAccessReadWrite {
		readOnly = false
	}
	return corev1.VolumeMount{
		Name:      domain.Name,
		MountPath: domain.MountPath,
		ReadOnly:  readOnly,
	}
}

// studentAccessibleContainer 判断容器是否声明允许学生通过终端进入。
func studentAccessibleContainer(spec workload.ComponentSpec) bool {
	return strings.EqualFold(strings.TrimSpace(spec.Labels[studentAccessLabel]), "true")
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
				"chaimir.io/sandbox-id": ids.Format(sb.ID),
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
	return path.Base(sb.SnapshotRef)
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

// servicesForPlan 由编译、冻结装载和资源创建共用，冲突必须在任何资源写入前拒绝。
func servicesForPlan(plan CreateSandboxPlan) ([]*corev1.Service, error) {
	var services []*corev1.Service
	seen := map[string]bool{}
	appendService := func(name, role string, ports []corev1.ServicePort) error {
		if problems := validation.IsDNS1035Label(name); len(problems) != 0 {
			return fmt.Errorf("Service 名称 %s 无效: %s", name, strings.Join(problems, "; "))
		}
		if seen[name] {
			return fmt.Errorf("沙箱 Service 名称 %s 重复", name)
		}
		seen[name] = true
		services = append(services, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: plan.Sandbox.Namespace, Labels: sandboxManagedLabels(plan.Sandbox)},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Selector: sandboxPodRoleLabels(plan.Sandbox, role), Ports: ports},
		})
		return nil
	}
	for _, pod := range podGroupForPlan(plan) {
		ports, err := servicePortsForPod(pod)
		if err != nil {
			return nil, fmt.Errorf("运行时 Pod %s 的 Service 端口无效: %w", pod.Name, err)
		}
		if len(ports) == 0 {
			continue
		}
		if err := appendService(runtimeServiceName(pod.Name), pod.Name, ports); err != nil {
			return nil, err
		}
	}
	for _, tool := range plan.Tools {
		if err := validateToolServicesAndRoutes(&tool.ResourceSpec); err != nil {
			return nil, fmt.Errorf("组件 %s 的 Service 或路由无效: %w", tool.Code, err)
		}
		for _, service := range tool.ResourceSpec.Services {
			ports, err := servicePortsForSpec(service.Ports)
			if err != nil {
				return nil, fmt.Errorf("工具 Service %s 端口无效: %w", service.Name, err)
			}
			if err := appendService(service.Name, toolComponentPodName(tool.Code, service.Component), ports); err != nil {
				return nil, err
			}
		}
	}
	return services, nil
}

// sandboxPodRoleLabels 返回 NetworkPolicy 和 Service 共同使用的受控 Pod 选择标签。
func sandboxPodRoleLabels(sb Sandbox, role string) map[string]string {
	return map[string]string{
		"app":                   "chaimir",
		"module":                "sandbox",
		"chaimir.io/sandbox-id": ids.Format(sb.ID),
		"chaimir.io/pod-role":   role,
	}
}

// servicePortsForPod 汇总运行时 Pod 中显式声明的端口,用于内部服务发现。
func servicePortsForPod(pod workload.PodSpec) ([]corev1.ServicePort, error) {
	var ports []workload.ServicePortSpec
	seenNames := map[string]struct{}{}
	for _, container := range pod.Containers {
		for _, port := range container.Ports {
			if port.Name == "" || port.ContainerPort <= 0 || port.ContainerPort > 65535 {
				return nil, fmt.Errorf("组件 %s 的容器端口声明无效", container.Name)
			}
			if _, ok := seenNames[port.Name]; ok {
				return nil, fmt.Errorf("重复 Service 端口名 %s", port.Name)
			}
			seenNames[port.Name] = struct{}{}
			if port.ServicePort == 0 {
				continue
			}
			ports = append(ports, workload.ServicePortSpec{Name: port.Name, Port: port.ServicePort, TargetPort: port.Name, Protocol: port.Protocol})
		}
	}
	return servicePortsForSpec(ports)
}

// servicePortsForSpec 统一检查并转换运行时和工具的 Service 端口。
func servicePortsForSpec(items []workload.ServicePortSpec) ([]corev1.ServicePort, error) {
	ports := make([]corev1.ServicePort, 0, len(items))
	seenNames := map[string]struct{}{}
	seenPorts := map[string]struct{}{}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		targetPort := strings.TrimSpace(item.TargetPort)
		if !portNamePattern.MatchString(name) || !portNamePattern.MatchString(targetPort) || item.Port < 1 || item.Port > 65535 {
			return nil, fmt.Errorf("Service 端口声明无效: name=%q target_port=%q port=%d", name, targetPort, item.Port)
		}
		protocol := corev1.ProtocolTCP
		switch strings.ToUpper(strings.TrimSpace(item.Protocol)) {
		case "", "TCP":
		case "UDP":
			protocol = corev1.ProtocolUDP
		default:
			return nil, fmt.Errorf("Service 端口 %s 的协议无效", name)
		}
		if _, exists := seenNames[name]; exists {
			return nil, fmt.Errorf("重复 Service 端口名 %s", name)
		}
		portKey := fmt.Sprintf("%d/%s", item.Port, protocol)
		if _, exists := seenPorts[portKey]; exists {
			return nil, fmt.Errorf("重复 Service 端口 %s", portKey)
		}
		seenNames[name] = struct{}{}
		seenPorts[portKey] = struct{}{}
		ports = append(ports, corev1.ServicePort{
			Name:       name,
			Port:       item.Port,
			TargetPort: intstr.FromString(targetPort),
			Protocol:   protocol,
		})
	}
	return ports, nil
}

// runtimeServiceName 生成运行时 Pod 的内部服务名,与工具 env 中的服务发现口径保持一致。
func runtimeServiceName(podName string) string {
	return boundedSandboxName("runtime-"+strings.TrimSpace(podName), 63)
}

// toolControlPlanePorts 汇总平台代理路由实际需要访问的工具组件端口。
func toolControlPlanePorts(tool Tool) map[string][]netv1.NetworkPolicyPort {
	componentPorts := componentPortMap(tool.ResourceSpec.Components)
	rolePorts := map[string][]netv1.NetworkPolicyPort{}
	for _, service := range tool.ResourceSpec.Services {
		role := toolComponentPodName(tool.Code, service.Component)
		for _, servicePort := range service.Ports {
			if !slices.ContainsFunc(tool.ResourceSpec.Routes, func(route workload.RouteSpec) bool {
				return route.Service == service.Name && route.Port == servicePort.Name
			}) {
				continue
			}
			port := componentPorts[service.Component][servicePort.TargetPort]
			if port <= 0 || slices.ContainsFunc(rolePorts[role], func(existing netv1.NetworkPolicyPort) bool { return existing.Port.IntVal == port }) {
				continue
			}
			rolePorts[role] = append(rolePorts[role], networkPolicyPort(port))
		}
	}
	return rolePorts
}

// toolComponentPodName 生成工具组件 Pod 名,避免回退到旧的单工具 Pod 命名。
func toolComponentPodName(toolCode, componentName string) string {
	return boundedSandboxName("tool-"+strings.TrimSpace(toolCode)+"-"+strings.TrimSpace(componentName), 63)
}

// runtimePodNames 只等待本阶段的运行时和基础设施;其他工具按当前状态开放入口。
func runtimePodNames(plan CreateSandboxPlan, created []*corev1.Pod) []string {
	pods := podGroupForPlan(plan)
	var names []string
	for _, pod := range pods {
		names = append(names, pod.Name)
	}
	for _, tool := range plan.Tools {
		if tool.Category != "infra" {
			continue
		}
		for _, component := range tool.ResourceSpec.Components {
			name := toolComponentPodName(tool.Code, component.Name)
			if slices.ContainsFunc(created, func(pod *corev1.Pod) bool { return pod.Name == name }) {
				names = append(names, name)
			}
		}
	}
	return names
}

// runtimeComputePodNames 选择包含运行时主容器的 Pod,学生工作区独立 Pod 不参与链重置。
func runtimeComputePodNames(plan CreateSandboxPlan, runtimeInstance string) []string {
	result := []string{}
	for _, pod := range podGroupForPlan(plan) {
		runtimePlan, ok := runtimePlanForPod(plan, pod.Name)
		if !ok || runtimePlan.InstanceCode != runtimeInstance {
			continue
		}
		for _, container := range pod.Containers {
			if container.Name == runtimePlan.Runtime.AdapterSpec.RuntimeContainer.Name {
				result = append(result, pod.Name)
				break
			}
		}
	}
	return result
}

// waitPodDeleted 等待同名 Pod 消失后再允许重建。
func waitPodDeleted(ctx context.Context, cs kubernetes.Interface, namespace, name string, pollSeconds int) error {
	if pollSeconds <= 0 {
		return fmt.Errorf("SANDBOX_READY_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	ticker := time.NewTicker(sandboxPodReadyPollInterval(pollSeconds))
	defer ticker.Stop()
	for {
		_, err := cs.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("查询待删除的 Pod %s 失败: %w", name, err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待 Pod %s 删除超时: %w", name, ctx.Err())
		case <-ticker.C:
		}
	}
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
func (o *K8sOrchestrator) prepullDaemonSet(image RuntimeImage, compositionDigest string, specs []PrepullImageSpec) *appsv1.DaemonSet {
	labels := map[string]string{"app": "chaimir", "module": "sandbox", "runtime_image_id": ids.Format(image.ID)}
	digestPrefix := strings.TrimPrefix(strings.TrimSpace(compositionDigest), "sha256:")
	if len(digestPrefix) > 16 {
		digestPrefix = digestPrefix[:16]
	}
	if digestPrefix == "" {
		digestPrefix = "unknown"
	}
	labels["composition_digest"] = digestPrefix
	automount := false
	initContainers := make([]corev1.Container, 0, len(specs))
	for idx, spec := range specs {
		if spec.Hold {
			continue
		}
		initContainers = append(initContainers, corev1.Container{
			Name:            prepullContainerName(idx, spec.ImageURL),
			Image:           spec.ImageURL,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         spec.Command,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(o.cfg.PrepullRequestCPU), corev1.ResourceMemory: resource.MustParse(o.cfg.PrepullRequestMemory)},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(o.cfg.PrepullLimitCPU), corev1.ResourceMemory: resource.MustParse(o.cfg.PrepullLimitMemory)},
			},
			SecurityContext: containerSecurityContext(true),
			VolumeMounts:    prepullVolumeMounts(spec),
		})
	}
	holdSpec := prepullHoldSpec(specs)
	hold := corev1.Container{
		Name:            "prepull-ready",
		Image:           holdSpec.ImageURL,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         prepullHoldCommand(holdSpec.Command),
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(o.cfg.PrepullRequestCPU), corev1.ResourceMemory: resource.MustParse(o.cfg.PrepullRequestMemory)},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(o.cfg.PrepullLimitCPU), corev1.ResourceMemory: resource.MustParse(o.cfg.PrepullLimitMemory)},
		},
		SecurityContext: containerSecurityContext(true),
		VolumeMounts:    prepullVolumeMounts(holdSpec),
	}
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "chaimir-prepull-" + ids.Format(image.ID) + "-" + digestPrefix, Namespace: o.cfg.PrepullNamespace, Labels: labels},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: &automount,
					RestartPolicy:                corev1.RestartPolicyAlways,
					ImagePullSecrets:             imagePullSecrets(o.cfg.ImagePullSecretNames),
					NodeSelector:                 copyStringMap(o.cfg.SandboxNodeSelector),
					Tolerations:                  sandboxTolerations(o.cfg.SandboxNodeTolerations),
					InitContainers:               initContainers,
					Containers:                   []corev1.Container{hold},
					SecurityContext:              podSecurityContext(),
					Volumes:                      prepullVolumes(specs),
				},
			},
		},
	}
}

// prepullHoldCommand 先执行镜像自检,成功后让保持容器持续运行以维持节点就绪。
func prepullHoldCommand(command []string) []string {
	parts := make([]string, 0, len(command))
	for _, part := range command {
		parts = append(parts, shellQuote(part))
	}
	return []string{"sh", "-lc", strings.Join(parts, " ") + " && exec sleep 2147483647"}
}

// shellQuote 将受控 argv 参数编码为单引号 shell 字面量。
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

// prepullVolumeMounts 提供镜像自检所需的最小临时工作目录,并复用镜像清单声明的临时可写目录。
func prepullVolumeMounts(spec PrepullImageSpec) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{Name: "prepull-workspace", MountPath: "/workspace"},
		{Name: "prepull-tmp", MountPath: "/tmp"},
	}
	seenPaths := map[string]struct{}{"/workspace": {}, "/tmp": {}}
	for _, mount := range spec.EphemeralMounts {
		mountPath := path.Clean(strings.TrimSpace(mount.MountPath))
		if !strings.HasPrefix(mountPath, "/") {
			continue
		}
		if _, exists := seenPaths[mountPath]; exists {
			continue
		}
		seenPaths[mountPath] = struct{}{}
		mounts = append(mounts, corev1.VolumeMount{
			Name:      prepullExtraVolumeName(mount),
			MountPath: mountPath,
		})
	}
	return mounts
}

// prepullVolumes 为预拉取容器提供只在当前 Pod 生命周期内存在的临时卷。
func prepullVolumes(specs []PrepullImageSpec) []corev1.Volume {
	volumes := []corev1.Volume{
		{Name: "prepull-workspace", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		{Name: "prepull-tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}
	seen := map[string]struct{}{"prepull-workspace": {}, "prepull-tmp": {}}
	for _, spec := range specs {
		for _, mount := range spec.EphemeralMounts {
			if mountPath := path.Clean(strings.TrimSpace(mount.MountPath)); !strings.HasPrefix(mountPath, "/") {
				continue
			}
			name := prepullExtraVolumeName(mount)
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			volumes = append(volumes, corev1.Volume{
				Name:         name,
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			})
		}
	}
	return volumes
}

// prepullExtraVolumeName 为预拉取临时挂载生成稳定卷名,避免长名称和同名路径冲突。
func prepullExtraVolumeName(mount workload.EphemeralMountSpec) string {
	key := strings.TrimSpace(mount.Name) + "\x00" + strings.TrimSpace(mount.MountPath)
	sum := sha256.Sum256([]byte(key))
	suffix := hex.EncodeToString(sum[:4])
	base := "prepull-" + strings.TrimSpace(mount.Name)
	base = strings.ToLower(base)
	var b strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	base = strings.Trim(b.String(), "-")
	if base == "" || base == "prepull" {
		base = "prepull-mount"
	}
	maxBase := 63 - len(suffix) - 1
	if len(base) > maxBase {
		base = base[:maxBase]
	}
	return strings.TrimRight(base, "-") + "-" + suffix
}

// prepullHoldSpec 选择运行时声明的长期保持容器,确保 DaemonSet Ready 后新节点会持续补拉。
func prepullHoldSpec(specs []PrepullImageSpec) PrepullImageSpec {
	for _, spec := range specs {
		if spec.Hold {
			return spec
		}
	}
	return PrepullImageSpec{}
}

// prepullSpecImageURLs 提取预拉取状态响应中的镜像 URL 列表。
func prepullSpecImageURLs(specs []PrepullImageSpec) []string {
	out := make([]string, 0, len(specs))
	seen := map[string]struct{}{}
	for _, spec := range specs {
		imageURL := strings.TrimSpace(spec.ImageURL)
		if imageURL == "" {
			continue
		}
		if _, exists := seen[imageURL]; exists {
			continue
		}
		seen[imageURL] = struct{}{}
		out = append(out, imageURL)
	}
	return out
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

// imagePullSecrets 把配置中的 registry 拉取凭据名称转换为 Kubernetes 引用。
func imagePullSecrets(names []string) []corev1.LocalObjectReference {
	if len(names) == 0 {
		return nil
	}
	out := make([]corev1.LocalObjectReference, 0, len(names))
	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			out = append(out, corev1.LocalObjectReference{Name: trimmed})
		}
	}
	return out
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

// envVarsWithSecrets 合并非敏感字面量和受控 SecretKeyRef 环境变量。
func envVarsWithSecrets(items []workload.EnvVarSpec, secrets []workload.SecretEnvVarSpec) []corev1.EnvVar {
	out := envVars(items)
	for _, item := range secrets {
		out = append(out, corev1.EnvVar{Name: item.Name, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: item.SecretName},
			Key:                  item.SecretKey,
		}}})
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
