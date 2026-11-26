// 记录受控备份结果，供平台管理员查询。
package admin

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/platform/pagex"
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
	if s == nil || s.store == nil || s.ids == nil || req.Type != BackupTypeFull || req.SizeBytes < 0 || req.StartedAt.IsZero() || req.FinishedAt.IsZero() || req.FinishedAt.Before(req.StartedAt) {
		return fmt.Errorf("备份记录请求不完整")
	}
	switch req.Status {
	case BackupStatusSucceeded:
		if strings.TrimSpace(req.StorageRef) == "" {
			return fmt.Errorf("成功备份缺少清单引用")
		}
	case BackupStatusFailed:
		if req.StorageRef != "" {
			return fmt.Errorf("失败备份不能引用未完成产物")
		}
	default:
		return fmt.Errorf("备份结果状态无效")
	}
	return s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.CreateBackupRecord(ctx, s.ids.Generate(), req)
		return err
	})
}

// ListBackups 查询备份记录;status 传 0 表示不按结果过滤。
func (s *Service) ListBackups(ctx context.Context, status int16, page, size int) ([]BackupRecordDTO, int64, int, int, pagex.Facets, error) {
	if _, err := requirePlatform(ctx); err != nil {
		return nil, 0, page, size, nil, err
	}
	page, size = pagex.Normalize(page, size)
	var total int64
	var byStatus pagex.FacetGroup
	rows, err := runAdminRead(ctx, s.store, 0, func(ctx context.Context, tx TxStore) ([]BackupRecordDTO, error) {
		out, count, err := tx.ListBackupRecords(ctx, status, page, size)
		if err != nil {
			return nil, err
		}
		total = count
		byStatus, err = tx.CountBackupRecordsByStatus(ctx)
		return out, err
	})
	return rows, total, page, size, pagex.Facets{"status": byStatus}, err
}
