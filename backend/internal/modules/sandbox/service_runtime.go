// sandbox service_runtime 文件实现运行时、镜像、工具和配额管理编排。
package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/prepull"
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

// GetRuntime 按 ID 查询单个运行时,供平台详情页深链和刷新使用。
func (s *Service) GetRuntime(ctx context.Context, runtimeID int64) (Runtime, error) {
	if runtimeID <= 0 {
		return Runtime{}, apperr.ErrSandboxRuntimeNotFound
	}
	var out Runtime
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.GetRuntimeByID(ctx, runtimeID)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return Runtime{}, err
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
	var disabled RuntimeImage
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		disabled, err = tx.DisableRuntimeImage(ctx, runtimeID, imageID)
		if err != nil {
			return apperr.ErrSandboxImageDisableFailed.WithCause(err)
		}
		if err := tx.DeleteCompositionPrepullByRuntimeImage(ctx, imageID); err != nil {
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
		images, listErr := tx.ListRuntimeImages(ctx, runtimeID)
		if listErr != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(listErr)
		}
		var selected bool
		image, selected = selectRuntimeSelftestImage(images)
		if !selected {
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
		Namespace:      namespaceFor("sbx-selftest", selftestID),
		Phase:          SandboxPhaseAllocating,
		Status:         SandboxStatusCreating,
		OwnerAccountID: 0,
	}
	plan := CreateSandboxPlan{Sandbox: sb, WorkspaceRuntimeInstance: runtime.Code, Runtimes: []RuntimePlan{{InstanceCode: runtime.Code, Runtime: runtime, Image: image}}}
	err = s.orchestrator.CreateSandboxResources(operationCtx, plan)
	if err == nil {
		_, _, err = s.orchestrator.Exec(operationCtx, sb.Namespace, workspaceExecTarget(runtime), runtime.AdapterSpec.WorkspaceOps.Selftest, nil, false)
	}
	if err == nil {
		err = s.runRuntimeCapabilitySelftest(operationCtx, plan)
	}
	cleanupBase := logging.WithAttrs(context.WithoutCancel(ctx), logging.AttrsFromContext(ctx)...)
	cleanupCtx, cleanupCancel := context.WithTimeout(cleanupBase, timeDurationSeconds(s.cfg.SelftestRecycleTimeoutSeconds))
	defer cleanupCancel()
	if cleanupErr := s.orchestrator.DestroySandboxResources(cleanupCtx, sb); cleanupErr != nil {
		logging.ErrorContext(operationCtx, "sandbox selftest cleanup failed", cleanupErr.Error(), slog.Int64("tenant_id", 0), slog.Int64("runtime_id", runtimeID), slog.Int64("sandbox_id", sb.ID), slog.String("namespace", sb.Namespace))
	}
	status, runtimeStatus := runtimeSelftestStatuses(runtime, err)
	detail, encodeErr := jsonBytes(map[string]any{"result": "passed", "attempt_id": attemptID})
	if encodeErr != nil {
		return RuntimeSelftestResponse{}, apperr.ErrSandboxSelftestFailed.WithCause(encodeErr)
	}
	if err == nil && runtimeStatus == RuntimeStatusOnboarding {
		detail, encodeErr = jsonBytes(map[string]any{
			"result":     "passed",
			"stage":      "node-selftest",
			"reason":     "运行时节点可启动,但尚未声明平台标准链能力",
			"attempt_id": attemptID,
		})
		if encodeErr != nil {
			return RuntimeSelftestResponse{}, apperr.ErrSandboxSelftestFailed.WithCause(encodeErr)
		}
	}
	if err != nil {
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

// runtimeSelftestStatuses 只根据基础工作负载自检结果决定运行时是否可部署；原生链动作由 adapter_level 单独控制。
func runtimeSelftestStatuses(runtime Runtime, selftestErr error) (int16, int16) {
	if selftestErr != nil {
		return RuntimeSelftestFailed, RuntimeStatusOnboarding
	}
	return RuntimeSelftestPassed, RuntimeStatusAvailable
}

// selectRuntimeSelftestImage 只选择已启用且已烘焙创世数据的版本;组合预拉取属于具体组合,不阻塞运行时接入自检。
func selectRuntimeSelftestImage(images []RuntimeImage) (RuntimeImage, bool) {
	for _, image := range images {
		if image.Status != RuntimeImageStatusAvailable ||
			!image.GenesisBaked {
			continue
		}
		return image, true
	}
	return RuntimeImage{}, false
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

// runtimeSelftestStoredDetail 描述服务端持久化的自检明细,并发写回字段不能下发给平台界面。
type runtimeSelftestStoredDetail struct {
	Result    string `json:"result,omitempty"`
	Stage     string `json:"stage,omitempty"`
	Reason    string `json:"reason,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	AttemptID string `json:"attempt_id,omitempty"`
}

// runtimeSelftestPublicDetail 严格解析服务端明细后只输出稳定的自检状态、原因和报障编号。
// attempt_id 只用于同一批次的并发写回校验,不得进入 HTTP 响应。
func runtimeSelftestPublicDetail(raw json.RawMessage) (RuntimeSelftestDetail, error) {
	var stored runtimeSelftestStoredDetail
	if err := jsonx.DecodeStrictKnownFields(raw, &stored); err != nil {
		return RuntimeSelftestDetail{}, err
	}
	return RuntimeSelftestDetail{
		Result:  stored.Result,
		Stage:   stored.Stage,
		Reason:  stored.Reason,
		TraceID: stored.TraceID,
	}, nil
}

// runRuntimeCapabilitySelftest 用标准 L2 能力执行 reset/deploy/query/reset 自检闭环。
func (s *Service) runRuntimeCapabilitySelftest(ctx context.Context, plan CreateSandboxPlan) error {
	sb, runtime := plan.Sandbox, workspaceRuntimeForPlan(plan)
	if runtime.AdapterLevel < RuntimeAdapterLevelStandard && strings.TrimSpace(runtime.CapabilityImpl) == "" && strings.TrimSpace(runtime.PluginRef) == "" {
		return nil
	}
	cap, err := s.resolveCapability(runtime)
	if err != nil {
		return err
	}
	if err := s.resetRuntimeForPlan(ctx, plan, cap); err != nil {
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
	return s.resetRuntimeForPlan(ctx, plan, cap)
}

// resetRuntimeForPlan 根据运行时契约选择控制面重建或生态原生 reset 命令。
func (s *Service) resetRuntimeForPlan(ctx context.Context, plan CreateSandboxPlan, cap ChainCapability) error {
	runtime := workspaceRuntimeForPlan(plan)
	if strings.TrimSpace(runtime.AdapterSpec.CapabilityCommands.ResetStrategy) == "recreate_runtime" {
		return s.orchestrator.ResetSandboxRuntime(ctx, plan)
	}
	return cap.Reset(ctx, plan.Sandbox, runtime)
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

// PrepullRuntimeImage 按已发布组合摘要触发闭包预拉取并以真实节点状态更新数据库。
func (s *Service) PrepullRuntimeImage(ctx context.Context, runtimeID, imageID int64, compositionDigest string) (PrepullResponse, error) {
	compositionDigest = strings.TrimSpace(compositionDigest)
	if runtimeID <= 0 || imageID <= 0 || compositionDigest == "" {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullParamInvalid
	}
	attemptID := ids.Format(s.ids.Generate())
	var runtime Runtime
	var image RuntimeImage
	var snapshot contracts.SandboxCompositionSnapshot
	var prepullSpecs []PrepullImageSpec
	var imageURLs []string
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		image, err = tx.GetRuntimeImageByIDForUpdate(ctx, runtimeID, imageID)
		if err != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		runtime, err = tx.GetRuntimeByID(ctx, runtimeID)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		rawSnapshot, err := tx.GetPublishedCompositionSnapshot(ctx, compositionDigest)
		if err != nil {
			return apperr.ErrSandboxImagePrepullParamInvalid.WithCause(err)
		}
		if err := jsonx.DecodeStrictKnownFields(rawSnapshot, &snapshot); err != nil || snapshot.Digest != compositionDigest {
			return apperr.ErrSandboxImagePrepullParamInvalid.WithCause(err)
		}
		canonicalDigest, err := contracts.CanonicalSnapshotDigest(snapshot)
		if err != nil || canonicalDigest != compositionDigest {
			return apperr.ErrSandboxImagePrepullParamInvalid
		}
		matched := false
		for _, frozen := range snapshot.Runtimes {
			if frozen.RuntimeID == runtimeID && frozen.ImageID == imageID && frozen.ImageURL == image.ImageURL && frozen.ImageVersion == image.Version {
				matched = true
				break
			}
		}
		if !matched {
			return apperr.ErrSandboxRuntimeImageNotFound
		}
		if !canPrepullRuntime(runtime.Status) || image.Status != RuntimeImageStatusAvailable {
			return apperr.ErrSandboxRuntimeUnavailable
		}
		prepullSpecs, err = prepullImageSpecsForSnapshot(snapshot)
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
		closure, err := json.Marshal(snapshot.ImageClosure)
		if err != nil {
			return apperr.ErrSandboxImagePrepullFailed.WithCause(err)
		}
		_, err = tx.StartCompositionPrepull(ctx, s.ids.Generate(), imageID, compositionDigest, attemptID, closure, startingDetail)
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
	result, err := s.orchestrator.PrepullImage(operationCtx, image, compositionDigest, prepullSpecs)
	status := ImagePrepullSucceeded
	if err != nil {
		status = ImagePrepullFailed
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
		_, updateErr := tx.FinishCompositionPrepull(ctx, imageID, compositionDigest, attemptID, status, result.DaemonSet, result.DesiredNodes, result.ReadyNodes, detail)
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
	return PrepullResponse{ImageID: ids.ID(imageID), CompositionDigest: compositionDigest, PrepullStatus: status, DesiredNodes: result.DesiredNodes, ReadyNodes: result.ReadyNodes, ImageCount: len(imageURLs)}, nil
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

// prepullImageSpecsForSnapshot 从不可变组合快照重建预拉取闭包,不读取当前工具目录,避免发布后目录变化污染运行环境。
func prepullImageSpecsForSnapshot(snapshot contracts.SandboxCompositionSnapshot) ([]PrepullImageSpec, error) {
	collector := newPrepullSpecCollector(len(snapshot.Runtimes) + len(snapshot.Components))
	for _, frozen := range snapshot.Runtimes {
		var adapter AdapterSpec
		if err := jsonx.DecodeStrictKnownFields(frozen.AdapterSpec, &adapter); err != nil {
			return nil, fmt.Errorf("运行时 %s 适配器快照无效: %w", frozen.InstanceCode, err)
		}
		runtime := Runtime{Code: frozen.Code, Eco: frozen.Eco, AdapterSpec: adapter}
		image := RuntimeImage{ID: frozen.ImageID, RuntimeID: frozen.RuntimeID, ImageURL: frozen.ImageURL, Version: frozen.ImageVersion, Status: RuntimeImageStatusAvailable, GenesisBaked: true}
		runtimeSpecs, err := prepull.RuntimeImageSpecs(image.ImageURL, runtimePrepullDefinition(runtime))
		if err != nil {
			return nil, err
		}
		for _, spec := range runtimeSpecs {
			if err := collector.add(spec.ImageURL, spec.Command, spec.Hold, spec.EphemeralMounts); err != nil {
				return nil, err
			}
		}
	}
	for _, component := range snapshot.Components {
		var resourceSpec ToolResourceSpec
		if err := jsonx.DecodeStrictKnownFields(component.ResourceSpec, &resourceSpec); err != nil {
			return nil, fmt.Errorf("组件 %s 工作负载快照无效: %w", component.Code, err)
		}
		for _, workloadComponent := range resourceSpec.Components {
			command := workloadComponent.PrepullCommand
			if len(command) == 0 {
				command = resourceSpec.PrepullCommand
			}
			if err := collector.add(workloadComponent.ImageURL, command, workloadComponent.PrepullHold, workloadComponent.EphemeralMounts); err != nil {
				return nil, err
			}
		}
	}
	expectedSpecs := make([]PrepullImageSpec, 0, len(snapshot.ImageClosure))
	closureURLs := make(map[string]struct{}, len(snapshot.ImageClosure))
	for _, item := range snapshot.ImageClosure {
		url := strings.TrimSpace(item.ImageURL)
		if url == "" {
			return nil, fmt.Errorf("组合闭包包含空镜像地址")
		}
		if len(compactCommand(item.PrepullCommand)) == 0 {
			return nil, fmt.Errorf("组合闭包镜像 %s 缺少预拉取命令", url)
		}
		closureURLs[url] = struct{}{}
		mounts := make([]workload.EphemeralMountSpec, 0, len(item.EphemeralMounts))
		for _, mount := range item.EphemeralMounts {
			mounts = append(mounts, workload.EphemeralMountSpec{Name: mount.Name, MountPath: mount.MountPath})
		}
		expectedSpecs = append(expectedSpecs, PrepullImageSpec{ImageURL: url, Command: item.PrepullCommand, Hold: item.PrepullHold, EphemeralMounts: mounts})
	}
	if !equivalentPrepullSpecs(collector.specs, expectedSpecs) {
		return nil, fmt.Errorf("组合闭包中的预拉取命令、保持用途或临时挂载与工作负载不一致")
	}
	actualURLs := prepullImageURLs(collector.specs)
	if len(closureURLs) != len(actualURLs) {
		return nil, fmt.Errorf("组合镜像闭包与适配器工作负载不一致")
	}
	for _, url := range actualURLs {
		if _, ok := closureURLs[url]; !ok {
			return nil, fmt.Errorf("组合闭包缺少工作负载镜像 %s", url)
		}
	}
	holdCount := 0
	for _, spec := range collector.specs {
		if spec.Hold {
			holdCount++
		}
	}
	if holdCount != 1 {
		return nil, fmt.Errorf("组合预拉取闭包必须且只能包含一个保持容器,实际为 %d 个", holdCount)
	}
	return collector.specs, nil
}

// runtimePrepullContractChanged 只比较会改变预拉取镜像、自检命令、常驻用途或临时目录的声明。
func runtimePrepullContractChanged(previous, current Runtime) bool {
	changed, err := prepull.RuntimeDefinitionsChanged(runtimePrepullDefinition(previous), runtimePrepullDefinition(current))
	return err != nil || changed
}

// runtimePrepullDefinition 把沙箱运行时投影为共享预拉取比较所需的最小声明。
func runtimePrepullDefinition(runtime Runtime) prepull.RuntimeDefinition {
	return prepull.RuntimeDefinition{
		RuntimeContainerName:  runtime.AdapterSpec.RuntimeContainer.Name,
		RuntimePrepullCommand: runtime.AdapterSpec.RuntimeContainer.PrepullCommand,
		InfraSidecars:         runtime.AdapterSpec.InfraSidecars,
		Pods:                  runtime.AdapterSpec.Pods,
	}
}

// equivalentPrepullSpecs 忽略声明顺序比较闭包;镜像、命令、常驻用途和挂载内容仍必须完全一致。
func equivalentPrepullSpecs(left, right []PrepullImageSpec) bool {
	return prepull.Equivalent(left, right)
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
	var out []workload.EphemeralMountSpec
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

// GetRuntimeImagePrepull 查询指定组合摘要的镜像预拉取状态。
func (s *Service) GetRuntimeImagePrepull(ctx context.Context, runtimeID, imageID int64, compositionDigest string) (PrepullResponse, error) {
	compositionDigest = strings.TrimSpace(compositionDigest)
	if runtimeID <= 0 || imageID <= 0 || compositionDigest == "" {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullParamInvalid
	}
	var prepull CompositionPrepull
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		image, err := tx.GetRuntimeImageByID(ctx, runtimeID, imageID)
		if err != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		prepull, err = tx.GetCompositionPrepullForUpdate(ctx, image.ID, compositionDigest)
		if err != nil {
			return apperr.ErrSandboxImagePrepullFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return PrepullResponse{}, err
	}
	var closure []contracts.ImageClosureItem
	if err := jsonx.DecodeStrictKnownFields(prepull.ImageClosure, &closure); err != nil {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(err)
	}
	return PrepullResponse{ImageID: ids.ID(imageID), CompositionDigest: compositionDigest, PrepullStatus: prepull.Status, DesiredNodes: prepull.DesiredNodes, ReadyNodes: prepull.ReadyNodes, ImageCount: len(closure)}, nil
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

// invalidateToolAffectedRuntimePrepull 工具执行契约变化会影响所有引用它的已发布组合,统一撤销各运行时的组合证明。
func invalidateToolAffectedRuntimePrepull(ctx context.Context, tx TxStore, previous Tool, hasPrevious bool, current Tool) error {
	if !toolHasPrepullWorkload(previous) && !toolHasPrepullWorkload(current) {
		return nil
	}
	if hasPrevious && previous.Kind == current.Kind && previous.Status == current.Status && reflect.DeepEqual(previous.ResourceSpec, current.ResourceSpec) {
		return nil
	}
	runtimes, err := tx.ListRuntimes(ctx)
	if err != nil {
		return err
	}
	for _, runtime := range runtimes {
		if err := invalidateRuntimePrepull(ctx, tx, runtime.ID, "tool_execution_contract_changed", current.Code); err != nil {
			return err
		}
	}
	return nil
}

// toolHasPrepullWorkload 判断工具是否真正贡献镜像闭包;纯平台内建能力变化不应撤销镜像证明。
func toolHasPrepullWorkload(tool Tool) bool {
	for _, component := range tool.ResourceSpec.Components {
		if strings.TrimSpace(component.ImageURL) != "" {
			return true
		}
	}
	return false
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
	return tx.InvalidateCompositionPrepullByRuntime(ctx, runtimeID, detail)
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
	var tools []CatalogTool
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		runtimes, err = tx.ListCatalogRuntimes(ctx)
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
