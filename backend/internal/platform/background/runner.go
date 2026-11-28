// Package background 管理模块周期任务和执行期间续租的生命周期。
package background

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

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

// Runner 汇总服务实例的后台任务，由组合根取消上下文后等待退出。
type Runner struct {
	ctx   context.Context
	tasks sync.WaitGroup
}

// New 创建共享同一取消上下文的任务运行器。
func New(ctx context.Context) *Runner { return &Runner{ctx: ctx} }

// Start 校验整批任务后启动；所有注册须在取消和 Wait 之前完成。
func (r *Runner) Start(tasks ...Task) error {
	if r.ctx == nil {
		return fmt.Errorf("后台任务缺少 context")
	}
	for _, task := range tasks {
		if task.Name == "" || task.Run == nil || task.Interval <= 0 {
			return fmt.Errorf("后台任务 %q 缺少名称、回调或有效间隔", task.Name)
		}
	}
	for _, task := range tasks {
		r.tasks.Go(func() { run(r.ctx, task) })
	}
	return nil
}

// Wait 等待所有回调及其清理结束；调用者须先取消上下文。
func (r *Runner) Wait() { r.tasks.Wait() }

// WithRenewal 在已确认的租约内执行 run；leaseDuration 至少 1 秒。
// 续租 I/O 以当前确认窗口为截止时间；成功后按请求开始时间计算下个窗口。
// 续租失败取消工作并返回 leaseLost。所有退出路径均取消并等待续租结束，
// 停止后的续租结果不算丢失租约；并发错误保留完整错误链，续租错误优先。
func WithRenewal(ctx context.Context, leaseDuration time.Duration, renew func(context.Context) (bool, error), run func(context.Context) error) (leaseLost bool, resultErr error) {
	leaseUntil := time.Now().Add(leaseDuration)
	workCtx, cancelWork := context.WithCancel(ctx)
	renewCtx, cancelRenew := context.WithCancel(ctx)
	stop := make(chan struct{})
	done := make(chan struct{})
	var renewalErr error
	go func() {
		defer close(done)
		ticker := time.NewTicker(min(leaseDuration/3, 30*time.Second))
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				if renewCtx.Err() != nil {
					return
				}
				started := time.Now()
				callCtx, cancel := context.WithDeadline(renewCtx, leaseUntil)
				renewed, err := renew(callCtx)
				if err == nil {
					err = callCtx.Err()
				}
				cancel()
				select {
				case <-stop:
					return
				case <-ctx.Done():
					return
				default:
				}
				if err == nil && renewed {
					leaseUntil = started.Add(leaseDuration)
					continue
				}
				if err == nil {
					err = fmt.Errorf("租约已过期或被其他 worker 接管")
				}
				renewalErr = err
				cancelWork()
				return
			}
		}
	}()
	defer func() {
		close(stop)
		cancelRenew()
		<-done
		cancelWork()
		leaseLost = renewalErr != nil
		if leaseLost {
			resultErr = errors.Join(renewalErr, resultErr)
		} else if resultErr == nil {
			resultErr = ctx.Err()
		}
	}()
	return false, run(workCtx)
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
	ctx = logging.WithTrace(ctx, traceID)
	ctx = logging.WithAttrs(ctx,
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
