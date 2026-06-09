// eventbus 封装 NATS 事件总线,用于跨模块反向通知和解耦通信。
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/pkg/apperr"

	"github.com/nats-io/nats.go"
)

// Bus 定义事件总线的统一接口,便于装配和测试。
type Bus interface {
	// Publish 发布一条事件并等待 flush 确认。
	Publish(ctx context.Context, subject string, payload any) error
	// Subscribe 注册普通或队列组订阅。
	Subscribe(subject, queue string, handler Handler) (Subscription, error)
	// Close 关闭底层连接。
	Close()
}

// Handler 处理一条事件消息。
type Handler func(ctx context.Context, data []byte) error

// Subscription 表示一个可取消的订阅。
type Subscription interface {
	// Unsubscribe 取消订阅。
	Unsubscribe() error
}

type natsConn interface {
	Publish(subj string, data []byte) error
	FlushWithContext(ctx context.Context) error
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	Subscribe(subj string, cb nats.MsgHandler) (*nats.Subscription, error)
	Close()
}

type natsBus struct {
	conn natsConn
	cfg  deliveryConfig
}

// New 建立真实 NATS 连接;事件总线不可用时启动失败。
func New(cfg config.NATSConfig) (Bus, error) {
	opts := []nats.Option{
		nats.Name("chaimir-backend"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Duration(cfg.ReconnectWaitSeconds) * time.Second),
	}
	if cfg.Token != "" {
		opts = append(opts, nats.Token(cfg.Token))
	}
	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("连接 NATS 失败: %w", err)
	}
	return &natsBus{conn: conn, cfg: newDeliveryConfig(cfg)}, nil
}

// Publish 序列化并发布事件,同时等待 flush 确认,避免只停留在客户端缓冲。
func (b *natsBus) Publish(ctx context.Context, subject string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("事件序列化失败: %w", err)
	}
	if err := b.conn.Publish(subject, data); err != nil {
		return fmt.Errorf("发布事件 %s 失败: %w", subject, err)
	}
	if err := b.conn.FlushWithContext(ctx); err != nil {
		return fmt.Errorf("确认发布事件 %s 失败: %w", subject, err)
	}
	return nil
}

// Subscribe 注册订阅,并把处理失败统一写入结构化日志。
func (b *natsBus) Subscribe(subject, queue string, handler Handler) (Subscription, error) {
	cb := func(msg *nats.Msg) {
		b.handleMessage(context.Background(), subject, queue, msg.Data, handler)
	}
	var (
		sub *nats.Subscription
		err error
	)
	if queue != "" {
		sub, err = b.conn.QueueSubscribe(subject, queue, cb)
	} else {
		sub, err = b.conn.Subscribe(subject, cb)
	}
	if err != nil {
		return nil, fmt.Errorf("订阅事件 %s 失败: %w", subject, err)
	}
	return sub, nil
}

// Close 关闭底层 NATS 连接。
func (b *natsBus) Close() {
	b.conn.Close()
}

// Decode 统一解码事件 payload,保留调用方给出的模块错误语义。
func Decode(data []byte, dst any, invalid *apperr.Error) error {
	if err := json.Unmarshal(data, dst); err != nil {
		return invalid.WithCause(err)
	}
	return nil
}
