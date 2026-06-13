// judge row_convert 文件负责 sqlc 行到 M3 领域模型的纯转换。
package judge

import (
	"encoding/json"

	"chaimir/internal/modules/judge/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// judgerFromRow 转换平台级判题器定义。
func judgerFromRow(row sqlcgen.Judger) (Judger, error) {
	spec, err := decodeResourceSpec(row.ResourceSpec, row.Type, row.RuntimeRequired)
	if err != nil {
		return Judger{}, err
	}
	return Judger{
		ID:                row.ID,
		Code:              row.Code,
		Name:              row.Name,
		Type:              row.Type,
		ExecutorRef:       row.ExecutorRef,
		RuntimeRequired:   row.RuntimeRequired,
		DefaultTimeoutSec: row.DefaultTimeoutSec,
		ResourceSpec:      spec,
		SelftestStatus:    row.SelftestStatus,
		Status:            row.Status,
		CreatedAt:         timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:         timex.FromTimestamptz(row.UpdatedAt),
	}, nil
}

// taskFromRow 转换任务行并解析输入快照。
func taskFromRow(row sqlcgen.JudgeTask) (JudgeTask, error) {
	snapshot, err := decodeSnapshot(row.InputSnapshot)
	if err != nil {
		return JudgeTask{}, err
	}
	return JudgeTask{
		ID:               row.ID,
		TenantID:         row.TenantID,
		JudgerID:         row.JudgerID,
		SourceRef:        row.SourceRef,
		SubmitterID:      row.SubmitterID,
		ProblemRef:       row.ProblemRef,
		CodeStorageKey:   row.CodeStorageKey,
		CodeHash:         row.CodeHash,
		InputSnapshot:    snapshot,
		SandboxMode:      row.SandboxMode,
		TargetSandboxRef: pgtypex.TextValue(row.TargetSandboxRef),
		Priority:         row.Priority,
		Status:           row.Status,
		RetryCount:       row.RetryCount,
		MaxRetries:       row.MaxRetries,
		LastError:        pgtypex.TextValue(row.LastError),
		CreatedAt:        timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:        timex.FromTimestamptz(row.UpdatedAt),
	}, nil
}

// taskInfoFromJoined 转换带可选结果的查询行。
func taskInfoFromJoined(row sqlcgen.GetJudgeTaskWithResultRow) (JudgeTaskInfo, error) {
	task, err := taskFromRow(sqlcgen.JudgeTask{
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
		TargetSandboxRef: row.TargetSandboxRef,
		Priority:         row.Priority,
		Status:           row.Status,
		RetryCount:       row.RetryCount,
		MaxRetries:       row.MaxRetries,
		LastError:        row.LastError,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	})
	if err != nil {
		return JudgeTaskInfo{}, err
	}
	info := JudgeTaskInfo{Task: task}
	if row.Passed.Valid {
		details, err := decodeDetails(row.Details)
		if err != nil {
			return JudgeTaskInfo{}, err
		}
		info.Result = &JudgeResult{
			TaskID:          row.ID,
			TenantID:        row.TenantID,
			Passed:          row.Passed.Bool,
			Score:           row.Score.Int32,
			MaxScore:        row.MaxScore.Int32,
			Details:         details,
			JudgeSandboxRef: pgtypex.TextValue(row.JudgeSandboxRef),
			JudgedAt:        timex.FromTimestamptz(row.JudgedAt),
			IsRejudge:       row.IsRejudge.Bool,
		}
	}
	return info, nil
}

// taskInfosFromRows 批量转换列表查询。
func taskInfosFromRows(rows []sqlcgen.ListJudgeTasksRow) ([]JudgeTaskInfo, error) {
	out := make([]JudgeTaskInfo, 0, len(rows))
	for _, row := range rows {
		info, err := taskInfoFromJoined(sqlcgen.GetJudgeTaskWithResultRow(row))
		if err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}

// fingerprintFromRow 转换查重特征行。
func fingerprintFromRow(row sqlcgen.SubmissionFingerprint) (SubmissionFingerprint, error) {
	vector := map[string]float64{}
	if len(row.SimVector) > 0 {
		if err := jsonx.DecodeStrict(row.SimVector, &vector); err != nil {
			return SubmissionFingerprint{}, err
		}
	}
	return SubmissionFingerprint{
		ID:          row.ID,
		TenantID:    row.TenantID,
		SourceRef:   row.SourceRef,
		ProblemRef:  row.ProblemRef,
		SubmitterID: row.SubmitterID,
		CodeHash:    row.CodeHash,
		SimVector:   vector,
		CreatedAt:   timex.FromTimestamptz(row.CreatedAt),
	}, nil
}

// outboxFromRow 转换终态事件 outbox。
func outboxFromRow(row sqlcgen.JudgeEventOutbox) JudgeEventOutbox {
	return JudgeEventOutbox{
		ID:         row.ID,
		TenantID:   row.TenantID,
		TaskID:     row.TaskID,
		Subject:    row.Subject,
		Payload:    row.Payload,
		Status:     row.Status,
		RetryCount: row.RetryCount,
		LastError:  pgtypex.TextValue(row.LastError),
		CreatedAt:  timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:  timex.FromTimestamptz(row.UpdatedAt),
	}
}

// decodeResourceSpec 解码数据库中的判题器资源配置。
func decodeResourceSpec(raw []byte, typ int16, runtimeRequired bool) (JudgerResourceSpec, error) {
	return parseJudgerResourceSpec(json.RawMessage(raw), typ, runtimeRequired)
}

// decodeSnapshot 解码任务输入快照。
func decodeSnapshot(raw []byte) (JudgeInputSnapshot, error) {
	var out JudgeInputSnapshot
	if len(raw) == 0 {
		return out, nil
	}
	if err := jsonx.DecodeStrict(raw, &out); err != nil {
		return JudgeInputSnapshot{}, err
	}
	return out, nil
}

// decodeDetails 解码判题结果详情。
func decodeDetails(raw []byte) ([]JudgeResultDetail, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out []JudgeResultDetail
	if err := jsonx.DecodeStrict(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
