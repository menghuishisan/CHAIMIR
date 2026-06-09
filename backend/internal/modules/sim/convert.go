// M4 转换层:处理领域 DTO、contracts DTO 与 HTTP 输出结构之间的纯转换。
package sim

import (
	"chaimir/internal/platform/ids"
)

// packageAuthorScope 是权限规则所需的最小作者归属信息。
type packageAuthorScope struct {
	AuthorType int16
	AuthorID   int64
}

// packageAuthorScopeFromSnapshot 从仿真包投影提取权限判断所需字段。
func packageAuthorScopeFromSnapshot(row PackageSnapshot) packageAuthorScope {
	if !row.HasAuthorID {
		return packageAuthorScope{AuthorType: row.AuthorType}
	}
	return packageAuthorScope{AuthorType: row.AuthorType, AuthorID: row.AuthorID}
}

// packageToDTO 转换仿真包投影。
func packageToDTO(row PackageSnapshot) PackageDTO {
	return PackageDTO{ID: ids.Format(row.ID), Code: row.Code, Version: row.Version, Name: row.Name, Category: row.Category,
		Compute: computeString(row.Compute), ScaleLimit: row.ScaleLimit, BundleHash: row.BundleHash,
		BackendAdapter: row.BackendAdapter, Status: row.Status}
}

// packagesToDTO 批量转换仿真包投影。
func packagesToDTO(rows []PackageSnapshot) []PackageDTO {
	out := make([]PackageDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, packageToDTO(row))
	}
	return out
}

// reviewToDTO 转换审核记录投影。
func reviewToDTO(row ReviewSnapshot) ReviewDTO {
	dto := ReviewDTO{ID: ids.Format(row.ID), PackageID: ids.Format(row.PackageID), SubmitterID: ids.Format(row.SubmitterID),
		PreviewReport: row.PreviewReport, Result: row.Result}
	if row.HasReviewerID {
		dto.ReviewerID = ids.Format(row.ReviewerID)
	}
	if row.HasComment {
		dto.Comment = row.Comment
	}
	return dto
}

// reviewsToDTO 批量转换审核记录。
func reviewsToDTO(rows []ReviewSnapshot) []ReviewDTO {
	out := make([]ReviewDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, reviewToDTO(row))
	}
	return out
}

// sessionToDTO 转换会话与包信息为创建响应。
func sessionToDTO(row SessionSnapshot, pkg PackageSnapshot) SessionDTO {
	return SessionDTO{SessionID: ids.Format(row.ID), Compute: computeString(row.Compute), BundleRef: pkg.BundleKey,
		PackageCode: pkg.Code, Version: pkg.Version, Seed: row.Seed, InitParams: row.InitParams, Status: row.Status}
}

// actionToDTO 转换操作日志投影。
func actionToDTO(row ActionSnapshot) ActionDTO {
	return ActionDTO{Seq: row.Seq, AtTick: row.AtTick, EventType: row.EventType, Payload: row.Payload, CreatedAt: row.CreatedAt}
}

// replayFromSnapshots 组装回放响应。
func replayFromSnapshots(row ReplaySnapshot, actions []ActionSnapshot) ReplayDTO {
	out := ReplayDTO{PackageCode: row.PackageCode, Version: row.PackageVersion, Seed: row.Seed, InitParams: row.InitParams, Actions: []ActionDTO{}}
	for _, action := range actions {
		out.Actions = append(out.Actions, actionToDTO(action))
	}
	return out
}

// backendSessionFromReplay 转换回放投影为后端计算会话投影。
func backendSessionFromReplay(row ReplaySnapshot) BackendSessionSnapshot {
	return BackendSessionSnapshot{
		ID: row.ID, OwnerAccountID: row.OwnerAccountID, Compute: row.Compute,
		BackendAdapter: row.PackageBackend, BackendConfig: row.PackageBackendConf, HasBackend: row.HasPackageBackend,
	}
}
