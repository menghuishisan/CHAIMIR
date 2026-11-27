package sandbox

import (
	"context"
	"fmt"
	"time"

	platformk8s "chaimir/internal/platform/k8s"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

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
	if o.cfg.ReadyTimeoutSeconds <= 0 || o.cfg.ReadyPollIntervalSeconds <= 0 {
		return fmt.Errorf("停止沙箱计算的等待配置无效")
	}
	ctx, cancel := context.WithTimeout(ctx, timeDurationSeconds(o.cfg.ReadyTimeoutSeconds))
	defer cancel()
	cs := o.client.Clientset()
	namespace, err := sandboxNamespace(ctx, cs, sb)
	if err != nil || namespace == nil {
		return err
	}
	client := cs.CoreV1().Pods(sb.Namespace)
	pods, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("列出沙箱计算 Pod 失败: %w", err)
	}
	for index := range pods.Items {
		pod := &pods.Items[index]
		desired := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Labels: sandboxManagedLabels(sb)}}
		if err := platformk8s.VerifyManagedLabels(pod, desired); err != nil {
			return err
		}
	}
	if err := deleteObservedPods(ctx, cs, pods.Items, o.cfg.ReadyPollIntervalSeconds); err != nil {
		return err
	}
	remaining, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("复查沙箱计算 Pod 失败: %w", err)
	}
	if len(remaining.Items) != 0 {
		return fmt.Errorf("沙箱计算停止期间出现新的 Pod，拒绝继续归档或快照")
	}
	return nil
}

// cleanupSnapshotRetainedNamespace 回收服务入口和临时凭据，保留存储与隔离限制。
func (o *K8sOrchestrator) cleanupSnapshotRetainedNamespace(ctx context.Context, cs kubernetes.Interface, sb Sandbox) error {
	namespace, err := sandboxNamespace(ctx, cs, sb)
	if err != nil || namespace == nil {
		return err
	}
	servicesClient := cs.CoreV1().Services(sb.Namespace)
	services, err := servicesClient.List(ctx, metav1.ListOptions{LabelSelector: labels.Set(sandboxManagedLabels(sb)).AsSelector().String()})
	if err != nil {
		return fmt.Errorf("列出待回收沙箱 Service 失败: %w", err)
	}
	secretsClient := cs.CoreV1().Secrets(sb.Namespace)
	secrets, err := secretsClient.List(ctx, metav1.ListOptions{LabelSelector: "app=chaimir,module=sandbox,chaimir.io/managed-by=chaimir-backend,chaimir.io/purpose in (image-pull,sandbox-runtime)"})
	if err != nil {
		return fmt.Errorf("列出待回收沙箱 Secret 失败: %w", err)
	}
	type deletion struct {
		object  metav1.Object
		remove  func(context.Context, string, metav1.DeleteOptions) error
		options metav1.DeleteOptions
	}
	var targets []deletion
	for index := range services.Items {
		targets = append(targets, deletion{object: &services.Items[index], remove: servicesClient.Delete})
	}
	for index := range secrets.Items {
		targets = append(targets, deletion{object: &secrets.Items[index], remove: secretsClient.Delete})
	}
	for index := range targets {
		targets[index].options, err = resourceDeleteOptions(targets[index].object)
		if err != nil {
			return err
		}
	}
	for _, target := range targets {
		if err := target.remove(ctx, target.object.GetName(), target.options); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("回收沙箱临时资源 %s 失败: %w", target.object.GetName(), err)
		}
	}
	return nil
}

func deleteObservedPods(ctx context.Context, cs kubernetes.Interface, pods []corev1.Pod, pollSeconds int) error {
	if pollSeconds <= 0 {
		return fmt.Errorf("SANDBOX_READY_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	for _, pod := range pods {
		if pod.UID == "" {
			return fmt.Errorf("沙箱 Pod %s 缺少 UID，拒绝删除", pod.Name)
		}
	}
	for _, pod := range pods {
		options := metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &pod.UID}}
		if err := cs.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, options); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("删除沙箱 Pod %s 失败: %w", pod.Name, err)
		}
	}
	for index := range pods {
		if err := waitPodDeleted(ctx, cs, &pods[index], pollSeconds); err != nil {
			return err
		}
	}
	return nil
}

// waitPodDeleted 只等待观察到的 Pod，同名替换须交回调用者处理。
func waitPodDeleted(ctx context.Context, cs kubernetes.Interface, pod *corev1.Pod, pollSeconds int) error {
	if pollSeconds <= 0 {
		return fmt.Errorf("SANDBOX_READY_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	ticker := time.NewTicker(sandboxPodReadyPollInterval(pollSeconds))
	defer ticker.Stop()
	for {
		current, err := cs.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("查询待删除的 Pod %s 失败: %w", pod.Name, err)
		}
		if current.UID != pod.UID {
			return fmt.Errorf("沙箱 Pod %s 的 UID 已改变，拒绝继续处理同名对象", pod.Name)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待 Pod %s 删除超时: %w", pod.Name, ctx.Err())
		case <-ticker.C:
		}
	}
}
