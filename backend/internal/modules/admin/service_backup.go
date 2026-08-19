package admin

import (
	"context"
	"fmt"

	"chaimir/pkg/snowflake"
)

// BackupRecorder 是受控运维任务写入 M9 备份结果的窄服务入口。
// 组合根只依赖该入口,不直接装配或调用 M9 repo。
type BackupRecorder struct {
	store Store
	ids   snowflake.Generator
}

// NewBackupRecorder 构造 cron 可用的备份结果服务。
func NewBackupRecorder(store Store, ids snowflake.Generator) (*BackupRecorder, error) {
	if store == nil || ids == nil {
		return nil, fmt.Errorf("备份记录服务依赖不完整")
	}
	return &BackupRecorder{store: store, ids: ids}, nil
}

// RecordResult 在 M9 自有平台事务中记录一次备份结果。
func (s *BackupRecorder) RecordResult(ctx context.Context, req BackupRecordCreate) error {
	if s == nil || s.store == nil || s.ids == nil || req.StorageRef == "" {
		return fmt.Errorf("备份记录请求不完整")
	}
	return s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.CreateBackupRecord(ctx, s.ids.Generate(), req)
		return err
	})
}
