// sim service_package 文件实现仿真包注册、bundle 读取、动态校验和审核状态机。
package sim

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// ListPackages 返回仿真包分页列表。
// authorID > 0 时只返回该教师作者提交的包(教师维护自己的仿真场景),
// 为 0 时返回全部符合状态过滤的包(学生与教师浏览可用场景)。
func (s *Service) ListPackages(ctx context.Context, status int16, category, keyword string, authorID int64, page, size int) ([]SimPackageResponse, int64, int, int, map[string]map[string]int64, error) {
	page, size = pagex.Normalize(page, size)
	limit, offset := pagex.LimitOffset(page, size)
	var items []Package
	var total int64
	var byCategory map[string]int64
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListPackages(ctx, status, strings.TrimSpace(category), strings.TrimSpace(keyword), authorID, limit, offset)
		if err != nil {
			return err
		}
		byCategory, err = tx.CountPackagesByCategory(ctx, status, strings.TrimSpace(category), strings.TrimSpace(keyword), authorID)
		return err
	}); err != nil {
		return nil, 0, page, size, nil, lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
	}
	out := make([]SimPackageResponse, 0, len(items))
	for _, item := range items {
		mapped, err := packageToResponse(item)
		if err != nil {
			return nil, 0, page, size, nil, err
		}
		out = append(out, mapped)
	}
	return out, total, page, size, map[string]map[string]int64{"category": byCategory}, nil
}

// ListPackageVersions 返回指定 code 的全部版本。
func (s *Service) ListPackageVersions(ctx context.Context, code string) ([]SimPackageResponse, error) {
	if !simCodePattern.MatchString(strings.TrimSpace(code)) {
		return nil, apperr.ErrSimPackageInvalid
	}
	var items []Package
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListPackageVersions(ctx, strings.TrimSpace(code))
		return err
	}); err != nil {
		return nil, lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
	}
	out := make([]SimPackageResponse, 0, len(items))
	for _, item := range items {
		if item.Status != PackageStatusPublished {
			continue
		}
		mapped, err := packageToResponse(item)
		if err != nil {
			return nil, err
		}
		out = append(out, mapped)
	}
	return out, nil
}

// SubmitPackage 上传仿真包、执行静态校验并创建审核记录。
//
// 执行位置与运行能力在此派生而非由请求提供:教师提交的包是外部代码,恒在隔离容器内执行
// (见 docs/04-仿真可视化引擎/02-架构设计.md §8),绑定平台的扩展包运行器能力。
// 请求结构里没有这两个字段,因此不存在"客户端声明了一个平台无法运行的执行位置"这种状态。
func (s *Service) SubmitPackage(ctx context.Context, tenantID, accountID int64, req SubmitPackageRequest, input BundleInput) (SimPackageSubmissionResponse, error) {
	req = normalizePackageRequest(req)
	if err := validatePackageRequest(req, accountID); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.ensurePackageRunnerAvailable(); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.ensurePackageVersionAvailable(ctx, req.Code, req.Version); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	packageID := s.ids.Generate()
	stored, err := s.storeBundle(ctx, tenantID, accountID, packageID, input, req)
	if err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	scale, err := decodeObject(req.ScaleLimit)
	if err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	pkg := Package{
		ID: packageID, Code: req.Code, Version: req.Version, Name: req.Name, Category: req.Category,
		Compute: ComputeIsolated, ScaleLimit: scale,
		BundleKey: stored.ObjectRef, BundleHash: stored.BundleHash, Entry: stored.Entry,
		BackendAdapter: s.packageRunnerCode, BackendConfig: map[string]any{},
		InteractionSchema: stored.InteractionSchema, CodeTrace: stored.CodeTrace,
		AuthorType: AuthorTeacher, AuthorID: accountID, Status: PackageStatusReviewing,
	}
	var created Package
	var review Review
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.GetPackageByCodeVersion(ctx, pkg.Code, pkg.Version); err == nil {
			return apperr.ErrSimPackageVersionConflict
		} else if !isNoRows(err) {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		var err error
		created, err = tx.CreatePackage(ctx, pkg)
		if err != nil {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		review, err = tx.CreateReview(ctx, s.ids.Generate(), created.ID, accountID, stored.Report)
		if err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.uploadPlannedBundle(ctx, stored.ObjectRef, input); err != nil {
		if rollbackErr := s.markUploadFailed(ctx, created.ID, review.ID); rollbackErr != nil {
			logging.ErrorContext(ctx, "sim package upload rollback failed", rollbackErr.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("package_id", created.ID), slog.Int64("review_id", review.ID))
			return SimPackageSubmissionResponse{}, apperr.ErrSimBundleUnreadable.WithCause(errors.Join(err, rollbackErr))
		}
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.writeAuditFromContext(ctx, tenantID, "sim.package.submit", "sim_package", created.ID, map[string]any{"code": created.Code, "version": created.Version}); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	out, err := packageToResponse(created)
	if err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	reviewOut, err := reviewToResponse(review)
	if err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	return SimPackageSubmissionResponse{SimPackageResponse: out, Review: reviewOut}, nil
}

// UpdatePackage 更新草稿或退回包的新 bundle,并重新进入审核中。
// 不改 compute/backend_adapter/author_type:更新一个包不改变它的作者,也就不该改变执行位置。
func (s *Service) UpdatePackage(ctx context.Context, tenantID, accountID, packageID int64, req SubmitPackageRequest, input BundleInput) (SimPackageSubmissionResponse, error) {
	req = normalizePackageRequest(req)
	if err := validatePackageRequest(req, accountID); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.ensurePackageRunnerAvailable(); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.ensurePackageEditable(ctx, accountID, packageID, req.Code, req.Version); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	stored, err := s.storeBundle(ctx, tenantID, accountID, packageID, input, req)
	if err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	scale, err := decodeObject(req.ScaleLimit)
	if err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	pkg := Package{
		ID: packageID, Name: req.Name, Category: req.Category, ScaleLimit: scale,
		BundleKey: stored.ObjectRef, BundleHash: stored.BundleHash, Entry: stored.Entry,
		InteractionSchema: stored.InteractionSchema, CodeTrace: stored.CodeTrace,
		Status: PackageStatusReviewing,
	}
	var updated Package
	var review Review
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		updated, err = tx.UpdatePackageDraft(ctx, pkg)
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageUnavailable, apperr.ErrSimPackageQueryFailed)
		}
		review, err = tx.CreateReview(ctx, s.ids.Generate(), updated.ID, accountID, stored.Report)
		if err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.uploadPlannedBundle(ctx, stored.ObjectRef, input); err != nil {
		if rollbackErr := s.markUploadFailed(ctx, updated.ID, review.ID); rollbackErr != nil {
			logging.ErrorContext(ctx, "sim package upload rollback failed", rollbackErr.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("package_id", updated.ID), slog.Int64("review_id", review.ID))
			return SimPackageSubmissionResponse{}, apperr.ErrSimBundleUnreadable.WithCause(errors.Join(err, rollbackErr))
		}
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.writeAuditFromContext(ctx, tenantID, "sim.package.update", "sim_package", updated.ID, map[string]any{"code": updated.Code, "version": updated.Version}); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	out, err := packageToResponse(updated)
	if err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	reviewOut, err := reviewToResponse(review)
	if err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	return SimPackageSubmissionResponse{SimPackageResponse: out, Review: reviewOut}, nil
}

// ensurePackageVersionAvailable 在执行昂贵 bundle 扫描前拒绝明显的版本冲突。
func (s *Service) ensurePackageVersionAvailable(ctx context.Context, code, version string) error {
	return s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.GetPackageByCodeVersion(ctx, code, version); err == nil {
			return apperr.ErrSimPackageVersionConflict
		} else if !isNoRows(err) {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		return nil
	})
}

// ensurePackageEditable 在执行昂贵 bundle 扫描前校验包归属和可更新状态。
func (s *Service) ensurePackageEditable(ctx context.Context, accountID, packageID int64, code, version string) error {
	return s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		existing, err := tx.GetPackageByID(ctx, packageID)
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
		}
		if existing.AuthorID != accountID || existing.Code != code || existing.Version != version {
			return apperr.ErrForbidden
		}
		if existing.Status != PackageStatusDraft && existing.Status != PackageStatusRejected {
			return apperr.ErrSimPackageUnavailable
		}
		return nil
	})
}

// PackagePreview 返回作者自己的最新审核报告和包摘要,避免待审报告被任意教师窥探。
func (s *Service) PackagePreview(ctx context.Context, accountID, packageID int64) (SimPackagePreviewResponse, error) {
	if accountID <= 0 {
		return SimPackagePreviewResponse{}, apperr.ErrForbidden
	}
	var pkg Package
	var review Review
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		pkg, err = tx.GetPackageByID(ctx, packageID)
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
		}
		if pkg.AuthorType != AuthorTeacher || pkg.AuthorID != accountID {
			return apperr.ErrForbidden
		}
		review, err = tx.GetLatestReviewForPackage(ctx, packageID)
		if err != nil {
			return lookupError(err, apperr.ErrSimReviewNotFound, apperr.ErrSimReviewQueryFailed)
		}
		if review.SubmitterID != accountID {
			return apperr.ErrForbidden
		}
		return nil
	}); err != nil {
		return SimPackagePreviewResponse{}, err
	}
	pkgOut, err := packageToResponse(pkg)
	if err != nil {
		return SimPackagePreviewResponse{}, err
	}
	reviewOut, err := reviewToResponse(review)
	if err != nil {
		return SimPackagePreviewResponse{}, err
	}
	return SimPackagePreviewResponse{Package: pkgOut, Review: reviewOut}, nil
}

// ListReviews 返回审核分页列表。
func (s *Service) ListReviews(ctx context.Context, result int16, page, size int) ([]SimPackageReviewResponse, int64, int, int, error) {
	page, size = pagex.Normalize(page, size)
	limit, offset := pagex.LimitOffset(page, size)
	var items []ReviewInfo
	var total int64
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListReviews(ctx, result, limit, offset)
		return err
	}); err != nil {
		return nil, 0, page, size, lookupError(err, apperr.ErrSimReviewNotFound, apperr.ErrSimReviewQueryFailed)
	}
	out := make([]SimPackageReviewResponse, 0, len(items))
	for _, item := range items {
		mapped, err := reviewInfoToResponse(item)
		if err != nil {
			return nil, 0, page, size, err
		}
		out = append(out, mapped)
	}
	return out, total, page, size, nil
}

// ApproveReview 通过审核并上架包,要求四项校验全部通过。
func (s *Service) ApproveReview(ctx context.Context, reviewerID, reviewID int64) (SimReviewDecisionResponse, error) {
	var pkg Package
	var review Review
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		review, err = tx.GetReview(ctx, reviewID)
		if err != nil {
			return lookupError(err, apperr.ErrSimReviewNotFound, apperr.ErrSimReviewQueryFailed)
		}
		pkg, err = tx.GetPackageByID(ctx, review.PackageID)
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
		}
		if pkg.Status != PackageStatusReviewing || review.Result != ReviewPending {
			return apperr.ErrSimReviewStateInvalid
		}
		if err := validateApprovalReport(review.PreviewReport, pkg); err != nil {
			return err
		}
		if err := validateBackendAdapterConfig(pkg.Compute, pkg.BackendAdapter, pkg.BackendConfig, s.backends); err != nil {
			return err
		}
		review, err = tx.CompleteReview(ctx, reviewID, ReviewApproved, reviewerID, "")
		if err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		pkg, err = tx.UpdatePackageStatus(ctx, review.PackageID, PackageStatusPublished)
		if err != nil {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return SimReviewDecisionResponse{}, err
	}
	if err := s.writeAuditFromContext(ctx, 0, "sim.package.approve", "sim_package", pkg.ID, map[string]any{"code": pkg.Code, "version": pkg.Version}); err != nil {
		return SimReviewDecisionResponse{}, err
	}
	pkgOut, err := packageToResponse(pkg)
	if err != nil {
		return SimReviewDecisionResponse{}, err
	}
	reviewOut, err := reviewToResponse(review)
	if err != nil {
		return SimReviewDecisionResponse{}, err
	}
	return SimReviewDecisionResponse{Package: pkgOut, Review: reviewOut}, nil
}

// RejectReview 退回审核并写入意见。
func (s *Service) RejectReview(ctx context.Context, reviewerID, reviewID int64, comment string) (SimReviewDecisionResponse, error) {
	if strings.TrimSpace(comment) == "" || len(comment) > 500 {
		return SimReviewDecisionResponse{}, apperr.ErrSimReviewStateInvalid
	}
	var pkg Package
	var review Review
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		review, err = tx.GetReview(ctx, reviewID)
		if err != nil {
			return lookupError(err, apperr.ErrSimReviewNotFound, apperr.ErrSimReviewQueryFailed)
		}
		pkg, err = tx.GetPackageByID(ctx, review.PackageID)
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
		}
		if pkg.Status != PackageStatusReviewing || review.Result != ReviewPending {
			return apperr.ErrSimReviewStateInvalid
		}
		review, err = tx.CompleteReview(ctx, reviewID, ReviewRejected, reviewerID, strings.TrimSpace(comment))
		if err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		pkg, err = tx.UpdatePackageStatus(ctx, review.PackageID, PackageStatusRejected)
		if err != nil {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return SimReviewDecisionResponse{}, err
	}
	if err := s.writeAuditFromContext(ctx, 0, "sim.package.reject", "sim_package", pkg.ID, map[string]any{"code": pkg.Code, "version": pkg.Version}); err != nil {
		return SimReviewDecisionResponse{}, err
	}
	pkgOut, err := packageToResponse(pkg)
	if err != nil {
		return SimReviewDecisionResponse{}, err
	}
	reviewOut, err := reviewToResponse(review)
	if err != nil {
		return SimReviewDecisionResponse{}, err
	}
	return SimReviewDecisionResponse{Package: pkgOut, Review: reviewOut}, nil
}

// ArchivePackage 下架已发布包,不影响历史回放。
func (s *Service) ArchivePackage(ctx context.Context, packageID int64) (SimPackageResponse, error) {
	var pkg Package
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		existing, err := tx.GetPackageByID(ctx, packageID)
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
		}
		if existing.Status != PackageStatusPublished {
			return apperr.ErrSimPackageUnavailable
		}
		pkg, err = tx.UpdatePackageStatus(ctx, packageID, PackageStatusArchived)
		if err != nil {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return SimPackageResponse{}, err
	}
	if err := s.writeAuditFromContext(ctx, 0, "sim.package.archive", "sim_package", pkg.ID, map[string]any{"code": pkg.Code, "version": pkg.Version}); err != nil {
		return SimPackageResponse{}, err
	}
	return packageToResponse(pkg)
}

// RepublishPackage 仅允许已下架包重新上架,不得绕过审核发布草稿或退回包。
func (s *Service) RepublishPackage(ctx context.Context, packageID int64) (SimPackageResponse, error) {
	var pkg Package
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		existing, err := tx.GetPackageByID(ctx, packageID)
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
		}
		if existing.Status != PackageStatusArchived {
			return apperr.ErrSimPackageUnavailable
		}
		if err := validateBackendAdapterConfig(existing.Compute, existing.BackendAdapter, existing.BackendConfig, s.backends); err != nil {
			return err
		}
		pkg, err = tx.UpdatePackageStatus(ctx, packageID, PackageStatusPublished)
		if err != nil {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return SimPackageResponse{}, err
	}
	if err := s.writeAuditFromContext(ctx, 0, "sim.package.republish", "sim_package", pkg.ID, map[string]any{"code": pkg.Code, "version": pkg.Version}); err != nil {
		return SimPackageResponse{}, err
	}
	return packageToResponse(pkg)
}

// markUploadFailed 回滚业务可见状态,避免对象上传失败后留下可审核的包记录。
func (s *Service) markUploadFailed(ctx context.Context, packageID, reviewID int64) error {
	return s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.CompleteReview(ctx, reviewID, ReviewRejected, 0, "资源上传失败,请重新提交"); err != nil && !isNoRows(err) {
			return err
		}
		_, err := tx.UpdatePackageStatus(ctx, packageID, PackageStatusRejected)
		return err
	})
}

// loadPackage 按 code/version 查询平台级包并归一错误码。
func (s *Service) loadPackage(ctx context.Context, code, version string) (Package, error) {
	if !simCodePattern.MatchString(strings.TrimSpace(code)) || !semverPattern.MatchString(strings.TrimSpace(version)) {
		return Package{}, apperr.ErrSimPackageInvalid
	}
	var pkg Package
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		pkg, err = tx.GetPackageByCodeVersion(ctx, strings.TrimSpace(code), strings.TrimSpace(version))
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
		}
		return nil
	}); err != nil {
		return Package{}, err
	}
	return pkg, nil
}

// decodeObject 在进入数据库前把已通过 rules 校验的 JSON 对象转换为 map。
func decodeObject(raw []byte) (map[string]any, error) {
	out := map[string]any{}
	if err := jsonx.DecodeStrict(raw, &out); err != nil {
		return nil, apperr.ErrSimPackageInvalid.WithCause(err)
	}
	return out, nil
}
