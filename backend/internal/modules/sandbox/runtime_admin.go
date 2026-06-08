// M2 运行时管理补全:更新、自检查询、镜像预拉取。
package sandbox

import (
	"context"
	"strings"
	"time"

	"chaimir/internal/modules/sandbox/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
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
	var row sqlcgen.Runtime
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetRuntimeByID(ctx, runtimeID); err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrRuntimeNotFound
			}
			return err
		}
		row, err = q.UpdateRuntime(ctx, sqlcgen.UpdateRuntimeParams{
			ID: runtimeID, Name: req.Name, Eco: req.Eco, AdapterLevel: req.AdapterLevel, AdapterSpec: spec,
			CapabilityImpl: pgText(req.CapabilityImpl),
			PluginRef:      pgText(req.PluginRef),
			Status:         req.Status,
		})
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	if err := s.writeAudit(ctx, 0, auditActionRuntimeUpdate, auditTargetRuntime, row.ID, map[string]any{"code": row.Code}); err != nil {
		return nil, err
	}
	return runtimeToMap(row), nil
}

// GetRuntimeSelftest 查询运行时最近一次自检结果。
func (s *Service) GetRuntimeSelftest(ctx context.Context, runtimeID int64) (map[string]any, error) {
	var row sqlcgen.Runtime
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetRuntimeByID(ctx, runtimeID)
		if err != nil && db.IsNoRows(err) {
			return apperr.ErrRuntimeNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}
	return map[string]any{
		"runtime_id":      ids.Format(row.ID),
		"selftest_status": row.SelftestStatus,
		"selftest_detail": jsonx.ObjectMap(row.SelftestDetail),
		"runtime_status":  row.Status,
		"capability_impl": textValue(row.CapabilityImpl),
		"runtime_code":    row.Code,
		"runtime_name":    row.Name,
		"adapter_level":   row.AdapterLevel,
		"adapter_spec":    jsonx.ObjectMap(row.AdapterSpec),
	}, nil
}

// PrepullRuntimeImage 触发真实 DaemonSet 预拉取,并以目标节点状态更新控制面。
func (s *Service) PrepullRuntimeImage(ctx context.Context, runtimeID, imageID int64) (map[string]any, error) {
	var image sqlcgen.RuntimeImage
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetRuntimeByID(ctx, runtimeID); err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrRuntimeNotFound
			}
			return err
		}
		current, err := q.GetRuntimeImage(ctx, sqlcgen.GetRuntimeImageParams{ID: imageID, RuntimeID: runtimeID})
		if err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrRuntimeImageNotFound
			}
			return err
		}
		image = current
		if err := validateRuntimeImageURL(image.ImageUrl, s.cfg); err != nil {
			return apperr.ErrRuntimePrepullFailed.WithCause(err)
		}
		detail, marshalErr := jsonx.ObjectBytes(map[string]any{"stage": "started"}, apperr.ErrRuntimePrepullFailed)
		if marshalErr != nil {
			return marshalErr
		}
		_, err = q.UpdateRuntimeImagePrepull(ctx, sqlcgen.UpdateRuntimeImagePrepullParams{
			ID: imageID, RuntimeID: runtimeID, Prepulled: false,
			PrepullStatus: RuntimeImagePrepullRunning, PrepullDetail: detail,
			PrepulledAt: pgtype.Timestamptz{},
		})
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrRuntimePersistenceFail.WithCause(err)
	}

	if s.orchestrator == nil {
		return nil, apperr.ErrRuntimePrepullFailed
	}
	status, err := s.orchestrator.PrepullImage(ctx, ImagePrepullSpec{
		RuntimeImageID: imageID,
		RuntimeID:      runtimeID,
		ImageURL:       image.ImageUrl,
	})
	if err != nil {
		if _, markErr := s.updateRuntimeImagePrepull(ctx, runtimeID, imageID, false, RuntimeImagePrepullFailed, status, nil); markErr != nil {
			return nil, apperr.ErrRuntimePrepullFailed.WithCause(markErr)
		}
		return nil, apperr.ErrRuntimePrepullFailed.WithCause(err)
	}
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
	var image sqlcgen.RuntimeImage
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		image, err = q.GetRuntimeImage(ctx, sqlcgen.GetRuntimeImageParams{ID: imageID, RuntimeID: runtimeID})
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
) (sqlcgen.RuntimeImage, error) {
	detail, err := jsonx.ObjectBytes(map[string]any{
		"daemonset":     status.DaemonSet,
		"desired_nodes": status.DesiredNodes,
		"ready_nodes":   status.ReadyNodes,
		"failed_nodes":  status.FailedNodes,
		"failure":       status.Failure,
	}, apperr.ErrRuntimePrepullFailed)
	if err != nil {
		return sqlcgen.RuntimeImage{}, err
	}
	var at pgtype.Timestamptz
	if prepulledAt != nil {
		at = timex.RequiredTimestamptz(*prepulledAt)
	}
	var row sqlcgen.RuntimeImage
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.UpdateRuntimeImagePrepull(ctx, sqlcgen.UpdateRuntimeImagePrepullParams{
			ID: imageID, RuntimeID: runtimeID, Prepulled: prepulled,
			PrepullStatus: prepullStatus, PrepullDetail: detail, PrepulledAt: at,
		})
		return e
	}); err != nil {
		return sqlcgen.RuntimeImage{}, err
	}
	return row, nil
}

// ptrTime 构造预拉取完成时间指针,用于可选 DTO 字段。
func ptrTime(t time.Time) *time.Time {
	return &t
}
