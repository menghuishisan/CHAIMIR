package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/prepull"
	"chaimir/internal/platform/response"
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
		previous, err := tx.GetOrCreateRuntimeForUpdate(ctx, s.ids.Generate(), req, spec)
		if err != nil {
			return apperr.ErrSandboxRuntimePersistFailed.WithCause(err)
		}
		selftestStatus, selftestDetail, executionChanged := runtimeStateForUpdate(previous, &req, spec)
		runtime, err = tx.UpdateRuntime(ctx, previous.ID, req, spec, selftestStatus, selftestDetail)
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

func runtimeStateForUpdate(previous Runtime, req *RuntimeRequest, spec AdapterSpec) (int16, []byte, bool) {
	executionChanged := runtimeExecutionContractChanged(previous, *req, spec)
	if req.Status == 0 {
		req.Status = previous.Status
	}
	if !executionChanged {
		return previous.SelftestStatus, append([]byte(nil), previous.SelftestDetail...), false
	}
	if req.Status != RuntimeStatusDisabled {
		req.Status = RuntimeStatusOnboarding
	}
	return RuntimeSelftestPending, []byte(`{"result":"pending","reason":"runtime_execution_contract_changed"}`), true
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
		selftestStatus, selftestDetail, executionChanged := runtimeStateForUpdate(existing, &req, spec)
		runtime, err = tx.UpdateRuntime(ctx, runtimeID, req, spec, selftestStatus, selftestDetail)
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

// ListRuntimes 查询平台已登记运行时列表,并在同一事务里带回每个运行时的可用镜像版本数(列表卡片的接入进度用)。
func (s *Service) ListRuntimes(ctx context.Context) ([]Runtime, map[int64]int64, error) {
	var out []Runtime
	var availableImages map[int64]int64
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListRuntimes(ctx)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		availableImages, err = tx.CountAvailableRuntimeImages(ctx)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return out, availableImages, nil
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
		if _, err := tx.GetRuntimeByID(ctx, runtimeID); err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
			}
			return apperr.ErrSandboxImagePersistFailed.WithCause(err)
		}
		var err error
		image, err = tx.CreateRuntimeImage(ctx, s.ids.Generate(), runtimeID, req)
		if err != nil {
			// 唯一约束统一裁决并发重复,不在写入前维护另一套查重逻辑。
			if db.IsUniqueViolation(err) {
				return apperr.ErrSandboxImageConflict.WithCause(err)
			}
			return apperr.ErrSandboxImagePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return RuntimeImage{}, err
	}
	return image, s.writeAuditFromContext(ctx, 0, "sandbox.image.register", "runtime_image", image.ID, map[string]any{"image_url": image.ImageURL})
}

// DisableRuntimeImage 先撤销镜像准入和预拉取证明，再清理资源；清理失败可重试。
func (s *Service) DisableRuntimeImage(ctx context.Context, runtimeID, imageID int64) (RuntimeImage, error) {
	if runtimeID <= 0 || imageID <= 0 {
		return RuntimeImage{}, apperr.ErrSandboxImageDisableParamInvalid
	}
	var image RuntimeImage
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		image, err = tx.GetRuntimeImageByIDForUpdate(ctx, runtimeID, imageID)
		if err != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		image, err = tx.DisableRuntimeImage(ctx, runtimeID, imageID)
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
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	cleanupErr := s.orchestrator.DeletePrepullDaemonSet(cleanupCtx, image)
	auditErr := s.writeAuditFromContext(cleanupCtx, 0, "sandbox.image.disable", "runtime_image", imageID, map[string]any{"runtime_id": runtimeID})
	if err := errors.Join(cleanupErr, auditErr); err != nil {
		return RuntimeImage{}, apperr.ErrSandboxImageDisableFailed.WithCause(err)
	}
	return image, nil
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
	if s.cfg.SelftestTimeoutSeconds <= 0 || s.cfg.SelftestRecycleTimeoutSeconds <= 0 || s.cfg.ReadyTimeoutSeconds <= 0 {
		return RuntimeSelftestResponse{}, apperr.ErrSandboxSelftestRecycleConfigInvalid
	}
	attemptID := ids.Format(s.ids.Generate())
	startingDetail, err := jsonBytes(runtimeSelftestStoredDetail{Result: "running", Stage: "selftest", AttemptID: attemptID})
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
		if !canPrepareRuntime(runtime.Status) {
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
		image, txErr = tx.GetRuntimeImageByIDForUpdate(ctx, runtimeID, image.ID)
		if txErr != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(txErr)
		}
		if image.Status != RuntimeImageStatusAvailable {
			return apperr.ErrSandboxRuntimeUnavailable
		}
		if err := validatePlanImagesCurrentlyAdmitted(s.cfg, runtime, image, nil, nil); err != nil {
			return err
		}
		_, txErr = tx.StartRuntimeSelftest(ctx, runtimeID, startingDetail)
		if txErr != nil {
			return apperr.ErrSandboxSelftestFailed.WithCause(txErr)
		}
		return nil
	}); err != nil {
		return RuntimeSelftestResponse{}, err
	}
	instance := boundedSandboxName(runtime.Code, 63)
	runtime.AdapterSpec = MaterializeRuntimeTopology(instance, runtime.AdapterSpec)
	selftestID := s.ids.Generate()
	sb := Sandbox{
		ID:             selftestID,
		TenantID:       0,
		Namespace:      namespaceFor("sbx-selftest", selftestID),
		Phase:          SandboxPhaseAllocating,
		Status:         SandboxStatusCreating,
		OwnerAccountID: 0,
	}
	plan := CreateSandboxPlan{Sandbox: sb, WorkspaceRuntimeInstance: instance, Runtimes: []RuntimePlan{{InstanceCode: instance, Runtime: runtime, Image: image}}}
	ctx = logging.WithAttrs(ctx, slog.Int64("runtime_id", runtimeID), slog.Int64("sandbox_id", sb.ID), slog.String("namespace", sb.Namespace), slog.String("attempt_id", attemptID))
	operationCtx, operationCancel := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.SelftestTimeoutSeconds))
	defer operationCancel()
	failureStage := "prepare"
	// 所有执行阶段共用执行预算,每步进入前复核;清理和写回另行计时。
	runStage := func(stage, target string, index int, run func() error) error {
		started := time.Now()
		stageErr := operationCtx.Err()
		if stageErr == nil {
			stageErr = run()
		}
		if stageErr == nil {
			stageErr = operationCtx.Err()
		}
		logPlatformSelftestStage(operationCtx, stage, started, stageErr, slog.String("exec_target", target), slog.Int("command_index", index))
		if stageErr != nil {
			failureStage = stage
			return fmt.Errorf("运行时自检%s阶段失败: %w", stage, stageErr)
		}
		return nil
	}
	err = runStage("prepare", "", 0, func() error {
		archives, err := s.prepareRuntimeInitArchives(operationCtx, sb, plan.Runtimes)
		if err != nil {
			return err
		}
		return s.orchestrator.CreateSandboxResources(operationCtx, plan, archives)
	})
	if err == nil {
		// 私有执行目标必须真实支持清理命令,不能用旧工作区镜像的普通自检代替。
		err = runStage("workspace", workspaceExecTarget(runtime), 0, func() error { return s.cleanWorkspaceBeforeAccess(operationCtx, sb, runtime) })
	}
	commands := runtimeSelftestCommands(runtime)
	if err == nil && len(commands) == 0 {
		err = runStage("toolchain", "", 0, func() error { return apperr.ErrSandboxSelftestFailed })
	}
	// 先在运行时主容器执行 manifest 声明的工具链自检,再执行工作区和原生能力自检。
	// PrepullCommand 是目录把 manifest selftest.commands 固化后的唯一命令来源,
	// 不能只把它用于预拉取而在平台自检中遗漏,否则“自检通过”不代表用户环境可用。
	for index, command := range commands {
		if err != nil {
			break
		}
		err = runStage("toolchain", command.target, index+1, func() error {
			_, _, execErr := s.orchestrator.Exec(operationCtx, sb.Namespace, command.target, command.argv, nil, false)
			return execErr
		})
	}
	if err == nil {
		err = runStage("capability", runtimeExecTarget(runtime), 0, func() error { return s.runRuntimeCapabilitySelftest(operationCtx, plan) })
	}
	operationCancel()
	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), timeDurationSeconds(s.cfg.SelftestRecycleTimeoutSeconds))
	defer cleanupCancel()
	cleanupStarted := time.Now()
	cleanupErr := s.orchestrator.DestroySandboxResources(cleanupCtx, sb)
	if cleanupErr == nil {
		cleanupErr = cleanupCtx.Err()
	}
	logPlatformSelftestStage(cleanupCtx, "cleanup", cleanupStarted, cleanupErr)
	if cleanupErr != nil {
		// 资源回收是自检完成条件,不能只记日志后把未清理的尝试标为通过。
		logging.ErrorContext(cleanupCtx, "sandbox selftest cleanup failed", apperr.ErrSandboxSelftestFailed.WithCause(cleanupErr).LogString(), slog.Int64("tenant_id", 0), slog.Int64("runtime_id", runtimeID), slog.Int64("sandbox_id", sb.ID), slog.String("namespace", sb.Namespace))
		if err == nil {
			failureStage = "recycle"
		}
		err = errors.Join(err, fmt.Errorf("回收运行时自检资源失败: %w", cleanupErr))
	}
	status, runtimeStatus := RuntimeSelftestPassed, RuntimeStatusAvailable
	persistCtx, persistCancel := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer persistCancel()
	var updated Runtime
	persistStarted := time.Now()
	updateErr := s.store.PlatformTx(persistCtx, func(ctx context.Context, tx TxStore) error {
		// 与启动和运行时更新保持同一锁序，镜像停用不能与通过结论交错提交。
		if _, lockErr := tx.GetRuntimeByIDForUpdate(ctx, runtimeID); lockErr != nil {
			return lockErr
		}
		currentImage, lockErr := tx.GetRuntimeImageByIDForUpdate(ctx, runtimeID, image.ID)
		if lockErr != nil {
			return lockErr
		}
		if currentImage.Status != RuntimeImageStatusAvailable {
			if err == nil {
				failureStage = "prepare"
			}
			err = errors.Join(err, apperr.ErrSandboxRuntimeUnavailable)
		}
		detail := runtimeSelftestStoredDetail{Result: "passed", AttemptID: attemptID}
		if err != nil {
			status, runtimeStatus = RuntimeSelftestFailed, RuntimeStatusOnboarding
			detail.Result, detail.Stage, detail.TraceID = "failed", failureStage, response.TraceFromContext(ctx)
			logging.ErrorContext(ctx, "sandbox runtime selftest failed", apperr.ErrSandboxSelftestFailed.WithCause(err).LogString(), slog.Int64("tenant_id", 0))
		}
		rawDetail, encodeErr := jsonBytes(detail)
		if encodeErr != nil {
			return encodeErr
		}
		updated, lockErr = tx.FinishRuntimeSelftest(ctx, runtimeID, attemptID, status, runtimeStatus, rawDetail)
		return lockErr
	})
	logPlatformSelftestStage(persistCtx, "persist", persistStarted, updateErr)
	if updateErr != nil {
		return RuntimeSelftestResponse{}, apperr.ErrSandboxSelftestFailed.WithCause(errors.Join(err, updateErr))
	}
	if auditErr := s.writeAuditFromContext(persistCtx, 0, "sandbox.runtime.selftest", "runtime", runtimeID, map[string]any{"status": status, "attempt_id": attemptID}); auditErr != nil {
		return RuntimeSelftestResponse{}, auditErr
	}
	if publishErr := s.bus.Publish(persistCtx, contracts.SubjectSandboxRuntimeSelftestCompleted, contracts.SandboxRuntimeSelftestCompletedEvent{
		TenantID:       0,
		TraceID:        response.TraceFromContext(persistCtx),
		RuntimeID:      runtimeID,
		RuntimeCode:    runtime.Code,
		SelftestStatus: updated.SelftestStatus,
		RuntimeStatus:  updated.Status,
		FinishedAt:     time.Now(),
	}); publishErr != nil {
		return RuntimeSelftestResponse{}, apperr.ErrSandboxSelftestFailed.WithCause(publishErr)
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

// runtimeSelftestCommands 返回运行时目录声明的完整接入自检顺序。
// 主容器工具链、工作区入口和原生能力分别覆盖不同边界,不能用其中一项代替另一项。
func runtimeSelftestCommands(runtime Runtime) []struct {
	target string
	argv   []string
} {
	if len(runtime.AdapterSpec.RuntimeContainer.PrepullCommand) == 0 {
		return nil
	}
	return []struct {
		target string
		argv   []string
	}{
		{target: runtimeExecTarget(runtime), argv: append([]string(nil), runtime.AdapterSpec.RuntimeContainer.PrepullCommand...)},
		{target: workspaceExecTarget(runtime), argv: append([]string(nil), runtime.AdapterSpec.WorkspaceOps.Selftest...)},
	}
}

// 创世状态由各运行时的启动契约处理，不以 genesis_baked 作为通用自检门禁。
func selectRuntimeSelftestImage(images []RuntimeImage) (RuntimeImage, bool) {
	for _, image := range images {
		if image.Status != RuntimeImageStatusAvailable {
			continue
		}
		return image, true
	}
	return RuntimeImage{}, false
}

// canPrepareRuntime 只允许接入中或已可用运行时准备镜像、执行接入自检。
func canPrepareRuntime(status int16) bool {
	return status == RuntimeStatusOnboarding || status == RuntimeStatusAvailable
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

// runRuntimeCapabilitySelftest 验证原生部署、可选交易、查询及重置后的能力恢复。
func (s *Service) runRuntimeCapabilitySelftest(ctx context.Context, plan CreateSandboxPlan) error {
	sb, runtime := plan.Sandbox, workspaceRuntimeForPlan(plan)
	if runtime.AdapterLevel < RuntimeAdapterLevelStandard && strings.TrimSpace(runtime.CapabilityImpl) == "" && strings.TrimSpace(runtime.PluginRef) == "" {
		return nil
	}
	cap, err := s.resolveCapability(runtime)
	if err != nil {
		return err
	}
	target, ok := runtime.AdapterSpec.Selftest["query_target"].(string)
	if !ok || strings.TrimSpace(target) == "" {
		return apperr.ErrSandboxSelftestSpecInvalid
	}
	// Kubernetes Ready 只表示容器探针通过，不能替代运行时声明的原生能力。
	// 先以无副作用的 query_target 确认节点可服务，再执行会改变链状态的 deploy/tx。
	if err := s.waitRuntimeCapabilityReady(ctx, cap, sb, runtime, strings.TrimSpace(target)); err != nil {
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
	if _, err := cap.Query(ctx, sb, runtime, strings.TrimSpace(target)); err != nil {
		return err
	}
	return s.resetRuntimeForPlan(ctx, plan, plan.WorkspaceRuntimeInstance, cap)
}

// waitRuntimeCapabilityReady 在既有 Pod 就绪窗口内等待声明的只读查询能力，
// 避免容器已启动但节点仍在导入账户、账本或 RPC 初始化时提前执行 deploy。
func (s *Service) waitRuntimeCapabilityReady(ctx context.Context, cap ChainCapability, sb Sandbox, runtime Runtime, target string) error {
	if s.cfg.ReadyTimeoutSeconds <= 0 || s.cfg.ReadyPollIntervalSeconds <= 0 || s.cfg.ChainRPCTimeoutSeconds <= 0 {
		return apperr.ErrSandboxSelftestRecycleConfigInvalid
	}
	probeRuntime := runtime
	query := &probeRuntime.AdapterSpec.CapabilityCommands.Query
	chainTimeout := int32(s.cfg.ChainRPCTimeoutSeconds)
	if query.TimeoutSeconds <= 0 || query.TimeoutSeconds > chainTimeout {
		query.TimeoutSeconds = chainTimeout
	}
	readyCtx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	ticker := time.NewTicker(sandboxPodReadyPollInterval(s.cfg.ReadyPollIntervalSeconds))
	defer ticker.Stop()
	var lastErr error
	for {
		if _, err := cap.Query(readyCtx, sb, probeRuntime, target); err == nil {
			return nil
		} else if !errors.Is(err, apperr.ErrSandboxChainFailed) && !errors.Is(err, context.DeadlineExceeded) {
			return err
		} else {
			lastErr = err
		}
		select {
		case <-readyCtx.Done():
			return fmt.Errorf("运行时声明查询能力未在 %s 内就绪: %w", timeDurationSeconds(s.cfg.ReadyTimeoutSeconds), lastErr)
		case <-ticker.C:
		}
	}
}

func (s *Service) resetRuntimeForPlan(ctx context.Context, plan CreateSandboxPlan, instance string, cap ChainCapability) error {
	selected, ok := plan.RuntimeInstance(instance)
	if !ok {
		return apperr.ErrSandboxContractRequestInvalid
	}
	runtime := selected.Runtime
	target, ok := runtime.AdapterSpec.Selftest["query_target"].(string)
	if !ok || strings.TrimSpace(target) == "" {
		return apperr.ErrSandboxSelftestSpecInvalid
	}
	if s.cfg.ReadyTimeoutSeconds <= 0 || s.cfg.ReadyPollIntervalSeconds <= 0 || s.cfg.ChainRPCTimeoutSeconds <= 0 {
		return apperr.ErrSandboxSelftestRecycleConfigInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(runtime.AdapterSpec.CapabilityCommands.ResetStrategy) == "recreate_runtime" {
		archives, err := s.prepareRuntimeInitArchives(ctx, plan.Sandbox, []RuntimePlan{selected})
		if err != nil {
			return err
		}
		if err := s.orchestrator.ResetSandboxRuntime(ctx, plan, selected.InstanceCode, archives); err != nil {
			return err
		}
	} else if err := cap.Reset(ctx, plan.Sandbox, runtime); err != nil {
		return err
	}
	return s.waitRuntimeCapabilityReady(ctx, cap, plan.Sandbox, runtime, strings.TrimSpace(target))
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

// PrepullRuntimeImage 按冻结组合摘要触发闭包预拉取并以真实节点状态更新数据库。
func (s *Service) PrepullRuntimeImage(ctx context.Context, runtimeID, imageID int64, compositionDigest string) (PrepullResponse, error) {
	return s.prepullRuntimeImage(ctx, runtimeID, imageID, compositionDigest, false)
}

// prepullRuntimeImage 执行运行时镜像闭包预拉取。
// 平台管理员直接操作保留请求身份审计;判题目录由运行时事件触发时使用系统任务审计,
// 避免内部事件错误地依赖不存在的 HTTP 用户上下文。
func (s *Service) prepullRuntimeImage(ctx context.Context, runtimeID, imageID int64, compositionDigest string, systemAudit bool) (PrepullResponse, error) {
	compositionDigest = strings.TrimSpace(compositionDigest)
	if runtimeID <= 0 || imageID <= 0 || compositionDigest == "" {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullParamInvalid
	}
	attemptID := ids.Format(s.ids.Generate())
	var image RuntimeImage
	var snapshot contracts.SandboxCompositionSnapshot
	var prepullSpecs []PrepullImageSpec
	var imageURLs []string
	var attempt CompositionPrepull
	joined := false
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		image, err = tx.GetRuntimeImageByIDForUpdate(ctx, runtimeID, imageID)
		if err != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		rawSnapshot, err := tx.GetCompositionSnapshot(ctx, compositionDigest)
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
		if _, err := s.loadSnapshotPlan(ctx, tx, snapshot, false); err != nil {
			return err
		}
		matched := false
		for _, frozen := range snapshot.Runtimes {
			if frozen.RuntimeID == runtimeID && frozen.ImageID == imageID {
				matched = true
				break
			}
		}
		if !matched {
			return apperr.ErrSandboxRuntimeImageNotFound
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
		// 镜像行锁使首次插入也串行；仅复用仍在原始总预算内的同摘要尝试。
		existing, err := tx.GetCompositionPrepullForUpdate(ctx, imageID, compositionDigest)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrSandboxImagePrepullFailed.WithCause(err)
		}
		budget := timeDurationSeconds(s.cfg.PrepullTimeoutSeconds) + timeDurationSeconds(s.cfg.ReadyTimeoutSeconds)
		if err == nil && existing.Status == ImagePrepullRunning && existing.AttemptID != "" && !existing.StartedAt.IsZero() && time.Now().Before(existing.StartedAt.Add(budget)) {
			attempt, joined = existing, true
			return nil
		}
		startingDetail, err := prepullAttemptDetail("starting", attemptID, PrepullResult{}, imageURLs, nil)
		if err != nil {
			return apperr.ErrSandboxImagePrepullFailed.WithCause(err)
		}
		closure, err := json.Marshal(snapshot.ImageClosure)
		if err != nil {
			return apperr.ErrSandboxImagePrepullFailed.WithCause(err)
		}
		attempt, err = tx.StartCompositionPrepull(ctx, s.ids.Generate(), imageID, compositionDigest, attemptID, closure, startingDetail)
		return err
	}); err != nil {
		return PrepullResponse{}, err
	}
	if joined {
		result, err := s.waitCompositionPrepull(ctx, attempt)
		if err != nil {
			return PrepullResponse{}, err
		}
		attempt.Status = result.PrepullStatus
		if err := s.auditCompositionPrepull(ctx, runtimeID, attempt, systemAudit, true); err != nil {
			return PrepullResponse{}, err
		}
		return result, nil
	}
	// 一旦 running 状态提交,即使管理员关闭页面也必须继续等待 K8s 并写回最终结果,避免永久卡在进行中。
	operationCtx, cancel := context.WithDeadline(context.WithoutCancel(ctx), attempt.StartedAt.Add(timeDurationSeconds(s.cfg.PrepullTimeoutSeconds)))
	defer cancel()
	result, err := s.orchestrator.PrepullImage(operationCtx, image, compositionDigest, prepullSpecs)
	status := ImagePrepullSucceeded
	if err != nil {
		status = ImagePrepullFailed
		logging.ErrorContext(operationCtx, "sandbox image prepull failed", logging.SanitizeError(err.Error()), slog.Int64("tenant_id", 0), slog.Int64("runtime_id", runtimeID), slog.Int64("image_id", imageID), slog.String("daemonset", result.DaemonSet), slog.String("attempt_id", attemptID))
	}
	// 执行超时也必须能回写失败；持久化仍受该尝试的原始总截止时间约束。
	persistCtx, persistCancel := context.WithDeadline(context.WithoutCancel(ctx), attempt.StartedAt.Add(timeDurationSeconds(s.cfg.PrepullTimeoutSeconds)+timeDurationSeconds(s.cfg.ReadyTimeoutSeconds)))
	defer persistCancel()
	imageDisabled := false
	if updateErr := s.store.PlatformTx(persistCtx, func(ctx context.Context, tx TxStore) error {
		current, readErr := tx.GetRuntimeImageByIDForUpdate(ctx, runtimeID, imageID)
		if readErr != nil {
			return readErr
		}
		imageDisabled = current.Status == RuntimeImageStatusDisabled
		if imageDisabled {
			return apperr.ErrSandboxRuntimeUnavailable
		}
		if _, admissionErr := s.loadSnapshotPlan(ctx, tx, snapshot, false); admissionErr != nil {
			err = errors.Join(err, admissionErr)
			status = ImagePrepullFailed
			logging.ErrorContext(ctx, "sandbox composition admission revoked during prepull", apperr.ErrSandboxImagePrepullFailed.WithCause(admissionErr).LogString(), slog.Int64("runtime_id", runtimeID), slog.Int64("image_id", imageID), slog.String("composition_digest", compositionDigest))
		}
		stage := "succeeded"
		if err != nil {
			stage = "failed"
		}
		detail, encodeErr := prepullAttemptDetail(stage, attemptID, result, imageURLs, err)
		if encodeErr != nil {
			return encodeErr
		}
		var updateErr error
		attempt, updateErr = tx.FinishCompositionPrepull(ctx, imageID, compositionDigest, attemptID, status, result.DaemonSet, result.DesiredNodes, result.ReadyNodes, detail)
		return updateErr
	}); updateErr != nil {
		var cleanupErr error
		if imageDisabled {
			// 停用禁止新尝试；旧执行者可能在停用清理后才完成 K8s 创建。
			cleanupErr = s.orchestrator.DeletePrepullDaemonSet(persistCtx, image)
		}
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(errors.Join(err, updateErr, cleanupErr))
	}
	if auditErr := s.auditCompositionPrepull(persistCtx, runtimeID, attempt, systemAudit, false); auditErr != nil {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(errors.Join(err, auditErr))
	}
	if err != nil {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(err)
	}
	return PrepullResponse{ImageID: ids.ID(imageID), CompositionDigest: compositionDigest, PrepullStatus: status, DesiredNodes: result.DesiredNodes, ReadyNodes: result.ReadyNodes, ImageCount: len(imageURLs)}, nil
}

// waitCompositionPrepull 等待已认领尝试的真实终态，不持有跨轮询事务或重启原执行者。
func (s *Service) waitCompositionPrepull(ctx context.Context, attempt CompositionPrepull) (PrepullResponse, error) {
	deadline := attempt.StartedAt.Add(timeDurationSeconds(s.cfg.PrepullTimeoutSeconds) + timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	waitCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	ticker := time.NewTicker(timeDurationSeconds(s.cfg.PrepullPollIntervalSeconds))
	defer ticker.Stop()
	for {
		var current CompositionPrepull
		if err := s.store.PlatformTx(waitCtx, func(ctx context.Context, tx TxStore) error {
			var err error
			current, err = tx.GetCompositionPrepullForUpdate(ctx, attempt.RuntimeImageID, attempt.CompositionDigest)
			return err
		}); err != nil {
			return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(err)
		}
		if current.AttemptID != attempt.AttemptID {
			return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(fmt.Errorf("预拉取尝试已失效: attempt_id=%s current_attempt_id=%s", attempt.AttemptID, current.AttemptID))
		}
		switch current.Status {
		case ImagePrepullSucceeded:
			var closure []contracts.ImageClosureItem
			if err := jsonx.DecodeStrictKnownFields(current.ImageClosure, &closure); err != nil {
				return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(err)
			}
			return PrepullResponse{ImageID: ids.ID(current.RuntimeImageID), CompositionDigest: current.CompositionDigest, PrepullStatus: current.Status, DesiredNodes: current.DesiredNodes, ReadyNodes: current.ReadyNodes, ImageCount: len(closure)}, nil
		case ImagePrepullRunning:
			// 在同一截止时间内继续等待，不把加入进行中视为已成功。
		default:
			return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(fmt.Errorf("预拉取尝试未成功: attempt_id=%s status=%d detail=%s", current.AttemptID, current.Status, current.Detail))
		}
		select {
		case <-waitCtx.Done():
			return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(waitCtx.Err())
		case <-ticker.C:
		}
	}
}

// auditCompositionPrepull 统一记录用户或系统触发的预拉取结果和复用关系。
func (s *Service) auditCompositionPrepull(ctx context.Context, runtimeID int64, attempt CompositionPrepull, systemAudit, joined bool) error {
	detail := map[string]any{"runtime_id": runtimeID, "composition_digest": attempt.CompositionDigest, "status": attempt.Status, "attempt_id": attempt.AttemptID, "joined": joined}
	if systemAudit {
		return s.writeSystemAudit(ctx, 0, "sandbox.image.prepull", "runtime_image", attempt.RuntimeImageID, detail)
	}
	return s.writeAuditFromContext(ctx, 0, "sandbox.image.prepull", "runtime_image", attempt.RuntimeImageID, detail)
}

// PrepareCompositions 按冻结摘要准备完整镜像闭包,重复摘要只执行一次。
func (s *Service) PrepareCompositions(ctx context.Context, compositionDigests []string) error {
	digests, err := uniqueCompositionDigests(compositionDigests)
	if err != nil {
		return apperr.ErrSandboxImagePrepullParamInvalid.WithCause(err)
	}
	for _, digest := range digests {
		if err := s.prepullComposition(ctx, digest); err != nil {
			return err
		}
	}
	return nil
}

// prepullComposition 只从冻结快照重建闭包,不读取当前工具目录。
func (s *Service) prepullComposition(ctx context.Context, compositionDigest string) error {
	compositionDigest = strings.TrimSpace(compositionDigest)
	if compositionDigest == "" {
		return apperr.ErrSandboxImagePrepullParamInvalid
	}
	var snapshot contracts.SandboxCompositionSnapshot
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		raw, err := tx.GetCompositionSnapshot(ctx, compositionDigest)
		if err != nil {
			return apperr.ErrSandboxImagePrepullParamInvalid.WithCause(err)
		}
		if err := jsonx.DecodeStrictKnownFields(raw, &snapshot); err != nil {
			return apperr.ErrSandboxImagePrepullParamInvalid.WithCause(err)
		}
		canonical, err := contracts.CanonicalSnapshotDigest(snapshot)
		if err != nil || canonical != compositionDigest || snapshot.Digest != compositionDigest {
			return apperr.ErrSandboxImagePrepullParamInvalid
		}
		return nil
	}); err != nil {
		return err
	}
	seen := make(map[[2]int64]struct{}, len(snapshot.Runtimes))
	for _, runtime := range snapshot.Runtimes {
		key := [2]int64{runtime.RuntimeID, runtime.ImageID}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if _, err := s.prepullRuntimeImage(ctx, runtime.RuntimeID, runtime.ImageID, compositionDigest, true); err != nil {
			return err
		}
	}
	if len(seen) == 0 {
		return apperr.ErrSandboxImagePrepullParamInvalid
	}
	return nil
}

// uniqueCompositionDigests trims and deduplicates a batch while preserving its first-seen order.
func uniqueCompositionDigests(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		digest := strings.TrimSpace(value)
		if digest == "" {
			return nil, fmt.Errorf("组合摘要不能为空")
		}
		if _, exists := seen[digest]; exists {
			continue
		}
		seen[digest] = struct{}{}
		unique = append(unique, digest)
	}
	return unique, nil
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

// prepullImageSpecsForSnapshot 从不可变组合快照重建预拉取闭包,不读取当前工具目录,避免发布后目录变化污染运行环境。
func prepullImageSpecsForSnapshot(snapshot contracts.SandboxCompositionSnapshot) ([]PrepullImageSpec, error) {
	specs := make([]PrepullImageSpec, 0, len(snapshot.Runtimes)+len(snapshot.Components)+len(snapshot.PrivateSidecars))
	for _, frozen := range snapshot.Runtimes {
		var adapter AdapterSpec
		if err := jsonx.DecodeStrictKnownFields(frozen.AdapterSpec, &adapter); err != nil {
			return nil, fmt.Errorf("运行时 %s 适配器快照无效: %w", frozen.InstanceCode, err)
		}
		runtime := Runtime{Code: frozen.Code, Eco: frozen.Eco, AdapterSpec: adapter}
		runtimeSpecs, err := prepull.RuntimeImageSpecs(frozen.ImageURL, runtimePrepullDefinition(runtime))
		if err != nil {
			return nil, err
		}
		specs = append(specs, runtimeSpecs...)
	}
	for _, component := range snapshot.Components {
		var resourceSpec ToolResourceSpec
		if err := jsonx.DecodeStrictKnownFields(component.ResourceSpec, &resourceSpec); err != nil {
			return nil, fmt.Errorf("组件 %s 工作负载快照无效: %w", component.Code, err)
		}
		for _, workloadComponent := range resourceSpec.Components {
			specs = append(specs, PrepullImageSpec{ImageURL: workloadComponent.ImageURL, Command: workloadComponent.PrepullCommand, Hold: workloadComponent.PrepullHold, EphemeralMounts: workloadComponent.EphemeralMounts})
		}
	}
	for _, sidecar := range snapshot.PrivateSidecars {
		specs = append(specs, PrepullImageSpec{ImageURL: sidecar.ImageURL, Command: sidecar.PrepullCommand, Hold: sidecar.PrepullHold, EphemeralMounts: privateSidecarEphemeralMounts(sidecar.EphemeralMounts)})
	}
	specs, err := prepull.Collect(specs)
	if err != nil {
		return nil, err
	}
	expectedSpecs := make([]PrepullImageSpec, 0, len(snapshot.ImageClosure))
	for _, item := range snapshot.ImageClosure {
		url := strings.TrimSpace(item.ImageURL)
		if url == "" {
			return nil, fmt.Errorf("组合闭包包含空镜像地址")
		}
		mounts := make([]workload.EphemeralMountSpec, 0, len(item.EphemeralMounts))
		for _, mount := range item.EphemeralMounts {
			mounts = append(mounts, workload.EphemeralMountSpec{Name: mount.Name, MountPath: mount.MountPath})
		}
		expectedSpecs = append(expectedSpecs, PrepullImageSpec{ImageURL: url, Command: item.PrepullCommand, Hold: item.PrepullHold, EphemeralMounts: mounts})
	}
	expectedSpecs, err = prepull.Collect(expectedSpecs)
	if err != nil {
		return nil, err
	}
	if !prepull.Equivalent(specs, expectedSpecs) {
		return nil, fmt.Errorf("组合闭包中的预拉取命令、保持用途或临时挂载与工作负载不一致")
	}
	holdCount := 0
	for _, spec := range specs {
		if spec.Hold {
			holdCount++
		}
	}
	if holdCount != 1 {
		return nil, fmt.Errorf("组合预拉取闭包必须且只能包含一个保持容器,实际为 %d 个", holdCount)
	}
	return specs, nil
}

// privateSidecarEphemeralMounts 将跨模块判题 sidecar 的临时目录投影为预拉取声明。
func privateSidecarEphemeralMounts(items []contracts.SandboxEphemeralMountSpec) []workload.EphemeralMountSpec {
	out := make([]workload.EphemeralMountSpec, 0, len(items))
	for _, item := range items {
		out = append(out, workload.EphemeralMountSpec{Name: item.Name, MountPath: item.MountPath})
	}
	return out
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
		previous, err := tx.GetOrCreateToolForUpdate(ctx, s.ids.Generate(), req, spec)
		if err != nil {
			return apperr.ErrSandboxToolPersistFailed.WithCause(err)
		}
		tool, err = tx.UpdateTool(ctx, previous.ID, req, spec)
		if err != nil {
			return apperr.ErrSandboxToolPersistFailed.WithCause(err)
		}
		if err := invalidateToolAffectedRuntimePrepull(ctx, tx, previous, tool); err != nil {
			return apperr.ErrSandboxToolPersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return Tool{}, err
	}
	return tool, s.writeAuditFromContext(ctx, 0, "sandbox.tool.upsert", "tool", tool.ID, map[string]any{"code": tool.Code})
}

// invalidateToolAffectedRuntimePrepull 工具执行契约变化会影响所有引用它的冻结组合,统一撤销各运行时的组合证明。
func invalidateToolAffectedRuntimePrepull(ctx context.Context, tx TxStore, previous, current Tool) error {
	if !toolHasPrepullWorkload(previous) && !toolHasPrepullWorkload(current) {
		return nil
	}
	if previous.Kind == current.Kind && previous.Status == current.Status && reflect.DeepEqual(previous.ResourceSpec, current.ResourceSpec) {
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

// invalidateRuntimePrepull 撤销运行时自身镜像及引用它的组合证明，保留失效原因。
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
	if req == nil || runtimeCapabilityKey(req.AdapterLevel, req.CapabilityImpl, req.PluginRef) != "" {
		return
	}
	if hasCapabilityCommands(spec.CapabilityCommands) {
		req.CapabilityImpl = BuiltinExecCapability
	}
}
