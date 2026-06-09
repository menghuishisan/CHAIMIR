// M2 运行时接入即测实现。
// 自检在独立沙箱中按文档固定流程执行:起链 -> deploy -> query -> reset,全部通过才可用。
package sandbox

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// RunRuntimeSelftest 执行运行时接入自检,通过一次性沙箱验证 boot/deploy/query/reset 能力。
func (s *Service) RunRuntimeSelftest(ctx context.Context, runtimeID int64) (map[string]any, error) {
	// 先读取运行时与默认镜像,自检只验证已登记的控制面配置。
	runtime, image, err := s.repo.getRuntimeWithDefaultImage(ctx, runtimeID)
	if err != nil {
		return nil, err
	}
	// 再解析能力实现器并确认它支持自检接口,否则运行时保持接入中状态。
	if strings.TrimSpace(runtime.CapabilityImpl) == "" {
		return s.finishRuntimeSelftest(ctx, runtimeID, RuntimeSelftestFailed, RuntimeStatusOnboarding, map[string]any{
			"passed": false,
			"reason": "运行时能力实现器尚未注册,无法执行接入自检",
		})
	}
	capability, ok := s.capabilities.Get(runtime.CapabilityImpl)
	if !ok {
		return s.finishRuntimeSelftest(ctx, runtimeID, RuntimeSelftestFailed, RuntimeStatusOnboarding, map[string]any{
			"passed": false,
			"reason": "运行时能力实现器未装配到服务",
		})
	}
	selftester, ok := capability.(RuntimeSelftester)
	if !ok {
		return s.finishRuntimeSelftest(ctx, runtimeID, RuntimeSelftestFailed, RuntimeStatusOnboarding, map[string]any{
			"passed": false,
			"reason": "当前能力实现器未提供接入即测能力",
		})
	}

	// 创建一次性自检沙箱并注册 defer 清理,避免失败路径遗留 K8s 资源。
	spec, err := parseRuntimeAdapterSpec(runtime.AdapterSpec)
	if err != nil {
		return nil, err
	}
	selftestSandbox := SandboxCreateSpec{
		SandboxID:      s.idgen.Generate(),
		TenantID:       0,
		Namespace:      sandboxNamespace("selftest", s.idgen.Generate()),
		Runtime:        RuntimeDefinition{ID: runtime.ID, Code: runtime.Code, Eco: runtime.Eco, CapabilityImpl: runtime.CapabilityImpl, AdapterSpec: spec},
		Image:          RuntimeImageDefinition{ID: image.ID, ImageURL: image.ImageURL, Version: image.Version, GenesisBaked: image.GenesisBaked},
		SourceRef:      "runtime:2026:selftest:" + ids.Format(runtimeID),
		CodeStorageKey: "",
	}
	if err := s.orchestrator.Create(ctx, selftestSandbox); err != nil {
		return s.finishRuntimeSelftest(ctx, runtimeID, RuntimeSelftestFailed, RuntimeStatusOnboarding, map[string]any{
			"passed": false,
			"reason": "自检沙箱创建失败",
			"error":  err.Error(),
		})
	}
	defer func() {
		if s.cfg.SelftestRecycleTimeoutSeconds <= 0 {
			logging.ErrorContext(ctx, "sandbox selftest recycle config invalid", apperr.ErrRuntimeSelftestConfigInvalid.Message,
				slog.String("namespace", selftestSandbox.Namespace))
			return
		}
		cleanupCtx, cancel := context.WithTimeout(detachRuntimeSelftestCleanupContext(ctx), time.Duration(s.cfg.SelftestRecycleTimeoutSeconds)*time.Second)
		defer cancel()
		if recycleErr := s.orchestrator.Recycle(cleanupCtx, selftestSandbox.Namespace); recycleErr != nil {
			logging.ErrorContext(ctx, "sandbox selftest recycle failed", recycleErr.Error(), slog.String("namespace", selftestSandbox.Namespace))
		}
	}()

	// 等待运行时就绪后执行能力自检,具体 boot/deploy/query/reset 步骤由能力实现器封装。
	if err := s.orchestrator.WaitReady(ctx, selftestSandbox.Namespace); err != nil {
		return s.finishRuntimeSelftest(ctx, runtimeID, RuntimeSelftestFailed, RuntimeStatusOnboarding, map[string]any{
			"passed": false,
			"reason": "自检沙箱未就绪",
			"error":  err.Error(),
		})
	}
	binding, err := s.orchestrator.RuntimeBinding(ctx, selftestSandbox.Namespace)
	if err != nil {
		return s.finishRuntimeSelftest(ctx, runtimeID, RuntimeSelftestFailed, RuntimeStatusOnboarding, map[string]any{
			"passed": false,
			"reason": "自检沙箱未就绪",
			"error":  err.Error(),
		})
	}
	binding.WorkspaceDir = spec.WorkspaceDir
	if err := selftester.Selftest(ctx, binding, spec.Selftest); err != nil {
		return s.finishRuntimeSelftest(ctx, runtimeID, RuntimeSelftestFailed, RuntimeStatusOnboarding, map[string]any{
			"passed": false,
			"reason": "接入即测失败",
			"error":  err.Error(),
		})
	}
	return s.finishRuntimeSelftest(ctx, runtimeID, RuntimeSelftestPassed, RuntimeStatusAvailable, map[string]any{
		"passed": true,
		"steps":  []string{"boot", "deploy", "query", "reset"},
	})
}

// detachRuntimeSelftestCleanupContext 保留日志追踪信息,并让自检清理不受请求取消影响。
func detachRuntimeSelftestCleanupContext(parent context.Context) context.Context {
	ctx := audit.WithRequestContext(context.Background(), audit.RequestContextFrom(parent))
	return logging.WithAttrs(ctx)
}

// finishRuntimeSelftest 统一落库并返回运行时视图。
func (s *Service) finishRuntimeSelftest(ctx context.Context, runtimeID int64, selftestStatus, runtimeStatus int16, detail map[string]any) (map[string]any, error) {
	data, err := jsonx.ObjectBytes(detail, apperr.ErrRuntimeSelftestFailed)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.updateRuntimeSelftest(ctx, runtimeID, selftestStatus, runtimeStatus, data)
	if err != nil {
		return nil, err
	}
	if err := s.writeAudit(ctx, 0, auditActionRuntimeSelftest, auditTargetRuntime, runtimeID, map[string]any{
		"selftest_status": selftestStatus,
		"runtime_status":  runtimeStatus,
	}); err != nil {
		return nil, err
	}
	return runtimeToMap(row), nil
}
