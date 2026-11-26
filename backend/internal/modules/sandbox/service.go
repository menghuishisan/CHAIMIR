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
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/limitio"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"
	"chaimir/pkg/terminal"
)

// Orchestrator 定义 M2 service 需要的 K8s 编排能力。
type Orchestrator interface {
	// CreateSandboxResources 创建 Namespace、资源限制、默认拒绝网络、PVC、Pod 和工具 Service。
	CreateSandboxResources(ctx context.Context, plan CreateSandboxPlan, archives []RuntimeInitArchive) error
	// DestroySandboxResources 删除普通沙箱资源。
	DestroySandboxResources(ctx context.Context, sb Sandbox) error
	// CleanupExpiredRuntimeSelftests 补偿请求内清理失败的无持久化运行时自检资源。
	CleanupExpiredRuntimeSelftests(ctx context.Context) error
	// StopComputeKeepSnapshot 释放计算工作负载但保留快照命名空间。
	StopComputeKeepSnapshot(ctx context.Context, sb Sandbox) error
	// CreateSnapshot 创建 CSI VolumeSnapshot 并返回 namespaced 引用与实际覆盖卷域。
	CreateSnapshot(ctx context.Context, plan CreateSandboxPlan, retention time.Duration) (SnapshotResult, error)
	// ResourceUsage 汇总沙箱当前已申请资源,用于状态查询返回资源用量。
	ResourceUsage(ctx context.Context, sb Sandbox) (contracts.SandboxResourceUsage, error)
	// PrepareWorkspaceArchive 停止计算写入，仅挂载原工作区卷提供最终归档。
	PrepareWorkspaceArchive(ctx context.Context, plan CreateSandboxPlan) error
	// StopSandboxCompute 正常删除全部计算 Pod 并等待退出，保留卷与运行配置。
	StopSandboxCompute(ctx context.Context, sb Sandbox) error
	// Exec 在沙箱容器中执行受控命令。
	Exec(ctx context.Context, namespace, container string, command []string, stdin []byte, tty bool) ([]byte, []byte, error)
	// ExecStream 在沙箱容器中执行交互式命令并透传流。
	ExecStream(ctx context.Context, namespace, container string, command []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, tty bool, sizes terminal.SizeQueue) error
	// ResetSandboxRuntime 清空目标账本并重注入材料，保留工作区和其他实例数据。
	ResetSandboxRuntime(ctx context.Context, plan CreateSandboxPlan, runtimeInstance string, archives []RuntimeInitArchive) error
	// PrepullImage 创建或更新预拉取 DaemonSet 并等待工作负载镜像集合在真实节点 Ready。
	PrepullImage(ctx context.Context, image RuntimeImage, compositionDigest string, specs []PrepullImageSpec) (PrepullResult, error)
	// DeletePrepullDaemonSet 删除镜像预拉取 DaemonSet,用于镜像停用或删除闭环。
	DeletePrepullDaemonSet(ctx context.Context, image RuntimeImage) error
	// ToolReady 校验 Web 工具容器已达到可代理状态。
	ToolReady(ctx context.Context, sb Sandbox, tool Tool) error
	// SnapshotSupported 返回当前集群是否安装并启用 CSI 快照能力。
	SnapshotSupported(ctx context.Context) (bool, error)
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
	CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) (int64, error)
	Delete(ctx context.Context, bucket, key string) error
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	BucketCode() string
	BucketAttach() string
}

// Service 承载 sandbox 模块业务编排,依赖 repo 接口和平台横切能力。
type Service struct {
	store         Store
	ids           snowflake.Generator
	cfg           config.SandboxConfig
	objectStorage objectStorage
	orchestrator  Orchestrator
	audit         audit.Writer
	identity      contracts.IdentityService
	bus           eventbus.Bus
	wsHub         *ws.Hub
	capabilities  map[string]ChainCapability
	saveMu        sync.Mutex
	saveTimers    map[int64]*time.Timer
}

// resolvedSandboxCreateDependencies 是创建或发布前校验沙箱模板时解析出的 M2 能力快照。
type resolvedSandboxCreateDependencies struct {
	Plan  CreateSandboxPlan
	Quota TenantQuota
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
	capabilities[BuiltinExecCapability] = execChainCapability{orchestrator: deps.Orchestrator}
	return &Service{
		store:         deps.Store,
		ids:           deps.IDs,
		cfg:           deps.Config,
		objectStorage: deps.Storage,
		orchestrator:  deps.Orchestrator,
		audit:         deps.Audit,
		identity:      deps.Identity,
		bus:           deps.EventBus,
		wsHub:         deps.WSHub,
		capabilities:  capabilities,
		saveTimers:    map[int64]*time.Timer{},
	}, nil
}

// ValidateSandboxTemplate 校验教师/业务模块声明的实验环境能解析到当前可调度的运行时、镜像和工具。
func (s *Service) ValidateSandboxTemplate(ctx context.Context, req contracts.SandboxCreateRequest) error {
	input := normalizeSandboxCreateRequest(req)
	if err := validateCreateRequest(input); err != nil {
		return err
	}
	digest, err := contracts.CanonicalSnapshotDigest(req.CompositionSnapshot)
	if err != nil || digest != req.CompositionSnapshot.Digest {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	if err := s.validateSnapshotCapability(ctx, input.SnapshotEnabled); err != nil {
		return err
	}
	return s.store.TenantTx(ctx, input.TenantID, func(ctx context.Context, tx TxStore) error {
		_, err := s.resolveSandboxCreateDependencies(ctx, tx, input, false)
		return err
	})
}

// validateSnapshotCapability 在数据库事务开始前检查当前集群的快照能力。
func (s *Service) validateSnapshotCapability(ctx context.Context, enabled bool) error {
	if !enabled {
		return nil
	}
	supported, err := s.orchestrator.SnapshotSupported(ctx)
	if err != nil {
		return apperr.ErrSandboxSnapshotUnavailable.WithCause(err)
	}
	if !supported {
		return apperr.ErrSandboxSnapshotUnavailable
	}
	return nil
}

// CreateSandbox 创建沙箱控制面记录并异步推进 K8s 启动。
func (s *Service) CreateSandbox(ctx context.Context, req contracts.SandboxCreateRequest) (contracts.SandboxInfo, error) {
	digest, err := contracts.CanonicalSnapshotDigest(req.CompositionSnapshot)
	if err != nil || strings.TrimSpace(req.CompositionSnapshot.Digest) == "" || req.CompositionSnapshot.Digest != digest {
		return contracts.SandboxInfo{}, apperr.ErrSandboxCreateRequestInvalid
	}
	input := normalizeSandboxCreateRequest(req)
	if err := validateCreateRequest(input); err != nil {
		return contracts.SandboxInfo{}, err
	}
	// 用户模块开关在业务入口校验；本契约也承接已受理判题和对局的异步资源编排。
	sharedAccountIDs, err := normalizeSandboxSharedAccountIDs(input.OwnerAccountID, input.AuthorizedAccountIDs)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	if err := s.validateSandboxSharedAccounts(ctx, input.TenantID, sharedAccountIDs); err != nil {
		return contracts.SandboxInfo{}, err
	}
	idempotentSource := isBattleMatchSourceRef(input.SourceRef) || input.SourceRef == input.ScopeRef
	if input.SnapshotEnabled && idempotentSource {
		var items []Sandbox
		if err := s.store.TenantTx(ctx, input.TenantID, func(ctx context.Context, tx TxStore) error {
			var err error
			items, err = tx.ListSandboxesByScopeRef(ctx, input.TenantID, input.ScopeRef)
			return err
		}); err != nil {
			return contracts.SandboxInfo{}, apperr.ErrSandboxCreateFailed.WithCause(err)
		}
		if len(items) > 0 {
			return s.GetSandbox(ctx, input.TenantID, items[0].ID)
		}
	}
	if err := s.validateSnapshotCapability(ctx, input.SnapshotEnabled); err != nil {
		return contracts.SandboxInfo{}, err
	}
	var plan CreateSandboxPlan
	var accepted contracts.SandboxInfo
	var existingID int64
	if err := s.store.TenantTx(ctx, input.TenantID, func(ctx context.Context, tx TxStore) error {
		if idempotentSource {
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
		if err := requireCompositionPrepull(ctx, tx, resolved.Plan.Runtimes, input.CompositionSnapshot.Digest); err != nil {
			return err
		}
		sb, err := s.createSandboxRecord(ctx, tx, input, sharedAccountIDs, resolved.Quota)
		if err != nil {
			return err
		}
		detail, err := jsonBytes(map[string]any{
			"runtime_instances": runtimePlanAuditCodes(resolved.Plan.Runtimes),
			"source_ref":        input.SourceRef,
		})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), input.TenantID, sb.ID, EventTypeCreate, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		plan = resolved.Plan
		plan.Sandbox = sb
		accepted, err = s.sandboxInfo(ctx, sb)
		if err != nil {
			return err
		}
		return s.writeSystemAudit(ctx, input.TenantID, "sandbox.create", "sandbox", sb.ID, map[string]any{"source_ref": input.SourceRef})
	}); err != nil {
		if idempotentSource && errors.Is(err, apperr.ErrConflict) {
			return s.infoBySourceRef(ctx, input.TenantID, input.SourceRef)
		}
		return contracts.SandboxInfo{}, err
	}
	if existingID > 0 {
		return s.GetSandbox(ctx, input.TenantID, existingID)
	}
	s.scheduleSandboxStart(ctx, plan)
	s.broadcastProgress(ctx, input.TenantID, plan.Sandbox.ID, SandboxPhaseAllocating, SandboxStatusCreating, response.TraceFromContext(ctx))
	return accepted, nil
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

// UpdateSandboxAuthorizedAccounts 原子替换沙箱共享账号集合。
func (s *Service) UpdateSandboxAuthorizedAccounts(ctx context.Context, req contracts.SandboxAuthorizedAccountsRequest) error {
	if req.TenantID <= 0 || req.SandboxID <= 0 || strings.TrimSpace(req.SourceRef) == "" {
		return apperr.ErrSandboxContractRequestInvalid
	}
	var revoked []int64
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		sb, err := tx.GetSandboxForUpdate(ctx, req.TenantID, req.SandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if sb.SourceRef != strings.TrimSpace(req.SourceRef) {
			return apperr.ErrSandboxOwnershipInvalid
		}
		accountIDs, err := normalizeSandboxSharedAccountIDs(sb.OwnerAccountID, req.AuthorizedAccountIDs)
		if err != nil {
			return err
		}
		if err := s.validateSandboxSharedAccounts(ctx, req.TenantID, accountIDs); err != nil {
			return err
		}
		next := make(map[int64]struct{}, len(accountIDs))
		for _, accountID := range accountIDs {
			next[accountID] = struct{}{}
		}
		for _, accountID := range sb.SharedAccountIDs {
			if _, ok := next[accountID]; !ok {
				revoked = append(revoked, accountID)
			}
		}
		_, err = tx.UpdateSandboxAuthorizedAccounts(ctx, req.TenantID, req.SandboxID, accountIDs)
		return err
	}); err != nil {
		return err
	}
	var closeErrs []error
	if err := publishSandboxSessionsRevoked(ctx, s.bus, req.TenantID, req.SandboxID, revoked); err != nil {
		closeErrs = append(closeErrs, fmt.Errorf("广播沙箱成员连接撤权失败: %w", err))
	}
	for _, accountID := range revoked {
		if err := s.closeSandboxSessions(req.TenantID, accountID, req.SandboxID); err != nil {
			closeErrs = append(closeErrs, fmt.Errorf("关闭已撤销沙箱成员连接失败: tenant_id=%d sandbox_id=%d account_id=%d: %w", req.TenantID, req.SandboxID, accountID, err))
		}
	}
	if err := errors.Join(closeErrs...); err != nil {
		return apperr.ErrSandboxToolProxyUnavailable.WithCause(err)
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
	return s.GetSandbox(ctx, tenantID, items[0].ID)
}

// resolveSandboxCreateDependencies 用同一套规则服务发布前校验和真实创建,避免 M7 与 M2 对运行时/工具可用性判断分叉。
func (s *Service) resolveSandboxCreateDependencies(ctx context.Context, tx TxStore, input contracts.SandboxCreateRequest, enforceLiveCapacity bool) (resolvedSandboxCreateDependencies, error) {
	plan, err := s.loadSnapshotPlan(ctx, tx, input.CompositionSnapshot, true)
	if err != nil {
		return resolvedSandboxCreateDependencies{}, err
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
	plan.Sandbox.SnapshotEnabled = input.SnapshotEnabled
	if err := validateQuotaForCreate(input, quota, active, s.cfg, plan); err != nil {
		return resolvedSandboxCreateDependencies{}, err
	}
	return resolvedSandboxCreateDependencies{Plan: plan, Quota: quota}, nil
}

func requireCompositionPrepull(ctx context.Context, tx TxStore, runtimes []RuntimePlan, digest string) error {
	for _, runtime := range runtimes {
		prepull, err := tx.GetCompositionPrepullForUpdate(ctx, runtime.Image.ID, digest)
		if err != nil {
			return apperr.ErrSandboxRuntimeUnavailable.WithCause(fmt.Errorf("读取创建组合预拉取证明失败: runtime_id=%d image_id=%d composition_digest=%s: %w", runtime.Runtime.ID, runtime.Image.ID, digest, err))
		}
		if prepull.Status != ImagePrepullSucceeded {
			return apperr.ErrSandboxRuntimeUnavailable.WithCause(fmt.Errorf("创建组合预拉取尚未通过: runtime_id=%d image_id=%d composition_digest=%s status=%d", runtime.Runtime.ID, runtime.Image.ID, digest, prepull.Status))
		}
	}
	return nil
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

// runtimePlanAuditCodes 把运行时计划写成「实例编码:运行时编码」列表,审计记录这次创建用了哪些运行时。
func runtimePlanAuditCodes(plans []RuntimePlan) []string {
	out := make([]string, 0, len(plans))
	for _, plan := range plans {
		out = append(out, plan.InstanceCode+":"+plan.Runtime.Code)
	}
	return out
}

// GetSandboxBySourceRef 按稳定来源引用读取仍未销毁的沙箱,用于业务页面刷新后的状态恢复。
func (s *Service) GetSandboxBySourceRef(ctx context.Context, tenantID int64, sourceRef string) (contracts.SandboxInfo, error) {
	if tenantID <= 0 || strings.TrimSpace(sourceRef) == "" {
		return contracts.SandboxInfo{}, apperr.ErrSandboxNotFound
	}
	var items []Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListSandboxesBySourceRef(ctx, tenantID, strings.TrimSpace(sourceRef))
		return err
	}); err != nil {
		return contracts.SandboxInfo{}, apperr.ErrSandboxNotFound.WithCause(err)
	}
	if len(items) == 0 {
		return contracts.SandboxInfo{}, apperr.ErrSandboxNotFound
	}
	if len(items) != 1 {
		return contracts.SandboxInfo{}, apperr.ErrSandboxCreateFailed
	}
	return s.GetSandbox(ctx, tenantID, items[0].ID)
}

// GetSandboxForOwner 查询用户自己的沙箱,防止同租户内横向访问。
func (s *Service) GetSandboxForOwner(ctx context.Context, tenantID, accountID, sandboxID int64) (contracts.SandboxInfo, error) {
	if _, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID); err != nil {
		return contracts.SandboxInfo{}, err
	}
	info, err := s.GetSandbox(ctx, tenantID, sandboxID)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	return info, nil
}

func (s *Service) planForExistingSandbox(ctx context.Context, sb Sandbox) (CreateSandboxPlan, error) {
	snapshot, err := verifiedCompositionSnapshot(sb)
	if err != nil {
		return CreateSandboxPlan{}, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	plan, err := s.planForSnapshot(ctx, snapshot)
	if err != nil {
		return CreateSandboxPlan{}, err
	}
	plan.Sandbox = sb
	return plan, nil
}

// planForSnapshot 按冻结规格装载完整工作负载，当前目录只负责身份与撤销门禁。
func (s *Service) planForSnapshot(ctx context.Context, snapshot contracts.SandboxCompositionSnapshot) (CreateSandboxPlan, error) {
	var plan CreateSandboxPlan
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		plan, err = s.loadPreparedSnapshotPlan(ctx, tx, snapshot)
		return err
	}); err != nil {
		return CreateSandboxPlan{}, err
	}
	return plan, nil
}

func (s *Service) loadPreparedSnapshotPlan(ctx context.Context, tx TxStore, snapshot contracts.SandboxCompositionSnapshot) (CreateSandboxPlan, error) {
	plan, err := s.loadSnapshotPlan(ctx, tx, snapshot, true)
	if err != nil {
		return CreateSandboxPlan{}, err
	}
	if err := requireCompositionPrepull(ctx, tx, plan.Runtimes, snapshot.Digest); err != nil {
		return CreateSandboxPlan{}, err
	}
	return plan, nil
}

// 预拉取允许接入中状态；创建、恢复和平台能力自检还须通过运行时接入自检。
func (s *Service) loadSnapshotPlan(ctx context.Context, tx TxStore, snapshot contracts.SandboxCompositionSnapshot, requireSelftest bool) (CreateSandboxPlan, error) {
	runtimePlans := make([]RuntimePlan, 0, len(snapshot.Runtimes))
	runtimeAdapters := make(map[string]AdapterSpec, len(snapshot.Runtimes))
	for _, frozen := range snapshot.Runtimes {
		runtime, err := tx.GetRuntimeByID(ctx, frozen.RuntimeID)
		if err != nil {
			return CreateSandboxPlan{}, apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		if runtime.Code != frozen.Code || runtime.PluginRef != frozen.PluginRef || !canPrepareRuntime(runtime.Status) ||
			(requireSelftest && (runtime.Status != RuntimeStatusAvailable || runtime.SelftestStatus != RuntimeSelftestPassed)) {
			return CreateSandboxPlan{}, apperr.ErrSandboxRuntimeUnavailable.WithCause(fmt.Errorf("冻结运行时门禁拒绝: runtime_id=%d instance=%s code=%s frozen_code=%s status=%d selftest_status=%d", runtime.ID, frozen.InstanceCode, runtime.Code, frozen.Code, runtime.Status, runtime.SelftestStatus))
		}
		var adapter AdapterSpec
		if err := jsonx.DecodeStrictKnownFields(frozen.AdapterSpec, &adapter); err != nil {
			return CreateSandboxPlan{}, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
		}
		if _, err := initScriptExecTarget(adapter); err != nil {
			return CreateSandboxPlan{}, apperr.ErrSandboxRuntimeUnavailable.WithCause(err)
		}
		runtime.AdapterSpec = MaterializeRuntimeTopology(frozen.InstanceCode, adapter)
		runtime.AdapterLevel, runtime.CapabilityImpl, runtime.PluginRef = frozen.AdapterLevel, frozen.CapabilityImpl, frozen.PluginRef
		image, err := tx.GetRuntimeImageByID(ctx, frozen.RuntimeID, frozen.ImageID)
		if err != nil {
			return CreateSandboxPlan{}, apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		if image.RuntimeID != frozen.RuntimeID || image.ImageURL != frozen.ImageURL || image.Version != frozen.ImageVersion || image.Status != RuntimeImageStatusAvailable {
			return CreateSandboxPlan{}, apperr.ErrSandboxRuntimeUnavailable.WithCause(fmt.Errorf("镜像与冻结声明不一致或已停用: runtime_id=%d image_id=%d composition_digest=%s status=%d", runtime.ID, image.ID, snapshot.Digest, image.Status))
		}
		runtimePlans = append(runtimePlans, RuntimePlan{InstanceCode: frozen.InstanceCode, Runtime: runtime, Image: image})
		runtimeAdapters[frozen.InstanceCode] = runtime.AdapterSpec
	}
	if len(runtimePlans) == 0 {
		return CreateSandboxPlan{}, apperr.ErrSandboxRuntimeUnavailable
	}
	tools := make([]Tool, 0, len(snapshot.Components))
	toolByCode := make(map[string]Tool, len(snapshot.Components))
	for _, component := range snapshot.Components {
		tool, err := tx.GetToolByCode(ctx, component.Code)
		if err != nil {
			return CreateSandboxPlan{}, apperr.ErrSandboxToolNotFound.WithCause(err)
		}
		if tool.ID != component.ComponentID || tool.Status != ToolStatusAvailable {
			return CreateSandboxPlan{}, apperr.ErrSandboxToolIncompatible
		}
		spec, err := toolResourceSpecFromJSON(component.ResourceSpec)
		if err != nil {
			return CreateSandboxPlan{}, apperr.ErrSandboxToolIncompatible.WithCause(err)
		}
		tool.ResourceSpec, tool.Kind, tool.Category = spec, component.Kind, component.Category
		tools = append(tools, tool)
		toolByCode[component.Code] = tool
	}
	if err := validateNetworkRulesForRuntimes(runtimeAdapters, toolByCode); err != nil {
		return CreateSandboxPlan{}, err
	}
	for _, tool := range tools {
		if err := validateToolNetworkRulesForComposition(tool, runtimeAdapters, toolByCode); err != nil {
			return CreateSandboxPlan{}, apperr.ErrSandboxToolIncompatible.WithCause(err)
		}
	}
	privateSidecars := privateSidecarsFromContract(snapshot.PrivateSidecars)
	if err := validatePrivateSidecars(privateSidecars, s.cfg, adapterSpecForPrivateSidecarValidation(runtimePlans)); err != nil {
		return CreateSandboxPlan{}, err
	}
	for _, runtimePlan := range runtimePlans {
		if err := validatePlanImagesCurrentlyAdmitted(s.cfg, runtimePlan.Runtime, runtimePlan.Image, tools, privateSidecars); err != nil {
			return CreateSandboxPlan{}, err
		}
	}
	plan := CreateSandboxPlan{WorkspaceRuntimeInstance: snapshot.Spec.WorkspaceRuntimeInstance, InitChainSteps: snapshot.Spec.InitChainSteps, Runtimes: runtimePlans, Tools: tools, PrivateSidecars: privateSidecars}
	if _, err := servicesForPlan(plan); err != nil {
		return CreateSandboxPlan{}, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	return plan, nil
}

// Stats 返回租户级沙箱资源统计。
func (s *Service) Stats(ctx context.Context, tenantID int64) (contracts.SandboxQuotaStats, error) {
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

// createSandboxRecord 计算过期时间后创建沙箱主记录,首次保存成功后才登记代码归档。
func (s *Service) createSandboxRecord(ctx context.Context, tx TxStore, req contracts.SandboxCreateRequest, sharedAccountIDs []int64, quota TenantQuota) (Sandbox, error) {
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
		InitCodeRef:         req.InitCodeRef,
		InitScriptRef:       req.InitScriptRef,
		KeepAliveUntil:      keepAliveUntil,
		SnapshotExpireAt:    snapshotExpireAt,
		ExpireAt:            now.Add(time.Duration(quota.MaxLifetimeMin) * time.Minute),
	})
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

// GetSandbox 读取当前生命周期、冻结能力及可用的资源观测。
func (s *Service) GetSandbox(ctx context.Context, tenantID, sandboxID int64) (contracts.SandboxInfo, error) {
	var sb Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return contracts.SandboxInfo{}, err
	}
	return s.sandboxInfo(ctx, sb)
}

func (s *Service) sandboxInfo(ctx context.Context, sb Sandbox) (contracts.SandboxInfo, error) {
	snapshot, err := verifiedCompositionSnapshot(sb)
	if err != nil {
		return contracts.SandboxInfo{}, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	runtime, image, err := workspaceRuntimeSnapshot(sb)
	if err != nil {
		return contracts.SandboxInfo{}, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	tools, err := s.currentSandboxTools(ctx, sb)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	out := sandboxInfoFromModel(sb, snapshot, runtime, image, tools)
	out.Capabilities, err = sandboxCapabilitiesFromSnapshot(snapshot, tools, s.capabilities)
	if err != nil {
		return contracts.SandboxInfo{}, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	if s.orchestrator != nil && shouldLoadLiveResourceUsage(sb) {
		usageCtx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ResourceUsageTimeoutSeconds))
		usage, err := s.orchestrator.ResourceUsage(usageCtx, sb)
		cancel()
		if err != nil {
			// 用量属于独立观测结果，未知明确为 null，不覆盖生命周期或伪装成零。
			logging.ErrorContext(ctx, "sandbox resource usage unavailable", apperr.ErrSandboxResourceUsageFailed.WithCause(err).LogString(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID))
		} else {
			out.ResourceUsage = &usage
		}
	}
	return out, nil
}

// verifiedCompositionSnapshot 严格解码沙箱的组合快照并核对摘要,是读取快照的唯一入口。
func verifiedCompositionSnapshot(sb Sandbox) (contracts.SandboxCompositionSnapshot, error) {
	var snapshot contracts.SandboxCompositionSnapshot
	if err := jsonx.DecodeStrictKnownFields(sb.CompositionSnapshot, &snapshot); err != nil {
		return contracts.SandboxCompositionSnapshot{}, err
	}
	digest, err := contracts.CanonicalSnapshotDigest(snapshot)
	if err != nil {
		return contracts.SandboxCompositionSnapshot{}, fmt.Errorf("沙箱组合快照无效: %w", err)
	}
	if digest != sb.CompositionDigest || snapshot.Digest != sb.CompositionDigest {
		return contracts.SandboxCompositionSnapshot{}, fmt.Errorf("沙箱组合快照无效")
	}
	return snapshot, nil
}

// workspaceRuntimeSnapshot 校验准确摘要并解析冻结工作区身份,供状态观察和执行准入共同使用。
func workspaceRuntimeSnapshot(sb Sandbox) (Runtime, RuntimeImage, error) {
	snapshot, err := verifiedCompositionSnapshot(sb)
	if err != nil {
		return Runtime{}, RuntimeImage{}, err
	}
	return runtimeSnapshotForInstance(snapshot, snapshot.Spec.WorkspaceRuntimeInstance)
}

// runtimeSnapshotForInstance 只解析已校验的冻结组合，拓扑实例化对编译快照幂等。
func runtimeSnapshotForInstance(snapshot contracts.SandboxCompositionSnapshot, instance string) (Runtime, RuntimeImage, error) {
	var frozen contracts.CompiledRuntimeSnapshot
	found := false
	for _, item := range snapshot.Runtimes {
		if item.InstanceCode == instance {
			if found {
				return Runtime{}, RuntimeImage{}, fmt.Errorf("运行时实例重复")
			}
			frozen = item
			found = true
		}
	}
	if !found || frozen.RuntimeID <= 0 || frozen.ImageID <= 0 || strings.TrimSpace(frozen.Code) == "" {
		return Runtime{}, RuntimeImage{}, fmt.Errorf("运行时实例未包含在快照中或身份不完整")
	}
	var adapter AdapterSpec
	if err := jsonx.DecodeStrictKnownFields(frozen.AdapterSpec, &adapter); err != nil {
		return Runtime{}, RuntimeImage{}, err
	}
	return Runtime{
		ID: frozen.RuntimeID, Code: frozen.Code, Eco: frozen.Eco,
		AdapterLevel: frozen.AdapterLevel, CapabilityImpl: frozen.CapabilityImpl, PluginRef: frozen.PluginRef,
		AdapterSpec: MaterializeRuntimeTopology(frozen.InstanceCode, adapter),
	}, RuntimeImage{ID: frozen.ImageID, RuntimeID: frozen.RuntimeID, Version: frozen.ImageVersion, ImageURL: frozen.ImageURL}, nil
}

// workspaceRuntimeForSandbox 在冻结身份校验后执行当前目录准入,供工作区操作使用。
func (s *Service) workspaceRuntimeForSandbox(ctx context.Context, sb Sandbox) (Runtime, RuntimeImage, error) {
	frozenRuntime, frozenImage, err := workspaceRuntimeSnapshot(sb)
	if err != nil {
		return Runtime{}, RuntimeImage{}, err
	}
	var runtime Runtime
	var image RuntimeImage
	err = s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		runtime, image, err = admittedRuntimeSnapshot(ctx, tx, frozenRuntime, frozenImage)
		return err
	})
	return runtime, image, err
}

// admittedRuntimeSnapshot 复核当前目录准入，执行声明始终取冻结快照。
func admittedRuntimeSnapshot(ctx context.Context, tx TxStore, frozenRuntime Runtime, frozenImage RuntimeImage) (Runtime, RuntimeImage, error) {
	runtime, err := tx.GetRuntimeByID(ctx, frozenRuntime.ID)
	if err != nil {
		return Runtime{}, RuntimeImage{}, err
	}
	image, err := tx.GetRuntimeImageByID(ctx, frozenRuntime.ID, frozenImage.ID)
	if err != nil {
		return Runtime{}, RuntimeImage{}, err
	}
	if runtime.Code != frozenRuntime.Code || runtime.PluginRef != frozenRuntime.PluginRef || runtime.Status != RuntimeStatusAvailable || runtime.SelftestStatus != RuntimeSelftestPassed || image.RuntimeID != frozenRuntime.ID || image.Version != frozenImage.Version || image.ImageURL != frozenImage.ImageURL || image.Status != RuntimeImageStatusAvailable {
		return Runtime{}, RuntimeImage{}, fmt.Errorf("运行时或镜像当前不可用")
	}
	runtime.AdapterSpec = frozenRuntime.AdapterSpec
	runtime.AdapterLevel = frozenRuntime.AdapterLevel
	runtime.CapabilityImpl = frozenRuntime.CapabilityImpl
	runtime.PluginRef = frozenRuntime.PluginRef
	return runtime, image, nil
}

// shouldLoadLiveResourceUsage 判断当前状态是否存在稳定计算资源,避免创建/回收阶段把 metrics 暂无数据当成业务失败。
func shouldLoadLiveResourceUsage(sb Sandbox) bool {
	return sb.Status == SandboxStatusReady || sb.Status == SandboxStatusRunning || sb.Status == SandboxStatusPaused || sb.Status == SandboxStatusIdle
}

// namespaceFor 根据配置前缀和沙箱 ID 生成动态命名空间。
func namespaceFor(prefix string, id int64) string {
	return strings.Trim(prefix, "-") + "-" + ids.Format(id)
}
