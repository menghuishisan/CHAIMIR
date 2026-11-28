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
	"chaimir/pkg/terminal"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	netv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	podresource "k8s.io/component-helpers/resource"
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
	created, err := createPodsInStartupOrder(plan, orderedPods, func(pod *corev1.Pod) (*corev1.Pod, error) {
		return createOrVerifyPod(ctx, cs, pod)
	}, func(pods []*corev1.Pod) error {
		return o.waitPodsReady(ctx, cs, pods)
	})
	if err != nil {
		return err
	}
	return o.waitPodsReady(ctx, cs, runtimePods(plan, created))
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
			if err := deleteCreatedSnapshot(cleanupCtx, client, snapshot); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("清理未完成快照 %s 失败: %w", snapshot.GetName(), err))
			}
		}
	}()
	for _, domain := range domains {
		name := volumeSnapshotName(plan.Sandbox, domain)
		obj := volumeSnapshotObject(plan.Sandbox, domain, name, retention, o.cfg.VolumeSnapshotClassName)
		snapshot, newlyCreated, err := createOrVerifyVolumeSnapshot(ctx, client, obj)
		if newlyCreated && snapshot != nil {
			created = append(created, snapshot)
		}
		if err != nil {
			return SnapshotResult{}, fmt.Errorf("创建 VolumeSnapshot 失败: %w", err)
		}
		if err := o.waitVolumeSnapshotReady(ctx, gvr, snapshot); err != nil {
			return SnapshotResult{}, err
		}
	}
	return SnapshotResult{Ref: plan.Sandbox.Namespace + "/" + snapshotGroupName(plan.Sandbox), Domains: domains}, nil
}

func deleteCreatedSnapshot(ctx context.Context, client dynamic.ResourceInterface, accepted *unstructured.Unstructured) error {
	current, err := client.Get(ctx, accepted.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if current.GetUID() != accepted.GetUID() {
		return fmt.Errorf("VolumeSnapshot %s 已被替换，拒绝补偿删除", accepted.GetName())
	}
	if err := verifyVolumeSnapshot(current, accepted); err != nil {
		return err
	}
	options, err := resourceDeleteOptions(current)
	if err != nil {
		return err
	}
	if err := client.Delete(ctx, current.GetName(), options); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (o *K8sOrchestrator) cleanupSupersededSnapshots(ctx context.Context, sb Sandbox) error {
	if err := validateSnapshotRefForNamespace(sb); err != nil {
		return err
	}
	gvr := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	client := o.client.Dynamic().Resource(gvr).Namespace(sb.Namespace)
	managed := sandboxManagedLabels(sb)
	snapshots, err := client.List(ctx, metav1.ListOptions{LabelSelector: labels.Set(managed).AsSelector().String()})
	if err != nil {
		return fmt.Errorf("列出旧快照失败: %w", err)
	}
	type deletion struct {
		name    string
		options metav1.DeleteOptions
	}
	var targets []deletion
	for index := range snapshots.Items {
		snapshot := &snapshots.Items[index]
		domain := snapshot.GetLabels()["chaimir.io/volume"]
		desired := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: snapshot.GetName(), Namespace: sb.Namespace, Labels: managed}}
		if platformk8s.VerifyManagedLabels(snapshot, desired) != nil || domain == "" {
			continue
		}
		if snapshot.GetName() == volumeSnapshotName(sb, domain) {
			continue
		}
		if err := platformk8s.VerifyManagedResource(snapshot, desired); err != nil {
			return err
		}
		options, err := resourceDeleteOptions(snapshot)
		if err != nil {
			return err
		}
		targets = append(targets, deletion{name: snapshot.GetName(), options: options})
	}
	for _, target := range targets {
		if err := client.Delete(ctx, target.name, target.options); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("删除旧快照 %s 失败: %w", target.name, err)
		}
	}
	return nil
}

// waitVolumeSnapshotReady 等待 CSI 快照 readyToUse,避免记录尚不可恢复的快照引用。
func (o *K8sOrchestrator) waitVolumeSnapshotReady(ctx context.Context, gvr schema.GroupVersionResource, accepted *unstructured.Unstructured) error {
	if o.cfg.ReadyPollIntervalSeconds <= 0 {
		return fmt.Errorf("SANDBOX_READY_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	contentGVR := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotcontents"}
	ticker := time.NewTicker(time.Duration(o.cfg.ReadyPollIntervalSeconds) * time.Second)
	defer ticker.Stop()
	namespace, name := accepted.GetNamespace(), accepted.GetName()
	for {
		current, err := o.client.Dynamic().Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("查询 VolumeSnapshot 状态失败: %w", err)
		}
		if current.GetUID() != accepted.GetUID() {
			return fmt.Errorf("VolumeSnapshot %s 在等待期间已被替换或修改", name)
		}
		if err := verifyVolumeSnapshot(current, accepted); err != nil {
			return err
		}
		if class, found, err := unstructured.NestedString(current.Object, "spec", "volumeSnapshotClassName"); err == nil && found {
			// 控制器可补齐默认类；一旦观察到选择结果，后续轮询固定为该类。
			accepted = accepted.DeepCopy()
			if err := unstructured.SetNestedField(accepted.Object, class, "spec", "volumeSnapshotClassName"); err != nil {
				return fmt.Errorf("记录 VolumeSnapshot 默认类失败: %w", err)
			}
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
		addResourceUsage(&usage, corev1.ResourceRequirements{
			Requests: podresource.PodRequests(&pod, podresource.PodResourcesOptions{}),
			Limits:   podresource.PodLimits(&pod, podresource.PodResourcesOptions{}),
		})
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
	if err := platformk8s.VerifyManagedLabels(pvc, expectedPVC); err != nil {
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
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: plan.Sandbox.Namespace, Labels: sandboxPodRoleLabels(plan.Sandbox, podName)},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: &automount, EnableServiceLinks: &serviceLinks,
			ImagePullSecrets: imagePullSecrets(o.cfg.ImagePullSecretNames),
			RuntimeClassName: &o.cfg.RuntimeClass, ServiceAccountName: sandboxWorkloadServiceAccount,
			RestartPolicy: corev1.RestartPolicyNever, SecurityContext: podSecurityContext(),
			NodeSelector: copyStringMap(o.cfg.SandboxNodeSelector), Tolerations: sandboxTolerations(o.cfg.SandboxNodeTolerations),
			Containers: []corev1.Container{{
				Name: containerName, Image: component.ImageURL, ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{"sleep", "2147483647"}, Resources: resources(component.Resources, o.cfg),
				SecurityContext: containerSecurityContext(true),
				VolumeMounts:    []corev1.VolumeMount{{Name: VolumeDomainWorkspace, MountPath: runtime.Runtime.AdapterSpec.WorkspaceDir, ReadOnly: true}},
			}},
			Volumes: []corev1.Volume{{Name: VolumeDomainWorkspace, VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: VolumeDomainWorkspace},
			}}},
		},
	}
	created, err := createOrVerifyPod(ctx, cs, pod)
	if err != nil {
		return fmt.Errorf("创建工作区归档 Pod 失败: %w", err)
	}
	return o.waitPodsReady(ctx, cs, []*corev1.Pod{created})
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

func (o *K8sOrchestrator) waitPodsReady(ctx context.Context, cs kubernetes.Interface, expected []*corev1.Pod) error {
	if o.cfg.ReadyPollIntervalSeconds <= 0 {
		return fmt.Errorf("SANDBOX_READY_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	ticker := time.NewTicker(sandboxPodReadyPollInterval(o.cfg.ReadyPollIntervalSeconds))
	defer ticker.Stop()
	for {
		ready := true
		for _, accepted := range expected {
			if accepted == nil || accepted.UID == "" {
				return fmt.Errorf("沙箱 Pod 缺少已受理身份")
			}
			pod, err := cs.CoreV1().Pods(accepted.Namespace).Get(ctx, accepted.Name, metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("查询沙箱 Pod 状态失败: %w", err)
			}
			if err == nil {
				if pod.UID != accepted.UID {
					return fmt.Errorf("沙箱 Pod %s 的 UID 已改变", accepted.Name)
				}
				if err := platformk8s.VerifyManagedLabels(pod, accepted); err != nil {
					return err
				}
				if pod.DeletionTimestamp != nil || pod.Status.Phase == corev1.PodSucceeded || !platformk8s.SamePodExecutionSpec(pod.Spec, accepted.Spec) {
					return fmt.Errorf("沙箱 Pod %s 已终止、正在删除或执行规格改变", accepted.Name)
				}
				if failed := podFailureReason(pod); failed != "" {
					return fmt.Errorf("沙箱 Pod %s 失败: %s", accepted.Name, failed)
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
func (o *K8sOrchestrator) toolServiceEndpointsReady(ctx context.Context, namespace string, service workload.ServiceSpec, pod *corev1.Pod, targetPorts map[string]workload.PortSpec) error {
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
					*endpointPort.Port == targetPorts[port.TargetPort.StrVal].ContainerPort && *endpointPort.Port > 0 && protocol == port.Protocol {
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

// createOrVerifyPod 创建或核对 Pod,防止旧 Pod 以相同名称承载不同镜像或挂载配置。
func createOrVerifyPod(ctx context.Context, cs kubernetes.Interface, desired *corev1.Pod) (*corev1.Pod, error) {
	client := cs.CoreV1().Pods(desired.Namespace)
	existing, err := client.Create(ctx, desired, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		existing, err = client.Get(ctx, desired.Name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, err
	}
	if err := platformk8s.VerifyManagedResource(existing, desired); err != nil {
		return nil, err
	}
	if !platformk8s.SamePodExecutionSpec(existing.Spec, desired.Spec) {
		return nil, fmt.Errorf("已存在 Pod %s 的工作负载规格与当前组合不一致", desired.Name)
	}
	return existing, nil
}

// createOrVerifyResourceQuota 创建或核对 Namespace 资源硬限,不接受旧配额继续承载新组合。
func createOrVerifyResourceQuota(ctx context.Context, cs kubernetes.Interface, desired *corev1.ResourceQuota) error {
	client := cs.CoreV1().ResourceQuotas(desired.Namespace)
	existing, err := client.Create(ctx, desired, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		existing, err = client.Get(ctx, desired.Name, metav1.GetOptions{})
	}
	if err != nil {
		return fmt.Errorf("准备 ResourceQuota 失败: %w", err)
	}
	if err := platformk8s.VerifyManagedResource(existing, desired); err != nil {
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
	existing, err := client.Create(ctx, desired, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		existing, err = client.Get(ctx, desired.Name, metav1.GetOptions{})
	}
	if err != nil {
		return fmt.Errorf("准备 LimitRange 失败: %w", err)
	}
	if err := platformk8s.VerifyManagedResource(existing, desired); err != nil {
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
	created := false
	if apierrors.IsNotFound(err) && allowCreate {
		existing, err = client.Create(ctx, desired, metav1.CreateOptions{})
		created = err == nil
		if apierrors.IsAlreadyExists(err) {
			existing, err = client.Get(ctx, desired.Name, metav1.GetOptions{})
		}
	}
	if err != nil {
		return fmt.Errorf("准备沙箱卷 PVC %s 失败: %w", desired.Name, err)
	}
	if err := platformk8s.VerifyManagedResource(existing, desired); err != nil {
		return err
	}
	if !sameSandboxPVCSpec(existing.Spec, desired.Spec) {
		return fmt.Errorf("已存在 PVC %s 的卷规格与当前组合不一致", desired.Name)
	}
	if created && !sameSandboxPVCSource(existing.Spec, desired.Spec, desired.Namespace) {
		return fmt.Errorf("新建 PVC %s 的恢复来源与当前组合不一致", desired.Name)
	}
	return nil
}

// createOrVerifyService 核对服务发现声明，允许服务器分配集群地址。
func createOrVerifyService(ctx context.Context, cs kubernetes.Interface, desired *corev1.Service) error {
	client := cs.CoreV1().Services(desired.Namespace)
	existing, err := client.Create(ctx, desired, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		existing, err = client.Get(ctx, desired.Name, metav1.GetOptions{})
	}
	if err != nil {
		return err
	}
	if err := platformk8s.VerifyManagedResource(existing, desired); err != nil {
		return err
	}
	if !apiequality.Semantic.DeepEqual(normalizeSandboxServiceSpec(existing.Spec), normalizeSandboxServiceSpec(desired.Spec)) {
		return fmt.Errorf("Service %s 的规格与当前组合不一致", desired.Name)
	}
	return nil
}

func normalizeSandboxServiceSpec(spec corev1.ServiceSpec) corev1.ServiceSpec {
	spec = *spec.DeepCopy()
	if spec.ClusterIP != corev1.ClusterIPNone {
		spec.ClusterIP = ""
	}
	spec.ClusterIPs, spec.IPFamilies = nil, nil
	if spec.Type == "" {
		spec.Type = corev1.ServiceTypeClusterIP
	}
	if spec.SessionAffinity == "" {
		spec.SessionAffinity = corev1.ServiceAffinityNone
	}
	if spec.InternalTrafficPolicy == nil {
		policy := corev1.ServiceInternalTrafficPolicyCluster
		spec.InternalTrafficPolicy = &policy
	}
	if spec.IPFamilyPolicy == nil {
		policy := corev1.IPFamilyPolicySingleStack
		spec.IPFamilyPolicy = &policy
	}
	for index := range spec.Ports {
		port := &spec.Ports[index]
		if port.Protocol == "" {
			port.Protocol = corev1.ProtocolTCP
		}
		if port.TargetPort == intstr.FromInt32(0) || port.TargetPort == intstr.FromString("") {
			port.TargetPort = intstr.FromInt32(port.Port)
		}
	}
	return spec
}

// createOrVerifyVolumeSnapshot 创建或核对快照对象,避免恢复到错误 PVC 或错误保留策略。
func createOrVerifyVolumeSnapshot(ctx context.Context, client dynamic.ResourceInterface, desired *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	existing, err := client.Create(ctx, desired, metav1.CreateOptions{})
	created := err == nil
	if apierrors.IsAlreadyExists(err) {
		existing, err = client.Get(ctx, desired.GetName(), metav1.GetOptions{})
	}
	if err != nil {
		return nil, false, err
	}
	if err := platformk8s.VerifyManagedResource(existing, desired); err != nil {
		return nil, false, err
	}
	return existing, created, verifyVolumeSnapshot(existing, desired)
}

func verifyVolumeSnapshot(existing, desired *unstructured.Unstructured) error {
	if err := platformk8s.VerifyManagedResource(existing, desired); err != nil {
		return err
	}
	for key, value := range desired.GetAnnotations() {
		if existing.GetAnnotations()[key] != value {
			return fmt.Errorf("VolumeSnapshot %s 的保留策略不一致", desired.GetName())
		}
	}
	desiredSpec, desiredFound, desiredErr := unstructured.NestedMap(desired.Object, "spec")
	existingSpec, existingFound, existingErr := unstructured.NestedMap(existing.Object, "spec")
	if _, specified := desiredSpec["volumeSnapshotClassName"]; !specified {
		if class, found := existingSpec["volumeSnapshotClassName"]; found {
			if value, valid := class.(string); !valid || strings.TrimSpace(value) == "" {
				return fmt.Errorf("VolumeSnapshot %s 的默认快照类无效", desired.GetName())
			}
			delete(existingSpec, "volumeSnapshotClassName")
		}
	}
	if desiredErr != nil || existingErr != nil || !desiredFound || !existingFound || !apiequality.Semantic.DeepEqual(existingSpec, desiredSpec) {
		return fmt.Errorf("VolumeSnapshot %s 的来源与当前组合不一致", desired.GetName())
	}
	return nil
}

// sameSandboxPVCSpec 核对可复用规格；快照数据源只记录卷最初的创建来源，不代表当前内容。
func sameSandboxPVCSpec(current, desired corev1.PersistentVolumeClaimSpec) bool {
	current, desired = *current.DeepCopy(), *desired.DeepCopy()
	current.DataSource, desired.DataSource = nil, nil
	current.DataSourceRef, desired.DataSourceRef = nil, nil
	if desired.StorageClassName == nil {
		current.StorageClassName = nil
	}
	if desired.VolumeName == "" {
		current.VolumeName = ""
	}
	filesystem := corev1.PersistentVolumeFilesystem
	if current.VolumeMode == nil {
		current.VolumeMode = &filesystem
	}
	if desired.VolumeMode == nil {
		desired.VolumeMode = &filesystem
	}
	return apiequality.Semantic.DeepEqual(current, desired)
}

func sameSandboxPVCSource(current, desired corev1.PersistentVolumeClaimSpec, namespace string) bool {
	currentSource, currentValid := sandboxPVCSource(current, namespace)
	desiredSource, desiredValid := sandboxPVCSource(desired, namespace)
	return currentValid && desiredValid && apiequality.Semantic.DeepEqual(currentSource, desiredSource)
}

// sandboxPVCSource 合并 API Server 自动互补的本地来源字段，保留跨命名空间来源边界。
func sandboxPVCSource(spec corev1.PersistentVolumeClaimSpec, namespace string) (*corev1.TypedObjectReference, bool) {
	normalize := func(source *corev1.TypedObjectReference) *corev1.TypedObjectReference {
		if source == nil {
			return nil
		}
		source = source.DeepCopy()
		if source.Namespace != nil && (*source.Namespace == "" || *source.Namespace == namespace) {
			source.Namespace = nil
		}
		if source.APIGroup != nil && *source.APIGroup == "" {
			source.APIGroup = nil
		}
		return source
	}
	reference := normalize(spec.DataSourceRef)
	if spec.DataSource == nil {
		return reference, true
	}
	local := normalize(&corev1.TypedObjectReference{APIGroup: spec.DataSource.APIGroup, Kind: spec.DataSource.Kind, Name: spec.DataSource.Name})
	return local, reference == nil || apiequality.Semantic.DeepEqual(local, reference)
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
	created := false
	if apierrors.IsNotFound(err) {
		existing, err = client.Create(ctx, sa, metav1.CreateOptions{})
		created = err == nil
		if apierrors.IsAlreadyExists(err) {
			existing, err = client.Get(ctx, sa.Name, metav1.GetOptions{})
		}
	}
	if err != nil {
		return fmt.Errorf("准备沙箱 ServiceAccount 失败: %w", err)
	}
	if err := platformk8s.VerifyManagedResource(existing, sa); err != nil {
		return err
	}
	if !created && (!slices.Equal(existing.ImagePullSecrets, refs) ||
		existing.AutomountServiceAccountToken == nil ||
		*existing.AutomountServiceAccountToken) {
		updated := existing.DeepCopy()
		updated.ImagePullSecrets = refs
		updated.AutomountServiceAccountToken = &automount
		accepted, err := client.Update(ctx, updated, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("更新沙箱 ServiceAccount 失败: %w", err)
		}
		if accepted.UID != existing.UID {
			return fmt.Errorf("沙箱 ServiceAccount %s 在更新期间已被替换", sa.Name)
		}
		existing = accepted
	}
	if err := platformk8s.VerifyManagedResource(existing, sa); err != nil {
		return err
	}
	if !slices.Equal(existing.ImagePullSecrets, refs) || existing.AutomountServiceAccountToken == nil || *existing.AutomountServiceAccountToken {
		return fmt.Errorf("沙箱 ServiceAccount %s 的凭据策略与当前配置不一致", sa.Name)
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
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-deny-all", Namespace: plan.Sandbox.Namespace, Labels: sandboxManagedLabels(plan.Sandbox)},
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
	existing, err := policies.Create(ctx, desired, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		existing, err = policies.Get(ctx, desired.Name, metav1.GetOptions{})
	}
	if err != nil {
		return fmt.Errorf("准备 NetworkPolicy 失败: %w", err)
	}
	if err := platformk8s.VerifyManagedResource(existing, desired); err != nil {
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
			Labels:    sandboxManagedLabels(sb),
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
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: sb.Namespace, Labels: sandboxManagedLabels(sb)},
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
		ObjectMeta: metav1.ObjectMeta{Name: networkPolicyRuleName("sandbox-allow", rule, "ingress"), Namespace: plan.Sandbox.Namespace, Labels: sandboxManagedLabels(plan.Sandbox)},
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
		ObjectMeta: metav1.ObjectMeta{Name: networkPolicyRuleName("sandbox-allow", rule, "egress"), Namespace: plan.Sandbox.Namespace, Labels: sandboxManagedLabels(plan.Sandbox)},
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
		ObjectMeta: metav1.ObjectMeta{Name: networkPolicyRuleName("sandbox-allow-tool", workload.NetworkRuleSpec{Name: rule.Name, From: fromRole, To: toRole, Ports: rule.Ports}, "ingress"), Namespace: plan.Sandbox.Namespace, Labels: sandboxManagedLabels(plan.Sandbox)},
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
		ObjectMeta: metav1.ObjectMeta{Name: networkPolicyRuleName("sandbox-allow-tool", workload.NetworkRuleSpec{Name: rule.Name, From: fromRole, To: toRole, Ports: rule.Ports}, "egress"), Namespace: plan.Sandbox.Namespace, Labels: sandboxManagedLabels(plan.Sandbox)},
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
		items = append(items, strings.TrimSpace(port.Name)+"="+strconv.Itoa(int(port.Port))+"/"+valueOrDefault(port.Protocol, "TCP"))
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
func createPodsInStartupOrder(plan CreateSandboxPlan, pods []*corev1.Pod, create func(*corev1.Pod) (*corev1.Pod, error), waitReady func([]*corev1.Pod) error) ([]*corev1.Pod, error) {
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
			return nil, fmt.Errorf("沙箱 Pod %s 不属于声明的运行时或工具", pod.Name)
		}
		if !required[code] {
			code = ""
		}
		groups[code] = append(groups[code], pod)
	}
	var created []*corev1.Pod
	for _, code := range order {
		var group []*corev1.Pod
		for _, pod := range groups[code] {
			accepted, err := create(pod)
			if err != nil {
				return nil, fmt.Errorf("创建沙箱 Pod %s 失败: %w", pod.Name, err)
			}
			group = append(group, accepted)
		}
		if code != "" && len(group) > 0 {
			if err := waitReady(group); err != nil {
				return nil, err
			}
		}
		created = append(created, group...)
	}
	return created, nil
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
			ImagePullSecrets:             imagePullSecrets(o.cfg.ImagePullSecretNames),
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
			ImagePullSecrets:             imagePullSecrets(o.cfg.ImagePullSecretNames),
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
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "snapshot.storage.k8s.io/v1",
		"kind":       "VolumeSnapshot",
		"metadata": map[string]any{
			"name":      name,
			"namespace": sb.Namespace,
			"annotations": map[string]any{
				"chaimir.io/retention-seconds": fmt.Sprintf("%.0f", retention.Seconds()),
			},
		},
		"spec": spec,
	}}
	managed := sandboxManagedLabels(sb)
	managed["chaimir.io/volume"] = domain
	obj.SetLabels(managed)
	return obj
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
	labels := sandboxManagedLabels(sb)
	labels["chaimir.io/pod-role"] = role
	return labels
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
			protocol := corev1.Protocol(valueOrDefault(port.Protocol, "TCP"))
			if port.ContainerPort <= 0 || slices.ContainsFunc(rolePorts[role], func(existing netv1.NetworkPolicyPort) bool {
				return existing.Port.IntVal == port.ContainerPort && existing.Protocol != nil && *existing.Protocol == protocol
			}) {
				continue
			}
			rolePorts[role] = append(rolePorts[role], networkPolicyPort(port.ContainerPort, port.Protocol))
		}
	}
	return rolePorts
}

// toolComponentPodName 生成工具组件 Pod 名,避免回退到旧的单工具 Pod 命名。
func toolComponentPodName(toolCode, componentName string) string {
	return boundedSandboxName("tool-"+strings.TrimSpace(toolCode)+"-"+strings.TrimSpace(componentName), 63)
}

// runtimePods 只等待本阶段受理的运行时和基础设施；其他工具按当前状态开放入口。
func runtimePods(plan CreateSandboxPlan, created []*corev1.Pod) []*corev1.Pod {
	pods := podGroupForPlan(plan)
	waitFor := make(map[string]bool, len(pods))
	for _, pod := range pods {
		waitFor[pod.Name] = true
	}
	for _, tool := range plan.Tools {
		if tool.Category != "infra" {
			continue
		}
		for _, component := range tool.ResourceSpec.Components {
			waitFor[toolComponentPodName(tool.Code, component.Name)] = true
		}
	}
	var ready []*corev1.Pod
	for _, pod := range created {
		if waitFor[pod.Name] {
			ready = append(ready, pod)
		}
	}
	return ready
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

// splitExecTarget 把内部执行目标拆成 pod/container,未带 Pod 时使用单 Pod 拓扑名称。
func splitExecTarget(target string) (string, string) {
	parts := strings.Split(strings.TrimSpace(target), "/")
	if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return "sandbox", strings.TrimSpace(target)
}

// prepullDaemonSet 构造镜像预拉取 DaemonSet。
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

func networkPolicyPort(port int32, protocol string) netv1.NetworkPolicyPort {
	transport := corev1.Protocol(valueOrDefault(protocol, "TCP"))
	return netv1.NetworkPolicyPort{Protocol: &transport, Port: &intstr.IntOrString{Type: intstr.Int, IntVal: port}}
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
	seen := map[workload.NetworkPortRef]bool{}
	for _, pod := range pods {
		for _, container := range pod.Containers {
			for _, port := range container.Ports {
				identity := workload.NetworkPortRef{Port: port.ContainerPort, Protocol: valueOrDefault(port.Protocol, "TCP")}
				if seen[identity] {
					continue
				}
				seen[identity] = true
				ports = append(ports, networkPolicyPort(port.ContainerPort, port.Protocol))
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
		ports = append(ports, networkPolicyPort(ref.Port, ref.Protocol))
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
