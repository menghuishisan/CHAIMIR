// M3 错误码测试:确保评测引擎错误码覆盖判题器、任务与查重细分场景。
package apperr

import "testing"

// TestJudgeErrorCodesAreUniqueAndSegmented 确认 M3 错误码不重复且落在 3xxxx 段。
func TestJudgeErrorCodesAreUniqueAndSegmented(t *testing.T) {
	codes := []*Error{
		ErrJudgerNotFound,
		ErrJudgerUnavailable,
		ErrJudgerInvalid,
		ErrJudgerSelftestFailed,
		ErrJudgerPersistence,
		ErrJudgeTaskNotFound,
		ErrJudgeTaskInvalid,
		ErrJudgeTaskQueuedFail,
		ErrJudgeTaskRunFail,
		ErrJudgeTaskTimeout,
		ErrJudgeTaskRateLimited,
		ErrJudgeTaskInvalidState,
		ErrJudgeManualScoreInvalid,
		ErrJudgeConfigUnavailable,
		ErrJudgeTaskPersistence,
		ErrJudgeEventPublish,
		ErrJudgeAuditFail,
		ErrJudgeInputArchiveInvalid,
		ErrFingerprintNotFound,
		ErrFingerprintInvalid,
		ErrSimilarityFailed,
	}
	seen := map[string]bool{}
	for _, item := range codes {
		if item.Code == "" || item.Code[0] != '3' {
			t.Fatalf("judge code must be in 3xxxx segment, got %q", item.Code)
		}
		if seen[item.Code] {
			t.Fatalf("duplicate judge code %s", item.Code)
		}
		seen[item.Code] = true
	}
}
