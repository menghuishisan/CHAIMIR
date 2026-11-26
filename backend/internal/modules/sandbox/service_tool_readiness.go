package sandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"chaimir/pkg/apperr"
)

var (
	errToolNotReady = errors.New("工具工作负载或服务端点尚未就绪")
	errToolFailed   = errors.New("工具工作负载启动失败")
)

func frozenSandboxTools(sb Sandbox) ([]SandboxTool, error) {
	snapshot, err := verifiedCompositionSnapshot(sb)
	if err != nil {
		return nil, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	tools := make([]SandboxTool, 0, len(snapshot.Components))
	seen := make(map[string]struct{}, len(snapshot.Components))
	for _, component := range snapshot.Components {
		if component.Category == "infra" {
			continue
		}
		if _, exists := seen[component.Code]; exists {
			return nil, apperr.ErrSandboxCreateRequestInvalid
		}
		seen[component.Code] = struct{}{}
		spec, err := toolResourceSpecFromJSON(component.ResourceSpec)
		if err != nil {
			return nil, apperr.ErrSandboxToolIncompatible.WithCause(err)
		}
		definition := Tool{ID: component.ComponentID, Code: component.Code, Kind: component.Kind, ResourceSpec: spec}
		tools = append(tools, SandboxTool{ToolCode: component.Code, Kind: component.Kind, ResourceSpec: spec, AccessEndpoint: toolEndpoint(sb.ID, definition)})
	}
	return tools, nil
}

func (s *Service) currentSandboxTools(ctx context.Context, sb Sandbox) ([]SandboxTool, error) {
	tools, err := frozenSandboxTools(sb)
	if err != nil {
		return nil, err
	}
	var failures []error
	for index := range tools {
		status, err := s.sandboxToolStatus(ctx, sb, tools[index])
		if err != nil {
			failures = append(failures, fmt.Errorf("读取工具状态失败: tool_code=%s: %w", tools[index].ToolCode, err))
		}
		tools[index].Status = status
	}
	if err := errors.Join(failures...); err != nil {
		return nil, apperr.ErrSandboxToolProxyUnavailable.WithCause(err)
	}
	return tools, nil
}

func (s *Service) sandboxToolStatus(ctx context.Context, sb Sandbox, tool SandboxTool) (int16, error) {
	if !sandboxExecAllowed(sb) {
		if sb.Status == SandboxStatusFailed || sb.Status == SandboxStatusDestroyed {
			return SandboxToolStatusFailed, nil
		}
		return SandboxToolStatusStarting, nil
	}
	if tool.Kind != SandboxToolKindWebEmbed && tool.Kind != SandboxToolKindCommand {
		return SandboxToolStatusReady, nil
	}
	if s.orchestrator == nil {
		return 0, fmt.Errorf("工具就绪检查缺少编排器")
	}
	definition := Tool{Code: tool.ToolCode, Kind: tool.Kind, ResourceSpec: tool.ResourceSpec}
	switch err := s.orchestrator.ToolReady(ctx, sb, definition); {
	case err == nil:
		return SandboxToolStatusReady, nil
	case errors.Is(err, errToolFailed):
		return SandboxToolStatusFailed, nil
	case errors.Is(err, errToolNotReady):
		return SandboxToolStatusStarting, nil
	default:
		return 0, err
	}
}

func (s *Service) readySandboxTool(ctx context.Context, sb Sandbox, kind int16, code string) (SandboxTool, error) {
	if !sandboxExecAllowed(sb) {
		return SandboxTool{}, apperr.ErrSandboxStateInvalid
	}
	tools, err := frozenSandboxTools(sb)
	if err != nil {
		return SandboxTool{}, err
	}
	for _, tool := range tools {
		if !strings.EqualFold(tool.ToolCode, strings.TrimSpace(code)) || tool.Kind != kind {
			continue
		}
		tool.Status, err = s.sandboxToolStatus(ctx, sb, tool)
		if err != nil {
			return SandboxTool{}, apperr.ErrSandboxToolProxyUnavailable.WithCause(err)
		}
		if tool.Status != SandboxToolStatusReady {
			return SandboxTool{}, apperr.ErrSandboxToolProxyUnavailable.WithCause(errToolNotReady)
		}
		return tool, nil
	}
	return SandboxTool{}, apperr.ErrSandboxToolNotFound
}
