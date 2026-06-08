// M10 事件消费测试:覆盖重试配置、取消语义与死信发布边界。
package notify

import (
	"context"
	"errors"
	"testing"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

// TestWithRetryStopsWhenContextIsCanceled 确认事件消费等待可被上游取消,避免固定 sleep 拖住关闭流程。
func TestWithRetryStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	svc := &Service{
		eventRetryMax:     3,
		eventRetryDelayMs: 1000,
		waitRetryDelay: func(context.Context, int) error {
			cancel()
			return context.Canceled
		},
	}
	attempts := 0

	err := svc.withRetry(ctx, &recordingEventBus{}, "notify.send", []byte(`{}`), func() error {
		attempts++
		return errors.New("send failed")
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("retry should stop after cancellation, attempts=%d", attempts)
	}
}

// TestWithRetryUsesExponentialBackoff 确认事件消费失败按指数退避重试,符合 DLQ 前可靠投递策略。
func TestWithRetryUsesExponentialBackoff(t *testing.T) {
	delays := []int{}
	svc := &Service{
		eventRetryMax:     4,
		eventRetryDelayMs: 25,
		waitRetryDelay: func(_ context.Context, delayMs int) error {
			delays = append(delays, delayMs)
			return nil
		},
	}
	attempts := 0

	err := svc.withRetry(context.Background(), &recordingEventBus{}, "notify.send", []byte(`{}`), func() error {
		attempts++
		return errors.New("send failed")
	})

	if err == nil {
		t.Fatalf("expected retry exhaustion error")
	}
	if attempts != 4 {
		t.Fatalf("expected four delivery attempts, got %d", attempts)
	}
	want := []int{25, 50, 100}
	if len(delays) != len(want) {
		t.Fatalf("expected delays %v, got %v", want, delays)
	}
	for i := range want {
		if delays[i] != want[i] {
			t.Fatalf("expected delays %v, got %v", want, delays)
		}
	}
}

// TestNewServiceKeepsNotifyRetryConfig 确认 M10 装配直接使用统一配置,不在模块内保留默认重试策略。
func TestNewServiceKeepsNotifyRetryConfig(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, config.NotifyConfig{EventRetryMax: 4, EventRetryDelayMs: 25, UnreadTTLHours: 72})

	if svc.eventRetryMax != 4 || svc.eventRetryDelayMs != 25 {
		t.Fatalf("notify retry config was not injected: max=%d delay=%d", svc.eventRetryMax, svc.eventRetryDelayMs)
	}
	counter, ok := svc.unread.(*redisUnreadCounter)
	if !ok {
		t.Fatalf("expected redis unread counter, got %T", svc.unread)
	}
	if counter.ttl.Hours() != 72 {
		t.Fatalf("notify unread ttl was not injected: %s", counter.ttl)
	}
}

// TestSubscribeEventsRequiresConfiguredBus 确认通知事件入口缺少总线时显式失败,避免发送/推送链路静默失效。
func TestSubscribeEventsRequiresConfiguredBus(t *testing.T) {
	svc := &Service{}

	err := svc.SubscribeEvents(nil)
	if err == nil {
		t.Fatalf("expected missing notify event bus to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrNotifySendFailed.Code {
		t.Fatalf("expected notify send failed error, got %v", err)
	}
}

type recordingEventBus struct{}

func (b *recordingEventBus) Publish(context.Context, string, any) error { return nil }
func (b *recordingEventBus) Subscribe(string, string, eventbus.Handler) (eventbus.Subscription, error) {
	return nil, nil
}
func (b *recordingEventBus) Close() {}
