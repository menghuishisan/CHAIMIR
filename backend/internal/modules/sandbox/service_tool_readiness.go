// sandbox service_tool_readiness 统一冻结工具规格和实时就绪判断,供详情与所有访问入口复用。
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

var errToolNotReady = errors.New("工具工作负载或服务端点尚未就绪")

// currentSandboxTools 投影当前工具状态;目录和上次启动检查不能覆盖冻结规格或实时就绪事实。
func (s *Service) currentSandboxTools(ctx context.Context, sb Sandbox, tools []SandboxTool) ([]SandboxTool, error) {
	if len(tools) == 0 {
		return tools, nil
	}
	var snapshot contracts.SandboxCompositionSnapshot
	if err := jsonx.DecodeStrictKnownFields(sb.CompositionSnapshot, &snapshot); err != nil {
		return nil, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	if snapshot.Digest != sb.CompositionDigest || snapshot.Digest == "" {
		return nil, apperr.ErrSandboxCreateRequestInvalid
	}
	frozen := make(map[int64]contracts.CompiledComponentSnapshot, len(snapshot.Components))
	for _, component := range snapshot.Components {
		if _, exists := frozen[component.ComponentID]; exists {
			return nil, apperr.ErrSandboxCreateRequestInvalid
		}
		frozen[component.ComponentID] = component
	}
	out := append([]SandboxTool(nil), tools...)
	for index := range out {
		tool := &out[index]
		component, ok := frozen[tool.ToolID]
		if !ok || component.Code != tool.ToolCode {
			return nil, apperr.ErrSandboxToolIncompatible
		}
		spec, err := toolResourceSpecFromJSON(component.ResourceSpec)
		if err != nil {
			return nil, apperr.ErrSandboxToolIncompatible.WithCause(err)
		}
		tool.ResourceSpec = spec
		tool.Kind = component.Kind
		definition := Tool{ID: tool.ToolID, Code: tool.ToolCode, Kind: tool.Kind, ResourceSpec: tool.ResourceSpec}
		tool.AccessEndpoint = toolEndpoint(sb.ID, definition)
		if tool.Kind != SandboxToolKindWebEmbed && tool.Kind != SandboxToolKindCommand {
			continue
		}
		if !sandboxExecAllowed(sb) {
			tool.Status = SandboxToolStatusStarting
			if sb.Status == SandboxStatusFailed || sb.Status == SandboxStatusDestroyed {
				tool.Status = SandboxToolStatusFailed
			}
			continue
		}
		if s.orchestrator == nil {
			return nil, apperr.ErrSandboxToolProxyUnavailable.WithCause(fmt.Errorf("工具就绪检查缺少编排器"))
		}
		if err := s.orchestrator.ToolReady(ctx, sb, definition); err != nil {
			if !errors.Is(err, errToolNotReady) {
				return nil, apperr.ErrSandboxToolProxyUnavailable.WithCause(err)
			}
			// 正常启动或就绪回退保持准备态;启动已超时的失败记录保留,真实恢复后仍可重读解锁。
			if tool.Status != SandboxToolStatusFailed {
				tool.Status = SandboxToolStatusStarting
			}
			continue
		}
		tool.Status = SandboxToolStatusReady
	}
	return out, nil
}

// readySandboxTool 在已完成主体授权后选择工具,再使用同一实时状态门禁决定是否开放。
func (s *Service) readySandboxTool(ctx context.Context, sb Sandbox, tools []SandboxTool, kind int16, code string) (SandboxTool, error) {
	if !sandboxExecAllowed(sb) {
		return SandboxTool{}, apperr.ErrSandboxStateInvalid
	}
	for _, tool := range tools {
		if !strings.EqualFold(tool.ToolCode, strings.TrimSpace(code)) {
			continue
		}
		current, err := s.currentSandboxTools(ctx, sb, []SandboxTool{tool})
		if err != nil {
			return SandboxTool{}, err
		}
		if current[0].Kind != kind {
			return SandboxTool{}, apperr.ErrSandboxToolNotFound
		}
		if current[0].Status != SandboxToolStatusReady {
			return SandboxTool{}, apperr.ErrSandboxToolProxyUnavailable.WithCause(errToolNotReady)
		}
		return current[0], nil
	}
	return SandboxTool{}, apperr.ErrSandboxToolNotFound
}
