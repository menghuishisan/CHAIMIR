// eventbus 封装唯一JetStream持久事件总线,用于跨模块反向通知和重启后恢复。
package eventbus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/response"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Bus 定义事件总线的统一接口,便于装配和测试。
type Bus interface {
	// Publish 等待broker持久接收确认,不代表业务handler已完成。
	Publish(ctx context.Context, subject string, payload any) error
	// Subscribe 以主题和稳定队列标识注册持久订阅。
	Subscribe(subject, queue string, handler Handler) (Subscription, error)
	// Close 关闭底层连接并返回释放失败。
	Close() error
}

// FanoutBus 把控制信号送到每个在线副本,订阅进度不按副本持久化。
type FanoutBus interface {
	// PublishFanout 等待 JetStream 持久接收控制信号。
	PublishFanout(ctx context.Context, subject string, payload any) error
	// SubscribeFanout 为当前进程建立独立的临时消费,各进程都收到新信号。
	SubscribeFanout(subject string, handler Handler) (Subscription, error)
}

// Handler 处理一条事件消息。
type Handler func(ctx context.Context, data []byte) error

// Subscription 表示一个可取消的订阅。
type Subscription interface {
	// Unsubscribe 取消订阅。
	Unsubscribe() error
}

type natsBus struct {
	conn   *nats.Conn
	js     jetstream.JetStream
	cfg    deliveryConfig
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	closed bool
	subs   []Subscription
}

// durableSubscription 仅停止当前进程消费,保留broker端durable与确认进度。
type durableSubscription struct {
	consume jetstream.ConsumeContext
	cancel  context.CancelFunc
	once    sync.Once
}

type fanoutSubscription struct {
	consume      jetstream.ConsumeContext
	cancel       context.CancelFunc
	js           jetstream.JetStream
	stream       string
	consumerName string
	timeout      time.Duration
	once         sync.Once
	err          error
}

// Unsubscribe 停止本地投递并删除临时消费。
func (s *fanoutSubscription) Unsubscribe() error {
	s.once.Do(func() {
		s.cancel()
		s.consume.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
		defer cancel()
		s.err = s.js.DeleteConsumer(ctx, s.stream, s.consumerName)
		if errors.Is(s.err, jetstream.ErrConsumerNotFound) {
			s.err = nil
		}
	})
	return s.err
}

// Unsubscribe 取消本地处理,不删除consumer或尚未确认的事件。
func (s *durableSubscription) Unsubscribe() error {
	s.once.Do(func() { s.cancel(); s.consume.Stop() })
	return nil
}

// New 连接NATS并建立唯一持久流;不可用或配置冲突时启动失败。
func New(cfg config.NATSConfig) (Bus, error) {
	if strings.TrimSpace(cfg.URL) == "" || cfg.ReconnectWaitSeconds <= 0 || cfg.RequestTimeoutMs <= 0 || cfg.ConsumerAckWaitSeconds <= 0 || cfg.ConsumerRetryMax <= 0 || cfg.ConsumerRetryDelayMs <= 0 ||
		cfg.StreamMaxBytes <= 0 || cfg.StreamMaxAgeSeconds <= 0 || cfg.DeadLetterMaxBytes <= 0 || cfg.StreamReplicas < 1 || cfg.StreamReplicas > 5 ||
		strings.TrimSpace(cfg.StreamName) == "" || len(cfg.StreamSubjects) == 0 || !concreteSubject(cfg.DeadLetterPrefix) {
		return nil, fmt.Errorf("事件总线持久交付配置非法")
	}
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
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("建立JetStream客户端失败: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	b := &natsBus{conn: conn, js: js, cfg: newDeliveryConfig(cfg), ctx: ctx, cancel: cancel}
	// 两个流独立保留;容量满拒绝新写,死信不随业务保留期删除。
	for _, stream := range []jetstream.StreamConfig{
		{Name: cfg.StreamName, Subjects: cfg.StreamSubjects, Storage: jetstream.FileStorage, Retention: jetstream.LimitsPolicy,
			Discard: jetstream.DiscardNew, MaxBytes: cfg.StreamMaxBytes, MaxAge: time.Duration(cfg.StreamMaxAgeSeconds) * time.Second, Replicas: cfg.StreamReplicas},
		{Name: cfg.StreamName + "_DEAD", Subjects: []string{cfg.DeadLetterPrefix + ".>"}, Storage: jetstream.FileStorage,
			Retention: jetstream.LimitsPolicy, Discard: jetstream.DiscardNew, MaxBytes: cfg.DeadLetterMaxBytes, Replicas: cfg.StreamReplicas},
	} {
		requestCtx, requestCancel := b.requestContext(ctx)
		_, err = js.CreateOrUpdateStream(requestCtx, stream)
		requestCancel()
		if err != nil {
			closeErr := b.Close()
			return nil, errors.Join(fmt.Errorf("建立持久事件流 %s 失败: %w", stream.Name, err), closeErr)
		}
	}
	return b, nil
}

// Publish 序列化事件并等待指定业务流持久ACK,不使用Core发送路径。
func (b *natsBus) Publish(ctx context.Context, subject string, payload any) error {
	if b == nil || b.js == nil {
		return fmt.Errorf("事件总线未初始化")
	}
	subject = strings.TrimSpace(subject)
	if !concreteSubject(subject) || subject == b.cfg.deadPrefix || strings.HasPrefix(subject, b.cfg.deadPrefix+".") {
		return fmt.Errorf("业务事件主题非法")
	}
	data, err := marshalEventPayload(ctx, payload)
	if err != nil {
		return fmt.Errorf("事件序列化失败: %w", err)
	}
	requestCtx, cancel := b.requestContext(ctx)
	defer cancel()
	if _, err := b.js.Publish(requestCtx, subject, data, jetstream.WithExpectStream(b.cfg.stream)); err != nil {
		return fmt.Errorf("持久发布事件 %s 失败: %w", subject, err)
	}
	return nil
}

// PublishFanout 向该主题的所有在线进程发送控制信号。
func (b *natsBus) PublishFanout(ctx context.Context, subject string, payload any) error {
	return b.Publish(ctx, subject, payload)
}

// Subscribe 使用主题和队列的稳定摘要创建durable,从保留事件最早处恢复。
func (b *natsBus) Subscribe(subject, queue string, handler Handler) (Subscription, error) {
	if b == nil || b.js == nil {
		return nil, fmt.Errorf("事件总线未初始化")
	}
	subject = strings.TrimSpace(subject)
	queue = strings.TrimSpace(queue)
	if !concreteSubject(subject) {
		return nil, fmt.Errorf("事件主题非法")
	}
	if queue == "" {
		return nil, fmt.Errorf("持久事件订阅必须提供稳定队列标识")
	}
	if handler == nil {
		return nil, fmt.Errorf("事件处理器不能为空")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, fmt.Errorf("事件总线已关闭")
	}
	name := durableName(subject, queue)
	requestCtx, requestCancel := b.requestContext(b.ctx)
	consumer, err := b.js.CreateOrUpdateConsumer(requestCtx, b.cfg.stream, jetstream.ConsumerConfig{
		Durable: name, FilterSubject: subject, DeliverPolicy: jetstream.DeliverAllPolicy, AckPolicy: jetstream.AckExplicitPolicy,
		AckWait: b.cfg.ackWait, MaxDeliver: -1, MaxAckPending: 1,
	})
	requestCancel()
	if err != nil {
		return nil, fmt.Errorf("建立持久订阅 %s/%s 失败: %w", subject, queue, err)
	}
	subCtx, cancel := context.WithCancel(b.ctx)
	consume, err := consumer.Consume(func(msg jetstream.Msg) { b.handleMessage(subCtx, subject, queue, msg, handler) }, jetstream.PullMaxMessages(1),
		jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
			logging.ErrorContext(subCtx, "持久事件消费连接异常", err.Error(), slog.String("subject", subject), slog.String("queue", queue))
		}))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("订阅持久事件 %s/%s 失败: %w", subject, queue, err)
	}
	sub := &durableSubscription{consume: consume, cancel: cancel}
	b.subs = append(b.subs, sub)
	return sub, nil
}

// SubscribeFanout 为当前进程建立独立临时消费,确保所有在线进程都收到信号。
func (b *natsBus) SubscribeFanout(subject string, handler Handler) (Subscription, error) {
	if b == nil || b.js == nil {
		return nil, fmt.Errorf("事件总线未初始化")
	}
	subject = strings.TrimSpace(subject)
	if !concreteSubject(subject) {
		return nil, fmt.Errorf("广播主题非法")
	}
	if handler == nil {
		return nil, fmt.Errorf("广播处理器不能为空")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, fmt.Errorf("事件总线已关闭")
	}
	requestCtx, requestCancel := b.requestContext(b.ctx)
	consumer, err := b.js.CreateConsumer(requestCtx, b.cfg.stream, jetstream.ConsumerConfig{
		FilterSubject: subject, DeliverPolicy: jetstream.DeliverNewPolicy, AckPolicy: jetstream.AckExplicitPolicy,
		AckWait: b.cfg.ackWait, MaxDeliver: -1, InactiveThreshold: b.cfg.inactiveThreshold,
	})
	requestCancel()
	if err != nil {
		return nil, fmt.Errorf("建立广播消费 %s 失败: %w", subject, err)
	}
	info := consumer.CachedInfo()
	if info == nil || info.Name == "" {
		return nil, fmt.Errorf("广播消费 %s 缺少服务端名称", subject)
	}
	subCtx, cancel := context.WithCancel(b.ctx)
	consume, err := consumer.Consume(func(msg jetstream.Msg) {
		b.handleMessage(subCtx, subject, "fanout", msg, handler)
	}, jetstream.PullMaxMessages(1), jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
		logging.ErrorContext(subCtx, "广播消费连接异常", err.Error(), slog.String("subject", subject))
	}))
	if err != nil {
		cancel()
		requestCtx, requestCancel := b.requestContext(context.Background())
		deleteErr := b.js.DeleteConsumer(requestCtx, b.cfg.stream, info.Name)
		requestCancel()
		failures := []error{fmt.Errorf("订阅广播事件 %s 失败: %w", subject, err)}
		if deleteErr != nil && !errors.Is(deleteErr, jetstream.ErrConsumerNotFound) {
			failures = append(failures, fmt.Errorf("删除未使用的广播消费失败: %w", deleteErr))
		}
		return nil, errors.Join(failures...)
	}
	wrapped := &fanoutSubscription{
		consume: consume, cancel: cancel, js: b.js, stream: b.cfg.stream,
		consumerName: info.Name, timeout: b.cfg.requestTimeout,
	}
	b.subs = append(b.subs, wrapped)
	return wrapped, nil
}

// Close 取消当前处理并关闭连接,未确认消息仍由broker保留重投。
func (b *natsBus) Close() error {
	if b == nil || b.conn == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	b.cancel()
	var closeErr error
	for _, sub := range b.subs {
		if err := sub.Unsubscribe(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("停止持久订阅失败: %w", err))
		}
	}
	if err := b.conn.FlushTimeout(b.cfg.requestTimeout); err != nil {
		closeErr = errors.Join(closeErr, fmt.Errorf("刷新事件总线连接失败: %w", err))
	}
	b.conn.Close()
	return closeErr
}

// durableName 在不同副本/重启间保持同一主题与队列的唯一消费身份。
func durableName(subject, queue string) string {
	return fmt.Sprintf("c_%x", sha256.Sum256([]byte(subject+"\x00"+queue)))
}

// concreteSubject 限定发布/订阅为单个业务主题,拒绝系统主题与通配注入。
func concreteSubject(subject string) bool {
	if subject == "" || strings.HasPrefix(subject, "$") || strings.ContainsAny(subject, " \t\r\n*>") {
		return false
	}
	for _, token := range strings.Split(subject, ".") {
		if token == "" {
			return false
		}
	}
	return true
}

// Decode 统一解码事件 payload,保留调用方给出的模块错误语义。
func Decode(data []byte, dst any, invalid *apperr.Error) error {
	if err := jsonx.DecodeStrictKnownFields(data, dst); err != nil {
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
