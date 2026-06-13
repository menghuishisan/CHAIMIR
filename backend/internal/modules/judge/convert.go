// judge convert 文件负责 DTO、领域模型与跨模块契约之间的纯转换。
package judge

import (
	"encoding/json"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// contractSubmitFromDTO 把内部 HTTP 请求转换为跨模块判题契约。
func contractSubmitFromDTO(tenantID int64, req SubmitTaskRequest) contracts.JudgeSubmitRequest {
	return contracts.JudgeSubmitRequest{
		TenantID:         tenantID,
		JudgerCode:       req.JudgerCode,
		ItemCode:         req.ItemCode,
		ItemVersion:      req.ItemVersion,
		CodeStorageKey:   req.CodeStorageKey,
		CodeHash:         req.CodeHash,
		SubmitterID:      req.SubmitterID,
		SourceRef:        req.SourceRef,
		SandboxMode:      req.SandboxMode,
		TargetSandboxRef: req.TargetSandboxRef,
		ExtraInput:       req.ExtraInput,
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
		Result:      contractResult(info.Result),
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
func contractResult(result *JudgeResult) contracts.JudgeTaskResult {
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
		Passed:      result.Passed,
		Score:       result.Score,
		MaxScore:    result.MaxScore,
		Details:     details,
		SnapshotRef: snapshotRef(result.TaskID),
	}
}

// taskInfoToMap 转成 API 响应 map,避免直接暴露内部字段。
func taskInfoToMap(info JudgeTaskInfo) map[string]any {
	out := map[string]any{
		"task_id":      info.Task.ID,
		"tenant_id":    info.Task.TenantID,
		"source_ref":   info.Task.SourceRef,
		"submitter_id": info.Task.SubmitterID,
		"status":       statusText(info.Task.Status),
	}
	if info.Result != nil {
		out["result"] = map[string]any{
			"passed":       info.Result.Passed,
			"score":        info.Result.Score,
			"max_score":    info.Result.MaxScore,
			"details":      info.Result.Details,
			"snapshot_ref": snapshotRef(info.Task.ID),
		}
	}
	return out
}

// judgerToMap 转换判题器定义为 API 输出。
func judgerToMap(j Judger) (map[string]any, error) {
	spec, err := jsonx.AnyBytes(j.ResourceSpec, apperr.ErrJudgerConfigInvalid)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":                  j.ID,
		"code":                j.Code,
		"name":                j.Name,
		"type":                j.Type,
		"executor_ref":        j.ExecutorRef,
		"runtime_required":    j.RuntimeRequired,
		"default_timeout_sec": j.DefaultTimeoutSec,
		"resource_spec":       json.RawMessage(spec),
		"selftest_status":     j.SelftestStatus,
		"status":              j.Status,
	}, nil
}

// fingerprintToMatch 转换查重命中为跨模块契约。
func fingerprintToMatch(fp SubmissionFingerprint, score float64) contracts.FingerprintMatch {
	return contracts.FingerprintMatch{SourceRef: fp.SourceRef, SubmitterID: fp.SubmitterID, Score: score, CodeHash: fp.CodeHash}
}

// snapshotRef 生成面向调用方的判题快照引用。
func snapshotRef(taskID int64) string {
	return "judge:2026:sub:" + ids.Format(taskID)
}
