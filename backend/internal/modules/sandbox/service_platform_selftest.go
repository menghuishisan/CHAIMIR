// service_platform_selftest 文件实现平台级临时沙箱自检,不创建租户业务沙箱记录。
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// RunPlatformSelftest 在已编译的平台组合中执行真实自检命令,结束后无论成功失败都回收临时资源。
func (s *Service) RunPlatformSelftest(ctx context.Context, req contracts.SandboxPlatformSelftestRequest) error {
	if err := validatePlatformSelftestRequest(req); err != nil {
		return err
	}
	digest, err := contracts.CanonicalSnapshotDigest(req.CompositionSnapshot)
	if err != nil || digest != req.CompositionSnapshot.Digest {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	runtimePlans, err := s.runtimePlansForSnapshot(ctx, req.CompositionSnapshot)
	if err != nil {
		return err
	}
	execTarget, err := materializePlatformSelftestExecTarget(req.ExecTarget, req.CompositionSnapshot.Spec.WorkspaceRuntimeInstance)
	if err != nil {
		return err
	}
	privateSidecars := privateSidecarsFromContract(req.CompositionSnapshot.PrivateSidecars)
	if err := validatePrivateSidecars(privateSidecars, s.cfg, adapterSpecForPrivateSidecarValidation(runtimePlans)); err != nil {
		return err
	}
	for _, runtimePlan := range runtimePlans {
		if err := validatePlanImagesCurrentlyAdmitted(s.cfg, runtimePlan.Runtime, runtimePlan.Image, nil, privateSidecars); err != nil {
			return err
		}
	}

	selftestID := s.ids.Generate()
	sb := Sandbox{
		ID:             selftestID,
		TenantID:       0,
		Namespace:      namespaceFor("sbx-selftest", selftestID),
		Phase:          SandboxPhaseAllocating,
		Status:         SandboxStatusCreating,
		OwnerAccountID: 0,
	}
	plan := CreateSandboxPlan{
		Sandbox:                  sb,
		WorkspaceRuntimeInstance: req.CompositionSnapshot.Spec.WorkspaceRuntimeInstance,
		Runtimes:                 runtimePlans,
		PrivateSidecars:          privateSidecars,
	}

	ctx = logging.WithAttrs(ctx, slog.Int64("sandbox_id", sb.ID), slog.String("namespace", sb.Namespace),
		slog.String("composition_digest", digest), slog.String("exec_target", execTarget))
	operationCtx, operationCancel := context.WithTimeout(ctx, time.Duration(req.TimeoutSec)*time.Second)
	defer operationCancel()
	started := time.Now()
	runErr := s.orchestrator.CreateSandboxResources(operationCtx, plan)
	if runErr == nil {
		runErr = operationCtx.Err()
	}
	logPlatformSelftestStage(operationCtx, "prepare", started, runErr)
	if runErr != nil {
		runErr = fmt.Errorf("平台自检资源准备失败: %w", runErr)
	} else {
		for index, command := range req.Commands {
			if err := operationCtx.Err(); err != nil {
				runErr = fmt.Errorf("平台自检第%d条命令开始前预算已结束: %w", index+1, err)
				logPlatformSelftestStage(operationCtx, "execute", time.Now(), runErr, slog.Int("command_index", index+1))
				break
			}
			started = time.Now()
			_, stderr, commandErr := s.orchestrator.Exec(operationCtx, sb.Namespace, execTarget, command, nil, false)
			if commandErr == nil {
				commandErr = operationCtx.Err()
			}
			logPlatformSelftestStage(operationCtx, "execute", started, commandErr,
				slog.Int("command_index", index+1), slog.String("executable", command[0]))
			if commandErr != nil {
				runErr = fmt.Errorf("平台自检命令 %q 执行失败: %w: %s", command[0], commandErr, strings.TrimSpace(string(stderr)))
				break
			}
		}
	}
	operationCancel()

	cleanupBase := logging.WithAttrs(context.WithoutCancel(ctx), logging.AttrsFromContext(ctx)...)
	cleanupCtx, cleanupCancel := context.WithTimeout(cleanupBase, time.Duration(s.cfg.SelftestRecycleTimeoutSeconds)*time.Second)
	defer cleanupCancel()
	started = time.Now()
	cleanupErr := s.orchestrator.DestroySandboxResources(cleanupCtx, sb)
	logPlatformSelftestStage(cleanupCtx, "cleanup", started, cleanupErr)
	if cleanupErr != nil {
		runErr = errors.Join(runErr, fmt.Errorf("回收平台自检资源失败: %w", cleanupErr))
	}
	if runErr != nil {
		return apperr.ErrSandboxSelftestFailed.WithCause(runErr)
	}
	return nil
}

// logPlatformSelftestStage 记录阶段和剩余预算，便于区分准备耗时、命令超时与回收失败。
func logPlatformSelftestStage(ctx context.Context, stage string, started time.Time, err error, attrs ...slog.Attr) {
	attrs = append(attrs, slog.String("stage", stage), slog.Duration("duration", time.Since(started)))
	if deadline, ok := ctx.Deadline(); ok {
		attrs = append(attrs, slog.Duration("remaining", time.Until(deadline)))
	}
	if err != nil {
		logging.ErrorContext(ctx, "platform sandbox selftest stage failed", apperr.ErrSandboxSelftestFailed.WithCause(err).LogString(), attrs...)
		return
	}
	slog.Default().LogAttrs(ctx, slog.LevelInfo, "platform sandbox selftest stage completed", logging.AttrsFromContext(ctx, attrs...)...)
}

// materializePlatformSelftestExecTarget 将判题器 manifest 的声明式 Pod 名绑定到本次组合实例。
// manifest 只声明 sandbox/testcase-evm 这类稳定目标,实际沙箱必须使用 source-chain-sandbox 等实例化名称。
func materializePlatformSelftestExecTarget(target, workspaceInstance string) (string, error) {
	if !safeExecTarget(strings.TrimSpace(target)) || strings.TrimSpace(workspaceInstance) == "" {
		return "", apperr.ErrSandboxContractRequestInvalid
	}
	parts := strings.SplitN(strings.TrimSpace(target), "/", 2)
	return runtimePodName(workspaceInstance, parts[0]) + "/" + parts[1], nil
}

// validatePlatformSelftestRequest 校验平台自检只能执行完整 argv,并限制在判题私有组合内。
func validatePlatformSelftestRequest(req contracts.SandboxPlatformSelftestRequest) error {
	if req.CompositionSnapshot.Spec.AccessProfile != contracts.SandboxAccessJudgePrivate || !safeExecTarget(strings.TrimSpace(req.ExecTarget)) || req.TimeoutSec <= 0 || len(req.Commands) == 0 {
		return apperr.ErrSandboxContractRequestInvalid
	}
	if strings.TrimSpace(req.CompositionSnapshot.Digest) == "" {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	for _, command := range req.Commands {
		if !workload.ValidCommand(command) {
			return apperr.ErrSandboxContractRequestInvalid
		}
	}
	return nil
}

// runtimePlansForSnapshot 从平台目录读取快照锁定的运行时和镜像,不从当前目录反推执行版本。
func (s *Service) runtimePlansForSnapshot(ctx context.Context, snapshot contracts.SandboxCompositionSnapshot) ([]RuntimePlan, error) {
	runtimePlans := make([]RuntimePlan, 0, len(snapshot.Runtimes))
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		for _, frozen := range snapshot.Runtimes {
			runtime, err := tx.GetRuntimeByID(ctx, frozen.RuntimeID)
			if err != nil {
				return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
			}
			if runtime.Code != frozen.Code || runtime.PluginRef != frozen.PluginRef || runtime.Status != RuntimeStatusAvailable || runtime.SelftestStatus != RuntimeSelftestPassed {
				return apperr.ErrSandboxRuntimeUnavailable.WithCause(fmt.Errorf("平台自检运行时门禁拒绝: runtime_id=%d instance=%s code=%s frozen_code=%s status=%d selftest_status=%d", runtime.ID, frozen.InstanceCode, runtime.Code, frozen.Code, runtime.Status, runtime.SelftestStatus))
			}
			var frozenAdapter AdapterSpec
			if err := jsonx.DecodeStrictKnownFields(frozen.AdapterSpec, &frozenAdapter); err != nil {
				return apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
			}
			runtime.AdapterSpec = materializeRuntimeTopology(frozen.InstanceCode, frozenAdapter)
			runtime.AdapterLevel = frozen.AdapterLevel
			runtime.CapabilityImpl = frozen.CapabilityImpl
			runtime.PluginRef = frozen.PluginRef
			image, err := tx.GetRuntimeImageByID(ctx, frozen.RuntimeID, frozen.ImageID)
			if err != nil {
				return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
			}
			if image.ImageURL != frozen.ImageURL || image.Version != frozen.ImageVersion {
				return apperr.ErrSandboxRuntimeUnavailable.WithCause(fmt.Errorf("平台自检镜像与冻结声明不一致: runtime_id=%d image_id=%d composition_digest=%s", runtime.ID, image.ID, snapshot.Digest))
			}
			prepull, err := tx.GetCompositionPrepullForUpdate(ctx, image.ID, snapshot.Digest)
			if err != nil {
				return apperr.ErrSandboxRuntimeUnavailable.WithCause(fmt.Errorf("读取平台自检组合预拉取证明失败: runtime_id=%d image_id=%d composition_digest=%s: %w", runtime.ID, image.ID, snapshot.Digest, err))
			}
			if prepull.Status != ImagePrepullSucceeded {
				return apperr.ErrSandboxRuntimeUnavailable.WithCause(fmt.Errorf("平台自检组合预拉取尚未通过: runtime_id=%d image_id=%d composition_digest=%s status=%d", runtime.ID, image.ID, snapshot.Digest, prepull.Status))
			}
			runtimePlans = append(runtimePlans, RuntimePlan{InstanceCode: frozen.InstanceCode, Runtime: runtime, Image: image})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return runtimePlans, nil
}
