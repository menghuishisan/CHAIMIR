// judge rules 文件定义纯输入校验、状态机和脱敏安全规则,不访问 repo/db/contracts。
package judge

import (
	"regexp"
	"strings"

	"chaimir/pkg/apperr"
	"chaimir/pkg/privacy"
)

var (
	codePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)
)

// validTaskState 判断任务列表的运维分组取值是否受支持。
// 分组语义与 db/queries/judge.sql 里 ListJudgeTasks/CountJudgeTasks 的 state 条件一一对应,
// 两处必须同时改,否则指标带与列表会给出不同口径。
func validTaskState(state string) bool {
	switch state {
	case TaskStateAll, TaskStateActive, TaskStateAbnormal:
		return true
	default:
		return false
	}
}

// validateManualScore 校验人工评分不会超过分值边界。
func validateManualScore(req ManualScoreRequest) error {
	if req.MaxScore <= 0 || req.Score < 0 || req.Score > req.MaxScore || strings.TrimSpace(req.Comment) == "" {
		return apperr.ErrJudgeManualScoreInvalid
	}
	return nil
}

// validateResultDetails 校验可解释结果存在且不包含明显敏感字段。
func validateResultDetails(details []JudgeResultDetail) error {
	if len(details) == 0 {
		return apperr.ErrJudgeWorkerFailed
	}
	for _, item := range details {
		if strings.TrimSpace(item.Case) == "" && strings.TrimSpace(item.Source) == "" && strings.TrimSpace(item.Target) == "" {
			return apperr.ErrJudgeWorkerFailed
		}
		if containsSensitiveMaterial(item.ExpectedLabel) || containsSensitiveMaterial(item.Actual) || containsSensitiveMaterial(item.Hint) {
			return apperr.ErrJudgeWorkerFailed
		}
	}
	return nil
}

// containsSensitiveMaterial 以保守关键词防止答案、flag、私钥等进入学生可见结果。
func containsSensitiveMaterial(value string) bool {
	return privacy.ContainsResultSensitiveText(value)
}

// sanitizeReplayMap 递归过滤答案、凭据和内部字段,保留链交易复现所需的公开参数。
func sanitizeReplayMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		if key == "" || strings.HasPrefix(key, "_") || privacy.IsResultSensitiveKey(key) {
			continue
		}
		if clean, ok := sanitizeReplayValue(value); ok {
			out[key] = clean
		}
	}
	return out
}

// sanitizeReplayValue 只保留 JSON 基础类型,避免把运行时对象或未校验指针写入回放。
func sanitizeReplayValue(value any) (any, bool) {
	switch typed := value.(type) {
	case nil, bool, float64, int, int32, int64, string:
		return typed, true
	case map[string]any:
		return sanitizeReplayMap(typed), true
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			clean, ok := sanitizeReplayValue(item)
			if !ok {
				return nil, false
			}
			out = append(out, clean)
		}
		return out, true
	default:
		return nil, false
	}
}

// statusText 返回 API 用户向状态字符串。
func statusText(status int16) string {
	switch status {
	case JudgeTaskStatusQueued:
		return "queued"
	case JudgeTaskStatusJudging:
		return "judging"
	case JudgeTaskStatusDone:
		return "done"
	case JudgeTaskStatusTimeout:
		return "timeout"
	case JudgeTaskStatusFailed:
		return "failed"
	case JudgeTaskStatusError:
		return "error"
	case JudgeTaskStatusCancelled:
		return "cancelled"
	default:
		return "error"
	}
}
