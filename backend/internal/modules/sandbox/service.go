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
	"chaimir/internal/modules/sandbox/internal/sqlcgen"
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

	"github.com/jackc/pgx/v5/pgtype"
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
	var out []map[string]any
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		rows, err := q.ListRuntimes(ctx, sqlcgen.ListRuntimesParams{Limit: 100, Offset: 0})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, runtimeToMap(row))
		}
		return nil
	}); err != nil {
		return nil, apperr.ErrRuntimePersistenceFail.WithCause(err)
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
	var row sqlcgen.Runtime
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.CreateRuntime(ctx, sqlcgen.CreateRuntimeParams{
			ID: s.idgen.Generate(), Code: req.Code, Name: req.Name, Eco: req.Eco,
			AdapterLevel: req.AdapterLevel, AdapterSpec: spec,
			CapabilityImpl: pgText(req.CapabilityImpl), PluginRef: pgText(req.PluginRef),
			SelftestStatus: RuntimeSelftestPending, SelftestDetail: []byte("{}"),
			Status: RuntimeStatusOnboarding,
		})
		return e
	}); err != nil {
		return nil, apperr.ErrRuntimePersistenceFail.WithCause(err)
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
	var row sqlcgen.RuntimeImage
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetRuntimeByID(ctx, runtimeID); err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrRuntimeNotFound
			}
			return err
		}
		var e error
		row, e = q.CreateRuntimeImage(ctx, sqlcgen.CreateRuntimeImageParams{
			ID: s.idgen.Generate(), RuntimeID: runtimeID, ImageUrl: req.ImageURL, Version: req.Version,
			Prepulled: false, PrepullStatus: RuntimeImagePrepullPending,
			PrepullDetail: []byte("{}"), GenesisBaked: req.GenesisBaked, IsDefault: req.IsDefault,
		})
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrRuntimePersistenceFail.WithCause(err)
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
	var out []map[string]any
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		rows, err := q.ListTools(ctx, sqlcgen.ListToolsParams{Limit: 100, Offset: 0})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, toolToMap(row))
		}
		return nil
	}); err != nil {
		return nil, apperr.ErrToolPersistenceFail.WithCause(err)
	}
	return out, nil
}

// CreateTool 注册工具定义。
func (s *Service) CreateTool(ctx context.Context, req CreateToolRequest) (map[string]any, error) {
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
	var row sqlcgen.Tool
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.CreateTool(ctx, sqlcgen.CreateToolParams{
			ID: s.idgen.Generate(), Code: req.Code, Name: req.Name, Kind: req.Kind,
			ImageUrl: pgText(req.ImageURL), Port: pgInt4(req.Port, req.Port > 0),
			EcoTags: req.EcoTags, ResourceSpec: spec, Status: ToolStatusAvailable,
		})
		return e
	}); err != nil {
		return nil, apperr.ErrToolPersistenceFail.WithCause(err)
	}
	if err := s.writeAudit(ctx, 0, auditActionToolCreate, auditTargetTool, row.ID, map[string]any{"code": row.Code}); err != nil {
		return nil, err
	}
	return toolToMap(row), nil
}

// CreateSandbox 创建沙箱控制面记录并触发 K8s 编排。
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

	var (
		runtime sqlcgen.Runtime
		image   sqlcgen.RuntimeImage
		tools   []sqlcgen.Tool
		row     sqlcgen.Sandbox
	)
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		runtime, e = q.GetRuntimeByCode(ctx, req.RuntimeCode)
		if e != nil {
			if db.IsNoRows(e) {
				return apperr.ErrRuntimeNotFound
			}
			return e
		}
		if runtime.Status != RuntimeStatusAvailable || runtime.SelftestStatus != RuntimeSelftestPassed {
			return apperr.ErrRuntimeUnavailable
		}
		if strings.TrimSpace(req.RuntimeImageVersion) != "" {
			image, e = q.GetRuntimeImageByVersion(ctx, sqlcgen.GetRuntimeImageByVersionParams{
				RuntimeID: runtime.ID,
				Version:   strings.TrimSpace(req.RuntimeImageVersion),
			})
		} else {
			image, e = q.GetDefaultRuntimeImage(ctx, runtime.ID)
		}
		if e != nil {
			if db.IsNoRows(e) {
				return apperr.ErrRuntimeImageNotFound
			}
			return e
		}
		if !image.Prepulled || image.PrepullStatus != RuntimeImagePrepullDone {
			return apperr.ErrRuntimePrepullFailed
		}
		if !image.GenesisBaked {
			return apperr.ErrRuntimeUnavailable
		}
		for _, code := range req.ToolCodes {
			tool, e := q.GetToolByCode(ctx, code)
			if e != nil {
				if db.IsNoRows(e) {
					return apperr.ErrToolNotFound
				}
				return e
			}
			if !toolFitsRuntimeEco(tool.EcoTags, runtime.Eco) {
				return apperr.ErrToolNotFitRuntime
			}
			tools = append(tools, tool)
		}
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return contracts.SandboxInfo{}, ae
		}
		return contracts.SandboxInfo{}, apperr.ErrSandboxCreateFail.WithCause(err)
	}
	requestedUsage, err := s.sandboxResourceUsageFromRows(runtime, tools)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}

	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		quota, e := q.GetTenantQuota(ctx, tenantID)
		if e != nil {
			if db.IsNoRows(e) {
				return apperr.ErrQuotaInvalid
			}
			return e
		}
		active, e := q.CountActiveSandboxes(ctx)
		if e != nil {
			return e
		}
		if active >= int64(quota.MaxConcurrentSandbox) {
			return apperr.ErrQuotaExceeded
		}
		if req.KeepAlive && req.KeepAliveMinutes > quota.MaxKeepaliveMin {
			return apperr.ErrQuotaExceeded
		}
		if req.KeepAlive && req.KeepAliveMinutes > quota.MaxLifetimeMin {
			return apperr.ErrQuotaExceeded
		}
		if req.SnapshotEnabled && req.SnapshotRetentionMinutes > quota.MaxSnapshotRetentionMin {
			return apperr.ErrQuotaExceeded
		}
		if e := s.checkTenantResourceQuota(ctx, q, quota, requestedUsage); e != nil {
			return e
		}
		now := timex.Now()
		var keepAliveUntil pgtype.Timestamptz
		if req.KeepAlive {
			keepAliveUntil = timex.RequiredTimestamptz(now.Add(time.Duration(req.KeepAliveMinutes) * time.Minute))
		}
		var snapshotExpireAt pgtype.Timestamptz
		if req.SnapshotEnabled {
			snapshotExpireAt = timex.RequiredTimestamptz(now.Add(time.Duration(req.SnapshotRetentionMinutes) * time.Minute))
		}
		row, e = q.CreateSandbox(ctx, sqlcgen.CreateSandboxParams{
			ID: sandboxID, TenantID: tenantID, RuntimeID: runtime.ID, ImageID: image.ID,
			Namespace: namespace, SourceRef: req.SourceRef, OwnerAccountID: req.OwnerAccountID,
			Phase: SandboxPhaseAllocating, Status: SandboxStatusCreating,
			KeepAlive: req.KeepAlive, SnapshotEnabled: req.SnapshotEnabled,
			CodeStorageKey: codeKey, InitScriptRef: pgText(req.InitScriptRef),
			KeepAliveUntil: keepAliveUntil, SnapshotExpireAt: snapshotExpireAt,
			ExpireAt: timex.RequiredTimestamptz(now.Add(time.Duration(quota.MaxLifetimeMin) * time.Minute)),
		})
		if e != nil {
			return e
		}
		for _, tool := range tools {
			if _, e = q.CreateSandboxTool(ctx, sqlcgen.CreateSandboxToolParams{
				ID: s.idgen.Generate(), TenantID: tenantID, SandboxID: sandboxID, ToolID: tool.ID,
				AccessEndpoint: sandboxToolEndpoint(sandboxID, tool.Code, tool.Kind),
				Status:         SandboxToolStatusReady,
			}); e != nil {
				return e
			}
		}
		return s.writeEvent(ctx, q, tenantID, sandboxID, SandboxEventCreate, map[string]any{
			"runtime_code":          req.RuntimeCode,
			"runtime_image_version": image.Version,
			"source_ref":            req.SourceRef,
		})
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return contracts.SandboxInfo{}, ae
		}
		return contracts.SandboxInfo{}, apperr.ErrSandboxCreateFail.WithCause(err)
	}

	spec, err := s.buildSandboxCreateSpec(runtime, image, tools, row, req.InitCodeRef)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
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
	if eventErr := s.repo.inTenantID(ctx, spec.TenantID, func(q *sqlcgen.Queries) error {
		return s.writeEvent(ctx, q, spec.TenantID, spec.SandboxID, SandboxEventError, map[string]any{
			"stage": "initialization",
			"error": err.Error(),
		})
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
	if err := validateSourceRef(sourceRef); err != nil {
		return err
	}
	if err := validateSandboxSourceRefAccess(ctx, sourceRef); err != nil {
		return err
	}
	var rows []sqlcgen.Sandbox
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, e := q.ListSandboxesBySourceRef(ctx, sourceRef)
		if e != nil {
			return e
		}
		for _, row := range found {
			if row.Status == SandboxStatusDestroyed {
				continue
			}
			if row.Status == SandboxStatusRecycling {
				rows = append(rows, row)
				continue
			}
			recycled, e := q.RecycleSandbox(ctx, row.ID)
			if e != nil {
				return e
			}
			if e := s.writeEvent(ctx, q, tenantID, row.ID, SandboxEventRecycle, map[string]any{"reason": reason}); e != nil {
				return e
			}
			rows = append(rows, recycled)
		}
		return nil
	}); err != nil {
		return apperr.ErrSandboxRecycleFail.WithCause(err)
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
	var row sqlcgen.Sandbox
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		current, err := q.GetSandboxByID(ctx, sandboxID)
		if err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrSandboxNotFound
			}
			return err
		}
		if err := authorizeSandboxRowAccess(ctx, id, current); err != nil {
			return err
		}
		row, err = q.RecycleSandbox(ctx, sandboxID)
		if err != nil {
			return err
		}
		return s.writeEvent(ctx, q, id.TenantID, sandboxID, SandboxEventRecycle, map[string]any{"reason": reason})
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrSandboxRecycleFail.WithCause(err)
	}
	return s.finalizeSandboxRecycle(ctx, id.TenantID, row, reason)
}

// finalizeSandboxRecycle 完成回收闭环:保存代码、按需快照、删除资源、写 destroyed、发事件。
func (s *Service) finalizeSandboxRecycle(ctx context.Context, tenantID int64, row sqlcgen.Sandbox, reason string) error {
	binding, err := s.runtimeBindingForSandboxRow(ctx, row)
	if err != nil {
		return err
	}
	if err := s.saveFilesForSandbox(ctx, row, binding); err != nil {
		return err
	}
	if row.SnapshotEnabled {
		snapshot, err := s.orchestrator.SnapshotWorkspace(ctx, SnapshotSpec{
			SandboxID: row.ID,
			TenantID:  tenantID,
			Namespace: row.Namespace,
			ExpiresAt: timex.FromTimestamptz(row.SnapshotExpireAt),
		})
		if err != nil {
			return err
		}
		if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
			_, e := q.UpdateSandboxSnapshot(ctx, sqlcgen.UpdateSandboxSnapshotParams{
				ID: row.ID, SnapshotRef: pgText(snapshot.Ref),
				SnapshotCreatedAt: timex.RequiredTimestamptz(snapshot.CreatedAt), SnapshotExpireAt: timex.RequiredTimestamptz(snapshot.ExpiresAt),
			})
			return e
		}); err != nil {
			return err
		}
	}
	if row.SnapshotEnabled {
		if err := s.orchestrator.Pause(ctx, binding); err != nil {
			return err
		}
	} else {
		if err := s.orchestrator.Recycle(ctx, row.Namespace); err != nil {
			return err
		}
	}
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.DestroySandbox(ctx, row.ID); err != nil {
			return err
		}
		return s.writeEvent(ctx, q, tenantID, row.ID, SandboxEventRecycle, map[string]any{"reason": reason, "status": "destroyed"})
	}); err != nil {
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
	var quota sqlcgen.TenantQuotum
	var active int64
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var e error
		quota, e = q.GetTenantQuota(ctx, id.TenantID)
		if e != nil {
			if db.IsNoRows(e) {
				return apperr.ErrQuotaInvalid
			}
			return e
		}
		active, e = q.CountActiveSandboxes(ctx)
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrQuotaPersistenceFail.WithCause(err)
	}
	return quotaToMap(quota, active), nil
}

// Stats 读取租户沙箱资源统计,供 M9 看板经 contracts 只读聚合。
func (s *Service) Stats(ctx context.Context, tenantID int64) (contracts.SandboxStats, error) {
	if tenantID <= 0 {
		return contracts.SandboxStats{}, apperr.ErrQuotaInvalid
	}
	var quota sqlcgen.TenantQuotum
	var active int64
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		quota, err = q.GetTenantQuota(ctx, tenantID)
		if err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrQuotaInvalid
			}
			return err
		}
		active, err = q.CountActiveSandboxes(ctx)
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return contracts.SandboxStats{}, ae
		}
		return contracts.SandboxStats{}, apperr.ErrQuotaPersistenceFail.WithCause(err)
	}
	return contracts.SandboxStats{
		TenantID:             tenantID,
		ActiveSandboxCount:   active,
		MaxConcurrentSandbox: quota.MaxConcurrentSandbox,
		MaxCPU:               quota.MaxCpu,
		MaxMemoryMB:          quota.MaxMemoryMb,
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
	var quota sqlcgen.TenantQuotum
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var e error
		quota, e = q.UpsertTenantQuota(ctx, sqlcgen.UpsertTenantQuotaParams{
			TenantID: id.TenantID, MaxConcurrentSandbox: req.MaxConcurrentSandbox,
			MaxCpu: req.MaxCPU, MaxMemoryMb: req.MaxMemoryMB,
			IdleTimeoutMin: req.IdleTimeoutMin, MaxLifetimeMin: req.MaxLifetimeMin,
			MaxKeepaliveMin: req.MaxKeepaliveMin, MaxSnapshotRetentionMin: req.MaxSnapshotRetentionMin,
		})
		return e
	}); err != nil {
		return nil, apperr.ErrQuotaPersistenceFail.WithCause(err)
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
	if err := validateSourceRef(req.SourceRef); err != nil {
		return err
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
func (s *Service) sandboxResourceUsageFromRows(runtime sqlcgen.Runtime, tools []sqlcgen.Tool) (sandboxResourceUsage, error) {
	adapterSpec, err := parseRuntimeAdapterSpec(runtime.AdapterSpec)
	if err != nil {
		return sandboxResourceUsage{}, err
	}
	usage, err := s.runtimeResourceUsage(adapterSpec)
	if err != nil {
		return sandboxResourceUsage{}, err
	}
	for _, tool := range tools {
		toolDef := ToolDefinition{ID: tool.ID, Code: tool.Code, Name: tool.Name, Kind: tool.Kind, Port: int4Value(tool.Port), EcoTags: tool.EcoTags}
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
func (s *Service) checkTenantResourceQuota(ctx context.Context, q *sqlcgen.Queries, quota sqlcgen.TenantQuotum, requested sandboxResourceUsage) error {
	rows, err := q.ListActiveSandboxResourceSpecs(ctx)
	if err != nil {
		return err
	}
	active := sandboxResourceUsage{}
	seenRuntime := make(map[int64]struct{})
	for _, row := range rows {
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
		if !row.ToolID.Valid {
			continue
		}
		tool := ToolDefinition{
			ID:      row.ToolID.Int64,
			Code:    row.ToolCode.String,
			Name:    row.ToolName.String,
			Kind:    row.ToolKind.Int16,
			Port:    int4Value(row.ToolPort),
			EcoTags: row.ToolEcoTags.String,
		}
		spec, err := parseToolResourceSpec(tool, row.ToolResourceSpec)
		if err != nil {
			return apperr.ErrQuotaResourceBusy.WithCause(err)
		}
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
	if active.cpuMilli > int64(quota.MaxCpu)*1000 || active.memoryMB > int64(quota.MaxMemoryMb) {
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
func quotaToMap(q sqlcgen.TenantQuotum, active int64) map[string]any {
	return map[string]any{
		"tenant_id":                  ids.Format(q.TenantID),
		"max_concurrent_sandbox":     q.MaxConcurrentSandbox,
		"max_cpu":                    q.MaxCpu,
		"max_memory_mb":              q.MaxMemoryMb,
		"idle_timeout_min":           q.IdleTimeoutMin,
		"max_lifetime_min":           q.MaxLifetimeMin,
		"max_keepalive_min":          q.MaxKeepaliveMin,
		"max_snapshot_retention_min": q.MaxSnapshotRetentionMin,
		"active_sandbox_count":       active,
	}
}

// runtimeToMap 输出运行时配置。
func runtimeToMap(r sqlcgen.Runtime) map[string]any {
	return map[string]any{
		"id":              ids.Format(r.ID),
		"code":            r.Code,
		"name":            r.Name,
		"eco":             r.Eco,
		"adapter_level":   r.AdapterLevel,
		"adapter_spec":    jsonx.ObjectMap(r.AdapterSpec),
		"capability_impl": textValue(r.CapabilityImpl),
		"plugin_ref":      textValue(r.PluginRef),
		"selftest_status": r.SelftestStatus,
		"selftest_detail": jsonx.ObjectMap(r.SelftestDetail),
		"status":          r.Status,
	}
}

// runtimeImageToMap 输出运行时镜像配置。
func runtimeImageToMap(r sqlcgen.RuntimeImage) map[string]any {
	return map[string]any{
		"id":             ids.Format(r.ID),
		"runtime_id":     ids.Format(r.RuntimeID),
		"image_url":      r.ImageUrl,
		"version":        r.Version,
		"prepulled":      r.Prepulled,
		"prepull_status": r.PrepullStatus,
		"prepull_detail": jsonx.ObjectMap(r.PrepullDetail),
		"prepulled_at":   timex.FromTimestamptz(r.PrepulledAt),
		"genesis_baked":  r.GenesisBaked,
		"is_default":     r.IsDefault,
	}
}

// toolToMap 输出工具定义。
func toolToMap(t sqlcgen.Tool) map[string]any {
	return map[string]any{
		"id":            ids.Format(t.ID),
		"code":          t.Code,
		"name":          t.Name,
		"kind":          t.Kind,
		"image_url":     textValue(t.ImageUrl),
		"port":          int4Value(t.Port),
		"eco_tags":      t.EcoTags,
		"resource_spec": jsonx.ObjectMap(t.ResourceSpec),
		"status":        t.Status,
	}
}

// sandboxInfo 聚合沙箱主表与工具接入端点为 contracts DTO。
func (s *Service) sandboxInfo(ctx context.Context, tenantID int64, row sqlcgen.Sandbox) (contracts.SandboxInfo, error) {
	var tools []contracts.SandboxToolAccess
	var image sqlcgen.RuntimeImage
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		image, e = q.GetRuntimeImage(ctx, sqlcgen.GetRuntimeImageParams{ID: row.ImageID, RuntimeID: row.RuntimeID})
		if db.IsNoRows(e) {
			return apperr.ErrRuntimeImageNotFound
		}
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return contracts.SandboxInfo{}, ae
		}
		return contracts.SandboxInfo{}, apperr.ErrSandboxPersistenceFail.WithCause(err)
	}
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		rows, e := q.ListSandboxTools(ctx, row.ID)
		if e != nil {
			return e
		}
		for _, item := range rows {
			tools = append(tools, contracts.SandboxToolAccess{
				ToolCode: item.ToolCode,
				Kind:     item.ToolKind,
				Endpoint: item.AccessEndpoint,
				Status:   item.Status,
			})
		}
		return nil
	}); err != nil {
		return contracts.SandboxInfo{}, apperr.ErrSandboxPersistenceFail.WithCause(err)
	}
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

// capabilityForSandbox 根据沙箱使用的 runtime 找到 L2 能力实现器。
func (s *Service) capabilityForSandbox(ctx context.Context, sandboxID int64) (ChainCapability, SandboxRuntimeBinding, error) {
	if _, ok := tenantFromContext(ctx); !ok {
		return nil, SandboxRuntimeBinding{}, apperr.ErrUnauthorized
	}
	var row sqlcgen.Sandbox
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.GetSandboxByID(ctx, sandboxID)
		if e != nil {
			if db.IsNoRows(e) {
				return apperr.ErrSandboxNotFound
			}
			return e
		}
		if err := validateSandboxSourceRefAccess(ctx, row.SourceRef); err != nil {
			return err
		}
		if err := ensureSandboxInteractive(row.Status); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, SandboxRuntimeBinding{}, ae
		}
		return nil, SandboxRuntimeBinding{}, apperr.ErrSandboxPersistenceFail.WithCause(err)
	}
	var runtime sqlcgen.Runtime
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		runtime, e = q.GetRuntimeByID(ctx, row.RuntimeID)
		return e
	}); err != nil {
		return nil, SandboxRuntimeBinding{}, apperr.ErrRuntimeUnavailable.WithCause(err)
	}
	if !runtime.CapabilityImpl.Valid || strings.TrimSpace(runtime.CapabilityImpl.String) == "" {
		return nil, SandboxRuntimeBinding{}, apperr.ErrRuntimeCapabilityUnavailable
	}
	if s.capabilities == nil {
		return nil, SandboxRuntimeBinding{}, apperr.ErrRuntimeCapabilityUnavailable
	}
	capability, exists := s.capabilities.Get(runtime.CapabilityImpl.String)
	if !exists {
		return nil, SandboxRuntimeBinding{}, apperr.ErrRuntimeCapabilityUnavailable
	}
	spec, err := parseRuntimeAdapterSpec(runtime.AdapterSpec)
	if err != nil {
		return nil, SandboxRuntimeBinding{}, err
	}
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
	return s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.UpdateSandboxPhaseStatus(ctx, sqlcgen.UpdateSandboxPhaseStatusParams{
			ID: sandboxID, Phase: SandboxPhaseAllocating, Status: SandboxStatusError,
		}); err != nil {
			return err
		}
		return s.writeEvent(ctx, q, tenantID, sandboxID, SandboxEventError, map[string]any{"error": cause.Error()})
	})
}

// writeEvent 写沙箱生命周期事件,detail 使用 JSONB 保存结构化上下文。
func (s *Service) writeEvent(ctx context.Context, q *sqlcgen.Queries, tenantID, sandboxID int64, eventType string, detail map[string]any) error {
	data, err := jsonx.ObjectBytes(detail, apperr.ErrSandboxInvalidState)
	if err != nil {
		return err
	}
	return q.CreateSandboxEvent(ctx, sqlcgen.CreateSandboxEventParams{
		ID: s.idgen.Generate(), TenantID: tenantID, SandboxID: sandboxID, EventType: eventType, Detail: data,
	})
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

// validateSourceRef 校验来源标识格式,但不解析业务语义。
func validateSourceRef(ref string) error {
	parts := strings.Split(ref, ":")
	if len(parts) != 4 {
		return apperr.ErrSandboxRequestInvalid
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return apperr.ErrSandboxRequestInvalid
		}
	}
	return nil
}

// validateSandboxSourceRefAccess 校验服务间签名绑定的 source_ref 与沙箱归属一致。
func validateSandboxSourceRefAccess(ctx context.Context, sourceRef string) error {
	signedSourceRef, ok := auth.ServiceSourceRefFromContext(ctx)
	if !ok {
		return nil
	}
	if signedSourceRef != sourceRef {
		return apperr.ErrSandboxAccessDenied
	}
	return nil
}

// authorizeSandboxRowAccess 统一用户 owner 与内部服务 source_ref 两种沙箱归属校验。
func authorizeSandboxRowAccess(ctx context.Context, id tenant.Identity, row sqlcgen.Sandbox) error {
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

// pgText 把空字符串映射为 SQL NULL,保持 sqlc 参数构造集中。
func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// pgInt4 构造可选 int4 字段,用于资源配额等可空配置。
func pgInt4(v int32, valid bool) pgtype.Int4 {
	return pgtype.Int4{Int32: v, Valid: valid}
}

// textValue 将 SQL text 可空值转为 API 字符串,无效值保持空字符串。
func textValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

// int4Value 将 SQL int4 可空值转为 API 数值,无效值保持 0。
func int4Value(v pgtype.Int4) int32 {
	if !v.Valid {
		return 0
	}
	return v.Int32
}

// buildSandboxCreateSpec 把数据库行与声明式 spec 聚合成编排输入。
func (s *Service) buildSandboxCreateSpec(
	runtime sqlcgen.Runtime,
	image sqlcgen.RuntimeImage,
	tools []sqlcgen.Tool,
	row sqlcgen.Sandbox,
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
			CapabilityImpl: textValue(runtime.CapabilityImpl),
			AdapterSpec:    adapterSpec,
		},
		Image: RuntimeImageDefinition{
			ID:           image.ID,
			ImageURL:     image.ImageUrl,
			Version:      image.Version,
			GenesisBaked: image.GenesisBaked,
		},
		InitCodeRef:              initCodeRef,
		InitScriptRef:            textValue(row.InitScriptRef),
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
			Port:    int4Value(tool.Port),
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
			Port:     int4Value(tool.Port),
			EcoTags:  tool.EcoTags,
			ImageURL: textValue(tool.ImageUrl),
			Spec:     resourceSpec,
		})
	}
	return out, nil
}

// minutesBetween 把两个 timestamptz 字段转换为配置分钟数;任一缺失返回 0。
func minutesBetween(start, end pgtype.Timestamptz) int32 {
	if !start.Valid || !end.Valid {
		return 0
	}
	minutes := int32(end.Time.Sub(start.Time).Minutes())
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
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.UpdateSandboxPhaseStatus(ctx, sqlcgen.UpdateSandboxPhaseStatusParams{
			ID: sandboxID, Phase: phase, Status: status,
		}); err != nil {
			return err
		}
		return s.writeEvent(ctx, q, tenantID, sandboxID, SandboxEventPhaseChange, map[string]any{
			"phase":   phase,
			"status":  status,
			"stage":   stage,
			"message": message,
		})
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
