// M4 数据访问层:只读写 sim 模块自有表,全部经 sqlc 生成查询。
package sim

import (
	"context"
	"strings"

	"chaimir/internal/modules/sim/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// repo 封装 sim 模块数据库事务入口。
type repo struct {
	db *db.DB
}

// newRepo 构造 M4 repo。
func newRepo(database *db.DB) *repo {
	return &repo{db: database}
}

// queryFunc 是 M4 数据访问闭包,统一接收 sqlc 查询对象。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 从 ctx 取租户并注入 RLS 后执行查询。
func (r *repo) inTenant(ctx context.Context, fn queryFunc) error {
	return r.db.WithTenantTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inTenantID 用显式租户 ID 注入 RLS,供 contracts 内部调用使用。
func (r *repo) inTenantID(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inApp 访问仿真包和审核平台级配置表。
func (r *repo) inApp(ctx context.Context, fn queryFunc) error {
	return r.db.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// tenantFromContext 读取当前租户身份。
func tenantFromContext(ctx context.Context) (tenant.Identity, bool) {
	return tenant.FromContext(ctx)
}

// listPackages 读取仿真包列表。
func (r *repo) listPackages(ctx context.Context, category, keyword string, status int16) ([]PackageSnapshot, error) {
	var out []PackageSnapshot
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		rows, e := q.ListSimPackages(ctx, sqlcgen.ListSimPackagesParams{
			Limit: 100, Offset: 0, Category: pgtypex.Text(category), Keyword: pgtypex.Text(keyword), Status: pgtypex.Int2(status),
		})
		out = packageSnapshotsFromRows(rows)
		return e
	})
	return out, err
}

// listVersions 读取指定仿真包已上架版本。
func (r *repo) listVersions(ctx context.Context, code string) ([]PackageSnapshot, error) {
	var out []PackageSnapshot
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		rows, e := q.ListSimPackageVersions(ctx, code)
		out = packageSnapshotsFromRows(rows)
		return e
	})
	return out, err
}

// getPackageByCodeVersion 读取指定仿真包版本并映射不存在错误。
func (r *repo) getPackageByCodeVersion(ctx context.Context, code, version string) (PackageSnapshot, error) {
	var out PackageSnapshot
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.GetSimPackageByCodeVersion(ctx, sqlcgen.GetSimPackageByCodeVersionParams{Code: code, Version: version})
		if db.IsNoRows(e) {
			return apperr.ErrSimPackageNotFound
		}
		out = packageSnapshotFromRow(row)
		return e
	})
	return out, err
}

// getPackageByID 读取仿真包并映射不存在错误。
func (r *repo) getPackageByID(ctx context.Context, packageID int64) (PackageSnapshot, error) {
	var out PackageSnapshot
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.GetSimPackageByID(ctx, packageID)
		if db.IsNoRows(e) {
			return apperr.ErrSimPackageNotFound
		}
		out = packageSnapshotFromRow(row)
		return e
	})
	return out, err
}

// createPackageWithReview 原子创建仿真包版本和审核记录。
func (r *repo) createPackageWithReview(ctx context.Context, packageID, reviewID, submitterID int64, req SubmitPackageRequest, compute int16, scaleLimit, backendConfig, preview []byte) (PackageSnapshot, error) {
	authorID, _ := ids.Parse(req.AuthorID)
	var out PackageSnapshot
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.CreateSimPackage(ctx, sqlcgen.CreateSimPackageParams{
			ID: packageID, Code: req.Code, Version: req.Version, Name: req.Name, Category: req.Category, Compute: compute,
			ScaleLimit: scaleLimit, BundleKey: req.BundleKey, BundleHash: strings.ToLower(req.BundleHash),
			BackendAdapter: pgtypex.Text(req.BackendAdapter), BackendConfig: backendConfig,
			AuthorType: req.AuthorType, AuthorID: pgtypex.Int8(authorID), Status: PackageStatusReviewing,
		})
		if e != nil {
			return e
		}
		out = packageSnapshotFromRow(row)
		_, e = q.CreateSimPackageReview(ctx, sqlcgen.CreateSimPackageReviewParams{
			ID: reviewID, PackageID: row.ID, SubmitterID: submitterID, PreviewReport: preview,
		})
		return e
	})
	return out, err
}

// updatePackageDraftWithReview 原子更新草稿/退回包并创建新审核记录。
func (r *repo) updatePackageDraftWithReview(ctx context.Context, packageID, reviewID, submitterID int64, req UpdatePackageRequest, scaleLimit, backendConfig, preview []byte) (PackageSnapshot, error) {
	var out PackageSnapshot
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.UpdateSimPackageDraft(ctx, sqlcgen.UpdateSimPackageDraftParams{
			ID: packageID, Name: req.Name, Category: req.Category, ScaleLimit: scaleLimit,
			BundleKey: req.BundleKey, BundleHash: strings.ToLower(req.BundleHash), BackendAdapter: pgtypex.Text(req.BackendAdapter),
			BackendConfig: backendConfig,
		})
		if e != nil {
			return e
		}
		out = packageSnapshotFromRow(row)
		_, e = q.CreateSimPackageReview(ctx, sqlcgen.CreateSimPackageReviewParams{
			ID: reviewID, PackageID: row.ID, SubmitterID: submitterID, PreviewReport: preview,
		})
		return e
	})
	return out, err
}

// listReviews 读取仿真包审核列表。
func (r *repo) listReviews(ctx context.Context, result int16) ([]ReviewSnapshot, error) {
	var out []ReviewSnapshot
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		rows, e := q.ListSimReviews(ctx, sqlcgen.ListSimReviewsParams{Limit: 100, Offset: 0, Result: pgtypex.Int2(result)})
		out = reviewSnapshotsFromRows(rows)
		return e
	})
	return out, err
}

// getReviewByID 读取审核记录并映射不存在错误。
func (r *repo) getReviewByID(ctx context.Context, reviewID int64) (ReviewSnapshot, error) {
	var out ReviewSnapshot
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.GetSimReviewByID(ctx, reviewID)
		if db.IsNoRows(e) {
			return apperr.ErrSimReviewNotFound
		}
		out = reviewSnapshotFromRow(row)
		return e
	})
	return out, err
}

// completeReviewWithPackageStatus 完成审核并同步包状态。
func (r *repo) completeReviewWithPackageStatus(ctx context.Context, reviewID, reviewerID int64, result, packageStatus int16, comment string) (ReviewSnapshot, error) {
	var out ReviewSnapshot
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		review, e := q.CompleteSimReview(ctx, sqlcgen.CompleteSimReviewParams{ID: reviewID, Result: result, ReviewerID: pgtypex.Int8(reviewerID), Comment: pgtypex.Text(comment)})
		if db.IsNoRows(e) {
			return apperr.ErrSimReviewInvalidState
		}
		if e != nil {
			return e
		}
		out = reviewSnapshotFromRow(review)
		_, e = q.UpdateSimPackageStatus(ctx, sqlcgen.UpdateSimPackageStatusParams{ID: review.PackageID, Status: packageStatus})
		return e
	})
	return out, err
}

// getPendingReviewByPackageID 读取包当前待审记录。
func (r *repo) getPendingReviewByPackageID(ctx context.Context, packageID int64) (ReviewSnapshot, error) {
	var out ReviewSnapshot
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.GetPendingSimReviewByPackageID(ctx, packageID)
		if db.IsNoRows(e) {
			return apperr.ErrSimReviewNotFound
		}
		out = reviewSnapshotFromRow(row)
		return e
	})
	return out, err
}

// updateReviewPreviewReport 写入受控预览报告。
func (r *repo) updateReviewPreviewReport(ctx context.Context, packageID int64, report []byte) (ReviewSnapshot, error) {
	var out ReviewSnapshot
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.UpdateSimReviewPreviewReport(ctx, sqlcgen.UpdateSimReviewPreviewReportParams{PackageID: packageID, PreviewReport: report})
		if db.IsNoRows(e) {
			return apperr.ErrSimReviewNotFound
		}
		out = reviewSnapshotFromRow(row)
		return e
	})
	return out, err
}

// transitionPackageStatus 按期望前置状态变更包状态。
func (r *repo) transitionPackageStatus(ctx context.Context, packageID int64, from, to int16) (PackageSnapshot, error) {
	var out PackageSnapshot
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.TransitionSimPackageStatus(ctx, sqlcgen.TransitionSimPackageStatusParams{ID: packageID, Status: from, Status_2: to})
		if db.IsNoRows(e) {
			return apperr.ErrSimPackageUnavailable
		}
		out = packageSnapshotFromRow(row)
		return e
	})
	return out, err
}

// createSession 写入仿真会话。
func (r *repo) createSession(ctx context.Context, tenantID, sessionID, ownerID int64, req CreateSessionRequest, pkg PackageSnapshot, initParams []byte) (SessionSnapshot, error) {
	var out SessionSnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		row, e := q.CreateSimSession(ctx, sqlcgen.CreateSimSessionParams{
			ID: sessionID, TenantID: tenantID, PackageID: pkg.ID, SourceRef: req.SourceRef,
			OwnerAccountID: ownerID, Seed: req.Seed, InitParams: initParams, Compute: pkg.Compute,
		})
		out = sessionSnapshotFromRow(row)
		return e
	})
	return out, err
}

// createActionIfNext 校验会话归属、连续序号和幂等后写入操作。
func (r *repo) createActionIfNext(ctx context.Context, tenantID, actionID, sessionID int64, actor tenant.Identity, req ReportActionRequest, payload []byte) (ActionSnapshot, error) {
	var out ActionSnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		session, e := q.GetSimSessionByID(ctx, sessionID)
		if db.IsNoRows(e) {
			return apperr.ErrSimSessionNotFound
		}
		if e != nil {
			return e
		}
		if session.Status == SessionStatusArchived || session.Status == SessionStatusFailed {
			return apperr.ErrSimSessionInvalidState
		}
		if e = authorizeSessionOwner(actor, session.OwnerAccountID); e != nil {
			return e
		}
		existing, e := q.GetSimActionBySeq(ctx, sqlcgen.GetSimActionBySeqParams{SessionID: sessionID, Seq: req.Seq})
		if e == nil {
			dto := actionToDTO(actionSnapshotFromRow(existing))
			if e = validateNextAction(req.Seq, &dto, req); e != nil {
				return e
			}
			out = actionSnapshotFromRow(existing)
			return nil
		}
		if e != nil && !db.IsNoRows(e) {
			return e
		}
		last, e := q.GetLastSimAction(ctx, sessionID)
		lastSeq := int32(0)
		if e == nil {
			lastSeq = last.Seq
		} else if !db.IsNoRows(e) {
			return e
		}
		if e = validateNextAction(lastSeq, nil, req); e != nil {
			return e
		}
		row, e := q.CreateSimAction(ctx, sqlcgen.CreateSimActionParams{
			ID: actionID, TenantID: tenantID, SessionID: sessionID, Seq: req.Seq,
			AtTick: req.AtTick, EventType: req.EventType, Payload: payload,
		})
		out = actionSnapshotFromRow(row)
		return e
	})
	return out, err
}

// authorizeReplay 读取会话并校验当前账号可访问回放。
func (r *repo) authorizeReplay(ctx context.Context, tenantID, sessionID int64, actor tenant.Identity) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		session, e := q.GetSimSessionByID(ctx, sessionID)
		if db.IsNoRows(e) {
			return apperr.ErrSimSessionNotFound
		}
		if e != nil {
			return e
		}
		return authorizeSessionOwner(actor, session.OwnerAccountID)
	})
}

// archiveSessionsBySourceRef 按来源归档会话。
func (r *repo) archiveSessionsBySourceRef(ctx context.Context, tenantID int64, sourceRef string) ([]SessionSnapshot, error) {
	var out []SessionSnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		rows, e := q.ArchiveSimSessionsBySourceRef(ctx, sourceRef)
		out = make([]SessionSnapshot, 0, len(rows))
		for _, row := range rows {
			out = append(out, sessionSnapshotFromRow(row))
		}
		return e
	})
	return out, err
}

// archiveSession 归档单个仿真会话并返回归档行。
func (r *repo) archiveSession(ctx context.Context, tenantID, sessionID int64, actor tenant.Identity) (SessionSnapshot, error) {
	var out SessionSnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		current, e := q.GetSimSessionByID(ctx, sessionID)
		if db.IsNoRows(e) {
			return apperr.ErrSimSessionNotFound
		}
		if e != nil {
			return e
		}
		if e := validateSimSourceRefAccess(ctx, current.SourceRef); e != nil {
			return e
		}
		if e := authorizeSessionOwner(actor, current.OwnerAccountID); e != nil {
			return e
		}
		row, e := q.ArchiveSimSession(ctx, sessionID)
		out = sessionSnapshotFromRow(row)
		return e
	})
	return out, err
}

// createShare 校验会话归属后创建公开分享码索引。
func (r *repo) createShare(ctx context.Context, tenantID, shareID, sessionID, actorID int64, actor tenant.Identity, code string) (ShareSnapshot, error) {
	var out ShareSnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		session, e := q.GetSimSessionByID(ctx, sessionID)
		if db.IsNoRows(e) {
			return apperr.ErrSimSessionNotFound
		}
		if e != nil {
			return e
		}
		if e = authorizeSessionOwner(actor, session.OwnerAccountID); e != nil {
			return e
		}
		row, e := q.CreateSimShare(ctx, sqlcgen.CreateSimShareParams{
			ID: shareID, TenantID: tenantID, SessionID: sessionID, Code: code,
			CreatedBy: actorID, ExpireAt: pgtype.Timestamptz{},
		})
		out = ShareSnapshot{ID: row.ID, TenantID: row.TenantID, SessionID: row.SessionID, Code: row.Code}
		return e
	})
	return out, err
}

// getShareByCode 通过公开分享码读取租户和会话指针。
func (r *repo) getShareByCode(ctx context.Context, code string) (ShareSnapshot, error) {
	var out ShareSnapshot
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.GetSimShareByCode(ctx, code)
		if db.IsNoRows(e) {
			return apperr.ErrSimShareInvalid
		}
		out = ShareSnapshot{ID: row.ID, TenantID: row.TenantID, SessionID: row.SessionID, Code: row.Code}
		return e
	})
	return out, err
}

// upsertCheckpoint 校验会话和来源后保存检查点结果。
func (r *repo) upsertCheckpoint(ctx context.Context, tenantID, checkpointID, sessionID int64, req ReportCheckpointRequest, answer []byte) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		session, e := q.GetSimSessionByID(ctx, sessionID)
		if db.IsNoRows(e) {
			return apperr.ErrSimSessionNotFound
		}
		if e != nil {
			return e
		}
		if e := validateSimSourceRefAccess(ctx, session.SourceRef); e != nil {
			return e
		}
		_, e = q.UpsertSimCheckpoint(ctx, sqlcgen.UpsertSimCheckpointParams{
			ID: checkpointID, TenantID: tenantID, SessionID: sessionID,
			CheckpointID: req.CheckpointID, Answer: answer, Achieved: req.Achieved,
		})
		return e
	})
}

// replayInTenant 读取回放所需会话与操作序列。
func (r *repo) replayInTenant(ctx context.Context, tenantID, sessionID int64) (ReplaySnapshot, []ActionSnapshot, error) {
	var replay ReplaySnapshot
	var actions []ActionSnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		row, e := q.GetSimSessionWithPackage(ctx, sessionID)
		if db.IsNoRows(e) {
			return apperr.ErrSimSessionNotFound
		}
		if e != nil {
			return e
		}
		replay = replaySnapshotFromRow(row)
		if e := validateSimSourceRefAccess(ctx, row.SourceRef); e != nil {
			return e
		}
		foundActions, e := q.ListSimActions(ctx, sessionID)
		actions = actionSnapshotsFromRows(foundActions)
		return e
	})
	return replay, actions, err
}

// loadBackendSession 读取后端计算会话所需的包适配器信息。
func (r *repo) loadBackendSession(ctx context.Context, tenantID, sessionID int64) (BackendSessionSnapshot, error) {
	var out BackendSessionSnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		row, e := q.GetSimSessionWithPackage(ctx, sessionID)
		if db.IsNoRows(e) {
			return apperr.ErrSimSessionNotFound
		}
		out = backendSessionFromReplay(replaySnapshotFromRow(row))
		return e
	})
	return out, err
}

// encodeActionPayload 序列化操作载荷供 repo 写入 JSONB。
func encodeActionPayload(payload map[string]any) ([]byte, error) {
	return jsonx.ObjectBytes(payload, apperr.ErrSimActionInvalid)
}
