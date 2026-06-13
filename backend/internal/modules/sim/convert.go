// sim convert 文件负责 DTO、领域模型与跨模块契约之间的纯转换。
package sim

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
)

// packageToMap 转换仿真包为 API 输出。
func packageToMap(pkg Package) map[string]any {
	return map[string]any{
		"id":              pkg.ID,
		"code":            pkg.Code,
		"version":         pkg.Version,
		"name":            pkg.Name,
		"category":        pkg.Category,
		"compute":         computeText(pkg.Compute),
		"scale_limit":     pkg.ScaleLimit,
		"bundle_hash":     pkg.BundleHash,
		"backend_adapter": pkg.BackendAdapter,
		"backend_config":  pkg.BackendConfig,
		"author_type":     pkg.AuthorType,
		"author_id":       pkg.AuthorID,
		"status":          packageStatusText(pkg.Status),
		"created_at":      pkg.CreatedAt,
		"updated_at":      pkg.UpdatedAt,
	}
}

// reviewToMap 转换审核记录为 API 输出。
func reviewToMap(review Review) map[string]any {
	return map[string]any{
		"id":             review.ID,
		"package_id":     review.PackageID,
		"submitter_id":   review.SubmitterID,
		"preview_report": review.PreviewReport,
		"reviewer_id":    review.ReviewerID,
		"result":         reviewResultText(review.Result),
		"comment":        review.Comment,
		"created_at":     review.CreatedAt,
		"updated_at":     review.UpdatedAt,
	}
}

// reviewInfoToMap 转换审核列表投影为 API 输出。
func reviewInfoToMap(info ReviewInfo) map[string]any {
	out := reviewToMap(info.Review)
	out["package"] = map[string]any{
		"code":     info.PackageCode,
		"version":  info.PackageVersion,
		"name":     info.PackageName,
		"category": info.Category,
		"compute":  computeText(info.Compute),
		"status":   packageStatusText(info.PackageStatus),
	}
	return out
}

// sessionToContract 转换创建结果为跨模块契约。
func sessionToContract(session Session, pkg Package) contracts.SimSessionInfo {
	return contracts.SimSessionInfo{SessionID: session.ID, TenantID: session.TenantID, PackageCode: pkg.Code, Version: pkg.Version, Compute: computeText(session.Compute), BundleRef: pkg.BundleKey, SourceRef: session.SourceRef}
}

// replayToContract 转换回放数据为跨模块契约。
func replayToContract(session SessionWithPackage, actions []Action) contracts.SimReplayInfo {
	items := make([]contracts.SimActionInfo, 0, len(actions))
	for _, action := range actions {
		items = append(items, contracts.SimActionInfo{Seq: action.Seq, AtTick: action.AtTick, EventType: action.EventType, Payload: action.Payload})
	}
	return contracts.SimReplayInfo{PackageCode: session.PackageCode, Version: session.PackageVersion, Seed: session.Seed, InitParams: session.InitParams, Actions: items}
}

// replayToMap 转换回放数据为 HTTP 输出。
func replayToMap(session SessionWithPackage, actions []Action) map[string]any {
	items := make([]map[string]any, 0, len(actions))
	for _, action := range actions {
		items = append(items, actionToMap(action))
	}
	return map[string]any{"package_code": session.PackageCode, "version": session.PackageVersion, "seed": session.Seed, "init_params": session.InitParams, "actions": items}
}

// actionToMap 转换操作为 API 输出。
func actionToMap(action Action) map[string]any {
	return map[string]any{"seq": action.Seq, "at_tick": action.AtTick, "event_type": action.EventType, "payload": action.Payload, "created_at": action.CreatedAt}
}

// reportFromRequest 转换动态预览报告请求。
func reportFromRequest(req ValidationReportRequest) ValidationReport {
	return ValidationReport{DeterminismCheck: req.DeterminismCheck, WorkerPreview: req.WorkerPreview, Details: req.Details}
}

// rawReportMap 把动态报告请求解码为 key 集合,用于阻止覆盖后端静态字段。
func rawReportMap(raw []byte) (map[string]any, error) {
	out, err := jsonx.ObjectMapStrict(raw)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// packageStatusText 返回用户接口中的包状态字符串。
func packageStatusText(status int16) string {
	switch status {
	case PackageStatusDraft:
		return "draft"
	case PackageStatusReviewing:
		return "reviewing"
	case PackageStatusPublished:
		return "published"
	case PackageStatusArchived:
		return "archived"
	case PackageStatusRejected:
		return "rejected"
	default:
		return "unknown"
	}
}

// reviewResultText 返回审核状态字符串。
func reviewResultText(result int16) string {
	switch result {
	case ReviewPending:
		return "pending"
	case ReviewApproved:
		return "approved"
	case ReviewRejected:
		return "rejected"
	default:
		return "unknown"
	}
}
