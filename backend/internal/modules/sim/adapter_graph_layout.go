// sim adapter_graph_layout 文件实现 graph-layout-stdio 的 Kubernetes 隔离计算会话。
package sim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"chaimir/internal/platform/config"
	platformk8s "chaimir/internal/platform/k8s"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	graphLayoutAdapterCode    = "graph-layout-stdio"
	graphLayoutContainer      = "graph-layout"
	graphLayoutPod            = "compute"
	graphLayoutServiceAccount = "sim-compute"
)

// GraphLayoutAdapter 使用受控镜像为每个后端仿真会话提供确定性图布局计算。
type GraphLayoutAdapter struct {
	k8s      *platformk8s.Client
	cfg      config.SimBackendConfig
	sandbox  config.SandboxConfig
	active   sync.Map
	requests corev1.ResourceList
	limits   corev1.ResourceList
}

// NewGraphLayoutAdapter 校验资源配置并构造生产 graph-layout-stdio 适配器。
func NewGraphLayoutAdapter(k8sClient *platformk8s.Client, cfg config.SimBackendConfig, sandbox config.SandboxConfig) (*GraphLayoutAdapter, error) {
	if k8sClient == nil {
		return nil, fmt.Errorf("graph layout adapter 缺少 Kubernetes 客户端")
	}
	requests, limits, err := graphLayoutResources(cfg)
	if err != nil {
		return nil, err
	}
	return &GraphLayoutAdapter{k8s: k8sClient, cfg: cfg, sandbox: sandbox, requests: requests, limits: limits}, nil
}

// Descriptor 返回教师端可以选择的受控计算能力。
func (a *GraphLayoutAdapter) Descriptor() BackendAdapterDescriptor {
	return BackendAdapterDescriptor{
		Code:        graphLayoutAdapterCode,
		Name:        "图布局计算",
		Protocol:    "stdio-json",
		Description: "适用于需要在服务端重新计算大量节点位置的仿真。",
	}
}

// ValidateConfig 拒绝自由配置,镜像与资源边界只能由部署环境控制。
func (a *GraphLayoutAdapter) ValidateConfig(value map[string]any) error {
	if len(value) != 0 {
		return fmt.Errorf("graph-layout-stdio 不接受自定义后端配置")
	}
	return nil
}

// Serve 创建隔离计算资源,先推送初始布局,再逐条处理已通过 M4 schema 校验的事件。
func (a *GraphLayoutAdapter) Serve(ctx context.Context, session SessionWithPackage, conn BackendConn) error {
	if _, loaded := a.active.LoadOrStore(session.ID, struct{}{}); loaded {
		return fmt.Errorf("仿真会话已有后端计算连接")
	}
	defer a.active.Delete(session.ID)
	defer func() { _ = a.Release(context.Background(), session) }()

	if err := a.prepareSession(ctx, session); err != nil {
		return err
	}
	initial, err := a.runLayout(ctx, session, session.InitParams)
	if err != nil {
		return fmt.Errorf("计算初始图布局失败: %w", err)
	}
	if err := conn.SendJSON(BackendState{Tick: 0, State: initial}); err != nil {
		return fmt.Errorf("发送初始图布局失败: %w", err)
	}

	var tick int64
	for {
		var event BackendEvent
		if err := conn.ReadJSON(&event); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("读取图布局事件失败: %w", err)
		}
		state, err := a.runLayout(ctx, session, event.Payload)
		if err != nil {
			return fmt.Errorf("执行图布局事件失败: %w", err)
		}
		tick++
		if err := conn.SendJSON(BackendState{Tick: tick, State: state}); err != nil {
			return fmt.Errorf("发送图布局状态失败: %w", err)
		}
	}
}

// Release 删除会话独占命名空间,调用可重复执行。
func (a *GraphLayoutAdapter) Release(_ context.Context, session SessionWithPackage) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.cfg.PodReadyTimeoutSeconds)*time.Second)
	defer cancel()
	policy := metav1.DeletePropagationBackground
	err := a.k8s.Clientset().CoreV1().Namespaces().Delete(ctx, a.namespace(session.ID), metav1.DeleteOptions{PropagationPolicy: &policy})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除图布局会话资源失败: %w", err)
	}
	return nil
}

// prepareSession 清理残留资源后创建独立命名空间、拒绝全部网络的策略和计算 Pod。
func (a *GraphLayoutAdapter) prepareSession(ctx context.Context, session SessionWithPackage) error {
	namespace := a.namespace(session.ID)
	if err := a.deleteNamespaceAndWait(ctx, namespace); err != nil {
		return err
	}
	podLabels := map[string]string{
		"app.kubernetes.io/name":      "sim-backend",
		"app.kubernetes.io/component": "graph-layout",
		"chaimir.io/session-id":       strconv.FormatInt(session.ID, 10),
	}
	namespaceLabels := map[string]string{
		"app.kubernetes.io/name":             "sim-backend",
		"app.kubernetes.io/component":        "graph-layout",
		"app.kubernetes.io/part-of":          "chaimir",
		"chaimir.io/session-id":              strconv.FormatInt(session.ID, 10),
		"chaimir.io/sim":                     "true",
		"chaimir.io/managed-by":              "chaimir-backend",
		"pod-security.kubernetes.io/enforce": "restricted",
		"pod-security.kubernetes.io/audit":   "restricted",
		"pod-security.kubernetes.io/warn":    "restricted",
	}
	if _, err := a.k8s.Clientset().CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: namespaceLabels}}, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建图布局会话命名空间失败: %w", err)
	}
	if err := a.k8s.SyncImagePullSecrets(ctx, a.sandbox.ControlNamespace, namespace, "sim", a.sandbox.ImagePullSecretNames); err != nil {
		return err
	}
	automount := false
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta:                   metav1.ObjectMeta{Name: graphLayoutServiceAccount, Namespace: namespace, Labels: podLabels},
		AutomountServiceAccountToken: &automount,
	}
	if _, err := a.k8s.Clientset().CoreV1().ServiceAccounts(namespace).Create(ctx, serviceAccount, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建图布局计算 ServiceAccount 失败: %w", err)
	}
	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-all", Namespace: namespace},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		},
	}
	if _, err := a.k8s.Clientset().NetworkingV1().NetworkPolicies(namespace).Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建图布局网络隔离策略失败: %w", err)
	}
	pod := a.sessionPod(namespace, podLabels)
	if _, err := a.k8s.Clientset().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建图布局计算 Pod 失败: %w", err)
	}
	readyCtx, cancel := context.WithTimeout(ctx, time.Duration(a.cfg.PodReadyTimeoutSeconds)*time.Second)
	defer cancel()
	if err := wait.PollUntilContextCancel(readyCtx, 250*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		current, err := a.k8s.Clientset().CoreV1().Pods(namespace).Get(ctx, graphLayoutPod, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if current.Status.Phase == corev1.PodFailed || current.Status.Phase == corev1.PodSucceeded {
			return false, fmt.Errorf("图布局计算 Pod 提前结束: phase=%s", current.Status.Phase)
		}
		for _, condition := range current.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("等待图布局计算 Pod 就绪失败: %w", err)
	}
	return nil
}

// deleteNamespaceAndWait 删除服务异常退出后遗留的同会话资源,避免复用未知状态。
func (a *GraphLayoutAdapter) deleteNamespaceAndWait(ctx context.Context, namespace string) error {
	policy := metav1.DeletePropagationBackground
	err := a.k8s.Clientset().CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{PropagationPolicy: &policy})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("清理残留图布局命名空间失败: %w", err)
	}
	if apierrors.IsNotFound(err) {
		return nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(a.cfg.PodReadyTimeoutSeconds)*time.Second)
	defer cancel()
	if err := wait.PollUntilContextCancel(waitCtx, 250*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		_, err := a.k8s.Clientset().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err == nil {
			return false, nil
		}
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}); err != nil {
		return fmt.Errorf("等待残留图布局命名空间删除失败: %w", err)
	}
	return nil
}

// sessionPod 构造无网络、非 root、只读根文件系统的计算 Pod。
func (a *GraphLayoutAdapter) sessionPod(namespace string, labels map[string]string) *corev1.Pod {
	zero := int64(0)
	nonRoot := true
	readOnly := true
	allowEscalation := false
	automount := false
	pullSecrets := make([]corev1.LocalObjectReference, 0, len(a.sandbox.ImagePullSecretNames))
	for _, name := range a.sandbox.ImagePullSecretNames {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: name})
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: graphLayoutPod, Namespace: namespace, Labels: labels},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken:  &automount,
			ServiceAccountName:            graphLayoutServiceAccount,
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: &zero,
			ImagePullSecrets:              pullSecrets,
			NodeSelector:                  a.sandbox.SandboxNodeSelector,
			Tolerations:                   graphLayoutTolerations(a.sandbox.SandboxNodeTolerations),
			SecurityContext:               &corev1.PodSecurityContext{RunAsNonRoot: &nonRoot, SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}},
			Containers: []corev1.Container{{
				Name:            graphLayoutContainer,
				Image:           a.cfg.GraphLayoutImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"/bin/sh", "-c", "trap : TERM INT; tail -f /dev/null & wait"},
				Resources:       corev1.ResourceRequirements{Requests: a.requests, Limits: a.limits},
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot:             &nonRoot,
					ReadOnlyRootFilesystem:   &readOnly,
					AllowPrivilegeEscalation: &allowEscalation,
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				},
			}},
		},
	}
}

// runLayout 校验输入规模,调用镜像入口并把输出约束为节点和边对象。
func (a *GraphLayoutAdapter) runLayout(ctx context.Context, session SessionWithPackage, graph map[string]any) (map[string]any, error) {
	if err := a.validateGraph(graph); err != nil {
		return nil, err
	}
	input, err := json.Marshal(graph)
	if err != nil {
		return nil, fmt.Errorf("编码图布局输入失败: %w", err)
	}
	if int64(len(input)) > a.cfg.MaxInputBytes {
		return nil, fmt.Errorf("图布局输入超过部署上限")
	}
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(a.cfg.ExecTimeoutSeconds)*time.Second)
	defer cancel()
	stdout := newLimitedBuffer(a.cfg.MaxOutputBytes)
	stderr := newLimitedBuffer(a.cfg.MaxOutputBytes)
	if err := a.k8s.Exec(execCtx, a.namespace(session.ID), graphLayoutPod, graphLayoutContainer, []string{"python", "/sim/graph_layout.py"}, bytes.NewReader(input), stdout, stderr, false); err != nil {
		return nil, fmt.Errorf("图布局镜像执行失败: %w: %s", err, stderr.String())
	}
	var output map[string]any
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	decoder.UseNumber()
	if err := decoder.Decode(&output); err != nil {
		return nil, fmt.Errorf("图布局镜像输出无效: %w", err)
	}
	if err := a.validateGraph(output); err != nil {
		return nil, fmt.Errorf("图布局镜像输出不符合协议: %w", err)
	}
	return output, nil
}

// validateGraph 只接受 nodes/edges 两个数组并执行部署规模上限。
func (a *GraphLayoutAdapter) validateGraph(graph map[string]any) error {
	if graph == nil {
		return fmt.Errorf("图布局数据不能为空")
	}
	for key := range graph {
		if key != "nodes" && key != "edges" {
			return fmt.Errorf("图布局数据包含未声明字段")
		}
	}
	nodes, ok := graph["nodes"].([]any)
	if !ok {
		return fmt.Errorf("图布局节点必须为数组")
	}
	edges, ok := graph["edges"].([]any)
	if !ok {
		return fmt.Errorf("图布局边必须为数组")
	}
	if len(nodes) > a.cfg.MaxGraphNodes || len(edges) > a.cfg.MaxGraphEdges {
		return fmt.Errorf("图布局规模超过部署上限")
	}
	for _, node := range nodes {
		if _, ok := node.(map[string]any); !ok {
			return fmt.Errorf("图布局节点必须为对象")
		}
	}
	for _, edge := range edges {
		if _, ok := edge.(map[string]any); !ok {
			return fmt.Errorf("图布局边必须为对象")
		}
	}
	return nil
}

// namespace 返回只由受控前缀和服务端会话编号构成的资源名。
func (a *GraphLayoutAdapter) namespace(sessionID int64) string {
	return a.cfg.NamespacePrefix + strconv.FormatInt(sessionID, 10)
}

// graphLayoutResources 把已在启动配置层校验过的 quantity 转为 Pod 资源对象。
func graphLayoutResources(cfg config.SimBackendConfig) (corev1.ResourceList, corev1.ResourceList, error) {
	parse := func(name, value string) (resource.Quantity, error) {
		quantity, err := resource.ParseQuantity(value)
		if err != nil {
			return resource.Quantity{}, fmt.Errorf("解析 %s 失败: %w", name, err)
		}
		return quantity, nil
	}
	cpuRequest, err := parse("SIM_BACKEND_CPU_REQUEST", cfg.CPURequest)
	if err != nil {
		return nil, nil, err
	}
	memoryRequest, err := parse("SIM_BACKEND_MEMORY_REQUEST", cfg.MemoryRequest)
	if err != nil {
		return nil, nil, err
	}
	cpuLimit, err := parse("SIM_BACKEND_CPU_LIMIT", cfg.CPULimit)
	if err != nil {
		return nil, nil, err
	}
	memoryLimit, err := parse("SIM_BACKEND_MEMORY_LIMIT", cfg.MemoryLimit)
	if err != nil {
		return nil, nil, err
	}
	storageLimit, err := parse("SIM_BACKEND_EPHEMERAL_STORAGE_LIMIT", cfg.EphemeralStorageLimit)
	if err != nil {
		return nil, nil, err
	}
	return corev1.ResourceList{corev1.ResourceCPU: cpuRequest, corev1.ResourceMemory: memoryRequest}, corev1.ResourceList{corev1.ResourceCPU: cpuLimit, corev1.ResourceMemory: memoryLimit, corev1.ResourceEphemeralStorage: storageLimit}, nil
}

// graphLayoutTolerations 转换共享调度配置,避免 M4 依赖 M2 内部转换函数。
func graphLayoutTolerations(items []config.SandboxToleration) []corev1.Toleration {
	out := make([]corev1.Toleration, 0, len(items))
	for _, item := range items {
		out = append(out, corev1.Toleration{Key: item.Key, Operator: corev1.TolerationOperator(item.Operator), Value: item.Value, Effect: corev1.TaintEffect(item.Effect), TolerationSeconds: item.TolerationSeconds})
	}
	return out
}

// limitedBuffer 限制 Kubernetes exec 的单路输出大小。
type limitedBuffer struct {
	buffer bytes.Buffer
	limit  int64
}

// newLimitedBuffer 构造显式上限的输出缓冲区。
func newLimitedBuffer(limit int64) *limitedBuffer { return &limitedBuffer{limit: limit} }

// Write 在超过上限时中止 exec 流,防止异常镜像耗尽后端内存。
func (b *limitedBuffer) Write(data []byte) (int, error) {
	remaining := b.limit - int64(b.buffer.Len())
	if remaining <= 0 {
		return 0, fmt.Errorf("计算输出超过部署上限")
	}
	if int64(len(data)) > remaining {
		_, _ = b.buffer.Write(data[:remaining])
		return int(remaining), fmt.Errorf("计算输出超过部署上限")
	}
	return b.buffer.Write(data)
}

// Bytes 返回已接收的受限输出。
func (b *limitedBuffer) Bytes() []byte { return b.buffer.Bytes() }

// String 返回已接收的受限输出文本。
func (b *limitedBuffer) String() string { return b.buffer.String() }
