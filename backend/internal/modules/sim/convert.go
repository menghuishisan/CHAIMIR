// sim convert 文件负责 DTO、领域模型与跨模块契约之间的纯转换。
package sim

import (
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// packageToMap 转换仿真包为 API 输出。
func packageToMap(pkg Package) (map[string]any, error) {
	compute, err := computeText(pkg.Compute)
	if err != nil {
		return nil, err
	}
	status, err := packageStatusText(pkg.Status)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"id":              ids.Format(pkg.ID),
		"code":            pkg.Code,
		"version":         pkg.Version,
		"name":            pkg.Name,
		"category":        pkg.Category,
		"compute":         compute,
		"scale_limit":     pkg.ScaleLimit,
		"bundle_hash":     pkg.BundleHash,
		"backend_adapter": pkg.BackendAdapter,
		"status":          status,
		"created_at":      pkg.CreatedAt,
		"updated_at":      pkg.UpdatedAt,
	}
	if pkg.AuthorID < 0 {
		return nil, apperr.ErrSimPackageDataCorrupt.WithCause(fmt.Errorf("仿真包 %d 的作者字段异常: author_id=%d", pkg.ID, pkg.AuthorID))
	}
	return out, nil
}

// reviewToMap 转换审核记录为 API 输出。
func reviewToMap(review Review) (map[string]any, error) {
	result, err := reviewResultText(review.Result)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"id":             ids.Format(review.ID),
		"package_id":     ids.Format(review.PackageID),
		"submitter_id":   ids.Format(review.SubmitterID),
		"preview_report": review.PreviewReport,
		"result":         result,
		"comment":        review.Comment,
		"created_at":     review.CreatedAt,
		"updated_at":     review.UpdatedAt,
	}
	if review.ReviewerID > 0 {
		out["reviewer_id"] = ids.Format(review.ReviewerID)
	}
	return out, nil
}

// reviewInfoToMap 转换审核列表投影为 API 输出。
func reviewInfoToMap(info ReviewInfo) (map[string]any, error) {
	out, err := reviewToMap(info.Review)
	if err != nil {
		return nil, err
	}
	compute, err := computeText(info.Compute)
	if err != nil {
		return nil, err
	}
	status, err := packageStatusText(info.PackageStatus)
	if err != nil {
		return nil, err
	}
	out["package"] = map[string]any{
		"code":     info.PackageCode,
		"version":  info.PackageVersion,
		"name":     info.PackageName,
		"category": info.Category,
		"compute":  compute,
		"status":   status,
	}
	return out, nil
}

// sessionToContract 转换创建结果为跨模块契约。
func sessionToContract(session Session, pkg Package) (contracts.SimSessionInfo, error) {
	compute, err := computeText(session.Compute)
	if err != nil {
		return contracts.SimSessionInfo{}, err
	}
	return contracts.SimSessionInfo{SessionID: session.ID, TenantID: session.TenantID, PackageCode: pkg.Code, Version: pkg.Version, Compute: compute, BundleRef: pkg.BundleKey, SourceRef: session.SourceRef}, nil
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

// replayToMapPublic 转换公开分享剧本,过滤检查点答案、令牌和内部绑定字段。
func replayToMapPublic(session SessionWithPackage, actions []Action) map[string]any {
	items := make([]map[string]any, 0, len(actions))
	for _, action := range actions {
		items = append(items, map[string]any{"seq": action.Seq, "at_tick": action.AtTick, "event_type": action.EventType, "payload": publicReplayMap(action.Payload)})
	}
	return map[string]any{"package_code": session.PackageCode, "version": session.PackageVersion, "seed": session.Seed, "init_params": publicReplayMap(session.InitParams), "actions": items}
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
func packageStatusText(status int16) (string, error) {
	switch status {
	case PackageStatusDraft:
		return "draft", nil
	case PackageStatusReviewing:
		return "reviewing", nil
	case PackageStatusPublished:
		return "published", nil
	case PackageStatusArchived:
		return "archived", nil
	case PackageStatusRejected:
		return "rejected", nil
	default:
		return "", apperr.ErrSimPackageDataCorrupt.WithCause(fmt.Errorf("仿真包状态异常: status=%d", status))
	}
}

// reviewResultText 返回审核状态字符串。
func reviewResultText(result int16) (string, error) {
	switch result {
	case ReviewPending:
		return "pending", nil
	case ReviewApproved:
		return "approved", nil
	case ReviewRejected:
		return "rejected", nil
	default:
		return "", apperr.ErrSimReviewDataCorrupt.WithCause(fmt.Errorf("审核记录状态异常: result=%d", result))
	}
}
