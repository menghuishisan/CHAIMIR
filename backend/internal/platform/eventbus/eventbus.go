// eventbus 封装 NATS 事件总线,用于跨模块反向通知和解耦通信。
package eventbus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/response"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

	"github.com/google/uuid"
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
	if b == nil || b.conn == nil {
		return fmt.Errorf("事件总线未初始化")
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return fmt.Errorf("事件主题不能为空")
	}
	data, err := marshalEventPayload(ctx, payload)
	if err != nil {
		return fmt.Errorf("事件序列化失败: %w", err)
	}
	if err := b.conn.Publish(subject, data); err != nil {
		return fmt.Errorf("发布事件 %s 失败: %w", subject, err)
	}
	if err := b.flush(ctx); err != nil {
		return fmt.Errorf("确认发布事件 %s 失败: %w", subject, err)
	}
	return nil
}

// Subscribe 注册订阅,并把处理失败统一写入结构化日志。
func (b *natsBus) Subscribe(subject, queue string, handler Handler) (Subscription, error) {
	if b == nil || b.conn == nil {
		return nil, fmt.Errorf("事件总线未初始化")
	}
	subject = strings.TrimSpace(subject)
	queue = strings.TrimSpace(queue)
	if subject == "" {
		return nil, fmt.Errorf("事件主题不能为空")
	}
	if handler == nil {
		return nil, fmt.Errorf("事件处理器不能为空")
	}
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
	if b == nil || b.conn == nil {
		return
	}
	b.conn.Close()
}

// Decode 统一解码事件 payload,保留调用方给出的模块错误语义。
func Decode(data []byte, dst any, invalid *apperr.Error) error {
	if err := json.Unmarshal(data, dst); err != nil {
		return invalid.WithCause(err)
	}
	return nil
}

// eventMetadata 是所有跨模块事件必须携带的排障上下文。
type eventMetadata struct {
	TenantID int64  `json:"tenant_id"`
	TraceID  string `json:"trace_id"`
}

// marshalEventPayload 校验事件载荷必须携带真实租户与 trace,并把当前请求 trace 写入事件。
func marshalEventPayload(ctx context.Context, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&obj); err != nil {
		return nil, err
	}
	if _, ok := obj["tenant_id"]; !ok {
		return nil, fmt.Errorf("事件载荷缺少 tenant_id")
	}
	tenantID, ok := numericTenantID(obj["tenant_id"])
	if !ok || tenantID < 0 {
		return nil, fmt.Errorf("事件载荷 tenant_id 非法")
	}
	traceID, _ := obj["trace_id"].(string)
	if traceID == "" {
		traceID = response.TraceFromContext(ctx)
	}
	if traceID == "" {
		return nil, fmt.Errorf("事件载荷缺少 trace_id")
	}
	obj["trace_id"] = traceID
	return json.Marshal(obj)
}

// eventContext 从事件载荷中提取真实租户和 trace,供消费失败日志和 handler 继续传递。
func eventContext(ctx context.Context, data []byte) (context.Context, error) {
	meta, ok := extractEventMetadata(data)
	if !ok {
		traceID := uuid.NewString()
		ctx = response.WithTrace(ctx, traceID)
		ctx = logging.WithAttrs(ctx, slog.String("trace_id", traceID), slog.Int64("tenant_id", 0), slog.String("operation_scope", "event_metadata_validation"))
		return ctx, fmt.Errorf("事件载荷缺少真实 tenant_id 或 trace_id")
	}
	if meta.TraceID != "" {
		ctx = response.WithTrace(ctx, meta.TraceID)
	}
	attrs := []slog.Attr{}
	if meta.TraceID != "" {
		attrs = append(attrs, slog.String("trace_id", meta.TraceID))
	}
	if meta.TenantID >= 0 {
		attrs = append(attrs, slog.Int64("tenant_id", meta.TenantID))
	}
	return logging.WithAttrs(ctx, attrs...), nil
}

// extractEventMetadata 只从真实存在的事件字段提取排障上下文,不为缺失字段制造默认值。
func extractEventMetadata(data []byte) (eventMetadata, bool) {
	var obj map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&obj); err != nil {
		return eventMetadata{}, false
	}
	tenantValue, hasTenant := obj["tenant_id"]
	traceID, hasTrace := obj["trace_id"].(string)
	tenantID, tenantOK := numericTenantID(tenantValue)
	if !hasTenant || !tenantOK || !hasTrace || traceID == "" {
		return eventMetadata{}, false
	}
	return eventMetadata{TenantID: tenantID, TraceID: traceID}, true
}

// numericTenantID 处理 JSON 解码后的数字形态,只用于验证事件字段真实存在且可解析。
func numericTenantID(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), n == float64(int64(n))
	case int64:
		return n, true
	case int:
		return int64(n), true
	case json.Number:
		out, err := n.Int64()
		return out, err == nil
	case string:
		out, err := strconv.ParseInt(n, 10, 64)
		return out, err == nil
	default:
		return 0, false
	}
}
