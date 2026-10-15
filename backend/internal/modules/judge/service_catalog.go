// judge service_catalog 文件负责运行时状态变化后的平台判题器目录重编译与闭环。
package judge

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
)

// ReconcileRuntimeDependentJudgers 让运行时自检结果驱动所有依赖判题器的快照、预拉取和自检状态。
func (s *Service) ReconcileRuntimeDependentJudgers(ctx context.Context, event contracts.SandboxRuntimeSelftestCompletedEvent) error {
	if strings.TrimSpace(event.RuntimeCode) == "" {
		return fmt.Errorf("运行时自检事件缺少 runtime_code")
	}
	var judgers []Judger
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		judgers, err = tx.ListJudgers(ctx)
		return err
	}); err != nil {
		return err
	}

	var firstErr error
	for _, judger := range judgers {
		if !judgerNeedsSandbox(judger.Type, judger.RuntimeRequired) {
			continue
		}
		composition, ok := judgerComposition(judger)
		if !ok {
			judger.ResourceSpec.DisabledReason = "判题器缺少正式 CompositionSpec,无法重建执行快照"
			if err := s.persistReconciledJudger(ctx, judger, JudgerSelftestPending, JudgerStatusDisabled); err != nil && firstErr == nil {
				firstErr = err
			}
			continue
		}
		if !compositionReferencesRuntime(composition, event.RuntimeCode) {
			continue
		}
		if event.RuntimeStatus != contracts.SandboxRuntimeStatusAvailable || event.SelftestStatus != contracts.SandboxRuntimeSelftestPassed {
			judger.ResourceSpec.DisabledReason = fmt.Sprintf("运行时 %s 尚未完成平台自检", event.RuntimeCode)
			if err := s.persistReconciledJudger(ctx, judger, JudgerSelftestPending, JudgerStatusDisabled); err != nil && firstErr == nil {
				firstErr = err
			}
			continue
		}

		compiled, err := s.sandbox.CompilePlatformSandboxComposition(ctx, composition)
		if err != nil {
			judger.ResourceSpec.DisabledReason = fmt.Sprintf("运行时 %s 的判题组合尚未完成编译", event.RuntimeCode)
			if persistErr := s.persistReconciledJudger(ctx, judger, JudgerSelftestPending, JudgerStatusDisabled); persistErr != nil && firstErr == nil {
				firstErr = persistErr
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		baseDigest := compiled.Digest
		compiled.PrivateSidecars = privateSidecarsForSandbox(judger.ResourceSpec.ExecutionSidecars)
		if err := contracts.AppendPrivateSidecarImageClosure(&compiled); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		digest, err := contracts.CanonicalSnapshotDigest(compiled)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		compiled.Digest = digest
		previousDigests := []string{baseDigest, judger.ResourceSpec.CompositionSnapshot.Digest}
		if err := s.sandbox.ReplacePlatformSandboxCompositionSnapshot(ctx, previousDigests, compiled); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := s.sandbox.PrepullPlatformComposition(ctx, compiled.Digest); err != nil {
			judger.ResourceSpec.CompositionSnapshot = compiled
			judger.ResourceSpec.DisabledReason = "判题组合预拉取未完成"
			if persistErr := s.persistReconciledJudger(ctx, judger, JudgerSelftestPending, JudgerStatusDisabled); persistErr != nil && firstErr == nil {
				firstErr = persistErr
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		judger.ResourceSpec.Composition = composition
		judger.ResourceSpec.CompositionSnapshot = compiled
		judger.ResourceSpec.DisabledReason = ""
		selftestStatus, judgerStatus := JudgerSelftestPassed, JudgerStatusAvailable
		if err := s.executeJudgerSelftest(ctx, judger); err != nil {
			selftestStatus, judgerStatus = JudgerSelftestFailed, JudgerStatusDisabled
			judger.ResourceSpec.DisabledReason = "判题器平台自检未通过"
			if firstErr == nil {
				firstErr = err
			}
		}
		if err := s.persistReconciledJudger(ctx, judger, selftestStatus, judgerStatus); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return firstErr
	}
	return s.writeSystemAudit(ctx, 0, "judge.catalog.reconcile", "runtime", event.RuntimeID, map[string]any{
		"runtime_code":    event.RuntimeCode,
		"selftest_status": event.SelftestStatus,
		"runtime_status":  event.RuntimeStatus,
	})
}

// persistReconciledJudger 以同一份资源执行事实更新判题器状态,不重新解析平台 manifest。
func (s *Service) persistReconciledJudger(ctx context.Context, item Judger, selftestStatus, status int16) error {
	return s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.UpsertJudger(ctx, item.ID, JudgerRequest{
			Code:              item.Code,
			Name:              item.Name,
			Type:              item.Type,
			ExecutorRef:       item.ExecutorRef,
			RuntimeRequired:   item.RuntimeRequired,
			DefaultTimeoutSec: item.DefaultTimeoutSec,
			Status:            status,
		}, item.ResourceSpec, selftestStatus)
		return err
	})
}

// judgerComposition 读取唯一可重编译的正式组合声明;缺失时严格拒绝反推执行事实。
func judgerComposition(item Judger) (contracts.SandboxCompositionSpec, bool) {
	composition := item.ResourceSpec.Composition
	if strings.TrimSpace(composition.ID) == "" || len(composition.Runtimes) == 0 || !composition.AccessProfile.Valid() {
		return contracts.SandboxCompositionSpec{}, false
	}
	return composition, true
}

// compositionReferencesRuntime 判断判题器组合是否依赖指定运行时。
func compositionReferencesRuntime(spec contracts.SandboxCompositionSpec, runtimeCode string) bool {
	for _, runtime := range spec.Runtimes {
		if strings.EqualFold(strings.TrimSpace(runtime.Code), strings.TrimSpace(runtimeCode)) {
			return true
		}
	}
	return false
}
