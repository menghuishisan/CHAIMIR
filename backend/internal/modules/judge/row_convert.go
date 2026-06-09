// M3 行转换层:集中处理 sqlc 行到领域投影和响应 DTO 的纯转换。
package judge

import (
	"chaimir/internal/contracts"
	"chaimir/internal/modules/judge/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"time"
)

// judgerSnapshotFromRow 把判题器表行转换为 service/worker 使用的业务投影。
func judgerSnapshotFromRow(row sqlcgen.Judger) JudgerSnapshot {
	return JudgerSnapshot{
		ID:                row.ID,
		Code:              row.Code,
		Name:              row.Name,
		Type:              row.Type,
		ExecutorRef:       row.ExecutorRef,
		RuntimeRequired:   row.RuntimeRequired,
		DefaultTimeoutSec: row.DefaultTimeoutSec,
		ResourceSpec:      row.ResourceSpec,
		SelftestStatus:    row.SelftestStatus,
		SelftestDetail:    row.SelftestDetail,
		Status:            row.Status,
		UpdatedAtText:     row.UpdatedAt.Time.UTC().Format(time.RFC3339Nano),
	}
}

// judgeTaskSnapshotFromRow 把判题任务表行转换为状态机使用的业务投影。
func judgeTaskSnapshotFromRow(row sqlcgen.JudgeTask) JudgeTaskSnapshot {
	return JudgeTaskSnapshot{
		ID:               row.ID,
		TenantID:         row.TenantID,
		JudgerID:         row.JudgerID,
		SourceRef:        row.SourceRef,
		SubmitterID:      row.SubmitterID,
		ProblemRef:       row.ProblemRef,
		CodeStorageKey:   row.CodeStorageKey,
		CodeHash:         row.CodeHash,
		InputSnapshot:    row.InputSnapshot,
		SandboxMode:      row.SandboxMode,
		TargetSandboxRef: row.TargetSandboxRef.String,
		Priority:         row.Priority,
		Status:           row.Status,
		RetryCount:       row.RetryCount,
		MaxRetries:       row.MaxRetries,
		CreatedAtUnixMs:  row.CreatedAt.Time.UnixNano() / 1_000_000,
	}
}

// fingerprintSnapshotFromRow 把查重表行转换为查重输出投影。
func fingerprintSnapshotFromRow(row sqlcgen.SubmissionFingerprint) SubmissionFingerprintSnapshot {
	return SubmissionFingerprintSnapshot{
		ID:          row.ID,
		SourceRef:   row.SourceRef,
		ProblemRef:  row.ProblemRef,
		SubmitterID: row.SubmitterID,
		CodeHash:    row.CodeHash,
		SimVector:   row.SimVector,
		CreatedAt:   timex.FromTimestamptz(row.CreatedAt),
	}
}

// outboxSnapshotFromRow 把终态事件 outbox 表行转换为发布投影。
func outboxSnapshotFromRow(row sqlcgen.JudgeEventOutbox) JudgeOutboxSnapshot {
	return JudgeOutboxSnapshot{
		ID:        row.ID,
		TenantID:  row.TenantID,
		TaskID:    row.TaskID,
		Subject:   row.Subject,
		Payload:   row.Payload,
		LastError: row.LastError.String,
	}
}

// taskViewFromJoined 从任务与结果联查行构造 M3 HTTP 输出视图。
func taskViewFromJoined(row sqlcgen.GetJudgeTaskWithResultRow) judgeTaskView {
	view := judgeTaskView{JudgeTaskInfo: taskInfoFromJoined(row)}
	if result, ok := taskResultFromJoined(row); ok {
		view.Result = &result
	}
	return view
}

// taskInfoFromJoined 从任务与结果联查行构造 contracts 层任务摘要。
func taskInfoFromJoined(row sqlcgen.GetJudgeTaskWithResultRow) contracts.JudgeTaskInfo {
	info := contracts.JudgeTaskInfo{TaskID: row.ID, TenantID: row.TenantID, SourceRef: row.SourceRef, SubmitterID: row.SubmitterID, Status: row.Status}
	if row.ResultScore.Valid {
		info.Score = row.ResultScore.Int32
	}
	if row.ResultPassed.Valid {
		info.Passed = row.ResultPassed.Bool
	}
	return info
}

// taskResultFromJoined 从联查行构造 HTTP 结果视图,仅在存在结果时返回。
func taskResultFromJoined(row sqlcgen.GetJudgeTaskWithResultRow) (judgeTaskResultView, bool) {
	if !row.ResultScore.Valid || !row.ResultMaxScore.Valid || !row.ResultPassed.Valid {
		return judgeTaskResultView{}, false
	}
	details := any(nil)
	if len(row.ResultDetails) > 0 {
		details = jsonx.Decode(row.ResultDetails, any(nil))
	}
	return judgeTaskResultView{
		TaskID:          row.ID,
		Passed:          row.ResultPassed.Bool,
		Score:           row.ResultScore.Int32,
		MaxScore:        row.ResultMaxScore.Int32,
		Details:         details,
		JudgedAt:        timex.FromTimestamptz(row.ResultJudgedAt),
		IsRejudge:       row.ResultIsRejudge.Bool,
		JudgeSandboxRef: row.ResultJudgeSandboxRef.String,
	}, true
}
