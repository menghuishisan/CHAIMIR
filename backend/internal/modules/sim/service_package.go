// sim service_package 文件实现仿真包注册、bundle 读取、动态校验和审核状态机。
package sim

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"chaimir/internal/platform/storage"
	"chaimir/pkg/apperr"
)

// ListPackages 返回仿真包分页列表。
func (s *Service) ListPackages(ctx context.Context, status int16, category, keyword string, page, size int) ([]map[string]any, int64, int, int, error) {
	if page < 1 || size < 1 || size > 100 {
		return nil, 0, page, size, apperr.ErrQueryParamInvalid
	}
	var items []Package
	var total int64
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListPackages(ctx, status, strings.TrimSpace(category), strings.TrimSpace(keyword), int32(size), int32((page-1)*size))
		return err
	}); err != nil {
		return nil, 0, page, size, apperr.ErrSimPackageNotFound.WithCause(err)
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, packageToMap(item))
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
		return nil, apperr.ErrSimPackageNotFound.WithCause(err)
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item.Status != PackageStatusPublished {
			continue
		}
		out = append(out, packageToMap(item))
	}
	return out, nil
}

// ReadPublishedBundle 读取已上架包 bundle,由 API 边界按鉴权结果流式返回。
func (s *Service) ReadPublishedBundle(ctx context.Context, code, version string) (io.ReadCloser, string, error) {
	pkg, err := s.loadPackage(ctx, code, version)
	if err != nil {
		return nil, "", err
	}
	if pkg.Status != PackageStatusPublished {
		return nil, "", apperr.ErrSimPackageUnavailable
	}
	ref, err := storage.ParseObjectRef(pkg.BundleKey)
	if err != nil {
		return nil, "", apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	rc, err := s.storage.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return nil, "", apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	return rc, pkg.BundleHash, nil
}

// SubmitPackage 上传仿真包、执行后端静态校验并创建审核记录。
func (s *Service) SubmitPackage(ctx context.Context, tenantID, accountID int64, req SubmitPackageRequest, input BundleInput) (map[string]any, error) {
	req, compute, err := normalizePackageRequest(req, AuthorTeacher)
	if err != nil {
		return nil, err
	}
	if err := validatePackageRequest(req, compute, accountID); err != nil {
		return nil, err
	}
	packageID := s.ids.Generate()
	bundleRef, bundleHash, report, err := s.storeBundle(ctx, tenantID, accountID, packageID, input)
	if err != nil {
		return nil, err
	}
	scale, err := decodeObject(req.ScaleLimit)
	if err != nil {
		return nil, err
	}
	backend, err := decodeObject(req.BackendConfig)
	if err != nil {
		return nil, err
	}
	pkg := Package{ID: packageID, Code: req.Code, Version: req.Version, Name: req.Name, Category: req.Category, Compute: compute, ScaleLimit: scale, BundleKey: bundleRef, BundleHash: bundleHash, BackendAdapter: req.BackendAdapter, BackendConfig: backend, AuthorType: req.AuthorType, AuthorID: accountID, Status: PackageStatusReviewing}
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
		review, err = tx.CreateReview(ctx, s.ids.Generate(), created.ID, accountID, report)
		if err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := s.writeAudit(ctx, tenantID, accountID, 3, "sim.package.submit", "sim_package", created.ID, map[string]any{"code": created.Code, "version": created.Version}); err != nil {
		return nil, err
	}
	out := packageToMap(created)
	out["review"] = reviewToMap(review)
	return out, nil
}

// UpdatePackage 更新草稿或退回包的新 bundle,并重新进入审核中。
func (s *Service) UpdatePackage(ctx context.Context, tenantID, accountID, packageID int64, req UpdatePackageRequest, input BundleInput) (map[string]any, error) {
	req, compute, err := normalizePackageRequest(req, AuthorTeacher)
	if err != nil {
		return nil, err
	}
	if err := validatePackageRequest(req, compute, accountID); err != nil {
		return nil, err
	}
	bundleRef, bundleHash, report, err := s.storeBundle(ctx, tenantID, accountID, packageID, input)
	if err != nil {
		return nil, err
	}
	scale, err := decodeObject(req.ScaleLimit)
	if err != nil {
		return nil, err
	}
	backend, err := decodeObject(req.BackendConfig)
	if err != nil {
		return nil, err
	}
	pkg := Package{ID: packageID, Name: req.Name, Category: req.Category, Compute: compute, ScaleLimit: scale, BundleKey: bundleRef, BundleHash: bundleHash, BackendAdapter: req.BackendAdapter, BackendConfig: backend, Status: PackageStatusReviewing}
	var updated Package
	var review Review
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		existing, err := tx.GetPackageByID(ctx, packageID)
		if err != nil {
			return apperr.ErrSimPackageNotFound.WithCause(err)
		}
		if existing.AuthorID != accountID || existing.Code != req.Code || existing.Version != req.Version {
			return apperr.ErrForbidden
		}
		updated, err = tx.UpdatePackageDraft(ctx, pkg)
		if err != nil {
			return apperr.ErrSimPackageUnavailable.WithCause(err)
		}
		review, err = tx.CreateReview(ctx, s.ids.Generate(), updated.ID, accountID, report)
		if err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := s.writeAudit(ctx, tenantID, accountID, 3, "sim.package.update", "sim_package", updated.ID, map[string]any{"code": updated.Code, "version": updated.Version}); err != nil {
		return nil, err
	}
	out := packageToMap(updated)
	out["review"] = reviewToMap(review)
	return out, nil
}

// PackagePreview 返回最新审核报告和包摘要,实际隔离预览由前端 Worker/受控流程执行。
func (s *Service) PackagePreview(ctx context.Context, packageID int64) (map[string]any, error) {
	var pkg Package
	var review Review
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		pkg, err = tx.GetPackageByID(ctx, packageID)
		if err != nil {
			return apperr.ErrSimPackageNotFound.WithCause(err)
		}
		review, err = tx.GetLatestReviewForPackage(ctx, packageID)
		if err != nil {
			return apperr.ErrSimReviewNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{"package": packageToMap(pkg), "review": reviewToMap(review)}, nil
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
	report := reportFromRequest(req)
	report.Details = trimMapStrings(report.Details)
	var review Review
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		review, err = tx.MergeValidationReport(ctx, packageID, report)
		if err != nil {
			return apperr.ErrSimReviewNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return reviewToMap(review), nil
}

// ListReviews 返回审核分页列表。
func (s *Service) ListReviews(ctx context.Context, result int16, page, size int) ([]map[string]any, int64, int, int, error) {
	if page < 1 || size < 1 || size > 100 {
		return nil, 0, page, size, apperr.ErrQueryParamInvalid
	}
	var items []ReviewInfo
	var total int64
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListReviews(ctx, result, int32(size), int32((page-1)*size))
		return err
	}); err != nil {
		return nil, 0, page, size, apperr.ErrSimReviewNotFound.WithCause(err)
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, reviewInfoToMap(item))
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
			return apperr.ErrSimReviewNotFound.WithCause(err)
		}
		if err := validateApprovalReport(review.PreviewReport); err != nil {
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
	return map[string]any{"package": packageToMap(pkg), "review": reviewToMap(review)}, nil
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
	return map[string]any{"package": packageToMap(pkg), "review": reviewToMap(review)}, nil
}

// ArchivePackage 下架已发布包,不影响历史回放。
func (s *Service) ArchivePackage(ctx context.Context, packageID int64) (map[string]any, error) {
	var pkg Package
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		existing, err := tx.GetPackageByID(ctx, packageID)
		if err != nil {
			return apperr.ErrSimPackageNotFound.WithCause(err)
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
	return packageToMap(pkg), nil
}

// RepublishPackage 仅允许已下架包重新上架,不得绕过审核发布草稿或退回包。
func (s *Service) RepublishPackage(ctx context.Context, packageID int64) (map[string]any, error) {
	var pkg Package
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		existing, err := tx.GetPackageByID(ctx, packageID)
		if err != nil {
			return apperr.ErrSimPackageNotFound.WithCause(err)
		}
		if existing.Status != PackageStatusArchived {
			return apperr.ErrSimPackageUnavailable
		}
		pkg, err = tx.UpdatePackageStatus(ctx, packageID, PackageStatusPublished)
		if err != nil {
			return apperr.ErrSimPackageInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return packageToMap(pkg), nil
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
			return apperr.ErrSimPackageNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return Package{}, err
	}
	return pkg, nil
}

// decodeObject 在进入数据库前把已通过 rules 校验的 JSON 对象转换为 map。
func decodeObject(raw json.RawMessage) (map[string]any, error) {
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, apperr.ErrSimPackageInvalid.WithCause(err)
	}
	return out, nil
}
