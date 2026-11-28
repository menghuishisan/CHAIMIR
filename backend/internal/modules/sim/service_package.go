// sim service_package 文件实现仿真包注册、bundle 读取、动态校验和审核状态机。
package sim

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/storage"
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
	var byStatus map[int16]int64
	var rejections map[int64]string
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListPackages(ctx, status, strings.TrimSpace(category), strings.TrimSpace(keyword), authorID, limit, offset)
		if err != nil {
			return err
		}
		byCategory, err = tx.CountPackagesByCategory(ctx, status, strings.TrimSpace(keyword), authorID)
		if err != nil {
			return err
		}
		// 状态分段计数:套用分类、关键词与作者范围,不套用状态筛选
		byStatus, err = tx.CountPackagesByStatus(ctx, strings.TrimSpace(category), strings.TrimSpace(keyword), authorID)
		if err != nil {
			return err
		}
		// 只有作者本人的视角带退回意见:平台与可用场景视角看不到别人的审核意见
		if authorID == 0 {
			return nil
		}
		rejected := make([]int64, 0, len(items))
		for _, item := range items {
			if item.Status == PackageStatusRejected {
				rejected = append(rejected, item.ID)
			}
		}
		rejections, err = tx.LatestRejections(ctx, rejected)
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
		mapped.RejectComment = rejections[item.ID]
		out = append(out, mapped)
	}
	statusGroup, err := namedFacetGroup(byStatus, packageStatusText)
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	return out, total, page, size, map[string]map[string]int64{"category": byCategory, "status": statusGroup}, nil
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

// SubmitPackage 校验教师包并创建审核记录。扩展包固定使用平台的隔离运行器。
func (s *Service) SubmitPackage(ctx context.Context, tenantID, accountID int64, req SubmitPackageRequest, input BundleInput) (SimPackageSubmissionResponse, error) {
	req = normalizePackageRequest(req)
	scale, err := validatePackageRequest(req, accountID)
	if err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.ensurePackageRunnerAvailable(); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.ensurePackageVersionAvailable(ctx, req.Code, req.Version); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	packageID := s.ids.Generate()
	stored, err := s.prepareBundle(ctx, tenantID, accountID, packageID, input, req, scale)
	if err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.uploadPlannedBundle(ctx, stored.ObjectRef, input); err != nil {
		return SimPackageSubmissionResponse{}, apperr.AsAppError(storage.CompensateUpload(ctx, time.Duration(s.upload.CleanupTimeoutSeconds)*time.Second, err, false, func(ctx context.Context) error {
			return s.discardPackageBundle(ctx, tenantID, packageID, stored.ObjectRef)
		}))
	}
	pkg := Package{
		ID: packageID, Code: req.Code, Version: req.Version, Name: req.Name, Category: req.Category,
		Compute: ComputeIsolated, ScaleLimit: scale,
		BundleKey: stored.ObjectRef, BundleHash: stored.BundleHash, Entry: stored.Entry,
		BackendAdapter: s.packageRunnerCode, BackendConfig: map[string]any{},
		InteractionSchema: stored.InteractionSchema, CodeTrace: stored.CodeTrace,
		AuthorType: AuthorTeacher, AuthorID: accountID, Status: PackageStatusReviewing,
	}
	var out SimPackageSubmissionResponse
	var commitAttempted bool
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.GetPackageByCodeVersion(ctx, pkg.Code, pkg.Version); err == nil {
			return apperr.ErrSimPackageVersionConflict
		} else if !isNoRows(err) {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		created, err := tx.CreatePackage(ctx, pkg)
		if err != nil {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		review, err := tx.CreateReview(ctx, s.ids.Generate(), created.ID, accountID, stored.Report)
		if err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		out.SimPackageResponse, err = packageToResponse(created)
		if err != nil {
			return err
		}
		out.Review, err = reviewToResponse(review)
		if err != nil {
			return err
		}
		if err := s.writeAuditFromContext(ctx, tenantID, "sim.package.submit", "sim_package", created.ID, map[string]any{"code": created.Code, "version": created.Version}); err != nil {
			return err
		}
		commitAttempted = true
		return nil
	}); err != nil {
		err = storage.CompensateUpload(ctx, time.Duration(s.upload.CleanupTimeoutSeconds)*time.Second, err, commitAttempted, func(ctx context.Context) error {
			return s.discardPackageBundle(ctx, tenantID, packageID, stored.ObjectRef)
		})
		return SimPackageSubmissionResponse{}, apperr.AsAppError(err)
	}
	return out, nil
}

// UpdatePackage 更新草稿或退回包的新 bundle,并重新进入审核中。
// 不改 compute/backend_adapter/author_type:更新一个包不改变它的作者,也就不该改变执行位置。
func (s *Service) UpdatePackage(ctx context.Context, tenantID, accountID, packageID int64, req SubmitPackageRequest, input BundleInput) (SimPackageSubmissionResponse, error) {
	req = normalizePackageRequest(req)
	scale, err := validatePackageRequest(req, accountID)
	if err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.ensurePackageRunnerAvailable(); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.ensurePackageEditable(ctx, accountID, packageID, req.Code, req.Version); err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	stored, err := s.prepareBundle(ctx, tenantID, accountID, packageID, input, req, scale)
	if err != nil {
		return SimPackageSubmissionResponse{}, err
	}
	if err := s.uploadPlannedBundle(ctx, stored.ObjectRef, input); err != nil {
		return SimPackageSubmissionResponse{}, apperr.AsAppError(storage.CompensateUpload(ctx, time.Duration(s.upload.CleanupTimeoutSeconds)*time.Second, err, false, func(ctx context.Context) error {
			return s.discardPackageBundle(ctx, tenantID, packageID, stored.ObjectRef)
		}))
	}
	pkg := Package{
		ID: packageID, Name: req.Name, Category: req.Category, ScaleLimit: scale,
		BundleKey: stored.ObjectRef, BundleHash: stored.BundleHash, Entry: stored.Entry,
		InteractionSchema: stored.InteractionSchema, CodeTrace: stored.CodeTrace,
		Status: PackageStatusReviewing,
	}
	var out SimPackageSubmissionResponse
	var oldBundle string
	var commitAttempted bool
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		existing, err := tx.GetPackageForUpdate(ctx, packageID)
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
		}
		if existing.AuthorID != accountID || existing.Code != req.Code || existing.Version != req.Version {
			return apperr.ErrForbidden
		}
		oldBundle = existing.BundleKey
		updated, err := tx.UpdatePackageDraft(ctx, pkg)
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageUnavailable, apperr.ErrSimPackageQueryFailed)
		}
		review, err := tx.CreateReview(ctx, s.ids.Generate(), updated.ID, accountID, stored.Report)
		if err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		out.SimPackageResponse, err = packageToResponse(updated)
		if err != nil {
			return err
		}
		out.Review, err = reviewToResponse(review)
		if err != nil {
			return err
		}
		if err := s.writeAuditFromContext(ctx, tenantID, "sim.package.update", "sim_package", updated.ID, map[string]any{"code": updated.Code, "version": updated.Version}); err != nil {
			return err
		}
		commitAttempted = true
		return nil
	}); err != nil {
		err = storage.CompensateUpload(ctx, time.Duration(s.upload.CleanupTimeoutSeconds)*time.Second, err, commitAttempted, func(ctx context.Context) error {
			return s.discardPackageBundle(ctx, tenantID, packageID, stored.ObjectRef)
		})
		return SimPackageSubmissionResponse{}, apperr.AsAppError(err)
	}
	if oldBundle != "" && oldBundle != stored.ObjectRef {
		if err := s.discardPackageBundle(ctx, tenantID, packageID, oldBundle); err != nil {
			logging.ErrorContext(ctx, "清理已替换仿真包对象失败", err.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("package_id", packageID))
		}
	}
	return out, nil
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
func (s *Service) PackagePreview(ctx context.Context, accountID, packageID int64) (SimPackageReviewDetailResponse, error) {
	if accountID <= 0 {
		return SimPackageReviewDetailResponse{}, apperr.ErrForbidden
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
		return SimPackageReviewDetailResponse{}, err
	}
	pkgOut, err := packageToResponse(pkg)
	if err != nil {
		return SimPackageReviewDetailResponse{}, err
	}
	reviewOut, err := reviewToResponse(review)
	if err != nil {
		return SimPackageReviewDetailResponse{}, err
	}
	return SimPackageReviewDetailResponse{Package: pkgOut, Review: reviewOut}, nil
}

// ListReviews 返回审核分页列表。
func (s *Service) ListReviews(ctx context.Context, result int16, page, size int) ([]SimPackageReviewResponse, int64, int, int, pagex.Facets, error) {
	page, size = pagex.Normalize(page, size)
	limit, offset := pagex.LimitOffset(page, size)
	var items []ReviewInfo
	var total int64
	var byResult map[int16]int64
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListReviews(ctx, result, limit, offset)
		if err != nil {
			return err
		}
		byResult, err = tx.CountReviewsByResult(ctx)
		return err
	}); err != nil {
		return nil, 0, page, size, nil, lookupError(err, apperr.ErrSimReviewNotFound, apperr.ErrSimReviewQueryFailed)
	}
	submitterIDs := make([]int64, 0, len(items))
	for _, item := range items {
		submitterIDs = append(submitterIDs, item.SubmitterID)
	}
	// 提交人是各校教师,平台身份经身份契约一次批量解析姓名
	names, err := s.identity.PlatformAccountNames(ctx, submitterIDs)
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	out := make([]SimPackageReviewResponse, 0, len(items))
	for _, item := range items {
		mapped, err := reviewInfoToResponse(item)
		if err != nil {
			return nil, 0, page, size, nil, err
		}
		mapped.SubmitterName = names[item.SubmitterID]
		out = append(out, mapped)
	}
	resultGroup, err := namedFacetGroup(byResult, reviewResultText)
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	return out, total, page, size, pagex.Facets{"result": resultGroup}, nil
}

// ApproveReview 在运行门禁通过后由平台人工审核上架，保留扫描风险提示。
func (s *Service) ApproveReview(ctx context.Context, reviewerID, reviewID int64) (SimPackageReviewDetailResponse, error) {
	var pkg Package
	var review Review
	var out SimPackageReviewDetailResponse
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
		out.Package, err = packageToResponse(pkg)
		if err != nil {
			return err
		}
		out.Review, err = reviewToResponse(review)
		if err != nil {
			return err
		}
		return s.writeAuditFromContext(ctx, 0, "sim.package.approve", "sim_package", pkg.ID, map[string]any{"code": pkg.Code, "version": pkg.Version})
	}); err != nil {
		return SimPackageReviewDetailResponse{}, err
	}
	return out, nil
}

// RejectReview 退回审核并写入意见。
func (s *Service) RejectReview(ctx context.Context, reviewerID, reviewID int64, comment string) (SimPackageReviewDetailResponse, error) {
	if strings.TrimSpace(comment) == "" || len(comment) > 500 {
		return SimPackageReviewDetailResponse{}, apperr.ErrSimReviewStateInvalid
	}
	var pkg Package
	var review Review
	var out SimPackageReviewDetailResponse
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
		out.Package, err = packageToResponse(pkg)
		if err != nil {
			return err
		}
		out.Review, err = reviewToResponse(review)
		if err != nil {
			return err
		}
		return s.writeAuditFromContext(ctx, 0, "sim.package.reject", "sim_package", pkg.ID, map[string]any{"code": pkg.Code, "version": pkg.Version})
	}); err != nil {
		return SimPackageReviewDetailResponse{}, err
	}
	return out, nil
}

// ArchivePackage 下架已发布包,不影响历史回放。
func (s *Service) ArchivePackage(ctx context.Context, packageID int64) (SimPackageResponse, error) {
	var out SimPackageResponse
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		existing, err := tx.GetPackageForUpdate(ctx, packageID)
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
		}
		if existing.Status != PackageStatusPublished {
			return apperr.ErrSimPackageUnavailable
		}
		pkg, err := tx.UpdatePackageStatus(ctx, packageID, PackageStatusArchived)
		if err != nil {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		out, err = packageToResponse(pkg)
		if err != nil {
			return err
		}
		return s.writeAuditFromContext(ctx, 0, "sim.package.archive", "sim_package", pkg.ID, map[string]any{"code": pkg.Code, "version": pkg.Version})
	}); err != nil {
		return SimPackageResponse{}, err
	}
	return out, nil
}

// RepublishPackage 仅允许已下架包重新上架,不得绕过审核发布草稿或退回包。
func (s *Service) RepublishPackage(ctx context.Context, packageID int64) (SimPackageResponse, error) {
	var out SimPackageResponse
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		existing, err := tx.GetPackageForUpdate(ctx, packageID)
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
		}
		if existing.Status != PackageStatusArchived {
			return apperr.ErrSimPackageUnavailable
		}
		if err := validateBackendAdapterConfig(existing.Compute, existing.BackendAdapter, existing.BackendConfig, s.backends); err != nil {
			return err
		}
		pkg, err := tx.UpdatePackageStatus(ctx, packageID, PackageStatusPublished)
		if err != nil {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		out, err = packageToResponse(pkg)
		if err != nil {
			return err
		}
		return s.writeAuditFromContext(ctx, 0, "sim.package.republish", "sim_package", pkg.ID, map[string]any{"code": pkg.Code, "version": pkg.Version})
	}); err != nil {
		return SimPackageResponse{}, err
	}
	return out, nil
}

func (s *Service) loadRunnablePackage(ctx context.Context, code, version string) (Package, error) {
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
	if pkg.Status != PackageStatusPublished {
		return Package{}, apperr.ErrSimPackageUnavailable
	}
	if _, err := computeText(pkg.Compute); err != nil {
		return Package{}, err
	}
	if err := validateBackendAdapterConfig(pkg.Compute, pkg.BackendAdapter, pkg.BackendConfig, s.backends); err != nil {
		return Package{}, err
	}
	return pkg, nil
}
