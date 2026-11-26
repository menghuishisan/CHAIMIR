// 保存并查询受控运维任务的备份结果。
package admin

import (
	"context"

	"chaimir/internal/modules/admin/internal/sqlcgen"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/timex"
)

// CreateBackupRecord 写入真实运维备份执行结果。
func (t *txStore) CreateBackupRecord(ctx context.Context, id int64, req BackupRecordCreate) (BackupRecordDTO, error) {
	row, err := t.q.CreateBackupRecord(ctx, sqlcgen.CreateBackupRecordParams{ID: id, Type: req.Type, StorageRef: req.StorageRef, SizeBytes: req.SizeBytes, Status: req.Status, StartedAt: timex.RequiredTimestamptz(req.StartedAt), FinishedAt: timex.RequiredTimestamptz(req.FinishedAt)})
	if err != nil {
		return BackupRecordDTO{}, err
	}
	return backupDTO(row), nil
}

// CountBackupRecordsByStatus 按状态聚合全部备份记录。
func (t *txStore) CountBackupRecordsByStatus(ctx context.Context) (pagex.FacetGroup, error) {
	rows, err := t.q.CountBackupRecordsByStatus(ctx)
	if err != nil {
		return nil, err
	}
	group := pagex.FacetGroup{}
	for _, row := range rows {
		group.Add(row.Status, row.Count)
	}
	return group, nil
}

// ListBackupRecords 查询备份记录和总数。
// status 传 0 表示不按结果过滤;总数按同一条件计,与列表同口径。
func (t *txStore) ListBackupRecords(ctx context.Context, status int16, page, size int) ([]BackupRecordDTO, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListBackupRecords(ctx, sqlcgen.ListBackupRecordsParams{Status: status, PageLimit: limit, PageOffset: offset})
	if err != nil {
		return nil, 0, err
	}
	out := make([]BackupRecordDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, backupDTO(row))
	}
	total, err := t.q.CountBackupRecords(ctx, status)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
