// M10 事件消费:订阅通知发送与实时推送事件,失败重试后进入死信队列。
package notify

import (
	"context"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

// SubscribeEvents 订阅通知事件总线 subject。
func (s *Service) SubscribeEvents(bus eventbus.Bus) error {
	if bus == nil {
		return apperr.ErrNotifySendFailed
	}
	if _, err := bus.Subscribe(contracts.SubjectNotifySend, "notify", s.onNotifySend(bus)); err != nil {
		return apperr.ErrNotifySendFailed.WithCause(err)
	}
	if _, err := bus.Subscribe(contracts.SubjectNotifyPush, "notify", s.onNotifyPush(bus)); err != nil {
		return apperr.ErrNotifySendFailed.WithCause(err)
	}
	return nil
}

// onNotifySend 解码并投递站内信通知事件。
func (s *Service) onNotifySend(bus eventbus.Bus) eventbus.Handler {
	return func(ctx context.Context, data []byte) error {
		var req contracts.NotifySendRequest
		if err := eventbus.Decode(data, &req, apperr.ErrNotifyInvalid); err != nil {
			return err
		}
		return s.withRetry(ctx, bus, contracts.SubjectNotifySend, data, func() error {
			return s.Send(ctx, req)
		})
	}
}

// onNotifyPush 解码并投递实时推送事件。
func (s *Service) onNotifyPush(bus eventbus.Bus) eventbus.Handler {
	return func(ctx context.Context, data []byte) error {
		var req contracts.NotifyPushRequest
		if err := eventbus.Decode(data, &req, apperr.ErrNotifyInvalid); err != nil {
			return err
		}
		return s.withRetry(ctx, bus, contracts.SubjectNotifyPush, data, func() error {
			return s.Push(ctx, req)
		})
	}
}

// withRetry 对通知投递执行有限重试,耗尽后发布 DLQ 事件。
func (s *Service) withRetry(ctx context.Context, bus eventbus.Bus, subject string, data []byte, fn func() error) error {
	var last error
	maxAttempts := normalizeRetryMax(s.eventRetryMax)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			last = err
		}
		if attempt == maxAttempts-1 {
			break
		}
		if err := s.retryDelay(ctx, attempt); err != nil {
			return err
		}
	}
	dlq := contracts.NotifyDeadLetterEvent{Subject: subject, Reason: last.Error(), Payload: string(data)}
	if err := bus.Publish(ctx, contracts.SubjectNotifyDLQ, dlq); err != nil {
		return apperr.ErrNotifySendFailed.WithCause(err)
	}
	return apperr.ErrNotifySendFailed.WithCause(last)
}

// normalizeRetryMax 保证事件消费至少尝试一次,避免配置缺省导致消息被跳过。
func normalizeRetryMax(maxAttempts int) int {
	if maxAttempts <= 0 {
		return 1
	}
	return maxAttempts
}

// retryDelay 在通知事件重试之间按指数退避等待,测试可注入等待函数避免阻塞。
func (s *Service) retryDelay(ctx context.Context, attempt int) error {
	delayMs := s.exponentialDelayMs(attempt)
	if s.waitRetryDelay != nil {
		return s.waitRetryDelay(ctx, delayMs)
	}
	return waitNotifyRetryDelay(ctx, delayMs)
}

// exponentialDelayMs 根据已失败次数计算本次重试等待时间。
func (s *Service) exponentialDelayMs(attempt int) int {
	if s.eventRetryDelayMs <= 0 {
		return 0
	}
	delay := s.eventRetryDelayMs
	for i := 0; i < attempt; i++ {
		delay *= 2
	}
	return delay
}

// waitNotifyRetryDelay 使用可取消 timer,请求结束时立即停止后续重试。
func waitNotifyRetryDelay(ctx context.Context, delayMs int) error {
	if delayMs <= 0 {
		return nil
	}
	timer := time.NewTimer(time.Duration(delayMs) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
