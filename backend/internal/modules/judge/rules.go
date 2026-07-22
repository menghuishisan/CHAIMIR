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
