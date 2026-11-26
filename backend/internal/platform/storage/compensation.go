package storage

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// CompensateUpload 仅在未尝试提交时限时清理本次对象，并保留原错误。
// 提交报错后，查询未命中也无法排除尚未可见的提交，必须保留对象。
func CompensateUpload(ctx context.Context, timeout time.Duration, cause error, commitAttempted bool, discard func(context.Context) error) error {
	if commitAttempted {
		return fmt.Errorf("事务提交结果未确认，保留本次对象: %w", cause)
	}
	if timeout <= 0 {
		return errors.Join(cause, fmt.Errorf("上传补偿缺少有效超时，保留本次对象"))
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	if err := discard(ctx); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}
