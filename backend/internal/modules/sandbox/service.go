// M2 沙箱服务层:承载沙箱生命周期业务规则、配额校验与编排触发。
package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Service 是 M2 沙箱引擎服务,负责控制面状态、配额与数据面编排触发。
type Service struct {
	repo         *repo
	idgen        *snowflake.Node
	orchestrator Orchestrator
	capabilities CapabilityRegistry
	bus          eventbus.Bus
	store        *storage.Storage
	hub          *ws.Hub
	cfg          config.SandboxConfig
	wsOrigin     ws.OriginPolicy
	auditor      audit.Writer
	identity     contracts.IdentityService
}

// NewService 构造 M2 服务,生产路径必须注入真实 Orchestrator。
func NewService(
	database *db.DB,
	idgen *snowflake.Node,
	orch Orchestrator,
	capabilities CapabilityRegistry,
	bus eventbus.Bus,
	store *storage.Storage,
	hub *ws.Hub,
	cfg config.SandboxConfig,
	wsOrigin ws.OriginPolicy,
	auditor audit.Writer,
	identity contracts.IdentityService,
) *Service {
	return &Service{
		repo:         newRepo(database),
		idgen:        idgen,
		orchestrator: orch,
		capabilities: capabilities,
		bus:          bus,
		store:        store,
		hub:          hub,
		cfg:          cfg,
		wsOrigin:     wsOrigin,
		auditor:      auditor,
		identity:     identity,
	}
}

// ListRuntimes 查询运行时配置列表。
func (s *Service) ListRuntimes(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.repo.listRuntimes(ctx, 100, 0)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, runtimeToMap(row))
	}
	return out, nil
}

// CreateRuntime 注册运行时,新运行时默认接入中且待自检。
func (s *Service) CreateRuntime(ctx context.Context, req CreateRuntimeRequest) (map[string]any, error) {
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Eco) == "" ||
		req.AdapterLevel < 1 || req.AdapterLevel > 3 {
		return nil, apperr.ErrRuntimeInvalid
	}
	spec, err := jsonx.ObjectBytes(req.AdapterSpec, apperr.ErrRuntimeInvalid)
	if err != nil {
		return nil, err
	}
	if _, err := parseRuntimeAdapterSpec(spec); err != nil {
		return nil, err
	}
	row, err := s.repo.createRuntime(ctx, s.idgen.Generate(), req, spec)
	if err != nil {
		return nil, err
	}
	if err := s.writeAudit(ctx, 0, auditActionRuntimeCreate, auditTargetRuntime, row.ID, map[string]any{"code": row.Code}); err != nil {
		return nil, err
	}
	return runtimeToMap(row), nil
}

// CreateRuntimeImage 登记运行时镜像版本。
func (s *Service) CreateRuntimeImage(ctx context.Context, runtimeID int64, req CreateRuntimeImageRequest) (map[string]any, error) {
	if strings.TrimSpace(req.ImageURL) == "" || strings.TrimSpace(req.Version) == "" {
		return nil, apperr.ErrRuntimeImageCreateInvalid
	}
	if err := validateImageSecurityGate(ImageSecuritySpec{
		ImageURL: req.ImageURL,
		Digest:   req.Digest,
	}, s.cfg); err != nil {
		return nil, err
	}
	row, err := s.repo.createRuntimeImage(ctx, s.idgen.Generate(), runtimeID, req)
	if err != nil {
		return nil, err
	}
	if err := s.writeAudit(ctx, 0, auditActionRuntimeImageCreate, auditTargetRuntimeImage, row.ID, map[string]any{
		"runtime_id": runtimeID,
		"version":    row.Version,
	}); err != nil {
		return nil, err
	}
	return runtimeImageToMap(row), nil
}

// ListTools 查询工具定义列表。
func (s *Service) ListTools(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.repo.listTools(ctx, 100, 0)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, toolToMap(row))
	}
	return out, nil
}

// CreateTool 注册工具定义,Web 工具必须先通过镜像安全门禁再进入控制面配置。
func (s *Service) CreateTool(ctx context.Context, req CreateToolRequest) (map[string]any, error) {
	// 第一步校验工具基础字段和类型,Web 工具额外要求镜像与端口。
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.EcoTags) == "" ||
		(req.Kind != ToolKindTerminal && req.Kind != ToolKindWebEmbed && req.Kind != ToolKindPlatformBuiltin) {
		return nil, apperr.ErrToolCreateInvalid
	}
	if req.Kind == ToolKindWebEmbed && (strings.TrimSpace(req.ImageURL) == "" || req.Port <= 0) {
		return nil, apperr.ErrToolCreateInvalid
	}
	if req.Kind == ToolKindWebEmbed {
		if err := validateImageSecurityGate(ImageSecuritySpec{
			ImageURL: req.ImageURL,
			Digest:   req.Digest,
		}, s.cfg); err != nil {
			return nil, apperr.ErrToolCreateInvalid.WithCause(err)
		}
	}
	// 第二步把声明式资源配置转成 JSONB 并复用解析器校验,避免坏配置进入控制面。
	spec, err := jsonx.ObjectBytes(req.ResourceSpec, apperr.ErrToolCreateInvalid)
	if err != nil {
		return nil, err
	}
	if _, err := parseToolResourceSpec(ToolDefinition{
		Code:    req.Code,
		Name:    req.Name,
		Kind:    req.Kind,
		Port:    req.Port,
		EcoTags: req.EcoTags,
	}, spec); err != nil {
		return nil, err
	}
	// 第三步写入全局工具定义并记录审计,后续沙箱创建只引用这份权威配置。
	row, err := s.repo.createTool(ctx, s.idgen.Generate(), req, spec)
	if err != nil {
		return nil, err
	}
	if err := s.writeAudit(ctx, 0, auditActionToolCreate, auditTargetTool, row.ID, map[string]any{"code": row.Code}); err != nil {
		return nil, err
	}
	return toolToMap(row), nil
}

// CreateSandbox 创建沙箱控制面记录并异步触发 K8s 编排,请求返回时只保证控制面已进入创建中。
func (s *Service) CreateSandbox(ctx context.Context, req contracts.SandboxCreateRequest) (contracts.SandboxInfo, error) {
	if err := validateCreateSandboxRequest(req); err != nil {
		return contracts.SandboxInfo{}, err
	}
	if err := validateSandboxSourceRefAccess(ctx, req.SourceRef); err != nil {
		return contracts.SandboxInfo{}, err
	}
	if req.SnapshotEnabled {
		if s.orchestrator == nil {
			return contracts.SandboxInfo{}, apperr.ErrSandboxSnapshotUnavailable
		}
		if err := s.orchestrator.SnapshotAvailable(ctx); err != nil {
			return contracts.SandboxInfo{}, apperr.ErrSandboxSnapshotUnavailable.WithCause(err)
		}
	}
	tenantID := req.TenantID
	sandboxID := s.idgen.Generate()
	namespace := sandboxNamespace(s.cfg.NSPrefixStudent, sandboxID)
	codeKey, err := storage.ObjectKey(tenantID, "sandbox", "code", ids.Format(sandboxID))
	if err != nil {
		return contracts.SandboxInfo{}, apperr.ErrSandboxCreateFail.WithCause(err)
	}

	// 先在 app 连接读取运行时、镜像和工具定义,避免租户事务承担全局配置查询。
	runtime, image, tools, err := s.repo.getRuntimeSelectionByCode(ctx, req.RuntimeCode, strings.TrimSpace(req.RuntimeImageVersion), req.ToolCodes)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	if runtime.Status != RuntimeStatusAvailable || runtime.SelftestStatus != RuntimeSelftestPassed {
		return contracts.SandboxInfo{}, apperr.ErrRuntimeUnavailable
	}
	if !image.Prepulled || image.PrepullStatus != RuntimeImagePrepullDone {
		return contracts.SandboxInfo{}, apperr.ErrRuntimePrepullFailed
	}
	if !image.GenesisBaked {
		return contracts.SandboxInfo{}, apperr.ErrRuntimeUnavailable
	}
	for _, tool := range tools {
		if !toolFitsRuntimeEco(tool.EcoTags, runtime.Eco) {
			return contracts.SandboxInfo{}, apperr.ErrToolNotFitRuntime
		}
	}
	requestedUsage, err := s.sandboxResourceUsageFromRows(runtime, tools)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}

	// 随后进入租户事务检查配额并创建控制面记录,确保并发数、生命周期和资源总量同时受控。
	now := timex.Now()
	record := SandboxCreateRecord{
		ID:              sandboxID,
		TenantID:        tenantID,
		RuntimeID:       runtime.ID,
		ImageID:         image.ID,
		Namespace:       namespace,
		SourceRef:       req.SourceRef,
		OwnerAccountID:  req.OwnerAccountID,
		KeepAlive:       req.KeepAlive,
		SnapshotEnabled: req.SnapshotEnabled,
		CodeStorageKey:  codeKey,
		InitScriptRef:   req.InitScriptRef,
	}
	if req.KeepAlive {
		record.KeepAliveUntil = now.Add(time.Duration(req.KeepAliveMinutes) * time.Minute)
	}
	if req.SnapshotEnabled {
		record.SnapshotExpireAt = now.Add(time.Duration(req.SnapshotRetentionMinutes) * time.Minute)
	}
	toolRecords := make([]SandboxToolCreateRecord, 0, len(tools))
	for _, tool := range tools {
		toolRecords = append(toolRecords, SandboxToolCreateRecord{
			ID:             s.idgen.Generate(),
			TenantID:       tenantID,
			SandboxID:      sandboxID,
			ToolID:         tool.ID,
			AccessEndpoint: sandboxToolEndpoint(sandboxID, tool.Code, tool.Kind),
			Status:         SandboxToolStatusReady,
		})
	}
	eventDetail, err := jsonx.ObjectBytes(map[string]any{
		"runtime_code":          req.RuntimeCode,
		"runtime_image_version": image.Version,
		"source_ref":            req.SourceRef,
	}, apperr.ErrSandboxInvalidState)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	row, err := s.repo.createSandboxControlPlane(ctx, record, toolRecords, s.idgen.Generate(), eventDetail, func(quota TenantQuotaSnapshot, active int64, activeResources []ActiveSandboxResourceSnapshot) (time.Time, error) {
		expireAt := now.Add(time.Duration(quota.MaxLifetimeMin) * time.Minute)
		if active >= int64(quota.MaxConcurrentSandbox) {
			return time.Time{}, apperr.ErrQuotaExceeded
		}
		if req.KeepAlive && req.KeepAliveMinutes > quota.MaxKeepaliveMin {
			return time.Time{}, apperr.ErrQuotaExceeded
		}
		if req.KeepAlive && req.KeepAliveMinutes > quota.MaxLifetimeMin {
			return time.Time{}, apperr.ErrQuotaExceeded
		}
		if req.SnapshotEnabled && req.SnapshotRetentionMinutes > quota.MaxSnapshotRetentionMin {
			return time.Time{}, apperr.ErrQuotaExceeded
		}
		return expireAt, s.checkTenantResourceQuota(quota, activeResources, requestedUsage)
	})
	if err != nil {
		return contracts.SandboxInfo{}, err
	}

	spec, err := s.buildSandboxCreateSpec(runtime, image, tools, row, req.InitCodeRef)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	// 最后构造编排 spec 并启动异步数据面创建,HTTP 请求不阻塞等待 Pod Ready。
	if err := s.writeAudit(ctx, tenantID, auditActionSandboxCreate, auditTargetSandbox, row.ID, map[string]any{
		"runtime_code":          req.RuntimeCode,
		"runtime_image_version": image.Version,
		"source_ref":            req.SourceRef,
	}); err != nil {
		if markErr := s.markSandboxError(ctx, tenantID, sandboxID, err); markErr != nil {
			return contracts.SandboxInfo{}, apperr.ErrSandboxCreateFail.WithCause(
				fmt.Errorf("审计失败: %w; 标记沙箱错误状态失败: %v", err, markErr),
			)
		}
		return contracts.SandboxInfo{}, err
	}
	go s.startSandboxAsync(detachSandboxContext(ctx, tenantID, req.OwnerAccountID), spec)
	return s.sandboxInfo(ctx, tenantID, row)
}

// startSandboxAsync 异步推进阶段一就绪和阶段二个性化初始化。
func (s *Service) startSandboxAsync(ctx context.Context, spec SandboxCreateSpec) {
	// 阶段一等待 K8s 工作负载 Ready,失败时进入 error 并等待回收调度处理。
	if err := s.orchestrator.Create(ctx, spec); err != nil {
		s.recordAsyncSandboxError(ctx, spec, "create_resources", err)
		return
	}
	if err := s.orchestrator.WaitReady(ctx, spec.Namespace); err != nil {
		s.recordAsyncSandboxError(ctx, spec, "wait_ready", err)
		return
	}
	if err := s.updateSandboxProgress(ctx, spec.TenantID, spec.SandboxID, SandboxPhaseEnvironmentReady, SandboxStatusReady, "环境就绪", "节点已就绪,可进入"); err != nil {
		s.recordAsyncSandboxError(ctx, spec, "environment_ready", err)
		return
	}

	// 阶段二执行代码恢复与初始化脚本;失败不伪装为完全就绪。
	if err := s.runSandboxInitialization(ctx, spec); err != nil {
		s.recordAsyncSandboxInitializationError(ctx, spec, err)
	}
}

// detachSandboxContext 保留异步任务需要的租户与审计追踪信息,同时切断 HTTP 请求取消传播。
func detachSandboxContext(parent context.Context, tenantID, accountID int64) context.Context {
	req := audit.RequestContextFrom(parent)
	ctx := audit.WithRequestContext(context.Background(), req)
	ctx = tenant.WithContext(ctx, tenant.Identity{TenantID: tenantID, AccountID: accountID})
	return logging.WithAttrs(ctx,
		slog.Int64("tenant_id", tenantID),
		slog.Int64("account_id", accountID),
	)
}

// recordAsyncSandboxError 显式处理后台启动错误,避免异步任务吞错。
func (s *Service) recordAsyncSandboxError(ctx context.Context, spec SandboxCreateSpec, stage string, err error) {
	if markErr := s.markSandboxError(ctx, spec.TenantID, spec.SandboxID, err); markErr != nil {
		logging.ErrorContext(ctx, "sandbox async error mark failed", markErr.Error(),
			slog.Int64("tenant_id", spec.TenantID),
			slog.Int64("sandbox_id", spec.SandboxID),
			slog.String("namespace", spec.Namespace),
			slog.String("stage", stage),
			slog.String("original_error", err.Error()),
		)
		return
	}
	if s.orchestrator != nil {
		if recycleErr := s.orchestrator.Recycle(ctx, spec.Namespace); recycleErr != nil {
			logging.ErrorContext(ctx, "sandbox async cleanup failed", recycleErr.Error(),
				slog.Int64("tenant_id", spec.TenantID),
				slog.Int64("sandbox_id", spec.SandboxID),
				slog.String("namespace", spec.Namespace),
				slog.String("stage", stage),
				slog.String("original_error", err.Error()),
			)
		}
	}
	logging.ErrorContext(ctx, "sandbox async startup failed", err.Error(),
		slog.Int64("tenant_id", spec.TenantID),
		slog.Int64("sandbox_id", spec.SandboxID),
		slog.String("namespace", spec.Namespace),
		slog.String("stage", stage),
	)
}

// recordAsyncSandboxInitializationError 记录阶段二失败;环境已可进入,因此不把沙箱置为 error。
func (s *Service) recordAsyncSandboxInitializationError(ctx context.Context, spec SandboxCreateSpec, err error) {
	if eventErr := s.recordSandboxEvent(ctx, spec.TenantID, spec.SandboxID, SandboxEventError, map[string]any{
		"stage": "initialization",
		"error": err.Error(),
	}); eventErr != nil {
		logging.ErrorContext(ctx, "sandbox initialization error event failed", eventErr.Error(),
			slog.Int64("tenant_id", spec.TenantID),
			slog.Int64("sandbox_id", spec.SandboxID),
			slog.String("namespace", spec.Namespace),
			slog.String("original_error", err.Error()),
		)
		return
	}
	logging.ErrorContext(ctx, "sandbox initialization failed", err.Error(),
		slog.Int64("tenant_id", spec.TenantID),
		slog.Int64("sandbox_id", spec.SandboxID),
		slog.String("namespace", spec.Namespace),
		slog.String("stage", "initialization"),
	)
	if s.hub != nil {
		s.hub.Broadcast(progressTopic(spec.SandboxID), progressPayload(SandboxProgressEvent{
			SandboxID: spec.SandboxID,
			Phase:     SandboxPhaseInitializing,
			Stage:     "初始化失败",
			Message:   "实验环境初始化失败,请稍后重试",
			Status:    SandboxStatusRunning,
		}))
	}
}

// GetSandbox 查询沙箱摘要。
func (s *Service) GetSandbox(ctx context.Context, sandboxID int64) (contracts.SandboxInfo, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return contracts.SandboxInfo{}, apperr.ErrUnauthorized
	}
	row, err := s.loadSandboxRow(ctx, sandboxID)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	return s.sandboxInfo(ctx, id.TenantID, row)
}

// RecycleBySourceRef 按来源标识级联回收并发布 sandbox.recycled 事件。
func (s *Service) RecycleBySourceRef(ctx context.Context, tenantID int64, sourceRef, reason string) error {
	if !auth.ValidSourceRef(sourceRef) {
		return apperr.ErrSandboxRequestInvalid
	}
	if err := validateSandboxSourceRefAccess(ctx, sourceRef); err != nil {
		return err
	}
	found, err := s.repo.listSandboxesBySourceRef(ctx, tenantID, sourceRef)
	if err != nil {
		return err
	}
	rows := make([]SandboxLifecycleSnapshot, 0, len(found))
	for _, row := range found {
		if row.Status == SandboxStatusDestroyed {
			continue
		}
		if row.Status == SandboxStatusRecycling {
			rows = append(rows, row)
			continue
		}
		recycled, err := s.repo.recycleSandbox(ctx, tenantID, row.ID)
		if err != nil {
			return err
		}
		if err := s.recordSandboxEvent(ctx, tenantID, row.ID, SandboxEventRecycle, map[string]any{"reason": reason}); err != nil {
			return apperr.ErrSandboxRecycleFail.WithCause(err)
		}
		rows = append(rows, recycled)
	}
	for _, row := range rows {
		if row.Status == SandboxStatusDestroyed {
			continue
		}
		if err := s.finalizeSandboxRecycle(ctx, tenantID, row, reason); err != nil {
			return apperr.ErrSandboxRecycleFail.WithCause(err)
		}
	}
	return nil
}

// DestroySandbox 主动销毁单个沙箱并发布回收事件。
func (s *Service) DestroySandbox(ctx context.Context, sandboxID int64, reason string) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	current, err := s.repo.getSandbox(ctx, sandboxID)
	if err != nil {
		return err
	}
	if err := authorizeSandboxRowAccess(ctx, id, current); err != nil {
		return err
	}
	row, err := s.repo.recycleSandbox(ctx, id.TenantID, sandboxID)
	if err != nil {
		return err
	}
	if err := s.recordSandboxEvent(ctx, id.TenantID, sandboxID, SandboxEventRecycle, map[string]any{"reason": reason}); err != nil {
		return apperr.ErrSandboxRecycleFail.WithCause(err)
	}
	return s.finalizeSandboxRecycle(ctx, id.TenantID, row, reason)
}

// finalizeSandboxRecycle 完成回收闭环:保存代码、按需快照、删除或暂停资源、写终态并发布事件。
func (s *Service) finalizeSandboxRecycle(ctx context.Context, tenantID int64, row SandboxLifecycleSnapshot, reason string) error {
	// 先解析运行时绑定并尽力保存工作区代码;已不可交互的沙箱跳过保存但继续回收控制面。
	binding, err := s.runtimeBindingForSandboxRow(ctx, row)
	if err != nil {
		if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrSandboxInvalidState.Code {
			return err
		}
	} else if err := s.saveFilesForSandbox(ctx, row, binding); err != nil {
		return err
	}
	if row.SnapshotEnabled {
		// 快照沙箱在删除/暂停前先创建工作区快照,快照引用仍落在 M2 自有表。
		if binding.Namespace == "" {
			return apperr.ErrSandboxSnapshotUnavailable
		}
		snapshot, err := s.orchestrator.SnapshotWorkspace(ctx, SnapshotSpec{
			SandboxID: row.ID,
			TenantID:  tenantID,
			Namespace: row.Namespace,
			ExpiresAt: row.SnapshotExpireAt,
		})
		if err != nil {
			return err
		}
		if err := s.repo.updateSandboxSnapshot(ctx, tenantID, row.ID, snapshot); err != nil {
			return err
		}
	}
	if row.SnapshotEnabled {
		// 启用快照时暂停数据面保留恢复基础,否则直接销毁 Namespace 释放资源。
		if err := s.orchestrator.Pause(ctx, binding); err != nil {
			return err
		}
	} else {
		if err := s.orchestrator.Recycle(ctx, row.Namespace); err != nil {
			return err
		}
	}
	// 控制面终态、审计和 sandbox.recycled 事件按顺序完成,缺事件时必须显式失败。
	if err := s.repo.destroySandbox(ctx, tenantID, row.ID); err != nil {
		return err
	}
	if err := s.recordSandboxEvent(ctx, tenantID, row.ID, SandboxEventRecycle, map[string]any{"reason": reason, "status": "destroyed"}); err != nil {
		return err
	}
	if err := s.writeAudit(ctx, tenantID, auditActionSandboxRecycle, auditTargetSandbox, row.ID, map[string]any{
		"reason":     reason,
		"source_ref": row.SourceRef,
	}); err != nil {
		return err
	}
	if err := s.publishSandboxRecycled(ctx, contracts.SandboxRecycledEvent{
		TenantID: tenantID, SandboxID: row.ID, SourceRef: row.SourceRef, Reason: reason,
	}); err != nil {
		return err
	}
	return nil
}

// publishSandboxRecycled 发布沙箱回收终态事件,供上层业务模块解除实例占用或更新状态。
func (s *Service) publishSandboxRecycled(ctx context.Context, event contracts.SandboxRecycledEvent) error {
	if s.bus == nil {
		return apperr.ErrSandboxRecycleFail
	}
	if err := s.bus.Publish(ctx, contracts.SubjectSandboxRecycled, event); err != nil {
		return apperr.ErrSandboxRecycleFail.WithCause(err)
	}
	return nil
}

// ChainDeploy 调用运行时 L2 部署能力。
func (s *Service) ChainDeploy(ctx context.Context, sandboxID int64, payload map[string]any) (map[string]any, error) {
	capability, binding, err := s.capabilityForSandbox(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	return capability.Deploy(ctx, binding, payload)
}

// ChainSendTx 调用运行时 L2 发交易能力。
func (s *Service) ChainSendTx(ctx context.Context, sandboxID int64, payload map[string]any) (map[string]any, error) {
	capability, binding, err := s.capabilityForSandbox(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	return capability.SendTx(ctx, binding, payload)
}

// ChainQuery 调用运行时 L2 查询能力。
func (s *Service) ChainQuery(ctx context.Context, sandboxID int64, target string) (map[string]any, error) {
	if strings.TrimSpace(target) == "" {
		return nil, apperr.ErrSandboxChainOperationFail
	}
	capability, binding, err := s.capabilityForSandbox(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	return capability.Query(ctx, binding, target)
}

// ChainReset 调用运行时 L2 重置能力。
func (s *Service) ChainReset(ctx context.Context, sandboxID int64) error {
	capability, binding, err := s.capabilityForSandbox(ctx, sandboxID)
	if err != nil {
		return err
	}
	return capability.Reset(ctx, binding)
}

// GetQuota 查询当前租户配额。
func (s *Service) GetQuota(ctx context.Context) (map[string]any, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return nil, apperr.ErrUnauthorized
	}
	quota, active, err := s.repo.getTenantQuotaWithActiveCount(ctx, id.TenantID)
	if err != nil {
		return nil, err
	}
	return quotaToMap(quota, active), nil
}

// Stats 读取租户沙箱资源统计,供 M9 看板经 contracts 只读聚合。
func (s *Service) Stats(ctx context.Context, tenantID int64) (contracts.SandboxStats, error) {
	if tenantID <= 0 {
		return contracts.SandboxStats{}, apperr.ErrQuotaInvalid
	}
	quota, active, err := s.repo.getTenantQuotaWithActiveCount(ctx, tenantID)
	if err != nil {
		return contracts.SandboxStats{}, err
	}
	return contracts.SandboxStats{
		TenantID:             tenantID,
		ActiveSandboxCount:   active,
		MaxConcurrentSandbox: quota.MaxConcurrentSandbox,
		MaxCPU:               quota.MaxCPU,
		MaxMemoryMB:          quota.MaxMemoryMB,
		IdleTimeoutMin:       quota.IdleTimeoutMin,
		MaxLifetimeMin:       quota.MaxLifetimeMin,
		MaxKeepaliveMin:      quota.MaxKeepaliveMin,
		MaxSnapshotRetention: quota.MaxSnapshotRetentionMin,
	}, nil
}

// UpdateQuota 调整当前租户配额;私有化校管和 SaaS 平台管理策略由 API 鉴权层收口。
func (s *Service) UpdateQuota(ctx context.Context, req QuotaRequest) (map[string]any, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return nil, apperr.ErrUnauthorized
	}
	if err := validateQuotaRequest(req); err != nil {
		return nil, err
	}
	quota, err := s.repo.upsertTenantQuota(ctx, id.TenantID, req)
	if err != nil {
		return nil, err
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionQuotaUpdate, auditTargetQuota, id.TenantID, map[string]any{
		"max_concurrent_sandbox": req.MaxConcurrentSandbox,
		"max_cpu":                req.MaxCPU,
		"max_memory_mb":          req.MaxMemoryMB,
	}); err != nil {
		return nil, err
	}
	return quotaToMap(quota, 0), nil
}

// validateCreateSandboxRequest 校验内部创建沙箱请求的业务边界。
func validateCreateSandboxRequest(req contracts.SandboxCreateRequest) error {
	if req.TenantID <= 0 || req.OwnerAccountID <= 0 || strings.TrimSpace(req.RuntimeCode) == "" || len(req.ToolCodes) == 0 {
		return apperr.ErrSandboxRequestInvalid
	}
	if !auth.ValidSourceRef(req.SourceRef) {
		return apperr.ErrSandboxRequestInvalid
	}
	if req.KeepAlive && req.KeepAliveMinutes <= 0 {
		return apperr.ErrQuotaInvalid
	}
	if req.SnapshotEnabled && req.SnapshotRetentionMinutes <= 0 {
		return apperr.ErrQuotaInvalid
	}
	return nil
}

// validateQuotaRequest 校验配额必须为正数,避免写入不可执行的资源策略。
func validateQuotaRequest(req QuotaRequest) error {
	if req.MaxConcurrentSandbox <= 0 || req.MaxCPU <= 0 || req.MaxMemoryMB <= 0 ||
		req.IdleTimeoutMin <= 0 || req.MaxLifetimeMin <= 0 || req.MaxKeepaliveMin <= 0 ||
		req.MaxSnapshotRetentionMin <= 0 {
		return apperr.ErrQuotaInvalid
	}
	return nil
}

// sandboxResourceUsage 保存按 Kubernetes limits 估算的沙箱资源占用。
type sandboxResourceUsage struct {
	cpuMilli int64
	memoryMB int64
}

// add 累加一段容器资源用量。
func (u *sandboxResourceUsage) add(v sandboxResourceUsage) {
	u.cpuMilli += v.cpuMilli
	u.memoryMB += v.memoryMB
}

// sandboxResourceUsageFromRows 计算一次创建请求声明的运行时、infra sidecar 与 web 工具资源。
func (s *Service) sandboxResourceUsageFromRows(runtime RuntimeConfigSnapshot, tools []ToolConfigSnapshot) (sandboxResourceUsage, error) {
	adapterSpec, err := parseRuntimeAdapterSpec(runtime.AdapterSpec)
	if err != nil {
		return sandboxResourceUsage{}, err
	}
	usage, err := s.runtimeResourceUsage(adapterSpec)
	if err != nil {
		return sandboxResourceUsage{}, err
	}
	for _, tool := range tools {
		toolDef := ToolDefinition{ID: tool.ID, Code: tool.Code, Name: tool.Name, Kind: tool.Kind, Port: tool.Port, EcoTags: tool.EcoTags}
		spec, err := parseToolResourceSpec(toolDef, tool.ResourceSpec)
		if err != nil {
			return sandboxResourceUsage{}, err
		}
		if tool.Kind != ToolKindWebEmbed {
			continue
		}
		item, err := s.resourceSpecUsage(spec.Resources)
		if err != nil {
			return sandboxResourceUsage{}, err
		}
		usage.add(item)
	}
	return usage, nil
}

// checkTenantResourceQuota 聚合当前租户活跃沙箱资源,确保本次创建不会超过 CPU/内存上限。
func (s *Service) checkTenantResourceQuota(quota TenantQuotaSnapshot, rows []ActiveSandboxResourceSnapshot, requested sandboxResourceUsage) error {
	active := sandboxResourceUsage{}
	seenRuntime := make(map[int64]struct{})
	for _, row := range rows {
		// 同一沙箱会因多个工具产生多行,运行时主容器和 infra sidecar 只能按 sandbox 计一次。
		if _, ok := seenRuntime[row.SandboxID]; !ok {
			adapterSpec, err := parseRuntimeAdapterSpec(row.RuntimeAdapterSpec)
			if err != nil {
				return apperr.ErrQuotaResourceBusy.WithCause(err)
			}
			item, err := s.runtimeResourceUsage(adapterSpec)
			if err != nil {
				return err
			}
			active.add(item)
			seenRuntime[row.SandboxID] = struct{}{}
		}
		if row.Tool == nil {
			continue
		}
		tool := ToolDefinition{
			ID:      row.Tool.ID,
			Code:    row.Tool.Code,
			Name:    row.Tool.Name,
			Kind:    row.Tool.Kind,
			Port:    row.Tool.Port,
			EcoTags: row.Tool.EcoTags,
		}
		spec, err := parseToolResourceSpec(tool, row.Tool.ResourceSpec)
		if err != nil {
			return apperr.ErrQuotaResourceBusy.WithCause(err)
		}
		// 只有 Web Embed 工具有独立容器资源,终端和平台内置工具不额外消耗 K8s 工作负载。
		if tool.Kind != ToolKindWebEmbed {
			continue
		}
		item, err := s.resourceSpecUsage(spec.Resources)
		if err != nil {
			return err
		}
		active.add(item)
	}
	active.add(requested)
	if active.cpuMilli > int64(quota.MaxCPU)*1000 || active.memoryMB > int64(quota.MaxMemoryMB) {
		return apperr.ErrQuotaExceeded
	}
	return nil
}

// runtimeResourceUsage 计算运行时主容器与声明式 infra sidecar 的资源占用。
func (s *Service) runtimeResourceUsage(spec RuntimeAdapterSpec) (sandboxResourceUsage, error) {
	usage, err := s.resourceSpecUsage(spec.RuntimeContainer.Resources)
	if err != nil {
		return sandboxResourceUsage{}, err
	}
	for _, sidecar := range spec.InfraSidecars {
		item, err := s.resourceSpecUsage(sidecar.Resources)
		if err != nil {
			return sandboxResourceUsage{}, err
		}
		usage.add(item)
	}
	return usage, nil
}

// resourceSpecUsage 将声明式资源 limits 转成配额单位;缺省时使用平台 LimitRange 默认值。
func (s *Service) resourceSpecUsage(spec ResourceSpec) (sandboxResourceUsage, error) {
	cpu := strings.TrimSpace(spec.Limits.CPU)
	if cpu == "" {
		cpu = s.cfg.DefaultCPU
	}
	memory := strings.TrimSpace(spec.Limits.Memory)
	if memory == "" {
		memory = s.cfg.DefaultMemory
	}
	cpuQuantity, err := resource.ParseQuantity(cpu)
	if err != nil {
		return sandboxResourceUsage{}, apperr.ErrQuotaInvalid.WithCause(err)
	}
	memoryQuantity, err := resource.ParseQuantity(memory)
	if err != nil {
		return sandboxResourceUsage{}, apperr.ErrQuotaInvalid.WithCause(err)
	}
	const bytesPerMiB = 1024 * 1024
	memoryBytes := memoryQuantity.Value()
	return sandboxResourceUsage{
		cpuMilli: cpuQuantity.MilliValue(),
		memoryMB: (memoryBytes + bytesPerMiB - 1) / bytesPerMiB,
	}, nil
}

// quotaToMap 输出配额与当前用量。
func quotaToMap(q TenantQuotaSnapshot, active int64) map[string]any {
	return map[string]any{
		"tenant_id":                  ids.Format(q.TenantID),
		"max_concurrent_sandbox":     q.MaxConcurrentSandbox,
		"max_cpu":                    q.MaxCPU,
		"max_memory_mb":              q.MaxMemoryMB,
		"idle_timeout_min":           q.IdleTimeoutMin,
		"max_lifetime_min":           q.MaxLifetimeMin,
		"max_keepalive_min":          q.MaxKeepaliveMin,
		"max_snapshot_retention_min": q.MaxSnapshotRetentionMin,
		"active_sandbox_count":       active,
	}
}

// runtimeToMap 输出运行时配置。
func runtimeToMap(r RuntimeConfigSnapshot) map[string]any {
	return map[string]any{
		"id":              ids.Format(r.ID),
		"code":            r.Code,
		"name":            r.Name,
		"eco":             r.Eco,
		"adapter_level":   r.AdapterLevel,
		"adapter_spec":    jsonx.ObjectMap(r.AdapterSpec),
		"capability_impl": r.CapabilityImpl,
		"plugin_ref":      r.PluginRef,
		"selftest_status": r.SelftestStatus,
		"selftest_detail": jsonx.ObjectMap(r.SelftestDetail),
		"status":          r.Status,
	}
}

// runtimeImageToMap 输出运行时镜像配置。
func runtimeImageToMap(r RuntimeImageSnapshot) map[string]any {
	return map[string]any{
		"id":             ids.Format(r.ID),
		"runtime_id":     ids.Format(r.RuntimeID),
		"image_url":      r.ImageURL,
		"version":        r.Version,
		"prepulled":      r.Prepulled,
		"prepull_status": r.PrepullStatus,
		"prepull_detail": jsonx.ObjectMap(r.PrepullDetail),
		"prepulled_at":   r.PrepulledAt,
		"genesis_baked":  r.GenesisBaked,
		"is_default":     r.IsDefault,
	}
}

// toolToMap 输出工具定义。
func toolToMap(t ToolConfigSnapshot) map[string]any {
	return map[string]any{
		"id":            ids.Format(t.ID),
		"code":          t.Code,
		"name":          t.Name,
		"kind":          t.Kind,
		"image_url":     t.ImageURL,
		"port":          t.Port,
		"eco_tags":      t.EcoTags,
		"resource_spec": jsonx.ObjectMap(t.ResourceSpec),
		"status":        t.Status,
	}
}

// sandboxInfo 聚合沙箱主表、镜像版本和工具接入端点为跨模块 contracts DTO。
func (s *Service) sandboxInfo(ctx context.Context, tenantID int64, row SandboxLifecycleSnapshot) (contracts.SandboxInfo, error) {
	tools := []contracts.SandboxToolAccess{}
	// 第一步读取全局镜像版本,contracts DTO 需要暴露版本号而不是内部 image_id。
	image, err := s.repo.getRuntimeImage(ctx, row.RuntimeID, row.ImageID)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	// 第二步在租户 RLS 下读取工具端点,避免跨租户泄露沙箱接入信息。
	rows, err := s.repo.listSandboxToolAccess(ctx, tenantID, row.ID)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	for _, item := range rows {
		tools = append(tools, contracts.SandboxToolAccess{
			ToolCode: item.ToolCode,
			Kind:     item.ToolKind,
			Endpoint: item.AccessEndpoint,
			Status:   item.Status,
		})
	}
	// 第三步只返回跨模块契约需要的摘要字段,不暴露 Pod IP 或 kubeconfig。
	return contracts.SandboxInfo{
		SandboxID:           row.ID,
		TenantID:            row.TenantID,
		Namespace:           row.Namespace,
		SourceRef:           row.SourceRef,
		OwnerID:             row.OwnerAccountID,
		RuntimeImageVersion: image.Version,
		Phase:               row.Phase,
		Status:              row.Status,
		ToolAccess:          tools,
	}, nil
}

// capabilityForSandbox 校验沙箱可交互边界后,根据 runtime 找到 L2 链能力实现器和实时绑定。
func (s *Service) capabilityForSandbox(ctx context.Context, sandboxID int64) (ChainCapability, SandboxRuntimeBinding, error) {
	if _, ok := tenantFromContext(ctx); !ok {
		return nil, SandboxRuntimeBinding{}, apperr.ErrUnauthorized
	}
	row, err := s.repo.getSandbox(ctx, sandboxID)
	if err != nil {
		return nil, SandboxRuntimeBinding{}, err
	}
	if err := validateSandboxSourceRefAccess(ctx, row.SourceRef); err != nil {
		return nil, SandboxRuntimeBinding{}, err
	}
	if err := ensureSandboxInteractive(row.Status); err != nil {
		return nil, SandboxRuntimeBinding{}, err
	}
	// 再从全局 runtime 配置解析能力实现器,模块外部只能通过 contracts 调用这里。
	runtime, err := s.repo.getRuntime(ctx, row.RuntimeID)
	if err != nil {
		return nil, SandboxRuntimeBinding{}, err
	}
	if strings.TrimSpace(runtime.CapabilityImpl) == "" {
		return nil, SandboxRuntimeBinding{}, apperr.ErrRuntimeCapabilityUnavailable
	}
	if s.capabilities == nil {
		return nil, SandboxRuntimeBinding{}, apperr.ErrRuntimeCapabilityUnavailable
	}
	capability, exists := s.capabilities.Get(runtime.CapabilityImpl)
	if !exists {
		return nil, SandboxRuntimeBinding{}, apperr.ErrRuntimeCapabilityUnavailable
	}
	spec, err := parseRuntimeAdapterSpec(runtime.AdapterSpec)
	if err != nil {
		return nil, SandboxRuntimeBinding{}, err
	}
	// 最后读取实时 K8s 绑定,避免对已经漂移或被回收的数据面继续执行链操作。
	binding, err := s.orchestrator.RuntimeBinding(ctx, row.Namespace)
	if err != nil {
		return nil, SandboxRuntimeBinding{}, apperr.ErrRuntimeUnavailable.WithCause(err)
	}
	if binding.WorkspaceDir == "" {
		binding.WorkspaceDir = spec.WorkspaceDir
	}
	return capability, binding, nil
}

// ensureSandboxInteractive 限制交互与链能力只作用于仍可进入的沙箱。
func ensureSandboxInteractive(status int16) error {
	switch status {
	case SandboxStatusReady, SandboxStatusRunning, SandboxStatusIdle:
		return nil
	default:
		return apperr.ErrSandboxInvalidState
	}
}

// markSandboxError 把创建失败的控制面状态落为 error,保留事件便于排查。
func (s *Service) markSandboxError(ctx context.Context, tenantID, sandboxID int64, cause error) error {
	if err := s.repo.updateSandboxPhaseStatus(ctx, tenantID, sandboxID, SandboxPhaseAllocating, SandboxStatusError); err != nil {
		return err
	}
	return s.recordSandboxEvent(ctx, tenantID, sandboxID, SandboxEventError, map[string]any{"error": cause.Error()})
}

// recordSandboxEvent 写沙箱生命周期事件,detail 使用 JSONB 保存结构化上下文。
func (s *Service) recordSandboxEvent(ctx context.Context, tenantID, sandboxID int64, eventType string, detail map[string]any) error {
	data, err := jsonx.ObjectBytes(detail, apperr.ErrSandboxInvalidState)
	if err != nil {
		return err
	}
	return s.repo.createSandboxEvent(ctx, tenantID, sandboxID, s.idgen.Generate(), eventType, data)
}

// sandboxToolEndpoint 生成控制面代理端点,前端不直接接触 Pod IP 或 kubeconfig。
func sandboxToolEndpoint(sandboxID int64, toolCode string, kind int16) string {
	if kind == ToolKindTerminal {
		return "/api/v1/sandbox/sandboxes/" + ids.Format(sandboxID) + "/terminal?container=runtime"
	}
	return "/api/v1/sandbox/sandboxes/" + ids.Format(sandboxID) + "/tools/" + toolCode + "/"
}

// sandboxNamespace 生成每沙箱独占的 K8s Namespace 名称。
func sandboxNamespace(prefix string, sandboxID int64) string {
	return strings.Trim(prefix, "-") + "-" + ids.Format(sandboxID)
}

// validateSandboxSourceRefAccess 校验服务间签名绑定的 source_ref 与沙箱归属一致。
func validateSandboxSourceRefAccess(ctx context.Context, sourceRef string) error {
	if !auth.ServiceSourceRefAuthorized(ctx, sourceRef) {
		return apperr.ErrSandboxAccessDenied
	}
	return nil
}

// authorizeSandboxRowAccess 统一用户 owner 与内部服务 source_ref 两种沙箱归属校验。
func authorizeSandboxRowAccess(ctx context.Context, id tenant.Identity, row SandboxLifecycleSnapshot) error {
	if _, ok := auth.ServiceSourceRefFromContext(ctx); ok {
		return validateSandboxSourceRefAccess(ctx, row.SourceRef)
	}
	if row.OwnerAccountID != id.AccountID && !id.IsPlatform {
		return apperr.ErrSandboxAccessDenied
	}
	return nil
}

// validateRuntimeImageURL 限制运行时镜像只能来自配置声明的私有仓库前缀。
func validateRuntimeImageURL(imageURL string, cfg config.SandboxConfig) error {
	registry := strings.Trim(strings.TrimSpace(cfg.ImageRegistry), "/")
	if registry == "" {
		return apperr.ErrRuntimeInvalid
	}
	normalized := strings.TrimSpace(imageURL)
	if normalized == "" || !strings.HasPrefix(normalized, registry+"/") {
		return apperr.ErrRuntimeInvalid
	}
	return nil
}

// toolFitsRuntimeEco 按逗号分隔生态标签判断工具是否适配运行时。
func toolFitsRuntimeEco(tags, eco string) bool {
	for _, tag := range strings.Split(tags, ",") {
		if strings.TrimSpace(tag) == eco {
			return true
		}
	}
	return false
}

// buildSandboxCreateSpec 把数据库行与声明式 spec 聚合成编排输入。
func (s *Service) buildSandboxCreateSpec(
	runtime RuntimeConfigSnapshot,
	image RuntimeImageSnapshot,
	tools []ToolConfigSnapshot,
	row SandboxLifecycleSnapshot,
	initCodeRef string,
) (SandboxCreateSpec, error) {
	adapterSpec, err := parseRuntimeAdapterSpec(runtime.AdapterSpec)
	if err != nil {
		return SandboxCreateSpec{}, err
	}
	out := SandboxCreateSpec{
		SandboxID: row.ID,
		TenantID:  row.TenantID,
		Namespace: row.Namespace,
		Runtime: RuntimeDefinition{
			ID:             runtime.ID,
			Code:           runtime.Code,
			Eco:            runtime.Eco,
			CapabilityImpl: runtime.CapabilityImpl,
			AdapterSpec:    adapterSpec,
		},
		Image: RuntimeImageDefinition{
			ID:           image.ID,
			ImageURL:     image.ImageURL,
			Version:      image.Version,
			GenesisBaked: image.GenesisBaked,
		},
		InitCodeRef:              initCodeRef,
		InitScriptRef:            row.InitScriptRef,
		OwnerAccountID:           row.OwnerAccountID,
		SourceRef:                row.SourceRef,
		KeepAlive:                row.KeepAlive,
		KeepAliveMinutes:         minutesBetween(row.CreatedAt, row.KeepAliveUntil),
		SnapshotEnabled:          row.SnapshotEnabled,
		SnapshotRetentionMinutes: minutesBetween(row.CreatedAt, row.SnapshotExpireAt),
		CodeStorageKey:           row.CodeStorageKey,
	}
	for _, tool := range tools {
		resourceSpec, err := parseToolResourceSpec(ToolDefinition{
			ID:      tool.ID,
			Code:    tool.Code,
			Name:    tool.Name,
			Kind:    tool.Kind,
			Port:    tool.Port,
			EcoTags: tool.EcoTags,
		}, tool.ResourceSpec)
		if err != nil {
			return SandboxCreateSpec{}, err
		}
		out.Tools = append(out.Tools, ToolDefinition{
			ID:       tool.ID,
			Code:     tool.Code,
			Name:     tool.Name,
			Kind:     tool.Kind,
			Port:     tool.Port,
			EcoTags:  tool.EcoTags,
			ImageURL: tool.ImageURL,
			Spec:     resourceSpec,
		})
	}
	return out, nil
}

// minutesBetween 把两个 timestamptz 字段转换为配置分钟数;任一缺失返回 0。
func minutesBetween(start, end time.Time) int32 {
	if start.IsZero() || end.IsZero() {
		return 0
	}
	minutes := int32(end.Sub(start).Minutes())
	if minutes < 0 {
		return 0
	}
	return minutes
}

// updateSandboxProgress 推进控制面阶段并向 progress WS 广播。
func (s *Service) updateSandboxProgress(
	ctx context.Context,
	tenantID, sandboxID int64,
	phase, status int16,
	stage, message string,
) error {
	if err := s.repo.updateSandboxPhaseStatus(ctx, tenantID, sandboxID, phase, status); err != nil {
		return apperr.ErrSandboxPersistenceFail.WithCause(err)
	}
	if err := s.recordSandboxEvent(ctx, tenantID, sandboxID, SandboxEventPhaseChange, map[string]any{
		"phase":   phase,
		"status":  status,
		"stage":   stage,
		"message": message,
	}); err != nil {
		return apperr.ErrSandboxPersistenceFail.WithCause(err)
	}
	if s.hub != nil {
		s.hub.Broadcast(progressTopic(sandboxID), progressPayload(SandboxProgressEvent{
			SandboxID: sandboxID,
			Phase:     phase,
			Stage:     stage,
			Message:   message,
			Status:    status,
		}))
	}
	return nil
}

// runSandboxInitialization 恢复代码、执行初始化脚本,并把沙箱推进到完全就绪。
func (s *Service) runSandboxInitialization(ctx context.Context, spec SandboxCreateSpec) error {
	if err := s.updateSandboxProgress(ctx, spec.TenantID, spec.SandboxID, SandboxPhaseInitializing, SandboxStatusRunning, "个性化初始化中", "正在恢复代码并执行初始化脚本"); err != nil {
		return err
	}
	binding, err := s.orchestrator.RuntimeBinding(ctx, spec.Namespace)
	if err != nil {
		return apperr.ErrSandboxCreateFail.WithCause(err)
	}
	if binding.WorkspaceDir == "" {
		binding.WorkspaceDir = spec.Runtime.AdapterSpec.WorkspaceDir
	}
	if err := s.restoreInitialCode(ctx, spec, binding); err != nil {
		return err
	}
	if err := s.runInitScript(ctx, spec, binding); err != nil {
		return err
	}
	return s.updateSandboxProgress(ctx, spec.TenantID, spec.SandboxID, SandboxPhaseReady, SandboxStatusRunning, "完全就绪", "初始化完成,可开始操作")
}

// restoreInitialCode 把初始代码恢复到运行时工作目录。
func (s *Service) restoreInitialCode(ctx context.Context, spec SandboxCreateSpec, binding SandboxRuntimeBinding) (err error) {
	if strings.TrimSpace(spec.InitCodeRef) == "" || s.store == nil {
		return nil
	}
	ref, err := storage.ParseObjectRef(spec.InitCodeRef)
	if err != nil {
		return apperr.ErrSandboxFileInvalid.WithCause(err)
	}
	reader, err := s.store.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return apperr.ErrSandboxInitFail.WithCause(err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			err = errors.Join(err, apperr.ErrSandboxInitFail.WithCause(closeErr))
		}
	}()

	var archive bytes.Buffer
	if _, err := io.Copy(&archive, io.LimitReader(reader, s.cfg.InitArchiveMaxUnpackedBytes+1)); err != nil {
		return apperr.ErrSandboxInitFail.WithCause(err)
	}
	if int64(archive.Len()) > s.cfg.InitArchiveMaxUnpackedBytes {
		return apperr.ErrSandboxFileInvalid
	}
	safeArchive, err := safeSandboxInitArchive(archive.Bytes(), s.cfg)
	if err != nil {
		return err
	}
	command := []string{"sh", "-lc", "mkdir -p " + shellQuote(binding.WorkspaceDir) + " && tar -xzf - -C " + shellQuote(binding.WorkspaceDir)}
	if err := s.orchestrator.Exec(ctx, binding, command, bytes.NewReader(safeArchive), nil, nil, false); err != nil {
		return apperr.ErrSandboxInitFail.WithCause(err)
	}
	return nil
}

// runInitScript 在运行时主容器内执行初始化脚本。
func (s *Service) runInitScript(ctx context.Context, spec SandboxCreateSpec, binding SandboxRuntimeBinding) (err error) {
	if strings.TrimSpace(spec.InitScriptRef) == "" || s.store == nil {
		return nil
	}
	ref, err := storage.ParseObjectRef(spec.InitScriptRef)
	if err != nil {
		return apperr.ErrSandboxFileInvalid.WithCause(err)
	}
	reader, err := s.store.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return apperr.ErrSandboxInitFail.WithCause(err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			err = errors.Join(err, apperr.ErrSandboxInitFail.WithCause(closeErr))
		}
	}()

	var script bytes.Buffer
	if _, err := io.Copy(&script, reader); err != nil {
		return apperr.ErrSandboxInitFail.WithCause(err)
	}
	command := []string{"sh", "-lc", "cd " + shellQuote(binding.WorkspaceDir) + " && sh -s"}
	if err := s.orchestrator.Exec(ctx, binding, command, bytes.NewReader(script.Bytes()), nil, nil, false); err != nil {
		return apperr.ErrSandboxInitFail.WithCause(err)
	}
	return nil
}
