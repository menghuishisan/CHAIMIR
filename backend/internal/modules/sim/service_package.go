// sim service_package 文件实现仿真包注册、bundle 读取、动态校验和审核状态机。
package sim

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// ListPackages 返回仿真包分页列表。
func (s *Service) ListPackages(ctx context.Context, status int16, category, keyword string, page, size int) ([]map[string]any, int64, int, int, error) {
	page, size = pagex.Normalize(page, size)
	var items []Package
	var total int64
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListPackages(ctx, status, strings.TrimSpace(category), strings.TrimSpace(keyword), int32(size), int32((page-1)*size))
		return err
	}); err != nil {
		return nil, 0, page, size, lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		mapped, err := packageToMap(item)
		if err != nil {
			return nil, 0, page, size, err
		}
		out = append(out, mapped)
	}
	return out, total, page, size, nil
}

// ListPackageVersions 返回指定 code 的全部版本。
func (s *Service) ListPackageVersions(ctx context.Context, code string) ([]map[string]any, error) {
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
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item.Status != PackageStatusPublished {
			continue
		}
		mapped, err := packageToMap(item)
		if err != nil {
			return nil, err
		}
		out = append(out, mapped)
	}
	return out, nil
}

// SubmitPackage 上传仿真包、执行后端静态校验并创建审核记录。
func (s *Service) SubmitPackage(ctx context.Context, tenantID, accountID int64, req SubmitPackageRequest, input BundleInput) (map[string]any, error) {
	req, compute, err := normalizePackageRequest(req)
	if err != nil {
		return nil, err
	}
	if err := validatePackageRequest(req, compute, accountID); err != nil {
		return nil, err
	}
	backend, err := decodeObject(req.BackendConfig)
	if err != nil {
		return nil, err
	}
	if err := validateBackendAdapterConfig(compute, req.BackendAdapter, backend, s.backends); err != nil {
		return nil, err
	}
	if err := s.ensurePackageVersionAvailable(ctx, req.Code, req.Version); err != nil {
		return nil, err
	}
	packageID := s.ids.Generate()
	bundleRef, bundleHash, report, interactionSchema, codeTrace, err := s.storeBundle(ctx, tenantID, accountID, packageID, input, req, compute)
	if err != nil {
		return nil, err
	}
	scale, err := decodeObject(req.ScaleLimit)
	if err != nil {
		return nil, err
	}
	pkg := Package{ID: packageID, Code: req.Code, Version: req.Version, Name: req.Name, Category: req.Category, Compute: compute, ScaleLimit: scale, BundleKey: bundleRef, BundleHash: bundleHash, BackendAdapter: req.BackendAdapter, BackendConfig: backend, InteractionSchema: interactionSchema, CodeTrace: codeTrace, AuthorType: AuthorTeacher, AuthorID: accountID, Status: PackageStatusReviewing}
	var created Package
	var review Review
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.GetPackageByCodeVersion(ctx, pkg.Code, pkg.Version); err == nil {
			return apperr.ErrSimPackageVersionConflict
		} else if !isNoRows(err) {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		created, err = tx.CreatePackage(ctx, pkg)
		if err != nil {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		review, err = tx.CreateReview(ctx, s.ids.Generate(), created.ID, accountID, report)
		if err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := s.uploadPlannedBundle(ctx, bundleRef, input); err != nil {
		if rollbackErr := s.markUploadFailed(ctx, created.ID, review.ID); rollbackErr != nil {
			logging.ErrorContext(ctx, "sim package upload rollback failed", rollbackErr.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("package_id", created.ID), slog.Int64("review_id", review.ID))
			return nil, apperr.ErrSimBundleUnreadable.WithCause(errors.Join(err, rollbackErr))
		}
		return nil, err
	}
	if err := s.writeAuditFromContext(ctx, tenantID, "sim.package.submit", "sim_package", created.ID, map[string]any{"code": created.Code, "version": created.Version}); err != nil {
		return nil, err
	}
	out, err := packageToMap(created)
	if err != nil {
		return nil, err
	}
	reviewOut, err := reviewToMap(review)
	if err != nil {
		return nil, err
	}
	out["review"] = reviewOut
	return out, nil
}

// UpdatePackage 更新草稿或退回包的新 bundle,并重新进入审核中。
func (s *Service) UpdatePackage(ctx context.Context, tenantID, accountID, packageID int64, req SubmitPackageRequest, input BundleInput) (map[string]any, error) {
	req, compute, err := normalizePackageRequest(req)
	if err != nil {
		return nil, err
	}
	if err := validatePackageRequest(req, compute, accountID); err != nil {
		return nil, err
	}
	backend, err := decodeObject(req.BackendConfig)
	if err != nil {
		return nil, err
	}
	if err := validateBackendAdapterConfig(compute, req.BackendAdapter, backend, s.backends); err != nil {
		return nil, err
	}
	if err := s.ensurePackageEditable(ctx, accountID, packageID, req.Code, req.Version); err != nil {
		return nil, err
	}
	bundleRef, bundleHash, report, interactionSchema, codeTrace, err := s.storeBundle(ctx, tenantID, accountID, packageID, input, req, compute)
	if err != nil {
		return nil, err
	}
	scale, err := decodeObject(req.ScaleLimit)
	if err != nil {
		return nil, err
	}
	pkg := Package{ID: packageID, Name: req.Name, Category: req.Category, Compute: compute, ScaleLimit: scale, BundleKey: bundleRef, BundleHash: bundleHash, BackendAdapter: req.BackendAdapter, BackendConfig: backend, InteractionSchema: interactionSchema, CodeTrace: codeTrace, Status: PackageStatusReviewing}
	var updated Package
	var review Review
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		updated, err = tx.UpdatePackageDraft(ctx, pkg)
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageUnavailable, apperr.ErrSimPackageQueryFailed)
		}
		review, err = tx.CreateReview(ctx, s.ids.Generate(), updated.ID, accountID, report)
		if err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := s.uploadPlannedBundle(ctx, bundleRef, input); err != nil {
		if rollbackErr := s.markUploadFailed(ctx, updated.ID, review.ID); rollbackErr != nil {
			logging.ErrorContext(ctx, "sim package upload rollback failed", rollbackErr.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("package_id", updated.ID), slog.Int64("review_id", review.ID))
			return nil, apperr.ErrSimBundleUnreadable.WithCause(errors.Join(err, rollbackErr))
		}
		return nil, err
	}
	if err := s.writeAuditFromContext(ctx, tenantID, "sim.package.update", "sim_package", updated.ID, map[string]any{"code": updated.Code, "version": updated.Version}); err != nil {
		return nil, err
	}
	out, err := packageToMap(updated)
	if err != nil {
		return nil, err
	}
	reviewOut, err := reviewToMap(review)
	if err != nil {
		return nil, err
	}
	out["review"] = reviewOut
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
func (s *Service) PackagePreview(ctx context.Context, accountID, packageID int64) (map[string]any, error) {
	if accountID <= 0 {
		return nil, apperr.ErrForbidden
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
		return nil, err
	}
	pkgOut, err := packageToMap(pkg)
	if err != nil {
		return nil, err
	}
	reviewOut, err := reviewToMap(review)
	if err != nil {
		return nil, err
	}
	return map[string]any{"package": pkgOut, "review": reviewOut}, nil
}

// SubmitValidationReport 合并动态校验报告,不得覆盖后端生成的静态字段。
func (s *Service) SubmitValidationReport(ctx context.Context, packageID int64, req ValidationReportRequest, raw []byte) (map[string]any, error) {
	keys, err := rawReportMap(raw)
	if err != nil {
		return nil, apperr.ErrSimPackageValidationFailed.WithCause(err)
	}
	if err := validateDynamicReport(keys); err != nil {
		return nil, err
	}
	if err := validateValidationReportRequest(req); err != nil {
		return nil, err
	}
	report := reportFromRequest(req)
	report.Details = trimMapStrings(report.Details)
	var review Review
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		pkg, err := tx.GetPackageByID(ctx, packageID)
		if err != nil {
			return lookupError(err, apperr.ErrSimPackageNotFound, apperr.ErrSimPackageQueryFailed)
		}
		if pkg.Status != PackageStatusReviewing {
			return apperr.ErrSimPackageUnavailable
		}
		if err := ensureValidationReportTenant(ctx, pkg); err != nil {
			return err
		}
		review, err = tx.MergeValidationReport(ctx, packageID, report)
		if err != nil {
			return lookupError(err, apperr.ErrSimReviewNotFound, apperr.ErrSimReviewQueryFailed)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return reviewToMap(review)
}

// ensureValidationReportTenant 绑定受控预览服务的租户签名和包资源归属。
func ensureValidationReportTenant(ctx context.Context, pkg Package) error {
	id, ok := tenant.FromContext(ctx)
	if !ok || !id.IsSystem || id.TenantID <= 0 {
		return apperr.ErrServiceUnauthorized
	}
	ref, err := storage.ParseObjectRef(pkg.BundleKey)
	if err != nil {
		return apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	ownerTenantID, err := tenantIDFromBundleKey(ref.Key)
	if err != nil {
		return err
	}
	if ownerTenantID != id.TenantID {
		return apperr.ErrServiceUnauthorized
	}
	return nil
}

// ListReviews 返回审核分页列表。
func (s *Service) ListReviews(ctx context.Context, result int16, page, size int) ([]map[string]any, int64, int, int, error) {
	page, size = pagex.Normalize(page, size)
	var items []ReviewInfo
	var total int64
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListReviews(ctx, result, int32(size), int32((page-1)*size))
		return err
	}); err != nil {
		return nil, 0, page, size, lookupError(err, apperr.ErrSimReviewNotFound, apperr.ErrSimReviewQueryFailed)
	}
	accountIDs := make([]int64, 0, len(items)*2)
	for _, item := range items {
		accountIDs = append(accountIDs, item.SubmitterID)
		if item.ReviewerID > 0 {
			accountIDs = append(accountIDs, item.ReviewerID)
		}
	}
	accounts, err := s.identity.BatchGetAccounts(ctx, accountIDs)
	if err != nil {
		return nil, 0, page, size, apperr.ErrSimReviewQueryFailed.WithCause(err)
	}
	accountNames := make(map[int64]string, len(accounts))
	for _, account := range accounts {
		accountNames[account.AccountID] = account.Name
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		mapped, err := reviewInfoToMap(item)
		if err != nil {
			return nil, 0, page, size, err
		}
		name, ok := accountNames[item.SubmitterID]
		if !ok {
			return nil, 0, page, size, apperr.ErrSimReviewDataCorrupt.WithCause(fmt.Errorf("仿真审核提交账号不存在: account_id=%d", item.SubmitterID))
		}
		mapped["submitter_name"] = name
		if item.ReviewerID > 0 {
			reviewerName, ok := accountNames[item.ReviewerID]
			if !ok {
				return nil, 0, page, size, apperr.ErrSimReviewDataCorrupt.WithCause(fmt.Errorf("仿真审核处理账号不存在: account_id=%d", item.ReviewerID))
			}
			mapped["reviewer_name"] = reviewerName
		}
		out = append(out, mapped)
	}
	return out, total, page, size, nil
}

// ApproveReview 通过审核并上架包,要求四项校验全部通过。
func (s *Service) ApproveReview(ctx context.Context, reviewerID, reviewID int64) (map[string]any, error) {
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
		return nil, err
	}
	if err := s.writeAuditFromContext(ctx, 0, "sim.package.approve", "sim_package", pkg.ID, map[string]any{"code": pkg.Code, "version": pkg.Version}); err != nil {
		return nil, err
	}
	pkgOut, err := packageToMap(pkg)
	if err != nil {
		return nil, err
	}
	reviewOut, err := reviewToMap(review)
	if err != nil {
		return nil, err
	}
	return map[string]any{"package": pkgOut, "review": reviewOut}, nil
}

// RejectReview 退回审核并写入意见。
func (s *Service) RejectReview(ctx context.Context, reviewerID, reviewID int64, comment string) (map[string]any, error) {
	if strings.TrimSpace(comment) == "" || len(comment) > 500 {
		return nil, apperr.ErrSimReviewStateInvalid
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
		return nil, err
	}
	if err := s.writeAuditFromContext(ctx, 0, "sim.package.reject", "sim_package", pkg.ID, map[string]any{"code": pkg.Code, "version": pkg.Version}); err != nil {
		return nil, err
	}
	pkgOut, err := packageToMap(pkg)
	if err != nil {
		return nil, err
	}
	reviewOut, err := reviewToMap(review)
	if err != nil {
		return nil, err
	}
	return map[string]any{"package": pkgOut, "review": reviewOut}, nil
}

// ArchivePackage 下架已发布包,不影响历史回放。
func (s *Service) ArchivePackage(ctx context.Context, packageID int64) (map[string]any, error) {
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
		return nil, err
	}
	if err := s.writeAuditFromContext(ctx, 0, "sim.package.archive", "sim_package", pkg.ID, map[string]any{"code": pkg.Code, "version": pkg.Version}); err != nil {
		return nil, err
	}
	return packageToMap(pkg)
}

// RepublishPackage 仅允许已下架包重新上架,不得绕过审核发布草稿或退回包。
func (s *Service) RepublishPackage(ctx context.Context, packageID int64) (map[string]any, error) {
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
		return nil, err
	}
	if err := s.writeAuditFromContext(ctx, 0, "sim.package.republish", "sim_package", pkg.ID, map[string]any{"code": pkg.Code, "version": pkg.Version}); err != nil {
		return nil, err
	}
	return packageToMap(pkg)
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
