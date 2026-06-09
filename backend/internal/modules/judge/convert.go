// M3 转换层:处理领域 DTO、contracts DTO 与 HTTP 输出结构之间的纯转换。
package judge

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
)

// judgerToMap 把判题器投影转换为 API 输出结构。
func judgerToMap(row JudgerSnapshot) map[string]any {
	return map[string]any{
		"id":                  ids.Format(row.ID),
		"code":                row.Code,
		"name":                row.Name,
		"type":                row.Type,
		"executor_ref":        row.ExecutorRef,
		"runtime_required":    row.RuntimeRequired,
		"default_timeout_sec": row.DefaultTimeoutSec,
		"resource_spec":       jsonx.ObjectMap(row.ResourceSpec),
		"selftest_status":     row.SelftestStatus,
		"selftest_detail":     jsonx.ObjectMap(row.SelftestDetail),
		"status":              row.Status,
	}
}

// fingerprintToMap 把提交指纹投影转换为 API 输出结构。
func fingerprintToMap(row SubmissionFingerprintSnapshot) map[string]any {
	return map[string]any{
		"id":           ids.Format(row.ID),
		"source_ref":   row.SourceRef,
		"problem_ref":  row.ProblemRef,
		"submitter_id": ids.Format(row.SubmitterID),
		"code_hash":    row.CodeHash,
		"created_at":   row.CreatedAt,
	}
}

// taskInfoFromTask 从任务投影构造 contracts 层任务摘要。
func taskInfoFromTask(row JudgeTaskSnapshot) contracts.JudgeTaskInfo {
	return contracts.JudgeTaskInfo{TaskID: row.ID, TenantID: row.TenantID, SourceRef: row.SourceRef, SubmitterID: row.SubmitterID, Status: row.Status}
}

// taskInfoToMap 转换 contracts DTO 为 HTTP 与服务输出结构。
func taskInfoToMap(info contracts.JudgeTaskInfo) map[string]any {
	return map[string]any{
		"task_id":    ids.Format(info.TaskID),
		"tenant_id":  ids.Format(info.TenantID),
		"source_ref": info.SourceRef,
		"status":     info.Status,
		"score":      info.Score,
		"passed":     info.Passed,
	}
}

// taskViewToMap 转换 M3 HTTP 任务视图,按接口文档在完成后嵌入 result。
func taskViewToMap(view judgeTaskView) map[string]any {
	out := taskInfoToMap(view.JudgeTaskInfo)
	if view.Result != nil {
		out["result"] = resultToMap(*view.Result)
	}
	return out
}

// resultToMap 转换 M3 判题结果视图为文档化 HTTP 输出。
func resultToMap(result judgeTaskResultView) map[string]any {
	out := map[string]any{
		"passed":       result.Passed,
		"score":        result.Score,
		"max_score":    result.MaxScore,
		"details":      result.Details,
		"judged_at":    result.JudgedAt,
		"is_rejudge":   result.IsRejudge,
		"snapshot_ref": "judge:task:" + ids.Format(result.TaskID),
	}
	if result.JudgeSandboxRef != "" {
		out["judge_sandbox_ref"] = result.JudgeSandboxRef
	}
	return out
}

// tasksToMaps 把任务投影列表转换为 API 输出结构。
func tasksToMaps(rows []JudgeTaskSnapshot) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, taskInfoToMap(taskInfoFromTask(row)))
	}
	return out
}
