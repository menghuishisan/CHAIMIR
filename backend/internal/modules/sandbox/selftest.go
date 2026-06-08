// M2 运行时接入即测实现。
// 自检在独立沙箱中按文档固定流程执行:起链 -> deploy -> query -> reset,全部通过才可用。
package sandbox

import (
	"context"
	"log/slog"
	"strings"

	"chaimir/internal/modules/sandbox/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// RunRuntimeSelftest 执行接入自检。
func (s *Service) RunRuntimeSelftest(ctx context.Context, runtimeID int64) (map[string]any, error) {
	var runtime sqlcgen.Runtime
	var image sqlcgen.RuntimeImage
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		runtime, err = q.GetRuntimeByID(ctx, runtimeID)
		if err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrRuntimeNotFound
			}
			return err
		}
		image, err = q.GetDefaultRuntimeImage(ctx, runtime.ID)
		if err != nil && db.IsNoRows(err) {
			return apperr.ErrRuntimeImageNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	if !runtime.CapabilityImpl.Valid || strings.TrimSpace(runtime.CapabilityImpl.String) == "" {
		return s.finishRuntimeSelftest(ctx, runtimeID, RuntimeSelftestFailed, RuntimeStatusOnboarding, map[string]any{
			"passed": false,
			"reason": "运行时能力实现器尚未注册,无法执行接入自检",
		})
	}
	capability, ok := s.capabilities.Get(runtime.CapabilityImpl.String)
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

	spec, err := parseRuntimeAdapterSpec(runtime.AdapterSpec)
	if err != nil {
		return nil, err
	}
	selftestSandbox := SandboxCreateSpec{
		SandboxID:      s.idgen.Generate(),
		TenantID:       0,
		Namespace:      sandboxNamespace("selftest", s.idgen.Generate()),
		Runtime:        RuntimeDefinition{ID: runtime.ID, Code: runtime.Code, Eco: runtime.Eco, CapabilityImpl: textValue(runtime.CapabilityImpl), AdapterSpec: spec},
		Image:          RuntimeImageDefinition{ID: image.ID, ImageURL: image.ImageUrl, Version: image.Version, GenesisBaked: image.GenesisBaked},
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
		if recycleErr := s.orchestrator.Recycle(context.Background(), selftestSandbox.Namespace); recycleErr != nil {
			logging.ErrorContext(ctx, "sandbox selftest recycle failed", recycleErr.Error(), slog.String("namespace", selftestSandbox.Namespace))
		}
	}()

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

// finishRuntimeSelftest 统一落库并返回运行时视图。
func (s *Service) finishRuntimeSelftest(ctx context.Context, runtimeID int64, selftestStatus, runtimeStatus int16, detail map[string]any) (map[string]any, error) {
	data, err := jsonx.ObjectBytes(detail, apperr.ErrRuntimeSelftestFailed)
	if err != nil {
		return nil, err
	}
	var row sqlcgen.Runtime
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.UpdateRuntimeSelftest(ctx, sqlcgen.UpdateRuntimeSelftestParams{
			ID: runtimeID, SelftestStatus: selftestStatus, SelftestDetail: data, Status: runtimeStatus,
		})
		return err
	}); err != nil {
		return nil, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	if err := s.writeAudit(ctx, 0, auditActionRuntimeSelftest, auditTargetRuntime, runtimeID, map[string]any{
		"selftest_status": selftestStatus,
		"runtime_status":  runtimeStatus,
	}); err != nil {
		return nil, err
	}
	return runtimeToMap(row), nil
}
