// sandbox service 文件定义服务依赖注入和通用业务编排,不接收数据库连接。
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/workload"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/limitio"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"
)

// Orchestrator 定义 M2 service 需要的 K8s 编排能力。
type Orchestrator interface {
	// CreateSandboxResources 创建 Namespace、资源限制、默认拒绝网络、PVC、Pod 和工具 Service。
	CreateSandboxResources(ctx context.Context, plan CreateSandboxPlan) error
	// DestroySandboxResources 删除普通沙箱资源。
	DestroySandboxResources(ctx context.Context, sb Sandbox) error
	// StopComputeKeepSnapshot 释放计算工作负载但保留快照命名空间。
	StopComputeKeepSnapshot(ctx context.Context, sb Sandbox) error
	// CreateSnapshot 创建 CSI VolumeSnapshot 并返回 namespaced 引用与实际覆盖卷域。
	CreateSnapshot(ctx context.Context, plan CreateSandboxPlan, retention time.Duration) (SnapshotResult, error)
	// CleanupSnapshotResources 清理快照保留到期后的 Namespace/PVC/VolumeSnapshot。
	CleanupSnapshotResources(ctx context.Context, sb Sandbox) error
	// RestoreSnapshotResources 基于保留 PVC 或 VolumeSnapshot 恢复沙箱运行资源。
	RestoreSnapshotResources(ctx context.Context, plan CreateSandboxPlan) error
	// ResourceUsage 汇总沙箱当前已申请资源,用于状态查询返回资源用量。
	ResourceUsage(ctx context.Context, sb Sandbox) (contracts.SandboxResourceUsage, error)
	// EnsureWorkspaceAccess 在最终归档前恢复已终止的运行时 Pod,并重新挂载原工作区 PVC。
	EnsureWorkspaceAccess(ctx context.Context, plan CreateSandboxPlan) error
	// Exec 在沙箱容器中执行受控命令。
	Exec(ctx context.Context, namespace, container string, command []string, stdin []byte, tty bool) ([]byte, []byte, error)
	// ExecStream 在沙箱容器中执行交互式命令并透传流。
	ExecStream(ctx context.Context, namespace, container string, command []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, tty bool) error
	// ResetSandboxRuntime 只重建运行时计算 Pod,保留工作区卷和沙箱命名空间。
	ResetSandboxRuntime(ctx context.Context, plan CreateSandboxPlan) error
	// PrepullImage 创建或更新预拉取 DaemonSet 并等待工作负载镜像集合在真实节点 Ready。
	PrepullImage(ctx context.Context, image RuntimeImage, compositionDigest string, specs []PrepullImageSpec) (PrepullResult, error)
	// DeletePrepullDaemonSet 删除镜像预拉取 DaemonSet,用于镜像停用或删除闭环。
	DeletePrepullDaemonSet(ctx context.Context, image RuntimeImage) error
	// ToolReady 校验 Web 工具容器已达到可代理状态。
	ToolReady(ctx context.Context, sb Sandbox, tool Tool) error
	// SnapshotSupported 返回当前集群是否安装并启用 CSI 快照能力。
	SnapshotSupported(ctx context.Context) (bool, error)
}

// sandboxResourceReadiness 是编排器可选的只读就绪探针,用于服务重启后恢复已存在的资源。
// 旧的编排器实现没有该探针时仍走完整的幂等创建流程。
type sandboxResourceReadiness interface {
	SandboxResourcesReady(context.Context, CreateSandboxPlan) (bool, error)
}

// sandboxExecFailure 保留统一的输出超限错误,其余底层执行失败映射到调用场景已有错误码。
func sandboxExecFailure(fallback *apperr.Error, err error, stderr []byte) error {
	if errors.Is(err, limitio.ErrLimitExceeded) {
		return apperr.ErrSandboxExecOutputLimitExceeded.WithCause(err)
	}
	return fallback.WithCause(fmt.Errorf("%w: %s", err, string(stderr)))
}

// PrepullResult 描述 K8s 预拉取 DaemonSet 的真实节点状态。
type PrepullResult struct {
	DesiredNodes int32
	ReadyNodes   int32
	DaemonSet    string
	Detail       []byte
}

// ChainCapability 定义运行时 L2 链能力实现器。
type ChainCapability interface {
	Deploy(ctx context.Context, sb Sandbox, runtime Runtime, payload map[string]any) (map[string]any, error)
	SendTx(ctx context.Context, sb Sandbox, runtime Runtime, payload map[string]any) (map[string]any, error)
	Query(ctx context.Context, sb Sandbox, runtime Runtime, target string) (map[string]any, error)
	Reset(ctx context.Context, sb Sandbox, runtime Runtime) error
}

// objectStorage 描述 M2 需要复用的统一对象存储能力,生产实现来自 platform/storage。
type objectStorage interface {
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	BucketCode() string
	BucketAttach() string
}

// Service 承载 sandbox 模块业务编排,依赖 repo 接口和平台横切能力。
type Service struct {
	store        Store
	ids          snowflake.Generator
	cfg          config.SandboxConfig
	minio        objectStorage
	orchestrator Orchestrator
	audit        audit.Writer
	identity     contracts.IdentityService
	bus          eventbus.Bus
	wsHub        *ws.Hub
	capabilities map[string]ChainCapability
	saveMu       sync.Mutex
	saveTimers   map[int64]*time.Timer
	startupMu    sync.Mutex
	startup      map[int64]struct{}
}

// resolvedSandboxCreateDependencies 是创建或发布前校验沙箱模板时解析出的 M2 能力快照。
type resolvedSandboxCreateDependencies struct {
	Runtimes []RuntimePlan
	Quota    TenantQuota
	Tools    []Tool
}

// ServiceDeps 是 sandbox service 的装配依赖集合。
type ServiceDeps struct {
	Store        Store
	IDs          snowflake.Generator
	Config       config.SandboxConfig
	Storage      *storage.Storage
	Orchestrator Orchestrator
	Audit        audit.Writer
	Identity     contracts.IdentityService
	EventBus     eventbus.Bus
	WSHub        *ws.Hub
	Capabilities map[string]ChainCapability
}

// NewService 构造 sandbox 服务,不接收数据库连接,由装配层传入 Store。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("sandbox service 缺少 store")
	}
	if deps.IDs == nil {
		return nil, fmt.Errorf("sandbox service 缺少 ID 生成器")
	}
	if deps.Orchestrator == nil {
		return nil, fmt.Errorf("sandbox service 缺少 K8s 编排器")
	}
	if deps.Storage == nil {
		return nil, fmt.Errorf("sandbox service 缺少统一对象存储")
	}
	if deps.Audit == nil {
		return nil, fmt.Errorf("sandbox service 缺少审计写入器")
	}
	if deps.Identity == nil {
		return nil, fmt.Errorf("sandbox service 缺少身份读取契约")
	}
	if deps.EventBus == nil {
		return nil, fmt.Errorf("sandbox service 缺少事件总线")
	}
	capabilities := map[string]ChainCapability{}
	for key, capability := range deps.Capabilities {
		capabilities[key] = capability
	}
	capabilities[BuiltinExecCapability] = execChainCapability{orchestrator: deps.Orchestrator, timeoutSeconds: deps.Config.ChainRPCTimeoutSeconds}
	return &Service{
		store:        deps.Store,
		ids:          deps.IDs,
		cfg:          deps.Config,
		minio:        deps.Storage,
		orchestrator: deps.Orchestrator,
		audit:        deps.Audit,
		identity:     deps.Identity,
		bus:          deps.EventBus,
		wsHub:        deps.WSHub,
		capabilities: capabilities,
		saveTimers:   map[int64]*time.Timer{},
		startup:      map[int64]struct{}{},
	}, nil
}

// ValidateSandboxTemplate 校验教师/业务模块声明的实验环境能解析到当前可调度的运行时、镜像和工具。
func (s *Service) validateSandboxTemplateContract(ctx context.Context, req contracts.SandboxCreateRequest) error {
	input := createInputFromContract(req)
	if err := validateCreateRequest(input); err != nil {
		return err
	}
	digest, err := contracts.CanonicalSnapshotDigest(req.CompositionSnapshot)
	if err != nil || digest != req.CompositionSnapshot.Digest {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	return s.store.TenantTx(ctx, input.TenantID, func(ctx context.Context, tx TxStore) error {
		_, err := s.resolveSandboxCreateDependencies(ctx, tx, input, false)
		return err
	})
}

// CreateSandbox 创建沙箱控制面记录并异步推进 K8s 启动。
func (s *Service) createSandboxContract(ctx context.Context, req contracts.SandboxCreateRequest) (contracts.SandboxInfo, error) {
	digest, err := contracts.CanonicalSnapshotDigest(req.CompositionSnapshot)
	if err != nil || strings.TrimSpace(req.CompositionSnapshot.Digest) == "" || req.CompositionSnapshot.Digest != digest {
		return contracts.SandboxInfo{}, apperr.ErrSandboxCreateRequestInvalid
	}
	input := createInputFromContract(req)
	if err := validateCreateRequest(input); err != nil {
		return contracts.SandboxInfo{}, err
	}
	sharedAccountIDs, err := normalizeSandboxSharedAccountIDs(input.OwnerAccountID, input.AuthorizedAccountIDs)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	if err := s.validateSandboxSharedAccounts(ctx, input.TenantID, sharedAccountIDs); err != nil {
		return contracts.SandboxInfo{}, err
	}
	var plan CreateSandboxPlan
	var existingID int64
	if err := s.store.TenantTx(ctx, input.TenantID, func(ctx context.Context, tx TxStore) error {
		if isBattleMatchSourceRef(input.SourceRef) || input.SourceRef == input.ScopeRef {
			items, err := tx.ListSandboxesByScopeRef(ctx, input.TenantID, input.ScopeRef)
			if err != nil {
				return apperr.ErrSandboxCreateFailed.WithCause(err)
			}
			if len(items) > 0 {
				existingID = items[0].ID
				return nil
			}
		}
		resolved, err := s.resolveSandboxCreateDependencies(ctx, tx, input, true)
		if err != nil {
			return err
		}
		if len(resolved.Runtimes) == 0 {
			return apperr.ErrSandboxRuntimeUnavailable
		}
		sb, err := s.createSandboxRecord(ctx, tx, input, sharedAccountIDs, resolved.Quota)
		if err != nil {
			return err
		}
		if _, err := s.createToolRecords(ctx, tx, sb, resolved.Tools); err != nil {
			return err
		}
		detail, err := jsonBytes(map[string]any{
			"runtime_instances": runtimePlanAuditCodes(resolved.Runtimes),
			"source_ref":        input.SourceRef,
		})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), input.TenantID, sb.ID, EventTypeCreate, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		plan = CreateSandboxPlan{Sandbox: sb, WorkspaceRuntimeInstance: input.CompositionSnapshot.Spec.WorkspaceRuntimeInstance, Runtimes: resolved.Runtimes, Tools: resolved.Tools, PrivateSidecars: input.PrivateSidecars}
		plan.Sandbox.Status = SandboxStatusCreating
		return nil
	}); err != nil {
		if (isBattleMatchSourceRef(input.SourceRef) || input.SourceRef == input.ScopeRef) && db.IsUniqueViolation(err) {
			return s.infoBySourceRef(ctx, input.TenantID, input.SourceRef)
		}
		return contracts.SandboxInfo{}, err
	}
	if existingID > 0 {
		return s.info(ctx, input.TenantID, existingID)
	}
	if err := s.writeSystemAudit(ctx, input.TenantID, "sandbox.create", "sandbox", plan.Sandbox.ID, map[string]any{"source_ref": input.SourceRef}); err != nil {
		s.cleanupCreatedSandboxAfterAuditFailure(ctx, plan.Sandbox, err)
		return contracts.SandboxInfo{}, err
	}
	s.startAsync(ctx, plan)
	s.broadcastProgress(ctx, input.TenantID, plan.Sandbox.ID, SandboxPhaseAllocating, SandboxStatusCreating, response.TraceFromContext(ctx))
	return s.info(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID)
}

// validateSandboxSharedAccounts 确认共享账号真实存在且全部属于目标租户,弥补数组字段无法声明逐元素外键的限制。
func (s *Service) validateSandboxSharedAccounts(ctx context.Context, tenantID int64, accountIDs []int64) error {
	if len(accountIDs) == 0 {
		return nil
	}
	accounts, err := s.identity.BatchGetAccounts(ctx, accountIDs)
	if err != nil {
		return apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	want := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		want[accountID] = struct{}{}
	}
	for _, account := range accounts {
		if account.TenantID != tenantID {
			return apperr.ErrSandboxCreateRequestInvalid
		}
		delete(want, account.AccountID)
	}
	if len(want) != 0 {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	return nil
}

// isBattleMatchSourceRef 识别 M8 单场对局的稳定来源键,其余来源不施加单例限制。
func isBattleMatchSourceRef(sourceRef string) bool {
	parts := strings.Split(strings.TrimSpace(sourceRef), ":")
	if len(parts) != 4 || parts[0] != "contest" || len(parts[1]) != 4 || parts[2] != "battle" {
		return false
	}
	for _, ch := range parts[1] {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	_, ok := ids.Parse(parts[3])
	return ok
}

// infoBySourceRef 处理稳定来源键的并发创建竞争,返回已经创建的沙箱。
func (s *Service) infoBySourceRef(ctx context.Context, tenantID int64, sourceRef string) (contracts.SandboxInfo, error) {
	var items []Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListSandboxesBySourceRef(ctx, tenantID, sourceRef)
		return err
	}); err != nil {
		return contracts.SandboxInfo{}, apperr.ErrSandboxCreateFailed.WithCause(err)
	}
	if len(items) != 1 {
		return contracts.SandboxInfo{}, apperr.ErrSandboxCreateFailed
	}
	return s.info(ctx, tenantID, items[0].ID)
}

// resolveSandboxCreateDependencies 用同一套规则服务发布前校验和真实创建,避免 M7 与 M2 对运行时/工具可用性判断分叉。
func (s *Service) resolveSandboxCreateDependencies(ctx context.Context, tx TxStore, input CreateSandboxInputModel, enforceLiveCapacity bool) (resolvedSandboxCreateDependencies, error) {
	runtimes := make([]RuntimePlan, 0, len(input.CompositionSnapshot.Runtimes))
	for _, frozen := range input.CompositionSnapshot.Runtimes {
		runtime, err := tx.GetRuntimeByID(ctx, frozen.RuntimeID)
		if err != nil {
			return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		if runtime.Code != frozen.Code || runtime.Status != RuntimeStatusAvailable || runtime.SelftestStatus != RuntimeSelftestPassed {
			return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxRuntimeUnavailable
		}
		var frozenAdapter AdapterSpec
		if err := jsonx.DecodeStrictKnownFields(frozen.AdapterSpec, &frozenAdapter); err != nil {
			return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
		}
		runtime.AdapterSpec = frozenAdapter
		runtime.CapabilityImpl = frozen.CapabilityImpl
		image, err := tx.GetRuntimeImageByID(ctx, frozen.RuntimeID, frozen.ImageID)
		if err != nil {
			return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		if image.Version != frozen.ImageVersion || image.ImageURL != frozen.ImageURL || image.RuntimeID != frozen.RuntimeID || !image.GenesisBaked {
			return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxRuntimeImageNotFound
		}
		prepull, err := tx.GetCompositionPrepullForUpdate(ctx, image.ID, input.CompositionSnapshot.Digest)
		if err != nil || prepull.Status != ImagePrepullSucceeded {
			return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxRuntimeUnavailable
		}
		runtimes = append(runtimes, RuntimePlan{InstanceCode: frozen.InstanceCode, Runtime: runtime, Image: image})
	}
	if len(runtimes) == 0 {
		return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxRuntimeUnavailable
	}
	if _, err := tx.EnsureTenantQuota(ctx, s.defaultTenantQuota(input.TenantID)); err != nil {
		return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxQuotaInvalid.WithCause(err)
	}
	quota, err := tx.GetTenantQuotaForUpdate(ctx, input.TenantID)
	if err != nil {
		return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxQuotaInvalid.WithCause(err)
	}
	active := int64(0)
	if enforceLiveCapacity {
		active, err = tx.CountActiveSandboxes(ctx, input.TenantID)
		if err != nil {
			return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxCreateFailed.WithCause(err)
		}
	}
	tools := make([]Tool, 0, len(input.CompositionSnapshot.Components))
	for _, component := range input.CompositionSnapshot.Components {
		catalog, err := tx.GetToolByCode(ctx, component.Code)
		if err != nil || catalog.ID != component.ComponentID || catalog.Status != ToolStatusAvailable {
			return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxToolIncompatible
		}
		var resourceSpec ToolResourceSpec
		if err := jsonx.DecodeStrictKnownFields(component.ResourceSpec, &resourceSpec); err != nil {
			return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxToolIncompatible.WithCause(err)
		}
		catalog.Category = component.Category
		catalog.ResourceSpec = resourceSpec
		if err := validateToolForRuntime(catalog, runtimes); err != nil {
			return resolvedSandboxCreateDependencies{}, err
		}
		tools = append(tools, catalog)
	}
	if err := validatePrivateSidecars(input.PrivateSidecars, s.cfg, adapterSpecForPrivateSidecarValidation(runtimes)); err != nil {
		return resolvedSandboxCreateDependencies{}, err
	}
	for _, runtimePlan := range runtimes {
		if err := validatePlanImagesCurrentlyAdmitted(s.cfg, runtimePlan.Runtime, runtimePlan.Image, tools, input.PrivateSidecars); err != nil {
			return resolvedSandboxCreateDependencies{}, err
		}
	}
	if err := validateQuotaForCreate(input, quota, active, s.cfg, aggregateRuntimeAdapterSpecs(runtimes), tools); err != nil {
		return resolvedSandboxCreateDependencies{}, err
	}
	if input.SnapshotEnabled {
		ok, err := s.orchestrator.SnapshotSupported(ctx)
		if err != nil {
			return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxSnapshotUnavailable.WithCause(err)
		}
		if !ok {
			return resolvedSandboxCreateDependencies{}, apperr.ErrSandboxSnapshotUnavailable
		}
	}
	return resolvedSandboxCreateDependencies{Runtimes: runtimes, Quota: quota, Tools: tools}, nil
}

// adapterSpecForPrivateSidecarValidation aggregates every runtime container so private names cannot collide with another runtime instance.
func adapterSpecForPrivateSidecarValidation(plans []RuntimePlan) AdapterSpec {
	var out AdapterSpec
	for _, plan := range plans {
		adapter := plan.Runtime.AdapterSpec
		out.InfraSidecars = append(out.InfraSidecars, adapter.RuntimeContainer)
		out.InfraSidecars = append(out.InfraSidecars, adapter.InfraSidecars...)
		for _, pod := range adapter.Pods {
			out.InfraSidecars = append(out.InfraSidecars, pod.Containers...)
		}
	}
	return out
}

// aggregateRuntimeAdapterSpecs combines runtime resource domains for tenant quota validation.
func aggregateRuntimeAdapterSpecs(plans []RuntimePlan) AdapterSpec {
	items := make(map[string]AdapterSpec, len(plans))
	for _, plan := range plans {
		items[plan.InstanceCode] = plan.Runtime.AdapterSpec
	}
	return aggregateRuntimeAdapterSpecsByMap(items)
}

func aggregateRuntimeAdapterSpecsByMap(items map[string]AdapterSpec) AdapterSpec {
	var out AdapterSpec
	for _, item := range items {
		out.VolumeDomains = append(out.VolumeDomains, item.VolumeDomains...)
		out.NetworkRules = append(out.NetworkRules, item.NetworkRules...)
	}
	return out
}

func runtimePlanAuditCodes(plans []RuntimePlan) []string {
	out := make([]string, 0, len(plans))
	for _, plan := range plans {
		out = append(out, plan.InstanceCode+":"+plan.Runtime.Code)
	}
	return out
}

// cleanupCreatedSandboxAfterAuditFailure 避免审计失败后留下未启动的 creating 记录。
func (s *Service) cleanupCreatedSandboxAfterAuditFailure(ctx context.Context, sb Sandbox, cause error) {
	cleanupBase := logging.WithAttrs(context.WithoutCancel(ctx), logging.AttrsFromContext(ctx)...)
	cleanupCtx, cancel := context.WithTimeout(cleanupBase, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	if err := s.orchestrator.DestroySandboxResources(cleanupCtx, sb); err != nil {
		logging.ErrorContext(cleanupCtx, "sandbox create audit k8s cleanup failed", err.Error(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID), slog.String("namespace", sb.Namespace))
	}
	if err := s.store.TenantTx(cleanupCtx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, sb.TenantID, sb.ID, sb.Phase, SandboxStatusDestroyed); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		detail, err := jsonBytes(map[string]any{"stage": "create_audit", "error": logging.SanitizeError(cause.Error())})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeError, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		logging.ErrorContext(cleanupCtx, "sandbox create audit state cleanup failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID), slog.String("audit_error", logging.SanitizeError(cause.Error())))
	}
}

// GetSandbox 查询单个沙箱当前状态与工具接入信息。
func (s *Service) getSandboxContract(ctx context.Context, tenantID, sandboxID int64) (contracts.SandboxInfo, error) {
	return s.info(ctx, tenantID, sandboxID)
}

// GetSandboxForOwner 查询用户自己的沙箱,防止同租户内横向访问。
func (s *Service) GetSandboxForOwner(ctx context.Context, tenantID, accountID, sandboxID int64) (contracts.SandboxInfo, error) {
	if _, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID); err != nil {
		return contracts.SandboxInfo{}, err
	}
	info, err := s.info(ctx, tenantID, sandboxID)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	return info, nil
}

// PauseSandbox 暂停沙箱,按需创建 CSI 快照后释放计算工作负载。
func (s *Service) pauseSandboxContract(ctx context.Context, req contracts.SandboxControlRequest) error {
	if err := validateSandboxControlRequest(req); err != nil {
		return err
	}
	tenantID, sandboxID := req.TenantID, req.SandboxID
	var sb Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if sb.SourceRef != strings.TrimSpace(req.SourceRef) {
			return apperr.ErrSandboxOwnershipInvalid
		}
		if err := validateStateTransition(sb.Status, SandboxStatusPaused); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if sb.Status == SandboxStatusPaused {
		return nil
	}
	if _, _, err := s.saveSandboxFiles(ctx, tenantID, sandboxID); err != nil {
		return err
	}
	if sb.SnapshotEnabled {
		retention := time.Until(sb.SnapshotExpireAt)
		if retention <= 0 {
			retention = time.Minute
		}
		snapshotCtx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
		plan, err := s.planForExistingSandbox(ctx, sb)
		if err != nil {
			cancel()
			return err
		}
		result, err := s.orchestrator.CreateSnapshot(snapshotCtx, plan, retention)
		cancel()
		if err != nil {
			return apperr.ErrSandboxRecycleFailed.WithCause(err)
		}
		if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
			if _, err := tx.UpdateSandboxSnapshot(ctx, tenantID, sandboxID, result.Ref, result.Domains, timex.Now(), sb.SnapshotExpireAt); err != nil {
				return apperr.ErrSandboxStatePersistFailed.WithCause(err)
			}
			detail, err := jsonBytes(map[string]any{"snapshot_ref": result.Ref, "snapshot_domains": result.Domains})
			if err != nil {
				return apperr.ErrSandboxStatePersistFailed.WithCause(err)
			}
			if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), tenantID, sandboxID, EventTypePhaseChange, detail); err != nil {
				return apperr.ErrSandboxStatePersistFailed.WithCause(err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	if err := s.orchestrator.StopComputeKeepSnapshot(ctx, sb); err != nil {
		return apperr.ErrSandboxRecycleFailed.WithCause(err)
	}
	return s.transition(ctx, tenantID, sandboxID, SandboxPhaseReady, SandboxStatusPaused, "sandbox.pause")
}

// ResumeSandbox 恢复沙箱为运行态。
func (s *Service) resumeSandboxContract(ctx context.Context, req contracts.SandboxControlRequest) error {
	if err := validateSandboxControlRequest(req); err != nil {
		return err
	}
	tenantID, sandboxID := req.TenantID, req.SandboxID
	var sb Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if sb.SourceRef != strings.TrimSpace(req.SourceRef) {
			return apperr.ErrSandboxOwnershipInvalid
		}
		return nil
	}); err != nil {
		return err
	}
	if sb.Status == SandboxStatusDestroyed && strings.TrimSpace(sb.SnapshotRef) != "" {
		return s.restoreSnapshotSandbox(ctx, sb)
	}
	if sb.Status == SandboxStatusPaused {
		return s.resumePausedSandbox(ctx, sb)
	}
	return s.transition(ctx, tenantID, sandboxID, SandboxPhaseReady, SandboxStatusRunning, "sandbox.resume")
}

// resumePausedSandbox 重建已暂停沙箱的计算资源,成功后才恢复运行态。
func (s *Service) resumePausedSandbox(ctx context.Context, sb Sandbox) error {
	plan, err := s.planForExistingSandbox(ctx, sb)
	if err != nil {
		return err
	}
	if err := s.orchestrator.CreateSandboxResources(ctx, plan); err != nil {
		s.cleanupAfterResumeFailure(ctx, sb)
		s.markStartFailed(ctx, sb, err)
		return apperr.ErrSandboxCreateFailed.WithCause(err)
	}
	if err := s.updateToolReadiness(ctx, plan); err != nil {
		return err
	}
	return s.transition(ctx, sb.TenantID, sb.ID, SandboxPhaseReady, SandboxStatusRunning, "sandbox.resume")
}

// DestroySandbox 主动销毁单个沙箱。
func (s *Service) destroySandboxContract(ctx context.Context, req contracts.SandboxControlRequest) error {
	if err := validateSandboxControlRequest(req); err != nil {
		return err
	}
	tenantID, sandboxID := req.TenantID, req.SandboxID
	var sb Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if sb.SourceRef != strings.TrimSpace(req.SourceRef) {
			return apperr.ErrSandboxOwnershipInvalid
		}
		if err := validateStateTransition(sb.Status, SandboxStatusRecycling); err != nil {
			return err
		}
		_, err = tx.UpdateSandboxPhaseStatus(ctx, tenantID, sandboxID, sb.Phase, SandboxStatusRecycling)
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	return s.recycleOne(ctx, sb, "manual_destroy")
}

// validateSandboxControlRequest 校验内部生命周期控制请求必须绑定租户、沙箱和来源。
func validateSandboxControlRequest(req contracts.SandboxControlRequest) error {
	if req.TenantID <= 0 || req.SandboxID <= 0 || !validSourceRef(req.SourceRef) {
		return apperr.ErrSandboxContractRequestInvalid
	}
	return nil
}

// recycleByScopeRefContract 按生命周期作用域级联回收沙箱。
func (s *Service) recycleByScopeRefContract(ctx context.Context, req contracts.SandboxRecycleRequest) error {
	if req.TenantID <= 0 || !validSourceRef(req.SourceRef) || !auth.ValidScopeRef(req.ScopeRef) {
		return apperr.ErrSandboxRecycleRequestInvalid
	}
	var items []Sandbox
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListSandboxesByScopeRef(ctx, req.TenantID, req.ScopeRef)
		if err != nil {
			return apperr.ErrSandboxRecycleScanFailed.WithCause(err)
		}
		for _, item := range items {
			if item.Status != SandboxStatusRecycling {
				if err := validateStateTransition(item.Status, SandboxStatusRecycling); err != nil {
					return err
				}
				if _, err := tx.UpdateSandboxPhaseStatus(ctx, item.TenantID, item.ID, item.Phase, SandboxStatusRecycling); err != nil {
					return apperr.ErrSandboxStatePersistFailed.WithCause(err)
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, item := range items {
		if err := s.recycleOne(ctx, item, req.Reason); err != nil {
			return err
		}
	}
	return nil
}

// restoreSnapshotSandbox 恢复快照保留期内的沙箱计算资源并重新标记为运行中。
func (s *Service) restoreSnapshotSandbox(ctx context.Context, sb Sandbox) error {
	if !sb.SnapshotExpireAt.IsZero() && !sb.SnapshotExpireAt.After(timex.Now()) {
		return apperr.ErrSandboxSnapshotUnavailable
	}
	plan, err := s.planForExistingSandbox(ctx, sb)
	if err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, sb.TenantID, sb.ID, SandboxPhaseAllocating, SandboxStatusCreating); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := s.orchestrator.RestoreSnapshotResources(ctx, plan); err != nil {
		s.markStartFailed(ctx, sb, err)
		return apperr.ErrSandboxCreateFailed.WithCause(err)
	}
	if err := s.updateToolReadiness(ctx, plan); err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, sb.TenantID, sb.ID, SandboxPhaseReady, SandboxStatusReady); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		detail, err := jsonBytes(map[string]any{"phase": SandboxPhaseReady, "status": SandboxStatusReady, "mode": "snapshot_restore"})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypePhaseChange, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	s.broadcastProgress(ctx, sb.TenantID, sb.ID, SandboxPhaseReady, SandboxStatusReady, response.TraceFromContext(ctx))
	return s.writeSystemAudit(ctx, sb.TenantID, "sandbox.resume.snapshot", "sandbox", sb.ID, nil)
}

// planForExistingSandbox 重新加载沙箱恢复或暂停恢复所需的运行时、镜像和工具定义。
func (s *Service) planForExistingSandbox(ctx context.Context, sb Sandbox) (CreateSandboxPlan, error) {
	var snapshot contracts.SandboxCompositionSnapshot
	if err := jsonx.DecodeStrictKnownFields(sb.CompositionSnapshot, &snapshot); err != nil {
		return CreateSandboxPlan{}, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	digest, err := contracts.CanonicalSnapshotDigest(snapshot)
	if err != nil || digest != sb.CompositionDigest || snapshot.Digest != sb.CompositionDigest {
		return CreateSandboxPlan{}, apperr.ErrSandboxCreateRequestInvalid
	}
	runtimePlans := make([]RuntimePlan, 0, len(snapshot.Runtimes))
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		for _, frozen := range snapshot.Runtimes {
			runtime, err := tx.GetRuntimeByID(ctx, frozen.RuntimeID)
			if err != nil {
				return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
			}
			if runtime.Code != frozen.Code || runtime.Status != RuntimeStatusAvailable || runtime.SelftestStatus != RuntimeSelftestPassed {
				return apperr.ErrSandboxRuntimeUnavailable
			}
			if err := jsonx.DecodeStrictKnownFields(frozen.AdapterSpec, &runtime.AdapterSpec); err != nil {
				return apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
			}
			runtime.CapabilityImpl = frozen.CapabilityImpl
			image, err := tx.GetRuntimeImageByID(ctx, frozen.RuntimeID, frozen.ImageID)
			if err != nil {
				return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
			}
			prepull, prepullErr := tx.GetCompositionPrepullForUpdate(ctx, image.ID, snapshot.Digest)
			if image.ImageURL != frozen.ImageURL || image.Version != frozen.ImageVersion || prepullErr != nil || prepull.Status != ImagePrepullSucceeded || !image.GenesisBaked {
				return apperr.ErrSandboxRuntimeUnavailable
			}
			runtimePlans = append(runtimePlans, RuntimePlan{InstanceCode: frozen.InstanceCode, Runtime: runtime, Image: image})
		}
		return nil
	}); err != nil {
		return CreateSandboxPlan{}, err
	}
	tools, err := s.toolsForSandbox(ctx, sb.TenantID, sb.ID)
	if err != nil {
		return CreateSandboxPlan{}, err
	}
	for _, runtimePlan := range runtimePlans {
		if err := validatePlanImagesCurrentlyAdmitted(s.cfg, runtimePlan.Runtime, runtimePlan.Image, tools, nil); err != nil {
			return CreateSandboxPlan{}, err
		}
	}
	return CreateSandboxPlan{Sandbox: sb, WorkspaceRuntimeInstance: snapshot.Spec.WorkspaceRuntimeInstance, Runtimes: runtimePlans, Tools: tools}, nil
}

// toolsForSandbox 从不可变快照重建沙箱组件,当前目录只负责撤销门禁,不覆盖已发布执行规格。
func (s *Service) toolsForSandbox(ctx context.Context, tenantID, sandboxID int64) ([]Tool, error) {
	var sb Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		return err
	}); err != nil {
		return nil, apperr.ErrSandboxNotFound.WithCause(err)
	}
	var snapshot contracts.SandboxCompositionSnapshot
	if err := jsonx.DecodeStrictKnownFields(sb.CompositionSnapshot, &snapshot); err != nil {
		return nil, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	tools := make([]Tool, 0, len(snapshot.Components))
	for _, component := range snapshot.Components {
		var tool Tool
		if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
			var err error
			tool, err = tx.GetToolByCode(ctx, component.Code)
			if err != nil {
				return apperr.ErrSandboxToolNotFound.WithCause(err)
			}
			return nil
		}); err != nil {
			return nil, err
		}
		if tool.ID != component.ComponentID || tool.Status != ToolStatusAvailable {
			return nil, apperr.ErrSandboxToolIncompatible
		}
		if err := jsonx.DecodeStrictKnownFields(component.ResourceSpec, &tool.ResourceSpec); err != nil {
			return nil, apperr.ErrSandboxToolIncompatible.WithCause(err)
		}
		tool.Category = component.Category
		tools = append(tools, tool)
	}
	return tools, nil
}

// Stats 返回租户级沙箱资源统计。
func (s *Service) statsContract(ctx context.Context, tenantID int64) (contracts.SandboxQuotaStats, error) {
	var quota TenantQuota
	var active int64
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.EnsureTenantQuota(ctx, s.defaultTenantQuota(tenantID)); err != nil {
			return apperr.ErrSandboxQuotaInvalid.WithCause(err)
		}
		var err error
		quota, active, err = tx.StatsByTenant(ctx, tenantID)
		if err != nil {
			return apperr.ErrSandboxQuotaInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return contracts.SandboxQuotaStats{}, err
	}
	return contracts.SandboxQuotaStats{
		TenantID:                tenantID,
		ActiveSandboxCount:      active,
		MaxConcurrentSandbox:    quota.MaxConcurrentSandbox,
		MaxCPU:                  quota.MaxCPU,
		MaxMemoryMB:             quota.MaxMemoryMB,
		IdleTimeoutMin:          quota.IdleTimeoutMin,
		MaxLifetimeMin:          quota.MaxLifetimeMin,
		MaxKeepaliveMin:         quota.MaxKeepaliveMin,
		MaxSnapshotRetentionMin: quota.MaxSnapshotRetentionMin,
	}, nil
}

// EnsureTenantQuota 建立租户配额基线且不覆盖管理员已有配置。
func (s *Service) EnsureTenantQuota(ctx context.Context, tenantID int64) error {
	if tenantID <= 0 {
		return apperr.ErrSandboxQuotaInvalid
	}
	quota := s.defaultTenantQuota(tenantID)
	if err := validateQuota(quota); err != nil {
		return err
	}
	return s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		_, err := tx.EnsureTenantQuota(ctx, quota)
		if err != nil {
			return apperr.ErrSandboxQuotaPersistFailed.WithCause(err)
		}
		return nil
	})
}

// defaultTenantQuota 从唯一配置源构造新租户配额。
func (s *Service) defaultTenantQuota(tenantID int64) TenantQuota {
	return TenantQuota{
		TenantID:                tenantID,
		MaxConcurrentSandbox:    s.cfg.TenantDefaultMaxConcurrent,
		MaxCPU:                  s.cfg.TenantDefaultMaxCPU,
		MaxMemoryMB:             s.cfg.TenantDefaultMaxMemoryMB,
		IdleTimeoutMin:          s.cfg.TenantDefaultIdleTimeoutMin,
		MaxLifetimeMin:          s.cfg.TenantDefaultMaxLifetimeMin,
		MaxKeepaliveMin:         s.cfg.TenantDefaultMaxKeepaliveMin,
		MaxSnapshotRetentionMin: s.cfg.TenantDefaultSnapshotMin,
	}
}

// resolveTools 只解析调用方明确提交的工具并校验运行时适配性。
func (s *Service) resolveTools(ctx context.Context, tx TxStore, runtimes []RuntimePlan, codes []string) ([]Tool, error) {
	if len(codes) == 0 {
		return nil, apperr.ErrSandboxToolNotFound
	}
	tools := make([]Tool, 0, len(codes))
	for _, code := range codes {
		tool, err := tx.GetToolByCode(ctx, strings.TrimSpace(code))
		if err != nil {
			return nil, apperr.ErrSandboxToolNotFound.WithCause(err)
		}
		if err := validateToolForRuntime(tool, runtimes); err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

// validateToolForRuntime 统一工具目录、发布校验和真实创建使用的运行时兼容规则。
func validateToolForRuntime(tool Tool, runtimes []RuntimePlan) error {
	if tool.Status != ToolStatusAvailable {
		return apperr.ErrSandboxToolIncompatible
	}
	for _, runtime := range runtimes {
		if err := validateToolNetworkRulesForRuntime(tool, runtime.Runtime.AdapterSpec); err != nil {
			return err
		}
	}
	return nil
}

// createSandboxRecord 计算过期时间和对象存储 key 后创建沙箱主记录。
func (s *Service) createSandboxRecord(ctx context.Context, tx TxStore, req CreateSandboxInputModel, sharedAccountIDs []int64, quota TenantQuota) (Sandbox, error) {
	now := timex.Now()
	id := s.ids.Generate()
	keepAliveUntil := time.Time{}
	if req.KeepAlive {
		keepAliveUntil = now.Add(time.Duration(req.KeepAliveMinutes) * time.Minute)
	}
	snapshotExpireAt := time.Time{}
	if req.SnapshotEnabled {
		snapshotExpireAt = now.Add(time.Duration(req.SnapshotRetentionMinutes) * time.Minute)
	}
	codeKey, err := storage.ObjectKey(req.TenantID, "sandbox", "code", ids.Format(id), "workspace.tar")
	if err != nil {
		return Sandbox{}, apperr.ErrSandboxCreateFailed.WithCause(err)
	}
	snapshot, err := jsonx.AnyBytes(req.CompositionSnapshot, apperr.ErrSandboxCreateFailed)
	if err != nil {
		return Sandbox{}, err
	}
	return tx.CreateSandbox(ctx, CreateSandboxInput{
		ID:                  id,
		TenantID:            req.TenantID,
		Namespace:           namespaceFor(s.cfg.NSPrefixStudent, id),
		SourceRef:           req.SourceRef,
		ScopeRef:            req.ScopeRef,
		CompositionDigest:   req.CompositionSnapshot.Digest,
		CompositionSnapshot: snapshot,
		AccessProfile:       string(req.CompositionSnapshot.Spec.AccessProfile),
		OwnerAccountID:      req.OwnerAccountID,
		SharedAccountIDs:    sharedAccountIDs,
		Phase:               SandboxPhaseAllocating,
		Status:              SandboxStatusCreating,
		KeepAlive:           req.KeepAlive,
		SnapshotEnabled:     req.SnapshotEnabled,
		CodeStorageKey:      codeKey,
		InitCodeRef:         req.InitCodeRef,
		InitScriptRef:       req.InitScriptRef,
		KeepAliveUntil:      keepAliveUntil,
		SnapshotExpireAt:    snapshotExpireAt,
		ExpireAt:            now.Add(time.Duration(quota.MaxLifetimeMin) * time.Minute),
	})
}

// createToolRecords 写入沙箱工具挂载记录。
func (s *Service) createToolRecords(ctx context.Context, tx TxStore, sb Sandbox, tools []Tool) ([]SandboxTool, error) {
	out := make([]SandboxTool, 0, len(tools))
	for _, tool := range tools {
		if tool.Category == "infra" {
			continue
		}
		endpoint := toolEndpoint(sb.ID, tool)
		status := SandboxToolStatusReady
		if tool.Kind == SandboxToolKindWebEmbed || tool.Kind == SandboxToolKindCommand {
			status = SandboxToolStatusStarting
		}
		row, err := tx.CreateSandboxTool(ctx, s.ids.Generate(), sb.TenantID, sb.ID, tool, endpoint, status)
		if err != nil {
			return nil, apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		out = append(out, row)
	}
	return out, nil
}

// toolEndpoint 按工具类型生成前端工作台入口,只有 web-embed 走沙箱反向代理。
func toolEndpoint(sandboxID int64, tool Tool) string {
	switch tool.Kind {
	case SandboxToolKindBuiltin:
		return renderBuiltinToolEndpoint(sandboxID, tool.ResourceSpec)
	case SandboxToolKindTerminal:
		return "/api/v1/sandbox/sandboxes/" + ids.Format(sandboxID) + "/terminal"
	case SandboxToolKindCommand:
		return "/api/v1/sandbox/sandboxes/" + ids.Format(sandboxID) + "/command-tools/" + tool.Code + "/run"
	default:
		return "/api/v1/sandbox/sandboxes/" + ids.Format(sandboxID) + "/tools/" + tool.Code + "/"
	}
}

// renderBuiltinToolEndpoint 渲染平台内置工具端点模板,模板已在注册规则中限定到 sandbox 模块路径。
func renderBuiltinToolEndpoint(sandboxID int64, spec ToolResourceSpec) string {
	return strings.ReplaceAll(strings.TrimSpace(spec.BuiltinEndpoint), "{sandbox_id}", ids.Format(sandboxID))
}

// info 汇总沙箱、运行时、镜像和工具接入信息。
func (s *Service) info(ctx context.Context, tenantID, sandboxID int64) (contracts.SandboxInfo, error) {
	var sb Sandbox
	var runtime Runtime
	var image RuntimeImage
	var tools []SandboxTool
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		tools, err = tx.ListSandboxTools(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return contracts.SandboxInfo{}, err
	}
	var err error
	runtime, image, err = s.workspaceRuntimeForSandbox(ctx, sb)
	if err != nil {
		return contracts.SandboxInfo{}, apperr.ErrSandboxRuntimeNotFound.WithCause(err)
	}
	out := sandboxInfoFromModel(sb, runtime, image, tools)
	out.Capabilities = sandboxCapabilitiesFromSnapshot(sb, runtime, tools, s.capabilities)
	if sb.Status == SandboxStatusCreating {
		// 重启恢复必须从发布时保存的完整组合快照重建计划，不能从租户挂载表反推基础设施。
		plan, err := s.planForExistingSandbox(ctx, sb)
		if err != nil {
			return contracts.SandboxInfo{}, err
		}
		s.scheduleSandboxStart(ctx, plan)
	}
	if s.orchestrator != nil && shouldLoadLiveResourceUsage(sb) {
		usage, err := s.orchestrator.ResourceUsage(ctx, sb)
		if err != nil {
			return contracts.SandboxInfo{}, apperr.ErrSandboxResourceUsageFailed.WithCause(err)
		}
		out.ResourceUsage = usage
	}
	return out, nil
}

// workspaceRuntimeForSandbox 从不可变组合快照解析显式工作区实例,不读取已删除的单 runtime 索引字段。
func (s *Service) workspaceRuntimeForSandbox(ctx context.Context, sb Sandbox) (Runtime, RuntimeImage, error) {
	var snapshot contracts.SandboxCompositionSnapshot
	if err := jsonx.DecodeStrictKnownFields(sb.CompositionSnapshot, &snapshot); err != nil {
		return Runtime{}, RuntimeImage{}, err
	}
	if _, err := contracts.CanonicalSnapshotDigest(snapshot); err != nil || snapshot.Digest != sb.CompositionDigest {
		return Runtime{}, RuntimeImage{}, fmt.Errorf("沙箱组合快照无效")
	}
	var frozen contracts.CompiledRuntimeSnapshot
	found := false
	for _, item := range snapshot.Runtimes {
		if item.InstanceCode == snapshot.Spec.WorkspaceRuntimeInstance {
			if found {
				return Runtime{}, RuntimeImage{}, fmt.Errorf("工作区运行时实例重复")
			}
			frozen = item
			found = true
		}
	}
	if !found {
		return Runtime{}, RuntimeImage{}, fmt.Errorf("工作区运行时实例未包含在快照中")
	}
	var runtime Runtime
	var image RuntimeImage
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		runtime, err = tx.GetRuntimeByID(ctx, frozen.RuntimeID)
		if err != nil {
			return err
		}
		image, err = tx.GetRuntimeImageByID(ctx, frozen.RuntimeID, frozen.ImageID)
		return err
	}); err != nil {
		return Runtime{}, RuntimeImage{}, err
	}
	if runtime.Code != frozen.Code || runtime.Status != RuntimeStatusAvailable || runtime.SelftestStatus != RuntimeSelftestPassed || image.Version != frozen.ImageVersion || image.ImageURL != frozen.ImageURL || !image.GenesisBaked {
		return Runtime{}, RuntimeImage{}, fmt.Errorf("工作区运行时当前不可用")
	}
	if err := jsonx.DecodeStrictKnownFields(frozen.AdapterSpec, &runtime.AdapterSpec); err != nil {
		return Runtime{}, RuntimeImage{}, err
	}
	runtime.AdapterLevel = frozen.AdapterLevel
	runtime.CapabilityImpl = frozen.CapabilityImpl
	return runtime, image, nil
}

// shouldLoadLiveResourceUsage 判断当前状态是否存在稳定计算资源,避免创建/回收阶段把 metrics 暂无数据当成业务失败。
func shouldLoadLiveResourceUsage(sb Sandbox) bool {
	return sb.Status == SandboxStatusReady || sb.Status == SandboxStatusRunning || sb.Status == SandboxStatusPaused || sb.Status == SandboxStatusIdle
}

// transition 执行简单状态流转并写入审计。
func (s *Service) transition(ctx context.Context, tenantID, sandboxID int64, phase, status int16, action string) error {
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		sb, err := tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if err := validateStateTransition(sb.Status, status); err != nil {
			return err
		}
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, tenantID, sandboxID, phase, status); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		detail, err := jsonBytes(map[string]any{"status": status, "phase": phase})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), tenantID, sandboxID, EventTypePhaseChange, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	s.broadcastProgress(ctx, tenantID, sandboxID, phase, status, response.TraceFromContext(ctx))
	return s.writeSystemAudit(ctx, tenantID, action, "sandbox", sandboxID, nil)
}

// startAsync 提交异步启动任务,请求返回后继续推进 K8s 创建和阶段变化。
func (s *Service) startAsync(ctx context.Context, plan CreateSandboxPlan) {
	s.scheduleSandboxStart(ctx, plan)
}

// scheduleSandboxStart 保证每个沙箱只有一个启动任务,并允许服务重启后从持久化状态恢复未完成启动。
func (s *Service) scheduleSandboxStart(ctx context.Context, plan CreateSandboxPlan) {
	s.startupMu.Lock()
	if s.startup == nil {
		s.startup = map[int64]struct{}{}
	}
	if _, exists := s.startup[plan.Sandbox.ID]; exists {
		s.startupMu.Unlock()
		return
	}
	s.startup[plan.Sandbox.ID] = struct{}{}
	s.startupMu.Unlock()
	traceCtx := logging.WithAttrs(context.WithoutCancel(ctx), logging.AttrsFromContext(ctx)...)
	go func() {
		defer func() {
			s.startupMu.Lock()
			delete(s.startup, plan.Sandbox.ID)
			s.startupMu.Unlock()
		}()
		s.startSandbox(traceCtx, plan)
	}()
}

// startSandbox 推进 K8s 编排,阶段失败时写 error 并保留可排查事件。
func (s *Service) startSandbox(ctx context.Context, plan CreateSandboxPlan) {
	ctx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	resourcesReady := false
	if checker, ok := s.orchestrator.(sandboxResourceReadiness); ok {
		ready, err := checker.SandboxResourcesReady(ctx, plan)
		if err != nil {
			slog.WarnContext(ctx, "sandbox resource readiness probe failed; falling back to idempotent create", slog.String("error", logging.SanitizeError(err.Error())), slog.Int64("tenant_id", plan.Sandbox.TenantID), slog.Int64("sandbox_id", plan.Sandbox.ID))
		} else {
			resourcesReady = ready
		}
	}
	if !resourcesReady {
		if err := s.orchestrator.CreateSandboxResources(ctx, plan); err != nil {
			// 服务重启可能留下已经就绪的 K8s 资源,但原进程内协程已经消失;
			// 标记沙箱失败前必须再次探测资源状态。
			if checker, ok := s.orchestrator.(sandboxResourceReadiness); ok {
				ready, readinessErr := checker.SandboxResourcesReady(ctx, plan)
				if readinessErr == nil && ready {
					resourcesReady = true
				} else {
					s.cleanupAfterStartFailure(ctx, plan.Sandbox)
					s.markStartFailed(ctx, plan.Sandbox, err)
					return
				}
			} else {
				s.cleanupAfterStartFailure(ctx, plan.Sandbox)
				s.markStartFailed(ctx, plan.Sandbox, err)
				return
			}
		}
	}
	advanced, err := s.advanceStartupState(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID, SandboxPhaseReady, SandboxStatusReady, map[string]any{"phase": SandboxPhaseReady, "status": SandboxStatusReady}, SandboxStatusCreating)
	if err != nil {
		logging.ErrorContext(ctx, "sandbox phase update failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", plan.Sandbox.TenantID), slog.Int64("sandbox_id", plan.Sandbox.ID))
		return
	}
	if !advanced {
		return
	}
	s.broadcastProgress(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID, SandboxPhaseReady, SandboxStatusReady, response.TraceFromContext(ctx))
	go s.updateToolsAsync(ctx, plan)
	if sandboxNeedsInitialization(plan) {
		advanced, err = s.advanceStartupState(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID, SandboxPhaseInitializing, SandboxStatusRunning, map[string]any{"phase": SandboxPhaseInitializing}, SandboxStatusReady)
		if err != nil {
			logging.ErrorContext(ctx, "sandbox init phase update failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", plan.Sandbox.TenantID), slog.Int64("sandbox_id", plan.Sandbox.ID))
			return
		}
		if !advanced {
			return
		}
		s.broadcastProgress(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID, SandboxPhaseInitializing, SandboxStatusRunning, response.TraceFromContext(ctx))
		if err := s.applyInitAssetsIfNeeded(ctx, plan.Sandbox, workspaceRuntimeForPlan(plan)); err != nil {
			s.markInitFailed(ctx, plan.Sandbox, err)
			return
		}
		if strings.TrimSpace(plan.Sandbox.InitCodeRef) != "" {
			if err := s.restoreInitCodeIfNeeded(ctx, plan.Sandbox, workspaceRuntimeForPlan(plan), plan.Sandbox.InitCodeRef); err != nil {
				s.markInitFailed(ctx, plan.Sandbox, err)
				return
			}
		}
		if strings.TrimSpace(plan.Sandbox.InitScriptRef) != "" {
			if err := s.runInitScriptIfNeeded(ctx, plan.Sandbox, workspaceRuntimeForPlan(plan), plan.Sandbox.InitScriptRef); err != nil {
				s.markInitFailed(ctx, plan.Sandbox, err)
				return
			}
		}
		advanced, err = s.advanceStartupState(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID, SandboxPhaseFullyReady, SandboxStatusRunning, map[string]any{"phase": SandboxPhaseFullyReady}, SandboxStatusRunning)
		if err != nil {
			logging.ErrorContext(ctx, "sandbox init phase update failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", plan.Sandbox.TenantID), slog.Int64("sandbox_id", plan.Sandbox.ID))
			return
		}
		if !advanced {
			return
		}
		s.broadcastProgress(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID, SandboxPhaseFullyReady, SandboxStatusRunning, response.TraceFromContext(ctx))
	}
}

// updateToolsAsync 在环境就绪后独立推进 Web 工具状态,不让慢启动工具阻塞 phase=2。
func (s *Service) updateToolsAsync(ctx context.Context, plan CreateSandboxPlan) {
	base := logging.WithAttrs(context.WithoutCancel(ctx), logging.AttrsFromContext(ctx)...)
	toolCtx, cancel := context.WithTimeout(base, timeDurationSeconds(int(toolReadinessTimeoutForPlan(s.cfg, plan))))
	defer cancel()
	if err := s.updateToolReadiness(toolCtx, plan); err != nil {
		logging.ErrorContext(toolCtx, "sandbox tool readiness update failed", err.Error(), slog.Int64("tenant_id", plan.Sandbox.TenantID), slog.Int64("sandbox_id", plan.Sandbox.ID))
	}
}

// toolReadinessTimeoutForPlan 以工具声明式 readiness 窗口作为工具状态更新等待下限。
func toolReadinessTimeoutForPlan(cfg config.SandboxConfig, plan CreateSandboxPlan) int64 {
	timeout := int64(cfg.ReadyTimeoutSeconds)
	for _, tool := range plan.Tools {
		if tool.Kind != SandboxToolKindWebEmbed && tool.Kind != SandboxToolKindCommand {
			continue
		}
		for _, component := range tool.ResourceSpec.Components {
			timeout = max(timeout, readinessWindowSeconds(cfg, component.ReadinessProbe))
		}
	}
	if timeout <= 0 {
		return 1
	}
	return timeout
}

// readinessWindowSeconds 按 K8s 探针周期和失败阈值计算声明的最大 readiness 等待秒数。
func readinessWindowSeconds(cfg config.SandboxConfig, probe workload.ProbeSpec) int64 {
	if strings.TrimSpace(probe.Type) == "" {
		return 0
	}
	period := probe.PeriodSeconds
	if period <= 0 {
		period = cfg.ProbeDefaultPeriodSeconds
	}
	threshold := probe.FailureThreshold
	if threshold <= 0 {
		threshold = cfg.ProbeDefaultFailureThreshold
	}
	return int64(period) * int64(threshold)
}

// advanceStartupState 只允许异步启动任务从预期状态推进;若回收已接管则停止本次启动链路。
func (s *Service) advanceStartupState(ctx context.Context, tenantID, sandboxID int64, phase, status int16, detail map[string]any, expectedStatuses ...int16) (bool, error) {
	advanced := false
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		sb, err := tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if startupStateOwnedByRecycle(sb.Status) {
			return nil
		}
		if !statusIn(sb.Status, expectedStatuses...) {
			return apperr.ErrSandboxStateInvalid
		}
		if err := validateStateTransition(sb.Status, status); err != nil {
			return err
		}
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, tenantID, sandboxID, phase, status); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		rawDetail, err := jsonBytes(detail)
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), tenantID, sandboxID, EventTypePhaseChange, rawDetail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		advanced = true
		return nil
	}); err != nil {
		return false, err
	}
	return advanced, nil
}

// startupStateOwnedByRecycle 判断沙箱状态是否已由回收链路接管,异步启动不得再写入。
func startupStateOwnedByRecycle(status int16) bool {
	return status == SandboxStatusRecycling || status == SandboxStatusDestroyed
}

// statusIn 判断当前状态是否处在异步启动允许推进的前置状态集合中。
func statusIn(status int16, candidates ...int16) bool {
	for _, candidate := range candidates {
		if status == candidate {
			return true
		}
	}
	return false
}

// sandboxNeedsInitialization 判断沙箱是否存在个性化资产、代码或脚本需要异步执行。
func sandboxNeedsInitialization(plan CreateSandboxPlan) bool {
	for _, runtime := range plan.Runtimes {
		if len(runtime.Runtime.AdapterSpec.InitAssets) > 0 {
			return true
		}
	}
	return strings.TrimSpace(plan.Sandbox.InitCodeRef) != "" || strings.TrimSpace(plan.Sandbox.InitScriptRef) != ""
}

func workspaceRuntimeForPlan(plan CreateSandboxPlan) Runtime {
	runtime, ok := plan.WorkspaceRuntime(plan.WorkspaceRuntimeInstance)
	if !ok {
		return Runtime{}
	}
	return runtime.Runtime
}

// cleanupAfterStartFailure 在阶段一创建失败后用独立有界上下文清理可能已创建的 K8s 资源。
func (s *Service) cleanupAfterStartFailure(ctx context.Context, sb Sandbox) {
	cleanupBase := logging.WithAttrs(context.WithoutCancel(ctx), logging.AttrsFromContext(ctx)...)
	cleanupCtx, cancel := context.WithTimeout(cleanupBase, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	if err := s.orchestrator.DestroySandboxResources(cleanupCtx, sb); err != nil {
		logging.ErrorContext(ctx, "sandbox start cleanup failed", err.Error(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID), slog.String("namespace", sb.Namespace))
	}
}

// cleanupAfterResumeFailure 释放恢复失败时半启动的计算 Pod,但保留快照命名空间与 PVC,使沙箱仍可再次恢复或由回收链路兜底。
func (s *Service) cleanupAfterResumeFailure(ctx context.Context, sb Sandbox) {
	cleanupBase := logging.WithAttrs(context.WithoutCancel(ctx), logging.AttrsFromContext(ctx)...)
	cleanupCtx, cancel := context.WithTimeout(cleanupBase, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	if err := s.orchestrator.StopComputeKeepSnapshot(cleanupCtx, sb); err != nil {
		logging.ErrorContext(ctx, "sandbox resume cleanup failed", err.Error(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID), slog.String("namespace", sb.Namespace))
	}
}

// markStartFailed 记录启动失败,并避免把未完成资源伪装成 ready。
func (s *Service) markStartFailed(ctx context.Context, sb Sandbox, cause error) {
	persistCtx, cancel := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	shouldBroadcast := false
	broadcastPhase := sb.Phase
	if err := s.store.TenantTx(persistCtx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetSandbox(ctx, sb.TenantID, sb.ID)
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if startupStateOwnedByRecycle(current.Status) {
			return nil
		}
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, sb.TenantID, sb.ID, current.Phase, SandboxStatusFailed); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		broadcastPhase = current.Phase
		detail, err := jsonBytes(map[string]any{"stage": "start", "error": logging.SanitizeError(cause.Error())})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeError, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		shouldBroadcast = true
		return nil
	}); err != nil {
		logging.ErrorContext(persistCtx, "sandbox start failure mark failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID))
	}
	if shouldBroadcast {
		s.broadcastProgress(ctx, sb.TenantID, sb.ID, broadcastPhase, SandboxStatusFailed, response.TraceFromContext(ctx))
	}
	logging.ErrorContext(ctx, "sandbox start failed", apperr.AsAppError(cause).LogString(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID))
}

// markInitFailed 记录阶段二个性化初始化失败,保留阶段一可进入状态供用户继续查看和修复。
func (s *Service) markInitFailed(ctx context.Context, sb Sandbox, cause error) {
	persistCtx, cancel := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	shouldBroadcast := false
	if err := s.store.TenantTx(persistCtx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetSandbox(ctx, sb.TenantID, sb.ID)
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if startupStateOwnedByRecycle(current.Status) {
			return nil
		}
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, sb.TenantID, sb.ID, SandboxPhaseInitializing, SandboxStatusRunning); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		detail, err := jsonBytes(map[string]any{"stage": "init", "error": logging.SanitizeError(cause.Error())})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeError, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		shouldBroadcast = true
		return nil
	}); err != nil {
		logging.ErrorContext(persistCtx, "sandbox init failure mark failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID))
	}
	if shouldBroadcast {
		s.broadcastProgress(ctx, sb.TenantID, sb.ID, SandboxPhaseInitializing, SandboxStatusRunning, response.TraceFromContext(ctx))
	}
	logging.ErrorContext(ctx, "sandbox init failed", apperr.AsAppError(cause).LogString(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID))
}

// updateToolReadiness 将工具真实健康检查结果写回控制面,避免未就绪工具被访问。
func (s *Service) updateToolReadiness(ctx context.Context, plan CreateSandboxPlan) error {
	return runToolReadinessChecks(ctx, plan.Sandbox, plan.Tools, s.waitToolReady, s.persistToolStatus)
}

type toolReadinessWaitFunc func(context.Context, Sandbox, Tool) error
type toolReadinessPersistFunc func(context.Context, Sandbox, Tool, int16) error

// runToolReadinessChecks 并行推进工具就绪检查,避免某个慢工具阻塞其他工具入口解灰。
func runToolReadinessChecks(ctx context.Context, sb Sandbox, tools []Tool, wait toolReadinessWaitFunc, persist toolReadinessPersistFunc) error {
	errs := make(chan error, len(tools))
	var wg sync.WaitGroup
	for _, tool := range tools {
		if tool.Kind != SandboxToolKindWebEmbed && tool.Kind != SandboxToolKindCommand {
			continue
		}
		tool := tool
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := wait(ctx, sb, tool); err != nil {
				if persistErr := persist(ctx, sb, tool, SandboxToolStatusFailed); persistErr != nil {
					errs <- persistErr
					return
				}
				logging.ErrorContext(ctx, "sandbox tool readiness failed", err.Error(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID), slog.String("tool_code", tool.Code))
				errs <- apperr.ErrSandboxToolProxyUnavailable.WithCause(err)
				return
			}
			if err := persist(ctx, sb, tool, SandboxToolStatusReady); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// waitToolReady 按配置轮询工具就绪状态,避免慢启动工具被一次探测永久判失败。
func (s *Service) waitToolReady(ctx context.Context, sb Sandbox, tool Tool) error {
	interval := time.Duration(s.cfg.ReadyPollIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Second
	}
	var lastErr error
	for {
		if err := s.orchestrator.ToolReady(ctx, sb, tool); err != nil {
			lastErr = err
		} else {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %v", ctx.Err(), lastErr)
		case <-time.After(interval):
		}
	}
}

// persistToolStatus 写入单个工具状态和统一代理端点。
func (s *Service) persistToolStatus(ctx context.Context, sb Sandbox, tool Tool, status int16) error {
	endpoint := toolEndpoint(sb.ID, tool)
	persistCtx, cancel := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	if err := s.store.TenantTx(persistCtx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.UpdateSandboxToolStatus(ctx, sb.TenantID, sb.ID, tool, endpoint, status); err != nil {
			return apperr.ErrSandboxToolPersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// asyncPersistenceContext 为异步状态回写创建独立有界上下文,避免探测 deadline 取消数据库写入。
func asyncPersistenceContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = time.Second
	}
	base := logging.WithAttrs(context.WithoutCancel(ctx), logging.AttrsFromContext(ctx)...)
	return context.WithTimeout(base, timeout)
}

// selectRuntimeImage 按组合声明固定的镜像版本选择已登记镜像。
func selectRuntimeImage(ctx context.Context, tx TxStore, runtimeID int64, version string) (RuntimeImage, error) {
	if strings.TrimSpace(version) == "" {
		return RuntimeImage{}, apperr.ErrSandboxRuntimeImageNotFound
	}
	image, err := tx.GetRuntimeImageByVersionForShare(ctx, runtimeID, strings.TrimSpace(version))
	if err != nil {
		return RuntimeImage{}, apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
	}
	return image, nil
}

// namespaceFor 根据配置前缀和沙箱 ID 生成动态命名空间。
func namespaceFor(prefix string, id int64) string {
	return strings.Trim(prefix, "-") + "-" + ids.Format(id)
}
