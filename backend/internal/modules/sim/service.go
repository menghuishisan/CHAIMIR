// M4 服务层:承载仿真包管理、审核、会话、操作序列、分享、检查点与后端计算入口。
package sim

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/sim/internal/sqlcgen"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"

	"github.com/jackc/pgx/v5/pgtype"
)

// Service 是 M4 仿真可视化引擎服务,负责后端控制面与可复现数据持久化。
type Service struct {
	repo     *repo
	idgen    *snowflake.Node
	store    *storage.Storage
	bus      eventbus.Bus
	auditor  audit.Writer
	identity contracts.IdentityService
	backend  *BackendAdapterRegistry
	wsOrigin ws.OriginPolicy
}

// NewService 构造 M4 服务。
func NewService(database *db.DB, idgen *snowflake.Node, store *storage.Storage, bus eventbus.Bus, auditor audit.Writer, identity contracts.IdentityService, backend *BackendAdapterRegistry, wsOrigin ws.OriginPolicy) *Service {
	if backend == nil {
		backend = NewBackendAdapterRegistry()
	}
	return &Service{repo: newRepo(database), idgen: idgen, store: store, bus: bus, auditor: auditor, identity: identity, backend: backend, wsOrigin: wsOrigin}
}

// ListPackages 查询已上架或指定状态的仿真包列表。
func (s *Service) ListPackages(ctx context.Context, category, keyword string, status int16) ([]PackageDTO, error) {
	var rows []sqlcgen.SimPackage
	if status == 0 {
		status = PackageStatusPublished
	}
	id, ok := tenantFromContext(ctx)
	if !ok {
		return nil, apperr.ErrUnauthorized
	}
	if err := validatePackageListStatusAccess(id, status); err != nil {
		return nil, err
	}
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListSimPackages(ctx, sqlcgen.ListSimPackagesParams{
			Limit: 100, Offset: 0, Category: pgText(category), Keyword: pgText(keyword), Status: pgInt2(status),
		})
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrSimPackageQueryFailed.WithCause(err)
	}
	return packagesToDTO(rows), nil
}

// ListVersions 查询某个仿真包已上架版本。
func (s *Service) ListVersions(ctx context.Context, code string) ([]PackageDTO, error) {
	var rows []sqlcgen.SimPackage
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListSimPackageVersions(ctx, code)
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrSimPackageQueryFailed.WithCause(err)
	}
	return packagesToDTO(rows), nil
}

// GetBundleRef 返回仿真包对象存储引用;实际读取由统一对象存储网关鉴权完成。
func (s *Service) GetBundleRef(ctx context.Context, code, version string) (map[string]any, error) {
	pkg, err := s.loadPackage(ctx, code, version)
	if err != nil {
		return nil, err
	}
	if pkg.Status != PackageStatusPublished {
		return nil, apperr.ErrSimPackageUnavailable
	}
	return map[string]any{"bundle_ref": pkg.BundleKey, "bundle_hash": pkg.BundleHash}, nil
}

// submitPackageWithReport 提交已完成后端扫描的仿真包并保存审核预览报告。
func (s *Service) submitPackageWithReport(ctx context.Context, req SubmitPackageRequest, previewReport map[string]any) (PackageDTO, error) {
	if err := validateSubmitPackageRequest(req); err != nil {
		return PackageDTO{}, err
	}
	submitter, ok := tenantFromContext(ctx)
	if !ok {
		return PackageDTO{}, apperr.ErrUnauthorized
	}
	compute, err := parseCompute(req.Compute)
	if err != nil {
		return PackageDTO{}, err
	}
	if compute == ComputeBackend && !s.backend.Exists(strings.TrimSpace(req.BackendAdapter)) {
		return PackageDTO{}, apperr.ErrSimBackendUnavailable
	}
	scaleLimit, err := jsonx.ObjectBytes(req.ScaleLimit, apperr.ErrSimPackageValidationFail)
	if err != nil {
		return PackageDTO{}, err
	}
	backendConfig, err := jsonx.ObjectBytes(req.BackendConfig, apperr.ErrSimPackageValidationFail)
	if err != nil {
		return PackageDTO{}, err
	}
	preview, err := jsonx.ObjectBytes(previewReport, apperr.ErrSimPackageValidationFail)
	if err != nil {
		return PackageDTO{}, err
	}
	authorID, _ := ids.Parse(req.AuthorID)
	rowID := s.idgen.Generate()
	var row sqlcgen.SimPackage
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		created, e := q.CreateSimPackage(ctx, sqlcgen.CreateSimPackageParams{
			ID: rowID, Code: req.Code, Version: req.Version, Name: req.Name, Category: req.Category, Compute: compute,
			ScaleLimit: scaleLimit, BundleKey: req.BundleKey, BundleHash: strings.ToLower(req.BundleHash),
			BackendAdapter: pgText(req.BackendAdapter), BackendConfig: backendConfig,
			AuthorType: req.AuthorType, AuthorID: pgInt8(authorID), Status: PackageStatusReviewing,
		})
		if e != nil {
			return e
		}
		row = created
		_, e = q.CreateSimPackageReview(ctx, sqlcgen.CreateSimPackageReviewParams{
			ID: s.idgen.Generate(), PackageID: row.ID, SubmitterID: submitter.AccountID,
			PreviewReport: preview,
		})
		return e
	}); err != nil {
		return PackageDTO{}, apperr.ErrSimPackageVersionConflict.WithCause(err)
	}
	if err := s.writeAudit(ctx, 0, auditActionPackageSubmit, auditTargetPackage, row.ID, map[string]any{"code": row.Code, "version": row.Version}); err != nil {
		return PackageDTO{}, err
	}
	return packageToDTO(row), nil
}

// SubmitUploadedPackage 提交已由后端扫描并上传的 bundle。
func (s *Service) SubmitUploadedPackage(ctx context.Context, req SubmitPackageRequest, scanReport map[string]any) (PackageDTO, error) {
	report := buildPreviewReport(req)
	for key, value := range scanReport {
		report[key] = value
	}
	return s.submitPackageWithReport(ctx, req, report)
}

// UpdateUploadedPackage 更新已由上传处理路径完成扫描和对象存储写入的草稿/退回仿真包。
func (s *Service) UpdateUploadedPackage(ctx context.Context, packageID int64, req UpdatePackageRequest, scanReport map[string]any) (PackageDTO, error) {
	actor, ok := tenantFromContext(ctx)
	if !ok {
		return PackageDTO{}, apperr.ErrUnauthorized
	}
	if scanReport == nil {
		return PackageDTO{}, apperr.ErrSimPackageValidationFail
	}
	var current sqlcgen.SimPackage
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.GetSimPackageByID(ctx, packageID)
		current = row
		return e
	}); err != nil {
		return PackageDTO{}, apperr.ErrSimPackageNotFound.WithCause(err)
	}
	if err := validatePackageAuthorAccess(actor, current); err != nil {
		return PackageDTO{}, err
	}
	if err := validateUpdatePackageRequest(req, current.Compute); err != nil {
		return PackageDTO{}, err
	}
	if current.Compute == ComputeBackend && !s.backend.Exists(strings.TrimSpace(req.BackendAdapter)) {
		return PackageDTO{}, apperr.ErrSimBackendUnavailable
	}
	scaleLimit, err := jsonx.ObjectBytes(req.ScaleLimit, apperr.ErrSimPackageValidationFail)
	if err != nil {
		return PackageDTO{}, err
	}
	backendConfig, err := jsonx.ObjectBytes(req.BackendConfig, apperr.ErrSimPackageValidationFail)
	if err != nil {
		return PackageDTO{}, err
	}
	report := buildPreviewReport(SubmitPackageRequest{
		Code: current.Code, Version: current.Version, Name: req.Name, Category: req.Category,
		Compute: computeString(current.Compute), ScaleLimit: req.ScaleLimit,
		BundleKey: req.BundleKey, BundleHash: req.BundleHash, BackendAdapter: req.BackendAdapter,
		BackendConfig: req.BackendConfig, AuthorType: current.AuthorType,
		AuthorID: ids.Format(current.AuthorID.Int64),
	})
	for key, value := range scanReport {
		report[key] = value
	}
	preview, err := jsonx.ObjectBytes(report, apperr.ErrSimPackageValidationFail)
	if err != nil {
		return PackageDTO{}, err
	}
	var row sqlcgen.SimPackage
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		updated, e := q.UpdateSimPackageDraft(ctx, sqlcgen.UpdateSimPackageDraftParams{
			ID: packageID, Name: req.Name, Category: req.Category, ScaleLimit: scaleLimit,
			BundleKey: req.BundleKey, BundleHash: strings.ToLower(req.BundleHash), BackendAdapter: pgText(req.BackendAdapter),
			BackendConfig: backendConfig,
		})
		if e != nil {
			return e
		}
		row = updated
		_, e = q.CreateSimPackageReview(ctx, sqlcgen.CreateSimPackageReviewParams{
			ID: s.idgen.Generate(), PackageID: row.ID, SubmitterID: actor.AccountID,
			PreviewReport: preview,
		})
		return e
	}); err != nil {
		return PackageDTO{}, apperr.ErrSimPackageUnavailable.WithCause(err)
	}
	if err := s.writeAudit(ctx, 0, auditActionPackageUpdate, auditTargetPackage, row.ID, map[string]any{"id": ids.Format(row.ID)}); err != nil {
		return PackageDTO{}, err
	}
	return packageToDTO(row), nil
}

// GetPackagePreview 读取包预览所需的真实元数据,由前端 Worker 预览和审核流程使用。
func (s *Service) GetPackagePreview(ctx context.Context, packageID int64) (map[string]any, error) {
	actor, ok := tenantFromContext(ctx)
	if !ok {
		return nil, apperr.ErrUnauthorized
	}
	var row sqlcgen.SimPackage
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.GetSimPackageByID(ctx, packageID)
		row = found
		return err
	}); err != nil {
		return nil, apperr.ErrSimPackageNotFound.WithCause(err)
	}
	if err := validatePackageAuthorAccess(actor, row); err != nil {
		return nil, err
	}
	return map[string]any{
		"package_id":      ids.Format(row.ID),
		"code":            row.Code,
		"version":         row.Version,
		"compute":         computeString(row.Compute),
		"scale_limit":     jsonx.ObjectMap(row.ScaleLimit),
		"bundle_ref":      row.BundleKey,
		"bundle_hash":     row.BundleHash,
		"backend_adapter": row.BackendAdapter.String,
		"author_type":     row.AuthorType,
		"author_id":       ids.Format(row.AuthorID.Int64),
	}, nil
}

// ListReviews 查询仿真包审核列表。
func (s *Service) ListReviews(ctx context.Context, result int16) ([]ReviewDTO, error) {
	var rows []sqlcgen.SimPackageReview
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListSimReviews(ctx, sqlcgen.ListSimReviewsParams{Limit: 100, Offset: 0, Result: pgInt2(result)})
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrSimReviewQueryFailed.WithCause(err)
	}
	return reviewsToDTO(rows), nil
}

// ApproveReview 审核通过仿真包并上架锁定版本。
func (s *Service) ApproveReview(ctx context.Context, reviewID int64, comment string) (ReviewDTO, error) {
	reviewer, ok := tenantFromContext(ctx)
	if !ok {
		return ReviewDTO{}, apperr.ErrUnauthorized
	}
	var row sqlcgen.SimPackageReview
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		current, e := q.GetSimReviewByID(ctx, reviewID)
		if db.IsNoRows(e) {
			return apperr.ErrSimReviewNotFound
		}
		if e != nil {
			return e
		}
		if !previewReportPassed(jsonx.ObjectMap(current.PreviewReport)) {
			return apperr.ErrSimPackageValidationFail
		}
		review, e := q.CompleteSimReview(ctx, sqlcgen.CompleteSimReviewParams{ID: reviewID, Result: ReviewResultApproved, ReviewerID: pgInt8(reviewer.AccountID), Comment: pgText(comment)})
		if db.IsNoRows(e) {
			return apperr.ErrSimReviewInvalidState
		}
		if e != nil {
			return e
		}
		row = review
		_, e = q.UpdateSimPackageStatus(ctx, sqlcgen.UpdateSimPackageStatusParams{ID: review.PackageID, Status: PackageStatusPublished})
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ReviewDTO{}, ae
		}
		return ReviewDTO{}, apperr.ErrSimReviewNotFound.WithCause(err)
	}
	if err := s.writeAudit(ctx, 0, auditActionReviewApprove, auditTargetReview, row.ID, map[string]any{"package_id": ids.Format(row.PackageID)}); err != nil {
		return ReviewDTO{}, err
	}
	return reviewToDTO(row), nil
}

// UpdateValidationReport 保存受控预览流程回写的审核报告。
func (s *Service) UpdateValidationReport(ctx context.Context, packageID int64, report map[string]any) (ReviewDTO, error) {
	if report == nil {
		return ReviewDTO{}, apperr.ErrSimPackageValidationFail
	}
	var row sqlcgen.SimPackageReview
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		current, err := q.GetPendingSimReviewByPackageID(ctx, packageID)
		if db.IsNoRows(err) {
			return apperr.ErrSimReviewNotFound
		}
		if err != nil {
			return err
		}
		merged, err := mergeValidationReport(jsonx.ObjectMap(current.PreviewReport), report)
		if err != nil {
			return err
		}
		reportData, err := jsonx.ObjectBytes(merged, apperr.ErrSimPackageValidationFail)
		if err != nil {
			return err
		}
		updated, err := q.UpdateSimReviewPreviewReport(ctx, sqlcgen.UpdateSimReviewPreviewReportParams{
			PackageID:     packageID,
			PreviewReport: reportData,
		})
		if db.IsNoRows(err) {
			return apperr.ErrSimReviewNotFound
		}
		row = updated
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ReviewDTO{}, ae
		}
		return ReviewDTO{}, apperr.ErrSimReviewNotFound.WithCause(err)
	}
	return reviewToDTO(row), nil
}

// RejectReview 审核退回仿真包并记录审核意见。
func (s *Service) RejectReview(ctx context.Context, reviewID int64, comment string) (ReviewDTO, error) {
	reviewer, ok := tenantFromContext(ctx)
	if !ok {
		return ReviewDTO{}, apperr.ErrUnauthorized
	}
	var row sqlcgen.SimPackageReview
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		review, e := q.CompleteSimReview(ctx, sqlcgen.CompleteSimReviewParams{ID: reviewID, Result: ReviewResultRejected, ReviewerID: pgInt8(reviewer.AccountID), Comment: pgText(comment)})
		if db.IsNoRows(e) {
			return apperr.ErrSimReviewInvalidState
		}
		if e != nil {
			return e
		}
		row = review
		_, e = q.UpdateSimPackageStatus(ctx, sqlcgen.UpdateSimPackageStatusParams{ID: review.PackageID, Status: PackageStatusRejected})
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ReviewDTO{}, ae
		}
		return ReviewDTO{}, apperr.ErrSimReviewNotFound.WithCause(err)
	}
	if err := s.writeAudit(ctx, 0, auditActionReviewReject, auditTargetReview, row.ID, map[string]any{"package_id": ids.Format(row.PackageID)}); err != nil {
		return ReviewDTO{}, err
	}
	return reviewToDTO(row), nil
}

// CreateSession 创建仿真会话并锁定仿真包版本。
func (s *Service) CreateSession(ctx context.Context, tenantID int64, req CreateSessionRequest) (SessionDTO, error) {
	if err := validateCreateSessionRequest(req); err != nil {
		return SessionDTO{}, err
	}
	if err := validateSimSourceRefAccess(ctx, req.SourceRef); err != nil {
		return SessionDTO{}, err
	}
	ownerID, _ := ids.Parse(req.OwnerAccountID)
	pkg, err := s.loadPackage(ctx, req.PackageCode, req.Version)
	if err != nil {
		return SessionDTO{}, err
	}
	if pkg.Status != PackageStatusPublished {
		return SessionDTO{}, apperr.ErrSimPackageUnavailable
	}
	initParams, err := jsonx.ObjectBytes(req.InitParams, apperr.ErrSimSessionInvalid)
	if err != nil {
		return SessionDTO{}, err
	}
	var row sqlcgen.SimSession
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		created, e := q.CreateSimSession(ctx, sqlcgen.CreateSimSessionParams{
			ID: s.idgen.Generate(), TenantID: tenantID, PackageID: pkg.ID, SourceRef: req.SourceRef,
			OwnerAccountID: ownerID, Seed: req.Seed, InitParams: initParams, Compute: pkg.Compute,
		})
		row = created
		return e
	}); err != nil {
		return SessionDTO{}, apperr.ErrSimSessionInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, tenantID, auditActionSessionCreate, auditTargetSession, row.ID, map[string]any{"source_ref": row.SourceRef}); err != nil {
		return SessionDTO{}, err
	}
	return sessionToDTO(row, pkg), nil
}

// ReportAction 按连续序号记录仿真操作序列,支持同内容幂等重试。
func (s *Service) ReportAction(ctx context.Context, sessionID int64, req ReportActionRequest) (ActionDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ActionDTO{}, apperr.ErrUnauthorized
	}
	var out sqlcgen.SimActionLog
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		payload, e := jsonx.ObjectBytes(req.Payload, apperr.ErrSimActionInvalid)
		if e != nil {
			return e
		}
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
		if e = authorizeSessionOwner(id, session.OwnerAccountID); e != nil {
			return e
		}
		lastSeq, existing, e := s.loadActionCursor(ctx, q, sessionID, req.Seq)
		if e != nil {
			return e
		}
		if e = validateNextAction(lastSeq, existing, req); e != nil {
			return e
		}
		if existing != nil {
			out, e = actionDTOToRow(*existing, id.TenantID, sessionID)
			return e
		}
		out, e = q.CreateSimAction(ctx, sqlcgen.CreateSimActionParams{
			ID: s.idgen.Generate(), TenantID: id.TenantID, SessionID: sessionID, Seq: req.Seq,
			AtTick: req.AtTick, EventType: req.EventType, Payload: payload,
		})
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ActionDTO{}, ae
		}
		return ActionDTO{}, apperr.ErrSimActionInvalid.WithCause(err)
	}
	return actionToDTO(out), nil
}

// GetReplay 读取会话 seed、初始参数和有序操作序列。
func (s *Service) GetReplay(ctx context.Context, sessionID int64) (ReplayDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ReplayDTO{}, apperr.ErrUnauthorized
	}
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		session, e := q.GetSimSessionByID(ctx, sessionID)
		if db.IsNoRows(e) {
			return apperr.ErrSimSessionNotFound
		}
		if e != nil {
			return e
		}
		return authorizeSessionOwner(id, session.OwnerAccountID)
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ReplayDTO{}, ae
		}
		return ReplayDTO{}, apperr.ErrSimSessionNotFound.WithCause(err)
	}
	return s.replayInTenant(ctx, id.TenantID, sessionID)
}

// RecycleBySourceRef 按来源标识归档会话,供 M6/M7/M8 等上层编排调用。
func (s *Service) RecycleBySourceRef(ctx context.Context, tenantID int64, sourceRef, reason string) error {
	if !sourceRefRe.MatchString(sourceRef) {
		return apperr.ErrSimSessionInvalid
	}
	if err := validateSimSourceRefAccess(ctx, sourceRef); err != nil {
		return err
	}
	var archived []sqlcgen.SimSession
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		rows, err := q.ArchiveSimSessionsBySourceRef(ctx, sourceRef)
		archived = rows
		return err
	}); err != nil {
		return apperr.ErrSimSessionInvalidState.WithCause(err)
	}
	for _, row := range archived {
		if err := s.writeAudit(ctx, tenantID, auditActionSessionArchive, auditTargetSession, row.ID, map[string]any{"source_ref": sourceRef, "reason": reason}); err != nil {
			return err
		}
		if err := s.publishSessionEnded(ctx, tenantID, row.ID, row.SourceRef, reason); err != nil {
			return err
		}
	}
	return nil
}

// ArchiveSession 归档单个仿真会话。
func (s *Service) ArchiveSession(ctx context.Context, sessionID int64) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	var row sqlcgen.SimSession
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		current, err := q.GetSimSessionByID(ctx, sessionID)
		if db.IsNoRows(err) {
			return apperr.ErrSimSessionNotFound
		}
		if err != nil {
			return err
		}
		if err := validateSimSourceRefAccess(ctx, current.SourceRef); err != nil {
			return err
		}
		archived, err := q.ArchiveSimSession(ctx, sessionID)
		row = archived
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrSimSessionNotFound.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionSessionArchive, auditTargetSession, row.ID, map[string]any{"source_ref": row.SourceRef}); err != nil {
		return err
	}
	return s.publishSessionEnded(ctx, id.TenantID, row.ID, row.SourceRef, "manual")
}

// ShareSession 创建分享码,分享内容仍由会话和操作序列重建。
func (s *Service) ShareSession(ctx context.Context, sessionID int64) (ShareDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ShareDTO{}, apperr.ErrUnauthorized
	}
	code, err := newShareCode()
	if err != nil {
		return ShareDTO{}, apperr.ErrSimShareCodeGenerate.WithCause(err)
	}
	var row sqlcgen.SimShare
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		session, e := q.GetSimSessionByID(ctx, sessionID)
		if db.IsNoRows(e) {
			return apperr.ErrSimSessionNotFound
		} else if e != nil {
			return e
		}
		if e = authorizeSessionOwner(id, session.OwnerAccountID); e != nil {
			return e
		}
		created, e := q.CreateSimShare(ctx, sqlcgen.CreateSimShareParams{
			ID: s.idgen.Generate(), TenantID: id.TenantID, SessionID: sessionID, Code: code,
			CreatedBy: id.AccountID, ExpireAt: pgtype.Timestamptz{},
		})
		row = created
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ShareDTO{}, ae
		}
		return ShareDTO{}, apperr.ErrSimShareInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionShareCreate, auditTargetShare, row.ID, map[string]any{"session_id": ids.Format(sessionID)}); err != nil {
		return ShareDTO{}, err
	}
	return ShareDTO{Code: row.Code}, nil
}

// GetSharedReplay 按分享码读取可复现剧本。
func (s *Service) GetSharedReplay(ctx context.Context, code string) (ReplayDTO, error) {
	var share sqlcgen.SimShare
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, err := q.GetSimShareByCode(ctx, code)
		share = row
		return err
	}); err != nil {
		return ReplayDTO{}, apperr.ErrSimShareInvalid.WithCause(err)
	}
	return s.replayInTenant(ctx, share.TenantID, share.SessionID)
}

// ReportCheckpoint 保存叙事设问或目标达成结果快照。
func (s *Service) ReportCheckpoint(ctx context.Context, sessionID int64, req ReportCheckpointRequest) error {
	if err := validateCheckpointRequest(req); err != nil {
		return err
	}
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	return s.reportCheckpointInTenant(ctx, id.TenantID, sessionID, req)
}

// reportCheckpointInTenant 在显式租户 RLS 下保存检查点,供 HTTP 与 contracts 共用。
func (s *Service) reportCheckpointInTenant(ctx context.Context, tenantID, sessionID int64, req ReportCheckpointRequest) error {
	if err := validateCheckpointRequest(req); err != nil {
		return err
	}
	answer, err := jsonx.ObjectBytes(req.Answer, apperr.ErrSimCheckpointInvalid)
	if err != nil {
		return err
	}
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		session, e := q.GetSimSessionByID(ctx, sessionID)
		if db.IsNoRows(e) {
			return apperr.ErrSimSessionNotFound
		} else if e != nil {
			return e
		}
		if e := validateSimSourceRefAccess(ctx, session.SourceRef); e != nil {
			return e
		}
		_, e = q.UpsertSimCheckpoint(ctx, sqlcgen.UpsertSimCheckpointParams{
			ID: s.idgen.Generate(), TenantID: tenantID, SessionID: sessionID,
			CheckpointID: req.CheckpointID, Answer: answer, Achieved: req.Achieved,
		})
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrSimCheckpointInvalid.WithCause(err)
	}
	return nil
}

// publishSessionEnded 通过事件总线通知上层来源仿真会话已结束,低层不直接回调业务模块。
func (s *Service) publishSessionEnded(ctx context.Context, tenantID, sessionID int64, sourceRef, reason string) error {
	if s.bus == nil {
		return apperr.ErrSimEventPublish
	}
	if err := s.bus.Publish(ctx, contracts.SubjectSimSessionEnded, contracts.SimSessionEndedEvent{
		TenantID: tenantID, SessionID: sessionID, SourceRef: sourceRef, Reason: reason,
	}); err != nil {
		return apperr.ErrSimEventPublish.WithCause(err)
	}
	return nil
}

// loadPackage 读取指定仿真包版本。
func (s *Service) loadPackage(ctx context.Context, code, version string) (sqlcgen.SimPackage, error) {
	var row sqlcgen.SimPackage
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.GetSimPackageByCodeVersion(ctx, sqlcgen.GetSimPackageByCodeVersionParams{Code: code, Version: version})
		row = found
		return err
	}); err != nil {
		return sqlcgen.SimPackage{}, apperr.ErrSimPackageNotFound.WithCause(err)
	}
	return row, nil
}

// loadActionCursor 读取当前会话操作游标和同序号记录。
func (s *Service) loadActionCursor(ctx context.Context, q *sqlcgen.Queries, sessionID int64, seq int32) (int32, *ActionDTO, error) {
	existing, err := q.GetSimActionBySeq(ctx, sqlcgen.GetSimActionBySeqParams{SessionID: sessionID, Seq: seq})
	if err == nil {
		dto := actionToDTO(existing)
		return seq, &dto, nil
	}
	if err != nil && !db.IsNoRows(err) {
		return 0, nil, err
	}
	last, err := q.GetLastSimAction(ctx, sessionID)
	if db.IsNoRows(err) {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, err
	}
	return last.Seq, nil, nil
}

// replayInTenant 在指定租户 RLS 下重建回放数据。
func (s *Service) replayInTenant(ctx context.Context, tenantID, sessionID int64) (ReplayDTO, error) {
	var row sqlcgen.GetSimSessionWithPackageRow
	var actions []sqlcgen.SimActionLog
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, e := q.GetSimSessionWithPackage(ctx, sessionID)
		if db.IsNoRows(e) {
			return apperr.ErrSimSessionNotFound
		}
		if e != nil {
			return e
		}
		row = found
		if e := validateSimSourceRefAccess(ctx, row.SourceRef); e != nil {
			return e
		}
		actions, e = q.ListSimActions(ctx, sessionID)
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ReplayDTO{}, ae
		}
		return ReplayDTO{}, apperr.ErrSimReplayReadFailed.WithCause(err)
	}
	return replayFromRows(row, actions), nil
}

// buildPreviewReport 生成审核预检报告;具体 Worker/CSP 动态预览由前端与平台审核流程承接。
func buildPreviewReport(req SubmitPackageRequest) map[string]any {
	return map[string]any{
		"metadata_validation": "passed",
		"review_required":     true,
		"bundle_hash":         strings.ToLower(req.BundleHash),
		"compute":             req.Compute,
	}
}

// mergeValidationReport 合并受控预览的动态结果,并保护上传扫描生成的权威字段。
func mergeValidationReport(current, incoming map[string]any) (map[string]any, error) {
	if current == nil || incoming == nil {
		return nil, apperr.ErrSimPackageValidationFail
	}
	merged := make(map[string]any, len(current)+len(incoming))
	for key, value := range current {
		merged[key] = value
	}
	for key, value := range incoming {
		name := strings.TrimSpace(key)
		if name == "" || protectedValidationReportKeys[name] || !dynamicValidationReportKeys[name] {
			return nil, apperr.ErrSimPackageValidationFail
		}
		if !validDynamicValidationReportValue(name, value) {
			return nil, apperr.ErrSimPackageValidationFail
		}
		merged[name] = value
	}
	return merged, nil
}

var protectedValidationReportKeys = map[string]bool{
	"metadata_validation": true,
	"review_required":     true,
	"bundle_hash":         true,
	"compute":             true,
	"static_scan":         true,
	"file":                true,
	"file_count":          true,
}

var dynamicValidationReportKeys = map[string]bool{
	"determinism_check":  true,
	"determinism_detail": true,
	"worker_preview":     true,
	"worker_detail":      true,
	"checked_at":         true,
}

// validDynamicValidationReportValue 校验动态审核结果字段,避免任意 JSON 被塞进审核报告。
func validDynamicValidationReportValue(name string, value any) bool {
	switch name {
	case "determinism_check", "worker_preview":
		v, ok := value.(string)
		return ok && (v == "passed" || v == "failed")
	default:
		return value != nil
	}
}

// previewReportPassed 判断审核报告是否满足上架前置条件。
func previewReportPassed(report map[string]any) bool {
	return report["metadata_validation"] == "passed" &&
		report["static_scan"] == "passed" &&
		report["determinism_check"] == "passed" &&
		report["worker_preview"] == "passed"
}

// packageToDTO 转换仿真包数据库行。
func packageToDTO(row sqlcgen.SimPackage) PackageDTO {
	return PackageDTO{ID: ids.Format(row.ID), Code: row.Code, Version: row.Version, Name: row.Name, Category: row.Category,
		Compute: computeString(row.Compute), ScaleLimit: jsonx.ObjectMap(row.ScaleLimit), BundleHash: row.BundleHash,
		BackendAdapter: row.BackendAdapter.String, Status: row.Status}
}

// packagesToDTO 批量转换仿真包数据库行。
func packagesToDTO(rows []sqlcgen.SimPackage) []PackageDTO {
	out := make([]PackageDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, packageToDTO(row))
	}
	return out
}

// reviewToDTO 转换审核记录数据库行。
func reviewToDTO(row sqlcgen.SimPackageReview) ReviewDTO {
	dto := ReviewDTO{ID: ids.Format(row.ID), PackageID: ids.Format(row.PackageID), SubmitterID: ids.Format(row.SubmitterID),
		PreviewReport: jsonx.ObjectMap(row.PreviewReport), Result: row.Result}
	if row.ReviewerID.Valid {
		dto.ReviewerID = ids.Format(row.ReviewerID.Int64)
	}
	if row.Comment.Valid {
		dto.Comment = row.Comment.String
	}
	return dto
}

// reviewsToDTO 批量转换审核记录。
func reviewsToDTO(rows []sqlcgen.SimPackageReview) []ReviewDTO {
	out := make([]ReviewDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, reviewToDTO(row))
	}
	return out
}

// sessionToDTO 转换会话与包信息为创建响应。
func sessionToDTO(row sqlcgen.SimSession, pkg sqlcgen.SimPackage) SessionDTO {
	return SessionDTO{SessionID: ids.Format(row.ID), Compute: computeString(row.Compute), BundleRef: pkg.BundleKey,
		PackageCode: pkg.Code, Version: pkg.Version, Seed: row.Seed, InitParams: jsonx.ObjectMap(row.InitParams), Status: row.Status}
}

// actionToDTO 转换操作日志数据库行。
func actionToDTO(row sqlcgen.SimActionLog) ActionDTO {
	return ActionDTO{Seq: row.Seq, AtTick: row.AtTick, EventType: row.EventType, Payload: jsonx.ObjectMap(row.Payload), CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// actionDTOToRow 把幂等命中的 DTO 转为统一返回路径使用的行结构。
func actionDTOToRow(dto ActionDTO, tenantID, sessionID int64) (sqlcgen.SimActionLog, error) {
	payload, err := jsonx.ObjectBytes(dto.Payload, apperr.ErrSimActionInvalid)
	if err != nil {
		return sqlcgen.SimActionLog{}, err
	}
	return sqlcgen.SimActionLog{TenantID: tenantID, SessionID: sessionID, Seq: dto.Seq, AtTick: dto.AtTick, EventType: dto.EventType, Payload: payload}, nil
}

// replayFromRows 组装回放响应。
func replayFromRows(row sqlcgen.GetSimSessionWithPackageRow, actions []sqlcgen.SimActionLog) ReplayDTO {
	out := ReplayDTO{PackageCode: row.PackageCode, Version: row.PackageVersion, Seed: row.Seed, InitParams: jsonx.ObjectMap(row.InitParams), Actions: []ActionDTO{}}
	for _, action := range actions {
		out.Actions = append(out.Actions, actionToDTO(action))
	}
	return out
}

// newShareCode 生成不可从 session_id 推导的分享码。
func newShareCode() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成仿真分享码失败: %w", err)
	}
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(buf), "="), nil
}

// pgText 把可选字符串转换为 pgtype.Text。
func pgText(v string) pgtype.Text {
	return pgtype.Text{String: strings.TrimSpace(v), Valid: strings.TrimSpace(v) != ""}
}

// pgInt8 把可选 int64 转为 pgtype.Int8。
func pgInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: v > 0}
}

// pgInt2 把可选 int16 转为 pgtype.Int2。
func pgInt2(v int16) pgtype.Int2 {
	return pgtype.Int2{Int16: v, Valid: v > 0}
}
