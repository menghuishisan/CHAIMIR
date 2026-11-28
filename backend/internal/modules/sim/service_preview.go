// 仿真包的确定性校验与样例帧生产；预览和正式运行共用隔离设施。
package sim

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/platform/background"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/textx"
)

// RunPackagePreviewOnce 执行一次隔离预览扫描,供统一 background runner 调用。
//
// 单包运行失败不阻断其他包；整轮返回执行与回写错误，算法不合格正常写入审核报告。
func (s *Service) RunPackagePreviewOnce(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	batchSize, ok := intx.Int32(s.previewBatchSize)
	if !ok || batchSize <= 0 {
		return apperr.ErrSimPackageQueryFailed
	}
	leaseDuration := time.Duration(s.previewLeaseDurationMs) * time.Millisecond
	if leaseDuration < 3*time.Second {
		return apperr.ErrSimPackageQueryFailed
	}
	var pending []Package
	leaseToken, err := pkgcrypto.RandomToken(48)
	if err != nil {
		return apperr.ErrSimPackageQueryFailed.WithCause(err)
	}
	leaseUntil := time.Now().Add(leaseDuration)
	maxAttempts, ok := intx.Int32(s.previewMaxAttempts)
	if !ok || maxAttempts <= 0 {
		return apperr.ErrSimPackageQueryFailed
	}
	staleBefore := time.Now()
	exhaustedReport := previewAttemptsExhaustedReport()
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.ExhaustPreviewAttempts(ctx, maxAttempts, staleBefore, exhaustedReport); err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		items, err := tx.ClaimPackagesForPreview(ctx, batchSize, maxAttempts, staleBefore, leaseUntil, leaseToken)
		if err != nil {
			return apperr.ErrSimPackageQueryFailed.WithCause(err)
		}
		pending = items
		return nil
	}); err != nil {
		return errors.Join(err, ctx.Err())
	}
	var resultErr error
	for _, pkg := range pending {
		if err := ctx.Err(); err != nil {
			return errors.Join(resultErr, err)
		}
		if err := s.previewPackageWithLease(ctx, pkg, leaseDuration); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("执行仿真包预览失败 package_id=%d code=%s version=%s: %w", pkg.ID, pkg.Code, pkg.Version, err))
		}
	}
	return errors.Join(resultErr, ctx.Err())
}

// previewPackageWithLease 确认租约后执行预览，返回前等待续租退出。
func (s *Service) previewPackageWithLease(ctx context.Context, pkg Package, leaseDuration time.Duration) error {
	renewed, err := s.renewPreviewLease(ctx, pkg, leaseDuration)
	if err != nil {
		return apperr.ErrSimReviewStateInvalid.WithCause(err)
	}
	if !renewed {
		return nil
	}
	_, err = background.WithRenewal(ctx, leaseDuration, func(ctx context.Context) (bool, error) {
		return s.renewPreviewLease(ctx, pkg, leaseDuration)
	}, func(ctx context.Context) error {
		return s.previewPackageOnce(ctx, pkg)
	})
	return err
}

// renewPreviewLease 仅允许当前令牌延长未过期的预览租约。
func (s *Service) renewPreviewLease(ctx context.Context, pkg Package, leaseDuration time.Duration) (bool, error) {
	var renewed bool
	err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		renewed, err = tx.RenewPreviewLease(ctx, pkg.PreviewReviewID, pkg.ID, pkg.PreviewLeaseToken, time.Now().Add(leaseDuration))
		return err
	})
	return renewed, err
}

// previewPackageOnce 对单个待审包执行隔离预览并回写报告。
//
// 预览本身出错(取不到归档、Pod 起不来、容器崩溃)与"包不合格"是两件事:
// 前者不写结论,交由下次轮询重试;后者写 failed 结论并附原因,让作者能据此修包。
func (s *Service) previewPackageOnce(ctx context.Context, pkg Package) error {
	adapter := s.backends[pkg.BackendAdapter]
	if adapter == nil {
		return s.handlePreviewInfrastructureFailure(ctx, pkg, apperr.ErrSimBackendComputeUnavailable.WithCause(fmt.Errorf("仿真包 %s@%s 绑定的执行能力 %q 未注册", pkg.Code, pkg.Version, pkg.BackendAdapter)))
	}
	runID := s.ids.Generate()
	bundle, err := s.loadBundleForPreview(ctx, pkg, runID)
	if err != nil {
		return s.handlePreviewInfrastructureFailure(ctx, pkg, err)
	}
	result, previewErr := adapter.Preview(ctx, pkg, runID, bundle, s.previewFrameCount)
	if previewErr != nil {
		return s.handlePreviewInfrastructureFailure(ctx, pkg, previewErr)
	}
	report, err := previewReport(result, pkg.ScaleLimit)
	if err != nil {
		return s.handlePreviewInfrastructureFailure(ctx, pkg, err)
	}
	return s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.MergeValidationReport(ctx, pkg.PreviewReviewID, pkg.ID, pkg.PreviewLeaseToken, report); err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		return nil
	})
}

// handlePreviewInfrastructureFailure 保留动态预览可重试语义;只有已用尽预算时才写受控的终态报告。
func (s *Service) handlePreviewInfrastructureFailure(ctx context.Context, pkg Package, previewErr error) error {
	// 恢复状态必须提交成功;执行错误在事务外返回,防止释放租约或耗尽结论随回调错误回滚。
	err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		if int(pkg.PreviewAttemptCount) >= s.previewMaxAttempts {
			if _, err := tx.MergeValidationReport(ctx, pkg.PreviewReviewID, pkg.ID, pkg.PreviewLeaseToken, previewAttemptsExhaustedReport()); err != nil {
				return apperr.ErrSimReviewStateInvalid.WithCause(err)
			}
			return nil
		}
		affected, err := tx.ReleasePreviewLease(ctx, pkg.PreviewReviewID, pkg.ID, pkg.PreviewLeaseToken)
		if err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		if affected != 1 {
			return apperr.ErrSimReviewStateInvalid
		}
		return nil
	})
	return errors.Join(previewErr, err)
}

// previewReport 把预览产出转成审核报告的动态字段。
//
// 样例帧序列化失败必须显式失败而不是静默丢帧:帧是审核人判断"算法实现对不对"的唯一依据,
// 少了它审核只剩两个徽章,而报告本身看不出帧曾经存在过。
func previewReport(result PreviewResult, scaleLimit map[string]any) (ValidationReport, error) {
	if err := validatePreviewFrames(result.Frames, scaleLimit); err != nil {
		return ValidationReport{}, err
	}
	report := ValidationReport{
		WorkerPreview: ValidationStatus{Status: validationPassed},
	}
	if result.DeterminismPassed {
		report.DeterminismCheck = ValidationStatus{Status: validationPassed}
	} else {
		report.DeterminismCheck = ValidationStatus{Status: validationFailed, Message: boundedValidationMessage(result.Detail)}
	}
	raw, err := jsonx.AnyBytes(result.Frames, apperr.ErrSimPackageValidationFailed)
	if err != nil {
		return ValidationReport{}, err
	}
	report.PreviewFrames = raw
	return report, nil
}

// previewAttemptsExhaustedReport 给无法完成预览的包留下可行动、无基础设施细节的审核结论。
func previewAttemptsExhaustedReport() ValidationReport {
	message := "隔离预览多次未完成，请联系平台管理员检查预览环境后重新提交。"
	return ValidationReport{
		DeterminismCheck: ValidationStatus{Status: validationFailed, Message: message},
		WorkerPreview:    ValidationStatus{Status: validationFailed, Message: message},
	}
}

// boundedValidationMessage 把容器给出的原因截断到报告字段的展示上限。
// 文本来自隔离容器输出,长度不可信;截断而不是拒写 —— 作者需要看到原因,哪怕只有开头。
func boundedValidationMessage(message string) string {
	return textx.TruncateRunes(strings.TrimSpace(message), maxValidationMessageLength)
}

// loadBundleForPreview 取出待审包的归档正文,交隔离容器装配。
func (s *Service) loadBundleForPreview(ctx context.Context, pkg Package, runID int64) (*ExecutionBundle, error) {
	return s.loadBundleForExecution(ctx, SessionWithPackage{
		Session:        Session{ID: runID},
		BundleKey:      pkg.BundleKey,
		BundleHash:     pkg.BundleHash,
		Entry:          pkg.Entry,
		BackendAdapter: pkg.BackendAdapter,
	})
}
