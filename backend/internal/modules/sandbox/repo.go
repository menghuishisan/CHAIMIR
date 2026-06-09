// M2 数据访问层(repo):只读写 sandbox 模块自有表,全部经 sqlc 生成查询。
package sandbox

import (
	"context"
	"time"

	"chaimir/internal/modules/sandbox/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// repo 封装 sandbox 模块数据库事务入口。
type repo struct {
	db *db.DB
}

// newRepo 绑定平台数据库入口,所有查询仍通过显式事务方法进入。
func newRepo(database *db.DB) *repo {
	return &repo{db: database}
}

// queryFunc 是在 sqlc 查询对象上执行的事务闭包。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 从 ctx 取租户并注入 RLS 后执行查询。
func (r *repo) inTenant(ctx context.Context, fn queryFunc) error {
	return r.db.WithTenantTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inTenantID 用显式租户 ID 注入 RLS,供内部 contracts 调用使用。
func (r *repo) inTenantID(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inApp 访问 runtime/tool 等无 RLS 平台级配置表。
func (r *repo) inApp(ctx context.Context, fn queryFunc) error {
	return r.db.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inMaintenancePrivileged 仅供 M2 后台维护任务扫描本模块 RLS 表候选行。
func (r *repo) inMaintenancePrivileged(ctx context.Context, fn queryFunc) error {
	return r.db.WithPrivilegedModuleTx(ctx, "sandbox", func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// listRuntimes 读取平台级运行时配置列表。
func (r *repo) listRuntimes(ctx context.Context, limit, offset int32) ([]RuntimeConfigSnapshot, error) {
	var rows []sqlcgen.Runtime
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListRuntimes(ctx, sqlcgen.ListRuntimesParams{Limit: limit, Offset: offset})
		return err
	}); err != nil {
		return nil, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	return runtimeConfigsFromRows(rows), nil
}

// createRuntime 写入运行时控制面配置。
func (r *repo) createRuntime(ctx context.Context, id int64, req CreateRuntimeRequest, spec []byte) (RuntimeConfigSnapshot, error) {
	var row sqlcgen.Runtime
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.CreateRuntime(ctx, sqlcgen.CreateRuntimeParams{
			ID:             id,
			Code:           req.Code,
			Name:           req.Name,
			Eco:            req.Eco,
			AdapterLevel:   req.AdapterLevel,
			AdapterSpec:    spec,
			CapabilityImpl: pgtypex.Text(req.CapabilityImpl),
			PluginRef:      pgtypex.Text(req.PluginRef),
			SelftestStatus: RuntimeSelftestPending,
			SelftestDetail: []byte("{}"),
			Status:         RuntimeStatusOnboarding,
		})
		return err
	}); err != nil {
		return RuntimeConfigSnapshot{}, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	return runtimeConfigFromRow(row), nil
}

// createRuntimeImage 校验运行时存在后写入镜像版本。
func (r *repo) createRuntimeImage(ctx context.Context, id, runtimeID int64, req CreateRuntimeImageRequest) (RuntimeImageSnapshot, error) {
	var row sqlcgen.RuntimeImage
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetRuntimeByID(ctx, runtimeID); err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrRuntimeNotFound
			}
			return err
		}
		var err error
		row, err = q.CreateRuntimeImage(ctx, sqlcgen.CreateRuntimeImageParams{
			ID:            id,
			RuntimeID:     runtimeID,
			ImageUrl:      req.ImageURL,
			Version:       req.Version,
			Prepulled:     false,
			PrepullStatus: RuntimeImagePrepullPending,
			PrepullDetail: []byte("{}"),
			GenesisBaked:  req.GenesisBaked,
			IsDefault:     req.IsDefault,
		})
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return RuntimeImageSnapshot{}, ae
		}
		return RuntimeImageSnapshot{}, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	return runtimeImageFromRow(row), nil
}

// listTools 读取平台级工具定义列表。
func (r *repo) listTools(ctx context.Context, limit, offset int32) ([]ToolConfigSnapshot, error) {
	var rows []sqlcgen.Tool
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListTools(ctx, sqlcgen.ListToolsParams{Limit: limit, Offset: offset})
		return err
	}); err != nil {
		return nil, apperr.ErrToolPersistenceFail.WithCause(err)
	}
	return toolConfigsFromRows(rows), nil
}

// createTool 写入平台级工具定义。
func (r *repo) createTool(ctx context.Context, id int64, req CreateToolRequest, spec []byte) (ToolConfigSnapshot, error) {
	var row sqlcgen.Tool
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.CreateTool(ctx, sqlcgen.CreateToolParams{
			ID:           id,
			Code:         req.Code,
			Name:         req.Name,
			Kind:         req.Kind,
			ImageUrl:     pgtypex.Text(req.ImageURL),
			Port:         pgtypex.Int4When(req.Port, req.Port > 0),
			EcoTags:      req.EcoTags,
			ResourceSpec: spec,
			Status:       ToolStatusAvailable,
		})
		return err
	}); err != nil {
		return ToolConfigSnapshot{}, apperr.ErrToolPersistenceFail.WithCause(err)
	}
	return toolConfigFromRow(row), nil
}

// getSandbox 读取租户内沙箱生命周期投影。
func (r *repo) getSandbox(ctx context.Context, sandboxID int64) (SandboxLifecycleSnapshot, error) {
	var row sqlcgen.Sandbox
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetSandboxByID(ctx, sandboxID)
		if db.IsNoRows(err) {
			return apperr.ErrSandboxNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return SandboxLifecycleSnapshot{}, ae
		}
		return SandboxLifecycleSnapshot{}, apperr.ErrSandboxPersistenceFail.WithCause(err)
	}
	return sandboxLifecycleFromRow(row), nil
}

// listSandboxToolAccess 读取沙箱挂载工具端点。
func (r *repo) listSandboxToolAccess(ctx context.Context, tenantID, sandboxID int64) ([]SandboxToolAccessSnapshot, error) {
	var rows []sqlcgen.ListSandboxToolsRow
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListSandboxTools(ctx, sandboxID)
		return err
	}); err != nil {
		return nil, apperr.ErrSandboxPersistenceFail.WithCause(err)
	}
	return sandboxToolAccessesFromRows(rows), nil
}

// getSandboxToolForProxy 读取沙箱工具代理所需的工具端点投影。
func (r *repo) getSandboxToolForProxy(ctx context.Context, tenantID, sandboxID int64, toolCode string) (SandboxToolAccessSnapshot, error) {
	var row sqlcgen.GetSandboxToolForProxyRow
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetSandboxToolForProxy(ctx, sqlcgen.GetSandboxToolForProxyParams{
			SandboxID: sandboxID,
			Code:      toolCode,
		})
		if db.IsNoRows(err) {
			return apperr.ErrToolNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return SandboxToolAccessSnapshot{}, ae
		}
		return SandboxToolAccessSnapshot{}, apperr.ErrToolProxyFail.WithCause(err)
	}
	return sandboxToolProxyFromRow(row), nil
}

// getSandboxDependencies 读取恢复沙箱所需的运行时、镜像和工具定义。
func (r *repo) getSandboxDependencies(ctx context.Context, row SandboxLifecycleSnapshot) (RuntimeConfigSnapshot, RuntimeImageSnapshot, []ToolConfigSnapshot, error) {
	var runtime sqlcgen.Runtime
	var image sqlcgen.RuntimeImage
	var toolRows []sqlcgen.ListSandboxToolsRow
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		runtime, err = q.GetRuntimeByID(ctx, row.RuntimeID)
		if err != nil {
			return err
		}
		image, err = q.GetRuntimeImage(ctx, sqlcgen.GetRuntimeImageParams{ID: row.ImageID, RuntimeID: row.RuntimeID})
		if db.IsNoRows(err) {
			return apperr.ErrRuntimeImageNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return RuntimeConfigSnapshot{}, RuntimeImageSnapshot{}, nil, ae
		}
		return RuntimeConfigSnapshot{}, RuntimeImageSnapshot{}, nil, apperr.ErrRuntimeUnavailable.WithCause(err)
	}
	if err := r.inTenantID(ctx, row.TenantID, func(q *sqlcgen.Queries) error {
		var err error
		toolRows, err = q.ListSandboxTools(ctx, row.ID)
		return err
	}); err != nil {
		return RuntimeConfigSnapshot{}, RuntimeImageSnapshot{}, nil, apperr.ErrSandboxPersistenceFail.WithCause(err)
	}
	tools := make([]sqlcgen.Tool, 0, len(toolRows))
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		for _, item := range toolRows {
			tool, err := q.GetToolByCode(ctx, item.ToolCode)
			if err != nil {
				return err
			}
			tools = append(tools, tool)
		}
		return nil
	}); err != nil {
		return RuntimeConfigSnapshot{}, RuntimeImageSnapshot{}, nil, apperr.ErrToolNotFound.WithCause(err)
	}
	return runtimeConfigFromRow(runtime), runtimeImageFromRow(image), toolConfigsFromRows(tools), nil
}

// getRuntimeSelectionByCode 读取创建沙箱时请求的运行时、镜像和工具定义。
func (r *repo) getRuntimeSelectionByCode(ctx context.Context, runtimeCode, imageVersion string, toolCodes []string) (RuntimeConfigSnapshot, RuntimeImageSnapshot, []ToolConfigSnapshot, error) {
	var runtime sqlcgen.Runtime
	var image sqlcgen.RuntimeImage
	tools := make([]sqlcgen.Tool, 0, len(toolCodes))
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		runtime, err = q.GetRuntimeByCode(ctx, runtimeCode)
		if db.IsNoRows(err) {
			return apperr.ErrRuntimeNotFound
		}
		if err != nil {
			return err
		}
		if imageVersion != "" {
			image, err = q.GetRuntimeImageByVersion(ctx, sqlcgen.GetRuntimeImageByVersionParams{
				RuntimeID: runtime.ID,
				Version:   imageVersion,
			})
		} else {
			image, err = q.GetDefaultRuntimeImage(ctx, runtime.ID)
		}
		if db.IsNoRows(err) {
			return apperr.ErrRuntimeImageNotFound
		}
		if err != nil {
			return err
		}
		for _, code := range toolCodes {
			tool, err := q.GetToolByCode(ctx, code)
			if db.IsNoRows(err) {
				return apperr.ErrToolNotFound
			}
			if err != nil {
				return err
			}
			tools = append(tools, tool)
		}
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return RuntimeConfigSnapshot{}, RuntimeImageSnapshot{}, nil, ae
		}
		return RuntimeConfigSnapshot{}, RuntimeImageSnapshot{}, nil, apperr.ErrSandboxCreateFail.WithCause(err)
	}
	return runtimeConfigFromRow(runtime), runtimeImageFromRow(image), toolConfigsFromRows(tools), nil
}

// createSandboxControlPlane 在一个租户事务内完成配额读取、业务校验和控制面记录创建。
func (r *repo) createSandboxControlPlane(
	ctx context.Context,
	record SandboxCreateRecord,
	tools []SandboxToolCreateRecord,
	eventID int64,
	eventDetail []byte,
	checkQuota func(TenantQuotaSnapshot, int64, []ActiveSandboxResourceSnapshot) (time.Time, error),
) (SandboxLifecycleSnapshot, error) {
	var row sqlcgen.Sandbox
	if err := r.inTenantID(ctx, record.TenantID, func(q *sqlcgen.Queries) error {
		quota, err := q.GetTenantQuota(ctx, record.TenantID)
		if db.IsNoRows(err) {
			return apperr.ErrQuotaInvalid
		}
		if err != nil {
			return err
		}
		active, err := q.CountActiveSandboxes(ctx)
		if err != nil {
			return err
		}
		resourceRows, err := q.ListActiveSandboxResourceSpecs(ctx)
		if err != nil {
			return err
		}
		expireAt, err := checkQuota(tenantQuotaFromRow(quota), active, activeSandboxResourcesFromRows(resourceRows))
		if err != nil {
			return err
		}
		record.ExpireAt = expireAt
		row, err = q.CreateSandbox(ctx, sqlcgen.CreateSandboxParams{
			ID:               record.ID,
			TenantID:         record.TenantID,
			RuntimeID:        record.RuntimeID,
			ImageID:          record.ImageID,
			Namespace:        record.Namespace,
			SourceRef:        record.SourceRef,
			OwnerAccountID:   record.OwnerAccountID,
			Phase:            SandboxPhaseAllocating,
			Status:           SandboxStatusCreating,
			KeepAlive:        record.KeepAlive,
			SnapshotEnabled:  record.SnapshotEnabled,
			CodeStorageKey:   record.CodeStorageKey,
			InitScriptRef:    pgtypex.Text(record.InitScriptRef),
			KeepAliveUntil:   timestamptzFromTime(record.KeepAliveUntil),
			SnapshotExpireAt: timestamptzFromTime(record.SnapshotExpireAt),
			ExpireAt:         timex.RequiredTimestamptz(record.ExpireAt),
		})
		if err != nil {
			return err
		}
		for _, tool := range tools {
			if _, err = q.CreateSandboxTool(ctx, sqlcgen.CreateSandboxToolParams{
				ID:             tool.ID,
				TenantID:       tool.TenantID,
				SandboxID:      tool.SandboxID,
				ToolID:         tool.ToolID,
				AccessEndpoint: tool.AccessEndpoint,
				Status:         tool.Status,
			}); err != nil {
				return err
			}
		}
		return q.CreateSandboxEvent(ctx, sqlcgen.CreateSandboxEventParams{
			ID:        eventID,
			TenantID:  record.TenantID,
			SandboxID: record.ID,
			EventType: SandboxEventCreate,
			Detail:    eventDetail,
		})
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return SandboxLifecycleSnapshot{}, ae
		}
		return SandboxLifecycleSnapshot{}, apperr.ErrSandboxCreateFail.WithCause(err)
	}
	return sandboxLifecycleFromRow(row), nil
}

// updateSandboxCodeHash 写回沙箱代码归档哈希。
func (r *repo) updateSandboxCodeHash(ctx context.Context, tenantID, sandboxID int64, codeHash string) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.UpdateSandboxCodeHash(ctx, sqlcgen.UpdateSandboxCodeHashParams{ID: sandboxID, CodeHash: pgtypex.Text(codeHash)})
		return err
	})
}

// markSandboxActive 写回最近活跃时间,供空闲回收扫描使用。
func (r *repo) markSandboxActive(ctx context.Context, tenantID, sandboxID int64) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		return q.MarkSandboxActive(ctx, sandboxID)
	})
}

// updateSandboxPhaseStatus 写回沙箱阶段和生命周期状态。
func (r *repo) updateSandboxPhaseStatus(ctx context.Context, tenantID, sandboxID int64, phase, status int16) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.UpdateSandboxPhaseStatus(ctx, sqlcgen.UpdateSandboxPhaseStatusParams{
			ID: sandboxID, Phase: phase, Status: status,
		})
		return err
	})
}

// listSandboxesBySourceRef 读取同一来源标识下的沙箱生命周期投影。
func (r *repo) listSandboxesBySourceRef(ctx context.Context, tenantID int64, sourceRef string) ([]SandboxLifecycleSnapshot, error) {
	var rows []sqlcgen.Sandbox
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListSandboxesBySourceRef(ctx, sourceRef)
		return err
	}); err != nil {
		return nil, apperr.ErrSandboxRecycleFail.WithCause(err)
	}
	return sandboxLifecyclesFromRows(rows), nil
}

// recycleSandbox 把单个沙箱置为回收中。
func (r *repo) recycleSandbox(ctx context.Context, tenantID, sandboxID int64) (SandboxLifecycleSnapshot, error) {
	var row sqlcgen.Sandbox
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.RecycleSandbox(ctx, sandboxID)
		return err
	}); err != nil {
		return SandboxLifecycleSnapshot{}, apperr.ErrSandboxRecycleFail.WithCause(err)
	}
	return sandboxLifecycleFromRow(row), nil
}

// updateSandboxSnapshot 写入回收前生成的快照引用。
func (r *repo) updateSandboxSnapshot(ctx context.Context, tenantID, sandboxID int64, snapshot SnapshotResult) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.UpdateSandboxSnapshot(ctx, sqlcgen.UpdateSandboxSnapshotParams{
			ID:                sandboxID,
			SnapshotRef:       pgtypex.Text(snapshot.Ref),
			SnapshotCreatedAt: timex.RequiredTimestamptz(snapshot.CreatedAt),
			SnapshotExpireAt:  timex.RequiredTimestamptz(snapshot.ExpiresAt),
		})
		return err
	})
}

// destroySandbox 写入沙箱回收终态。
func (r *repo) destroySandbox(ctx context.Context, tenantID, sandboxID int64) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.DestroySandbox(ctx, sandboxID)
		return err
	})
}

// getTenantQuotaWithActiveCount 读取租户配额和当前活跃沙箱数量。
func (r *repo) getTenantQuotaWithActiveCount(ctx context.Context, tenantID int64) (TenantQuotaSnapshot, int64, error) {
	var quota sqlcgen.TenantQuotum
	var active int64
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		quota, err = q.GetTenantQuota(ctx, tenantID)
		if db.IsNoRows(err) {
			return apperr.ErrQuotaInvalid
		}
		if err != nil {
			return err
		}
		active, err = q.CountActiveSandboxes(ctx)
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return TenantQuotaSnapshot{}, 0, ae
		}
		return TenantQuotaSnapshot{}, 0, apperr.ErrQuotaPersistenceFail.WithCause(err)
	}
	return tenantQuotaFromRow(quota), active, nil
}

// upsertTenantQuota 写入租户配额。
func (r *repo) upsertTenantQuota(ctx context.Context, tenantID int64, req QuotaRequest) (TenantQuotaSnapshot, error) {
	var quota sqlcgen.TenantQuotum
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		quota, err = q.UpsertTenantQuota(ctx, sqlcgen.UpsertTenantQuotaParams{
			TenantID:                tenantID,
			MaxConcurrentSandbox:    req.MaxConcurrentSandbox,
			MaxCpu:                  req.MaxCPU,
			MaxMemoryMb:             req.MaxMemoryMB,
			IdleTimeoutMin:          req.IdleTimeoutMin,
			MaxLifetimeMin:          req.MaxLifetimeMin,
			MaxKeepaliveMin:         req.MaxKeepaliveMin,
			MaxSnapshotRetentionMin: req.MaxSnapshotRetentionMin,
		})
		return err
	}); err != nil {
		return TenantQuotaSnapshot{}, apperr.ErrQuotaPersistenceFail.WithCause(err)
	}
	return tenantQuotaFromRow(quota), nil
}

// getRuntime 读取平台级运行时配置。
func (r *repo) getRuntime(ctx context.Context, runtimeID int64) (RuntimeConfigSnapshot, error) {
	var row sqlcgen.Runtime
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetRuntimeByID(ctx, runtimeID)
		if db.IsNoRows(err) {
			return apperr.ErrRuntimeNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return RuntimeConfigSnapshot{}, ae
		}
		return RuntimeConfigSnapshot{}, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	return runtimeConfigFromRow(row), nil
}

// getRuntimeWithDefaultImage 读取运行时及其默认镜像。
func (r *repo) getRuntimeWithDefaultImage(ctx context.Context, runtimeID int64) (RuntimeConfigSnapshot, RuntimeImageSnapshot, error) {
	var runtime sqlcgen.Runtime
	var image sqlcgen.RuntimeImage
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		runtime, err = q.GetRuntimeByID(ctx, runtimeID)
		if db.IsNoRows(err) {
			return apperr.ErrRuntimeNotFound
		}
		if err != nil {
			return err
		}
		image, err = q.GetDefaultRuntimeImage(ctx, runtime.ID)
		if db.IsNoRows(err) {
			return apperr.ErrRuntimeImageNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return RuntimeConfigSnapshot{}, RuntimeImageSnapshot{}, ae
		}
		return RuntimeConfigSnapshot{}, RuntimeImageSnapshot{}, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	return runtimeConfigFromRow(runtime), runtimeImageFromRow(image), nil
}

// updateRuntime 更新平台级运行时配置。
func (r *repo) updateRuntime(ctx context.Context, runtimeID int64, req UpdateRuntimeRequest, spec []byte) (RuntimeConfigSnapshot, error) {
	var row sqlcgen.Runtime
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetRuntimeByID(ctx, runtimeID); err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrRuntimeNotFound
			}
			return err
		}
		var err error
		row, err = q.UpdateRuntime(ctx, sqlcgen.UpdateRuntimeParams{
			ID:             runtimeID,
			Name:           req.Name,
			Eco:            req.Eco,
			AdapterLevel:   req.AdapterLevel,
			AdapterSpec:    spec,
			CapabilityImpl: pgtypex.Text(req.CapabilityImpl),
			PluginRef:      pgtypex.Text(req.PluginRef),
			Status:         req.Status,
		})
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return RuntimeConfigSnapshot{}, ae
		}
		return RuntimeConfigSnapshot{}, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	return runtimeConfigFromRow(row), nil
}

// updateRuntimeSelftest 写回运行时自检结果。
func (r *repo) updateRuntimeSelftest(ctx context.Context, runtimeID int64, selftestStatus, runtimeStatus int16, detail []byte) (RuntimeConfigSnapshot, error) {
	var row sqlcgen.Runtime
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.UpdateRuntimeSelftest(ctx, sqlcgen.UpdateRuntimeSelftestParams{
			ID:             runtimeID,
			SelftestStatus: selftestStatus,
			SelftestDetail: detail,
			Status:         runtimeStatus,
		})
		return err
	}); err != nil {
		return RuntimeConfigSnapshot{}, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	return runtimeConfigFromRow(row), nil
}

// getRuntimeImage 读取运行时镜像并校验归属。
func (r *repo) getRuntimeImage(ctx context.Context, runtimeID, imageID int64) (RuntimeImageSnapshot, error) {
	var row sqlcgen.RuntimeImage
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetRuntimeImage(ctx, sqlcgen.GetRuntimeImageParams{ID: imageID, RuntimeID: runtimeID})
		if db.IsNoRows(err) {
			return apperr.ErrRuntimeImageNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return RuntimeImageSnapshot{}, ae
		}
		return RuntimeImageSnapshot{}, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	return runtimeImageFromRow(row), nil
}

// getRuntimeAndImageForPrepull 读取预拉取需要的运行时和镜像,并校验归属。
func (r *repo) getRuntimeAndImageForPrepull(ctx context.Context, runtimeID, imageID int64) (RuntimeImageSnapshot, error) {
	var image sqlcgen.RuntimeImage
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetRuntimeByID(ctx, runtimeID); err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrRuntimeNotFound
			}
			return err
		}
		var err error
		image, err = q.GetRuntimeImage(ctx, sqlcgen.GetRuntimeImageParams{ID: imageID, RuntimeID: runtimeID})
		if db.IsNoRows(err) {
			return apperr.ErrRuntimeImageNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return RuntimeImageSnapshot{}, ae
		}
		return RuntimeImageSnapshot{}, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	return runtimeImageFromRow(image), nil
}

// updateRuntimeImagePrepull 写回镜像预拉取状态。
func (r *repo) updateRuntimeImagePrepull(ctx context.Context, runtimeID, imageID int64, prepulled bool, prepullStatus int16, detail []byte, prepulledAt *time.Time) (RuntimeImageSnapshot, error) {
	var at pgtype.Timestamptz
	if prepulledAt != nil {
		at = timex.RequiredTimestamptz(*prepulledAt)
	}
	var row sqlcgen.RuntimeImage
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.UpdateRuntimeImagePrepull(ctx, sqlcgen.UpdateRuntimeImagePrepullParams{
			ID:            imageID,
			RuntimeID:     runtimeID,
			Prepulled:     prepulled,
			PrepullStatus: prepullStatus,
			PrepullDetail: detail,
			PrepulledAt:   at,
		})
		return err
	}); err != nil {
		return RuntimeImageSnapshot{}, err
	}
	return runtimeImageFromRow(row), nil
}

// listDueSandboxRecycles 锁定本模块自有表中的回收候选沙箱。
func (r *repo) listDueSandboxRecycles(ctx context.Context, limit, readyIdleTimeout int32) ([]SandboxLifecycleSnapshot, error) {
	var rows []sqlcgen.Sandbox
	if err := r.inMaintenancePrivileged(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListDueSandboxRecycles(ctx, sqlcgen.ListDueSandboxRecyclesParams{
			Limit:                   limit,
			ReadyIdleTimeoutSeconds: readyIdleTimeout,
		})
		if err != nil {
			return err
		}
		rows = found
		return nil
	}); err != nil {
		return nil, apperr.ErrSandboxRecycleScanFail.WithCause(err)
	}
	return sandboxLifecyclesFromRows(rows), nil
}

// listExpiredSandboxSnapshots 锁定到期快照保留 Namespace 的清理候选。
func (r *repo) listExpiredSandboxSnapshots(ctx context.Context, limit int32) ([]SandboxLifecycleSnapshot, error) {
	var rows []sqlcgen.Sandbox
	if err := r.inMaintenancePrivileged(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListExpiredSandboxSnapshots(ctx, limit)
		if err != nil {
			return err
		}
		rows = found
		return nil
	}); err != nil {
		return nil, apperr.ErrSandboxSnapshotCleanupFail.WithCause(err)
	}
	return sandboxLifecyclesFromRows(rows), nil
}

// createSandboxEvent 写入 M2 私有技术事件表。
func (r *repo) createSandboxEvent(ctx context.Context, tenantID, sandboxID, eventID int64, eventType string, detail []byte) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		return q.CreateSandboxEvent(ctx, sqlcgen.CreateSandboxEventParams{
			ID:        eventID,
			TenantID:  tenantID,
			SandboxID: sandboxID,
			EventType: eventType,
			Detail:    detail,
		})
	})
}

// tenantFromContext 读取当前租户身份。
func tenantFromContext(ctx context.Context) (tenant.Identity, bool) {
	return tenant.FromContext(ctx)
}

// timestamptzFromTime 把可选时间转换成数据库 timestamptz。
func timestamptzFromTime(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return timex.RequiredTimestamptz(t)
}
