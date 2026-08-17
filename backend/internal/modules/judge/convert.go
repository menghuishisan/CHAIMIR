// judge convert 文件负责 DTO、领域模型与跨模块契约之间的纯转换。
package judge

import (
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// contractSubmitFromDTO 把内部 HTTP 请求转换为跨模块判题契约,来源标识只取服务签名上下文。
func contractSubmitFromDTO(tenantID int64, sourceRef string, req SubmitTaskRequest) contracts.JudgeSubmitRequest {
	return contracts.JudgeSubmitRequest{
		TenantID:         tenantID,
		JudgerCode:       req.JudgerCode,
		ItemCode:         req.ItemCode,
		ItemVersion:      req.ItemVersion,
		CodeStorageKey:   req.CodeStorageKey,
		CodeHash:         req.CodeHash,
		SubmitterID:      req.SubmitterID.Int64(),
		SourceRef:        sourceRef,
		SourceOwnerID:    req.SourceOwnerID.Int64(),
		SourceCourseID:   req.SourceCourseID.Int64(),
		SourceScope:      req.SourceScope,
		ExtraInput:       req.ExtraInput,
		SandboxMode:      req.SandboxMode,
		TargetSandboxRef: req.TargetSandboxRef,
		Priority:         req.Priority,
	}
}

// contractTaskInfoFromModel 把 M3 任务摘要转换为跨模块返回契约。
func contractTaskInfoFromModel(info JudgeTaskInfo) contracts.JudgeTaskInfo {
	return contracts.JudgeTaskInfo{
		TaskID:      info.Task.ID,
		TenantID:    info.Task.TenantID,
		SourceRef:   info.Task.SourceRef,
		SubmitterID: info.Task.SubmitterID,
		Status:      contractStatus(info.Task.Status),
		Result:      contractResult(info.Task, info.Result),
	}
}

// contractStatus 将 M3 内部状态映射为 contracts 公开状态。
func contractStatus(status int16) int16 {
	switch status {
	case JudgeTaskStatusQueued:
		return contracts.JudgeTaskStatusQueued
	case JudgeTaskStatusJudging:
		return contracts.JudgeTaskStatusRunning
	case JudgeTaskStatusDone:
		return contracts.JudgeTaskStatusDone
	case JudgeTaskStatusCancelled:
		return contracts.JudgeTaskStatusCanceled
	default:
		return contracts.JudgeTaskStatusFailed
	}
}

// contractResult 转换判题结果,缺失结果时返回零值摘要。
func contractResult(task JudgeTask, result *JudgeResult) contracts.JudgeTaskResult {
	if result == nil {
		return contracts.JudgeTaskResult{}
	}
	details := make([]contracts.JudgeResultDetail, 0, len(result.Details))
	for _, detail := range result.Details {
		details = append(details, contracts.JudgeResultDetail{
			Case:          detail.Case,
			Passed:        detail.Passed,
			ExpectedLabel: detail.ExpectedLabel,
			Actual:        detail.Actual,
			Hint:          detail.Hint,
		})
	}
	return contracts.JudgeTaskResult{
		Passed:    result.Passed,
		Score:     result.Score,
		MaxScore:  result.MaxScore,
		Details:   details,
		ResultRef: resultRef(task),
		Replay:    result.Replay,
	}
}

// judgeTaskDTOFromModel 转换任务及其最新结果为公开 HTTP DTO。
func judgeTaskDTOFromModel(info JudgeTaskInfo) JudgeTaskDTO {
	out := JudgeTaskDTO{
		TaskID:      ids.ID(info.Task.ID),
		TenantID:    ids.ID(info.Task.TenantID),
		SourceRef:   info.Task.SourceRef,
		SubmitterID: ids.ID(info.Task.SubmitterID),
		Status:      statusText(info.Task.Status),
		Existing:    info.Existing,
	}
	if info.Result != nil {
		details := make([]JudgeResultDetailDTO, 0, len(info.Result.Details))
		for _, detail := range info.Result.Details {
			details = append(details, JudgeResultDetailDTO{
				Case:          detail.Case,
				Source:        detail.Source,
				Target:        detail.Target,
				Passed:        detail.Passed,
				ExpectedLabel: detail.ExpectedLabel,
				Actual:        detail.Actual,
				Hint:          detail.Hint,
			})
		}
		out.Result = &JudgeTaskResultDTO{
			Passed:    info.Result.Passed,
			Score:     info.Result.Score,
			MaxScore:  info.Result.MaxScore,
			Version:   info.Result.Version,
			IsRejudge: info.Result.IsRejudge,
			Details:   details,
			ResultRef: resultRef(info.Task),
		}
	}
	return out
}

// judgerDTOFromModel 转换判题器定义为公开 HTTP DTO。
func judgerDTOFromModel(j Judger) (JudgerDTO, error) {
	spec, err := jsonx.AnyBytes(j.ResourceSpec, apperr.ErrJudgerConfigInvalid)
	if err != nil {
		return JudgerDTO{}, err
	}
	return JudgerDTO{
		ID:                ids.ID(j.ID),
		Code:              j.Code,
		Name:              j.Name,
		Type:              j.Type,
		ExecutorRef:       j.ExecutorRef,
		RuntimeRequired:   j.RuntimeRequired,
		DefaultTimeoutSec: j.DefaultTimeoutSec,
		ResourceSpec:      spec,
		SelftestStatus:    j.SelftestStatus,
		Status:            j.Status,
	}, nil
}

// judgerCatalogResponse 把判题方式目录投影转换为 HTTP 稳定字段名。
func judgerCatalogResponse(items []CatalogJudger) JudgerCatalogResponse {
	out := JudgerCatalogResponse{Judgers: make([]CatalogJudgerResponse, 0, len(items))}
	for _, item := range items {
		out.Judgers = append(out.Judgers, CatalogJudgerResponse(item))
	}
	return out
}

// fingerprintToMatch 转换查重命中为跨模块契约。
func fingerprintToMatch(fp SubmissionFingerprint, score float64) contracts.FingerprintMatch {
	return contracts.FingerprintMatch{SourceRef: fp.SourceRef, SubmitterID: fp.SubmitterID, Score: score, CodeHash: fp.CodeHash}
}

// resultRef 生成面向调用方的判题结果引用,供实验记录关联判题详情。
func resultRef(task JudgeTask) string {
	return fmt.Sprintf("judge:%04d:result:%s", task.CreatedAt.Year(), ids.Format(task.ID))
}
