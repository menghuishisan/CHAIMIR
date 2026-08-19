// sim service_preview 文件实现上架前的隔离预览任务:审核四项门禁中两项动态校验的唯一生产者。
//
// 为什么必须有这个任务:approve 要求 metadata_validation、static_scan、determinism_check、
// worker_preview 四项全为 passed。前两项由上传流程写入,后两项只能由隔离容器执行后回写 ——
// 没有生产者的话教师提交的包会永久停在待审,管理员点"通过并上架"必然被 41005 拒,
// 而界面提示的"退回给作者修改"对作者是无从下手的:缺的两项不是作者能提供的东西。
// 审核链路必须闭合,不允许只有门禁没有执行者(见 docs/04-仿真可视化引擎/06-业务流程与状态机.md §4)。
//
// 预览与生产运行共用同一套隔离设施:"跑起来看效果对不对"本身就需要一个能安全执行第三方代码
// 的地方,那个地方就是容器,不为审核另建一套。
package sim

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/logging"
	"chaimir/pkg/textx"
)

// RunPackagePreviewOnce 执行一次隔离预览扫描,供统一 background runner 调用。
//
// 单个包失败不阻断整批:一个包的缺陷不该让同批其他包一起卡住,失败结论会写回报告
// 让作者看到原因。只有认领本身失败才向上返回,由 runner 重试。
func (s *Service) RunPackagePreviewOnce(ctx context.Context) error {
	batchSize, ok := intx.Int32(s.previewBatchSize)
	if !ok || batchSize <= 0 {
		return apperr.ErrSimPackageQueryFailed
	}
	var pending []Package
	leaseToken, err := pkgcrypto.RandomToken(48)
	if err != nil {
		return apperr.ErrSimPackageQueryFailed.WithCause(err)
	}
	leaseUntil := time.Now().Add(time.Duration(s.previewLeaseDurationMs) * time.Millisecond)
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
		return err
	}
	for _, pkg := range pending {
		if err := s.previewPackageOnce(ctx, pkg); err != nil {
			logging.ErrorContext(ctx, "sim package preview failed", err.Error(), slog.Int64("package_id", pkg.ID), slog.String("code", pkg.Code), slog.String("version", pkg.Version))
		}
	}
	return nil
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
	bundle, err := s.loadBundleForPreview(ctx, pkg)
	if err != nil {
		return s.handlePreviewInfrastructureFailure(ctx, pkg, err)
	}
	result, previewErr := adapter.Preview(ctx, pkg, bundle, s.previewFrameCount)
	if previewErr != nil {
		return s.handlePreviewInfrastructureFailure(ctx, pkg, previewErr)
	}
	report, err := previewReport(result, previewErr)
	if err != nil {
		return err
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
	return s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		if int(pkg.PreviewAttemptCount) >= s.previewMaxAttempts {
			if _, err := tx.MergeValidationReport(ctx, pkg.PreviewReviewID, pkg.ID, pkg.PreviewLeaseToken, previewAttemptsExhaustedReport()); err != nil {
				return apperr.ErrSimReviewStateInvalid.WithCause(err)
			}
			return previewErr
		}
		affected, err := tx.ReleasePreviewLease(ctx, pkg.PreviewReviewID, pkg.ID, pkg.PreviewLeaseToken)
		if err != nil {
			return apperr.ErrSimReviewStateInvalid.WithCause(err)
		}
		if affected != 1 {
			return apperr.ErrSimReviewStateInvalid
		}
		return previewErr
	})
}

// previewReport 把预览产出转成审核报告的动态字段。
//
// 样例帧序列化失败必须显式失败而不是静默丢帧:帧是审核人判断"算法实现对不对"的唯一依据,
// 少了它审核只剩两个徽章,而报告本身看不出帧曾经存在过。
func previewReport(result PreviewResult, previewErr error) (ValidationReport, error) {
	if previewErr != nil {
		return ValidationReport{}, previewErr
	}
	report := ValidationReport{
		WorkerPreview: ValidationStatus{Status: validationPassed},
	}
	if result.DeterminismPassed {
		report.DeterminismCheck = ValidationStatus{Status: validationPassed}
	} else {
		report.DeterminismCheck = ValidationStatus{Status: validationFailed, Message: boundedValidationMessage(result.Detail)}
	}
	if len(result.Frames) > 0 {
		raw, err := jsonx.AnyBytes(result.Frames, apperr.ErrSimPackageValidationFailed)
		if err != nil {
			return ValidationReport{}, err
		}
		report.PreviewFrames = raw
	}
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
func (s *Service) loadBundleForPreview(ctx context.Context, pkg Package) (*ExecutionBundle, error) {
	return s.loadBundleForExecution(ctx, SessionWithPackage{
		Session:    Session{ID: pkg.ID},
		BundleKey:  pkg.BundleKey,
		BundleHash: pkg.BundleHash,
		Entry:      pkg.Entry,
	})
}
