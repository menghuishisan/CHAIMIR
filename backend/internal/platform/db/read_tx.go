// db read_tx 仅为显式只读契约复用同租户应用事务,避免持锁时再次租借连接。
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type tenantReadTxKey struct{}

// tenantReadTx 是基础层私有的只读借用能力,不穿过contracts或特权事务边界。
type tenantReadTx struct {
	database *DB
	tenantID int64
	tx       pgx.Tx
}

// WithTenantReadTxID 供只执行读取查询的仓储使用;同一应用事务内不另占连接或提交。
func (d *DB) WithTenantReadTxID(ctx context.Context, tenantID int64, fn TxFunc) error {
	if d == nil || tenantID <= 0 || fn == nil {
		return fmt.Errorf("租户只读事务参数不完整")
	}
	if borrowed, ok := ctx.Value(tenantReadTxKey{}).(tenantReadTx); ok {
		if borrowed.database != d || borrowed.tenantID != tenantID || borrowed.tx == nil {
			return fmt.Errorf("只读事务不可跨数据库或租户复用")
		}
		return fn(ctx, borrowed.tx)
	}
	return d.WithTenantTxID(ctx, tenantID, fn)
}
