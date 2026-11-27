package sandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	platformk8s "chaimir/internal/platform/k8s"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const runtimeSelftestNamespacePrefix = "sbx-selftest-"
const selftestCleanupAfterAnnotation = "chaimir.io/selftest-cleanup-after"

// DestroySandboxResources 只删除归属当前沙箱且版本未变化的 Namespace，并等待原对象消失。
func (o *K8sOrchestrator) DestroySandboxResources(ctx context.Context, sb Sandbox) error {
	cs := o.client.Clientset()
	namespace, err := sandboxNamespace(ctx, cs, sb)
	if err != nil || namespace == nil {
		return err
	}
	options, err := resourceDeleteOptions(namespace)
	if err != nil {
		return err
	}
	if err := cs.CoreV1().Namespaces().Delete(ctx, sb.Namespace, options); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除沙箱 Namespace 失败: %w", err)
	}
	return waitNamespaceDeleted(ctx, cs, namespace, o.cfg.ReadyPollIntervalSeconds)
}

func sandboxNamespace(ctx context.Context, cs kubernetes.Interface, sb Sandbox) (*corev1.Namespace, error) {
	namespace, err := cs.CoreV1().Namespaces().Get(ctx, sb.Namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取沙箱 Namespace 失败: %w", err)
	}
	desired := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: sb.Namespace, Labels: sandboxManagedLabels(sb)}}
	if err := platformk8s.VerifyManagedLabels(namespace, desired); err != nil {
		return nil, err
	}
	return namespace, nil
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
		options, err := resourceDeleteOptions(namespace)
		if err != nil {
			cleanupErrs = append(cleanupErrs, err)
			continue
		}
		if err := cs.CoreV1().Namespaces().Delete(ctx, namespace.Name, options); err != nil && !apierrors.IsNotFound(err) {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("删除过期运行时自检 Namespace(%s)失败: %w", namespace.Name, err))
			continue
		}
		if err := waitNamespaceDeleted(ctx, cs, namespace, pollSeconds); err != nil {
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

// UID 防止同名替换，resourceVersion 防止归属或自检期限在读取后变更。
func resourceDeleteOptions(object metav1.Object) (metav1.DeleteOptions, error) {
	uid, version := object.GetUID(), object.GetResourceVersion()
	if uid == "" || version == "" {
		return metav1.DeleteOptions{}, fmt.Errorf("对象 %s 缺少 UID 或 resourceVersion，拒绝回收", object.GetName())
	}
	policy := metav1.DeletePropagationBackground
	return metav1.DeleteOptions{
		PropagationPolicy: &policy,
		Preconditions: &metav1.Preconditions{
			UID:             &uid,
			ResourceVersion: &version,
		},
	}, nil
}

// waitNamespaceDeleted 等待动态沙箱 Namespace 真正消失,避免数据库状态先行标记 destroyed 后存储仍残留。
func waitNamespaceDeleted(ctx context.Context, cs kubernetes.Interface, namespace *corev1.Namespace, pollSeconds int) error {
	if pollSeconds <= 0 {
		return fmt.Errorf("SANDBOX_READY_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	ticker := time.NewTicker(time.Duration(pollSeconds) * time.Second)
	defer ticker.Stop()
	for {
		current, err := cs.CoreV1().Namespaces().Get(ctx, namespace.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return fmt.Errorf("查询沙箱 Namespace 删除状态失败: %w", err)
		}
		if current.UID != namespace.UID {
			return fmt.Errorf("沙箱 Namespace %s 的 UID 已改变，拒绝继续处理同名对象", namespace.Name)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待沙箱 Namespace 删除超时: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// namespaceObject 构造带平台所有权标签的动态沙箱 Namespace。
func namespaceObject(name string, sb Sandbox) *corev1.Namespace {
	labels := sandboxManagedLabels(sb)
	labels["policy.sigstore.dev/include"] = "true"
	labels["pod-security.kubernetes.io/enforce"] = "restricted"
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   name,
		Labels: labels,
	}}
}

// createOrVerifyNamespace 创建或核对沙箱 Namespace,拒绝复用其他沙箱或旧安全标签的对象。
func createOrVerifyNamespace(ctx context.Context, cs kubernetes.Interface, desired *corev1.Namespace) error {
	client := cs.CoreV1().Namespaces()
	existing, err := client.Create(ctx, desired, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		existing, err = client.Get(ctx, desired.Name, metav1.GetOptions{})
	}
	if err != nil {
		return fmt.Errorf("准备 Namespace 失败: %w", err)
	}
	if err := platformk8s.VerifyManagedResource(existing, desired); err != nil {
		return err
	}
	for key, value := range desired.Annotations {
		if existing.Annotations[key] != value {
			return fmt.Errorf("沙箱 Namespace %s 的注解 %s 与请求不一致", desired.Name, key)
		}
	}
	return nil
}
