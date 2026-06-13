// transfer row_convert 文件负责统一导入导出任务的 sqlc 行模型转换。
package transfer

import (
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/transfer/internal/sqlcgen"
)

// taskFromRow 把 transfer_task 数据库行转换为基础层任务快照。
func taskFromRow(row sqlcgen.TransferTask) Task {
	return Task{
		TaskID:           row.ID,
		TenantID:         row.TenantID,
		AccountID:        row.AccountID,
		Channel:          Channel(row.Channel),
		Subject:          row.Subject,
		Status:           Status(row.Status),
		ContentType:      row.ContentType,
		FileName:         row.FileName,
		AttemptCount:     int(row.AttemptCount),
		MaxAttempts:      int(row.MaxAttempts),
		LastError:        row.LastError,
		Artifact:         Artifact{ObjectRef: row.ArtifactRef, Size: row.ArtifactSize, ContentType: row.ArtifactContentType, FileName: row.ArtifactFileName},
		CreatedAt:        timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:        timex.FromTimestamptz(row.UpdatedAt),
		CompletedAt:      timex.FromTimestamptz(row.CompletedAt),
		NextAttemptAfter: timex.FromTimestamptz(row.NextAttemptAfter),
	}
}

// tasksFromRows 批量转换 transfer_task 行结果。
func tasksFromRows(rows []sqlcgen.TransferTask) []Task {
	out := make([]Task, 0, len(rows))
	for _, row := range rows {
		out = append(out, taskFromRow(row))
	}
	return out
}
