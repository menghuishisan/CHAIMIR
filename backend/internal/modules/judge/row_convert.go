// judge row_convert 文件负责 sqlc 行到 M3 领域模型的纯转换。
package judge

import (
	"fmt"
	"reflect"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/judge/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"

	"github.com/jackc/pgx/v5/pgtype"
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
		ID:                row.ID,
		TenantID:          row.TenantID,
		JudgerID:          row.JudgerID,
		SourceRef:         row.SourceRef,
		SourceOwnerID:     row.SourceOwnerID,
		SourceCourseID:    row.SourceCourseID,
		SourceScope:       row.SourceScope,
		SubmitterTenantID: row.SubmitterTenantID,
		SubmitterID:       row.SubmitterID,
		ProblemRef:        row.ProblemRef,
		CodeStorageKey:    row.CodeStorageKey,
		CodeHash:          row.CodeHash,
		InputSnapshot:     snapshot,
		SandboxMode:       row.SandboxMode,
		TargetSandboxRef:  pgtypex.TextValue(row.TargetSandboxRef),
		Priority:          row.Priority,
		Status:            row.Status,
		RetryCount:        row.RetryCount,
		MaxRetries:        row.MaxRetries,
		LastError:         pgtypex.TextValue(row.LastError),
		CreatedAt:         timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:         timex.FromTimestamptz(row.UpdatedAt),
		LeaseToken:        row.LeaseToken,
		LeaseUntil:        timex.FromTimestamptz(row.LeaseUntil),
	}, nil
}

// taskFromDatabaseRow 将 sqlc 为每条查询生成的同构行统一投影为领域任务。
// sqlc 的查询行类型各自独立，反射只在数据库边界做一次字段投影，避免业务层携带生成类型差异。
func taskFromDatabaseRow(row any) (JudgeTask, error) {
	v := reflect.ValueOf(row)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return JudgeTask{}, fmt.Errorf("judge task row is nil")
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return JudgeTask{}, fmt.Errorf("judge task row must be struct, got %s", v.Kind())
	}
	var out sqlcgen.JudgeTask
	dst := reflect.ValueOf(&out).Elem()
	for i := 0; i < dst.NumField(); i++ {
		field := dst.Type().Field(i)
		src := v.FieldByName(field.Name)
		if !src.IsValid() || !src.Type().AssignableTo(dst.Field(i).Type()) {
			return JudgeTask{}, fmt.Errorf("judge task row missing compatible field %s", field.Name)
		}
		dst.Field(i).Set(src)
	}
	return taskFromRow(out)
}

// taskFromCreateRow 转换幂等创建 CTE 返回行,集中复用任务行解析规则。
func taskFromCreateRow(row sqlcgen.CreateJudgeTaskRow) (JudgeTask, error) {
	return taskFromDatabaseRow(row)
}

// taskInfoFromJoined 转换带可选结果的查询行。
func taskInfoFromJoined(row sqlcgen.GetJudgeTaskWithResultRow) (JudgeTaskInfo, error) {
	task, err := taskFromRow(sqlcgen.JudgeTask{
		ID:                row.ID,
		TenantID:          row.TenantID,
		JudgerID:          row.JudgerID,
		SourceRef:         row.SourceRef,
		SourceOwnerID:     row.SourceOwnerID,
		SourceCourseID:    row.SourceCourseID,
		SourceScope:       row.SourceScope,
		SubmitterTenantID: row.SubmitterTenantID,
		SubmitterID:       row.SubmitterID,
		ProblemRef:        row.ProblemRef,
		CodeStorageKey:    row.CodeStorageKey,
		CodeHash:          row.CodeHash,
		InputSnapshot:     row.InputSnapshot,
		SandboxMode:       row.SandboxMode,
		TargetSandboxRef:  row.TargetSandboxRef,
		Priority:          row.Priority,
		Status:            row.Status,
		RetryCount:        row.RetryCount,
		MaxRetries:        row.MaxRetries,
		LastError:         row.LastError,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
		LeaseToken:        row.LeaseToken,
		LeaseUntil:        row.LeaseUntil,
	})
	if err != nil {
		return JudgeTaskInfo{}, err
	}
	info := JudgeTaskInfo{Task: task}
	if row.ResultID > 0 {
		details, err := decodeDetails(row.Details)
		if err != nil {
			return JudgeTaskInfo{}, err
		}
		replay, err := decodeReplayTrace(row.ReplayTrace)
		if err != nil {
			return JudgeTaskInfo{}, err
		}
		info.Result = &JudgeResult{
			ID:              row.ResultID,
			TaskID:          row.ID,
			TenantID:        row.TenantID,
			Version:         row.ResultVersion,
			Passed:          row.Passed,
			Score:           row.Score,
			MaxScore:        row.MaxScore,
			Details:         details,
			Replay:          replay,
			JudgeSandboxRef: row.JudgeSandboxRef,
			JudgedAt:        timex.FromTimestamptz(row.JudgedAt),
			IsRejudge:       row.IsRejudge,
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
		ID:                row.ID,
		TenantID:          row.TenantID,
		SourceRef:         row.SourceRef,
		ProblemRef:        row.ProblemRef,
		SubmitterTenantID: row.SubmitterTenantID,
		SubmitterID:       row.SubmitterID,
		CodeHash:          row.CodeHash,
		SimVector:         vector,
		CreatedAt:         timex.FromTimestamptz(row.CreatedAt),
	}, nil
}

// fingerprintFromDatabaseRow 统一转换 sqlc 查询生成的指纹行类型。
func fingerprintFromDatabaseRow(row any) (SubmissionFingerprint, error) {
	v := reflect.ValueOf(row)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return SubmissionFingerprint{}, fmt.Errorf("submission fingerprint row is nil")
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return SubmissionFingerprint{}, fmt.Errorf("submission fingerprint row must be struct, got %s", v.Kind())
	}
	var out sqlcgen.SubmissionFingerprint
	dst := reflect.ValueOf(&out).Elem()
	for i := 0; i < dst.NumField(); i++ {
		field := dst.Type().Field(i)
		src := v.FieldByName(field.Name)
		if !src.IsValid() || !src.Type().AssignableTo(dst.Field(i).Type()) {
			return SubmissionFingerprint{}, fmt.Errorf("submission fingerprint row missing compatible field %s", field.Name)
		}
		dst.Field(i).Set(src)
	}
	return fingerprintFromRow(out)
}

// outboxFromRow 转换 sqlc outbox 模型,避免 service 直接依赖生成类型。
func outboxFromRow(row sqlcgen.JudgeEventOutbox) JudgeEventOutbox {
	return outboxFromFields(row.ID, row.TenantID, row.TaskID, row.Subject, row.Payload, row.Status, row.RetryCount, row.NextAttemptAt, row.LastError, row.CreatedAt, row.UpdatedAt, row.LeaseUntil, row.LeaseToken)
}

// outboxFromFields 汇总 outbox 行字段,避免业务层依赖 sqlc 临时类型。
func outboxFromFields(id, tenantID, taskID int64, subject string, payload []byte, status int16, retryCount int32, nextAttemptAt pgtype.Timestamptz, lastError pgtype.Text, createdAt, updatedAt, leaseUntil pgtype.Timestamptz, leaseToken string) JudgeEventOutbox {
	return JudgeEventOutbox{
		ID:            id,
		TenantID:      tenantID,
		TaskID:        taskID,
		Subject:       subject,
		Payload:       payload,
		Status:        status,
		RetryCount:    retryCount,
		NextAttemptAt: timex.FromTimestamptz(nextAttemptAt),
		LastError:     pgtypex.TextValue(lastError),
		CreatedAt:     timex.FromTimestamptz(createdAt),
		UpdatedAt:     timex.FromTimestamptz(updatedAt),
		LeaseToken:    leaseToken,
		LeaseUntil:    timex.FromTimestamptz(leaseUntil),
	}
}

// decodeResourceSpec 解码数据库中的判题器资源配置。
func decodeResourceSpec(raw []byte, typ int16, runtimeRequired bool) (JudgerResourceSpec, error) {
	return parseJudgerResourceSpec(jsonx.RawMessage(raw), typ, runtimeRequired)
}

// decodeSnapshot 解码任务输入快照。
func decodeSnapshot(raw []byte) (JudgeInputSnapshot, error) {
	var out JudgeInputSnapshot
	if len(raw) == 0 {
		return out, nil
	}
	if err := jsonx.DecodeStrictKnownFields(raw, &out); err != nil {
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
	if err := jsonx.DecodeStrictKnownFields(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// decodeReplayTrace 解码已持久化的链上回放轨迹,拒绝损坏数据继续进入跨模块结算。
func decodeReplayTrace(raw []byte) (contracts.JudgeReplayTrace, error) {
	if len(raw) == 0 {
		return contracts.JudgeReplayTrace{Actions: []contracts.JudgeReplayAction{}}, nil
	}
	var out contracts.JudgeReplayTrace
	if err := jsonx.DecodeStrictKnownFields(raw, &out); err != nil {
		return contracts.JudgeReplayTrace{}, err
	}
	if out.Actions == nil {
		out.Actions = []contracts.JudgeReplayAction{}
	}
	return out, nil
}
