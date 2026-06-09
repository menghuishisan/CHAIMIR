// M4 服务层:承载仿真包管理、审核、会话、操作序列、分享、检查点与后端计算入口。
package sim

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
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
	rows, err := s.repo.listPackages(ctx, category, keyword, status)
	if err != nil {
		return nil, apperr.ErrSimPackageQueryFailed.WithCause(err)
	}
	return packagesToDTO(rows), nil
}

// ListVersions 查询某个仿真包已上架版本。
func (s *Service) ListVersions(ctx context.Context, code string) ([]PackageDTO, error) {
	rows, err := s.repo.listVersions(ctx, code)
	if err != nil {
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

// submitPackageWithReport 提交已完成后端扫描的仿真包并保存审核预览报告,服务层不接收未扫描包体。
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
	// 先序列化配置和扫描报告,审核页只读取这里持久化的服务端预览。
	preview, err := jsonx.ObjectBytes(previewReport, apperr.ErrSimPackageValidationFail)
	if err != nil {
		return PackageDTO{}, err
	}
	rowID := s.idgen.Generate()
	row, err := s.repo.createPackageWithReview(ctx, rowID, s.idgen.Generate(), submitter.AccountID, req, compute, scaleLimit, backendConfig, preview)
	if err != nil {
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
	current, err := s.repo.getPackageByID(ctx, packageID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return PackageDTO{}, ae
		}
		return PackageDTO{}, apperr.ErrSimPackageQueryFailed.WithCause(err)
	}
	if err := validatePackageAuthorAccess(actor, packageAuthorScopeFromSnapshot(current)); err != nil {
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
	// 再用服务端当前元数据重建预览报告,并合并本次上传扫描结果。
	report := buildPreviewReport(SubmitPackageRequest{
		Code: current.Code, Version: current.Version, Name: req.Name, Category: req.Category,
		Compute: computeString(current.Compute), ScaleLimit: req.ScaleLimit,
		BundleKey: req.BundleKey, BundleHash: req.BundleHash, BackendAdapter: req.BackendAdapter,
		BackendConfig: req.BackendConfig, AuthorType: current.AuthorType,
		AuthorID: ids.Format(current.AuthorID),
	})
	for key, value := range scanReport {
		report[key] = value
	}
	preview, err := jsonx.ObjectBytes(report, apperr.ErrSimPackageValidationFail)
	if err != nil {
		return PackageDTO{}, err
	}
	row, err := s.repo.updatePackageDraftWithReview(ctx, packageID, s.idgen.Generate(), actor.AccountID, req, scaleLimit, backendConfig, preview)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return PackageDTO{}, ae
		}
		return PackageDTO{}, apperr.ErrSimPackageUpdateFailed.WithCause(err)
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
	row, err := s.repo.getPackageByID(ctx, packageID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrSimPackageQueryFailed.WithCause(err)
	}
	if err := validatePackageAuthorAccess(actor, packageAuthorScopeFromSnapshot(row)); err != nil {
		return nil, err
	}
	return map[string]any{
		"package_id":      ids.Format(row.ID),
		"code":            row.Code,
		"version":         row.Version,
		"compute":         computeString(row.Compute),
		"scale_limit":     row.ScaleLimit,
		"bundle_ref":      row.BundleKey,
		"bundle_hash":     row.BundleHash,
		"backend_adapter": row.BackendAdapter,
		"author_type":     row.AuthorType,
		"author_id":       ids.Format(row.AuthorID),
	}, nil
}

// ListReviews 查询仿真包审核列表。
func (s *Service) ListReviews(ctx context.Context, result int16) ([]ReviewDTO, error) {
	rows, err := s.repo.listReviews(ctx, result)
	if err != nil {
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
	current, err := s.repo.getReviewByID(ctx, reviewID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ReviewDTO{}, ae
		}
		return ReviewDTO{}, apperr.ErrSimReviewUpdateFailed.WithCause(err)
	}
	if !previewReportPassed(current.PreviewReport) {
		return ReviewDTO{}, apperr.ErrSimPackageValidationFail
	}
	row, err := s.repo.completeReviewWithPackageStatus(ctx, reviewID, reviewer.AccountID, ReviewResultApproved, PackageStatusPublished, comment)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ReviewDTO{}, ae
		}
		return ReviewDTO{}, apperr.ErrSimReviewUpdateFailed.WithCause(err)
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
	current, err := s.repo.getPendingReviewByPackageID(ctx, packageID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ReviewDTO{}, ae
		}
		return ReviewDTO{}, apperr.ErrSimReviewUpdateFailed.WithCause(err)
	}
	merged, err := mergeValidationReport(current.PreviewReport, report)
	if err != nil {
		return ReviewDTO{}, err
	}
	reportData, err := jsonx.ObjectBytes(merged, apperr.ErrSimPackageValidationFail)
	if err != nil {
		return ReviewDTO{}, err
	}
	row, err := s.repo.updateReviewPreviewReport(ctx, packageID, reportData)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ReviewDTO{}, ae
		}
		return ReviewDTO{}, apperr.ErrSimReviewUpdateFailed.WithCause(err)
	}
	return reviewToDTO(row), nil
}

// RejectReview 审核退回仿真包并记录审核意见。
func (s *Service) RejectReview(ctx context.Context, reviewID int64, comment string) (ReviewDTO, error) {
	reviewer, ok := tenantFromContext(ctx)
	if !ok {
		return ReviewDTO{}, apperr.ErrUnauthorized
	}
	row, err := s.repo.completeReviewWithPackageStatus(ctx, reviewID, reviewer.AccountID, ReviewResultRejected, PackageStatusRejected, comment)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ReviewDTO{}, ae
		}
		return ReviewDTO{}, apperr.ErrSimReviewUpdateFailed.WithCause(err)
	}
	if err := s.writeAudit(ctx, 0, auditActionReviewReject, auditTargetReview, row.ID, map[string]any{"package_id": ids.Format(row.PackageID)}); err != nil {
		return ReviewDTO{}, err
	}
	return reviewToDTO(row), nil
}

// ArchivePackage 下架已发布仿真包,历史会话和分享仍保留。
func (s *Service) ArchivePackage(ctx context.Context, packageID int64) (PackageDTO, error) {
	row, err := s.transitionPackageStatus(ctx, packageID, PackageStatusPublished, PackageStatusArchived)
	if err != nil {
		return PackageDTO{}, err
	}
	if err := s.writeAudit(ctx, 0, auditActionPackageArchive, auditTargetPackage, row.ID, map[string]any{"id": ids.Format(row.ID)}); err != nil {
		return PackageDTO{}, err
	}
	return packageToDTO(row), nil
}

// RepublishPackage 重新上架已下架仿真包,不允许绕过审核发布草稿或退回包。
func (s *Service) RepublishPackage(ctx context.Context, packageID int64) (PackageDTO, error) {
	row, err := s.transitionPackageStatus(ctx, packageID, PackageStatusArchived, PackageStatusPublished)
	if err != nil {
		return PackageDTO{}, err
	}
	if err := s.writeAudit(ctx, 0, auditActionPackagePublish, auditTargetPackage, row.ID, map[string]any{"id": ids.Format(row.ID)}); err != nil {
		return PackageDTO{}, err
	}
	return packageToDTO(row), nil
}

// transitionPackageStatus 按期望前置状态变更包状态,防止生命周期越级。
func (s *Service) transitionPackageStatus(ctx context.Context, packageID int64, from, to int16) (PackageSnapshot, error) {
	if s.repo == nil {
		return PackageSnapshot{}, apperr.ErrSimPackageUpdateFailed
	}
	row, err := s.repo.transitionPackageStatus(ctx, packageID, from, to)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return PackageSnapshot{}, ae
		}
		return PackageSnapshot{}, apperr.ErrSimPackageUpdateFailed.WithCause(err)
	}
	return row, nil
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
	row, err := s.repo.createSession(ctx, tenantID, s.idgen.Generate(), ownerID, req, pkg, initParams)
	if err != nil {
		return SessionDTO{}, apperr.ErrSimSessionInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, tenantID, auditActionSessionCreate, auditTargetSession, row.ID, map[string]any{"source_ref": row.SourceRef}); err != nil {
		return SessionDTO{}, err
	}
	return sessionToDTO(row, pkg), nil
}

// ReportAction 按连续序号记录仿真操作序列,支持同内容幂等重试以便前端断线后补发。
func (s *Service) ReportAction(ctx context.Context, sessionID int64, req ReportActionRequest) (ActionDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ActionDTO{}, apperr.ErrUnauthorized
	}
	payload, err := jsonx.ObjectBytes(req.Payload, apperr.ErrSimActionInvalid)
	if err != nil {
		return ActionDTO{}, err
	}
	out, err := s.repo.createActionIfNext(ctx, id.TenantID, s.idgen.Generate(), sessionID, id, req, payload)
	if err != nil {
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
	if err := s.repo.authorizeReplay(ctx, id.TenantID, sessionID, id); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ReplayDTO{}, ae
		}
		return ReplayDTO{}, apperr.ErrSimReplayReadFailed.WithCause(err)
	}
	return s.replayInTenant(ctx, id.TenantID, sessionID)
}

// RecycleBySourceRef 按来源标识归档会话,供 M6/M7/M8 等上层编排调用。
func (s *Service) RecycleBySourceRef(ctx context.Context, tenantID int64, sourceRef, reason string) error {
	if !auth.ValidSourceRef(sourceRef) {
		return apperr.ErrSimSessionInvalid
	}
	if err := validateSimSourceRefAccess(ctx, sourceRef); err != nil {
		return err
	}
	archived, err := s.repo.archiveSessionsBySourceRef(ctx, tenantID, sourceRef)
	if err != nil {
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
	row, err := s.repo.archiveSession(ctx, id.TenantID, sessionID, id)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrSimSessionInvalidState.WithCause(err)
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
	row, err := s.repo.createShare(ctx, id.TenantID, s.idgen.Generate(), sessionID, id.AccountID, id, code)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ShareDTO{}, ae
		}
		return ShareDTO{}, apperr.ErrSimShareCreateFailed.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionShareCreate, auditTargetShare, row.ID, map[string]any{"session_id": ids.Format(sessionID)}); err != nil {
		return ShareDTO{}, err
	}
	return ShareDTO{Code: row.Code}, nil
}

// GetSharedReplay 按分享码读取可复现剧本。
func (s *Service) GetSharedReplay(ctx context.Context, code string) (ReplayDTO, error) {
	share, err := s.repo.getShareByCode(ctx, code)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ReplayDTO{}, ae
		}
		return ReplayDTO{}, apperr.ErrSimShareReadFailed.WithCause(err)
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
	if err := s.repo.upsertCheckpoint(ctx, tenantID, s.idgen.Generate(), sessionID, req, answer); err != nil {
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
func (s *Service) loadPackage(ctx context.Context, code, version string) (PackageSnapshot, error) {
	row, err := s.repo.getPackageByCodeVersion(ctx, code, version)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return PackageSnapshot{}, ae
		}
		return PackageSnapshot{}, apperr.ErrSimPackageQueryFailed.WithCause(err)
	}
	return row, nil
}

// replayInTenant 在指定租户 RLS 下重建回放数据。
func (s *Service) replayInTenant(ctx context.Context, tenantID, sessionID int64) (ReplayDTO, error) {
	row, actions, err := s.repo.replayInTenant(ctx, tenantID, sessionID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ReplayDTO{}, ae
		}
		return ReplayDTO{}, apperr.ErrSimReplayReadFailed.WithCause(err)
	}
	return replayFromSnapshots(row, actions), nil
}

// loadBackendSession 校验会话存在、归属当前租户且运行位置为后端。
func (s *Service) loadBackendSession(ctx context.Context, sessionID int64) (BackendSessionSnapshot, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return BackendSessionSnapshot{}, apperr.ErrUnauthorized
	}
	row, err := s.repo.loadBackendSession(ctx, id.TenantID, sessionID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return BackendSessionSnapshot{}, ae
		}
		return BackendSessionSnapshot{}, apperr.ErrSimBackendUnavailable.WithCause(err)
	}
	if row.Compute != ComputeBackend || !row.HasBackend {
		return BackendSessionSnapshot{}, apperr.ErrSimBackendUnavailable
	}
	if err := authorizeSessionOwner(id, row.OwnerAccountID); err != nil {
		return BackendSessionSnapshot{}, err
	}
	return row, nil
}

// newShareCode 生成不可从 session_id 推导的分享码。
func newShareCode() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成仿真分享码失败: %w", err)
	}
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(buf), "="), nil
}
