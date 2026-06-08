// Package eventbus 封装 NATS 事件总线,用于模块反向通信(解耦,杜绝循环依赖)。
// 依据 docs/总-工程目录设计.md §3.1.1:低层通知高层走事件,不反向 import 业务模块。
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

	"github.com/nats-io/nats.go"
)

// Bus 是事件总线接口(便于测试替身与契约约束)。
type Bus interface {
	Publish(ctx context.Context, subject string, payload any) error
	Subscribe(subject, queue string, handler Handler) (Subscription, error)
	Close()
}

// Handler 处理一条事件消息。
type Handler func(ctx context.Context, data []byte) error

// Subscription 表示一个可取消的订阅。
type Subscription interface{ Unsubscribe() error }

type natsConn interface {
	Publish(subj string, data []byte) error
	FlushWithContext(ctx context.Context) error
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	Subscribe(subj string, cb nats.MsgHandler) (*nats.Subscription, error)
	Close()
}

type natsBus struct{ conn natsConn }

// New 连接真实 NATS 事件总线;运行期缺 bus 必须启动失败,不能注入空实现。
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
	return &natsBus{conn: conn}, nil
}

// Publish 序列化并发布事件,Flush 确认用于避免关键业务事件只停留在客户端缓冲。
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

// Decode 统一解码事件 payload,失败时保留调用模块传入的错误码和 JSON 原始错误链。
func Decode(data []byte, dst any, invalid *apperr.Error) error {
	if err := json.Unmarshal(data, dst); err != nil {
		return invalid.WithCause(err)
	}
	return nil
}

// Subscribe 注册普通订阅或队列组订阅,统一把处理失败写入结构化日志。
func (b *natsBus) Subscribe(subject, queue string, handler Handler) (Subscription, error) {
	cb := func(msg *nats.Msg) {
		dispatchMessage(context.Background(), subject, queue, msg.Data, handler)
	}
	var (
		sub *nats.Subscription
		err error
	)
	if queue != "" {
		sub, err = b.conn.QueueSubscribe(subject, queue, cb) // 队列组负载均衡。
	} else {
		sub, err = b.conn.Subscribe(subject, cb)
	}
	if err != nil {
		return nil, fmt.Errorf("订阅事件 %s 失败: %w", subject, err)
	}
	return sub, nil
}

// Close 关闭底层 NATS 连接,供后端进程退出时释放资源。
func (b *natsBus) Close() { b.conn.Close() }

// dispatchMessage 执行订阅处理并记录失败上下文,避免事件处理错误静默丢弃。
func dispatchMessage(ctx context.Context, subject, queue string, data []byte, handler Handler) {
	if err := handler(ctx, data); err != nil {
		logging.ErrorContext(ctx, "事件处理失败", err.Error(),
			slog.String("subject", subject),
			slog.String("queue", queue),
		)
	}
}
