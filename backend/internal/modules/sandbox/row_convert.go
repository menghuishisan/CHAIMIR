// M2 行转换层:集中处理 sqlc 行到领域投影和响应 DTO 的纯转换。
package sandbox

import (
	"chaimir/internal/modules/sandbox/internal/sqlcgen"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// runtimeConfigFromRow 把运行时配置表行转换为平台级运行时投影。
func runtimeConfigFromRow(row sqlcgen.Runtime) RuntimeConfigSnapshot {
	return RuntimeConfigSnapshot{
		ID:             row.ID,
		Code:           row.Code,
		Name:           row.Name,
		Eco:            row.Eco,
		AdapterLevel:   row.AdapterLevel,
		AdapterSpec:    row.AdapterSpec,
		CapabilityImpl: pgtypex.TextValue(row.CapabilityImpl),
		PluginRef:      pgtypex.TextValue(row.PluginRef),
		SelftestStatus: row.SelftestStatus,
		SelftestDetail: row.SelftestDetail,
		Status:         row.Status,
	}
}

// runtimeConfigsFromRows 批量转换运行时配置投影。
func runtimeConfigsFromRows(rows []sqlcgen.Runtime) []RuntimeConfigSnapshot {
	out := make([]RuntimeConfigSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, runtimeConfigFromRow(row))
	}
	return out
}

// runtimeImageFromRow 把运行时镜像表行转换为镜像投影。
func runtimeImageFromRow(row sqlcgen.RuntimeImage) RuntimeImageSnapshot {
	return RuntimeImageSnapshot{
		ID:            row.ID,
		RuntimeID:     row.RuntimeID,
		ImageURL:      row.ImageUrl,
		Version:       row.Version,
		Prepulled:     row.Prepulled,
		PrepullStatus: row.PrepullStatus,
		PrepullDetail: row.PrepullDetail,
		PrepulledAt:   timex.FromTimestamptz(row.PrepulledAt),
		GenesisBaked:  row.GenesisBaked,
		IsDefault:     row.IsDefault,
	}
}

// toolConfigFromRow 把工具表行转换为控制面工具投影。
func toolConfigFromRow(row sqlcgen.Tool) ToolConfigSnapshot {
	return ToolConfigSnapshot{
		ID:           row.ID,
		Code:         row.Code,
		Name:         row.Name,
		Kind:         row.Kind,
		ImageURL:     pgtypex.TextValue(row.ImageUrl),
		Port:         pgtypex.Int4Value(row.Port),
		EcoTags:      row.EcoTags,
		ResourceSpec: row.ResourceSpec,
		Status:       row.Status,
	}
}

// toolConfigsFromRows 批量转换工具配置投影。
func toolConfigsFromRows(rows []sqlcgen.Tool) []ToolConfigSnapshot {
	out := make([]ToolConfigSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, toolConfigFromRow(row))
	}
	return out
}

// sandboxToolAccessFromRow 把沙箱工具连接查询转换为访问端点投影。
func sandboxToolAccessFromRow(row sqlcgen.ListSandboxToolsRow) SandboxToolAccessSnapshot {
	return SandboxToolAccessSnapshot{
		ID:             row.ID,
		TenantID:       row.TenantID,
		SandboxID:      row.SandboxID,
		ToolID:         row.ToolID,
		AccessEndpoint: row.AccessEndpoint,
		Status:         row.Status,
		ToolCode:       row.ToolCode,
		ToolName:       row.ToolName,
		ToolKind:       row.ToolKind,
	}
}

// sandboxToolAccessesFromRows 批量转换沙箱工具访问端点投影。
func sandboxToolAccessesFromRows(rows []sqlcgen.ListSandboxToolsRow) []SandboxToolAccessSnapshot {
	out := make([]SandboxToolAccessSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, sandboxToolAccessFromRow(row))
	}
	return out
}

// sandboxToolProxyFromRow 把代理目标查询转换为访问端点投影。
func sandboxToolProxyFromRow(row sqlcgen.GetSandboxToolForProxyRow) SandboxToolAccessSnapshot {
	return SandboxToolAccessSnapshot{
		ID:             row.ID,
		TenantID:       row.TenantID,
		SandboxID:      row.SandboxID,
		ToolID:         row.ToolID,
		AccessEndpoint: row.AccessEndpoint,
		Status:         row.Status,
		ToolCode:       row.ToolCode,
		ToolName:       row.ToolName,
		ToolKind:       row.ToolKind,
	}
}

// tenantQuotaFromRow 把租户配额表行转换为配额投影。
func tenantQuotaFromRow(row sqlcgen.TenantQuotum) TenantQuotaSnapshot {
	return TenantQuotaSnapshot{
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

// activeSandboxResourceFromRow 把活跃资源统计行转换为领域投影。
func activeSandboxResourceFromRow(row sqlcgen.ListActiveSandboxResourceSpecsRow) ActiveSandboxResourceSnapshot {
	out := ActiveSandboxResourceSnapshot{
		SandboxID:          row.SandboxID,
		RuntimeAdapterSpec: row.RuntimeAdapterSpec,
	}
	if row.ToolID.Valid {
		out.Tool = &ToolConfigSnapshot{
			ID:           row.ToolID.Int64,
			Code:         row.ToolCode.String,
			Name:         row.ToolName.String,
			Kind:         row.ToolKind.Int16,
			ImageURL:     pgtypex.TextValue(row.ToolImageUrl),
			Port:         pgtypex.Int4Value(row.ToolPort),
			EcoTags:      row.ToolEcoTags.String,
			ResourceSpec: row.ToolResourceSpec,
		}
	}
	return out
}

// activeSandboxResourcesFromRows 批量转换活跃资源统计投影。
func activeSandboxResourcesFromRows(rows []sqlcgen.ListActiveSandboxResourceSpecsRow) []ActiveSandboxResourceSnapshot {
	out := make([]ActiveSandboxResourceSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, activeSandboxResourceFromRow(row))
	}
	return out
}

// sandboxLifecycleFromRow 把沙箱表行转换为生命周期领域投影。
func sandboxLifecycleFromRow(row sqlcgen.Sandbox) SandboxLifecycleSnapshot {
	return SandboxLifecycleSnapshot{
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
		InitScriptRef:     pgtypex.TextValue(row.InitScriptRef),
		SnapshotRef:       pgtypex.TextValue(row.SnapshotRef),
		SnapshotCreatedAt: timex.FromTimestamptz(row.SnapshotCreatedAt),
		SnapshotExpireAt:  timex.FromTimestamptz(row.SnapshotExpireAt),
		KeepAliveUntil:    timex.FromTimestamptz(row.KeepAliveUntil),
		LastActiveAt:      timex.FromTimestamptz(row.LastActiveAt),
		ExpireAt:          timex.FromTimestamptz(row.ExpireAt),
		CreatedAt:         timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:         timex.FromTimestamptz(row.UpdatedAt),
	}
}

// sandboxLifecyclesFromRows 批量转换沙箱生命周期投影。
func sandboxLifecyclesFromRows(rows []sqlcgen.Sandbox) []SandboxLifecycleSnapshot {
	out := make([]SandboxLifecycleSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, sandboxLifecycleFromRow(row))
	}
	return out
}
