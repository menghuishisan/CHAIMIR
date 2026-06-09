// background 提供后台轮询任务的统一运行语义,不承载任何业务状态机。
package background

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"chaimir/pkg/logging"
)

// Task 描述一个可按固定间隔运行的后台任务。
type Task struct {
	Name     string
	Interval time.Duration
	Run      func(context.Context) error
}

// Run 按固定间隔执行后台任务,单轮错误或 panic 只记录日志而不终止整个循环。
func Run(ctx context.Context, task Task) {
	interval := task.Interval
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		runOnce(ctx, task)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// runOnce 包装单轮执行的错误与 panic 边界,统一后台任务的失败处理方式。
func runOnce(ctx context.Context, task Task) {
	defer func() {
		if v := recover(); v != nil {
			logging.ErrorContext(ctx, "background task panic", fmt.Sprint(v), slog.String("task", task.Name))
		}
	}()
	if task.Run == nil {
		logging.ErrorContext(ctx, "background task missing runner", "nil runner", slog.String("task", task.Name))
		return
	}
	if err := task.Run(ctx); err != nil {
		logging.ErrorContext(ctx, "background task failed", err.Error(), slog.String("task", task.Name))
	}
}
