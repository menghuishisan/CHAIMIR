// Package background 测试后台任务运行器的通用停止与错误恢复语义。
package background

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestRunnerTicksUntilContextCancelled 确认后台任务按 interval 执行并响应 context 停止。
func TestRunnerTicksUntilContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	done := make(chan struct{})
	go func() {
		Run(ctx, Task{
			Name:     "test.tick",
			Interval: time.Millisecond,
			Run: func(context.Context) error {
				if calls.Add(1) >= 2 {
					cancel()
				}
				return nil
			},
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("runner did not stop after context cancellation")
	}
	if calls.Load() < 2 {
		t.Fatalf("runner did not tick enough times: %d", calls.Load())
	}
}

// TestRunnerContinuesAfterErrorAndPanic 确认单轮错误或 panic 不会杀死整个后台任务。
func TestRunnerContinuesAfterErrorAndPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	done := make(chan struct{})
	go func() {
		Run(ctx, Task{
			Name:     "test.recover",
			Interval: time.Millisecond,
			Run: func(context.Context) error {
				switch calls.Add(1) {
				case 1:
					return errors.New("boom")
				case 2:
					panic("panic boom")
				default:
					cancel()
					return nil
				}
			},
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("runner did not continue after error and panic")
	}
	if calls.Load() < 3 {
		t.Fatalf("runner stopped before recovery path: %d", calls.Load())
	}
}
