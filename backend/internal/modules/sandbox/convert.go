// sandbox convert 文件负责 DTO、内部模型与跨模块契约之间的纯转换。
package sandbox

import (
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/workload"
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

// sandboxResponseFromInfo 删除仅服务端内部可用的资源定位字段后返回给前端。
func sandboxResponseFromInfo(info contracts.SandboxInfo) SandboxResponse {
	return SandboxResponse{
		SandboxID:           info.SandboxID,
		TenantID:            info.TenantID,
		SourceRef:           info.SourceRef,
		OwnerAccountID:      info.OwnerAccountID,
		RuntimeCode:         info.RuntimeCode,
		RuntimeImageVersion: info.RuntimeImageVersion,
		Phase:               info.Phase,
		Status:              info.Status,
		ToolAccess:          info.ToolAccess,
		Capabilities:        info.Capabilities,
		ResourceUsage:       info.ResourceUsage,
	}
}

// sandboxCapabilitiesFromModel 根据运行时命令清单和组合根注册表生成权威工作台能力。
func sandboxCapabilitiesFromModel(runtime Runtime, tools []SandboxTool, registered map[string]ChainCapability) contracts.SandboxCapabilities {
	ops := runtime.AdapterSpec.WorkspaceOps
	out := contracts.SandboxCapabilities{
		FileWorkspace:   len(ops.ReadFile) > 0 && len(ops.WriteFile) > 0 && len(ops.ListFiles) > 0 && len(ops.PackTar) > 0,
		Terminal:        len(ops.Terminal) > 0,
		ChainOperations: []string{},
	}
	for _, tool := range tools {
		if tool.Kind == SandboxToolKindCommand {
			out.CommandTools = true
			break
		}
	}
	key := strings.TrimSpace(runtime.CapabilityImpl)
	if runtime.AdapterLevel == 3 {
		key = strings.TrimSpace(runtime.PluginRef)
	}
	if registered[key] == nil {
		return out
	}
	if runtime.AdapterLevel == 3 || len(runtime.AdapterSpec.CapabilityCommands.Deploy.Command) > 0 {
		out.ChainOperations = append(out.ChainOperations, "deploy")
	}
	if runtime.AdapterLevel == 3 || len(runtime.AdapterSpec.CapabilityCommands.Tx.Command) > 0 {
		out.ChainOperations = append(out.ChainOperations, "transaction")
	}
	if runtime.AdapterLevel == 3 || len(runtime.AdapterSpec.CapabilityCommands.Query.Command) > 0 {
		out.ChainOperations = append(out.ChainOperations, "query")
	}
	return out
}

// runtimeResponseFromModel 将运行时内部模型转换为 HTTP 稳定字段名。
func runtimeResponseFromModel(item Runtime) RuntimeResponse {
	return RuntimeResponse{
		ID:             item.ID,
		Code:           item.Code,
		Name:           item.Name,
		Eco:            item.Eco,
		AdapterLevel:   item.AdapterLevel,
		AdapterSpec:    item.AdapterSpec,
		CapabilityImpl: item.CapabilityImpl,
		PluginRef:      item.PluginRef,
		SelftestStatus: item.SelftestStatus,
		SelftestDetail: item.SelftestDetail,
		Status:         item.Status,
	}
}

// runtimeResponsesFromModels 批量转换运行时列表。
func runtimeResponsesFromModels(items []Runtime) []RuntimeResponse {
	out := make([]RuntimeResponse, 0, len(items))
	for _, item := range items {
		out = append(out, runtimeResponseFromModel(item))
	}
	return out
}

// runtimeImageResponseFromModel 将运行时镜像内部模型转换为 HTTP 稳定字段名。
func runtimeImageResponseFromModel(item RuntimeImage) RuntimeImageResponse {
	return RuntimeImageResponse{
		ID:            item.ID,
		RuntimeID:     item.RuntimeID,
		ImageURL:      item.ImageURL,
		Version:       item.Version,
		Status:        item.Status,
		Prepulled:     item.Prepulled,
		PrepullStatus: item.PrepullStatus,
		PrepullDetail: item.PrepullDetail,
		PrepulledAt:   timex.RFC3339OrEmpty(item.PrepulledAt),
		GenesisBaked:  item.GenesisBaked,
		IsDefault:     item.IsDefault,
	}
}

// runtimeImageResponsesFromModels 批量转换运行时镜像列表。
func runtimeImageResponsesFromModels(items []RuntimeImage) []RuntimeImageResponse {
	out := make([]RuntimeImageResponse, 0, len(items))
	for _, item := range items {
		out = append(out, runtimeImageResponseFromModel(item))
	}
	return out
}

// toolResponseFromModel 将工具内部模型转换为 HTTP 稳定字段名。
func toolResponseFromModel(item Tool) ToolResponse {
	return ToolResponse{
		ID:           item.ID,
		Code:         item.Code,
		Name:         item.Name,
		Kind:         item.Kind,
		EcoTags:      item.EcoTags,
		ResourceSpec: item.ResourceSpec,
		Status:       item.Status,
	}
}

// toolResponsesFromModels 批量转换沙箱工具列表。
func toolResponsesFromModels(items []Tool) []ToolResponse {
	out := make([]ToolResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toolResponseFromModel(item))
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
		PrivateSidecars:          privateSidecarsFromContract(req.PrivateSidecars),
	}
}

// privateSidecarsFromContract 在 M2 边界把跨模块 DTO 转为内部 WorkloadSpec。
func privateSidecarsFromContract(items []contracts.SandboxPrivateSidecarSpec) []workload.ComponentSpec {
	out := make([]workload.ComponentSpec, 0, len(items))
	for _, item := range items {
		env := make([]workload.EnvVarSpec, 0, len(item.Env))
		for _, v := range item.Env {
			env = append(env, workload.EnvVarSpec{Name: v.Name, Value: v.Value})
		}
		mounts := make([]workload.EphemeralMountSpec, 0, len(item.EphemeralMounts))
		for _, mount := range item.EphemeralMounts {
			mounts = append(mounts, workload.EphemeralMountSpec{Name: mount.Name, MountPath: mount.MountPath})
		}
		out = append(out, workload.ComponentSpec{
			Name:                   item.Name,
			ImageURL:               item.ImageURL,
			Command:                append([]string(nil), item.Command...),
			Args:                   append([]string(nil), item.Args...),
			Env:                    env,
			Resources:              workload.ResourceSpec{Requests: copyStringMap(item.Resources.Requests), Limits: copyStringMap(item.Resources.Limits)},
			Workdir:                item.Workdir,
			ReadOnlyRootFilesystem: item.ReadOnlyRootFilesystem,
			Labels:                 copyStringMap(item.Labels),
			MountWorkspace:         item.MountWorkspace,
			EphemeralMounts:        mounts,
		})
	}
	return out
}

// jsonBytes 在入库前把已校验结构转换为 JSON 字节,失败时显式返回错误链。
func jsonBytes(v any) ([]byte, error) {
	return jsonx.AnyBytes(v, apperr.ErrInternal)
}
