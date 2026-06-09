// M2 运行时管理补全:更新、自检查询、镜像预拉取。
package sandbox

import (
	"context"
	"strings"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// UpdateRuntime 更新运行时定义。
func (s *Service) UpdateRuntime(ctx context.Context, runtimeID int64, req UpdateRuntimeRequest) (map[string]any, error) {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Eco) == "" || req.AdapterLevel < 1 || req.AdapterLevel > 3 {
		return nil, apperr.ErrRuntimeInvalid
	}
	spec, err := jsonx.ObjectBytes(req.AdapterSpec, apperr.ErrRuntimeInvalid)
	if err != nil {
		return nil, err
	}
	if _, err := parseRuntimeAdapterSpec(spec); err != nil {
		return nil, err
	}
	row, err := s.repo.updateRuntime(ctx, runtimeID, req, spec)
	if err != nil {
		return nil, err
	}
	if err := s.writeAudit(ctx, 0, auditActionRuntimeUpdate, auditTargetRuntime, row.ID, map[string]any{"code": row.Code}); err != nil {
		return nil, err
	}
	return runtimeToMap(row), nil
}

// GetRuntimeSelftest 查询运行时最近一次自检结果。
func (s *Service) GetRuntimeSelftest(ctx context.Context, runtimeID int64) (map[string]any, error) {
	row, err := s.repo.getRuntime(ctx, runtimeID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"runtime_id":      ids.Format(row.ID),
		"selftest_status": row.SelftestStatus,
		"selftest_detail": jsonx.ObjectMap(row.SelftestDetail),
		"runtime_status":  row.Status,
		"capability_impl": row.CapabilityImpl,
		"runtime_code":    row.Code,
		"runtime_name":    row.Name,
		"adapter_level":   row.AdapterLevel,
		"adapter_spec":    jsonx.ObjectMap(row.AdapterSpec),
	}, nil
}

// PrepullRuntimeImage 触发真实 DaemonSet 预拉取,并以目标节点状态更新运行时镜像控制面。
func (s *Service) PrepullRuntimeImage(ctx context.Context, runtimeID, imageID int64) (map[string]any, error) {
	image, err := s.repo.getRuntimeAndImageForPrepull(ctx, runtimeID, imageID)
	if err != nil {
		return nil, err
	}
	if err := validateRuntimeImageURL(image.ImageURL, s.cfg); err != nil {
		return nil, apperr.ErrRuntimePrepullFailed.WithCause(err)
	}
	startedDetail, err := jsonx.ObjectBytes(map[string]any{"stage": "started"}, apperr.ErrRuntimePrepullFailed)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.updateRuntimeImagePrepull(ctx, runtimeID, imageID, false, RuntimeImagePrepullRunning, startedDetail, nil); err != nil {
		return nil, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}

	// 再调用 K8s 编排器创建/更新 DaemonSet,成功口径来自节点真实 ready 数。
	if s.orchestrator == nil {
		return nil, apperr.ErrRuntimePrepullFailed
	}
	status, err := s.orchestrator.PrepullImage(ctx, ImagePrepullSpec{
		RuntimeImageID: imageID,
		RuntimeID:      runtimeID,
		ImageURL:       image.ImageURL,
	})
	if err != nil {
		if _, markErr := s.updateRuntimeImagePrepull(ctx, runtimeID, imageID, false, RuntimeImagePrepullFailed, status, nil); markErr != nil {
			return nil, apperr.ErrRuntimePrepullFailed.WithCause(markErr)
		}
		return nil, apperr.ErrRuntimePrepullFailed.WithCause(err)
	}
	// 最后持久化完成状态和节点明细,供创建沙箱前做强门禁。
	row, err := s.updateRuntimeImagePrepull(ctx, runtimeID, imageID, true, RuntimeImagePrepullDone, status, ptrTime(timex.Now()))
	if err != nil {
		return nil, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	if err := s.writeAudit(ctx, 0, auditActionRuntimeImagePrepull, auditTargetRuntimeImage, imageID, map[string]any{
		"runtime_id":    runtimeID,
		"desired_nodes": status.DesiredNodes,
		"ready_nodes":   status.ReadyNodes,
	}); err != nil {
		return nil, err
	}
	return runtimeImageToMap(row), nil
}

// GetRuntimeImagePrepull 查询镜像预拉取状态。
func (s *Service) GetRuntimeImagePrepull(ctx context.Context, runtimeID, imageID int64) (map[string]any, error) {
	image, err := s.repo.getRuntimeImage(ctx, runtimeID, imageID)
	if err != nil {
		return nil, err
	}
	return runtimeImageToMap(image), nil
}

// updateRuntimeImagePrepull 持久化预拉取状态和节点明细。
func (s *Service) updateRuntimeImagePrepull(
	ctx context.Context,
	runtimeID, imageID int64,
	prepulled bool,
	prepullStatus int16,
	status ImagePrepullStatus,
	prepulledAt *time.Time,
) (RuntimeImageSnapshot, error) {
	detail, err := jsonx.ObjectBytes(map[string]any{
		"daemonset":     status.DaemonSet,
		"desired_nodes": status.DesiredNodes,
		"ready_nodes":   status.ReadyNodes,
		"failed_nodes":  status.FailedNodes,
		"failure":       status.Failure,
	}, apperr.ErrRuntimePrepullFailed)
	if err != nil {
		return RuntimeImageSnapshot{}, err
	}
	return s.repo.updateRuntimeImagePrepull(ctx, runtimeID, imageID, prepulled, prepullStatus, detail, prepulledAt)
}

// ptrTime 构造预拉取完成时间指针,用于可选 DTO 字段。
func ptrTime(t time.Time) *time.Time {
	return &t
}
