// service_platform_selftest 文件实现平台级临时沙箱自检,不创建租户业务沙箱记录。
package sandbox

import (
	"context"
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

	operationCtx, operationCancel := context.WithTimeout(ctx, time.Duration(req.TimeoutSec)*time.Second)
	defer operationCancel()
	var runErr error
	if runErr = s.orchestrator.CreateSandboxResources(operationCtx, plan); runErr == nil {
		for _, command := range req.Commands {
			_, stderr, commandErr := s.orchestrator.Exec(operationCtx, sb.Namespace, execTarget, command, nil, false)
			if commandErr != nil {
				runErr = fmt.Errorf("平台自检命令 %q 执行失败: %w: %s", command[0], commandErr, strings.TrimSpace(string(stderr)))
				break
			}
		}
	}

	cleanupBase := logging.WithAttrs(context.WithoutCancel(ctx), logging.AttrsFromContext(ctx)...)
	cleanupTimeout := s.cfg.SelftestRecycleTimeoutSeconds
	if cleanupTimeout <= 0 {
		cleanupTimeout = int(req.TimeoutSec)
	}
	cleanupCtx, cleanupCancel := context.WithTimeout(cleanupBase, time.Duration(cleanupTimeout)*time.Second)
	defer cleanupCancel()
	if cleanupErr := s.orchestrator.DestroySandboxResources(cleanupCtx, sb); cleanupErr != nil {
		logging.ErrorContext(cleanupCtx, "platform sandbox selftest cleanup failed", cleanupErr.Error(), slog.Int64("sandbox_id", sb.ID), slog.String("namespace", sb.Namespace))
		if runErr == nil {
			runErr = apperr.ErrSandboxSelftestFailed.WithCause(cleanupErr)
		}
	}
	return runErr
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
			if runtime.Code != frozen.Code || runtime.Status != RuntimeStatusAvailable || runtime.SelftestStatus != RuntimeSelftestPassed {
				return apperr.ErrSandboxRuntimeUnavailable
			}
			if err := jsonx.DecodeStrictKnownFields(frozen.AdapterSpec, &runtime.AdapterSpec); err != nil {
				return apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
			}
			runtime.AdapterSpec = materializeRuntimeTopology(frozen.InstanceCode, runtime.AdapterSpec)
			runtime.AdapterLevel = frozen.AdapterLevel
			runtime.CapabilityImpl = frozen.CapabilityImpl
			image, err := tx.GetRuntimeImageByID(ctx, frozen.RuntimeID, frozen.ImageID)
			if err != nil {
				return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
			}
			prepull, err := tx.GetCompositionPrepullForUpdate(ctx, image.ID, snapshot.Digest)
			if err != nil || image.ImageURL != frozen.ImageURL || image.Version != frozen.ImageVersion || prepull.Status != ImagePrepullSucceeded {
				return apperr.ErrSandboxRuntimeUnavailable
			}
			runtimePlans = append(runtimePlans, RuntimePlan{InstanceCode: frozen.InstanceCode, Runtime: runtime, Image: image})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return runtimePlans, nil
}
