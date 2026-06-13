// sandbox convert 文件负责 DTO、内部模型与跨模块契约之间的纯转换。
package sandbox

import (
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// sandboxInfoFromModel 把内部沙箱快照转换为跨模块摘要。
func sandboxInfoFromModel(sb Sandbox, runtime Runtime, image RuntimeImage, tools []SandboxTool) contracts.SandboxInfo {
	return contracts.SandboxInfo{
		SandboxID:           sb.ID,
		TenantID:            sb.TenantID,
		Namespace:           sb.Namespace,
		SourceRef:           sb.SourceRef,
		OwnerAccountID:      sb.OwnerAccountID,
		RuntimeCode:         runtime.Code,
		RuntimeImageVersion: image.Version,
		Phase:               sb.Phase,
		Status:              sb.Status,
		ToolAccess:          sandboxToolAccessFromModel(tools),
	}
}

// sandboxToolAccessFromModel 转换沙箱工具接入摘要。
func sandboxToolAccessFromModel(tools []SandboxTool) []contracts.SandboxToolAccess {
	out := make([]contracts.SandboxToolAccess, 0, len(tools))
	for _, tool := range tools {
		out = append(out, contracts.SandboxToolAccess{
			ToolCode: tool.ToolCode,
			Kind:     tool.Kind,
			Endpoint: tool.AccessEndpoint,
			Status:   tool.Status,
		})
	}
	return out
}

// contractCreateFromDTO 把内部 HTTP 创建请求转换为跨模块创建契约。
func contractCreateFromDTO(req CreateSandboxRequest) contracts.SandboxCreateRequest {
	return contracts.SandboxCreateRequest{
		TenantID:                 req.TenantID,
		RuntimeCode:              strings.TrimSpace(req.RuntimeCode),
		RuntimeImageVersion:      strings.TrimSpace(req.RuntimeImageVersion),
		ToolCodes:                req.Tools,
		InitCodeRef:              strings.TrimSpace(req.InitCodeRef),
		InitScriptRef:            strings.TrimSpace(req.InitScriptRef),
		OwnerAccountID:           req.OwnerAccountID,
		SourceRef:                strings.TrimSpace(req.SourceRef),
		KeepAlive:                req.KeepAlive,
		SnapshotEnabled:          req.SnapshotEnabled,
		KeepAliveMinutes:         req.KeepAliveMinutes,
		SnapshotRetentionMinutes: req.SnapshotRetentionMinutes,
	}
}

// createInputFromContract 把跨模块创建契约转换为规则层使用的本模块模型。
func createInputFromContract(req contracts.SandboxCreateRequest) CreateSandboxInputModel {
	return CreateSandboxInputModel{
		TenantID:                 req.TenantID,
		RuntimeCode:              strings.TrimSpace(req.RuntimeCode),
		RuntimeImageVersion:      strings.TrimSpace(req.RuntimeImageVersion),
		ToolCodes:                req.ToolCodes,
		InitCodeRef:              strings.TrimSpace(req.InitCodeRef),
		InitScriptRef:            strings.TrimSpace(req.InitScriptRef),
		OwnerAccountID:           req.OwnerAccountID,
		SourceRef:                strings.TrimSpace(req.SourceRef),
		KeepAlive:                req.KeepAlive,
		SnapshotEnabled:          req.SnapshotEnabled,
		KeepAliveMinutes:         req.KeepAliveMinutes,
		SnapshotRetentionMinutes: req.SnapshotRetentionMinutes,
	}
}

// jsonBytes 在入库前把已校验结构转换为 JSON 字节,失败时显式返回错误链。
func jsonBytes(v any) ([]byte, error) {
	return jsonx.AnyBytes(v, apperr.ErrInternal)
}
