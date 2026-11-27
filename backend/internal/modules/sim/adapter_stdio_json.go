// 扩展包与镜像内置算法共用 stdio-json 协议和隔离资源生命周期。
package sim

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	platformk8s "chaimir/internal/platform/k8s"
	platformredis "chaimir/internal/platform/redis"
	"chaimir/pkg/apperr"
	"chaimir/pkg/limitio"

	"github.com/google/uuid"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
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
	locker   executionLocker
	cfg      config.SimBackendConfig
	profile  config.SimBackendAdapterConfig
	sandbox  config.SandboxConfig
	requests corev1.ResourceList
	limits   corev1.ResourceList
}

type executionLocker interface {
	WithLease(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) error) error
}

type stdioJSONExecution struct {
	namespace metav1.ObjectMeta
	pod       *corev1.Pod
}

// NewStdioJSONBackendRegistry 从部署能力目录构造注册表,同协议算法共享一套编排代码。
func NewStdioJSONBackendRegistry(k8sClient *platformk8s.Client, cfg config.SimBackendConfig, sandbox config.SandboxConfig, locker executionLocker) (BackendRegistry, error) {
	if k8sClient == nil {
		return nil, fmt.Errorf("stdio-json adapter 缺少 Kubernetes 客户端")
	}
	if locker == nil {
		return nil, fmt.Errorf("stdio-json adapter 缺少跨副本会话锁")
	}
	if cfg.SessionLeaseDurationSeconds <= 0 {
		return nil, fmt.Errorf("stdio-json adapter 缺少有效会话租约时长")
	}
	registry := make(BackendRegistry, len(cfg.StdioAdapters))
	for _, profile := range cfg.StdioAdapters {
		adapter, err := newStdioJSONAdapter(k8sClient, cfg, profile, sandbox, locker)
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

func newStdioJSONAdapter(k8sClient *platformk8s.Client, cfg config.SimBackendConfig, profile config.SimBackendAdapterConfig, sandbox config.SandboxConfig, locker executionLocker) (*StdioJSONAdapter, error) {
	requests, limits, err := stdioJSONResources(profile)
	if err != nil {
		return nil, err
	}
	return &StdioJSONAdapter{k8s: k8sClient, locker: locker, cfg: cfg, profile: profile, sandbox: sandbox, requests: requests, limits: limits}, nil
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

// Serve 独占会话执行权，在隔离容器中执行命令并推送快照。
// 扩展包经 exec stdin 装配；容器保持只读根文件系统和 deny-all 网络。
// 每次 exec 按 seed 和已执行事件重算，重连也沿同一记录恢复。
func (a *StdioJSONAdapter) Serve(ctx context.Context, session SessionWithPackage, bundle *ExecutionBundle, conn BackendConn) error {
	return a.withExecution(ctx, simExecutionScope(session), func(ctx context.Context, execution *stdioJSONExecution) error {
		recorded, err := conn.LoadReplay(ctx)
		if err != nil {
			return err
		}
		return a.serveLocked(ctx, session, execution, bundle, recorded, conn)
	})
}

func (a *StdioJSONAdapter) serveLocked(ctx context.Context, session SessionWithPackage, execution *stdioJSONExecution, bundle *ExecutionBundle, recorded []Action, conn BackendConn) error {
	if err := a.prepareSession(ctx, session, execution); err != nil {
		return err
	}
	// 容器每次 exec 都是新进程，按 seed 和已确认事件恢复。
	history := newRunnerHistory(session.ScaleLimit)
	if err := history.seed(recorded); err != nil {
		return err
	}
	var replay []runnerEvent
	if session.Status == SessionArchived {
		replay = history.events
		history.reset()
	}
	frame, err := a.firstFrame(ctx, execution, session, bundle, history)
	if err != nil {
		return err
	}
	// 首帧带上包自描述信息:浏览器不执行扩展包代码,操作清单与检查点标题只能由容器给出。
	descriptor := frame.Descriptor
	if replay != nil {
		frame.Snapshot.InteractionAvailability = map[string]any{"advance": history.length() < int64(len(replay))}
	}
	if err := conn.SendFrame(ctx, BackendStreamMessage{Type: backendStreamReady, Descriptor: &descriptor, Snapshot: frame.Snapshot, EventCount: history.length()}); err != nil {
		return fmt.Errorf("发送初始仿真快照失败: %w", err)
	}

	for {
		command, err := conn.ReadCommand(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("读取隔离执行命令失败: %w", err)
		}
		if replay != nil {
			if err := history.seekRecorded(replay, command.Kind); err != nil {
				return err
			}
			frame, err = a.replayRunner(ctx, execution, session, bundle, history)
		} else {
			frame, err = a.advance(ctx, execution, session, bundle, history, command)
		}
		if err != nil {
			return err
		}
		// 先登记再推帧:这条事件已经在容器里生效了,推送失败只是连接断开,不该把已发生的操作丢掉。
		if replay != nil {
			frame.Snapshot.InteractionAvailability = map[string]any{"advance": history.length() < int64(len(replay))}
		} else if err := conn.RecordExecuted(ctx, command); err != nil {
			return fmt.Errorf("登记隔离执行操作失败: %w", err)
		}
		if err := conn.SendFrame(ctx, BackendStreamMessage{Type: backendStreamSnapshot, Snapshot: frame.Snapshot, EventCount: history.length()}); err != nil {
			return fmt.Errorf("发送隔离仿真快照失败: %w", err)
		}
	}
}

func (a *StdioJSONAdapter) firstFrame(ctx context.Context, execution *stdioJSONExecution, session SessionWithPackage, bundle *ExecutionBundle, history *runnerHistory) (runnerFrame, error) {
	if len(history.events) > 0 {
		return a.replayRunner(ctx, execution, session, bundle, history)
	}
	frame, err := a.execRunner(ctx, execution, session.ScaleLimit, runnerCommand{
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

// 回退和重来通过重放恢复，与浏览器 Worker 保持同一语义。
func (a *StdioJSONAdapter) advance(ctx context.Context, execution *stdioJSONExecution, session SessionWithPackage, bundle *ExecutionBundle, history *runnerHistory, command BackendCommand) (runnerFrame, error) {
	switch command.Kind {
	case BackendCommandBack:
		history.back()
		return a.replayRunner(ctx, execution, session, bundle, history)
	case BackendCommandRestart:
		history.reset()
		return a.replayRunner(ctx, execution, session, bundle, history)
	default:
		executed, next, err := history.plan(command)
		if err != nil {
			return runnerFrame{}, err
		}
		frame, err := a.execRunner(ctx, execution, session.ScaleLimit, runnerCommand{
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
		if err := history.accept(executed, frame.Snapshot.Tick); err != nil {
			return runnerFrame{}, err
		}
		return frame, nil
	}
}

func (a *StdioJSONAdapter) replayRunner(ctx context.Context, execution *stdioJSONExecution, session SessionWithPackage, bundle *ExecutionBundle, history *runnerHistory) (runnerFrame, error) {
	frame, err := a.execRunner(ctx, execution, session.ScaleLimit, runnerCommand{
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

// Preview 复用隔离执行设施，同 seed 双跑验证确定性并生成审核样例。
func (a *StdioJSONAdapter) Preview(ctx context.Context, pkg Package, runID int64, bundle *ExecutionBundle, frameCount int) (PreviewResult, error) {
	if runID <= 0 || frameCount <= 0 || frameCount > maxPreviewFrames {
		return PreviewResult{}, fmt.Errorf("隔离预览运行编号或帧数量无效")
	}
	var result PreviewResult
	err := a.withExecution(ctx, ids.Format(runID), func(ctx context.Context, execution *stdioJSONExecution) error {
		var err error
		result, err = a.previewLocked(ctx, pkg, runID, execution, bundle, frameCount)
		return err
	})
	if err != nil {
		return PreviewResult{}, err
	}
	return result, nil
}

func (a *StdioJSONAdapter) previewLocked(ctx context.Context, pkg Package, runID int64, execution *stdioJSONExecution, bundle *ExecutionBundle, frameCount int) (PreviewResult, error) {
	session := SessionWithPackage{
		Session:    Session{ID: runID, InitParams: map[string]any{}, Seed: previewSeed},
		ScaleLimit: pkg.ScaleLimit,
	}
	if err := a.prepareSession(ctx, session, execution); err != nil {
		return PreviewResult{}, err
	}
	raw, err := a.execRunnerRaw(ctx, execution, runnerCommand{
		Op:         runnerOpVerify,
		Bundle:     bundle,
		InitParams: map[string]any{},
		Seed:       previewSeed,
		FrameCount: frameCount,
	})
	if err != nil {
		return PreviewResult{}, err
	}
	return decodePreviewResponse(raw, frameCount, pkg)
}

func simExecutionScope(session SessionWithPackage) string {
	scope := ids.Format(session.ID)
	if session.Status == SessionArchived {
		return scope + "-replay"
	}
	return scope
}

func (a *StdioJSONAdapter) withExecution(ctx context.Context, scope string, run func(context.Context, *stdioJSONExecution) error) error {
	// 必须先固定清理集合，再申请租约；旧执行恢复后也看不到后来接管者创建的资源。
	items, err := a.k8s.Clientset().CoreV1().Namespaces().List(ctx, metav1.ListOptions{LabelSelector: "chaimir.io/managed-by=chaimir-backend,chaimir.io/sim=true,chaimir.io/execution-scope=" + scope})
	if err != nil {
		return fmt.Errorf("读取仿真会话资源失败: %w", err)
	}
	return a.locker.WithLease(ctx, "sim.execution:"+scope, time.Duration(a.cfg.SessionLeaseDurationSeconds)*time.Second, func(ctx context.Context) (runErr error) {
		var failures []error
		for _, namespace := range items.Items {
			desired := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace.Name, Labels: map[string]string{
				"chaimir.io/managed-by": "chaimir-backend", "chaimir.io/sim": "true", "chaimir.io/execution-scope": scope,
			}}}
			if !strings.HasPrefix(namespace.Name, "sim-") || namespace.UID == "" || namespace.Namespace != "" {
				failures = append(failures, fmt.Errorf("历史仿真命名空间身份无效: %s", namespace.Name))
				continue
			}
			if err := platformk8s.VerifyManagedLabels(&namespace, desired); err != nil {
				failures = append(failures, err)
				continue
			}
		}
		if err := errors.Join(failures...); err != nil {
			return err
		}
		for _, namespace := range items.Items {
			if err := a.deleteNamespaceAndWait(ctx, namespace.ObjectMeta); err != nil {
				failures = append(failures, err)
			}
		}
		if err := errors.Join(failures...); err != nil {
			return err
		}
		if run == nil {
			return nil
		}
		execution := &stdioJSONExecution{namespace: metav1.ObjectMeta{Name: a.namespace()}}
		// 取消执行后仍持有租约，独立清理结束才允许接管。
		defer func() {
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Duration(a.cfg.PodReadyTimeoutSeconds)*time.Second)
			defer cancel()
			if execution.namespace.UID != "" {
				runErr = errors.Join(runErr, a.deleteNamespaceAndWait(releaseCtx, execution.namespace))
			}
		}()
		return run(ctx, execution)
	})
}

// decodePreviewResponse 严格校验容器响应、明确终态和请求数量，再复用教学帧协议校验。
func decodePreviewResponse(raw []byte, frameCount int, pkg Package) (PreviewResult, error) {
	var response runnerVerifyResponse
	if err := jsonx.DecodeStrictKnownFields(raw, &response); err != nil {
		return PreviewResult{}, fmt.Errorf("隔离预览输出无效: %w", err)
	}
	if !response.OK {
		return PreviewResult{}, fmt.Errorf("隔离预览失败: %s", response.Error)
	}
	if response.Determinism != validationPassed && response.Determinism != validationFailed {
		return PreviewResult{}, fmt.Errorf("隔离预览缺少有效的确定性结论")
	}
	if len(response.Frames) != frameCount {
		return PreviewResult{}, fmt.Errorf("隔离预览样例帧数量与请求不一致: expected=%d actual=%d", frameCount, len(response.Frames))
	}
	if err := validateBackendDescriptor(response.Descriptor, pkg.Code, pkg.Version, pkg.InteractionSchema); err != nil {
		return PreviewResult{}, err
	}
	if err := validatePreviewFrames(response.Frames, pkg.ScaleLimit); err != nil {
		return PreviewResult{}, err
	}
	return PreviewResult{
		DeterminismPassed: response.Determinism == "passed",
		Detail:            response.Detail,
		Frames:            response.Frames,
	}, nil
}

// Release 沿唯一删除等待流程确认会话独占命名空间已回收,调用可重复执行。
func (a *StdioJSONAdapter) Release(ctx context.Context, session SessionWithPackage) error {
	return wait.PollUntilContextTimeout(ctx, time.Duration(a.sandbox.ReadyPollIntervalSeconds)*time.Second, time.Duration(a.cfg.PodReadyTimeoutSeconds)*time.Second, true, func(ctx context.Context) (bool, error) {
		err := a.withExecution(ctx, ids.Format(session.ID), nil)
		if errors.Is(err, platformredis.ErrLeaseHeld) {
			return false, nil
		}
		return true, err
	})
}

func (a *StdioJSONAdapter) prepareSession(ctx context.Context, session SessionWithPackage, execution *stdioJSONExecution) error {
	namespace := execution.namespace.Name
	podLabels := map[string]string{
		"app.kubernetes.io/name":      "sim-backend",
		"app.kubernetes.io/component": "stdio-json",
		"chaimir.io/adapter":          a.profile.Code,
		"chaimir.io/execution-scope":  simExecutionScope(session),
	}
	namespaceLabels := map[string]string{
		"app.kubernetes.io/name":             "sim-backend",
		"app.kubernetes.io/component":        "stdio-json",
		"app.kubernetes.io/part-of":          "chaimir",
		"chaimir.io/adapter":                 a.profile.Code,
		"chaimir.io/execution-scope":         simExecutionScope(session),
		"chaimir.io/tenant-id":               ids.Format(session.TenantID),
		"chaimir.io/sim":                     "true",
		"chaimir.io/managed-by":              "chaimir-backend",
		"pod-security.kubernetes.io/enforce": "restricted",
		"pod-security.kubernetes.io/audit":   "restricted",
		"pod-security.kubernetes.io/warn":    "restricted",
	}
	// 统计真实资源，清理失败或正在终止的环境不能释放容量。
	if err := a.locker.WithLease(ctx, "sim.capacity:"+ids.Format(session.TenantID), time.Duration(a.cfg.SessionLeaseDurationSeconds)*time.Second, func(ctx context.Context) error {
		items, err := a.k8s.Clientset().CoreV1().Namespaces().List(ctx, metav1.ListOptions{LabelSelector: "chaimir.io/managed-by=chaimir-backend,chaimir.io/sim=true,chaimir.io/tenant-id=" + ids.Format(session.TenantID)})
		if err != nil {
			return fmt.Errorf("读取租户仿真环境容量失败: %w", err)
		}
		if len(items.Items) >= a.cfg.MaxConcurrentSessionsPerTenant {
			return apperr.ErrSimBackendComputeUnavailable.WithCause(fmt.Errorf("租户仿真环境达到上限: tenant_id=%d limit=%d", session.TenantID, a.cfg.MaxConcurrentSessionsPerTenant))
		}
		desired := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: namespaceLabels}}
		accepted, err := a.k8s.Clientset().CoreV1().Namespaces().Create(ctx, desired, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		if err := platformk8s.VerifyManagedResource(accepted, desired); err != nil {
			return err
		}
		execution.namespace = *accepted.ObjectMeta.DeepCopy()
		return nil
	}); err != nil {
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
	acceptedAccount, err := a.k8s.Clientset().CoreV1().ServiceAccounts(namespace).Create(ctx, serviceAccount, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("创建后端仿真 ServiceAccount 失败: %w", err)
	}
	if err := platformk8s.VerifyManagedResource(acceptedAccount, serviceAccount); err != nil {
		return err
	}
	if acceptedAccount.AutomountServiceAccountToken == nil || *acceptedAccount.AutomountServiceAccountToken || len(acceptedAccount.ImagePullSecrets) != 0 {
		return fmt.Errorf("后端仿真 ServiceAccount 的凭据策略与请求不一致")
	}
	policy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "deny-all", Namespace: namespace, Labels: podLabels}, Spec: networkingv1.NetworkPolicySpec{PodSelector: metav1.LabelSelector{}, PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress}}}
	acceptedPolicy, err := a.k8s.Clientset().NetworkingV1().NetworkPolicies(namespace).Create(ctx, policy, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("创建后端仿真网络隔离策略失败: %w", err)
	}
	if err := platformk8s.VerifyManagedResource(acceptedPolicy, policy); err != nil {
		return err
	}
	if !apiequality.Semantic.DeepEqual(acceptedPolicy.Spec, policy.Spec) {
		return fmt.Errorf("后端仿真网络隔离策略与请求不一致")
	}
	desiredPod := a.sessionPod(namespace, podLabels)
	acceptedPod, err := a.k8s.Clientset().CoreV1().Pods(namespace).Create(ctx, desiredPod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("创建后端仿真计算 Pod 失败: %w", err)
	}
	if err := platformk8s.VerifyManagedResource(acceptedPod, desiredPod); err != nil {
		return err
	}
	if !platformk8s.SamePodExecutionSpec(acceptedPod.Spec, desiredPod.Spec) {
		return fmt.Errorf("后端仿真计算 Pod 的执行规格与请求不一致")
	}
	execution.pod = acceptedPod.DeepCopy()
	readyCtx, cancel := context.WithTimeout(ctx, time.Duration(a.cfg.PodReadyTimeoutSeconds)*time.Second)
	defer cancel()
	if err := wait.PollUntilContextCancel(readyCtx, 250*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		current, err := a.k8s.Clientset().CoreV1().Pods(namespace).Get(ctx, stdioJSONPod, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return stdioJSONPodReady(current, acceptedPod)
	}); err != nil {
		return fmt.Errorf("等待后端仿真计算 Pod 就绪失败: %w", err)
	}
	return nil
}

// deleteNamespaceAndWait 只删除指定 UID 的资源；同名资源被重建后即视为原资源已回收。
func (a *StdioJSONAdapter) deleteNamespaceAndWait(ctx context.Context, namespace metav1.ObjectMeta) error {
	if namespace.UID == "" || !strings.HasPrefix(namespace.Name, "sim-") || namespace.Namespace != "" {
		return fmt.Errorf("待回收仿真命名空间缺少已确认的身份: %s", namespace.Name)
	}
	policy := metav1.DeletePropagationBackground
	err := a.k8s.Clientset().CoreV1().Namespaces().Delete(ctx, namespace.Name, metav1.DeleteOptions{PropagationPolicy: &policy, Preconditions: &metav1.Preconditions{UID: &namespace.UID}})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("删除后端仿真命名空间失败: %w", err)
	}
	if apierrors.IsNotFound(err) {
		return nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(a.cfg.PodReadyTimeoutSeconds)*time.Second)
	defer cancel()
	if err := wait.PollUntilContextCancel(waitCtx, 250*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		current, err := a.k8s.Clientset().CoreV1().Namespaces().Get(ctx, namespace.Name, metav1.GetOptions{})
		if err == nil {
			if current.Name != namespace.Name || current.Namespace != "" || current.UID == "" {
				return false, fmt.Errorf("待回收仿真命名空间返回无效身份: %s", namespace.Name)
			}
			return current.UID != namespace.UID, nil
		}
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}); err != nil {
		return fmt.Errorf("等待后端仿真命名空间删除失败: %w", err)
	}
	return nil
}

// bundle 使用随命名空间销毁的 emptyDir；fsGroup 允许非 root 进程写入。
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
			EnableServiceLinks:            &automount,
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

// runnerEvent 对齐容器 SimEvent；seq 包含时刻事件，区别于仅计用户操作的 Action.Seq。
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

// runnerHistory 仅提交已确认事件；容器每次 exec 按此历史重算。
type runnerHistory struct {
	events     []runnerEvent
	tick       int64
	seq        int64
	scaleLimit map[string]any
}

func newRunnerHistory(scaleLimit map[string]any) *runnerHistory {
	return &runnerHistory{events: []runnerEvent{}, seq: 1, scaleLimit: scaleLimit}
}

func (h *runnerHistory) exhausted() bool {
	return int64(len(h.events)) >= int64(jsonx.IntFromAny(h.scaleLimit["max_events"]))
}

func (h *runnerHistory) length() int64 {
	return int64(len(h.events))
}

// seed 补齐时刻推进后放入用户操作；非法记录必须失败，不能截断后继续执行。
func (h *runnerHistory) seed(recorded []Action) error {
	for i, action := range recorded {
		if action.Seq != int32(i+1) || int64(action.AtTick) < h.tick {
			return fmt.Errorf("仿真记录序号或时刻不连续")
		}
		if err := validateActionReplayPosition(action.Seq, action.AtTick, h.scaleLimit); err != nil {
			return fmt.Errorf("仿真记录超过包声明的规模: %w", err)
		}
		for h.tick < int64(action.AtTick) {
			h.commit(runnerEvent{Type: runnerSourceTick, Source: runnerSourceTick, AtTick: h.tick, Seq: h.seq})
		}
		payload := action.Payload
		target := strings.TrimSpace(jsonx.StringFromAny(payload["target"]))
		h.commit(runnerEvent{Type: strings.TrimSpace(action.EventType), Source: runnerSourceUser, AtTick: h.tick, Seq: h.seq, Payload: payload, Target: target})
	}
	return nil
}

// plan 不推进位置，等待容器确认后才提交事件。
func (h *runnerHistory) plan(command BackendCommand) (runnerEvent, runnerNext, error) {
	if h.exhausted() || (command.Kind == BackendCommandStep && h.tick >= int64(jsonx.IntFromAny(h.scaleLimit["max_tick"]))) {
		return runnerEvent{}, runnerNext{}, fmt.Errorf("隔离执行已达到包声明的规模上限")
	}
	if command.Kind == BackendCommandStep {
		return runnerEvent{Type: runnerSourceTick, Source: runnerSourceTick, AtTick: h.tick, Seq: h.seq}, runnerNext{Type: runnerNextTick}, nil
	}
	eventType := strings.TrimSpace(command.Event.EventType)
	target := strings.TrimSpace(jsonx.StringFromAny(command.Event.Payload["target"]))
	executed := runnerEvent{Type: eventType, Source: runnerSourceUser, AtTick: h.tick, Seq: h.seq, Payload: command.Event.Payload, Target: target}
	return executed, runnerNext{Type: runnerNextUser, EventType: eventType, Payload: command.Event.Payload, Target: target}, nil
}

// accept 核对容器确认的时刻；终态单步可能不推进，此时不追加虚构 tick。
func (h *runnerHistory) accept(event runnerEvent, tick int64) error {
	expected := h.tick
	if event.Source == runnerSourceTick {
		if tick == h.tick {
			return nil
		}
		expected++
	}
	if tick != expected {
		return fmt.Errorf("隔离执行返回的时刻与命令不一致")
	}
	h.commit(event)
	return nil
}

// commit 与容器引擎保持一致：序号逐条增加，时刻仅由 tick 事件推进。
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

func (h *runnerHistory) reset() {
	h.events = h.events[:0]
	h.tick, h.seq = 0, 1
}

// seekRecorded 只改变已记录事件的可见前缀，不接受浏览器注入新事件。
func (h *runnerHistory) seekRecorded(recorded []runnerEvent, command BackendCommandKind) error {
	cursor := len(h.events)
	switch command {
	case BackendCommandStep:
		cursor = min(cursor+1, len(recorded))
	case BackendCommandBack:
		cursor = max(cursor-1, 0)
	case BackendCommandRestart:
		cursor = 0
	default:
		return fmt.Errorf("归档回放不允许执行新操作")
	}
	h.events = recorded[:cursor]
	return nil
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
	Descriptor  BackendDescriptor `json:"descriptor"`
	Determinism string            `json:"determinism,omitempty"`
	Detail      string            `json:"detail,omitempty"`
	Frames      []BackendSnapshot `json:"frames,omitempty"`
}

// execRunner 执行一条命令并返回已通过后端协议校验的教学快照与包自描述信息。
func (a *StdioJSONAdapter) execRunner(ctx context.Context, execution *stdioJSONExecution, scaleLimit map[string]any, cmd runnerCommand) (runnerFrame, error) {
	raw, err := a.execRunnerRaw(ctx, execution, cmd)
	if err != nil {
		return runnerFrame{}, err
	}
	var response runnerSnapshotResponse
	if err := jsonx.DecodeStrictKnownFields(raw, &response); err != nil {
		return runnerFrame{}, fmt.Errorf("隔离执行输出无效: %w", err)
	}
	if !response.OK {
		return runnerFrame{}, fmt.Errorf("隔离执行失败: %s", response.Error)
	}
	// 包代码不可信，容器输出仍须在后端校验。
	if err := validateBackendSnapshot(response.Snapshot, scaleLimit); err != nil {
		return runnerFrame{}, err
	}
	return runnerFrame{Descriptor: response.Descriptor, Snapshot: response.Snapshot}, nil
}

// execRunnerRaw 编码命令、经 k8s exec 送入容器标准输入,并读回单行响应。
func (a *StdioJSONAdapter) execRunnerRaw(ctx context.Context, execution *stdioJSONExecution, cmd runnerCommand) ([]byte, error) {
	if cmd.InitParams == nil {
		cmd.InitParams = map[string]any{}
	}
	payload := map[string]any{
		"op":          cmd.Op,
		"seed":        cmd.Seed,
		"init_params": cmd.InitParams,
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
	if err := a.verifyExecution(execCtx, execution); err != nil {
		return nil, err
	}
	stdout := limitio.NewBuffer(a.profile.MaxOutputBytes)
	stderr := limitio.NewBuffer(a.profile.MaxOutputBytes)
	if err := a.k8s.Exec(execCtx, execution.namespace.Name, stdioJSONPod, stdioJSONContainer, a.profile.Command, bytes.NewReader(input), stdout, stderr, false); err != nil {
		return nil, fmt.Errorf("隔离执行镜像执行失败: %w: %s", err, stderr.String())
	}
	if err := a.verifyExecution(execCtx, execution); err != nil {
		return nil, err
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("隔离执行镜像没有输出")
	}
	return stdout.Bytes(), nil
}

func (a *StdioJSONAdapter) verifyExecution(ctx context.Context, execution *stdioJSONExecution) error {
	if execution == nil || execution.namespace.UID == "" || execution.pod == nil || execution.pod.UID == "" {
		return fmt.Errorf("后端仿真执行缺少已确认的资源身份")
	}
	namespace, err := a.k8s.Clientset().CoreV1().Namespaces().Get(ctx, execution.namespace.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("核验后端仿真命名空间失败: %w", err)
	}
	if err := platformk8s.VerifyManagedResource(namespace, &execution.namespace); err != nil {
		return err
	}
	if namespace.UID != execution.namespace.UID {
		return fmt.Errorf("后端仿真命名空间已被替换")
	}
	current, err := a.k8s.Clientset().CoreV1().Pods(namespace.Name).Get(ctx, execution.pod.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("核验后端仿真计算 Pod 失败: %w", err)
	}
	ready, err := stdioJSONPodReady(current, execution.pod)
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("后端仿真计算 Pod 已失去就绪状态")
	}
	return nil
}

func stdioJSONPodReady(current, original *corev1.Pod) (bool, error) {
	if err := platformk8s.VerifyManagedResource(current, original); err != nil {
		return false, err
	}
	if current.UID != original.UID || !platformk8s.SamePodExecutionSpec(current.Spec, original.Spec) {
		return false, fmt.Errorf("后端仿真计算 Pod 已被替换或修改")
	}
	if current.Status.Phase == corev1.PodFailed || current.Status.Phase == corev1.PodSucceeded {
		return false, fmt.Errorf("后端仿真计算 Pod 提前结束: phase=%s", current.Status.Phase)
	}
	if current.Status.Phase == corev1.PodRunning {
		for _, condition := range current.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
	}
	return false, nil
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

// 每次执行独占资源名,过期持有者只能清理本次资源,不能删掉新执行的命名空间。
func (a *StdioJSONAdapter) namespace() string {
	return a.cfg.NamespacePrefix + strings.ReplaceAll(uuid.NewString(), "-", "")[:22]
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
