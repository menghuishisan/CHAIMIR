// db read_tx 仅为显式只读契约复用同租户应用事务,避免持锁时再次租借连接。
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type transactionContextKey struct{}

type maintenanceTransaction struct{}

type transactionContext struct {
	database      *DB
	tenantID      int64
	tx            pgx.Tx
	notifications *[]func(context.Context) error
}

// WithTenantReadTxID 供只执行读取查询的仓储使用;同一应用事务内不另占连接或提交。
func (d *DB) WithTenantReadTxID(ctx context.Context, tenantID int64, fn TxFunc) error {
	if d == nil || tenantID <= 0 || fn == nil {
		return fmt.Errorf("租户只读事务参数不完整")
	}
	if current, ok := ctx.Value(transactionContextKey{}).(transactionContext); ok && current.tenantID != 0 {
		if current.database != d || current.tenantID != tenantID || current.tx == nil {
			return fmt.Errorf("只读事务不可跨数据库或租户复用")
		}
		return fn(ctx, current.tx)
	}
	return d.WithTenantTxID(ctx, tenantID, fn)
}

// WithTenantTxIfPresent 只在调用方已开启同数据库、同租户事务时执行 fn。
func (d *DB) WithTenantTxIfPresent(ctx context.Context, tenantID int64, fn TxFunc) (bool, error) {
	if d == nil || ctx == nil || tenantID <= 0 || fn == nil {
		return false, fmt.Errorf("租户事务复用参数不完整")
	}
	current, ok := ctx.Value(transactionContextKey{}).(transactionContext)
	if !ok {
		return false, nil
	}
	if current.database != d || current.tenantID != tenantID || current.tx == nil {
		return false, fmt.Errorf("租户事务不可跨数据库或租户复用")
	}
	return true, fn(ctx, current.tx)
}

// WithPlatformTxIfPresent 只在调用方已开启同数据库平台事务时执行 fn。
func (d *DB) WithPlatformTxIfPresent(ctx context.Context, fn TxFunc) (bool, error) {
	if d == nil || ctx == nil || fn == nil {
		return false, fmt.Errorf("平台事务复用参数不完整")
	}
	current, ok := ctx.Value(transactionContextKey{}).(transactionContext)
	if !ok {
		return false, nil
	}
	if current.database != d || current.tenantID != 0 || current.tx == nil {
		return false, fmt.Errorf("平台事务不可跨数据库或租户复用")
	}
	return true, fn(ctx, current.tx)
}
