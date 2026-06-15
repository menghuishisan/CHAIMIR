// sim repo 文件定义 M4 持久化接口和数据库事务边界,是 service 访问数据库的唯一入口。
package sim

import (
	"context"
	"errors"
	"fmt"

	"chaimir/internal/modules/sim/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
)

// Store 定义 service 所需的 sim 持久化能力,不暴露 sqlc 行类型。
type Store interface {
	PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error
	PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error
	TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error
}

// isNoRows 统一识别未命中错误,让 service 不直接依赖 pgx/db 实现细节。
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// TxStore 定义单个事务内可调用的 sim 数据访问能力。
type TxStore interface {
	GetPackageByCodeVersion(ctx context.Context, code, version string) (Package, error)
	GetPackageByID(ctx context.Context, id int64) (Package, error)
	ListPackages(ctx context.Context, status int16, category, keyword string, limit, offset int32) ([]Package, int64, error)
	ListPackageVersions(ctx context.Context, code string) ([]Package, error)
	CreatePackage(ctx context.Context, pkg Package) (Package, error)
	UpdatePackageDraft(ctx context.Context, pkg Package) (Package, error)
	UpdatePackageStatus(ctx context.Context, id int64, status int16) (Package, error)
	CreateReview(ctx context.Context, id, packageID, submitterID int64, report ValidationReport) (Review, error)
	GetReview(ctx context.Context, id int64) (Review, error)
	GetLatestReviewForPackage(ctx context.Context, packageID int64) (Review, error)
	ListReviews(ctx context.Context, result int16, limit, offset int32) ([]ReviewInfo, int64, error)
	MergeValidationReport(ctx context.Context, packageID int64, report ValidationReport) (Review, error)
	CompleteReview(ctx context.Context, id int64, result int16, reviewerID int64, comment string) (Review, error)
	CreateSession(ctx context.Context, session Session) (Session, error)
	GetSession(ctx context.Context, tenantID, sessionID int64) (Session, error)
	GetSessionWithPackage(ctx context.Context, tenantID, sessionID int64) (SessionWithPackage, error)
	ArchiveSession(ctx context.Context, tenantID, sessionID int64) (Session, error)
	ArchiveSessionsBySourceRef(ctx context.Context, tenantID int64, sourceRef string) ([]Session, error)
	GetLastAction(ctx context.Context, tenantID, sessionID int64) (Action, error)
	GetActionBySeq(ctx context.Context, tenantID, sessionID int64, seq int32) (Action, error)
	CreateAction(ctx context.Context, action Action) (Action, error)
	ListActions(ctx context.Context, tenantID, sessionID int64) ([]Action, error)
	UpsertCheckpoint(ctx context.Context, cp Checkpoint) (Checkpoint, error)
	CreateShare(ctx context.Context, share Share) (Share, error)
	GetShareByCode(ctx context.Context, code string) (Share, error)
}

type store struct {
	database *db.DB
}

type txStore struct {
	q *sqlcgen.Queries
}

// NewStore 创建 sim 模块持久化入口,仅装配层应调用。
func NewStore(database *db.DB) Store {
	return &store{database: database}
}

// PlatformTx 在应用连接中访问仿真包和审核表。
func (s *store) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("sim store 未初始化")
	}
	return s.database.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// PrivilegedTx 仅用于公开分享码预认证定位这类无租户上下文的 M4 自有表读取。
func (s *store) PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("sim store 未初始化")
	}
	return s.database.WithPrivilegedTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// TenantTx 在注入 RLS 租户变量后访问租户内会话、操作、检查点和分享表。
func (s *store) TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("sim store 未初始化")
	}
	return s.database.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// GetPackageByCodeVersion 按 code/version 查询包。
func (s *txStore) GetPackageByCodeVersion(ctx context.Context, code, version string) (Package, error) {
	row, err := s.q.GetSimPackageByCodeVersion(ctx, sqlcgen.GetSimPackageByCodeVersionParams{Code: code, Version: version})
	if err != nil {
		return Package{}, err
	}
	return packageFromRow(row)
}

// GetPackageByID 按 ID 查询包。
func (s *txStore) GetPackageByID(ctx context.Context, id int64) (Package, error) {
	row, err := s.q.GetSimPackageByID(ctx, id)
	if err != nil {
		return Package{}, err
	}
	return packageFromRow(row)
}

// ListPackages 查询包分页。
func (s *txStore) ListPackages(ctx context.Context, status int16, category, keyword string, limit, offset int32) ([]Package, int64, error) {
	params := sqlcgen.ListSimPackagesParams{Column1: status, Column2: category, Column3: keyword, Limit: limit, Offset: offset}
	rows, err := s.q.ListSimPackages(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountSimPackages(ctx, sqlcgen.CountSimPackagesParams{Column1: status, Column2: category, Column3: keyword})
	if err != nil {
		return nil, 0, err
	}
	items, err := packagesFromRows(rows)
	return items, total, err
}

// ListPackageVersions 查询同 code 下所有版本。
func (s *txStore) ListPackageVersions(ctx context.Context, code string) ([]Package, error) {
	rows, err := s.q.ListSimPackageVersions(ctx, code)
	if err != nil {
		return nil, err
	}
	return packagesFromRows(rows)
}

// CreatePackage 新建仿真包版本。
func (s *txStore) CreatePackage(ctx context.Context, pkg Package) (Package, error) {
	scale, backend, err := packageJSON(pkg)
	if err != nil {
		return Package{}, err
	}
	row, err := s.q.CreateSimPackage(ctx, sqlcgen.CreateSimPackageParams{ID: pkg.ID, Code: pkg.Code, Version: pkg.Version, Name: pkg.Name, Category: pkg.Category, Compute: pkg.Compute, ScaleLimit: scale, BundleKey: pkg.BundleKey, BundleHash: pkg.BundleHash, BackendAdapter: pgtypex.Text(pkg.BackendAdapter), BackendConfig: backend, AuthorType: pkg.AuthorType, AuthorID: pgtypex.Int8(pkg.AuthorID), Status: pkg.Status})
	if err != nil {
		return Package{}, err
	}
	return packageFromRow(row)
}

// UpdatePackageDraft 更新草稿或退回包。
func (s *txStore) UpdatePackageDraft(ctx context.Context, pkg Package) (Package, error) {
	scale, backend, err := packageJSON(pkg)
	if err != nil {
		return Package{}, err
	}
	row, err := s.q.UpdateSimPackageDraft(ctx, sqlcgen.UpdateSimPackageDraftParams{ID: pkg.ID, Name: pkg.Name, Category: pkg.Category, Compute: pkg.Compute, ScaleLimit: scale, BundleKey: pkg.BundleKey, BundleHash: pkg.BundleHash, BackendAdapter: pgtypex.Text(pkg.BackendAdapter), BackendConfig: backend, Status: pkg.Status})
	if err != nil {
		return Package{}, err
	}
	return packageFromRow(row)
}

// UpdatePackageStatus 更新包生命周期状态。
func (s *txStore) UpdatePackageStatus(ctx context.Context, id int64, status int16) (Package, error) {
	row, err := s.q.UpdateSimPackageStatus(ctx, sqlcgen.UpdateSimPackageStatusParams{ID: id, Status: status})
	if err != nil {
		return Package{}, err
	}
	return packageFromRow(row)
}

// CreateReview 创建待审记录。
func (s *txStore) CreateReview(ctx context.Context, id, packageID, submitterID int64, report ValidationReport) (Review, error) {
	raw, err := jsonx.AnyBytes(report, apperr.ErrSimPackageValidationFailed)
	if err != nil {
		return Review{}, err
	}
	row, err := s.q.CreateSimPackageReview(ctx, sqlcgen.CreateSimPackageReviewParams{ID: id, PackageID: packageID, SubmitterID: submitterID, PreviewReport: raw})
	if err != nil {
		return Review{}, err
	}
	return reviewFromRow(row)
}

// GetReview 按 ID 查询审核记录。
func (s *txStore) GetReview(ctx context.Context, id int64) (Review, error) {
	row, err := s.q.GetSimReviewByID(ctx, id)
	if err != nil {
		return Review{}, err
	}
	return reviewFromRow(row)
}

// GetLatestReviewForPackage 查询包的最新审核记录。
func (s *txStore) GetLatestReviewForPackage(ctx context.Context, packageID int64) (Review, error) {
	row, err := s.q.GetLatestSimReviewForPackage(ctx, packageID)
	if err != nil {
		return Review{}, err
	}
	return reviewFromRow(row)
}

// ListReviews 查询审核分页。
func (s *txStore) ListReviews(ctx context.Context, result int16, limit, offset int32) ([]ReviewInfo, int64, error) {
	rows, err := s.q.ListSimReviews(ctx, sqlcgen.ListSimReviewsParams{Column1: result, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountSimReviews(ctx, result)
	if err != nil {
		return nil, 0, err
	}
	items := make([]ReviewInfo, 0, len(rows))
	for _, row := range rows {
		item, err := reviewInfoFromRow(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, nil
}

// MergeValidationReport 合并动态预览报告。
func (s *txStore) MergeValidationReport(ctx context.Context, packageID int64, report ValidationReport) (Review, error) {
	raw, err := jsonx.AnyBytes(report, apperr.ErrSimPackageValidationFailed)
	if err != nil {
		return Review{}, err
	}
	row, err := s.q.MergeSimValidationReport(ctx, sqlcgen.MergeSimValidationReportParams{PackageID: packageID, PreviewReport: raw})
	if err != nil {
		return Review{}, err
	}
	return reviewFromRow(row)
}

// CompleteReview 完成审核记录。
func (s *txStore) CompleteReview(ctx context.Context, id int64, result int16, reviewerID int64, comment string) (Review, error) {
	row, err := s.q.CompleteSimReview(ctx, sqlcgen.CompleteSimReviewParams{ID: id, Result: result, ReviewerID: pgtypex.Int8(reviewerID), Comment: pgtypex.Text(comment)})
	if err != nil {
		return Review{}, err
	}
	return reviewFromRow(row)
}

// CreateSession 新建仿真会话。
func (s *txStore) CreateSession(ctx context.Context, session Session) (Session, error) {
	raw, err := jsonx.AnyBytes(session.InitParams, apperr.ErrSimSessionInvalid)
	if err != nil {
		return Session{}, err
	}
	row, err := s.q.CreateSimSession(ctx, sqlcgen.CreateSimSessionParams{ID: session.ID, TenantID: session.TenantID, PackageID: session.PackageID, SourceRef: session.SourceRef, OwnerAccountID: session.OwnerAccountID, Seed: session.Seed, InitParams: raw, Compute: session.Compute})
	if err != nil {
		return Session{}, err
	}
	return sessionFromRow(row)
}

// GetSession 查询仿真会话。
func (s *txStore) GetSession(ctx context.Context, tenantID, sessionID int64) (Session, error) {
	row, err := s.q.GetSimSession(ctx, sqlcgen.GetSimSessionParams{TenantID: tenantID, ID: sessionID})
	if err != nil {
		return Session{}, err
	}
	return sessionFromRow(row)
}

// GetSessionWithPackage 查询回放需要的会话和包摘要。
func (s *txStore) GetSessionWithPackage(ctx context.Context, tenantID, sessionID int64) (SessionWithPackage, error) {
	row, err := s.q.GetSimSessionWithPackage(ctx, sqlcgen.GetSimSessionWithPackageParams{TenantID: tenantID, ID: sessionID})
	if err != nil {
		return SessionWithPackage{}, err
	}
	return sessionWithPackageFromRow(row)
}

// ArchiveSession 归档单个会话。
func (s *txStore) ArchiveSession(ctx context.Context, tenantID, sessionID int64) (Session, error) {
	row, err := s.q.ArchiveSimSession(ctx, sqlcgen.ArchiveSimSessionParams{TenantID: tenantID, ID: sessionID})
	if err != nil {
		return Session{}, err
	}
	return sessionFromRow(row)
}

// ArchiveSessionsBySourceRef 按来源归档会话。
func (s *txStore) ArchiveSessionsBySourceRef(ctx context.Context, tenantID int64, sourceRef string) ([]Session, error) {
	rows, err := s.q.ArchiveSimSessionsBySourceRef(ctx, sqlcgen.ArchiveSimSessionsBySourceRefParams{TenantID: tenantID, SourceRef: sourceRef})
	if err != nil {
		return nil, err
	}
	items := make([]Session, 0, len(rows))
	for _, row := range rows {
		item, err := sessionFromRow(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// GetLastAction 查询会话最后一条操作。
func (s *txStore) GetLastAction(ctx context.Context, tenantID, sessionID int64) (Action, error) {
	row, err := s.q.GetLastSimAction(ctx, sqlcgen.GetLastSimActionParams{TenantID: tenantID, SessionID: sessionID})
	if err != nil {
		return Action{}, err
	}
	return actionFromRow(row)
}

// GetActionBySeq 查询指定 seq 操作。
func (s *txStore) GetActionBySeq(ctx context.Context, tenantID, sessionID int64, seq int32) (Action, error) {
	row, err := s.q.GetSimActionBySeq(ctx, sqlcgen.GetSimActionBySeqParams{TenantID: tenantID, SessionID: sessionID, Seq: seq})
	if err != nil {
		return Action{}, err
	}
	return actionFromRow(row)
}

// CreateAction 创建操作序列项。
func (s *txStore) CreateAction(ctx context.Context, action Action) (Action, error) {
	raw, err := jsonx.AnyBytes(action.Payload, apperr.ErrSimActionSeqInvalid)
	if err != nil {
		return Action{}, err
	}
	row, err := s.q.CreateSimAction(ctx, sqlcgen.CreateSimActionParams{ID: action.ID, TenantID: action.TenantID, SessionID: action.SessionID, Seq: action.Seq, AtTick: action.AtTick, EventType: action.EventType, Payload: raw})
	if err != nil {
		return Action{}, err
	}
	return actionFromRow(row)
}

// ListActions 查询回放操作序列。
func (s *txStore) ListActions(ctx context.Context, tenantID, sessionID int64) ([]Action, error) {
	rows, err := s.q.ListSimActions(ctx, sqlcgen.ListSimActionsParams{TenantID: tenantID, SessionID: sessionID})
	if err != nil {
		return nil, err
	}
	items := make([]Action, 0, len(rows))
	for _, row := range rows {
		item, err := actionFromRow(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// UpsertCheckpoint 保存或覆盖检查点结果快照。
func (s *txStore) UpsertCheckpoint(ctx context.Context, cp Checkpoint) (Checkpoint, error) {
	row, err := s.q.UpsertSimCheckpoint(ctx, sqlcgen.UpsertSimCheckpointParams{ID: cp.ID, TenantID: cp.TenantID, SessionID: cp.SessionID, CheckpointID: cp.CheckpointID, Answer: cp.Answer, Achieved: cp.Achieved})
	if err != nil {
		return Checkpoint{}, err
	}
	return Checkpoint{ID: row.ID, TenantID: row.TenantID, SessionID: row.SessionID, CheckpointID: row.CheckpointID, Answer: row.Answer, Achieved: row.Achieved, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}, nil
}

// CreateShare 创建分享码索引。
func (s *txStore) CreateShare(ctx context.Context, share Share) (Share, error) {
	row, err := s.q.CreateSimShare(ctx, sqlcgen.CreateSimShareParams{ID: share.ID, TenantID: share.TenantID, SessionID: share.SessionID, Code: share.Code, CreatedBy: share.CreatedBy, ExpireAt: timex.Timestamptz(share.ExpireAt)})
	if err != nil {
		return Share{}, err
	}
	return shareFromRow(row), nil
}

// GetShareByCode 按公开分享码查询分享索引,仅允许在特权预解析或对应租户事务中调用。
func (s *txStore) GetShareByCode(ctx context.Context, code string) (Share, error) {
	row, err := s.q.GetSimShareByCode(ctx, code)
	if err != nil {
		return Share{}, err
	}
	return shareFromRow(row), nil
}

// packagesFromRows 批量转换包行。
func packagesFromRows(rows []sqlcgen.SimPackage) ([]Package, error) {
	items := make([]Package, 0, len(rows))
	for _, row := range rows {
		item, err := packageFromRow(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// packageJSON 序列化包 JSONB 字段。
func packageJSON(pkg Package) ([]byte, []byte, error) {
	scale, err := jsonx.AnyBytes(pkg.ScaleLimit, apperr.ErrSimPackageInvalid)
	if err != nil {
		return nil, nil, err
	}
	backend, err := jsonx.AnyBytes(pkg.BackendConfig, apperr.ErrSimPackageInvalid)
	if err != nil {
		return nil, nil, err
	}
	return scale, backend, nil
}
