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

// ErrLeaseNotHeld 表示原持有者的租约已过期或被接管，不能再释放。
var ErrLeaseNotHeld = redislock.ErrLockNotHeld

// ObtainLease 取得带持有者标识的限时租约；成功路径可让它自然到期，失败时按原标识释放。
func (c *Client) ObtainLease(ctx context.Context, key string, ttl time.Duration) (*redislock.Lock, error) {
	if c == nil || c.rdb == nil || strings.TrimSpace(key) == "" || ttl < 3*time.Millisecond {
		return nil, fmt.Errorf("redis 租约参数不完整")
	}
	return redislock.Obtain(ctx, c.rdb, key, ttl, nil)
}

// WithLease 取得互斥执行权并续租到 fn 及其清理结束。已被占用时立即返回错误。
// 续租失败会取消 fn 的 context；调用方仍须隔离每次执行的资源,防止旧执行误删接管资源。
func (c *Client) WithLease(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) error) (resultErr error) {
	if fn == nil {
		return fmt.Errorf("redis 租约参数不完整")
	}
	started := time.Now()
	acquireCtx, cancelAcquire := context.WithTimeout(ctx, ttl)
	lock, err := c.ObtainLease(acquireCtx, key, ttl)
	if err == nil {
		err = acquireCtx.Err()
	}
	cancelAcquire()
	if lock == nil {
		return fmt.Errorf("获取执行租约失败: %w", err)
	}
	interval := ttl / 3
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), interval)
		defer cancel()
		if err := lock.Release(releaseCtx); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("释放执行租约失败: %w", err))
		}
	}()
	leaseUntil := started.Add(ttl)
	if err == nil && !time.Now().Before(leaseUntil) {
		err = context.DeadlineExceeded
	}
	if err != nil {
		return fmt.Errorf("确认执行租约失败: %w", err)
	}
	workCtx, cancelWork := context.WithCancelCause(ctx)
	// 连接取消后仍须续租直到独立清理完成,不能在清理期间放行下一个执行者。
	renewCtx, stopRenew := context.WithCancel(context.WithoutCancel(ctx))
	renewed := make(chan error, 1)
	go func() {
		timer := time.NewTimer(min(interval, time.Until(leaseUntil)/3))
		defer timer.Stop()
		for {
			select {
			case <-renewCtx.Done():
				renewed <- nil
				return
			case <-timer.C:
				started := time.Now()
				var err error
				if started.Before(leaseUntil) {
					deadline := started.Add(interval)
					if leaseUntil.Before(deadline) {
						deadline = leaseUntil
					}
					refreshCtx, cancel := context.WithDeadline(renewCtx, deadline)
					err = lock.Refresh(refreshCtx, ttl, nil)
					if err == nil {
						err = refreshCtx.Err()
						if err == nil && !time.Now().Before(deadline) {
							err = context.DeadlineExceeded
						}
					}
					cancel()
				} else {
					err = context.DeadlineExceeded
				}
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
				leaseUntil = started.Add(ttl)
				timer.Reset(min(interval, time.Until(leaseUntil)/3))
			}
		}
	}()
	defer func() {
		stopRenew()
		resultErr = errors.Join(resultErr, <-renewed)
		cancelWork(nil)
	}()
	return fn(workCtx)
}
