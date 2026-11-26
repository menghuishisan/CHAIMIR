// Package background 运行模块声明的周期任务。
package background

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"chaimir/internal/platform/response"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

	"github.com/google/uuid"
)

// Task 描述一个可按固定间隔运行的后台任务。
type Task struct {
	Name     string
	Interval time.Duration
	Run      func(context.Context) error
}

// Start 校验一批任务后立即启动，各任务在同一个 context 取消时结束。
func Start(ctx context.Context, tasks ...Task) error {
	if ctx == nil {
		return fmt.Errorf("后台任务缺少 context")
	}
	for _, task := range tasks {
		if task.Name == "" || task.Run == nil || task.Interval <= 0 {
			return fmt.Errorf("后台任务 %q 缺少名称、回调或有效间隔", task.Name)
		}
	}
	for _, task := range tasks {
		go run(ctx, task)
	}
	return nil
}

func run(ctx context.Context, task Task) {
	ticker := time.NewTicker(task.Interval)
	defer ticker.Stop()

	for ctx.Err() == nil {
		runOnce(ctx, task)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runOnce(ctx context.Context, task Task) {
	traceID := uuid.NewString()
	ctx = response.WithTrace(ctx, traceID)
	ctx = logging.WithAttrs(ctx,
		slog.String("trace_id", traceID),
		slog.Int64("tenant_id", 0),
		slog.String("task", task.Name),
		slog.String("operation_scope", "system_background"),
	)
	defer func() {
		if v := recover(); v != nil {
			logging.ErrorContext(ctx, "background task panic", fmt.Sprintf("%v\n%s", v, debug.Stack()))
		}
	}()
	if err := task.Run(ctx); err != nil {
		logging.ErrorContext(ctx, "background task failed", apperr.AsAppError(err).LogString())
	}
}
