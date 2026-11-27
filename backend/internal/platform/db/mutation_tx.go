package db

import (
	"context"
	"fmt"
	"log/slog"

	"chaimir/pkg/logging"

	"github.com/jackc/pgx/v5"
)

// WithTenantMutationTxID 显式复用同归属普通事务；无外层事务时自行提交。
// 借用时通知由持有者成功提交后执行，故障记录且不伪装为事务回滚；独立调用返回通知错误。
func (d *DB) WithTenantMutationTxID(ctx context.Context, tenantID int64, fn TxFunc, notify func(context.Context) error) error {
	if fn == nil || notify == nil {
		return fmt.Errorf("租户写事务缺少操作或提交通知")
	}
	joined, err := d.WithTenantTxIfPresent(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		current := ctx.Value(transactionContextKey{}).(transactionContext)
		if current.notifications == nil {
			return fmt.Errorf("调用方事务缺少提交通知队列")
		}
		if err := fn(ctx, tx); err != nil {
			return err
		}
		*current.notifications = append(*current.notifications, notify)
		return nil
	})
	if joined || err != nil {
		return err
	}
	if err := d.WithTenantTxID(ctx, tenantID, fn); err != nil {
		return err
	}
	return notify(notificationContext(ctx))
}

func notificationContext(ctx context.Context) context.Context {
	return context.WithValue(context.WithoutCancel(ctx), transactionContextKey{}, false)
}

func notifyTenantCommit(ctx context.Context, tenantID int64, notifications []func(context.Context) error) {
	ctx = notificationContext(ctx)
	for _, notify := range notifications {
		if err := notify(ctx); err != nil {
			logging.ErrorContext(ctx, "tenant transaction committed but notification failed", err.Error(), slog.Int64("tenant_id", tenantID))
		}
	}
}
