// sim convert 文件负责 DTO、领域模型与跨模块契约之间的纯转换。
package sim

import (
	"encoding/json"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
)

// packageToResponse 转换仿真包为公开 HTTP DTO。
func packageToResponse(pkg Package) (SimPackageResponse, error) {
	compute, err := computeText(pkg.Compute)
	if err != nil {
		return SimPackageResponse{}, err
	}
	status, err := packageStatusText(pkg.Status)
	if err != nil {
		return SimPackageResponse{}, err
	}
	scaleLimit, err := scaleLimitToResponse(pkg.ScaleLimit)
	if err != nil {
		return SimPackageResponse{}, err
	}
	if pkg.AuthorID < 0 {
		return SimPackageResponse{}, apperr.ErrSimPackageDataCorrupt.WithCause(fmt.Errorf("仿真包 %d 的作者字段异常: author_id=%d", pkg.ID, pkg.AuthorID))
	}
	return SimPackageResponse{ID: ids.ID(pkg.ID), Code: pkg.Code, Version: pkg.Version, Name: pkg.Name, Category: pkg.Category, Compute: compute, ScaleLimit: scaleLimit, BundleHash: pkg.BundleHash, Status: status, CreatedAt: pkg.CreatedAt, UpdatedAt: pkg.UpdatedAt}, nil
}

// scaleLimitToResponse 将已校验的规模上限转换为固定公开结构。
func scaleLimitToResponse(value map[string]any) (SimScaleLimitResponse, error) {
	if !validManifestScaleLimit(value) {
		return SimScaleLimitResponse{}, apperr.ErrSimPackageDataCorrupt.WithCause(fmt.Errorf("仿真包规模上限异常"))
	}
	nodes, _ := positiveJSONInt(value["nodes"])
	maxTick, _ := positiveJSONInt(value["max_tick"])
	maxEvents, _ := positiveJSONInt(value["max_events"])
	return SimScaleLimitResponse{Nodes: nodes, MaxTick: maxTick, MaxEvents: maxEvents}, nil
}

// reviewToResponse 转换审核记录为公开 HTTP DTO。
func reviewToResponse(review Review) (SimPackageReviewResponse, error) {
	result, err := reviewResultText(review.Result)
	if err != nil {
		return SimPackageReviewResponse{}, err
	}
	report, err := validationReportToResponse(review.PreviewReport)
	if err != nil {
		return SimPackageReviewResponse{}, err
	}
	out := SimPackageReviewResponse{ID: ids.ID(review.ID), PackageID: ids.ID(review.PackageID), SubmitterID: ids.ID(review.SubmitterID), PreviewReport: report, Result: result, Comment: review.Comment, CreatedAt: review.CreatedAt, UpdatedAt: review.UpdatedAt}
	if review.ReviewerID > 0 {
		out.ReviewerID = ids.ID(review.ReviewerID)
	}
	return out, nil
}

// validationReportToResponse 解析持久化报告中的动态教学帧,其余字段使用固定 DTO。
func validationReportToResponse(report ValidationReport) (SimValidationReportResponse, error) {
	out := SimValidationReportResponse{
		BundleHash:         report.BundleHash,
		MetadataValidation: SimValidationStatusResponse(report.MetadataValidation),
		StaticScan:         SimStaticScanReportResponse(report.StaticScan),
		DeterminismCheck:   SimValidationStatusResponse(report.DeterminismCheck),
		WorkerPreview:      SimValidationStatusResponse(report.WorkerPreview),
	}
	if len(report.PreviewFrames) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(report.PreviewFrames, &out.PreviewFrames); err != nil {
		return SimValidationReportResponse{}, apperr.ErrSimReviewDataCorrupt.WithCause(fmt.Errorf("审核样例教学帧数据异常: %w", err))
	}
	return out, nil
}

// reviewInfoToResponse 转换审核列表投影为公开 HTTP DTO。
func reviewInfoToResponse(info ReviewInfo) (SimPackageReviewResponse, error) {
	out, err := reviewToResponse(info.Review)
	if err != nil {
		return SimPackageReviewResponse{}, err
	}
	compute, err := computeText(info.Compute)
	if err != nil {
		return SimPackageReviewResponse{}, err
	}
	status, err := packageStatusText(info.PackageStatus)
	if err != nil {
		return SimPackageReviewResponse{}, err
	}
	out.Package = &SimReviewPackageResponse{Code: info.PackageCode, Version: info.PackageVersion, Name: info.PackageName, Category: info.Category, Compute: compute, Status: status}
	return out, nil
}

// sessionToContract 转换创建结果为跨模块契约。
func sessionToContract(session Session, pkg Package) (contracts.SimSessionInfo, error) {
	compute, err := computeText(session.Compute)
	if err != nil {
		return contracts.SimSessionInfo{}, err
	}
	return contracts.SimSessionInfo{SessionID: session.ID, TenantID: session.TenantID, PackageCode: pkg.Code, Version: pkg.Version, Compute: compute, SourceRef: session.SourceRef}, nil
}

// replayToContract 转换回放数据为跨模块契约。
func replayToContract(session SessionWithPackage, actions []Action) contracts.SimReplayInfo {
	items := make([]contracts.SimActionInfo, 0, len(actions))
	for _, action := range actions {
		items = append(items, contracts.SimActionInfo{Seq: action.Seq, AtTick: action.AtTick, EventType: action.EventType, Payload: action.Payload})
	}
	return contracts.SimReplayInfo{PackageCode: session.PackageCode, Version: session.PackageVersion, Seed: session.Seed, InitParams: session.InitParams, Actions: items}
}

// replayToResponse 转换登录用户回放为公开 HTTP DTO。
func replayToResponse(session SessionWithPackage, actions []Action) SimReplayResponse {
	items := make([]SimActionResponse, 0, len(actions))
	for _, action := range actions {
		items = append(items, actionToResponse(action))
	}
	return SimReplayResponse{PackageCode: session.PackageCode, Version: session.PackageVersion, Seed: session.Seed, InitParams: session.InitParams, Actions: items}
}

// replayToPublicResponse 转换公开分享剧本,过滤检查点答案、令牌和内部绑定字段。
func replayToPublicResponse(session SessionWithPackage, actions []Action) SimReplayResponse {
	items := make([]SimActionResponse, 0, len(actions))
	for _, action := range actions {
		items = append(items, SimActionResponse{Seq: action.Seq, AtTick: action.AtTick, EventType: action.EventType, Payload: publicReplayMap(action.Payload)})
	}
	return SimReplayResponse{PackageCode: session.PackageCode, Version: session.PackageVersion, Seed: session.Seed, InitParams: publicReplayMap(session.InitParams), Actions: items}
}

// actionToResponse 转换操作为公开 HTTP DTO。
func actionToResponse(action Action) SimActionResponse {
	return SimActionResponse{Seq: action.Seq, AtTick: action.AtTick, EventType: action.EventType, Payload: action.Payload, CreatedAt: action.CreatedAt}
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
