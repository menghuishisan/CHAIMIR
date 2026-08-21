// sandbox service_runtime 文件实现运行时、镜像、工具和配额管理编排。
package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

	"github.com/jackc/pgx/v5"
)

// RegisterRuntime 注册或更新运行时声明式适配器清单。
func (s *Service) RegisterRuntime(ctx context.Context, req RuntimeRequest) (Runtime, error) {
	spec, err := validateRuntimeRequest(req, s.cfg)
	if err != nil {
		return Runtime{}, err
	}
	applyBuiltinCapabilityDefault(&req, spec)
	var runtime Runtime
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		previous, previousErr := tx.GetRuntimeByCode(ctx, req.Code)
		hasPrevious := previousErr == nil
		if previousErr != nil && !errors.Is(previousErr, pgx.ErrNoRows) {
			return apperr.ErrSandboxRuntimePersistFailed.WithCause(previousErr)
		}
		if hasPrevious {
			// 注册接口按 code 幂等时仍锁定现有行,避免并发更新基于同一份旧自检状态互相覆盖。
			previous, previousErr = tx.GetRuntimeByIDForUpdate(ctx, previous.ID)
			if previousErr != nil {
				return apperr.ErrSandboxRuntimePersistFailed.WithCause(previousErr)
			}
		}
		selftestStatus, selftestDetail, executionChanged, err := runtimeStateForUpsert(previous, hasPrevious, &req, spec)
		if err != nil {
			return apperr.ErrSandboxRuntimePersistFailed.WithCause(err)
		}
		runtime, err = tx.UpsertRuntime(ctx, s.ids.Generate(), req, spec, selftestStatus, selftestDetail)
		if err != nil {
			return apperr.ErrSandboxRuntimePersistFailed.WithCause(err)
		}
		if executionChanged && runtimePrepullContractChanged(previous, runtime) {
			if err := invalidateRuntimePrepull(ctx, tx, runtime.ID, "runtime_prepull_contract_changed", runtime.Code); err != nil {
				return apperr.ErrSandboxRuntimePersistFailed.WithCause(err)
			}
		}
		return nil
	}); err != nil {
		return Runtime{}, err
	}
	return runtime, s.writeAuditFromContext(ctx, 0, "sandbox.runtime.upsert", "runtime", runtime.ID, map[string]any{"code": runtime.Code})
}

// runtimeStateForUpsert 只在实际执行契约变化时重置自检,展示字段更新不会破坏已验证状态。
func runtimeStateForUpsert(previous Runtime, hasPrevious bool, req *RuntimeRequest, spec AdapterSpec) (int16, []byte, bool, error) {
	if req == nil {
		return 0, nil, false, fmt.Errorf("运行时请求不能为空")
	}
	if !hasPrevious {
		if req.Status == 0 {
			req.Status = RuntimeStatusOnboarding
		}
		return RuntimeSelftestPending, []byte(`{}`), false, nil
	}
	executionChanged := runtimeExecutionContractChanged(previous, *req, spec)
	if req.Status == 0 {
		req.Status = previous.Status
	}
	if !executionChanged {
		return previous.SelftestStatus, append([]byte(nil), previous.SelftestDetail...), false, nil
	}
	if req.Status != RuntimeStatusDisabled {
		req.Status = RuntimeStatusOnboarding
	}
	detail, err := jsonBytes(map[string]any{"result": "pending", "reason": "runtime_execution_contract_changed"})
	if err != nil {
		return 0, nil, false, err
	}
	return RuntimeSelftestPending, detail, true, nil
}

// runtimeExecutionContractChanged 比较会影响沙箱启动、链能力或工具兼容性的字段,名称和状态不属于执行契约。
func runtimeExecutionContractChanged(previous Runtime, req RuntimeRequest, spec AdapterSpec) bool {
	return !strings.EqualFold(strings.TrimSpace(previous.Eco), strings.TrimSpace(req.Eco)) ||
		previous.AdapterLevel != req.AdapterLevel ||
		!reflect.DeepEqual(previous.AdapterSpec, spec) ||
		strings.TrimSpace(previous.CapabilityImpl) != strings.TrimSpace(req.CapabilityImpl) ||
		strings.TrimSpace(previous.PluginRef) != strings.TrimSpace(req.PluginRef)
}

// UpdateRuntime 按路径 ID 更新运行时,防止请求体 code 误更新或新建其他运行时。
func (s *Service) UpdateRuntime(ctx context.Context, runtimeID int64, req RuntimeRequest) (Runtime, error) {
	if runtimeID <= 0 {
		return Runtime{}, apperr.ErrPathIDInvalid
	}
	spec, err := validateRuntimeRequest(req, s.cfg)
	if err != nil {
		if errors.Is(err, apperr.ErrSandboxRuntimeCreateInvalid) {
			return Runtime{}, apperr.ErrSandboxRuntimeUpdateInvalid
		}
		return Runtime{}, err
	}
	applyBuiltinCapabilityDefault(&req, spec)
	var runtime Runtime
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		existing, err := tx.GetRuntimeByIDForUpdate(ctx, runtimeID)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		if strings.TrimSpace(req.Code) != existing.Code {
			return apperr.ErrSandboxRuntimeUpdateInvalid
		}
		selftestStatus, selftestDetail, executionChanged, err := runtimeStateForUpsert(existing, true, &req, spec)
		if err != nil {
			return apperr.ErrSandboxRuntimePersistFailed.WithCause(err)
		}
		runtime, err = tx.UpsertRuntime(ctx, runtimeID, req, spec, selftestStatus, selftestDetail)
		if err != nil {
			return apperr.ErrSandboxRuntimePersistFailed.WithCause(err)
		}
		if executionChanged && runtimePrepullContractChanged(existing, runtime) {
			if err := invalidateRuntimePrepull(ctx, tx, runtime.ID, "runtime_prepull_contract_changed", runtime.Code); err != nil {
				return apperr.ErrSandboxRuntimePersistFailed.WithCause(err)
			}
		}
		return nil
	}); err != nil {
		return Runtime{}, err
	}
	return runtime, s.writeAuditFromContext(ctx, 0, "sandbox.runtime.update", "runtime", runtimeID, map[string]any{"code": runtime.Code})
}

// ListRuntimes 查询平台已登记运行时列表。
func (s *Service) ListRuntimes(ctx context.Context) ([]Runtime, error) {
	var out []Runtime
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListRuntimes(ctx)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// RegisterRuntimeImage 登记运行时镜像版本并校验受控证明清单。
func (s *Service) RegisterRuntimeImage(ctx context.Context, runtimeID int64, req RuntimeImageRequest) (RuntimeImage, error) {
	if runtimeID <= 0 || strings.TrimSpace(req.ImageURL) == "" || strings.TrimSpace(req.Version) == "" || !imageAttested(s.cfg, req.ImageURL, req.Digest) {
		return RuntimeImage{}, apperr.ErrSandboxImageCreateInvalid
	}
	var image RuntimeImage
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		image, err = tx.CreateRuntimeImage(ctx, s.ids.Generate(), runtimeID, req)
		if err != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return RuntimeImage{}, err
	}
	return image, s.writeAuditFromContext(ctx, 0, "sandbox.image.register", "runtime_image", image.ID, map[string]any{"image_url": image.ImageURL})
}

// DisableRuntimeImage 停用运行时镜像并删除预拉取 DaemonSet,避免停用镜像继续留在节点预拉取闭环。
func (s *Service) DisableRuntimeImage(ctx context.Context, runtimeID, imageID int64) (RuntimeImage, error) {
	if runtimeID <= 0 || imageID <= 0 {
		return RuntimeImage{}, apperr.ErrSandboxImageDisableParamInvalid
	}
	var image RuntimeImage
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		image, err = tx.GetRuntimeImageByID(ctx, runtimeID, imageID)
		if err != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return RuntimeImage{}, err
	}
	if err := s.orchestrator.DeletePrepullDaemonSet(ctx, image); err != nil {
		return RuntimeImage{}, apperr.ErrSandboxImageDisableFailed.WithCause(err)
	}
	detail, err := jsonBytes(map[string]any{"stage": "disabled"})
	if err != nil {
		return RuntimeImage{}, apperr.ErrSandboxImageDisableFailed.WithCause(err)
	}
	var disabled RuntimeImage
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		disabled, err = tx.DisableRuntimeImage(ctx, runtimeID, imageID, detail)
		if err != nil {
			return apperr.ErrSandboxImageDisableFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return RuntimeImage{}, err
	}
	return disabled, s.writeAuditFromContext(ctx, 0, "sandbox.image.disable", "runtime_image", imageID, map[string]any{"runtime_id": runtimeID})
}

// ListRuntimeImages 查询指定运行时的镜像版本列表。
func (s *Service) ListRuntimeImages(ctx context.Context, runtimeID int64) ([]RuntimeImage, error) {
	if runtimeID <= 0 {
		return nil, apperr.ErrPathIDInvalid
	}
	var out []RuntimeImage
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListRuntimeImages(ctx, runtimeID)
		if err != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// RunRuntimeSelftest 创建临时沙箱执行运行时声明的接入即测命令并持久化结果。
func (s *Service) RunRuntimeSelftest(ctx context.Context, runtimeID int64) (RuntimeSelftestResponse, error) {
	if runtimeID <= 0 {
		return RuntimeSelftestResponse{}, apperr.ErrPathIDInvalid
	}
	if s.cfg.SelftestRecycleTimeoutSeconds <= 0 {
		return RuntimeSelftestResponse{}, apperr.ErrSandboxSelftestRecycleConfigInvalid
	}
	attemptID := ids.Format(s.ids.Generate())
	startingDetail, err := jsonBytes(map[string]any{"result": "running", "stage": "selftest", "attempt_id": attemptID})
	if err != nil {
		return RuntimeSelftestResponse{}, apperr.ErrSandboxSelftestFailed.WithCause(err)
	}
	var runtime Runtime
	var image RuntimeImage
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var txErr error
		runtime, txErr = tx.GetRuntimeByIDForUpdate(ctx, runtimeID)
		if txErr != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(txErr)
		}
		if !canRunRuntimeSelftest(runtime.Status) {
			return apperr.ErrSandboxRuntimeUnavailable
		}
		image, txErr = tx.GetDefaultRuntimeImageForShare(ctx, runtimeID)
		if txErr != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(txErr)
		}
		if !image.Prepulled || image.PrepullStatus != ImagePrepullSucceeded || !image.GenesisBaked {
			return apperr.ErrSandboxRuntimeUnavailable
		}
		_, txErr = tx.StartRuntimeSelftest(ctx, runtimeID, startingDetail)
		if txErr != nil {
			return apperr.ErrSandboxSelftestFailed.WithCause(txErr)
		}
		return nil
	}); err != nil {
		return RuntimeSelftestResponse{}, err
	}
	operationCtx, operationCancel := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.SelftestRecycleTimeoutSeconds))
	defer operationCancel()
	selftestID := s.ids.Generate()
	sb := Sandbox{
		ID:             selftestID,
		TenantID:       0,
		RuntimeID:      runtime.ID,
		ImageID:        image.ID,
		Namespace:      namespaceFor("sbx-selftest", selftestID),
		Phase:          SandboxPhaseAllocating,
		Status:         SandboxStatusCreating,
		OwnerAccountID: 0,
	}
	err = s.orchestrator.CreateSandboxResources(operationCtx, CreateSandboxPlan{Sandbox: sb, Runtime: runtime, Image: image})
	if err == nil {
		_, _, err = s.orchestrator.Exec(operationCtx, sb.Namespace, runtimeExecTarget(runtime), runtime.AdapterSpec.WorkspaceOps.Selftest, nil, false)
	}
	if err == nil {
		err = s.runRuntimeCapabilitySelftest(operationCtx, sb, runtime)
	}
	cleanupBase := logging.WithAttrs(context.WithoutCancel(ctx), logging.AttrsFromContext(ctx)...)
	cleanupCtx, cleanupCancel := context.WithTimeout(cleanupBase, timeDurationSeconds(s.cfg.SelftestRecycleTimeoutSeconds))
	defer cleanupCancel()
	if cleanupErr := s.orchestrator.DestroySandboxResources(cleanupCtx, sb); cleanupErr != nil {
		logging.ErrorContext(operationCtx, "sandbox selftest cleanup failed", cleanupErr.Error(), slog.Int64("tenant_id", 0), slog.Int64("runtime_id", runtimeID), slog.Int64("sandbox_id", sb.ID), slog.String("namespace", sb.Namespace))
	}
	status := RuntimeSelftestPassed
	runtimeStatus := RuntimeStatusAvailable
	detail, encodeErr := jsonBytes(map[string]any{"result": "passed", "attempt_id": attemptID})
	if encodeErr != nil {
		return RuntimeSelftestResponse{}, apperr.ErrSandboxSelftestFailed.WithCause(encodeErr)
	}
	if err != nil {
		status = RuntimeSelftestFailed
		runtimeStatus = RuntimeStatusOnboarding
		logging.ErrorContext(operationCtx, "sandbox runtime selftest failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", 0), slog.Int64("runtime_id", runtimeID), slog.Int64("sandbox_id", sb.ID), slog.String("namespace", sb.Namespace))
		detail, encodeErr = jsonBytes(map[string]any{"result": "failed", "stage": "selftest", "trace_id": traceIDFromLogContext(operationCtx), "attempt_id": attemptID})
		if encodeErr != nil {
			return RuntimeSelftestResponse{}, apperr.ErrSandboxSelftestFailed.WithCause(encodeErr)
		}
	}
	persistCtx, persistCancel := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer persistCancel()
	var updated Runtime
	if updateErr := s.store.PlatformTx(persistCtx, func(ctx context.Context, tx TxStore) error {
		var finishErr error
		updated, finishErr = tx.FinishRuntimeSelftest(ctx, runtimeID, attemptID, status, runtimeStatus, detail)
		if finishErr != nil {
			return apperr.ErrSandboxSelftestFailed.WithCause(finishErr)
		}
		return nil
	}); updateErr != nil {
		return RuntimeSelftestResponse{}, updateErr
	}
	if auditErr := s.writeAuditFromContext(persistCtx, 0, "sandbox.runtime.selftest", "runtime", runtimeID, map[string]any{"status": status, "attempt_id": attemptID}); auditErr != nil {
		return RuntimeSelftestResponse{}, auditErr
	}
	publicDetail, detailErr := runtimeSelftestPublicDetail(updated.SelftestDetail)
	if detailErr != nil {
		return RuntimeSelftestResponse{}, apperr.ErrSandboxSelftestFailed.WithCause(detailErr)
	}
	resp := RuntimeSelftestResponse{RuntimeID: ids.ID(runtimeID), SelftestStatus: updated.SelftestStatus, RuntimeStatus: updated.Status, Detail: publicDetail}
	if err != nil {
		return resp, apperr.ErrSandboxSelftestFailed.WithCause(err)
	}
	return resp, nil
}

// canRunRuntimeSelftest 只允许接入中或已可用运行时执行自检,停用状态不能被旧自检重新启用。
func canRunRuntimeSelftest(status int16) bool {
	return status == RuntimeStatusOnboarding || status == RuntimeStatusAvailable
}

// traceIDFromLogContext 从统一日志上下文读取 trace_id,用于持久状态只暴露报障编号。
func traceIDFromLogContext(ctx context.Context) string {
	for _, attr := range logging.AttrsFromContext(ctx) {
		if attr.Key == "trace_id" {
			return attr.Value.String()
		}
	}
	return ""
}

// runtimeSelftestPublicDetail 只向平台界面输出稳定的自检状态和报障编号。
// 持久化明细可能包含历史编排信息，因此不能作为 HTTP JSON 原样透传。
func runtimeSelftestPublicDetail(raw json.RawMessage) (RuntimeSelftestDetail, error) {
	var stored RuntimeSelftestDetail
	if err := jsonx.DecodeStrict(raw, &stored); err != nil {
		return RuntimeSelftestDetail{}, err
	}
	return stored, nil
}

// runRuntimeCapabilitySelftest 用标准 L2 能力执行 reset/deploy/query/reset 自检闭环。
func (s *Service) runRuntimeCapabilitySelftest(ctx context.Context, sb Sandbox, runtime Runtime) error {
	if runtime.AdapterLevel < RuntimeAdapterLevelStandard && strings.TrimSpace(runtime.CapabilityImpl) == "" && strings.TrimSpace(runtime.PluginRef) == "" {
		return nil
	}
	cap, err := s.resolveCapability(runtime)
	if err != nil {
		return err
	}
	if err := cap.Reset(ctx, sb, runtime); err != nil {
		return err
	}
	payload, ok := runtime.AdapterSpec.Selftest["deploy_payload"].(map[string]any)
	if !ok || len(payload) == 0 {
		return apperr.ErrSandboxSelftestSpecInvalid
	}
	if _, err := cap.Deploy(ctx, sb, runtime, payload); err != nil {
		return err
	}
	if txPayload, ok := runtime.AdapterSpec.Selftest["tx_payload"].(map[string]any); ok && len(txPayload) > 0 {
		if _, err := cap.SendTx(ctx, sb, runtime, txPayload); err != nil {
			return err
		}
	}
	target, ok := runtime.AdapterSpec.Selftest["query_target"].(string)
	if !ok || strings.TrimSpace(target) == "" {
		return apperr.ErrSandboxSelftestSpecInvalid
	}
	if _, err := cap.Query(ctx, sb, runtime, strings.TrimSpace(target)); err != nil {
		return err
	}
	return cap.Reset(ctx, sb, runtime)
}

// GetRuntimeSelftest 查询运行时接入即测结果。
func (s *Service) GetRuntimeSelftest(ctx context.Context, runtimeID int64) (RuntimeSelftestResponse, error) {
	if runtimeID <= 0 {
		return RuntimeSelftestResponse{}, apperr.ErrPathIDInvalid
	}
	var runtime Runtime
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		runtime, err = tx.GetRuntimeByID(ctx, runtimeID)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return RuntimeSelftestResponse{}, err
	}
	publicDetail, err := runtimeSelftestPublicDetail(runtime.SelftestDetail)
	if err != nil {
		return RuntimeSelftestResponse{}, apperr.ErrSandboxSelftestFailed.WithCause(err)
	}
	return RuntimeSelftestResponse{RuntimeID: ids.ID(runtime.ID), SelftestStatus: runtime.SelftestStatus, RuntimeStatus: runtime.Status, Detail: publicDetail}, nil
}

// PrepullRuntimeImage 触发 DaemonSet 全节点预拉取并以真实节点状态更新数据库。
func (s *Service) PrepullRuntimeImage(ctx context.Context, runtimeID, imageID int64) (PrepullResponse, error) {
	if runtimeID <= 0 || imageID <= 0 {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullParamInvalid
	}
	attemptID := ids.Format(s.ids.Generate())
	var runtime Runtime
	var image RuntimeImage
	var prepullSpecs []PrepullImageSpec
	var imageURLs []string
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		// 先锁证明行,再读取闭包。并发运行时/工具变更若先完成,本次读取新定义;
		// 若后发生,其失效更新会在本事务提交后覆盖 running,旧尝试无法写回成功。
		image, err = tx.GetRuntimeImageByIDForUpdate(ctx, runtimeID, imageID)
		if err != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		runtime, err = tx.GetRuntimeByID(ctx, runtimeID)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		tools, err := tx.ListTools(ctx)
		if err != nil {
			return apperr.ErrSandboxToolNotFound.WithCause(err)
		}
		if !canPrepullRuntime(runtime.Status) || image.Status != RuntimeImageStatusAvailable {
			return apperr.ErrSandboxRuntimeUnavailable
		}
		prepullSpecs, err = prepullImageSpecsForRuntime(runtime, image, tools)
		if err != nil {
			return apperr.ErrSandboxImageAttestationInvalid.WithCause(err)
		}
		if len(prepullSpecs) == 0 {
			return apperr.ErrSandboxImageAttestationInvalid
		}
		imageURLs = prepullImageURLs(prepullSpecs)
		for _, imageURL := range imageURLs {
			if !imageAttested(s.cfg, imageURL, digestFromImageURL(imageURL)) {
				return apperr.ErrSandboxImageAttestationInvalid
			}
		}
		if !containsString(imageURLs, image.ImageURL) {
			return apperr.ErrSandboxImageAttestationInvalid
		}
		startingDetail, err := prepullAttemptDetail("starting", attemptID, PrepullResult{}, imageURLs, nil)
		if err != nil {
			return apperr.ErrSandboxImagePrepullFailed.WithCause(err)
		}
		_, err = tx.StartRuntimeImagePrepull(ctx, runtimeID, imageID, startingDetail)
		return err
	}); err != nil {
		return PrepullResponse{}, err
	}
	// 一旦 running 状态提交,即使管理员关闭页面也必须继续等待 K8s 并写回最终结果,避免永久卡在进行中。
	operationCtx, cancel := asyncPersistenceContext(
		ctx,
		timeDurationSeconds(s.cfg.PrepullTimeoutSeconds+s.cfg.ReadyTimeoutSeconds),
	)
	defer cancel()
	result, err := s.orchestrator.PrepullImage(operationCtx, image, prepullSpecs)
	status := ImagePrepullSucceeded
	prepulled := true
	at := timex.Now()
	if err != nil {
		status = ImagePrepullFailed
		prepulled = false
		at = time.Time{}
		logging.ErrorContext(operationCtx, "sandbox image prepull failed", logging.SanitizeError(err.Error()), slog.Int64("tenant_id", 0), slog.Int64("runtime_id", runtimeID), slog.Int64("image_id", imageID), slog.String("daemonset", result.DaemonSet), slog.String("attempt_id", attemptID))
	}
	stage := "succeeded"
	if err != nil {
		stage = "failed"
	}
	detail, encodeErr := prepullAttemptDetail(stage, attemptID, result, imageURLs, err)
	if encodeErr != nil {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(encodeErr)
	}
	if updateErr := s.store.PlatformTx(operationCtx, func(ctx context.Context, tx TxStore) error {
		_, updateErr := tx.FinishRuntimeImagePrepull(ctx, runtimeID, imageID, attemptID, prepulled, status, detail, at)
		return updateErr
	}); updateErr != nil {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(updateErr)
	}
	if auditErr := s.writeAuditFromContext(operationCtx, 0, "sandbox.image.prepull", "runtime_image", imageID, map[string]any{"runtime_id": runtimeID, "status": status, "attempt_id": attemptID}); auditErr != nil {
		return PrepullResponse{}, auditErr
	}
	if err != nil {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(err)
	}
	return PrepullResponse{ImageID: ids.ID(imageID), PrepullStatus: status, DesiredNodes: result.DesiredNodes, ReadyNodes: result.ReadyNodes, ImageCount: len(imageURLs)}, nil
}

// prepullAttemptDetail 统一记录一次预拉取尝试的闭包、节点状态与脱敏失败原因。
func prepullAttemptDetail(stage, attemptID string, result PrepullResult, imageURLs []string, cause error) ([]byte, error) {
	detail := map[string]any{
		"stage":         stage,
		"attempt_id":    attemptID,
		"daemonset":     result.DaemonSet,
		"desired_nodes": result.DesiredNodes,
		"ready_nodes":   result.ReadyNodes,
		"image_count":   len(imageURLs),
		"images":        imageURLs,
	}
	if cause != nil {
		detail["error"] = logging.SanitizeError(cause.Error())
	}
	return jsonBytes(detail)
}

// canPrepullRuntime 判断运行时是否允许进入镜像预拉取阶段。
// 接入中的运行时必须先完成预拉取才能执行首次自检,已可用运行时允许刷新镜像,停用运行时禁止继续准备。
func canPrepullRuntime(status int16) bool {
	return status == RuntimeStatusOnboarding || status == RuntimeStatusAvailable
}

// prepullSpecCollector 按镜像、自检命令与常驻用途去重,保证真实预拉取与变更比较使用同一套闭包规则。
type prepullSpecCollector struct {
	seen  map[string]int
	specs []PrepullImageSpec
}

// newPrepullSpecCollector 创建保持声明顺序的预拉取集合收集器。
func newPrepullSpecCollector(capacity int) *prepullSpecCollector {
	return &prepullSpecCollector{seen: make(map[string]int), specs: make([]PrepullImageSpec, 0, capacity)}
}

// add 合并同镜像、同命令、同用途的临时目录,并要求每个真实镜像都带受控自检命令。
func (c *prepullSpecCollector) add(imageURL string, command []string, hold bool, mounts []workload.EphemeralMountSpec) error {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return nil
	}
	command = compactCommand(command)
	if len(command) == 0 {
		return fmt.Errorf("预拉取镜像 %s 缺少自检命令", imageURL)
	}
	key := prepullImageSpecKey(imageURL, command, hold)
	if index, exists := c.seen[key]; exists {
		c.specs[index].EphemeralMounts = mergePrepullEphemeralMounts(c.specs[index].EphemeralMounts, mounts)
		return nil
	}
	c.seen[key] = len(c.specs)
	c.specs = append(c.specs, PrepullImageSpec{
		ImageURL:        imageURL,
		Command:         command,
		Hold:            hold,
		EphemeralMounts: mergePrepullEphemeralMounts(nil, mounts),
	})
	return nil
}

// prepullImageSpecsForRuntime 汇总运行时默认工作负载会用到的不可变镜像和最小自检命令。
func prepullImageSpecsForRuntime(runtime Runtime, image RuntimeImage, tools []Tool) ([]PrepullImageSpec, error) {
	collector := newPrepullSpecCollector(1 + len(runtime.AdapterSpec.InfraSidecars) + len(tools))
	if err := collector.add(image.ImageURL, runtime.AdapterSpec.WorkspaceOps.Selftest, false, nil); err != nil {
		return nil, err
	}
	if err := addRuntimeAuxiliaryPrepullSpecs(collector, runtime.AdapterSpec); err != nil {
		return nil, err
	}
	for _, tool := range tools {
		if err := addToolPrepullSpecs(collector, runtime.Eco, tool); err != nil {
			return nil, err
		}
	}
	holdCount := 0
	for i := range collector.specs {
		if collector.specs[i].Hold {
			holdCount++
		}
	}
	if holdCount != 1 {
		return nil, fmt.Errorf("运行时 %s 的预拉取闭包必须且只能包含一个保持容器,实际为 %d 个", runtime.Code, holdCount)
	}
	return collector.specs, nil
}

// addRuntimeAuxiliaryPrepullSpecs 加入运行时声明的 infra 与独立 Pod 非主容器镜像。
func addRuntimeAuxiliaryPrepullSpecs(collector *prepullSpecCollector, spec AdapterSpec) error {
	for _, component := range spec.InfraSidecars {
		if err := collector.add(component.ImageURL, component.PrepullCommand, component.PrepullHold, component.EphemeralMounts); err != nil {
			return err
		}
	}
	for _, pod := range spec.Pods {
		for _, component := range pod.Containers {
			if strings.TrimSpace(component.Name) == strings.TrimSpace(spec.RuntimeContainer.Name) {
				continue
			}
			if err := collector.add(component.ImageURL, component.PrepullCommand, component.PrepullHold, component.EphemeralMounts); err != nil {
				return err
			}
		}
	}
	return nil
}

// addToolPrepullSpecs 按状态与生态加入一个工具对指定运行时的预拉取贡献。
func addToolPrepullSpecs(collector *prepullSpecCollector, eco string, tool Tool) error {
	if tool.Status != ToolStatusAvailable || !toolCompatible(eco, tool.EcoTags) {
		return nil
	}
	for _, component := range tool.ResourceSpec.Components {
		command := component.PrepullCommand
		if len(command) == 0 {
			command = tool.ResourceSpec.PrepullCommand
		}
		if err := collector.add(component.ImageURL, command, component.PrepullHold, component.EphemeralMounts); err != nil {
			return err
		}
	}
	return nil
}

// runtimePrepullContractChanged 只比较会改变预拉取镜像、自检命令、常驻用途或临时目录的声明。
func runtimePrepullContractChanged(previous, current Runtime) bool {
	if !strings.EqualFold(strings.TrimSpace(previous.Eco), strings.TrimSpace(current.Eco)) {
		return true
	}
	previousCollector := newPrepullSpecCollector(1 + len(previous.AdapterSpec.InfraSidecars))
	currentCollector := newPrepullSpecCollector(1 + len(current.AdapterSpec.InfraSidecars))
	if err := previousCollector.add("runtime-image", previous.AdapterSpec.WorkspaceOps.Selftest, false, nil); err != nil {
		return true
	}
	if err := currentCollector.add("runtime-image", current.AdapterSpec.WorkspaceOps.Selftest, false, nil); err != nil {
		return true
	}
	if err := addRuntimeAuxiliaryPrepullSpecs(previousCollector, previous.AdapterSpec); err != nil {
		return true
	}
	if err := addRuntimeAuxiliaryPrepullSpecs(currentCollector, current.AdapterSpec); err != nil {
		return true
	}
	return !equivalentPrepullSpecs(previousCollector.specs, currentCollector.specs)
}

// RuntimePrepullDefinitionChanged 供部署期 seed 复用运行时预拉取闭包比较,禁止维护第二套失效口径。
func RuntimePrepullDefinitionChanged(previousEco string, previousSpec AdapterSpec, currentEco string, currentSpec AdapterSpec) bool {
	return runtimePrepullContractChanged(
		Runtime{Eco: previousEco, AdapterSpec: previousSpec},
		Runtime{Eco: currentEco, AdapterSpec: currentSpec},
	)
}

// ToolPrepullDefinitionsChangedForEco 供部署期 seed 比较某生态全部工具的实际预拉取贡献。
func ToolPrepullDefinitionsChangedForEco(previous, current []Tool, eco string) (bool, error) {
	previousCollector := newPrepullSpecCollector(len(previous))
	for _, tool := range previous {
		if err := addToolPrepullSpecs(previousCollector, eco, tool); err != nil {
			return false, err
		}
	}
	currentCollector := newPrepullSpecCollector(len(current))
	for _, tool := range current {
		if err := addToolPrepullSpecs(currentCollector, eco, tool); err != nil {
			return false, err
		}
	}
	return !equivalentPrepullSpecs(previousCollector.specs, currentCollector.specs), nil
}

// equivalentPrepullSpecs 忽略声明顺序比较闭包;镜像、命令、常驻用途和挂载内容仍必须完全一致。
func equivalentPrepullSpecs(left, right []PrepullImageSpec) bool {
	return reflect.DeepEqual(canonicalPrepullSpecs(left), canonicalPrepullSpecs(right))
}

// canonicalPrepullSpecs 复制并排序预拉取声明,避免仅调整组件或临时目录顺序触发无意义重拉。
func canonicalPrepullSpecs(specs []PrepullImageSpec) []PrepullImageSpec {
	out := make([]PrepullImageSpec, 0, len(specs))
	for _, spec := range specs {
		item := PrepullImageSpec{
			ImageURL: strings.TrimSpace(spec.ImageURL),
			Command:  append([]string(nil), compactCommand(spec.Command)...),
			Hold:     spec.Hold,
		}
		item.EphemeralMounts = mergePrepullEphemeralMounts(nil, spec.EphemeralMounts)
		sort.Slice(item.EphemeralMounts, func(i, j int) bool {
			leftKey := item.EphemeralMounts[i].Name + "\x00" + item.EphemeralMounts[i].MountPath
			rightKey := item.EphemeralMounts[j].Name + "\x00" + item.EphemeralMounts[j].MountPath
			return leftKey < rightKey
		})
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		leftKey := prepullImageSpecKey(out[i].ImageURL, out[i].Command, out[i].Hold)
		rightKey := prepullImageSpecKey(out[j].ImageURL, out[j].Command, out[j].Hold)
		return leftKey < rightKey
	})
	return out
}

// prepullImageSpecKey 保持同镜像在不同自检命令和常驻用途下独立去重。
func prepullImageSpecKey(imageURL string, command []string, hold bool) string {
	return fmt.Sprintf("%s\x00%s\x00%t", imageURL, strings.Join(compactCommand(command), "\x00"), hold)
}

// mergePrepullEphemeralMounts 合并同镜像声明的预拉取临时目录,保留 manifest 首次出现顺序。
func mergePrepullEphemeralMounts(base, extra []workload.EphemeralMountSpec) []workload.EphemeralMountSpec {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := make([]workload.EphemeralMountSpec, 0, len(base)+len(extra))
	seen := map[string]struct{}{}
	add := func(mount workload.EphemeralMountSpec) {
		name := strings.TrimSpace(mount.Name)
		mountPath := strings.TrimSpace(mount.MountPath)
		if name == "" || mountPath == "" {
			return
		}
		key := name + "\x00" + mountPath
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		out = append(out, workload.EphemeralMountSpec{Name: name, MountPath: mountPath})
	}
	for _, mount := range base {
		add(mount)
	}
	for _, mount := range extra {
		add(mount)
	}
	return out
}

// prepullImageURLs 提取预拉取响应和审计使用的不可变镜像 URL 列表。
func prepullImageURLs(specs []PrepullImageSpec) []string {
	out := make([]string, 0, len(specs))
	seen := map[string]struct{}{}
	for _, spec := range specs {
		imageURL := strings.TrimSpace(spec.ImageURL)
		if imageURL == "" {
			continue
		}
		if _, exists := seen[imageURL]; exists {
			continue
		}
		seen[imageURL] = struct{}{}
		out = append(out, imageURL)
	}
	return out
}

// compactCommand 清理命令数组中的空白,保持 manifest 声明顺序。
func compactCommand(command []string) []string {
	out := make([]string, 0, len(command))
	for _, part := range command {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// GetRuntimeImagePrepull 查询镜像预拉取状态,只返回文档允许的进度字段。
func (s *Service) GetRuntimeImagePrepull(ctx context.Context, runtimeID, imageID int64) (PrepullResponse, error) {
	if runtimeID <= 0 || imageID <= 0 {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullParamInvalid
	}
	var image RuntimeImage
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		image, err = tx.GetRuntimeImageByID(ctx, runtimeID, imageID)
		if err != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return PrepullResponse{}, err
	}
	resp := PrepullResponse{ImageID: ids.ID(image.ID), PrepullStatus: image.PrepullStatus}
	if len(image.PrepullDetail) == 0 {
		return resp, nil
	}
	var detail struct {
		Stage        string   `json:"stage"`
		AttemptID    string   `json:"attempt_id"`
		DesiredNodes int32    `json:"desired_nodes"`
		ReadyNodes   int32    `json:"ready_nodes"`
		DaemonSet    string   `json:"daemonset"`
		ImageCount   int      `json:"image_count"`
		Images       []string `json:"images"`
		Error        string   `json:"error"`
		Reason       string   `json:"reason"`
		Subject      string   `json:"subject"`
		Source       string   `json:"source"`
		Prepulled    bool     `json:"prepulled"`
	}
	if err := jsonx.DecodeStrictKnownFields(image.PrepullDetail, &detail); err != nil {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(err)
	}
	resp.DesiredNodes = detail.DesiredNodes
	resp.ReadyNodes = detail.ReadyNodes
	resp.ImageCount = detail.ImageCount
	return resp, nil
}

// RegisterTool 注册或更新工具定义。
func (s *Service) RegisterTool(ctx context.Context, req ToolRequest) (Tool, error) {
	spec, err := validateToolRequest(req, s.cfg)
	if err != nil {
		return Tool{}, err
	}
	if req.Status == 0 {
		req.Status = ToolStatusAvailable
	}
	var tool Tool
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		previous, previousErr := tx.GetToolByCode(ctx, req.Code)
		hasPrevious := previousErr == nil
		if previousErr != nil && !errors.Is(previousErr, pgx.ErrNoRows) {
			return apperr.ErrSandboxToolPersistFailed.WithCause(previousErr)
		}
		var err error
		tool, err = tx.UpsertTool(ctx, s.ids.Generate(), req, spec)
		if err != nil {
			return apperr.ErrSandboxToolPersistFailed.WithCause(err)
		}
		if err := invalidateToolAffectedRuntimePrepull(ctx, tx, previous, hasPrevious, tool); err != nil {
			return apperr.ErrSandboxToolPersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return Tool{}, err
	}
	return tool, s.writeAuditFromContext(ctx, 0, "sandbox.tool.upsert", "tool", tool.ID, map[string]any{"code": tool.Code})
}

// invalidateToolAffectedRuntimePrepull 只撤销工具对某个运行时生态的实际预拉取贡献发生变化的证明。
func invalidateToolAffectedRuntimePrepull(ctx context.Context, tx TxStore, previous Tool, hasPrevious bool, current Tool) error {
	currentTools, err := tx.ListTools(ctx)
	if err != nil {
		return err
	}
	previousTools := make([]Tool, 0, len(currentTools))
	for _, tool := range currentTools {
		if tool.Code == current.Code {
			if hasPrevious {
				previousTools = append(previousTools, previous)
			}
			continue
		}
		previousTools = append(previousTools, tool)
	}
	runtimes, err := tx.ListRuntimes(ctx)
	if err != nil {
		return err
	}
	for _, runtime := range runtimes {
		changed, compareErr := ToolPrepullDefinitionsChangedForEco(previousTools, currentTools, runtime.Eco)
		if compareErr != nil {
			return compareErr
		}
		if !changed {
			continue
		}
		if err := invalidateRuntimePrepull(ctx, tx, runtime.ID, "tool_prepull_contract_changed", current.Code); err != nil {
			return err
		}
	}
	return nil
}

// invalidateRuntimePrepull 写入可审计的失效原因并统一把运行时镜像版本重置为待预拉取。
func invalidateRuntimePrepull(ctx context.Context, tx TxStore, runtimeID int64, reason, subject string) error {
	detail, err := jsonBytes(map[string]any{
		"stage":   "invalidated",
		"reason":  reason,
		"subject": strings.TrimSpace(subject),
	})
	if err != nil {
		return err
	}
	return tx.InvalidateRuntimeImagesPrepull(ctx, runtimeID, detail)
}

// ListTools 查询平台已登记工具列表。
func (s *Service) ListTools(ctx context.Context) ([]Tool, error) {
	var out []Tool
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListTools(ctx)
		if err != nil {
			return apperr.ErrSandboxToolNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// ListOrchestrationCatalog 返回业务模块编排环境所需的运行时(含可用镜像版本)与工具目录。
// 只出可编排字段:适配器清单、镜像地址、命令白名单与自检详情属平台运维面,
// 由 §2 的管理接口承载,不经本目录外泄(见 docs/02-沙箱引擎/04-接口设计.md §2.1)。
func (s *Service) ListOrchestrationCatalog(ctx context.Context) ([]CatalogRuntime, []CatalogTool, error) {
	var runtimes []CatalogRuntime
	var runtimeDefinitions []Runtime
	var tools []CatalogTool
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		runtimes, err = tx.ListCatalogRuntimes(ctx)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		runtimeDefinitions, err = tx.ListRuntimes(ctx)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		tools, err = tx.ListCatalogTools(ctx)
		if err != nil {
			return apperr.ErrSandboxToolNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	definitionsByCode := make(map[string]Runtime, len(runtimeDefinitions))
	for _, runtime := range runtimeDefinitions {
		definitionsByCode[runtime.Code] = runtime
	}
	for i := range runtimes {
		runtime, exists := definitionsByCode[runtimes[i].Code]
		if !exists {
			return nil, nil, apperr.ErrSandboxRuntimeNotFound
		}
		runtimes[i].ToolCodes = make([]string, 0, len(tools))
		for _, tool := range tools {
			definition := Tool{
				Code: tool.Code, Name: tool.Name, Kind: tool.Kind, EcoTags: tool.EcoTags,
				ResourceSpec: tool.ResourceSpec, Status: ToolStatusAvailable,
			}
			if validateToolForRuntime(definition, runtime) == nil {
				runtimes[i].ToolCodes = append(runtimes[i].ToolCodes, tool.Code)
			}
		}
	}
	return runtimes, tools, nil
}

// UpsertQuota 调整租户资源配额。
func (s *Service) UpsertQuota(ctx context.Context, quota TenantQuota) (TenantQuota, error) {
	if err := validateQuota(quota); err != nil {
		return TenantQuota{}, err
	}
	var out TenantQuota
	if err := s.store.TenantTx(ctx, quota.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.UpsertTenantQuota(ctx, quota)
		if err != nil {
			return apperr.ErrSandboxQuotaPersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return TenantQuota{}, err
	}
	return out, s.writeAuditFromContext(ctx, quota.TenantID, "sandbox.quota.upsert", "tenant_quota", quota.TenantID, nil)
}

// applyBuiltinCapabilityDefault 为声明式 L2 命令运行时补齐内置能力实现键,避免 capability_impl 与清单重复配置。
func applyBuiltinCapabilityDefault(req *RuntimeRequest, spec AdapterSpec) {
	if req == nil || strings.TrimSpace(req.CapabilityImpl) != "" || strings.TrimSpace(req.PluginRef) != "" {
		return
	}
	if hasCapabilityCommands(spec.CapabilityCommands) {
		req.CapabilityImpl = BuiltinExecCapability
	}
}
