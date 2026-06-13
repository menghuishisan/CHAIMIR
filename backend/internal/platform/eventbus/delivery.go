// eventbus 提供事件消费重试与死信投递能力,保证平台基础设施层的可靠交付语义。
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/pkg/logging"
)

// deliveryConfig 描述事件消费重试与死信策略,只承载基础设施配置语义。
type deliveryConfig struct {
	retryMax     int
	retryDelay   time.Duration
	deadPrefix   string
	deadSubject  string
	publishFlush bool
}

// deadLetterMessage 记录进入死信队列的原始事件与失败上下文,便于后续排查或补偿。
type deadLetterMessage struct {
	OriginalSubject string `json:"original_subject"`
	Queue           string `json:"queue,omitempty"`
	Payload         []byte `json:"payload"`
	LastError       string `json:"last_error"`
	AttemptCount    int    `json:"attempt_count"`
	OccurredAt      string `json:"occurred_at"`
}

// newDeliveryConfig 根据统一配置构造事件重试与死信策略。
func newDeliveryConfig(cfg config.NATSConfig) deliveryConfig {
	return deliveryConfig{
		retryMax:     cfg.ConsumerRetryMax,
		retryDelay:   time.Duration(cfg.ConsumerRetryDelayMs) * time.Millisecond,
		deadPrefix:   strings.TrimSpace(cfg.DeadLetterPrefix),
		publishFlush: true,
	}
}

// deadLetterSubject 生成某个主题对应的死信主题;未配置死信前缀时返回空字符串表示只记日志。
func (c deliveryConfig) deadLetterSubject(subject string) string {
	if strings.TrimSpace(c.deadSubject) != "" {
		return c.deadSubject
	}
	if strings.TrimSpace(c.deadPrefix) == "" || strings.TrimSpace(subject) == "" {
		return ""
	}
	return c.deadPrefix + "." + strings.TrimSpace(subject)
}

// handleMessage 统一执行消费重试、结构化日志与死信投递。
func (b *natsBus) handleMessage(ctx context.Context, subject, queue string, data []byte, handler Handler) {
	eventCtx, metaErr := eventContext(ctx, data)
	if metaErr != nil {
		logging.ErrorContext(eventCtx, "事件载荷上下文校验失败", metaErr.Error(),
			slog.String("subject", subject),
			slog.String("queue", queue),
		)
		if err := b.publishDeadLetter(eventCtx, subject, queue, data, metaErr); err != nil {
			logging.ErrorContext(eventCtx, "事件死信投递失败", err.Error(),
				slog.String("subject", subject),
				slog.String("queue", queue),
			)
		}
		return
	}
	// 第一步:按统一策略重试消费,避免短暂故障直接丢事件。
	lastErr := b.retryHandle(eventCtx, data, handler)
	if lastErr == nil {
		return
	}

	logging.ErrorContext(eventCtx, "事件处理失败", lastErr.Error(),
		slog.String("subject", subject),
		slog.String("queue", queue),
		slog.Int("attempt_count", max(b.cfg.retryMax, 1)),
	)

	// 第二步:重试耗尽后写入死信,为后续补偿和人工排查保留原始事件。
	if err := b.publishDeadLetter(eventCtx, subject, queue, data, lastErr); err != nil {
		logging.ErrorContext(eventCtx, "事件死信投递失败", err.Error(),
			slog.String("subject", subject),
			slog.String("queue", queue),
		)
	}
}

// retryHandle 按配置执行有限次消费重试,最终返回最后一次错误。
func (b *natsBus) retryHandle(ctx context.Context, data []byte, handler Handler) error {
	attempts := max(b.cfg.retryMax, 1)
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := handler(ctx, data); err != nil {
			lastErr = err
			if attempt == attempts {
				break
			}
			if b.cfg.retryDelay > 0 {
				// 这里显式等待重试间隔,把短暂网络/下游抖动留给同一次消费窗口吸收。
				timer := time.NewTimer(b.cfg.retryDelay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return fmt.Errorf("事件消费被取消: %w", ctx.Err())
				case <-timer.C:
				}
			}
			continue
		}
		return nil
	}
	return lastErr
}

// publishDeadLetter 把失败事件投递到死信主题,供上层补偿任务或人工处理读取。
func (b *natsBus) publishDeadLetter(ctx context.Context, subject, queue string, data []byte, lastErr error) error {
	deadSubject := b.cfg.deadLetterSubject(subject)
	if deadSubject == "" {
		return nil
	}
	payload, err := json.Marshal(deadLetterMessage{
		OriginalSubject: subject,
		Queue:           queue,
		Payload:         data,
		LastError:       logging.SanitizeError(lastErr.Error()),
		AttemptCount:    max(b.cfg.retryMax, 1),
		OccurredAt:      time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("序列化死信消息失败: %w", err)
	}
	if err := b.conn.Publish(deadSubject, payload); err != nil {
		return fmt.Errorf("发布死信 %s 失败: %w", deadSubject, err)
	}
	if b.cfg.publishFlush {
		// 死信也必须等待 flush 确认,否则仍可能只停留在客户端缓冲区。
		if err := b.conn.FlushWithContext(ctx); err != nil {
			return fmt.Errorf("确认死信 %s 失败: %w", deadSubject, err)
		}
	}
	return nil
}

// max 返回较大的整数,供重试配置兜底复用。
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
