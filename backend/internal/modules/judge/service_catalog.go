// judge service_catalog 统一恢复运行时依赖的判题目录,只维护唯一冻结快照。
package judge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// ReconcileRuntimeDependentJudgers 将事件作为协调触发器,以 M2 当前状态为准而不回放过期事件状态。
func (s *Service) ReconcileRuntimeDependentJudgers(ctx context.Context, event contracts.SandboxRuntimeSelftestCompletedEvent) error {
	if strings.TrimSpace(event.RuntimeCode) == "" {
		return fmt.Errorf("运行时自检事件缺少 runtime_code")
	}
	return s.reconcileJudgerCatalog(ctx, event.RuntimeCode)
}

// ReconcileJudgerCatalogOnce 在启动和周期扫描时恢复错过事件的目录记录。
func (s *Service) ReconcileJudgerCatalogOnce(ctx context.Context) error {
	return s.reconcileJudgerCatalog(ctx, "")
}

// reconcileJudgerCatalog 复用同一编译、自检和比较写回流程,单项失败不阻塞其他判题器。
func (s *Service) reconcileJudgerCatalog(ctx context.Context, runtimeCode string) error {
	var judgers []Judger
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		judgers, err = tx.ListJudgers(ctx)
		return err
	}); err != nil {
		return err
	}
	var failures []error
	for _, item := range judgers {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		// 没有自动停用原因的停用项由管理员控制,后台不重新启用。
		if !judgerNeedsSandbox(item.Type, item.RuntimeRequired) ||
			(item.Status == JudgerStatusDisabled && item.ResourceSpec.DisabledReason == "") {
			continue
		}
		composition, ok := judgerComposition(item)
		if runtimeCode != "" && ok && !compositionReferencesRuntime(composition, runtimeCode) {
			continue
		}
		if err := s.reconcileJudger(ctx, item, composition, ok); err != nil {
			logging.ErrorContext(ctx, "judge catalog reconciliation failed", apperr.AsAppError(err).LogString(),
				slog.Int64("tenant_id", 0), slog.Int64("judger_id", item.ID), slog.String("judger_code", item.Code))
			failures = append(failures, fmt.Errorf("判题器 %s 目录恢复失败: %w", item.Code, err))
		}
	}
	return errors.Join(failures...)
}

// reconcileJudger 验证完整依赖后才开放执行,失败时保留声明供下一轮恢复。
func (s *Service) reconcileJudger(ctx context.Context, original Judger, composition contracts.SandboxCompositionSpec, valid bool) error {
	item := original
	var executionErr error
	selftestStatus := JudgerSelftestPending
	if !valid {
		executionErr = apperr.ErrJudgerConfigInvalid
		item.ResourceSpec.DisabledReason = "判题环境配置不完整"
	} else {
		var snapshot contracts.SandboxCompositionSnapshot
		snapshot, executionErr = s.compileJudgerSnapshot(ctx, composition, item.ResourceSpec)
		if executionErr != nil {
			item.ResourceSpec.DisabledReason = "判题环境尚未准备完成"
		} else {
			// 相同最终摘要的成功记录无需再次创建环境和运行自检。
			if original.Status == JudgerStatusAvailable && original.SelftestStatus == JudgerSelftestPassed &&
				original.ResourceSpec.DisabledReason == "" && snapshot.Digest == original.ResourceSpec.CompositionSnapshot.Digest {
				return nil
			}
			item.ResourceSpec.CompositionSnapshot = snapshot
			executionErr = s.sandbox.PrepullPlatformComposition(ctx, snapshot.Digest)
			if executionErr != nil {
				item.ResourceSpec.DisabledReason = "判题环境正在分发"
			} else {
				executionErr = s.executeJudgerSelftest(ctx, item)
				selftestStatus = JudgerSelftestPassed
				item.ResourceSpec.DisabledReason = ""
				if executionErr != nil {
					selftestStatus = JudgerSelftestFailed
					item.ResourceSpec.DisabledReason = "判题器平台自检未通过"
				}
			}
		}
	}
	status := JudgerStatusDisabled
	if executionErr == nil {
		status = JudgerStatusAvailable
	}
	if reflect.DeepEqual(original.ResourceSpec, item.ResourceSpec) && original.SelftestStatus == selftestStatus && original.Status == status {
		return executionErr
	}
	_, updated, persistErr := s.persistJudgerExecutionState(ctx, original, item.ResourceSpec, selftestStatus, status)
	if persistErr != nil {
		return errors.Join(executionErr, persistErr)
	}
	if !updated {
		// 配置已变化或另一协调者已完成;旧结果不得覆盖当前配置。
		return executionErr
	}
	auditErr := s.writeSystemAudit(ctx, 0, "judge.catalog.reconcile", "judger", item.ID,
		map[string]any{"selftest_status": selftestStatus, "status": status, "composition_digest": item.ResourceSpec.CompositionSnapshot.Digest})
	return errors.Join(executionErr, auditErr)
}

// compileJudgerSnapshot 统一编译、补齐私有镜像闭包并登记最终摘要。
func (s *Service) compileJudgerSnapshot(ctx context.Context, composition contracts.SandboxCompositionSpec, resource JudgerResourceSpec) (contracts.SandboxCompositionSnapshot, error) {
	snapshot, err := s.sandbox.CompilePlatformSandboxComposition(ctx, composition)
	if err != nil {
		return contracts.SandboxCompositionSnapshot{}, err
	}
	baseDigest := snapshot.Digest
	snapshot.PrivateSidecars = privateSidecarsForSandbox(resource.ExecutionSidecars)
	if err := contracts.AppendPrivateSidecarImageClosure(&snapshot); err != nil {
		return contracts.SandboxCompositionSnapshot{}, err
	}
	digest, err := contracts.CanonicalSnapshotDigest(snapshot)
	if err != nil {
		return contracts.SandboxCompositionSnapshot{}, err
	}
	snapshot.Digest = digest
	// 仅移除本次编译中间摘要,已冻结旧版本仍可被在途任务引用。
	if err := s.sandbox.ReplacePlatformSandboxCompositionSnapshot(ctx, []string{baseDigest}, snapshot); err != nil {
		return contracts.SandboxCompositionSnapshot{}, err
	}
	return snapshot, nil
}

// persistJudgerExecutionState 回写唯一配置版本的状态,未命中表示结果已过期。
func (s *Service) persistJudgerExecutionState(ctx context.Context, original Judger, resource JudgerResourceSpec, selftestStatus, status int16) (Judger, bool, error) {
	var out Judger
	err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.UpdateJudgerExecutionState(ctx, original, resource, selftestStatus, status)
		return err
	})
	if s.store.IsNoRows(err) {
		return Judger{}, false, nil
	}
	if err != nil {
		return Judger{}, false, apperr.ErrJudgerPersistFailed.WithCause(err)
	}
	return out, true, nil
}

// judgerComposition 只读取唯一快照内的正式声明,不接受第二份 composition。
func judgerComposition(item Judger) (contracts.SandboxCompositionSpec, bool) {
	composition := item.ResourceSpec.CompositionSnapshot.Spec
	if strings.TrimSpace(composition.ID) == "" || len(composition.Runtimes) == 0 || composition.AccessProfile != contracts.SandboxAccessJudgePrivate {
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
