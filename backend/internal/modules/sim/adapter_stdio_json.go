// sim adapter_stdio_json 文件实现数据驱动的 stdio-json Kubernetes 隔离执行适配器。
//
// 一套编排承载两类执行:扩展包(包正文经 exec stdin 按会话投递)与重计算算法(固化在镜像内)。
// 差别只在是否投递 bundle,故不复制第二份 K8s 生命周期代码
// (见 docs/04-仿真可视化引擎/02-架构设计.md §8.3)。
package sim

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/jsonx"
	platformk8s "chaimir/internal/platform/k8s"
	"chaimir/pkg/logging"

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
	// stdioJSONWorkVolume/Path 是容器内唯一可写点:根文件系统只读,而扩展包归档需要落盘装配。
	// 路径与 runner 的 SIM_RUNNER_WORKDIR 默认值一致(见 sim-sdk containerBundle.ts)。
	stdioJSONWorkVolume = "sim-bundle"
	stdioJSONWorkPath   = "/tmp/sim-bundle"
	// stdioJSONFSGroup 让非 root 容器能写 emptyDir:emptyDir 默认 root 拥有,不设 fsGroup 就写不进去。
	stdioJSONFSGroup int64 = 101
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

// Serve 创建隔离计算资源,装配 bundle 后推送首帧,再逐条执行已通过 M4 schema 校验的命令。
//
// bundle 非 nil 时(扩展包)先经 exec stdin 把归档字节推入容器:计算 Pod 根文件系统只读且
// 网络 deny-all,容器既写不下也取不到对象存储正文,放开网络等于拆掉隔离边界。
// bundle 为 nil 时(算法固化在镜像内的重计算能力)跳过装配,协议其余部分一致。
//
// 容器不常驻会话状态:每条命令都带上 seed + 已有事件,容器每次从初始状态重放到当前位置。
// 代价是重放开销(受包声明 max_events 约束),换来的是容器崩溃、Pod 重建都不影响过程可复现。
// 首帧同理按已登记操作重放:刷新页面或断线重连要回到上次离开的位置,不从头开始。
func (a *StdioJSONAdapter) Serve(ctx context.Context, session SessionWithPackage, bundle *ExecutionBundle, recorded []Action, conn BackendConn) (serveErr error) {
	if _, loaded := a.active.LoadOrStore(session.ID, struct{}{}); loaded {
		return fmt.Errorf("仿真会话已有隔离执行连接")
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
	namespace := a.namespace(session.ID)
	// 会话过程由后端持有:容器每次 exec 都是新进程,状态靠 seed + 已执行事件在容器内重放得到。
	// tick 与 seq 必须与容器内引擎同口径推进(见 sim-sdk runtime/engine.ts applyEvent):
	// 事件带的是执行前的时刻与序号,tick 只在时刻推进事件后加一。
	history := newRunnerHistory(backendExecutionLimit(session.ScaleLimit))
	history.seed(recorded)
	frame, err := a.firstFrame(ctx, namespace, session, bundle, history)
	if err != nil {
		return err
	}
	// 首帧带上包自描述信息:浏览器不执行扩展包代码,操作清单与检查点标题只能由容器给出。
	descriptor := frame.Descriptor
	if err := conn.SendFrame(BackendStreamMessage{Type: backendStreamReady, Descriptor: &descriptor, Snapshot: frame.Snapshot, EventCount: history.length()}); err != nil {
		return fmt.Errorf("发送初始仿真快照失败: %w", err)
	}

	for {
		command, err := conn.ReadCommand()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("读取隔离执行命令失败: %w", err)
		}
		frame, err := a.advance(ctx, namespace, session, bundle, history, command)
		if err != nil {
			return err
		}
		// 先登记再推帧:这条事件已经在容器里生效了,推送失败只是连接断开,不该把已发生的操作丢掉。
		if err := conn.RecordExecuted(command); err != nil {
			return fmt.Errorf("登记隔离执行操作失败: %w", err)
		}
		if err := conn.SendFrame(BackendStreamMessage{Type: backendStreamSnapshot, Snapshot: frame.Snapshot, EventCount: history.length()}); err != nil {
			return fmt.Errorf("发送隔离仿真快照失败: %w", err)
		}
	}
}

// firstFrame 产出连接建立时的第一帧:没有历史操作就装配后取初始状态,有历史就重放到那个位置。
func (a *StdioJSONAdapter) firstFrame(ctx context.Context, namespace string, session SessionWithPackage, bundle *ExecutionBundle, history *runnerHistory) (runnerFrame, error) {
	if len(history.events) > 0 {
		return a.replayRunner(ctx, namespace, session, bundle, history)
	}
	frame, err := a.execRunner(ctx, namespace, session.ScaleLimit, runnerCommand{
		Op:         runnerOpInit,
		Bundle:     bundle,
		InitParams: session.InitParams,
		Seed:       session.Seed,
	})
	if err != nil {
		return runnerFrame{}, fmt.Errorf("装配并计算初始仿真状态失败: %w", err)
	}
	return frame, nil
}

// advance 执行一条受控命令并返回新的教学帧。
//
// 推进与注入走 apply(在已有过程后追加一条事件);回退与重来走 restore
// (容器从初始状态重放到目标位置)—— 状态可复现而非就地反算,与浏览器 Worker 同一口径
// (见 M4 需求 C2、sim-sdk runtime/engine.ts back/replay)。
func (a *StdioJSONAdapter) advance(ctx context.Context, namespace string, session SessionWithPackage, bundle *ExecutionBundle, history *runnerHistory, command BackendCommand) (runnerFrame, error) {
	switch command.Kind {
	case BackendCommandBack:
		history.back()
		return a.replayRunner(ctx, namespace, session, bundle, history)
	case BackendCommandRestart:
		history.reset()
		return a.replayRunner(ctx, namespace, session, bundle, history)
	default:
		if history.exhausted() {
			return runnerFrame{}, fmt.Errorf("隔离执行已达到包声明的执行步数上限")
		}
		executed, next := history.plan(command)
		frame, err := a.execRunner(ctx, namespace, session.ScaleLimit, runnerCommand{
			Op:         runnerOpApply,
			Bundle:     bundle,
			InitParams: session.InitParams,
			Seed:       session.Seed,
			Events:     history.events,
			Next:       &next,
		})
		if err != nil {
			return runnerFrame{}, fmt.Errorf("执行隔离仿真事件失败: %w", err)
		}
		history.commit(executed)
		return frame, nil
	}
}

// replayRunner 让容器从初始状态重放到当前已保留的事件位置。
func (a *StdioJSONAdapter) replayRunner(ctx context.Context, namespace string, session SessionWithPackage, bundle *ExecutionBundle, history *runnerHistory) (runnerFrame, error) {
	frame, err := a.execRunner(ctx, namespace, session.ScaleLimit, runnerCommand{
		Op:         runnerOpRestore,
		Bundle:     bundle,
		InitParams: session.InitParams,
		Seed:       session.Seed,
		Events:     history.events,
	})
	if err != nil {
		return runnerFrame{}, fmt.Errorf("重算隔离仿真过程失败: %w", err)
	}
	return frame, nil
}

// Preview 在隔离容器内完成上架前预览:同 seed 双跑比对确定性,并渲出样例教学帧。
//
// 与生产运行同一套设施:"跑起来看效果对不对"本身就需要一个能安全执行第三方代码的地方,
// 不为审核另建一套(见 docs/04-仿真可视化引擎/06-业务流程与状态机.md §4)。
func (a *StdioJSONAdapter) Preview(ctx context.Context, pkg Package, bundle *ExecutionBundle, frameCount int) (PreviewResult, error) {
	session := SessionWithPackage{
		Session:    Session{ID: pkg.ID, InitParams: map[string]any{}, Seed: previewSeed},
		ScaleLimit: pkg.ScaleLimit,
	}
	if _, loaded := a.active.LoadOrStore(session.ID, struct{}{}); loaded {
		return PreviewResult{}, fmt.Errorf("仿真包已有隔离预览在执行")
	}
	defer a.active.Delete(session.ID)
	// 释放失败记日志而不并进返回值:预览的返回错误会写成包作者能看到的审核结论,
	// 而"命名空间没删掉"是集群侧问题、不是包的问题 —— 并进去等于给作者一个他改不了的失败原因。
	// 但它也绝不能被吞掉:Pod 泄漏必须能从日志定位到具体包与命名空间。
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Duration(a.cfg.PodReadyTimeoutSeconds)*time.Second)
		defer cancel()
		if err := a.Release(releaseCtx, session); err != nil {
			logging.ErrorContext(releaseCtx, "sim package preview release failed", err.Error(),
				slog.Int64("package_id", pkg.ID), slog.String("code", pkg.Code), slog.String("version", pkg.Version))
		}
	}()

	if err := a.prepareSession(ctx, session); err != nil {
		return PreviewResult{}, err
	}
	raw, err := a.execRunnerRaw(ctx, a.namespace(session.ID), runnerCommand{
		Op:         runnerOpVerify,
		Bundle:     bundle,
		InitParams: map[string]any{},
		Seed:       previewSeed,
		FrameCount: frameCount,
	})
	if err != nil {
		return PreviewResult{}, err
	}
	var response runnerVerifyResponse
	if err := jsonx.DecodeStrict(raw, &response); err != nil {
		return PreviewResult{}, fmt.Errorf("隔离预览输出无效: %w", err)
	}
	if !response.OK {
		return PreviewResult{}, fmt.Errorf("隔离预览失败: %s", response.Error)
	}
	// 帧同样要过后端协议校验:它们会展示给平台管理员作为判断依据,不能把非法帧当审核证据。
	for _, frame := range response.Frames {
		if err := validateBackendSnapshot(frame, pkg.ScaleLimit); err != nil {
			return PreviewResult{}, err
		}
	}
	return PreviewResult{
		DeterminismPassed: response.Determinism == "passed",
		Detail:            response.Detail,
		Frames:            response.Frames,
	}, nil
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
	if err := platformk8s.WaitForSandboxBackendRBAC(ctx, a.k8s.Clientset(), namespace, platformk8s.SandboxBackendServiceAccountNamespace, platformk8s.SandboxBackendServiceAccountName, time.Duration(a.cfg.PodReadyTimeoutSeconds)*time.Second, time.Duration(a.sandbox.ReadyPollIntervalSeconds)*time.Second); err != nil {
		return err
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
//
// 挂一个 emptyDir 到 /tmp/sim-bundle:根文件系统只读,而扩展包归档需要落盘才能按 entry 装配。
// 它是容器内唯一可写点,随会话命名空间删除一并消失,不跨会话残留。
// fsGroup 必须显式设置:emptyDir 默认 root 拥有,非 root 容器写不进去 —— 这个坑只有实际起容器才看得见。
func (a *StdioJSONAdapter) sessionPod(namespace string, labels map[string]string) *corev1.Pod {
	zero := int64(0)
	nonRoot := true
	readOnly := true
	allowEscalation := false
	automount := false
	fsGroup := stdioJSONFSGroup
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
	sizeLimit := a.limits[corev1.ResourceEphemeralStorage]
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
			SecurityContext:               &corev1.PodSecurityContext{RunAsNonRoot: &nonRoot, FSGroup: &fsGroup, SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}},
			Volumes: []corev1.Volume{{
				Name:         stdioJSONWorkVolume,
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: &sizeLimit}},
			}},
			Containers: []corev1.Container{{
				Name:            stdioJSONContainer,
				Image:           a.profile.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         append([]string(nil), a.profile.IdleCommand...),
				Env:             env,
				Resources:       corev1.ResourceRequirements{Requests: a.requests, Limits: a.limits},
				VolumeMounts:    []corev1.VolumeMount{{Name: stdioJSONWorkVolume, MountPath: stdioJSONWorkPath}},
				SecurityContext: &corev1.SecurityContext{RunAsNonRoot: &nonRoot, ReadOnlyRootFilesystem: &readOnly, AllowPrivilegeEscalation: &allowEscalation, Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}},
			}},
		},
	}
}

// runner 协议常量与命令结构。容器一次 exec 读一行 JSON 命令、写一行 JSON 响应后退出,
// 与 frontend/packages/sim-sdk/src/runtime/containerHost.ts 的 RunnerCommand 一一对应。
const (
	runnerOpInit     = "init"
	runnerOpApply    = "apply"
	runnerOpRestore  = "restore"
	runnerOpVerify   = "verify"
	runnerSourceUser = "user"
	runnerSourceTick = "tick"
	runnerNextTick   = "tick"
	runnerNextUser   = "user"
	// previewSeed 是隔离预览使用的固定随机种子:预览要判定确定性,种子必须可复现且与业务无关。
	previewSeed int64 = 1
)

// runnerEvent 是投递给容器的已执行事件项,字段名与容器侧 SimEvent 契约对齐(含 camelCase atTick)。
// 容器用它从初始状态重放到当前位置,故 atTick/seq 必须是事件执行时的值,少一个字段就重放不出同一条过程。
type runnerEvent struct {
	Type    string         `json:"type"`
	Source  string         `json:"source"`
	AtTick  int64          `json:"atTick"`
	Seq     int64          `json:"seq"`
	Payload map[string]any `json:"payload,omitempty"`
	Target  string         `json:"target,omitempty"`
}

// runnerNext 是本轮要执行的下一条命令,与容器侧 RunnerCommand.next 契约一致:
// 推进时刻只带 type=tick,注入交互带事件名与载荷。
type runnerNext struct {
	Type      string         `json:"type"`
	EventType string         `json:"event_type,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	Target    string         `json:"target,omitempty"`
}

// runnerHistory 持有一次隔离会话已执行的事件与推进位置。
//
// 为什么后端持有而不是容器:容器一次 exec 一个进程,不常驻状态。用重放换无状态容器是有意取舍 ——
// 容器崩溃、Pod 重建、后端换副本都不影响过程可复现,代价是重放开销(受包声明 max_events 约束)。
type runnerHistory struct {
	events    []runnerEvent
	tick      int64
	seq       int64
	maxEvents int64
}

// newRunnerHistory 按包声明的事件上限初始化过程记录。
func newRunnerHistory(maxEvents int64) *runnerHistory {
	return &runnerHistory{events: make([]runnerEvent, 0, maxEvents), seq: 1, maxEvents: maxEvents}
}

// exhausted 判定是否已达到包声明的事件上限。
func (h *runnerHistory) exhausted() bool {
	return int64(len(h.events)) >= h.maxEvents
}

// length 返回当前过程已执行的事件数。
func (h *runnerHistory) length() int64 {
	return int64(len(h.events))
}

// seed 按会话已登记的用户操作重建过程位置。
//
// 操作记录只存用户操作(时刻推进由 seed 决定可复算),故这里按每条操作的 at_tick
// 先补齐时刻推进事件再放入该操作 —— 与浏览器内置包恢复现场的摊平算法同一口径
// (见 frontend features/sim/replayMoves.ts),顺序错了重放出的就不是原过程。
func (h *runnerHistory) seed(recorded []Action) {
	for _, action := range recorded {
		for h.tick < int64(action.AtTick) && !h.exhausted() {
			h.commit(runnerEvent{Type: runnerSourceTick, Source: runnerSourceTick, AtTick: h.tick, Seq: h.seq})
		}
		if h.exhausted() {
			return
		}
		payload := action.Payload
		target := strings.TrimSpace(jsonx.StringFromAny(payload["target"]))
		h.commit(runnerEvent{Type: strings.TrimSpace(action.EventType), Source: runnerSourceUser, AtTick: h.tick, Seq: h.seq, Payload: payload, Target: target})
	}
}

// plan 把一条受控命令翻译成"待记录事件 + 容器命令",尚不推进位置 ——
// 容器执行失败时不应留下一条从未生效的事件。
func (h *runnerHistory) plan(command BackendCommand) (runnerEvent, runnerNext) {
	if command.Kind == BackendCommandStep {
		return runnerEvent{Type: runnerSourceTick, Source: runnerSourceTick, AtTick: h.tick, Seq: h.seq}, runnerNext{Type: runnerNextTick}
	}
	eventType := strings.TrimSpace(command.Event.EventType)
	target := strings.TrimSpace(jsonx.StringFromAny(command.Event.Payload["target"]))
	executed := runnerEvent{Type: eventType, Source: runnerSourceUser, AtTick: h.tick, Seq: h.seq, Payload: command.Event.Payload, Target: target}
	return executed, runnerNext{Type: runnerNextUser, EventType: eventType, Payload: command.Event.Payload, Target: target}
}

// commit 记录已成功执行的事件并推进位置,口径与容器内引擎一致:
// 序号每条加一,时刻只在时刻推进事件后加一。
func (h *runnerHistory) commit(event runnerEvent) {
	h.events = append(h.events, event)
	h.seq++
	if event.Source == runnerSourceTick {
		h.tick++
	}
}

// back 丢掉最近一条事件并把位置退回到它之前,口径与容器内引擎 replay 一致。
func (h *runnerHistory) back() {
	if len(h.events) == 0 {
		return
	}
	h.events = h.events[:len(h.events)-1]
	if len(h.events) == 0 {
		h.tick, h.seq = 0, 1
		return
	}
	last := h.events[len(h.events)-1]
	h.seq = last.Seq + 1
	h.tick = last.AtTick
	if last.Source == runnerSourceTick {
		h.tick = last.AtTick + 1
	}
}

// reset 回到初始状态。
func (h *runnerHistory) reset() {
	h.events = h.events[:0]
	h.tick, h.seq = 0, 1
}

// runnerCommand 是一次 exec 的完整输入。
// Bundle 为 nil 时省略归档字段:算法固化在镜像内的重计算能力不需要投递包正文。
type runnerCommand struct {
	Op         string
	Bundle     *ExecutionBundle
	InitParams map[string]any
	Seed       int64
	Events     []runnerEvent
	Next       *runnerNext
	FrameCount int
}

// runnerSnapshotResponse 是 init/apply 命令的响应。
type runnerSnapshotResponse struct {
	OK         bool              `json:"ok"`
	Error      string            `json:"error,omitempty"`
	Descriptor BackendDescriptor `json:"descriptor"`
	Snapshot   BackendSnapshot   `json:"snapshot"`
}

// runnerFrame 是一次命令产出的包自描述信息与教学快照。
type runnerFrame struct {
	Descriptor BackendDescriptor
	Snapshot   BackendSnapshot
}

// runnerVerifyResponse 是 verify 命令的响应。
type runnerVerifyResponse struct {
	OK          bool              `json:"ok"`
	Error       string            `json:"error,omitempty"`
	Determinism string            `json:"determinism,omitempty"`
	Detail      string            `json:"detail,omitempty"`
	Frames      []BackendSnapshot `json:"frames,omitempty"`
}

// execRunner 执行一条命令并返回已通过后端协议校验的教学快照与包自描述信息。
func (a *StdioJSONAdapter) execRunner(ctx context.Context, namespace string, scaleLimit map[string]any, cmd runnerCommand) (runnerFrame, error) {
	raw, err := a.execRunnerRaw(ctx, namespace, cmd)
	if err != nil {
		return runnerFrame{}, err
	}
	var response runnerSnapshotResponse
	if err := jsonx.DecodeStrict(raw, &response); err != nil {
		return runnerFrame{}, fmt.Errorf("隔离执行输出无效: %w", err)
	}
	if !response.OK {
		return runnerFrame{}, fmt.Errorf("隔离执行失败: %s", response.Error)
	}
	// 容器输出是不可信输入:它由外部提交的仿真包代码算出。后端必须自己校验一遍,
	// 只靠前端等于把协议边界交给不可信来源的下游(见 07 安全设计 §3)。
	if err := validateBackendSnapshot(response.Snapshot, scaleLimit); err != nil {
		return runnerFrame{}, err
	}
	return runnerFrame{Descriptor: response.Descriptor, Snapshot: response.Snapshot}, nil
}

// execRunnerRaw 编码命令、经 k8s exec 送入容器标准输入,并读回单行响应。
func (a *StdioJSONAdapter) execRunnerRaw(ctx context.Context, namespace string, cmd runnerCommand) ([]byte, error) {
	payload := map[string]any{
		"op":          cmd.Op,
		"seed":        cmd.Seed,
		"init_params": orEmptyObject(cmd.InitParams),
	}
	if cmd.Bundle != nil {
		payload["bundle_base64"] = base64.StdEncoding.EncodeToString(cmd.Bundle.Data)
		payload["bundle_hash"] = cmd.Bundle.Hash
		payload["bundle_format"] = cmd.Bundle.Format
		payload["entry"] = cmd.Bundle.Entry
	}
	if cmd.Op == runnerOpApply {
		payload["events"] = cmd.Events
		payload["next"] = cmd.Next
	}
	if cmd.Op == runnerOpRestore {
		payload["events"] = cmd.Events
	}
	if cmd.Op == runnerOpVerify {
		payload["frame_count"] = cmd.FrameCount
	}
	input, err := jsonx.EncodeLineBytes(payload)
	if err != nil {
		return nil, fmt.Errorf("编码隔离执行输入失败: %w", err)
	}
	if int64(len(input)) > a.profile.MaxInputBytes {
		return nil, fmt.Errorf("隔离执行输入超过部署上限")
	}
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(a.profile.ExecTimeoutSeconds)*time.Second)
	defer cancel()
	stdout := newLimitedBuffer(a.profile.MaxOutputBytes)
	stderr := newLimitedBuffer(a.profile.MaxOutputBytes)
	if err := a.k8s.Exec(execCtx, namespace, stdioJSONPod, stdioJSONContainer, a.profile.Command, bytes.NewReader(input), stdout, stderr, false); err != nil {
		return nil, fmt.Errorf("隔离执行镜像执行失败: %w: %s", err, stderr.String())
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("隔离执行镜像没有输出")
	}
	return stdout.Bytes(), nil
}

// orEmptyObject 保证初始参数始终是对象,容器侧不必区分 null 与空对象。
func orEmptyObject(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
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

// Len 返回已收集的输出长度,供调用方区分"镜像没输出"与"输出无效"。
func (b *limitedBuffer) Len() int { return b.buffer.Len() }

// String 返回已接收的受限输出文本。
func (b *limitedBuffer) String() string { return b.buffer.String() }
