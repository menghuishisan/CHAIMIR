// sim adapter_stdio_json 文件实现数据驱动的 stdio-json Kubernetes 隔离计算适配器。
package sim

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"sync"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/jsonx"
	platformk8s "chaimir/internal/platform/k8s"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	stdioJSONContainer      = "compute"
	stdioJSONPod            = "compute"
	stdioJSONServiceAccount = "sim-compute"
)

// StdioJSONAdapter 使用一项受控能力配置执行任意遵循 stdio-json 协议的算法镜像。
type StdioJSONAdapter struct {
	k8s      *platformk8s.Client
	cfg      config.SimBackendConfig
	profile  config.SimBackendAdapterConfig
	sandbox  config.SandboxConfig
	active   sync.Map
	requests corev1.ResourceList
	limits   corev1.ResourceList
}

// NewStdioJSONBackendRegistry 从部署能力目录构造注册表,同协议算法共享一套编排代码。
func NewStdioJSONBackendRegistry(k8sClient *platformk8s.Client, cfg config.SimBackendConfig, sandbox config.SandboxConfig) (BackendRegistry, error) {
	if k8sClient == nil {
		return nil, fmt.Errorf("stdio-json adapter 缺少 Kubernetes 客户端")
	}
	registry := make(BackendRegistry, len(cfg.StdioAdapters))
	for _, profile := range cfg.StdioAdapters {
		adapter, err := newStdioJSONAdapter(k8sClient, cfg, profile, sandbox)
		if err != nil {
			return nil, fmt.Errorf("构造后端计算能力 %q 失败: %w", profile.Code, err)
		}
		if _, exists := registry[profile.Code]; exists {
			return nil, fmt.Errorf("后端计算能力编号重复: %s", profile.Code)
		}
		registry[profile.Code] = adapter
	}
	if len(registry) == 0 {
		return nil, fmt.Errorf("stdio-json adapter 能力目录不能为空")
	}
	return registry, nil
}

// newStdioJSONAdapter 把已在配置边界校验的资源值转换为 Kubernetes 对象。
func newStdioJSONAdapter(k8sClient *platformk8s.Client, cfg config.SimBackendConfig, profile config.SimBackendAdapterConfig, sandbox config.SandboxConfig) (*StdioJSONAdapter, error) {
	requests, limits, err := stdioJSONResources(profile)
	if err != nil {
		return nil, err
	}
	return &StdioJSONAdapter{k8s: k8sClient, cfg: cfg, profile: profile, sandbox: sandbox, requests: requests, limits: limits}, nil
}

// Descriptor 返回教师端可以安全选择的计算能力,不暴露镜像和集群配置。
func (a *StdioJSONAdapter) Descriptor() BackendAdapterDescriptor {
	return BackendAdapterDescriptor{Code: a.profile.Code, Name: a.profile.Name, Protocol: a.profile.Protocol, Description: a.profile.Description}
}

// ValidateConfig 拒绝包级自由配置,执行边界只能来自部署能力目录。
func (a *StdioJSONAdapter) ValidateConfig(value map[string]any) error {
	if len(value) != 0 {
		return fmt.Errorf("stdio-json 后端计算能力不接受自定义配置")
	}
	return nil
}

// Serve 创建隔离计算资源,先推送初始状态,再逐条执行已通过 M4 schema 校验的事件。
func (a *StdioJSONAdapter) Serve(ctx context.Context, session SessionWithPackage, conn BackendConn) (serveErr error) {
	if _, loaded := a.active.LoadOrStore(session.ID, struct{}{}); loaded {
		return fmt.Errorf("仿真会话已有后端计算连接")
	}
	defer a.active.Delete(session.ID)
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Duration(a.cfg.PodReadyTimeoutSeconds)*time.Second)
		defer cancel()
		if err := a.Release(releaseCtx, session); err != nil {
			serveErr = errors.Join(serveErr, err)
		}
	}()

	if err := a.prepareSession(ctx, session); err != nil {
		return err
	}
	initial, err := a.run(ctx, session, session.InitParams)
	if err != nil {
		return fmt.Errorf("计算初始仿真状态失败: %w", err)
	}
	if err := conn.SendJSON(BackendState{Tick: 0, State: initial}); err != nil {
		return fmt.Errorf("发送初始仿真状态失败: %w", err)
	}

	var tick int64
	maxTicks := backendExecutionLimit(session.ScaleLimit)
	for {
		if tick >= maxTicks {
			return fmt.Errorf("后端仿真已达到包声明的执行步数上限")
		}
		var event BackendEvent
		if err := conn.ReadJSON(&event); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("读取后端仿真事件失败: %w", err)
		}
		state, err := a.run(ctx, session, event.Payload)
		if err != nil {
			return fmt.Errorf("执行后端仿真事件失败: %w", err)
		}
		tick++
		if err := conn.SendJSON(BackendState{Tick: tick, State: state}); err != nil {
			return fmt.Errorf("发送后端仿真状态失败: %w", err)
		}
	}
}

// Release 删除会话独占命名空间,调用可重复执行。
func (a *StdioJSONAdapter) Release(ctx context.Context, session SessionWithPackage) error {
	policy := metav1.DeletePropagationBackground
	err := a.k8s.Clientset().CoreV1().Namespaces().Delete(ctx, a.namespace(session.ID), metav1.DeleteOptions{PropagationPolicy: &policy})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除后端仿真会话资源失败: %w", err)
	}
	return nil
}

// prepareSession 清理残留资源后创建独立命名空间、拒绝全部网络的策略和计算 Pod。
func (a *StdioJSONAdapter) prepareSession(ctx context.Context, session SessionWithPackage) error {
	namespace := a.namespace(session.ID)
	if err := a.deleteNamespaceAndWait(ctx, namespace); err != nil {
		return err
	}
	podLabels := map[string]string{
		"app.kubernetes.io/name":      "sim-backend",
		"app.kubernetes.io/component": "stdio-json",
		"chaimir.io/adapter":          a.profile.Code,
		"chaimir.io/session-id":       strconv.FormatInt(session.ID, 10),
	}
	namespaceLabels := map[string]string{
		"app.kubernetes.io/name":             "sim-backend",
		"app.kubernetes.io/component":        "stdio-json",
		"app.kubernetes.io/part-of":          "chaimir",
		"chaimir.io/adapter":                 a.profile.Code,
		"chaimir.io/session-id":              strconv.FormatInt(session.ID, 10),
		"chaimir.io/sim":                     "true",
		"chaimir.io/managed-by":              "chaimir-backend",
		"pod-security.kubernetes.io/enforce": "restricted",
		"pod-security.kubernetes.io/audit":   "restricted",
		"pod-security.kubernetes.io/warn":    "restricted",
	}
	if _, err := a.k8s.Clientset().CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: namespaceLabels}}, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建后端仿真会话命名空间失败: %w", err)
	}
	if err := a.k8s.SyncImagePullSecrets(ctx, a.sandbox.ControlNamespace, namespace, "sim", a.sandbox.ImagePullSecretNames); err != nil {
		return err
	}
	automount := false
	serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: stdioJSONServiceAccount, Namespace: namespace, Labels: podLabels}, AutomountServiceAccountToken: &automount}
	if _, err := a.k8s.Clientset().CoreV1().ServiceAccounts(namespace).Create(ctx, serviceAccount, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建后端仿真 ServiceAccount 失败: %w", err)
	}
	policy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "deny-all", Namespace: namespace}, Spec: networkingv1.NetworkPolicySpec{PodSelector: metav1.LabelSelector{}, PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress}}}
	if _, err := a.k8s.Clientset().NetworkingV1().NetworkPolicies(namespace).Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建后端仿真网络隔离策略失败: %w", err)
	}
	if _, err := a.k8s.Clientset().CoreV1().Pods(namespace).Create(ctx, a.sessionPod(namespace, podLabels), metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("创建后端仿真计算 Pod 失败: %w", err)
	}
	readyCtx, cancel := context.WithTimeout(ctx, time.Duration(a.cfg.PodReadyTimeoutSeconds)*time.Second)
	defer cancel()
	if err := wait.PollUntilContextCancel(readyCtx, 250*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		current, err := a.k8s.Clientset().CoreV1().Pods(namespace).Get(ctx, stdioJSONPod, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if current.Status.Phase == corev1.PodFailed || current.Status.Phase == corev1.PodSucceeded {
			return false, fmt.Errorf("后端仿真计算 Pod 提前结束: phase=%s", current.Status.Phase)
		}
		for _, condition := range current.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("等待后端仿真计算 Pod 就绪失败: %w", err)
	}
	return nil
}

// deleteNamespaceAndWait 删除服务异常退出后遗留的同会话资源,避免复用未知状态。
func (a *StdioJSONAdapter) deleteNamespaceAndWait(ctx context.Context, namespace string) error {
	policy := metav1.DeletePropagationBackground
	err := a.k8s.Clientset().CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{PropagationPolicy: &policy})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("清理残留后端仿真命名空间失败: %w", err)
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
		return fmt.Errorf("等待残留后端仿真命名空间删除失败: %w", err)
	}
	return nil
}

// sessionPod 构造无网络、非 root、只读根文件系统的计算 Pod。
func (a *StdioJSONAdapter) sessionPod(namespace string, labels map[string]string) *corev1.Pod {
	zero := int64(0)
	nonRoot := true
	readOnly := true
	allowEscalation := false
	automount := false
	pullSecrets := make([]corev1.LocalObjectReference, 0, len(a.sandbox.ImagePullSecretNames))
	for _, name := range a.sandbox.ImagePullSecretNames {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: name})
	}
	envNames := make([]string, 0, len(a.profile.Env))
	for name := range a.profile.Env {
		envNames = append(envNames, name)
	}
	sort.Strings(envNames)
	env := make([]corev1.EnvVar, 0, len(envNames))
	for _, name := range envNames {
		env = append(env, corev1.EnvVar{Name: name, Value: a.profile.Env[name]})
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: stdioJSONPod, Namespace: namespace, Labels: labels},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken:  &automount,
			ServiceAccountName:            stdioJSONServiceAccount,
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: &zero,
			ImagePullSecrets:              pullSecrets,
			NodeSelector:                  a.sandbox.SandboxNodeSelector,
			Tolerations:                   stdioJSONTolerations(a.sandbox.SandboxNodeTolerations),
			SecurityContext:               &corev1.PodSecurityContext{RunAsNonRoot: &nonRoot, SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}},
			Containers: []corev1.Container{{
				Name:            stdioJSONContainer,
				Image:           a.profile.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         append([]string(nil), a.profile.IdleCommand...),
				Env:             env,
				Resources:       corev1.ResourceRequirements{Requests: a.requests, Limits: a.limits},
				SecurityContext: &corev1.SecurityContext{RunAsNonRoot: &nonRoot, ReadOnlyRootFilesystem: &readOnly, AllowPrivilegeEscalation: &allowEscalation, Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}},
			}},
		},
	}
}

// run 限制 JSON 输入输出大小,再调用能力目录中登记的无 shell 命令。
func (a *StdioJSONAdapter) run(ctx context.Context, session SessionWithPackage, inputState map[string]any) (map[string]any, error) {
	if inputState == nil {
		return nil, fmt.Errorf("后端仿真输入不能为空")
	}
	if err := validateBackendStateScale(inputState, session.ScaleLimit); err != nil {
		return nil, err
	}
	input, err := jsonx.EncodeLineBytes(inputState)
	if err != nil {
		return nil, fmt.Errorf("编码后端仿真输入失败: %w", err)
	}
	if int64(len(input)) > a.profile.MaxInputBytes {
		return nil, fmt.Errorf("后端仿真输入超过部署上限")
	}
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(a.profile.ExecTimeoutSeconds)*time.Second)
	defer cancel()
	stdout := newLimitedBuffer(a.profile.MaxOutputBytes)
	stderr := newLimitedBuffer(a.profile.MaxOutputBytes)
	if err := a.k8s.Exec(execCtx, a.namespace(session.ID), stdioJSONPod, stdioJSONContainer, a.profile.Command, bytes.NewReader(input), stdout, stderr, false); err != nil {
		return nil, fmt.Errorf("后端仿真镜像执行失败: %w: %s", err, stderr.String())
	}
	var output map[string]any
	if err := jsonx.DecodeStrictUseNumber(stdout.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("后端仿真镜像输出无效: %w", err)
	}
	if output == nil {
		return nil, fmt.Errorf("后端仿真镜像输出必须为 JSON 对象")
	}
	if err := validateBackendStateScale(output, session.ScaleLimit); err != nil {
		return nil, fmt.Errorf("后端仿真镜像输出超过包声明边界: %w", err)
	}
	return output, nil
}

// backendExecutionLimit 取 max_tick 与 max_events 的更严格值,两者已在包审核边界校验为正整数。
func backendExecutionLimit(scaleLimit map[string]any) int64 {
	maxTick := int64(jsonx.IntFromAny(scaleLimit["max_tick"]))
	maxEvents := int64(jsonx.IntFromAny(scaleLimit["max_events"]))
	if maxTick < maxEvents {
		return maxTick
	}
	return maxEvents
}

// validateBackendStateScale 对通用状态中的 nodes 数组执行仿真包声明的规模上限。
func validateBackendStateScale(state map[string]any, scaleLimit map[string]any) error {
	value, exists := state["nodes"]
	if !exists {
		return nil
	}
	nodes, ok := value.([]any)
	if !ok {
		return fmt.Errorf("后端仿真状态的 nodes 必须为数组")
	}
	maxNodes := jsonx.IntFromAny(scaleLimit["nodes"])
	if len(nodes) > maxNodes {
		return fmt.Errorf("后端仿真节点数超过包声明上限")
	}
	return nil
}

// namespace 返回只由受控前缀和服务端会话编号构成的资源名。
func (a *StdioJSONAdapter) namespace(sessionID int64) string {
	return a.cfg.NamespacePrefix + strconv.FormatInt(sessionID, 10)
}

// stdioJSONResources 把已在启动配置层校验过的 quantity 转为 Pod 资源对象。
func stdioJSONResources(profile config.SimBackendAdapterConfig) (corev1.ResourceList, corev1.ResourceList, error) {
	parse := func(name, value string) (resource.Quantity, error) {
		quantity, err := resource.ParseQuantity(value)
		if err != nil {
			return resource.Quantity{}, fmt.Errorf("解析能力 %s 的 %s 失败: %w", profile.Code, name, err)
		}
		return quantity, nil
	}
	cpuRequest, err := parse("cpu_request", profile.CPURequest)
	if err != nil {
		return nil, nil, err
	}
	memoryRequest, err := parse("memory_request", profile.MemoryRequest)
	if err != nil {
		return nil, nil, err
	}
	cpuLimit, err := parse("cpu_limit", profile.CPULimit)
	if err != nil {
		return nil, nil, err
	}
	memoryLimit, err := parse("memory_limit", profile.MemoryLimit)
	if err != nil {
		return nil, nil, err
	}
	storageLimit, err := parse("ephemeral_storage_limit", profile.EphemeralStorageLimit)
	if err != nil {
		return nil, nil, err
	}
	return corev1.ResourceList{corev1.ResourceCPU: cpuRequest, corev1.ResourceMemory: memoryRequest}, corev1.ResourceList{corev1.ResourceCPU: cpuLimit, corev1.ResourceMemory: memoryLimit, corev1.ResourceEphemeralStorage: storageLimit}, nil
}

// stdioJSONTolerations 转换共享调度配置,避免 M4 依赖 M2 内部转换函数。
func stdioJSONTolerations(items []config.SandboxToleration) []corev1.Toleration {
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
		written, err := b.buffer.Write(data[:remaining])
		if err != nil {
			return written, fmt.Errorf("写入受限计算输出失败: %w", err)
		}
		return written, fmt.Errorf("计算输出超过部署上限")
	}
	return b.buffer.Write(data)
}

// Bytes 返回已接收的受限输出。
func (b *limitedBuffer) Bytes() []byte { return b.buffer.Bytes() }

// String 返回已接收的受限输出文本。
func (b *limitedBuffer) String() string { return b.buffer.String() }
