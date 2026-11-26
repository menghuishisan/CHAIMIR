package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bsm/redislock"
)

// ErrLeaseHeld 表示其他执行者持有租约；调用方可在自己的清理预算内等待。
var ErrLeaseHeld = redislock.ErrNotObtained

// WithLease 取得互斥执行权并续租到 fn 及其清理结束。已被占用时立即返回错误。
// 续租失败会取消 fn 的 context；调用方仍须隔离每次执行的资源,防止旧执行误删接管资源。
func (c *Client) WithLease(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) error) (resultErr error) {
	if c == nil || c.rdb == nil || strings.TrimSpace(key) == "" || ttl < 3*time.Millisecond || fn == nil {
		return fmt.Errorf("redis 租约参数不完整")
	}
	lock, err := redislock.Obtain(ctx, c.rdb, key, ttl, nil)
	if err != nil {
		return fmt.Errorf("获取执行租约失败: %w", err)
	}
	workCtx, cancelWork := context.WithCancelCause(ctx)
	// 连接取消后仍须续租直到独立清理完成,不能在清理期间放行下一个执行者。
	renewCtx, stopRenew := context.WithCancel(context.WithoutCancel(ctx))
	renewed := make(chan error, 1)
	interval := ttl / 3
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				renewed <- nil
				return
			case <-ticker.C:
				refreshCtx, cancel := context.WithTimeout(renewCtx, interval)
				err := lock.Refresh(refreshCtx, ttl, nil)
				cancel()
				if err != nil {
					if renewCtx.Err() != nil {
						renewed <- nil
						return
					}
					err = fmt.Errorf("执行租约续期失败: %w", err)
					cancelWork(err)
					renewed <- err
					return
				}
			}
		}
	}()
	defer func() {
		stopRenew()
		resultErr = errors.Join(resultErr, <-renewed)
		cancelWork(nil)
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), interval)
		defer cancel()
		if err := lock.Release(releaseCtx); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("释放执行租约失败: %w", err))
		}
	}()
	return fn(workCtx)
}
