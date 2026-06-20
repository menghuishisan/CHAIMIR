// sandbox row_convert 文件负责 sqlc 行类型到 M2 内部领域模型的纯映射。
package sandbox

import (
	"strings"

	"chaimir/internal/modules/sandbox/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// runtimeFromRow 把 sqlc runtime 行转换为内部 Runtime 模型。
func runtimeFromRow(row sqlcgen.Runtime) (Runtime, error) {
	var spec AdapterSpec
	if len(row.AdapterSpec) > 0 {
		if err := jsonx.DecodeStrict(row.AdapterSpec, &spec); err != nil {
			return Runtime{}, err
		}
	}
	return Runtime{
		ID:             row.ID,
		Code:           row.Code,
		Name:           row.Name,
		Eco:            row.Eco,
		AdapterLevel:   row.AdapterLevel,
		AdapterSpec:    spec,
		CapabilityImpl: pgtypex.TextValue(row.CapabilityImpl),
		PluginRef:      pgtypex.TextValue(row.PluginRef),
		SelftestStatus: row.SelftestStatus,
		SelftestDetail: jsonx.RawMessage(row.SelftestDetail),
		Status:         row.Status,
	}, nil
}

// runtimeImageFromRow 把 sqlc runtime_image 行转换为内部 RuntimeImage 模型。
func runtimeImageFromRow(row sqlcgen.RuntimeImage) RuntimeImage {
	return RuntimeImage{
		ID:            row.ID,
		RuntimeID:     row.RuntimeID,
		ImageURL:      row.ImageUrl,
		Version:       row.Version,
		Status:        row.Status,
		Prepulled:     row.Prepulled,
		PrepullStatus: row.PrepullStatus,
		PrepullDetail: jsonx.RawMessage(row.PrepullDetail),
		PrepulledAt:   timex.FromTimestamptz(row.PrepulledAt),
		GenesisBaked:  row.GenesisBaked,
		IsDefault:     row.IsDefault,
	}
}

// toolFromRow 把 sqlc tool 行转换为内部 Tool 模型。
func toolFromRow(row sqlcgen.Tool) (Tool, error) {
	var spec ToolResourceSpec
	if len(row.ResourceSpec) > 0 {
		if err := jsonx.DecodeStrict(row.ResourceSpec, &spec); err != nil {
			return Tool{}, err
		}
	}
	return Tool{
		ID:           row.ID,
		Code:         row.Code,
		Name:         row.Name,
		Kind:         row.Kind,
		EcoTags:      splitCSV(row.EcoTags),
		ResourceSpec: spec,
		Status:       row.Status,
	}, nil
}

// sandboxFromRow 把 sqlc sandbox 行转换为内部 Sandbox 快照。
func sandboxFromRow(row sqlcgen.Sandbox) (Sandbox, error) {
	domains, err := stringArrayFromJSON(row.SnapshotDomains)
	if err != nil {
		return Sandbox{}, err
	}
	return Sandbox{
		ID:                row.ID,
		TenantID:          row.TenantID,
		RuntimeID:         row.RuntimeID,
		ImageID:           row.ImageID,
		Namespace:         row.Namespace,
		SourceRef:         row.SourceRef,
		OwnerAccountID:    row.OwnerAccountID,
		Phase:             row.Phase,
		Status:            row.Status,
		KeepAlive:         row.KeepAlive,
		SnapshotEnabled:   row.SnapshotEnabled,
		CodeStorageKey:    row.CodeStorageKey,
		CodeHash:          pgtypex.TextValue(row.CodeHash),
		InitCodeRef:       pgtypex.TextValue(row.InitCodeRef),
		InitScriptRef:     pgtypex.TextValue(row.InitScriptRef),
		SnapshotRef:       pgtypex.TextValue(row.SnapshotRef),
		SnapshotDomains:   domains,
		SnapshotCreatedAt: timex.FromTimestamptz(row.SnapshotCreatedAt),
		SnapshotExpireAt:  timex.FromTimestamptz(row.SnapshotExpireAt),
		KeepAliveUntil:    timex.FromTimestamptz(row.KeepAliveUntil),
		LastActiveAt:      timex.FromTimestamptz(row.LastActiveAt),
		ExpireAt:          timex.FromTimestamptz(row.ExpireAt),
		CreatedAt:         timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:         timex.FromTimestamptz(row.UpdatedAt),
	}, nil
}

// quotaFromRow 把 sqlc tenant_quota 行转换为内部 TenantQuota 模型。
func quotaFromRow(row sqlcgen.TenantQuotum) TenantQuota {
	return TenantQuota{
		TenantID:                row.TenantID,
		MaxConcurrentSandbox:    row.MaxConcurrentSandbox,
		MaxCPU:                  row.MaxCpu,
		MaxMemoryMB:             row.MaxMemoryMb,
		IdleTimeoutMin:          row.IdleTimeoutMin,
		MaxLifetimeMin:          row.MaxLifetimeMin,
		MaxKeepaliveMin:         row.MaxKeepaliveMin,
		MaxSnapshotRetentionMin: row.MaxSnapshotRetentionMin,
	}
}

// sandboxRecycleOutbox 把 sqlc 回收 outbox 行转换为内部模型。
func sandboxRecycleOutbox(row sqlcgen.SandboxRecycleOutbox) SandboxRecycleOutbox {
	return SandboxRecycleOutbox{ID: row.ID, TenantID: row.TenantID, SandboxID: row.SandboxID, SourceRef: row.SourceRef, OwnerAccountID: row.OwnerAccountID, Reason: row.Reason, TraceID: row.TraceID, RecycledAt: timex.FromTimestamptz(row.RecycledAt), Status: row.Status, RetryCount: row.RetryCount, LastError: pgtypex.TextValue(row.LastError), CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// sandboxToolFromRow 把沙箱工具联查行转换为内部 SandboxTool 模型。
func sandboxToolFromRow(row sqlcgen.ListSandboxToolsRow) (SandboxTool, error) {
	spec, err := toolResourceSpecFromJSON(row.ResourceSpec)
	if err != nil {
		return SandboxTool{}, err
	}
	return SandboxTool{
		ID:             row.ID,
		TenantID:       row.TenantID,
		SandboxID:      row.SandboxID,
		ToolID:         row.ToolID,
		ToolCode:       row.Code,
		Kind:           row.Kind,
		ResourceSpec:   spec,
		AccessEndpoint: row.AccessEndpoint,
		Status:         row.Status,
	}, nil
}

// sandboxToolFromStatusRow 把 sqlc sandbox_tool 行和工具定义合成为内部工具挂载模型。
func sandboxToolFromStatusRow(row sqlcgen.SandboxTool, tool Tool) SandboxTool {
	return SandboxTool{
		ID:             row.ID,
		TenantID:       row.TenantID,
		SandboxID:      row.SandboxID,
		ToolID:         row.ToolID,
		ToolCode:       tool.Code,
		Kind:           tool.Kind,
		ResourceSpec:   tool.ResourceSpec,
		AccessEndpoint: row.AccessEndpoint,
		Status:         row.Status,
	}
}

// toolResourceSpecFromJSON 解析已入库工具 WorkloadSpec,失败时显式交给调用链处理。
func toolResourceSpecFromJSON(raw []byte) (ToolResourceSpec, error) {
	var spec ToolResourceSpec
	if len(raw) == 0 {
		return spec, nil
	}
	if err := jsonx.DecodeStrict(raw, &spec); err != nil {
		return ToolResourceSpec{}, err
	}
	return spec, nil
}

// stringArrayFromJSON 从 JSONB 字符串数组提取快照卷域,解析失败时显式返回错误。
func stringArrayFromJSON(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var out []string
	if err := jsonx.DecodeStrict(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// splitCSV 把工具生态标签拆成去空格列表。
func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if clean := strings.TrimSpace(part); clean != "" {
			out = append(out, clean)
		}
	}
	return out
}
