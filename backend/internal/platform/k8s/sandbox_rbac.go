// sandbox_rbac 文件定义动态运行命名空间的最小控制面 RBAC 契约与预配逻辑。
package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"chaimir/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// SandboxBackendClusterRoleName 是部署层维护的固定沙箱工作负载 ClusterRole 名称。
	SandboxBackendClusterRoleName = "chaimir-backend-sandbox-workload"
	// SandboxBackendRoleBindingName 是动态命名空间内固定的控制面 RoleBinding 名称。
	SandboxBackendRoleBindingName = "chaimir-backend-sandbox-workload"
	// SandboxBackendServiceAccountNamespace 是控制面 ServiceAccount 的部署命名空间。
	SandboxBackendServiceAccountNamespace = "chaimir-system"
	// SandboxBackendServiceAccountName 是控制面 ServiceAccount 的固定名称。
	SandboxBackendServiceAccountName = "chaimir-backend"
	// sandboxManagedByLabel 只允许平台创建的命名空间进入预配队列。
	sandboxManagedByLabel = "chaimir-backend"
)

// SandboxRBACProvisioner 监听平台拥有的动态命名空间并维护其命名空间级 RoleBinding。
type SandboxRBACProvisioner struct {
	client                    kubernetes.Interface
	backendServiceAccountNS   string
	backendServiceAccountName string
	resyncPeriod              time.Duration
}

// NewSandboxRBACProvisioner 构造隔离命名空间 RBAC 控制器。
func NewSandboxRBACProvisioner(client kubernetes.Interface, serviceAccountNamespace, serviceAccountName string, resyncPeriod time.Duration) (*SandboxRBACProvisioner, error) {
	if client == nil {
		return nil, fmt.Errorf("RBAC 预配器缺少 Kubernetes 客户端")
	}
	serviceAccountNamespace = strings.TrimSpace(serviceAccountNamespace)
	serviceAccountName = strings.TrimSpace(serviceAccountName)
	if serviceAccountNamespace == "" || serviceAccountName == "" || resyncPeriod <= 0 {
		return nil, fmt.Errorf("RBAC 预配器配置非法")
	}
	return &SandboxRBACProvisioner{client: client, backendServiceAccountNS: serviceAccountNamespace, backendServiceAccountName: serviceAccountName, resyncPeriod: resyncPeriod}, nil
}

// Run 启动基于 informer 的声明式修复循环,直到上下文取消。
func (p *SandboxRBACProvisioner) Run(ctx context.Context) error {
	listWatch := cache.NewFilteredListWatchFromClient(p.client.CoreV1().RESTClient(), "namespaces", "", func(options *metav1.ListOptions) {
		options.LabelSelector = labels.Set{"app.kubernetes.io/part-of": "chaimir", "chaimir.io/managed-by": sandboxManagedByLabel}.AsSelector().String()
	})
	informer := cache.NewSharedInformer(listWatch, &corev1.Namespace{}, p.resyncPeriod)
	queue := workqueue.NewTypedRateLimitingQueue(workqueue.NewTypedItemExponentialFailureRateLimiter[string](time.Second, time.Minute))
	defer queue.ShutDown()
	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { p.enqueueNamespace(queue, obj) },
		UpdateFunc: func(_, obj interface{}) { p.enqueueNamespace(queue, obj) },
	}); err != nil {
		return fmt.Errorf("注册动态命名空间事件处理器失败: %w", err)
	}
	go informer.Run(ctx.Done())
	go func() {
		<-ctx.Done()
		queue.ShutDown()
	}()
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("等待动态命名空间 informer 同步失败")
	}
	for p.processNext(ctx, queue) {
	}
	return nil
}

// enqueueNamespace 只把符合条件的命名空间名称加入修复队列。
func (p *SandboxRBACProvisioner) enqueueNamespace(queue workqueue.TypedInterface[string], obj interface{}) {
	ns, ok := obj.(*corev1.Namespace)
	if !ok || !isManagedSandboxNamespace(ns) {
		return
	}
	queue.Add(ns.Name)
}

// processNext 修复一个命名空间,并对临时错误执行限速重试。
func (p *SandboxRBACProvisioner) processNext(ctx context.Context, queue workqueue.TypedRateLimitingInterface[string]) bool {
	item, shutdown := queue.Get()
	if shutdown {
		return false
	}
	defer queue.Done(item)
	ns, err := p.client.CoreV1().Namespaces().Get(ctx, item, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		queue.Forget(item)
		return true
	}
	if err != nil {
		slog.Error("sandbox RBAC namespace reconcile failed", slog.String("namespace", item), slog.String("error", logging.SanitizeError(err.Error())))
		queue.AddRateLimited(item)
		return true
	}
	if !isManagedSandboxNamespace(ns) {
		queue.Forget(item)
		return true
	}
	if err := p.reconcileNamespace(ctx, ns.Name); err != nil {
		slog.Error("sandbox RBAC namespace reconcile failed", slog.String("namespace", item), slog.String("error", logging.SanitizeError(err.Error())))
		queue.AddRateLimited(item)
		return true
	}
	queue.Forget(item)
	return true
}

// reconcileNamespace 在单个命名空间内创建或修复指向部署层固定 ClusterRole 的 RoleBinding。
func (p *SandboxRBACProvisioner) reconcileNamespace(ctx context.Context, namespace string) error {
	bindings := p.client.RbacV1().RoleBindings(namespace)
	desiredBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: SandboxBackendRoleBindingName, Namespace: namespace, Labels: sandboxResourceLabels(namespace)},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: SandboxBackendClusterRoleName},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: p.backendServiceAccountNS, Name: p.backendServiceAccountName}},
	}
	existingBinding, err := bindings.Get(ctx, SandboxBackendRoleBindingName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, err := bindings.Create(ctx, desiredBinding, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("创建动态命名空间 RoleBinding 失败: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("查询动态命名空间 RoleBinding 失败: %w", err)
	} else if existingBinding.RoleRef != desiredBinding.RoleRef || !reflect.DeepEqual(existingBinding.Subjects, desiredBinding.Subjects) || !reflect.DeepEqual(existingBinding.Labels, desiredBinding.Labels) {
		updated := existingBinding.DeepCopy()
		updated.Labels = desiredBinding.Labels
		updated.RoleRef = desiredBinding.RoleRef
		updated.Subjects = desiredBinding.Subjects
		if _, err := bindings.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("修复动态命名空间 RoleBinding 失败: %w", err)
		}
	}
	return nil
}

// WaitForSandboxBackendRBAC 在创建沙箱资源前等待固定命名空间绑定就绪。
func WaitForSandboxBackendRBAC(ctx context.Context, client kubernetes.Interface, namespace, serviceAccountNamespace, serviceAccountName string, timeout, poll time.Duration) error {
	if client == nil || strings.TrimSpace(namespace) == "" || timeout <= 0 || poll <= 0 {
		return fmt.Errorf("等待动态命名空间 RBAC 的参数非法")
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		binding, err := client.RbacV1().RoleBindings(namespace).Get(waitCtx, SandboxBackendRoleBindingName, metav1.GetOptions{})
		if err == nil && roleBindingMatches(binding, serviceAccountNamespace, serviceAccountName) {
			return nil
		}
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("查询动态命名空间 RoleBinding 失败: %w", err)
		}
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("等待动态命名空间最小 RBAC 就绪超时: %w", waitCtx.Err())
		case <-ticker.C:
		}
	}
}

// isManagedSandboxNamespace 在授权前同时校验命名空间名称和所有权标签。
func isManagedSandboxNamespace(ns *corev1.Namespace) bool {
	if ns == nil || ns.DeletionTimestamp != nil || ns.Labels["app.kubernetes.io/part-of"] != "chaimir" || ns.Labels["chaimir.io/managed-by"] != sandboxManagedByLabel {
		return false
	}
	name := ns.Name
	if strings.HasPrefix(name, "sim-") {
		return ns.Labels["chaimir.io/sim"] == "true"
	}
	return (strings.HasPrefix(name, "sbx-") || strings.HasPrefix(name, "judge-") || strings.HasPrefix(name, "battle-")) && ns.Labels["chaimir.io/sandbox"] == "true"
}

// sandboxResourceLabels 标记控制器管理的 RBAC 对象,不复制用户可控的命名空间元数据。
func sandboxResourceLabels(namespace string) map[string]string {
	return map[string]string{"app.kubernetes.io/part-of": "chaimir", "chaimir.io/managed-by": sandboxManagedByLabel, "chaimir.io/namespace": namespace}
}

// roleBindingMatches 校验绑定只指向固定 ClusterRole 和指定后端 ServiceAccount。
func roleBindingMatches(binding *rbacv1.RoleBinding, namespace, name string) bool {
	if binding == nil || binding.RoleRef.Kind != "ClusterRole" || binding.RoleRef.Name != SandboxBackendClusterRoleName || binding.RoleRef.APIGroup != rbacv1.GroupName || len(binding.Subjects) != 1 {
		return false
	}
	subject := binding.Subjects[0]
	return subject.Kind == "ServiceAccount" && subject.Namespace == namespace && subject.Name == name
}
