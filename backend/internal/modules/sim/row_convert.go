// M4 行转换层:集中处理 sqlc 行到领域投影和响应 DTO 的纯转换。
package sim

import (
	"chaimir/internal/modules/sim/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
)

// packageSnapshotFromRow 转换仿真包行供 service 使用。
func packageSnapshotFromRow(row sqlcgen.SimPackage) PackageSnapshot {
	out := PackageSnapshot{
		ID: row.ID, Code: row.Code, Version: row.Version, Name: row.Name, Category: row.Category,
		Compute: row.Compute, ScaleLimit: jsonx.ObjectMap(row.ScaleLimit), BundleKey: row.BundleKey,
		BundleHash: row.BundleHash, BackendAdapter: row.BackendAdapter.String,
		BackendConfig: jsonx.ObjectMap(row.BackendConfig), AuthorType: row.AuthorType, Status: row.Status,
	}
	if row.AuthorID.Valid {
		out.AuthorID = row.AuthorID.Int64
		out.HasAuthorID = true
	}
	return out
}

// packageSnapshotsFromRows 批量转换仿真包行。
func packageSnapshotsFromRows(rows []sqlcgen.SimPackage) []PackageSnapshot {
	out := make([]PackageSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, packageSnapshotFromRow(row))
	}
	return out
}

// reviewSnapshotFromRow 转换审核记录行供 service 使用。
func reviewSnapshotFromRow(row sqlcgen.SimPackageReview) ReviewSnapshot {
	out := ReviewSnapshot{
		ID: row.ID, PackageID: row.PackageID, SubmitterID: row.SubmitterID,
		PreviewReport: jsonx.ObjectMap(row.PreviewReport), Result: row.Result,
	}
	if row.ReviewerID.Valid {
		out.ReviewerID = row.ReviewerID.Int64
		out.HasReviewerID = true
	}
	if row.Comment.Valid {
		out.Comment = row.Comment.String
		out.HasComment = true
	}
	return out
}

// reviewSnapshotsFromRows 批量转换审核记录行。
func reviewSnapshotsFromRows(rows []sqlcgen.SimPackageReview) []ReviewSnapshot {
	out := make([]ReviewSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, reviewSnapshotFromRow(row))
	}
	return out
}

// sessionSnapshotFromRow 转换会话行供 service 使用。
func sessionSnapshotFromRow(row sqlcgen.SimSession) SessionSnapshot {
	return SessionSnapshot{
		ID: row.ID, TenantID: row.TenantID, PackageID: row.PackageID, SourceRef: row.SourceRef,
		OwnerAccountID: row.OwnerAccountID, Seed: row.Seed, InitParams: jsonx.ObjectMap(row.InitParams),
		Compute: row.Compute, Status: row.Status,
	}
}

// actionSnapshotFromRow 转换操作日志行供 service 使用。
func actionSnapshotFromRow(row sqlcgen.SimActionLog) ActionSnapshot {
	return ActionSnapshot{Seq: row.Seq, AtTick: row.AtTick, EventType: row.EventType, Payload: jsonx.ObjectMap(row.Payload), CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// actionSnapshotsFromRows 批量转换操作日志行。
func actionSnapshotsFromRows(rows []sqlcgen.SimActionLog) []ActionSnapshot {
	out := make([]ActionSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, actionSnapshotFromRow(row))
	}
	return out
}

// replaySnapshotFromRow 转换会话包联查行供回放和后端计算使用。
func replaySnapshotFromRow(row sqlcgen.GetSimSessionWithPackageRow) ReplaySnapshot {
	out := ReplaySnapshot{
		ID: row.ID, TenantID: row.TenantID, PackageID: row.PackageID, SourceRef: row.SourceRef,
		OwnerAccountID: row.OwnerAccountID, Seed: row.Seed, InitParams: jsonx.ObjectMap(row.InitParams),
		Compute: row.Compute, Status: row.Status, PackageCode: row.PackageCode, PackageVersion: row.PackageVersion,
		PackageBundleKey: row.PackageBundleKey, PackageBundleHash: row.PackageBundleHash,
		PackageBackendConf: jsonx.ObjectMap(row.PackageBackendConfig),
	}
	if row.PackageBackendAdapter.Valid {
		out.PackageBackend = row.PackageBackendAdapter.String
		out.HasPackageBackend = true
	}
	return out
}
