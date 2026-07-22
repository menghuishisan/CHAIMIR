// sandbox service_runtime 文件实现运行时、镜像、工具和配额管理编排。
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// RegisterRuntime 注册或更新运行时声明式适配器清单。
func (s *Service) RegisterRuntime(ctx context.Context, req RuntimeRequest) (Runtime, error) {
	spec, err := validateRuntimeRequest(req, s.cfg)
	if err != nil {
		return Runtime{}, err
	}
	applyBuiltinCapabilityDefault(&req, spec)
	if req.Status == 0 {
		req.Status = RuntimeStatusOnboarding
	}
	var runtime Runtime
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		runtime, err = tx.UpsertRuntime(ctx, s.ids.Generate(), req, spec)
		if err != nil {
			return apperr.ErrSandboxRuntimePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return Runtime{}, err
	}
	return runtime, s.writeAuditFromContext(ctx, 0, "sandbox.runtime.upsert", "runtime", runtime.ID, map[string]any{"code": runtime.Code})
}

// UpdateRuntime 按路径 ID 更新运行时,防止请求体 code 误更新或新建其他运行时。
func (s *Service) UpdateRuntime(ctx context.Context, runtimeID int64, req RuntimeRequest) (Runtime, error) {
	if runtimeID <= 0 {
		return Runtime{}, apperr.ErrPathIDInvalid
	}
	var existing Runtime
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		existing, err = tx.GetRuntimeByID(ctx, runtimeID)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return Runtime{}, err
	}
	if strings.TrimSpace(req.Code) != existing.Code {
		return Runtime{}, apperr.ErrSandboxRuntimeUpdateInvalid
	}
	spec, err := validateRuntimeRequest(req, s.cfg)
	if err != nil {
		if errors.Is(err, apperr.ErrSandboxRuntimeCreateInvalid) {
			return Runtime{}, apperr.ErrSandboxRuntimeUpdateInvalid
		}
		return Runtime{}, err
	}
	applyBuiltinCapabilityDefault(&req, spec)
	if req.Status == 0 {
		req.Status = existing.Status
	}
	var runtime Runtime
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		runtime, err = tx.UpsertRuntime(ctx, runtimeID, req, spec)
		if err != nil {
			return apperr.ErrSandboxRuntimePersistFailed.WithCause(err)
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
	var runtime Runtime
	var image RuntimeImage
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		runtime, err = tx.GetRuntimeByID(ctx, runtimeID)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		image, err = tx.GetDefaultRuntimeImage(ctx, runtimeID)
		if err != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return RuntimeSelftestResponse{}, err
	}
	if !image.Prepulled || image.PrepullStatus != ImagePrepullSucceeded || !image.GenesisBaked {
		return RuntimeSelftestResponse{}, apperr.ErrSandboxRuntimeUnavailable
	}
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
	testCtx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.SelftestRecycleTimeoutSeconds))
	defer cancel()
	err := s.orchestrator.CreateSandboxResources(testCtx, CreateSandboxPlan{Sandbox: sb, Runtime: runtime, Image: image})
	if err == nil {
		_, _, err = s.orchestrator.Exec(testCtx, sb.Namespace, runtimeExecTarget(runtime), runtime.AdapterSpec.WorkspaceOps.Selftest, nil, false)
	}
	if err == nil {
		err = s.runRuntimeCapabilitySelftest(testCtx, sb, runtime)
	}
	cleanupBase := logging.WithAttrs(context.WithoutCancel(ctx), logging.AttrsFromContext(ctx)...)
	cleanupCtx, cleanupCancel := context.WithTimeout(cleanupBase, timeDurationSeconds(s.cfg.SelftestRecycleTimeoutSeconds))
	defer cleanupCancel()
	if cleanupErr := s.orchestrator.DestroySandboxResources(cleanupCtx, sb); cleanupErr != nil {
		logging.ErrorContext(ctx, "sandbox selftest cleanup failed", cleanupErr.Error(), slog.Int64("tenant_id", 0), slog.Int64("runtime_id", runtimeID), slog.Int64("sandbox_id", sb.ID), slog.String("namespace", sb.Namespace))
	}
	status := RuntimeSelftestPassed
	runtimeStatus := RuntimeStatusAvailable
	detail, encodeErr := jsonBytes(map[string]any{"result": "passed", "namespace": sb.Namespace})
	if encodeErr != nil {
		return RuntimeSelftestResponse{}, apperr.ErrSandboxSelftestFailed.WithCause(encodeErr)
	}
	if err != nil {
		status = RuntimeSelftestFailed
		runtimeStatus = RuntimeStatusOnboarding
		logging.ErrorContext(ctx, "sandbox runtime selftest failed", err.Error(), slog.Int64("tenant_id", 0), slog.Int64("runtime_id", runtimeID), slog.Int64("sandbox_id", sb.ID), slog.String("namespace", sb.Namespace))
		detail, encodeErr = jsonBytes(map[string]any{"result": "failed", "stage": "selftest", "trace_id": traceIDFromLogContext(ctx)})
		if encodeErr != nil {
			return RuntimeSelftestResponse{}, apperr.ErrSandboxSelftestFailed.WithCause(encodeErr)
		}
	}
	var updated Runtime
	if updateErr := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		updated, err = tx.UpdateRuntimeSelftest(ctx, runtimeID, status, runtimeStatus, detail)
		if err != nil {
			return apperr.ErrSandboxSelftestFailed.WithCause(err)
		}
		return nil
	}); updateErr != nil {
		return RuntimeSelftestResponse{}, updateErr
	}
	if auditErr := s.writeAuditFromContext(ctx, 0, "sandbox.runtime.selftest", "runtime", runtimeID, map[string]any{"status": status}); auditErr != nil {
		return RuntimeSelftestResponse{}, auditErr
	}
	resp := RuntimeSelftestResponse{RuntimeID: ids.ID(runtimeID), SelftestStatus: updated.SelftestStatus, RuntimeStatus: updated.Status, Detail: updated.SelftestDetail}
	if err != nil {
		return resp, apperr.ErrSandboxSelftestFailed.WithCause(err)
	}
	return resp, nil
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

// runRuntimeCapabilitySelftest 用标准 L2 能力执行 reset/deploy/query/reset 自检闭环。
func (s *Service) runRuntimeCapabilitySelftest(ctx context.Context, sb Sandbox, runtime Runtime) error {
	if runtime.AdapterLevel < 2 && strings.TrimSpace(runtime.CapabilityImpl) == "" && strings.TrimSpace(runtime.PluginRef) == "" {
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
	return RuntimeSelftestResponse{RuntimeID: ids.ID(runtime.ID), SelftestStatus: runtime.SelftestStatus, RuntimeStatus: runtime.Status, Detail: runtime.SelftestDetail}, nil
}

// PrepullRuntimeImage 触发 DaemonSet 全节点预拉取并以真实节点状态更新数据库。
func (s *Service) PrepullRuntimeImage(ctx context.Context, runtimeID, imageID int64) (PrepullResponse, error) {
	if runtimeID <= 0 || imageID <= 0 {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullParamInvalid
	}
	var runtime Runtime
	var image RuntimeImage
	var tools []Tool
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		runtime, err = tx.GetRuntimeByID(ctx, runtimeID)
		if err != nil {
			return apperr.ErrSandboxRuntimeNotFound.WithCause(err)
		}
		image, err = tx.GetRuntimeImageByID(ctx, runtimeID, imageID)
		if err != nil {
			return apperr.ErrSandboxRuntimeImageNotFound.WithCause(err)
		}
		tools, err = tx.ListTools(ctx)
		if err != nil {
			return apperr.ErrSandboxToolNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return PrepullResponse{}, err
	}
	if runtime.Status != RuntimeStatusAvailable || image.Status != RuntimeImageStatusAvailable {
		return PrepullResponse{}, apperr.ErrSandboxRuntimeUnavailable
	}
	prepullSpecs, err := prepullImageSpecsForRuntime(runtime, image, tools)
	if err != nil {
		return PrepullResponse{}, apperr.ErrSandboxImageAttestationInvalid.WithCause(err)
	}
	if len(prepullSpecs) == 0 {
		return PrepullResponse{}, apperr.ErrSandboxImageAttestationInvalid
	}
	imageURLs := prepullImageURLs(prepullSpecs)
	for _, imageURL := range imageURLs {
		if !imageAttested(s.cfg, imageURL, digestFromImageURL(imageURL)) {
			return PrepullResponse{}, apperr.ErrSandboxImageAttestationInvalid
		}
	}
	if !containsString(imageURLs, image.ImageURL) {
		return PrepullResponse{}, apperr.ErrSandboxImageAttestationInvalid
	}
	startingDetail, err := jsonBytes(map[string]any{"stage": "starting", "image_count": len(imageURLs), "images": imageURLs})
	if err != nil {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(err)
	}
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.UpdateRuntimeImagePrepull(ctx, runtimeID, imageID, false, ImagePrepullRunning, startingDetail, time.Time{})
		return err
	}); err != nil {
		return PrepullResponse{}, err
	}
	result, err := s.orchestrator.PrepullImage(ctx, image, prepullSpecs)
	status := ImagePrepullSucceeded
	prepulled := true
	at := timex.Now()
	if err != nil {
		status = ImagePrepullFailed
		prepulled = false
		at = time.Time{}
		logging.ErrorContext(ctx, "sandbox image prepull failed", err.Error(), slog.Int64("tenant_id", 0), slog.Int64("runtime_id", runtimeID), slog.Int64("image_id", imageID), slog.String("daemonset", result.DaemonSet))
		detail, encodeErr := jsonBytes(map[string]any{"stage": "failed", "daemonset": result.DaemonSet, "desired_nodes": result.DesiredNodes, "ready_nodes": result.ReadyNodes, "image_count": len(imageURLs), "images": imageURLs})
		if encodeErr != nil {
			return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(encodeErr)
		}
		result.Detail = detail
	}
	if updateErr := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, updateErr := tx.UpdateRuntimeImagePrepull(ctx, runtimeID, imageID, prepulled, status, result.Detail, at)
		return updateErr
	}); updateErr != nil {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(updateErr)
	}
	if auditErr := s.writeAuditFromContext(ctx, 0, "sandbox.image.prepull", "runtime_image", imageID, map[string]any{"runtime_id": runtimeID, "status": status}); auditErr != nil {
		return PrepullResponse{}, auditErr
	}
	if err != nil {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(err)
	}
	return PrepullResponse{ImageID: ids.ID(imageID), PrepullStatus: status, DesiredNodes: result.DesiredNodes, ReadyNodes: result.ReadyNodes, DaemonSet: result.DaemonSet, ImageCount: len(imageURLs), Images: imageURLs}, nil
}

// prepullImageSpecsForRuntime 汇总运行时默认工作负载会用到的不可变镜像和最小自检命令。
func prepullImageSpecsForRuntime(runtime Runtime, image RuntimeImage, tools []Tool) ([]PrepullImageSpec, error) {
	seen := map[string]int{}
	out := make([]PrepullImageSpec, 0, 1+len(runtime.AdapterSpec.InfraSidecars)+len(tools))
	add := func(imageURL string, command []string, hold bool, mounts []workload.EphemeralMountSpec) error {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL == "" {
			return nil
		}
		command = compactCommand(command)
		if len(command) == 0 {
			return fmt.Errorf("预拉取镜像 %s 缺少自检命令", imageURL)
		}
		key := prepullImageSpecKey(imageURL, hold)
		if index, exists := seen[key]; exists {
			out[index].EphemeralMounts = mergePrepullEphemeralMounts(out[index].EphemeralMounts, mounts)
			return nil
		}
		seen[key] = len(out)
		out = append(out, PrepullImageSpec{
			ImageURL:        imageURL,
			Command:         command,
			Hold:            hold,
			EphemeralMounts: mergePrepullEphemeralMounts(nil, mounts),
		})
		return nil
	}
	if err := add(image.ImageURL, runtime.AdapterSpec.WorkspaceOps.Selftest, false, nil); err != nil {
		return nil, err
	}
	for _, component := range runtime.AdapterSpec.InfraSidecars {
		if err := add(component.ImageURL, component.PrepullCommand, component.PrepullHold, component.EphemeralMounts); err != nil {
			return nil, err
		}
	}
	for _, pod := range runtime.AdapterSpec.Pods {
		for _, component := range pod.Containers {
			if strings.TrimSpace(component.Name) == strings.TrimSpace(runtime.AdapterSpec.RuntimeContainer.Name) {
				continue
			}
			if err := add(component.ImageURL, component.PrepullCommand, component.PrepullHold, component.EphemeralMounts); err != nil {
				return nil, err
			}
		}
	}
	for _, tool := range tools {
		if tool.Status != ToolStatusAvailable || !toolCompatible(runtime.Eco, tool.EcoTags) {
			continue
		}
		for _, component := range tool.ResourceSpec.Components {
			command := component.PrepullCommand
			if len(command) == 0 {
				command = tool.ResourceSpec.PrepullCommand
			}
			if err := add(component.ImageURL, command, component.PrepullHold, component.EphemeralMounts); err != nil {
				return nil, err
			}
		}
	}
	for i := range out {
		if out[i].Hold {
			return out, nil
		}
	}
	return nil, fmt.Errorf("运行时 %s 缺少预拉取保持容器命令", runtime.Code)
}

// prepullImageSpecKey 保持同镜像在自检和常驻两种用途下独立去重。
func prepullImageSpecKey(imageURL string, hold bool) string {
	return fmt.Sprintf("%s\x00%t", imageURL, hold)
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
		DesiredNodes int32    `json:"desired_nodes"`
		ReadyNodes   int32    `json:"ready_nodes"`
		DaemonSet    string   `json:"daemonset"`
		ImageCount   int      `json:"image_count"`
		Images       []string `json:"images"`
	}
	if err := jsonx.DecodeStrict(image.PrepullDetail, &detail); err != nil {
		return PrepullResponse{}, apperr.ErrSandboxImagePrepullFailed.WithCause(err)
	}
	resp.DesiredNodes = detail.DesiredNodes
	resp.ReadyNodes = detail.ReadyNodes
	resp.DaemonSet = detail.DaemonSet
	resp.ImageCount = detail.ImageCount
	resp.Images = detail.Images
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
		var err error
		tool, err = tx.UpsertTool(ctx, s.ids.Generate(), req, spec)
		if err != nil {
			return apperr.ErrSandboxToolPersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return Tool{}, err
	}
	return tool, s.writeAuditFromContext(ctx, 0, "sandbox.tool.upsert", "tool", tool.ID, map[string]any{"code": tool.Code})
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
