// sandbox service 文件定义服务依赖注入和通用业务编排,不接收数据库连接。
package sandbox

import (
	"context"
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
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/response"
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
	// Exec 在沙箱容器中执行受控命令。
	Exec(ctx context.Context, namespace, container string, command []string, stdin []byte, tty bool) ([]byte, []byte, error)
	// ExecStream 在沙箱容器中执行交互式命令并透传流。
	ExecStream(ctx context.Context, namespace, container string, command []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, tty bool) error
	// PrepullImage 创建或更新预拉取 DaemonSet 并等待真实节点 Ready。
	PrepullImage(ctx context.Context, image RuntimeImage) (PrepullResult, error)
	// DeletePrepullDaemonSet 删除镜像预拉取 DaemonSet,用于镜像停用或删除闭环。
	DeletePrepullDaemonSet(ctx context.Context, image RuntimeImage) error
	// ToolReady 校验 Web 工具容器已达到可代理状态。
	ToolReady(ctx context.Context, sb Sandbox, tool Tool) error
	// SnapshotSupported 返回当前集群是否安装并启用 CSI 快照能力。
	SnapshotSupported(ctx context.Context) (bool, error)
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
	bus          eventbus.Bus
	wsHub        *ws.Hub
	capabilities map[string]ChainCapability
	saveMu       sync.Mutex
	saveTimers   map[int64]*time.Timer
}

// ServiceDeps 是 sandbox service 的装配依赖集合。
type ServiceDeps struct {
	Store        Store
	IDs          snowflake.Generator
	Config       config.SandboxConfig
	Storage      *storage.Storage
	Orchestrator Orchestrator
	Audit        audit.Writer
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
		bus:          deps.EventBus,
		wsHub:        deps.WSHub,
		capabilities: capabilities,
		saveTimers:   map[int64]*time.Timer{},
	}, nil
}

// CreateSandbox 创建沙箱控制面记录并异步推进 K8s 启动。
func (s *Service) CreateSandbox(ctx context.Context, req contracts.SandboxCreateRequest) (contracts.SandboxInfo, error) {
	input := createInputFromContract(req)
	if err := validateCreateRequest(input); err != nil {
		return contracts.SandboxInfo{}, err
	}
	var plan CreateSandboxPlan
	if err := s.store.TenantTx(ctx, input.TenantID, func(ctx context.Context, tx TxStore) error {
		runtime, err := tx.GetRuntimeByCode(ctx, input.RuntimeCode)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		if runtime.Status != RuntimeStatusAvailable || runtime.SelftestStatus != RuntimeSelftestPassed {
			return apperr.ErrSandboxRuntimeUnavailable
		}
		image, err := selectRuntimeImage(ctx, tx, runtime.ID, input.RuntimeImageVersion)
		if err != nil {
			return err
		}
		if !image.Prepulled || image.PrepullStatus != ImagePrepullSucceeded || !image.GenesisBaked {
			return apperr.ErrSandboxRuntimeUnavailable
		}
		quota, err := tx.GetTenantQuota(ctx, input.TenantID)
		if err != nil {
			return apperr.ErrSandboxQuotaInvalid.WithCause(err)
		}
		active, err := tx.CountActiveSandboxes(ctx, input.TenantID)
		if err != nil {
			return apperr.ErrSandboxCreateFailed.WithCause(err)
		}
		if err := validateQuotaForCreate(input, quota, active, s.cfg); err != nil {
			return err
		}
		tools, err := s.resolveTools(ctx, tx, runtime, input.ToolCodes)
		if err != nil {
			return err
		}
		if input.SnapshotEnabled {
			ok, err := s.orchestrator.SnapshotSupported(ctx)
			if err != nil {
				return apperr.ErrSandboxSnapshotUnavailable.WithCause(err)
			}
			if !ok {
				return apperr.ErrSandboxSnapshotUnavailable
			}
		}
		sb, err := s.createSandboxRecord(ctx, tx, input, runtime, image, quota)
		if err != nil {
			return err
		}
		if _, err := s.createToolRecords(ctx, tx, sb, tools); err != nil {
			return err
		}
		detail, err := jsonBytes(map[string]any{
			"runtime_code": runtime.Code,
			"image":        image.ImageURL,
			"source_ref":   input.SourceRef,
		})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), input.TenantID, sb.ID, EventTypeCreate, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		plan = CreateSandboxPlan{Sandbox: sb, Runtime: runtime, Image: image, Tools: tools}
		plan.Sandbox.Status = SandboxStatusCreating
		return nil
	}); err != nil {
		return contracts.SandboxInfo{}, err
	}
	if err := s.writeAudit(ctx, input.TenantID, input.OwnerAccountID, 5, "sandbox.create", "sandbox", plan.Sandbox.ID, map[string]any{"source_ref": input.SourceRef}); err != nil {
		return contracts.SandboxInfo{}, err
	}
	s.startAsync(ctx, plan)
	s.broadcastProgress(ctx, input.TenantID, plan.Sandbox.ID, SandboxPhaseAllocating, SandboxStatusCreating, response.TraceFromContext(ctx))
	return s.info(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID)
}

// GetSandbox 查询单个沙箱当前状态与工具接入信息。
func (s *Service) GetSandbox(ctx context.Context, tenantID, sandboxID int64) (contracts.SandboxInfo, error) {
	return s.info(ctx, tenantID, sandboxID)
}

// GetSandboxForOwner 查询用户自己的沙箱,防止同租户内横向访问。
func (s *Service) GetSandboxForOwner(ctx context.Context, tenantID, accountID, sandboxID int64) (contracts.SandboxInfo, error) {
	info, err := s.info(ctx, tenantID, sandboxID)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	if info.OwnerAccountID != accountID {
		return contracts.SandboxInfo{}, apperr.ErrSandboxOwnershipInvalid
	}
	return info, nil
}

// PauseSandbox 暂停沙箱,按需创建 CSI 快照后释放计算工作负载。
func (s *Service) PauseSandbox(ctx context.Context, req contracts.SandboxControlRequest) error {
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
		snapshotCtx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.ReadyTimeoutSeconds)*time.Second)
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
			return tx.CreateSandboxEvent(ctx, s.ids.Generate(), tenantID, sandboxID, EventTypePhaseChange, detail)
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
func (s *Service) ResumeSandbox(ctx context.Context, req contracts.SandboxControlRequest) error {
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
		s.markStartFailed(ctx, sb, err)
		return apperr.ErrSandboxCreateFailed.WithCause(err)
	}
	if err := s.updateToolReadiness(ctx, plan); err != nil {
		return err
	}
	return s.transition(ctx, sb.TenantID, sb.ID, SandboxPhaseReady, SandboxStatusRunning, "sandbox.resume")
}

// DestroySandbox 主动销毁单个沙箱。
func (s *Service) DestroySandbox(ctx context.Context, req contracts.SandboxControlRequest) error {
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

// RecycleBySourceRef 按来源标识级联回收沙箱。
func (s *Service) RecycleBySourceRef(ctx context.Context, req contracts.SandboxRecycleRequest) error {
	if req.TenantID <= 0 || !validSourceRef(req.SourceRef) {
		return apperr.ErrSandboxRecycleRequestInvalid
	}
	var items []Sandbox
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListSandboxesBySourceRef(ctx, req.TenantID, req.SourceRef)
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
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, sb.TenantID, sb.ID, SandboxPhaseReady, SandboxStatusRunning); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		detail, err := jsonBytes(map[string]any{"phase": SandboxPhaseReady, "status": SandboxStatusRunning, "mode": "snapshot_restore"})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypePhaseChange, detail)
	}); err != nil {
		return err
	}
	s.broadcastProgress(ctx, sb.TenantID, sb.ID, SandboxPhaseReady, SandboxStatusRunning, response.TraceFromContext(ctx))
	return s.writeAudit(ctx, sb.TenantID, sb.OwnerAccountID, 5, "sandbox.resume.snapshot", "sandbox", sb.ID, nil)
}

// planForExistingSandbox 重新加载沙箱恢复或暂停恢复所需的运行时、镜像和工具定义。
func (s *Service) planForExistingSandbox(ctx context.Context, sb Sandbox) (CreateSandboxPlan, error) {
	var runtime Runtime
	var image RuntimeImage
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		runtime, err = tx.GetRuntimeByID(ctx, sb.RuntimeID)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		image, err = tx.GetRuntimeImageByID(ctx, sb.RuntimeID, sb.ImageID)
		if err != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return CreateSandboxPlan{}, err
	}
	tools, err := s.toolsForSandbox(ctx, sb.TenantID, sb.ID)
	if err != nil {
		return CreateSandboxPlan{}, err
	}
	return CreateSandboxPlan{Sandbox: sb, Runtime: runtime, Image: image, Tools: tools}, nil
}

// toolsForSandbox 重新加载沙箱已挂载工具的完整定义,用于快照恢复后重建工具 Service。
func (s *Service) toolsForSandbox(ctx context.Context, tenantID, sandboxID int64) ([]Tool, error) {
	var mounts []SandboxTool
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		mounts, err = tx.ListSandboxTools(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxToolNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	tools := make([]Tool, 0, len(mounts))
	for _, mount := range mounts {
		var tool Tool
		if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
			var err error
			tool, err = tx.GetToolByCode(ctx, mount.ToolCode)
			if err != nil {
				return apperr.ErrSandboxToolNotFound.WithCause(err)
			}
			return nil
		}); err != nil {
			return nil, err
		}
		if tool.Status == ToolStatusAvailable {
			tools = append(tools, tool)
		}
	}
	return tools, nil
}

// Stats 返回租户级沙箱资源统计。
func (s *Service) Stats(ctx context.Context, tenantID int64) (contracts.SandboxQuotaStats, error) {
	var quota TenantQuota
	var active int64
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
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

// resolveTools 按显式工具或运行时默认工具解析工具定义并校验兼容性。
func (s *Service) resolveTools(ctx context.Context, tx TxStore, runtime Runtime, codes []string) ([]Tool, error) {
	if len(codes) == 0 {
		codes = runtime.AdapterSpec.DefaultToolCodes
	}
	tools := make([]Tool, 0, len(codes))
	for _, code := range codes {
		tool, err := tx.GetToolByCode(ctx, strings.TrimSpace(code))
		if err != nil {
			return nil, apperr.ErrSandboxToolNotFound.WithCause(err)
		}
		if tool.Status != ToolStatusAvailable || !toolCompatible(runtime.Eco, tool.EcoTags) {
			return nil, apperr.ErrSandboxToolIncompatible
		}
		if err := validateToolNetworkRulesForRuntime(tool, runtime.AdapterSpec); err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

// createSandboxRecord 计算过期时间和对象存储 key 后创建沙箱主记录。
func (s *Service) createSandboxRecord(ctx context.Context, tx TxStore, req CreateSandboxInputModel, runtime Runtime, image RuntimeImage, quota TenantQuota) (Sandbox, error) {
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
	codeKey, err := storage.ObjectKey(req.TenantID, "sandbox", "code", fmt.Sprintf("%d", id), "workspace.tar")
	if err != nil {
		return Sandbox{}, apperr.ErrSandboxCreateFailed.WithCause(err)
	}
	return tx.CreateSandbox(ctx, CreateSandboxInput{
		ID:               id,
		TenantID:         req.TenantID,
		RuntimeID:        runtime.ID,
		ImageID:          image.ID,
		Namespace:        namespaceFor(s.cfg.NSPrefixStudent, id),
		SourceRef:        req.SourceRef,
		OwnerAccountID:   req.OwnerAccountID,
		Phase:            SandboxPhaseAllocating,
		Status:           SandboxStatusCreating,
		KeepAlive:        req.KeepAlive,
		SnapshotEnabled:  req.SnapshotEnabled,
		CodeStorageKey:   codeKey,
		InitCodeRef:      req.InitCodeRef,
		InitScriptRef:    req.InitScriptRef,
		KeepAliveUntil:   keepAliveUntil,
		SnapshotExpireAt: snapshotExpireAt,
		ExpireAt:         now.Add(time.Duration(quota.MaxLifetimeMin) * time.Minute),
	})
}

// createToolRecords 写入沙箱工具挂载记录。
func (s *Service) createToolRecords(ctx context.Context, tx TxStore, sb Sandbox, tools []Tool) ([]SandboxTool, error) {
	out := make([]SandboxTool, 0, len(tools))
	for _, tool := range tools {
		endpoint := toolEndpoint(sb.ID, tool)
		status := SandboxToolStatusReady
		if tool.Kind == SandboxToolKindWebEmbed {
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
		return fmt.Sprintf("/api/v1/sandbox/sandboxes/%d/terminal", sandboxID)
	default:
		return fmt.Sprintf("/api/v1/sandbox/sandboxes/%d/tools/%s/", sandboxID, tool.Code)
	}
}

// renderBuiltinToolEndpoint 渲染平台内置工具端点模板,模板已在注册规则中限定到 sandbox 模块路径。
func renderBuiltinToolEndpoint(sandboxID int64, spec ToolResourceSpec) string {
	return strings.ReplaceAll(strings.TrimSpace(spec.BuiltinEndpoint), "{sandbox_id}", fmt.Sprintf("%d", sandboxID))
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
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		// 平台级表无租户语义,这里通过沙箱已保存的 runtime_id/image_id 二次查询补齐摘要。
		runtime, err = tx.GetRuntimeByID(ctx, sb.RuntimeID)
		if err != nil {
			return err
		}
		image, err = tx.GetRuntimeImageByID(ctx, sb.RuntimeID, sb.ImageID)
		return err
	}); err != nil {
		return contracts.SandboxInfo{}, apperr.ErrSandboxRuntimeNotFound.WithCause(err)
	}
	out := sandboxInfoFromModel(sb, runtime, image, tools)
	if s.orchestrator != nil && shouldLoadLiveResourceUsage(sb) {
		usage, err := s.orchestrator.ResourceUsage(ctx, sb)
		if err != nil {
			return contracts.SandboxInfo{}, apperr.ErrSandboxResourceUsageFailed.WithCause(err)
		}
		out.ResourceUsage = usage
	}
	return out, nil
}

// shouldLoadLiveResourceUsage 判断当前状态是否存在稳定计算资源,避免创建/回收阶段把 metrics 暂无数据当成业务失败。
func shouldLoadLiveResourceUsage(sb Sandbox) bool {
	return sb.Status == SandboxStatusRunning || sb.Status == SandboxStatusPaused
}

// transition 执行简单状态流转并写入审计。
func (s *Service) transition(ctx context.Context, tenantID, sandboxID int64, phase, status int16, action string) error {
	var actorID int64
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		sb, err := tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if err := validateStateTransition(sb.Status, status); err != nil {
			return err
		}
		actorID = sb.OwnerAccountID
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, tenantID, sandboxID, phase, status); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		detail, err := jsonBytes(map[string]any{"status": status, "phase": phase})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), tenantID, sandboxID, EventTypePhaseChange, detail)
	}); err != nil {
		return err
	}
	s.broadcastProgress(ctx, tenantID, sandboxID, phase, status, response.TraceFromContext(ctx))
	return s.writeAudit(ctx, tenantID, actorID, 5, action, "sandbox", sandboxID, nil)
}

// startAsync 提交异步启动任务,请求返回后继续推进 K8s 创建和阶段变化。
func (s *Service) startAsync(ctx context.Context, plan CreateSandboxPlan) {
	traceCtx := logging.WithAttrs(context.Background(), logging.AttrsFromContext(ctx)...)
	go s.startSandbox(traceCtx, plan)
}

// startSandbox 推进 K8s 编排,阶段失败时写 error 并保留可排查事件。
func (s *Service) startSandbox(ctx context.Context, plan CreateSandboxPlan) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.ReadyTimeoutSeconds)*time.Second)
	defer cancel()
	if err := s.orchestrator.CreateSandboxResources(ctx, plan); err != nil {
		s.cleanupAfterStartFailure(ctx, plan.Sandbox)
		s.markStartFailed(ctx, plan.Sandbox, err)
		return
	}
	if err := s.updateToolReadiness(ctx, plan); err != nil {
		logging.ErrorContext(ctx, "sandbox tool readiness update failed", err.Error(), slog.Int64("tenant_id", plan.Sandbox.TenantID), slog.Int64("sandbox_id", plan.Sandbox.ID))
		s.cleanupAfterStartFailure(ctx, plan.Sandbox)
		s.markStartFailed(ctx, plan.Sandbox, err)
		return
	}
	if err := s.store.TenantTx(ctx, plan.Sandbox.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID, SandboxPhaseReady, SandboxStatusRunning); err != nil {
			return err
		}
		detail, err := jsonBytes(map[string]any{"phase": SandboxPhaseReady})
		if err != nil {
			return err
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), plan.Sandbox.TenantID, plan.Sandbox.ID, EventTypePhaseChange, detail)
	}); err != nil {
		logging.ErrorContext(ctx, "sandbox phase update failed", err.Error(), slog.Int64("tenant_id", plan.Sandbox.TenantID), slog.Int64("sandbox_id", plan.Sandbox.ID))
		return
	}
	s.broadcastProgress(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID, SandboxPhaseReady, SandboxStatusRunning, response.TraceFromContext(ctx))
	if sandboxNeedsInitialization(plan) {
		s.broadcastProgress(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID, SandboxPhaseInitializing, SandboxStatusRunning, response.TraceFromContext(ctx))
		if err := s.store.TenantTx(ctx, plan.Sandbox.TenantID, func(ctx context.Context, tx TxStore) error {
			if _, err := tx.UpdateSandboxPhaseStatus(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID, SandboxPhaseInitializing, SandboxStatusRunning); err != nil {
				return err
			}
			detail, err := jsonBytes(map[string]any{"phase": SandboxPhaseInitializing})
			if err != nil {
				return err
			}
			return tx.CreateSandboxEvent(ctx, s.ids.Generate(), plan.Sandbox.TenantID, plan.Sandbox.ID, EventTypePhaseChange, detail)
		}); err != nil {
			logging.ErrorContext(ctx, "sandbox init phase update failed", err.Error(), slog.Int64("tenant_id", plan.Sandbox.TenantID), slog.Int64("sandbox_id", plan.Sandbox.ID))
			return
		}
		if err := s.applyInitAssetsIfNeeded(ctx, plan.Sandbox, plan.Runtime); err != nil {
			s.markInitFailed(ctx, plan.Sandbox, err)
			return
		}
		if strings.TrimSpace(plan.Sandbox.InitCodeRef) != "" {
			if err := s.restoreInitCodeIfNeeded(ctx, plan.Sandbox, plan.Runtime, plan.Sandbox.InitCodeRef); err != nil {
				s.markInitFailed(ctx, plan.Sandbox, err)
				return
			}
		}
		if strings.TrimSpace(plan.Sandbox.InitScriptRef) != "" {
			if err := s.runInitScriptIfNeeded(ctx, plan.Sandbox, plan.Runtime, plan.Sandbox.InitScriptRef); err != nil {
				s.markInitFailed(ctx, plan.Sandbox, err)
				return
			}
		}
		if err := s.store.TenantTx(ctx, plan.Sandbox.TenantID, func(ctx context.Context, tx TxStore) error {
			if _, err := tx.UpdateSandboxPhaseStatus(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID, SandboxPhaseFullyReady, SandboxStatusRunning); err != nil {
				return err
			}
			detail, err := jsonBytes(map[string]any{"phase": SandboxPhaseFullyReady})
			if err != nil {
				return err
			}
			return tx.CreateSandboxEvent(ctx, s.ids.Generate(), plan.Sandbox.TenantID, plan.Sandbox.ID, EventTypePhaseChange, detail)
		}); err != nil {
			logging.ErrorContext(ctx, "sandbox init phase update failed", err.Error(), slog.Int64("tenant_id", plan.Sandbox.TenantID), slog.Int64("sandbox_id", plan.Sandbox.ID))
			return
		}
		s.broadcastProgress(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID, SandboxPhaseFullyReady, SandboxStatusRunning, response.TraceFromContext(ctx))
	}
}

// sandboxNeedsInitialization 判断沙箱是否存在个性化资产、代码或脚本需要异步执行。
func sandboxNeedsInitialization(plan CreateSandboxPlan) bool {
	return len(plan.Runtime.AdapterSpec.InitAssets) > 0 || strings.TrimSpace(plan.Sandbox.InitCodeRef) != "" || strings.TrimSpace(plan.Sandbox.InitScriptRef) != ""
}

// cleanupAfterStartFailure 在阶段一创建失败后用独立有界上下文清理可能已创建的 K8s 资源。
func (s *Service) cleanupAfterStartFailure(ctx context.Context, sb Sandbox) {
	cleanupBase := logging.WithAttrs(context.Background(), logging.AttrsFromContext(ctx)...)
	cleanupCtx, cancel := context.WithTimeout(cleanupBase, time.Duration(s.cfg.ReadyTimeoutSeconds)*time.Second)
	defer cancel()
	if err := s.orchestrator.DestroySandboxResources(cleanupCtx, sb); err != nil {
		logging.ErrorContext(ctx, "sandbox start cleanup failed", err.Error(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID), slog.String("namespace", sb.Namespace))
	}
}

// markStartFailed 记录启动失败,并避免把未完成资源伪装成 ready。
func (s *Service) markStartFailed(ctx context.Context, sb Sandbox, cause error) {
	if err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		_, err := tx.UpdateSandboxPhaseStatus(ctx, sb.TenantID, sb.ID, sb.Phase, SandboxStatusFailed)
		if err != nil {
			return err
		}
		detail, err := jsonBytes(map[string]any{"stage": "start", "error": logging.SanitizeError(cause.Error())})
		if err != nil {
			return err
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeError, detail)
	}); err != nil {
		logging.ErrorContext(ctx, "sandbox start failure mark failed", err.Error(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID))
	}
	s.broadcastProgress(ctx, sb.TenantID, sb.ID, sb.Phase, SandboxStatusFailed, response.TraceFromContext(ctx))
	logging.ErrorContext(ctx, "sandbox start failed", cause.Error(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID))
}

// markInitFailed 记录阶段二个性化初始化失败,保留阶段一可进入状态供用户继续查看和修复。
func (s *Service) markInitFailed(ctx context.Context, sb Sandbox, cause error) {
	if err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, sb.TenantID, sb.ID, SandboxPhaseInitializing, SandboxStatusRunning); err != nil {
			return err
		}
		detail, err := jsonBytes(map[string]any{"stage": "init", "error": logging.SanitizeError(cause.Error())})
		if err != nil {
			return err
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeError, detail)
	}); err != nil {
		logging.ErrorContext(ctx, "sandbox init failure mark failed", err.Error(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID))
	}
	s.broadcastProgress(ctx, sb.TenantID, sb.ID, SandboxPhaseInitializing, SandboxStatusRunning, response.TraceFromContext(ctx))
	logging.ErrorContext(ctx, "sandbox init failed", cause.Error(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID))
}

// updateToolReadiness 将 Web 工具真实健康检查结果写回控制面,避免未就绪工具被代理。
func (s *Service) updateToolReadiness(ctx context.Context, plan CreateSandboxPlan) error {
	for _, tool := range plan.Tools {
		if tool.Kind != SandboxToolKindWebEmbed {
			continue
		}
		status := SandboxToolStatusReady
		if err := s.orchestrator.ToolReady(ctx, plan.Sandbox, tool); err != nil {
			status = SandboxToolStatusFailed
			logging.ErrorContext(ctx, "sandbox tool readiness failed", err.Error(), slog.Int64("tenant_id", plan.Sandbox.TenantID), slog.Int64("sandbox_id", plan.Sandbox.ID), slog.String("tool_code", tool.Code))
		}
		endpoint := fmt.Sprintf("/api/v1/sandbox/sandboxes/%d/tools/%s/", plan.Sandbox.ID, tool.Code)
		if err := s.store.TenantTx(ctx, plan.Sandbox.TenantID, func(ctx context.Context, tx TxStore) error {
			if _, err := tx.UpdateSandboxToolStatus(ctx, plan.Sandbox.TenantID, plan.Sandbox.ID, tool, endpoint, status); err != nil {
				return apperr.ErrSandboxToolPersistFailed.WithCause(err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// selectRuntimeImage 按请求固定版本或默认版本选择已登记镜像。
func selectRuntimeImage(ctx context.Context, tx TxStore, runtimeID int64, version string) (RuntimeImage, error) {
	if strings.TrimSpace(version) != "" {
		image, err := tx.GetRuntimeImageByVersion(ctx, runtimeID, strings.TrimSpace(version))
		if err != nil {
			return RuntimeImage{}, apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		return image, nil
	}
	image, err := tx.GetDefaultRuntimeImage(ctx, runtimeID)
	if err != nil {
		return RuntimeImage{}, apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
	}
	return image, nil
}

// toolCompatible 判断工具生态标签是否包含运行时生态。
func toolCompatible(eco string, tags []string) bool {
	for _, tag := range tags {
		if tag == "*" || strings.EqualFold(tag, eco) {
			return true
		}
	}
	return false
}

// namespaceFor 根据配置前缀和沙箱 ID 生成动态命名空间。
func namespaceFor(prefix string, id int64) string {
	return fmt.Sprintf("%s-%d", strings.Trim(prefix, "-"), id)
}
