// sandbox row_convert 文件负责 sqlc 行类型到 M2 内部领域模型的纯映射。
package sandbox

import (
	"encoding/json"
	"strings"

	"chaimir/internal/modules/sandbox/internal/sqlcgen"
	"chaimir/internal/platform/timex"

	"github.com/jackc/pgx/v5/pgtype"
)

// runtimeFromRow 把 sqlc runtime 行转换为内部 Runtime 模型。
func runtimeFromRow(row sqlcgen.Runtime) (Runtime, error) {
	var spec AdapterSpec
	if len(row.AdapterSpec) > 0 {
		if err := json.Unmarshal(row.AdapterSpec, &spec); err != nil {
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
		CapabilityImpl: textValue(row.CapabilityImpl),
		PluginRef:      textValue(row.PluginRef),
		SelftestStatus: row.SelftestStatus,
		SelftestDetail: json.RawMessage(row.SelftestDetail),
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
		PrepullDetail: json.RawMessage(row.PrepullDetail),
		PrepulledAt:   timex.FromTimestamptz(row.PrepulledAt),
		GenesisBaked:  row.GenesisBaked,
		IsDefault:     row.IsDefault,
	}
}

// toolFromRow 把 sqlc tool 行转换为内部 Tool 模型。
func toolFromRow(row sqlcgen.Tool) (Tool, error) {
	var spec ToolResourceSpec
	if len(row.ResourceSpec) > 0 {
		if err := json.Unmarshal(row.ResourceSpec, &spec); err != nil {
			return Tool{}, err
		}
	}
	return Tool{
		ID:           row.ID,
		Code:         row.Code,
		Name:         row.Name,
		Kind:         row.Kind,
		ImageURL:     textValue(row.ImageUrl),
		Port:         int32Value(row.Port),
		EcoTags:      splitCSV(row.EcoTags),
		ResourceSpec: spec,
		Status:       row.Status,
	}, nil
}

// sandboxFromRow 把 sqlc sandbox 行转换为内部 Sandbox 快照。
func sandboxFromRow(row sqlcgen.Sandbox) Sandbox {
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
		CodeHash:          textValue(row.CodeHash),
		InitCodeRef:       textValue(row.InitCodeRef),
		InitScriptRef:     textValue(row.InitScriptRef),
		SnapshotRef:       textValue(row.SnapshotRef),
		SnapshotDomains:   stringArrayFromJSON(row.SnapshotDomains),
		SnapshotCreatedAt: timex.FromTimestamptz(row.SnapshotCreatedAt),
		SnapshotExpireAt:  timex.FromTimestamptz(row.SnapshotExpireAt),
		KeepAliveUntil:    timex.FromTimestamptz(row.KeepAliveUntil),
		LastActiveAt:      timex.FromTimestamptz(row.LastActiveAt),
		ExpireAt:          timex.FromTimestamptz(row.ExpireAt),
		CreatedAt:         timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:         timex.FromTimestamptz(row.UpdatedAt),
	}
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

// sandboxToolFromRow 把沙箱工具联查行转换为内部 SandboxTool 模型。
func sandboxToolFromRow(row sqlcgen.ListSandboxToolsRow) SandboxTool {
	return SandboxTool{
		ID:             row.ID,
		TenantID:       row.TenantID,
		SandboxID:      row.SandboxID,
		ToolID:         row.ToolID,
		ToolCode:       row.Code,
		Kind:           row.Kind,
		AccessEndpoint: row.AccessEndpoint,
		Status:         row.Status,
	}
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
		AccessEndpoint: row.AccessEndpoint,
		Status:         row.Status,
	}
}

// textValue 从 pgtype.Text 提取字符串,无效时返回空字符串。
func textValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

// int32Value 从 pgtype.Int4 提取 int32,无效时返回 0。
func int32Value(v pgtype.Int4) int32 {
	if !v.Valid {
		return 0
	}
	return v.Int32
}

// stringArrayFromJSON 从 JSONB 字符串数组提取快照卷域,解析失败时返回空列表交由上层恢复校验拒绝。
func stringArrayFromJSON(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return []string{}
	}
	return out
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
