package sandbox

import (
	"encoding/json"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
)

// sandboxInfoFromModel 把内部沙箱快照转换为跨模块摘要;运行时实例列表取自已校验的组合快照。
func sandboxInfoFromModel(sb Sandbox, snapshot contracts.SandboxCompositionSnapshot, runtime Runtime, image RuntimeImage, tools []SandboxTool) contracts.SandboxInfo {
	instances := make([]string, 0, len(snapshot.Runtimes))
	for _, item := range snapshot.Runtimes {
		instances = append(instances, strings.TrimSpace(item.InstanceCode))
	}
	return contracts.SandboxInfo{
		SandboxID:                sb.ID,
		TenantID:                 sb.TenantID,
		Namespace:                sb.Namespace,
		SourceRef:                sb.SourceRef,
		ScopeRef:                 sb.ScopeRef,
		CompositionDigest:        sb.CompositionDigest,
		AccessProfile:            snapshot.Spec.AccessProfile,
		OwnerAccountID:           sb.OwnerAccountID,
		RuntimeCode:              runtime.Code,
		RuntimeImageVersion:      image.Version,
		RuntimeInstances:         instances,
		WorkspaceRuntimeInstance: snapshot.Spec.WorkspaceRuntimeInstance,
		Phase:                    sb.Phase,
		Status:                   sb.Status,
		ToolAccess:               sandboxToolAccessFromModel(tools),
		WorkspaceRevision:        sb.WorkspaceRevision,
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
		SandboxID:           ids.ID(info.SandboxID),
		TenantID:            ids.ID(info.TenantID),
		SourceRef:           info.SourceRef,
		OwnerAccountID:      ids.ID(info.OwnerAccountID),
		RuntimeCode:         info.RuntimeCode,
		RuntimeImageVersion: info.RuntimeImageVersion,
		RuntimeInstances:    append([]string(nil), info.RuntimeInstances...),
		Phase:               info.Phase,
		Status:              info.Status,
		ToolAccess:          info.ToolAccess,
		Capabilities:        info.Capabilities,
		ResourceUsage:       info.ResourceUsage,
		WorkspaceRevision:   info.WorkspaceRevision,
	}
}

// runtimeChainOperations 只暴露该实例已声明且有注册实现的链操作。
func runtimeChainOperations(runtime Runtime, registered map[string]ChainCapability) []string {
	out := []string{}
	key := runtimeCapabilityKey(runtime.AdapterLevel, runtime.CapabilityImpl, runtime.PluginRef)
	if registered[key] == nil {
		return out
	}
	if runtime.AdapterLevel == RuntimeAdapterLevelPlugin || len(runtime.AdapterSpec.CapabilityCommands.Deploy.Command) > 0 {
		out = append(out, ChainOperationDeploy)
	}
	if runtime.AdapterLevel == RuntimeAdapterLevelPlugin || len(runtime.AdapterSpec.CapabilityCommands.Tx.Command) > 0 {
		out = append(out, ChainOperationTransaction)
	}
	if runtime.AdapterLevel == RuntimeAdapterLevelPlugin || len(runtime.AdapterSpec.CapabilityCommands.Query.Command) > 0 {
		out = append(out, ChainOperationQuery)
	}
	return out
}

// sandboxCapabilitiesFromSnapshot 按工作区归属和命名实例投影能力，不合并不同实例的执行入口。
func sandboxCapabilitiesFromSnapshot(snapshot contracts.SandboxCompositionSnapshot, tools []SandboxTool, registered map[string]ChainCapability) (contracts.SandboxCapabilities, error) {
	workspace, _, err := runtimeSnapshotForInstance(snapshot, snapshot.Spec.WorkspaceRuntimeInstance)
	if err != nil {
		return contracts.SandboxCapabilities{}, err
	}
	ops := workspace.AdapterSpec.WorkspaceOps
	out := contracts.SandboxCapabilities{
		FileWorkspace:   len(ops.ReadFile) > 0 && len(ops.WriteFile) > 0 && len(ops.ListFiles) > 0 && len(ops.PackTar) > 0,
		Terminal:        len(ops.Terminal) > 0,
		ChainOperations: map[string][]string{},
	}
	for _, tool := range tools {
		if tool.Kind == SandboxToolKindCommand {
			out.CommandTools = true
			break
		}
	}
	for _, frozen := range snapshot.Runtimes {
		runtime, _, err := runtimeSnapshotForInstance(snapshot, frozen.InstanceCode)
		if err != nil {
			return contracts.SandboxCapabilities{}, err
		}
		if operations := runtimeChainOperations(runtime, registered); len(operations) > 0 {
			out.ChainOperations[frozen.InstanceCode] = operations
		}
	}
	return out, nil
}

// runtimeResponseFromModel 将运行时内部模型转换为 HTTP 稳定字段名。
// adapter_spec 只在边界序列化为原始 JSON，避免把可执行 workload 模型泄露到公开 DTO。
func runtimeResponseFromModel(item Runtime) (RuntimeResponse, error) {
	adapterSpec, err := jsonx.AnyBytes(item.AdapterSpec, apperr.ErrInternal)
	if err != nil {
		return RuntimeResponse{}, err
	}
	return RuntimeResponse{
		ID:             ids.ID(item.ID),
		Code:           item.Code,
		Name:           item.Name,
		Eco:            item.Eco,
		AdapterLevel:   item.AdapterLevel,
		AdapterSpec:    json.RawMessage(adapterSpec),
		CapabilityImpl: item.CapabilityImpl,
		PluginRef:      item.PluginRef,
		SelftestStatus: item.SelftestStatus,
		Status:         item.Status,
	}, nil
}

// runtimeResponsesFromModels 批量转换运行时列表,每条带上可用镜像版本数(没有可用镜像的记 0)。
func runtimeResponsesFromModels(items []Runtime, availableImages map[int64]int64) ([]RuntimeResponse, error) {
	out := make([]RuntimeResponse, 0, len(items))
	for _, item := range items {
		response, err := runtimeResponseFromModel(item)
		if err != nil {
			return nil, err
		}
		count := availableImages[item.ID]
		response.AvailableImageCount = &count
		out = append(out, response)
	}
	return out, nil
}

// runtimeImageResponseFromModel 将运行时镜像内部模型转换为 HTTP 稳定字段名。
func runtimeImageResponseFromModel(item RuntimeImage) RuntimeImageResponse {
	return RuntimeImageResponse{
		ID:           ids.ID(item.ID),
		RuntimeID:    ids.ID(item.RuntimeID),
		ImageURL:     item.ImageURL,
		Version:      item.Version,
		Status:       item.Status,
		GenesisBaked: item.GenesisBaked,
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
// resource_spec 只在边界序列化为原始 JSON，避免把可执行 workload 模型泄露到公开 DTO。
func toolResponseFromModel(item Tool) (ToolResponse, error) {
	resourceSpec, err := jsonx.AnyBytes(item.ResourceSpec, apperr.ErrInternal)
	if err != nil {
		return ToolResponse{}, err
	}
	return ToolResponse{
		ID:           ids.ID(item.ID),
		Code:         item.Code,
		Name:         item.Name,
		Category:     item.Category,
		Kind:         item.Kind,
		EcoTags:      item.EcoTags,
		ResourceSpec: json.RawMessage(resourceSpec),
		Status:       item.Status,
	}, nil
}

// toolResponsesFromModels 批量转换沙箱工具列表。
func toolResponsesFromModels(items []Tool) ([]ToolResponse, error) {
	out := make([]ToolResponse, 0, len(items))
	for _, item := range items {
		response, err := toolResponseFromModel(item)
		if err != nil {
			return nil, err
		}
		out = append(out, response)
	}
	return out, nil
}

// orchestrationCatalogResponse 把编排目录投影转换为 HTTP 稳定字段名。
func orchestrationCatalogResponse(runtimes []CatalogRuntime, tools []CatalogTool) OrchestrationCatalogResponse {
	out := OrchestrationCatalogResponse{
		Runtimes: make([]CatalogRuntimeResponse, 0, len(runtimes)),
		Infra:    make([]CatalogToolResponse, 0),
		Tools:    make([]CatalogToolResponse, 0, len(tools)),
	}
	for _, runtime := range runtimes {
		images := make([]CatalogRuntimeImageResponse, 0, len(runtime.Images))
		for _, image := range runtime.Images {
			images = append(images, CatalogRuntimeImageResponse(image))
		}
		out.Runtimes = append(out.Runtimes, CatalogRuntimeResponse{
			Code:         runtime.Code,
			Name:         runtime.Name,
			Eco:          runtime.Eco,
			Images:       images,
			Capabilities: runtime.Capabilities,
		})
	}
	for _, tool := range tools {
		item := CatalogToolResponse{Code: tool.Code, Name: tool.Name, Kind: tool.Kind, Capabilities: tool.Capabilities}
		if tool.Category == "infra" {
			out.Infra = append(out.Infra, item)
			continue
		}
		out.Tools = append(out.Tools, item)
	}
	return out
}

// contractCreateFromDTO 把内部 HTTP 创建请求转换为跨模块创建契约。
func contractCreateFromDTO(req CreateSandboxRequest) contracts.SandboxCreateRequest {
	authorizedAccountIDs := make([]int64, 0, len(req.AuthorizedAccountIDs))
	for _, accountID := range req.AuthorizedAccountIDs {
		authorizedAccountIDs = append(authorizedAccountIDs, accountID.Int64())
	}
	snapshot := req.CompositionSnapshot
	snapshot.Digest = strings.TrimSpace(snapshot.Digest)
	return contracts.SandboxCreateRequest{
		TenantID:                 req.TenantID.Int64(),
		CompositionSnapshot:      snapshot,
		InitCodeRef:              strings.TrimSpace(snapshot.Spec.InitCodeRef),
		InitScriptRef:            strings.TrimSpace(snapshot.Spec.InitScriptRef),
		OwnerAccountID:           req.OwnerAccountID.Int64(),
		AuthorizedAccountIDs:     authorizedAccountIDs,
		SourceRef:                strings.TrimSpace(req.SourceRef),
		ScopeRef:                 strings.TrimSpace(req.ScopeRef),
		KeepAlive:                req.KeepAlive,
		SnapshotEnabled:          req.SnapshotEnabled,
		KeepAliveMinutes:         req.KeepAliveMinutes,
		SnapshotRetentionMinutes: req.SnapshotRetentionMinutes,
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
			PrepullCommand:         append([]string(nil), item.PrepullCommand...),
			PrepullHold:            item.PrepullHold,
			Env:                    env,
			Resources:              workload.ResourceSpec{Requests: copyStringMap(item.Resources.Requests), Limits: copyStringMap(item.Resources.Limits)},
			Workdir:                item.Workdir,
			ReadOnlyRootFilesystem: item.ReadOnlyRootFilesystem,
			Labels:                 copyStringMap(item.Labels),
			MountWorkspace:         item.MountWorkspace,
			MountDomains:           append([]string(nil), item.MountDomains...),
			EphemeralMounts:        mounts,
		})
	}
	return out
}

// jsonBytes 在入库前把已校验结构转换为 JSON 字节,失败时显式返回错误链。
func jsonBytes(v any) ([]byte, error) {
	return jsonx.AnyBytes(v, apperr.ErrInternal)
}
