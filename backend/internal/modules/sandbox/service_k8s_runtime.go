package sandbox

import (
	platformk8s "chaimir/internal/platform/k8s"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

type runtimeInitPod struct {
	pod      *corev1.Pod
	runtime  RuntimePlan
	archives []RuntimeInitArchive
}

func initAssetPodsForPlan(plan CreateSandboxPlan, pods []*corev1.Pod, archives []RuntimeInitArchive) ([]runtimeInitPod, error) {
	var jobs []runtimeInitPod
	for _, runtime := range plan.Runtimes {
		var selected []RuntimeInitArchive
		for _, archive := range archives {
			if archive.RuntimeInstance == runtime.InstanceCode {
				selected = append(selected, archive)
			}
		}
		if len(selected) == 0 {
			continue
		}
		adapter := runtime.Runtime.AdapterSpec
		if err := validateInitAssets(adapter); err != nil {
			return nil, err
		}
		if err := validateInitAssetExecTarget(adapter); err != nil {
			return nil, err
		}
		if err := validatePrivateArchiveOps(adapter.InitAssetOps); err != nil {
			return nil, err
		}
		targetPod, targetContainer := splitExecTarget(adapter.InitAssetOps.ExecTarget)
		var pod *corev1.Pod
		for _, source := range pods {
			if source.Name != targetPod {
				continue
			}
			for _, container := range source.Spec.Containers {
				if container.Name == targetContainer {
					pod = source.DeepCopy()
					pod.Spec.Containers = []corev1.Container{*container.DeepCopy()}
					break
				}
			}
		}
		if pod == nil {
			return nil, fmt.Errorf("运行时 %s 缺少初始化资产执行容器", runtime.InstanceCode)
		}
		allowed := map[string]bool{}
		for _, archive := range selected {
			if len(archive.Tarball) == 0 || !componentCanWriteDomain(adapter, adapter.InitAssetOps.ExecTarget, archive.TargetDomain) ||
				(archive.TargetDomain != VolumeDomainRuntimeState && archive.TargetDomain != VolumeDomainPublicAssets) {
				return nil, apperr.ErrSandboxInitAssetConfigInvalid
			}
			allowed[archive.TargetDomain] = true
		}
		container := &pod.Spec.Containers[0]
		container.Env, container.EnvFrom, container.Ports = nil, nil, nil
		container.ReadinessProbe, container.LivenessProbe, container.StartupProbe = nil, nil, nil
		container.VolumeMounts = slices.DeleteFunc(container.VolumeMounts, func(mount corev1.VolumeMount) bool {
			_, domain := volumeDomainByName(adapter, mount.Name)
			return domain && !allowed[mount.Name]
		})
		for domain := range allowed {
			if !slices.ContainsFunc(container.VolumeMounts, func(mount corev1.VolumeMount) bool { return mount.Name == domain && !mount.ReadOnly }) ||
				!slices.ContainsFunc(pod.Spec.Volumes, func(volume corev1.Volume) bool { return volume.Name == domain && volume.PersistentVolumeClaim != nil }) {
				return nil, fmt.Errorf("初始化资产域 %s 必须可写并使用独立 PVC", domain)
			}
		}
		pod.Spec.Volumes = slices.DeleteFunc(pod.Spec.Volumes, func(volume corev1.Volume) bool {
			return !slices.ContainsFunc(container.VolumeMounts, func(mount corev1.VolumeMount) bool { return mount.Name == volume.Name })
		})
		pod.Name = boundedSandboxName("assets-"+runtime.InstanceCode, 63)
		if slices.ContainsFunc(pods, func(existing *corev1.Pod) bool { return existing.Name == pod.Name }) {
			return nil, fmt.Errorf("初始化资产 Pod %s 与运行时拓扑重名", pod.Name)
		}
		pod.Labels = sandboxPodRoleLabels(plan.Sandbox, pod.Name)
		pod.Spec.InitContainers = nil
		applySandboxVolumeAffinity(plan.Sandbox, pod)
		jobs = append(jobs, runtimeInitPod{pod: pod, runtime: runtime, archives: selected})
	}
	count := 0
	for _, job := range jobs {
		count += len(job.archives)
	}
	if count != len(archives) {
		return nil, apperr.ErrSandboxInitAssetConfigInvalid
	}
	return jobs, nil
}

func (o *K8sOrchestrator) initializeRuntimeAssets(ctx context.Context, cs kubernetes.Interface, jobs []runtimeInitPod, exec func(context.Context, string, string, []string, []byte, bool) ([]byte, []byte, error)) error {
	for _, job := range jobs {
		if err := o.writeRuntimeInitArchives(ctx, cs, job, exec); err != nil {
			return err
		}
	}
	return nil
}

func (o *K8sOrchestrator) writeRuntimeInitArchives(ctx context.Context, cs kubernetes.Interface, job runtimeInitPod, exec func(context.Context, string, string, []string, []byte, bool) ([]byte, []byte, error)) (failure error) {
	pod, err := createOrVerifyPod(ctx, cs, job.pod)
	if err != nil {
		return err
	}
	if pod.UID == "" {
		return fmt.Errorf("初始化资产 Pod %s 缺少 UID", pod.Name)
	}
	defer func() {
		cleanupCtx, cancel := asyncPersistenceContext(ctx, timeDurationSeconds(o.cfg.ReadyTimeoutSeconds))
		defer cancel()
		if err := deleteObservedPods(cleanupCtx, cs, []corev1.Pod{*pod}, o.cfg.ReadyPollIntervalSeconds); err != nil {
			failure = errors.Join(failure, fmt.Errorf("回收初始化资产 Pod %s 失败: %w", pod.Name, err))
		}
	}()
	if err := o.waitPodsReady(ctx, cs, []*corev1.Pod{pod}); err != nil {
		return err
	}
	adapter := job.runtime.Runtime.AdapterSpec
	for _, archive := range job.archives {
		domain, _ := volumeDomainByName(adapter, archive.TargetDomain)
		command := workspaceCommand(adapter.InitAssetOps.UnpackTar, adapter.WorkspaceDir, domain.MountPath)
		if _, stderr, err := exec(ctx, pod.Namespace, pod.Name+"/"+job.pod.Spec.Containers[0].Name, command, archive.Tarball, false); err != nil {
			return sandboxExecFailure(apperr.ErrSandboxInitExecFailed, err, stderr)
		}
	}
	return nil
}

// ResetSandboxRuntime 先停止数据消费者，再用空账本和已校验材料恢复目标链。
func (o *K8sOrchestrator) ResetSandboxRuntime(ctx context.Context, plan CreateSandboxPlan, instance string, archives []RuntimeInitArchive) error {
	instance = strings.TrimSpace(instance)
	computePods := runtimeComputePodNames(plan, instance)
	if len(computePods) == 0 || o.cfg.ReadyTimeoutSeconds <= 0 || o.cfg.ReadyPollIntervalSeconds <= 0 {
		return fmt.Errorf("运行时重置目标或等待配置无效")
	}
	ctx, cancel := context.WithTimeout(ctx, timeDurationSeconds(o.cfg.ReadyTimeoutSeconds))
	defer cancel()
	pods, err := o.podsForPlan(plan)
	if err != nil {
		return err
	}
	for _, archive := range archives {
		if archive.RuntimeInstance != instance {
			return apperr.ErrSandboxInitAssetConfigInvalid
		}
	}
	jobs, err := initAssetPodsForPlan(plan, pods, archives)
	if err != nil {
		return err
	}
	ledger := volumeClaimName("runtime-"+instance, VolumeDomainRuntimeState)
	writes := map[string]bool{ledger: true}
	for _, archive := range archives {
		writes[volumeClaimName("runtime-"+instance, archive.TargetDomain)] = true
	}
	affected := map[string]*corev1.Pod{}
	var restart []*corev1.Pod
	for _, pod := range pods {
		if slices.Contains(computePods, pod.Name) || podUsesClaims(pod, writes) {
			affected[pod.Name] = pod
			restart = append(restart, pod)
		}
	}
	cs := o.client.Clientset()
	namespace, err := sandboxNamespace(ctx, cs, plan.Sandbox)
	if err != nil {
		return err
	}
	if namespace == nil {
		return fmt.Errorf("待重置沙箱 Namespace 不存在")
	}
	currentPods, err := cs.CoreV1().Pods(plan.Sandbox.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("读取账本消费者失败: %w", err)
	}
	var stopped []corev1.Pod
	for _, current := range currentPods.Items {
		desired := affected[current.Name]
		if desired == nil {
			if podUsesClaims(&current, writes) {
				return fmt.Errorf("账本被未声明的 Pod %s 使用", current.Name)
			}
			continue
		}
		if err := platformk8s.VerifyManagedLabels(&current, desired); err != nil {
			return err
		}
		stopped = append(stopped, current)
	}
	var resetLedger bool
	var oldLedger *corev1.PersistentVolumeClaim
	for _, domain := range persistentVolumeDomains(plan) {
		if !writes[domain.claimName()] {
			continue
		}
		current, err := cs.CoreV1().PersistentVolumeClaims(plan.Sandbox.Namespace).Get(ctx, domain.claimName(), metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if err == nil {
			desired := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: domain.claimName(), Labels: map[string]string{
				"app": "chaimir", "module": "sandbox", "chaimir.io/sandbox-id": ids.Format(plan.Sandbox.ID), "chaimir.io/volume": domain.claimName(),
			}}}
			if err := platformk8s.VerifyManagedLabels(current, desired); err != nil {
				return err
			}
			if domain.claimName() == ledger {
				oldLedger = current
			}
		}
		resetLedger = resetLedger || domain.claimName() == ledger
	}
	if resetLedger {
		if _, err := resource.ParseQuantity(o.cfg.WorkspaceStorage); err != nil {
			return fmt.Errorf("账本卷容量配置无效: %w", err)
		}
		if oldLedger != nil && oldLedger.UID == "" {
			return fmt.Errorf("账本卷 %s 缺少 UID，拒绝重置", ledger)
		}
	}
	if err := deleteObservedPods(ctx, cs, stopped, o.cfg.ReadyPollIntervalSeconds); err != nil {
		return err
	}
	if resetLedger {
		client := cs.CoreV1().PersistentVolumeClaims(plan.Sandbox.Namespace)
		if oldLedger != nil {
			if err := client.Delete(ctx, ledger, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &oldLedger.UID}}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			if err := wait.PollUntilContextCancel(ctx, sandboxPodReadyPollInterval(o.cfg.ReadyPollIntervalSeconds), true, func(ctx context.Context) (bool, error) {
				current, err := client.Get(ctx, ledger, metav1.GetOptions{})
				if apierrors.IsNotFound(err) {
					return true, nil
				}
				if err == nil && current.UID != oldLedger.UID {
					return false, fmt.Errorf("账本卷 %s 的 UID 已改变，拒绝继续重置", ledger)
				}
				return false, err
			}); err != nil {
				return fmt.Errorf("等待旧账本卷释放失败: %w", err)
			}
		}
		if err := createOrVerifyPVC(ctx, cs, o.volumeDomainPVC(plan, ledger, nil), true); err != nil {
			return err
		}
	}
	if err := o.initializeRuntimeAssets(ctx, cs, jobs, o.Exec); err != nil {
		return err
	}
	created, err := createPodsInStartupOrder(plan, restart, func(pod *corev1.Pod) (*corev1.Pod, error) {
		return createOrVerifyPod(ctx, cs, pod)
	}, func(pods []*corev1.Pod) error {
		return o.waitPodsReady(ctx, cs, pods)
	})
	if err != nil {
		return err
	}
	return o.waitPodsReady(ctx, cs, created)
}

func podUsesClaims(pod *corev1.Pod, claims map[string]bool) bool {
	return slices.ContainsFunc(pod.Spec.Volumes, func(volume corev1.Volume) bool {
		return volume.PersistentVolumeClaim != nil && claims[volume.PersistentVolumeClaim.ClaimName]
	})
}
