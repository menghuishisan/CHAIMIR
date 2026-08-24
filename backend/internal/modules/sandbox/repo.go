// sandbox repo 文件定义 M2 持久化接口和数据库事务边界,是 service 访问数据库的唯一入口。
package sandbox

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/modules/sandbox/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"

	"github.com/jackc/pgx/v5"
)

// Store 定义 service 所需的 sandbox 持久化能力,不暴露 sqlc 行类型。
type Store interface {
	PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error
	TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error
	PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error
}

// TxStore 定义单个事务内可调用的 sandbox 数据访问能力。
type TxStore interface {
	GetRuntimeByCode(ctx context.Context, code string) (Runtime, error)
	GetRuntimeByID(ctx context.Context, id int64) (Runtime, error)
	GetRuntimeByIDForUpdate(ctx context.Context, id int64) (Runtime, error)
	ListRuntimes(ctx context.Context) ([]Runtime, error)
	UpsertRuntime(ctx context.Context, id int64, req RuntimeRequest, spec AdapterSpec, selftestStatus int16, selftestDetail []byte) (Runtime, error)
	StartRuntimeSelftest(ctx context.Context, runtimeID int64, detail []byte) (Runtime, error)
	FinishRuntimeSelftest(ctx context.Context, runtimeID int64, attemptID string, status, runtimeStatus int16, detail []byte) (Runtime, error)
	GetRuntimeImageByID(ctx context.Context, runtimeID, imageID int64) (RuntimeImage, error)
	GetRuntimeImageByIDForUpdate(ctx context.Context, runtimeID, imageID int64) (RuntimeImage, error)
	GetRuntimeImageByVersion(ctx context.Context, runtimeID int64, version string) (RuntimeImage, error)
	GetRuntimeImageByVersionForShare(ctx context.Context, runtimeID int64, version string) (RuntimeImage, error)
	ListRuntimeImages(ctx context.Context, runtimeID int64) ([]RuntimeImage, error)
	CreateRuntimeImage(ctx context.Context, id, runtimeID int64, req RuntimeImageRequest) (RuntimeImage, error)
	GetPublishedCompositionSnapshot(ctx context.Context, compositionDigest string) ([]byte, error)
	UpsertPublishedCompositionSnapshot(ctx context.Context, compositionDigest string, snapshot []byte) error
	GetCompositionPrepullForUpdate(ctx context.Context, runtimeImageID int64, compositionDigest string) (CompositionPrepull, error)
	StartCompositionPrepull(ctx context.Context, id, runtimeImageID int64, compositionDigest, attemptID string, imageClosure, detail []byte) (CompositionPrepull, error)
	FinishCompositionPrepull(ctx context.Context, runtimeImageID int64, compositionDigest, attemptID string, status int16, daemonsetName string, desiredNodes, readyNodes int32, detail []byte) (CompositionPrepull, error)
	InvalidateCompositionPrepullByRuntime(ctx context.Context, runtimeID int64, detail []byte) error
	DeleteCompositionPrepullByRuntimeImage(ctx context.Context, runtimeImageID int64) error
	DisableRuntimeImage(ctx context.Context, runtimeID, imageID int64) (RuntimeImage, error)
	GetToolByCode(ctx context.Context, code string) (Tool, error)
	ListTools(ctx context.Context) ([]Tool, error)
	ListCatalogRuntimes(ctx context.Context) ([]CatalogRuntime, error)
	ListCatalogTools(ctx context.Context) ([]CatalogTool, error)
	UpsertTool(ctx context.Context, id int64, req ToolRequest, spec ToolResourceSpec) (Tool, error)
	GetTenantQuota(ctx context.Context, tenantID int64) (TenantQuota, error)
	EnsureTenantQuota(ctx context.Context, quota TenantQuota) (TenantQuota, error)
	GetTenantQuotaForUpdate(ctx context.Context, tenantID int64) (TenantQuota, error)
	UpsertTenantQuota(ctx context.Context, quota TenantQuota) (TenantQuota, error)
	CountActiveSandboxes(ctx context.Context, tenantID int64) (int64, error)
	CreateSandbox(ctx context.Context, input CreateSandboxInput) (Sandbox, error)
	GetSandbox(ctx context.Context, tenantID, sandboxID int64) (Sandbox, error)
	ClaimSandboxWorkspaceRevision(ctx context.Context, tenantID, sandboxID, expectedRevision int64) (int64, error)
	LockSandboxChain(ctx context.Context, tenantID, sandboxID int64) error
	ListSandboxesBySourceRef(ctx context.Context, tenantID int64, sourceRef string) ([]Sandbox, error)
	ListSandboxesByScopeRef(ctx context.Context, tenantID int64, scopeRef string) ([]Sandbox, error)
	ListRecycleCandidates(ctx context.Context, readyDeadline time.Time, limit int32) ([]Sandbox, error)
	MarkIdleSandboxes(ctx context.Context) ([]Sandbox, error)
	ListSnapshotCleanupCandidates(ctx context.Context, limit int32) ([]Sandbox, error)
	UpdateSandboxPhaseStatus(ctx context.Context, tenantID, sandboxID int64, phase, status int16) (Sandbox, error)
	MarkSandboxActive(ctx context.Context, tenantID, sandboxID int64) (Sandbox, error)
	UpdateSandboxCode(ctx context.Context, tenantID, sandboxID int64, key, hash string) (Sandbox, error)
	UpdateSandboxAuthorizedAccounts(ctx context.Context, tenantID, sandboxID int64, accountIDs []int64) (Sandbox, error)
	UpdateSandboxSnapshot(ctx context.Context, tenantID, sandboxID int64, ref string, domains []string, createdAt, expireAt time.Time) (Sandbox, error)
	CreateSandboxTool(ctx context.Context, id int64, tenantID int64, sandboxID int64, tool Tool, endpoint string, status int16) (SandboxTool, error)
	ListSandboxTools(ctx context.Context, tenantID, sandboxID int64) ([]SandboxTool, error)
	UpdateSandboxToolStatus(ctx context.Context, tenantID, sandboxID int64, tool Tool, endpoint string, status int16) (SandboxTool, error)
	CreateSandboxEvent(ctx context.Context, id, tenantID, sandboxID int64, typ string, detail []byte) error
	CreateSandboxRecycleOutbox(context.Context, int64, Sandbox, string, string, time.Time) (SandboxRecycleOutbox, error)
	ClaimPendingSandboxRecycleOutbox(context.Context, int32, int32, time.Time, time.Time) ([]SandboxRecycleOutbox, error)
	MarkSandboxRecycleOutboxPublished(context.Context, int64, int64, string) (SandboxRecycleOutbox, error)
	MarkSandboxRecycleOutboxFailed(context.Context, int64, int64, string, string) (SandboxRecycleOutbox, error)
	StatsByTenant(ctx context.Context, tenantID int64) (TenantQuota, int64, error)
}

// CreateSandboxInput 描述 repo 创建沙箱记录时需要的完整字段。
type CreateSandboxInput struct {
	ID                  int64
	TenantID            int64
	Namespace           string
	SourceRef           string
	ScopeRef            string
	CompositionDigest   string
	CompositionSnapshot []byte
	AccessProfile       string
	OwnerAccountID      int64
	SharedAccountIDs    []int64
	Phase               int16
	Status              int16
	KeepAlive           bool
	SnapshotEnabled     bool
	CodeStorageKey      string
	CodeHash            string
	InitCodeRef         string
	InitScriptRef       string
	SnapshotRef         string
	SnapshotDomains     []string
	SnapshotCreatedAt   time.Time
	SnapshotExpireAt    time.Time
	KeepAliveUntil      time.Time
	ExpireAt            time.Time
}

type store struct {
	database *db.DB
}

type txStore struct {
	q *sqlcgen.Queries
}

// NewStore 创建 sandbox 模块持久化入口,仅装配层应调用。
func NewStore(database *db.DB) Store {
	return &store{database: database}
}

// PlatformTx 在应用连接中访问运行时、镜像和工具等平台级表。
func (s *store) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("sandbox store 未初始化")
	}
	return s.database.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// TenantTx 在注入 RLS 租户变量后访问租户内沙箱表。
func (s *store) TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("sandbox store 未初始化")
	}
	return s.database.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// PrivilegedTx 用于后台回收扫描本模块租户表候选,不得作为普通业务路径使用。
func (s *store) PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("sandbox store 未初始化")
	}
	return s.database.WithPrivilegedModuleTx(ctx, "sandbox", func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// GetRuntimeByCode 查询运行时定义。
func (s *txStore) GetRuntimeByCode(ctx context.Context, code string) (Runtime, error) {
	row, err := s.q.GetRuntimeByCode(ctx, code)
	if err != nil {
		return Runtime{}, err
	}
	return runtimeFromRow(row)
}

// GetRuntimeByID 按主键查询运行时定义。
func (s *txStore) GetRuntimeByID(ctx context.Context, id int64) (Runtime, error) {
	row, err := s.q.GetRuntimeByID(ctx, id)
	if err != nil {
		return Runtime{}, err
	}
	return runtimeFromRow(row)
}

// GetRuntimeByIDForUpdate 锁定运行时定义,供同一事务精确比较执行契约并更新。
func (s *txStore) GetRuntimeByIDForUpdate(ctx context.Context, id int64) (Runtime, error) {
	row, err := s.q.GetRuntimeByIDForUpdate(ctx, id)
	if err != nil {
		return Runtime{}, err
	}
	return runtimeFromRow(row)
}

// ListRuntimes 查询运行时列表。
func (s *txStore) ListRuntimes(ctx context.Context) ([]Runtime, error) {
	rows, err := s.q.ListRuntimes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Runtime, 0, len(rows))
	for _, row := range rows {
		item, err := runtimeFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// UpsertRuntime 新建或更新运行时声明。
func (s *txStore) UpsertRuntime(ctx context.Context, id int64, req RuntimeRequest, spec AdapterSpec, selftestStatus int16, selftestDetail []byte) (Runtime, error) {
	rawSpec, err := jsonBytes(spec)
	if err != nil {
		return Runtime{}, err
	}
	row, err := s.q.UpsertRuntime(ctx, sqlcgen.UpsertRuntimeParams{
		ID:             id,
		Code:           req.Code,
		Name:           req.Name,
		Eco:            req.Eco,
		AdapterLevel:   req.AdapterLevel,
		AdapterSpec:    rawSpec,
		CapabilityImpl: pgtypex.Text(req.CapabilityImpl),
		PluginRef:      pgtypex.Text(req.PluginRef),
		SelftestStatus: selftestStatus,
		SelftestDetail: selftestDetail,
		Status:         req.Status,
	})
	if err != nil {
		return Runtime{}, err
	}
	return runtimeFromRow(row)
}

// StartRuntimeSelftest 写入自检批次并把运行时切回接入中。
func (s *txStore) StartRuntimeSelftest(ctx context.Context, runtimeID int64, detail []byte) (Runtime, error) {
	row, err := s.q.StartRuntimeSelftest(ctx, sqlcgen.StartRuntimeSelftestParams{
		ID:             runtimeID,
		SelftestDetail: detail,
	})
	if err != nil {
		return Runtime{}, err
	}
	return runtimeFromRow(row)
}

// FinishRuntimeSelftest 只让同一批次在运行时仍处于接入中时写回最终状态。
func (s *txStore) FinishRuntimeSelftest(ctx context.Context, runtimeID int64, attemptID string, status, runtimeStatus int16, detail []byte) (Runtime, error) {
	row, err := s.q.FinishRuntimeSelftest(ctx, sqlcgen.FinishRuntimeSelftestParams{
		ID:             runtimeID,
		SelftestStatus: status,
		SelftestDetail: detail,
		Status:         runtimeStatus,
		AttemptID:      attemptID,
	})
	if err != nil {
		return Runtime{}, err
	}
	return runtimeFromRow(row)
}

// GetRuntimeImageByID 查询指定镜像。
func (s *txStore) GetRuntimeImageByID(ctx context.Context, runtimeID, imageID int64) (RuntimeImage, error) {
	row, err := s.q.GetRuntimeImageByID(ctx, sqlcgen.GetRuntimeImageByIDParams{ID: imageID, RuntimeID: runtimeID})
	if err != nil {
		return RuntimeImage{}, err
	}
	return runtimeImageFromRow(row), nil
}

// GetRuntimeImageByIDForUpdate 锁定预拉取证明行,让闭包变更与预拉取开始形成确定顺序。
func (s *txStore) GetRuntimeImageByIDForUpdate(ctx context.Context, runtimeID, imageID int64) (RuntimeImage, error) {
	row, err := s.q.GetRuntimeImageByIDForUpdate(ctx, sqlcgen.GetRuntimeImageByIDForUpdateParams{ID: imageID, RuntimeID: runtimeID})
	if err != nil {
		return RuntimeImage{}, err
	}
	return runtimeImageFromRow(row), nil
}

// GetRuntimeImageByVersion 按固定版本查询镜像。
func (s *txStore) GetRuntimeImageByVersion(ctx context.Context, runtimeID int64, version string) (RuntimeImage, error) {
	row, err := s.q.GetRuntimeImageByVersion(ctx, sqlcgen.GetRuntimeImageByVersionParams{RuntimeID: runtimeID, Version: version})
	if err != nil {
		return RuntimeImage{}, err
	}
	return runtimeImageFromRow(row), nil
}

// GetRuntimeImageByVersionForShare 锁定显式镜像版本,防止创建事务与预拉取失效交叉放行。
func (s *txStore) GetRuntimeImageByVersionForShare(ctx context.Context, runtimeID int64, version string) (RuntimeImage, error) {
	row, err := s.q.GetRuntimeImageByVersionForShare(ctx, sqlcgen.GetRuntimeImageByVersionForShareParams{RuntimeID: runtimeID, Version: version})
	if err != nil {
		return RuntimeImage{}, err
	}
	return runtimeImageFromRow(row), nil
}

// ListRuntimeImages 查询运行时镜像列表。
func (s *txStore) ListRuntimeImages(ctx context.Context, runtimeID int64) ([]RuntimeImage, error) {
	rows, err := s.q.ListRuntimeImages(ctx, runtimeID)
	if err != nil {
		return nil, err
	}
	out := make([]RuntimeImage, 0, len(rows))
	for _, row := range rows {
		out = append(out, runtimeImageFromRow(row))
	}
	return out, nil
}

// CreateRuntimeImage 新增运行时镜像版本。
func (s *txStore) CreateRuntimeImage(ctx context.Context, id, runtimeID int64, req RuntimeImageRequest) (RuntimeImage, error) {
	row, err := s.q.CreateRuntimeImage(ctx, sqlcgen.CreateRuntimeImageParams{
		ID:           id,
		RuntimeID:    runtimeID,
		ImageUrl:     req.ImageURL,
		Version:      req.Version,
		GenesisBaked: req.GenesisBaked,
	})
	if err != nil {
		return RuntimeImage{}, err
	}
	return runtimeImageFromRow(row), nil
}

// GetPublishedCompositionSnapshot 按摘要读取已发布且不可变的组合快照。
func (s *txStore) GetPublishedCompositionSnapshot(ctx context.Context, compositionDigest string) ([]byte, error) {
	row, err := s.q.GetPublishedCompositionSnapshot(ctx, compositionDigest)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), row.Snapshot...), nil
}

// UpsertPublishedCompositionSnapshot 登记编译后的组合快照,供预拉取和启动阶段复用同一事实来源。
func (s *txStore) UpsertPublishedCompositionSnapshot(ctx context.Context, compositionDigest string, snapshot []byte) error {
	return s.q.UpsertPublishedCompositionSnapshot(ctx, sqlcgen.UpsertPublishedCompositionSnapshotParams{
		CompositionDigest: compositionDigest,
		Snapshot:          snapshot,
	})
}

// GetCompositionPrepullForUpdate 锁定某个组合摘要的预拉取证明。
func (s *txStore) GetCompositionPrepullForUpdate(ctx context.Context, runtimeImageID int64, compositionDigest string) (CompositionPrepull, error) {
	row, err := s.q.GetCompositionPrepullForUpdate(ctx, sqlcgen.GetCompositionPrepullForUpdateParams{RuntimeImageID: runtimeImageID, CompositionDigest: compositionDigest})
	if err != nil {
		return CompositionPrepull{}, err
	}
	return compositionPrepullFromRow(row), nil
}

// StartCompositionPrepull 把已核对的组合闭包标记为进行中。
func (s *txStore) StartCompositionPrepull(ctx context.Context, id, runtimeImageID int64, compositionDigest, attemptID string, imageClosure, detail []byte) (CompositionPrepull, error) {
	row, err := s.q.StartCompositionPrepull(ctx, sqlcgen.StartCompositionPrepullParams{
		ID: id, RuntimeImageID: runtimeImageID, CompositionDigest: compositionDigest,
		AttemptID: attemptID, ImageClosure: imageClosure, Detail: detail,
	})
	if err != nil {
		return CompositionPrepull{}, err
	}
	return compositionPrepullFromRow(row), nil
}

// FinishCompositionPrepull 仅允许同一次预拉取尝试写回最终节点状态。
func (s *txStore) FinishCompositionPrepull(ctx context.Context, runtimeImageID int64, compositionDigest, attemptID string, status int16, daemonsetName string, desiredNodes, readyNodes int32, detail []byte) (CompositionPrepull, error) {
	row, err := s.q.FinishCompositionPrepull(ctx, sqlcgen.FinishCompositionPrepullParams{
		RuntimeImageID: runtimeImageID, CompositionDigest: compositionDigest, AttemptID: attemptID,
		Status: status, DaemonsetName: daemonsetName, DesiredNodes: desiredNodes, ReadyNodes: readyNodes, Detail: detail,
	})
	if err != nil {
		return CompositionPrepull{}, err
	}
	return compositionPrepullFromRow(row), nil
}

// InvalidateCompositionPrepullByRuntime 在运行时工作负载闭包变化后撤销旧组合证明。
func (s *txStore) InvalidateCompositionPrepullByRuntime(ctx context.Context, runtimeID int64, detail []byte) error {
	return s.q.InvalidateCompositionPrepullByRuntime(ctx, sqlcgen.InvalidateCompositionPrepullByRuntimeParams{RuntimeID: runtimeID, Detail: detail})
}

// DeleteCompositionPrepullByRuntimeImage 删除停用镜像关联的组合证明和预拉取资源记录。
func (s *txStore) DeleteCompositionPrepullByRuntimeImage(ctx context.Context, runtimeImageID int64) error {
	return s.q.DeleteCompositionPrepullByRuntimeImage(ctx, runtimeImageID)
}

// DisableRuntimeImage 停用镜像版本,避免新沙箱继续调度该镜像。
func (s *txStore) DisableRuntimeImage(ctx context.Context, runtimeID, imageID int64) (RuntimeImage, error) {
	row, err := s.q.DisableRuntimeImage(ctx, sqlcgen.DisableRuntimeImageParams{
		ID:        imageID,
		RuntimeID: runtimeID,
	})
	if err != nil {
		return RuntimeImage{}, err
	}
	return runtimeImageFromRow(row), nil
}

// GetToolByCode 查询工具定义。
func (s *txStore) GetToolByCode(ctx context.Context, code string) (Tool, error) {
	row, err := s.q.GetToolByCode(ctx, code)
	if err != nil {
		return Tool{}, err
	}
	return toolFromRow(row)
}

// ListTools 查询工具列表。
func (s *txStore) ListTools(ctx context.Context) ([]Tool, error) {
	rows, err := s.q.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Tool, 0, len(rows))
	for _, row := range rows {
		item, err := toolFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// ListCatalogRuntimes 查询编排目录里的可用运行时,并把平铺的镜像行按运行时合并。
// SQL 已按 runtime_code 排序,故顺序扫描即可分组,不需要额外 map。
func (s *txStore) ListCatalogRuntimes(ctx context.Context) ([]CatalogRuntime, error) {
	rows, err := s.q.ListCatalogRuntimes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CatalogRuntime, 0, len(rows))
	for _, row := range rows {
		var capabilities CapabilityConstraints
		if len(row.AdapterSpec) > 0 {
			var envelope struct {
				Capabilities CapabilityConstraints `json:"capabilities"`
			}
			if err := jsonx.DecodeStrictKnownFields(row.AdapterSpec, &envelope); err != nil {
				return nil, err
			}
			capabilities = envelope.Capabilities
		}
		if len(out) == 0 || out[len(out)-1].Code != row.RuntimeCode {
			out = append(out, CatalogRuntime{Code: row.RuntimeCode, Name: row.RuntimeName, Eco: row.Eco, Images: []CatalogRuntimeImage{}, Capabilities: capabilities})
		}
		current := &out[len(out)-1]
		current.Images = append(current.Images, CatalogRuntimeImage{Version: row.ImageVersion})
	}
	return out, nil
}

// ListCatalogTools 查询编排目录里的可用工具。
func (s *txStore) ListCatalogTools(ctx context.Context) ([]CatalogTool, error) {
	rows, err := s.q.ListCatalogTools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CatalogTool, 0, len(rows))
	for _, row := range rows {
		resourceSpec, err := toolResourceSpecFromJSON(row.ResourceSpec)
		if err != nil {
			return nil, err
		}
		out = append(out, CatalogTool{
			Category: row.Category, Code: row.Code, Name: row.Name, Kind: row.Kind,
			EcoTags: splitCSV(row.EcoTags), ResourceSpec: resourceSpec, Capabilities: resourceSpec.Capabilities,
		})
	}
	return out, nil
}

// UpsertTool 新建或更新工具定义。
func (s *txStore) UpsertTool(ctx context.Context, id int64, req ToolRequest, spec ToolResourceSpec) (Tool, error) {
	rawSpec, err := jsonBytes(spec)
	if err != nil {
		return Tool{}, err
	}
	row, err := s.q.UpsertTool(ctx, sqlcgen.UpsertToolParams{
		ID:           id,
		Code:         req.Code,
		Name:         req.Name,
		Kind:         req.Kind,
		EcoTags:      stringsJoin(req.EcoTags),
		ResourceSpec: rawSpec,
		Status:       req.Status,
	})
	if err != nil {
		return Tool{}, err
	}
	return toolFromRow(row)
}

// GetTenantQuota 查询租户资源配额。
func (s *txStore) GetTenantQuota(ctx context.Context, tenantID int64) (TenantQuota, error) {
	row, err := s.q.GetTenantQuota(ctx, tenantID)
	if err != nil {
		return TenantQuota{}, err
	}
	return quotaFromRow(row), nil
}

// EnsureTenantQuota 只在缺失时写入默认配额,冲突时返回现有管理员配置。
func (s *txStore) EnsureTenantQuota(ctx context.Context, quota TenantQuota) (TenantQuota, error) {
	row, err := s.q.EnsureTenantQuota(ctx, sqlcgen.EnsureTenantQuotaParams{
		TenantID:                quota.TenantID,
		MaxConcurrentSandbox:    quota.MaxConcurrentSandbox,
		MaxCpu:                  quota.MaxCPU,
		MaxMemoryMb:             quota.MaxMemoryMB,
		IdleTimeoutMin:          quota.IdleTimeoutMin,
		MaxLifetimeMin:          quota.MaxLifetimeMin,
		MaxKeepaliveMin:         quota.MaxKeepaliveMin,
		MaxSnapshotRetentionMin: quota.MaxSnapshotRetentionMin,
	})
	if err != nil {
		return TenantQuota{}, err
	}
	return TenantQuota{
		TenantID:                row.TenantID,
		MaxConcurrentSandbox:    row.MaxConcurrentSandbox,
		MaxCPU:                  row.MaxCpu,
		MaxMemoryMB:             row.MaxMemoryMb,
		IdleTimeoutMin:          row.IdleTimeoutMin,
		MaxLifetimeMin:          row.MaxLifetimeMin,
		MaxKeepaliveMin:         row.MaxKeepaliveMin,
		MaxSnapshotRetentionMin: row.MaxSnapshotRetentionMin,
	}, nil
}

// GetTenantQuotaForUpdate 查询租户资源配额并加锁,防止创建并发竞态。
func (s *txStore) GetTenantQuotaForUpdate(ctx context.Context, tenantID int64) (TenantQuota, error) {
	row, err := s.q.GetTenantQuotaForUpdate(ctx, tenantID)
	if err != nil {
		return TenantQuota{}, err
	}
	return quotaFromRow(row), nil
}

// UpsertTenantQuota 新建或更新租户资源配额。
func (s *txStore) UpsertTenantQuota(ctx context.Context, quota TenantQuota) (TenantQuota, error) {
	row, err := s.q.UpsertTenantQuota(ctx, sqlcgen.UpsertTenantQuotaParams{
		TenantID:                quota.TenantID,
		MaxConcurrentSandbox:    quota.MaxConcurrentSandbox,
		MaxCpu:                  quota.MaxCPU,
		MaxMemoryMb:             quota.MaxMemoryMB,
		IdleTimeoutMin:          quota.IdleTimeoutMin,
		MaxLifetimeMin:          quota.MaxLifetimeMin,
		MaxKeepaliveMin:         quota.MaxKeepaliveMin,
		MaxSnapshotRetentionMin: quota.MaxSnapshotRetentionMin,
	})
	if err != nil {
		return TenantQuota{}, err
	}
	return quotaFromRow(row), nil
}

// CountActiveSandboxes 统计租户活跃沙箱数。
func (s *txStore) CountActiveSandboxes(ctx context.Context, tenantID int64) (int64, error) {
	return s.q.CountActiveSandboxes(ctx, tenantID)
}

// CreateSandbox 创建沙箱实例记录。
func (s *txStore) CreateSandbox(ctx context.Context, input CreateSandboxInput) (Sandbox, error) {
	snapshotDomains, err := jsonStringArray(input.SnapshotDomains)
	if err != nil {
		return Sandbox{}, fmt.Errorf("编码沙箱快照域失败: %w", err)
	}
	row, err := s.q.CreateSandbox(ctx, sqlcgen.CreateSandboxParams{
		ID:                  input.ID,
		TenantID:            input.TenantID,
		Namespace:           input.Namespace,
		SourceRef:           input.SourceRef,
		ScopeRef:            input.ScopeRef,
		CompositionDigest:   input.CompositionDigest,
		CompositionSnapshot: input.CompositionSnapshot,
		AccessProfile:       input.AccessProfile,
		OwnerAccountID:      input.OwnerAccountID,
		SharedAccountIds:    input.SharedAccountIDs,
		Phase:               input.Phase,
		Status:              input.Status,
		KeepAlive:           input.KeepAlive,
		SnapshotEnabled:     input.SnapshotEnabled,
		CodeStorageKey:      input.CodeStorageKey,
		CodeHash:            pgtypex.Text(input.CodeHash),
		InitCodeRef:         pgtypex.Text(input.InitCodeRef),
		InitScriptRef:       pgtypex.Text(input.InitScriptRef),
		SnapshotRef:         pgtypex.Text(input.SnapshotRef),
		SnapshotDomains:     snapshotDomains,
		SnapshotCreatedAt:   timex.Timestamptz(input.SnapshotCreatedAt),
		SnapshotExpireAt:    timex.Timestamptz(input.SnapshotExpireAt),
		KeepAliveUntil:      timex.Timestamptz(input.KeepAliveUntil),
		ExpireAt:            timex.RequiredTimestamptz(input.ExpireAt),
	})
	if err != nil {
		return Sandbox{}, err
	}
	return sandboxFromRow(row)
}

// GetSandbox 查询单个沙箱。
func (s *txStore) GetSandbox(ctx context.Context, tenantID, sandboxID int64) (Sandbox, error) {
	row, err := s.q.GetSandbox(ctx, sqlcgen.GetSandboxParams{TenantID: tenantID, ID: sandboxID})
	if err != nil {
		return Sandbox{}, err
	}
	out, err := sandboxFromRow(row)
	if err != nil {
		return Sandbox{}, err
	}
	return out, nil
}

// ClaimSandboxWorkspaceRevision 原子比较并前进工作区 revision,阻止并发学生覆盖彼此修改。
func (s *txStore) ClaimSandboxWorkspaceRevision(ctx context.Context, tenantID, sandboxID, expectedRevision int64) (int64, error) {
	row, err := s.q.ClaimSandboxWorkspaceRevision(ctx, sqlcgen.ClaimSandboxWorkspaceRevisionParams{TenantID: tenantID, ID: sandboxID, ExpectedRevision: expectedRevision})
	if err != nil {
		return 0, apperr.ErrConflict.WithCause(err)
	}
	return row, nil
}

// LockSandboxChain 在当前事务内持有单沙箱链操作锁,直到真实能力调用完成后事务提交。
func (s *txStore) LockSandboxChain(ctx context.Context, tenantID, sandboxID int64) error {
	return s.q.LockSandboxChain(ctx, sandboxID)
}

// ListSandboxesBySourceRef 查询来源下未销毁沙箱。
func (s *txStore) ListSandboxesBySourceRef(ctx context.Context, tenantID int64, sourceRef string) ([]Sandbox, error) {
	rows, err := s.q.ListSandboxesBySourceRef(ctx, sqlcgen.ListSandboxesBySourceRefParams{TenantID: tenantID, SourceRef: sourceRef})
	if err != nil {
		return nil, err
	}
	return sandboxRows(rows)
}

// ListSandboxesByScopeRef 查询生命周期作用域下仍未销毁的沙箱。
func (s *txStore) ListSandboxesByScopeRef(ctx context.Context, tenantID int64, scopeRef string) ([]Sandbox, error) {
	rows, err := s.q.ListSandboxesByScopeRef(ctx, sqlcgen.ListSandboxesByScopeRefParams{TenantID: tenantID, ScopeRef: scopeRef})
	if err != nil {
		return nil, err
	}
	items := make([]Sandbox, 0, len(rows))
	for _, row := range rows {
		item, err := sandboxFromRow(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// ListRecycleCandidates 查询需要回收的沙箱候选。
func (s *txStore) ListRecycleCandidates(ctx context.Context, readyDeadline time.Time, limit int32) ([]Sandbox, error) {
	rows, err := s.q.ListRecycleCandidates(ctx, sqlcgen.ListRecycleCandidatesParams{
		LastActiveAt: timex.RequiredTimestamptz(readyDeadline),
		Limit:        limit,
	})
	if err != nil {
		return nil, err
	}
	return sandboxRows(rows)
}

// MarkIdleSandboxes 将超时的运行中沙箱标记为空闲。
func (s *txStore) MarkIdleSandboxes(ctx context.Context) ([]Sandbox, error) {
	rows, err := s.q.MarkIdleSandboxes(ctx)
	if err != nil {
		return nil, err
	}
	return sandboxRows(rows)
}

// ListSnapshotCleanupCandidates 查询快照保留到期候选。
func (s *txStore) ListSnapshotCleanupCandidates(ctx context.Context, limit int32) ([]Sandbox, error) {
	rows, err := s.q.ListSnapshotCleanupCandidates(ctx, limit)
	if err != nil {
		return nil, err
	}
	return sandboxRows(rows)
}

// UpdateSandboxPhaseStatus 更新沙箱阶段和状态。
func (s *txStore) UpdateSandboxPhaseStatus(ctx context.Context, tenantID, sandboxID int64, phase, status int16) (Sandbox, error) {
	row, err := s.q.UpdateSandboxPhaseStatus(ctx, sqlcgen.UpdateSandboxPhaseStatusParams{TenantID: tenantID, ID: sandboxID, Phase: phase, Status: status})
	if err != nil {
		return Sandbox{}, err
	}
	return sandboxFromRow(row)
}

// MarkSandboxActive 更新沙箱最近活跃时间。
func (s *txStore) MarkSandboxActive(ctx context.Context, tenantID, sandboxID int64) (Sandbox, error) {
	row, err := s.q.MarkSandboxActive(ctx, sqlcgen.MarkSandboxActiveParams{TenantID: tenantID, ID: sandboxID})
	if err != nil {
		return Sandbox{}, err
	}
	return sandboxFromRow(row)
}

// UpdateSandboxCode 更新沙箱代码对象引用和哈希。
func (s *txStore) UpdateSandboxCode(ctx context.Context, tenantID, sandboxID int64, key, hash string) (Sandbox, error) {
	row, err := s.q.UpdateSandboxCode(ctx, sqlcgen.UpdateSandboxCodeParams{TenantID: tenantID, ID: sandboxID, CodeStorageKey: key, CodeHash: pgtypex.Text(hash)})
	if err != nil {
		return Sandbox{}, err
	}
	return sandboxFromRow(row)
}

// UpdateSandboxAuthorizedAccounts 持久化共享账号集合。
func (s *txStore) UpdateSandboxAuthorizedAccounts(ctx context.Context, tenantID, sandboxID int64, accountIDs []int64) (Sandbox, error) {
	row, err := s.q.UpdateSandboxAuthorizedAccounts(ctx, sqlcgen.UpdateSandboxAuthorizedAccountsParams{TenantID: tenantID, ID: sandboxID, SharedAccountIds: accountIDs})
	if err != nil {
		return Sandbox{}, apperr.ErrSandboxStatePersistFailed.WithCause(err)
	}
	return sandboxFromRow(row)
}

// UpdateSandboxSnapshot 更新沙箱快照引用和真实覆盖卷域。
func (s *txStore) UpdateSandboxSnapshot(ctx context.Context, tenantID, sandboxID int64, ref string, domains []string, createdAt, expireAt time.Time) (Sandbox, error) {
	snapshotDomains, err := jsonStringArray(domains)
	if err != nil {
		return Sandbox{}, fmt.Errorf("编码沙箱快照域失败: %w", err)
	}
	row, err := s.q.UpdateSandboxSnapshot(ctx, sqlcgen.UpdateSandboxSnapshotParams{
		TenantID:          tenantID,
		ID:                sandboxID,
		SnapshotRef:       pgtypex.Text(ref),
		SnapshotDomains:   snapshotDomains,
		SnapshotCreatedAt: timex.Timestamptz(createdAt),
		SnapshotExpireAt:  timex.Timestamptz(expireAt),
	})
	if err != nil {
		return Sandbox{}, err
	}
	return sandboxFromRow(row)
}

// CreateSandboxTool 创建沙箱工具挂载记录。
func (s *txStore) CreateSandboxTool(ctx context.Context, id int64, tenantID int64, sandboxID int64, tool Tool, endpoint string, status int16) (SandboxTool, error) {
	row, err := s.q.CreateSandboxTool(ctx, sqlcgen.CreateSandboxToolParams{ID: id, TenantID: tenantID, SandboxID: sandboxID, ToolID: tool.ID, AccessEndpoint: endpoint, Status: status})
	if err != nil {
		return SandboxTool{}, err
	}
	return SandboxTool{ID: row.ID, TenantID: row.TenantID, SandboxID: row.SandboxID, ToolID: row.ToolID, ToolCode: tool.Code, Kind: tool.Kind, AccessEndpoint: row.AccessEndpoint, Status: row.Status}, nil
}

// ListSandboxTools 查询沙箱工具接入信息。
func (s *txStore) ListSandboxTools(ctx context.Context, tenantID, sandboxID int64) ([]SandboxTool, error) {
	rows, err := s.q.ListSandboxTools(ctx, sqlcgen.ListSandboxToolsParams{TenantID: tenantID, SandboxID: sandboxID})
	if err != nil {
		return nil, err
	}
	out := make([]SandboxTool, 0, len(rows))
	for _, row := range rows {
		item, err := sandboxToolFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// UpdateSandboxToolStatus 更新沙箱内工具健康状态和访问端点。
func (s *txStore) UpdateSandboxToolStatus(ctx context.Context, tenantID, sandboxID int64, tool Tool, endpoint string, status int16) (SandboxTool, error) {
	row, err := s.q.UpdateSandboxToolStatus(ctx, sqlcgen.UpdateSandboxToolStatusParams{
		TenantID:       tenantID,
		SandboxID:      sandboxID,
		ToolID:         tool.ID,
		Status:         status,
		AccessEndpoint: endpoint,
	})
	if err != nil {
		return SandboxTool{}, err
	}
	return sandboxToolFromStatusRow(row, tool), nil
}

// CreateSandboxEvent 写入沙箱技术事件。
func (s *txStore) CreateSandboxEvent(ctx context.Context, id, tenantID, sandboxID int64, typ string, detail []byte) error {
	_, err := s.q.CreateSandboxEvent(ctx, sqlcgen.CreateSandboxEventParams{ID: id, TenantID: tenantID, SandboxID: sandboxID, EventType: typ, Detail: detail})
	return err
}

// CreateSandboxRecycleOutbox 在回收终态事务内保存回收事件。
func (s *txStore) CreateSandboxRecycleOutbox(ctx context.Context, id int64, sb Sandbox, reason, traceID string, recycledAt time.Time) (SandboxRecycleOutbox, error) {
	row, err := s.q.CreateSandboxRecycleOutbox(ctx, sqlcgen.CreateSandboxRecycleOutboxParams{ID: id, TenantID: sb.TenantID, SandboxID: sb.ID, SourceRef: sb.SourceRef, OwnerAccountID: sb.OwnerAccountID, Reason: reason, TraceID: traceID, RecycledAt: timex.RequiredTimestamptz(recycledAt)})
	if err != nil {
		return SandboxRecycleOutbox{}, err
	}
	return sandboxRecycleOutbox(row), nil
}

// ClaimPendingSandboxRecycleOutbox 跨租户领取待发布或失败待重试的回收事件。
func (s *txStore) ClaimPendingSandboxRecycleOutbox(ctx context.Context, limit, maxAttempts int32, staleBefore, leaseUntil time.Time) ([]SandboxRecycleOutbox, error) {
	leaseToken, err := pkgcrypto.RandomToken(48)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ClaimPendingSandboxRecycleOutbox(ctx, sqlcgen.ClaimPendingSandboxRecycleOutboxParams{LeaseToken: leaseToken, LeaseUntil: timex.RequiredTimestamptz(leaseUntil), StaleBefore: timex.RequiredTimestamptz(staleBefore), PageLimit: limit, MaxAttempts: maxAttempts})
	if err != nil {
		return nil, err
	}
	out := make([]SandboxRecycleOutbox, 0, len(rows))
	for _, row := range rows {
		out = append(out, sandboxRecycleOutbox(row))
	}
	return out, nil
}

// MarkSandboxRecycleOutboxPublished 标记回收事件发布成功。
func (s *txStore) MarkSandboxRecycleOutboxPublished(ctx context.Context, tenantID, id int64, leaseToken string) (SandboxRecycleOutbox, error) {
	row, err := s.q.MarkSandboxRecycleOutboxPublished(ctx, sqlcgen.MarkSandboxRecycleOutboxPublishedParams{TenantID: tenantID, ID: id, LeaseToken: leaseToken})
	if err != nil {
		return SandboxRecycleOutbox{}, err
	}
	return sandboxRecycleOutbox(row), nil
}

// MarkSandboxRecycleOutboxFailed 标记回收事件发布失败并保留脱敏原因。
func (s *txStore) MarkSandboxRecycleOutboxFailed(ctx context.Context, tenantID, id int64, reason, leaseToken string) (SandboxRecycleOutbox, error) {
	row, err := s.q.MarkSandboxRecycleOutboxFailed(ctx, sqlcgen.MarkSandboxRecycleOutboxFailedParams{TenantID: tenantID, ID: id, LastError: pgtypex.Text(reason), LeaseToken: leaseToken})
	if err != nil {
		return SandboxRecycleOutbox{}, err
	}
	return sandboxRecycleOutbox(row), nil
}

// StatsByTenant 查询租户配额和活跃数量。
func (s *txStore) StatsByTenant(ctx context.Context, tenantID int64) (TenantQuota, int64, error) {
	row, err := s.q.StatsByTenant(ctx, tenantID)
	if err != nil {
		return TenantQuota{}, 0, err
	}
	return TenantQuota{
		TenantID:                tenantID,
		MaxConcurrentSandbox:    row.MaxConcurrentSandbox,
		MaxCPU:                  row.MaxCpu,
		MaxMemoryMB:             row.MaxMemoryMb,
		IdleTimeoutMin:          row.IdleTimeoutMin,
		MaxLifetimeMin:          row.MaxLifetimeMin,
		MaxKeepaliveMin:         row.MaxKeepaliveMin,
		MaxSnapshotRetentionMin: row.MaxSnapshotRetentionMin,
	}, row.ActiveSandboxCount, nil
}

// sandboxRows 批量转换 sqlc 沙箱行。
func sandboxRows(rows []sqlcgen.Sandbox) ([]Sandbox, error) {
	out := make([]Sandbox, 0, len(rows))
	for _, row := range rows {
		item, err := sandboxFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// jsonStringArray 把字符串数组编码为 JSONB 参数,空数组保持可审计的显式空列表。
func jsonStringArray(values []string) ([]byte, error) {
	return jsonBytes(values)
}

// stringsJoin 把生态标签列表写成文档约定的逗号分隔字段。
func stringsJoin(values []string) string {
	out := ""
	for _, value := range values {
		if value == "" {
			continue
		}
		if out != "" {
			out += ","
		}
		out += value
	}
	return out
}
