// eventbus 统一持久消费确认、broker重投和死信写入,失败不删除原事件。
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/logging"

	"github.com/nats-io/nats.go/jetstream"
)

// deliveryConfig 只保存统一传输层策略,不包含业务状态或模块分发逻辑。
type deliveryConfig struct {
	retryMax       int
	retryDelay     time.Duration
	requestTimeout time.Duration
	ackWait        time.Duration
	deadPrefix     string
	stream         string
}

// deadLetterMessage 保留原事件与消费身份,供修复后受控重放和业务状态核对。
type deadLetterMessage struct {
	OriginalSubject string `json:"original_subject"`
	Queue           string `json:"queue"`
	Stream          string `json:"stream"`
	Consumer        string `json:"consumer"`
	StreamSequence  uint64 `json:"stream_sequence"`
	Payload         []byte `json:"payload"`
	LastError       string `json:"last_error"`
	AttemptCount    uint64 `json:"attempt_count"`
	OccurredAt      string `json:"occurred_at"`
}

// newDeliveryConfig 将显式配置转换为当前连接的不可变交付策略。
func newDeliveryConfig(cfg config.NATSConfig) deliveryConfig {
	return deliveryConfig{
		retryMax: cfg.ConsumerRetryMax, retryDelay: time.Duration(cfg.ConsumerRetryDelayMs) * time.Millisecond,
		requestTimeout: time.Duration(cfg.RequestTimeoutMs) * time.Millisecond,
		ackWait:        time.Duration(cfg.ConsumerAckWaitSeconds) * time.Second,
		deadPrefix:     strings.TrimSpace(cfg.DeadLetterPrefix), stream: cfg.StreamName,
	}
}

// handleMessage 单次投递仅调用一次handler,重试次数由broker持久记录。
func (b *natsBus) handleMessage(ctx context.Context, subject, queue string, msg jetstream.Msg, handler Handler) {
	eventCtx, eventErr := eventContext(ctx, msg.Data())
	meta, err := msg.Metadata()
	if err != nil {
		logging.ErrorContext(eventCtx, "读取持久事件投递信息失败", err.Error(), slog.String("subject", subject), slog.String("queue", queue))
		b.retryDelivery(eventCtx, subject, queue, msg)
		return
	}
	// 处理和确认分开计时;取消订阅/进程退出时不确认尚未完成的事务。
	if ctx.Err() != nil {
		return
	}
	if eventErr == nil {
		if meta.NumDelivered <= uint64(b.cfg.retryMax) {
			handlerCtx, cancel := context.WithTimeout(eventCtx, b.cfg.ackWait)
			eventErr = invokeHandler(handlerCtx, msg.Data(), handler)
			if eventErr == nil {
				eventErr = handlerCtx.Err()
			}
			cancel()
		} else {
			// 上次死信写入失败后只重试持久死信,不再执行已耗尽的业务handler。
			eventErr = fmt.Errorf("事件业务处理已耗尽%d次,等待持久死信", b.cfg.retryMax)
		}
	}
	if ctx.Err() != nil {
		return
	}
	if eventErr != nil {
		logging.ErrorContext(eventCtx, "持久事件处理失败", eventErr.Error(), slog.String("subject", subject),
			slog.String("queue", queue), slog.Uint64("attempt_count", meta.NumDelivered), slog.Uint64("stream_sequence", meta.Sequence.Stream))
		if meta.NumDelivered < uint64(b.cfg.retryMax) {
			b.retryDelivery(eventCtx, subject, queue, msg)
			return
		}
		if err := b.publishDeadLetter(eventCtx, subject, queue, msg.Data(), meta, eventErr); err != nil {
			logging.ErrorContext(eventCtx, "持久死信写入失败,保留原事件", err.Error(), slog.String("subject", subject), slog.String("queue", queue))
			b.retryDelivery(eventCtx, subject, queue, msg)
			return
		}
	}
	// 业务成功或死信已持久接收后,才确认原消息并等待broker确认。
	ackCtx, cancel := b.requestContext(eventCtx)
	defer cancel()
	if err := msg.DoubleAck(ackCtx); err != nil {
		logging.ErrorContext(eventCtx, "持久事件消费确认失败", err.Error(), slog.String("subject", subject), slog.String("queue", queue))
	}
}

// retryDelivery 将失败消息交回broker延迟重投,不在内存中循环重试。
func (b *natsBus) retryDelivery(ctx context.Context, subject, queue string, msg jetstream.Msg) {
	if err := msg.NakWithDelay(b.cfg.retryDelay); err != nil {
		logging.ErrorContext(ctx, "持久事件重投请求失败", err.Error(), slog.String("subject", subject), slog.String("queue", queue))
	}
}

// invokeHandler 将业务panic转为保留脱敏调用栈的错误,进入同一持久重试链。
func invokeHandler(ctx context.Context, data []byte, handler Handler) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("事件处理器 panic: %v\n%s", value, debug.Stack())
		}
	}()
	return handler(ctx, data)
}

// publishDeadLetter 等待独立死信流的持久ACK,写失败时调用方不得确认原消息。
func (b *natsBus) publishDeadLetter(ctx context.Context, subject, queue string, data []byte, meta *jetstream.MsgMetadata, lastErr error) error {
	payload, err := json.Marshal(deadLetterMessage{
		OriginalSubject: subject, Queue: queue, Stream: meta.Stream, Consumer: meta.Consumer, StreamSequence: meta.Sequence.Stream,
		Payload: data, LastError: logging.SanitizeError(lastErr.Error()), AttemptCount: meta.NumDelivered,
		OccurredAt: timex.RFC3339OrEmpty(timex.Now()),
	})
	if err != nil {
		return fmt.Errorf("序列化持久死信失败: %w", err)
	}
	requestCtx, cancel := b.requestContext(ctx)
	defer cancel()
	identity := fmt.Sprintf("%s:%s:%d", meta.Stream, meta.Consumer, meta.Sequence.Stream)
	if _, err := b.js.Publish(requestCtx, b.cfg.deadPrefix+"."+subject, payload,
		jetstream.WithExpectStream(b.cfg.stream+"_DEAD"), jetstream.WithMsgID(identity)); err != nil {
		return fmt.Errorf("发布持久死信失败: %w", err)
	}
	return nil
}

// requestContext 为所有管理/发布/确认请求设置上限,不延长调用方已有期限。
func (b *natsBus) requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, b.cfg.requestTimeout)
}
