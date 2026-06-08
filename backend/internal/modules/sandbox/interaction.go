// M2 沙箱交互服务:progress、pause/resume、工具代理目标。
package sandbox

import (
	"context"
	"fmt"

	"chaimir/internal/modules/sandbox/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/pkg/apperr"
)

// GetSandboxProgress 返回沙箱实时阶段状态。
func (s *Service) GetSandboxProgress(ctx context.Context, sandboxID int64) (SandboxProgressEvent, error) {
	row, err := s.loadSandboxRow(ctx, sandboxID)
	if err != nil {
		return SandboxProgressEvent{}, err
	}
	stage, message := phaseMessage(row.Phase)
	return SandboxProgressEvent{
		SandboxID: row.ID,
		Phase:     row.Phase,
		Stage:     stage,
		Message:   message,
		Status:    row.Status,
	}, nil
}

// PauseSandbox 暂停运行中的沙箱。
func (s *Service) PauseSandbox(ctx context.Context, sandboxID int64) error {
	row, binding, err := s.runtimeBindingForSandbox(ctx, sandboxID)
	if err != nil {
		return err
	}
	if err := s.SaveFiles(ctx, sandboxID); err != nil {
		return err
	}
	if err := s.orchestrator.Pause(ctx, binding); err != nil {
		return apperr.ErrSandboxRecycleFail.WithCause(err)
	}
	if err := s.updateSandboxProgress(ctx, row.TenantID, row.ID, row.Phase, SandboxStatusIdle, "已暂停", "实验环境已暂停,可稍后恢复"); err != nil {
		return err
	}
	return s.writeAudit(ctx, row.TenantID, auditActionSandboxPause, auditTargetSandbox, row.ID, map[string]any{
		"source_ref": row.SourceRef,
	})
}

// ResumeSandbox 恢复已暂停沙箱。
func (s *Service) ResumeSandbox(ctx context.Context, sandboxID int64) error {
	row, err := s.loadSandboxRow(ctx, sandboxID)
	if err != nil {
		return err
	}
	runtime, image, tools, err := s.loadSandboxDependencies(ctx, row)
	if err != nil {
		return err
	}
	spec, err := s.buildSandboxCreateSpec(runtime, image, tools, row, "")
	if err != nil {
		return err
	}
	if err := s.orchestrator.Resume(ctx, spec); err != nil {
		return apperr.ErrSandboxCreateFail.WithCause(err)
	}
	if err := s.updateSandboxProgress(ctx, row.TenantID, row.ID, row.Phase, SandboxStatusRunning, "已恢复", "实验环境已恢复"); err != nil {
		return err
	}
	return s.writeAudit(ctx, row.TenantID, auditActionSandboxResume, auditTargetSandbox, row.ID, map[string]any{
		"source_ref": row.SourceRef,
	})
}

// ToolProxyTarget 解析工具代理的 ClusterIP URL。
func (s *Service) ToolProxyTarget(ctx context.Context, sandboxID int64, toolCode string) (string, error) {
	row, _, err := s.runtimeBindingForSandbox(ctx, sandboxID)
	if err != nil {
		return "", err
	}
	if err := s.repo.inTenantID(ctx, row.TenantID, func(q *sqlcgen.Queries) error {
		tool, err := q.GetSandboxToolForProxy(ctx, sqlcgen.GetSandboxToolForProxyParams{
			SandboxID: sandboxID,
			Code:      toolCode,
		})
		if err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrToolNotFound
			}
			return err
		}
		if tool.ToolKind != ToolKindWebEmbed {
			return apperr.ErrToolProxyFail
		}
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return "", ae
		}
		return "", apperr.ErrToolProxyFail.WithCause(err)
	}
	endpoint, err := s.orchestrator.ToolEndpoint(ctx, row.Namespace, toolCode)
	if err != nil {
		return "", apperr.ErrToolProxyFail.WithCause(err)
	}
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", endpoint.ServiceName, row.Namespace, endpoint.ServicePort), nil
}

// loadSandboxRow 读取并校验当前租户可访问的沙箱。
func (s *Service) loadSandboxRow(ctx context.Context, sandboxID int64) (sqlcgen.Sandbox, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return sqlcgen.Sandbox{}, apperr.ErrUnauthorized
	}
	var row sqlcgen.Sandbox
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		current, err := q.GetSandboxByID(ctx, sandboxID)
		if err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrSandboxNotFound
			}
			return err
		}
		if err := authorizeSandboxRowAccess(ctx, id, current); err != nil {
			return err
		}
		row = current
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return sqlcgen.Sandbox{}, ae
		}
		return sqlcgen.Sandbox{}, apperr.ErrSandboxPersistenceFail.WithCause(err)
	}
	return row, nil
}

// loadSandboxDependencies 读取恢复/能力执行所需的 runtime/image/tool 配置。
func (s *Service) loadSandboxDependencies(ctx context.Context, row sqlcgen.Sandbox) (sqlcgen.Runtime, sqlcgen.RuntimeImage, []sqlcgen.Tool, error) {
	var runtime sqlcgen.Runtime
	var image sqlcgen.RuntimeImage
	var toolRows []sqlcgen.ListSandboxToolsRow
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		runtime, err = q.GetRuntimeByID(ctx, row.RuntimeID)
		if err != nil {
			return err
		}
		image, err = q.GetRuntimeImage(ctx, sqlcgen.GetRuntimeImageParams{ID: row.ImageID, RuntimeID: row.RuntimeID})
		if err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrRuntimeImageNotFound
			}
			return err
		}
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return sqlcgen.Runtime{}, sqlcgen.RuntimeImage{}, nil, ae
		}
		return sqlcgen.Runtime{}, sqlcgen.RuntimeImage{}, nil, apperr.ErrRuntimeUnavailable.WithCause(err)
	}
	if err := s.repo.inTenantID(ctx, row.TenantID, func(q *sqlcgen.Queries) error {
		rows, err := q.ListSandboxTools(ctx, row.ID)
		if err != nil {
			return err
		}
		toolRows = rows
		return nil
	}); err != nil {
		return sqlcgen.Runtime{}, sqlcgen.RuntimeImage{}, nil, apperr.ErrSandboxPersistenceFail.WithCause(err)
	}
	tools := make([]sqlcgen.Tool, 0, len(toolRows))
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		for _, item := range toolRows {
			tool, err := q.GetToolByCode(ctx, item.ToolCode)
			if err != nil {
				return err
			}
			tools = append(tools, tool)
		}
		return nil
	}); err != nil {
		return sqlcgen.Runtime{}, sqlcgen.RuntimeImage{}, nil, apperr.ErrToolNotFound.WithCause(err)
	}
	return runtime, image, tools, nil
}

// phaseMessage 将内部阶段码转成用户向进度文案,避免暴露 Pod 或镜像等技术细节。
func phaseMessage(phase int16) (string, string) {
	switch phase {
	case SandboxPhaseEnvironmentReady:
		return "环境就绪", "节点已就绪,可进入"
	case SandboxPhaseInitializing:
		return "个性化初始化中", "正在恢复代码并执行初始化脚本"
	case SandboxPhaseReady:
		return "完全就绪", "初始化完成,可开始操作"
	default:
		return "分配环境", "正在准备实验环境"
	}
}
