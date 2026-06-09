// eventbus_test 校验 NATS 事件总线的发布确认、错误脱敏与解码语义。
package eventbus

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/pkg/apperr"

	"github.com/nats-io/nats.go"
)

type publishFlushConn struct {
	publishErr error
	flushErr   error
}

// Publish 模拟底层 NATS publish。
func (c *publishFlushConn) Publish(string, []byte) error { return c.publishErr }

// FlushWithContext 模拟底层 flush 确认。
func (c *publishFlushConn) FlushWithContext(context.Context) error { return c.flushErr }

// QueueSubscribe 不应在当前测试路径被调用。
func (c *publishFlushConn) QueueSubscribe(string, string, nats.MsgHandler) (*nats.Subscription, error) {
	return nil, errors.New("unexpected queue subscribe")
}

// Subscribe 不应在当前测试路径被调用。
func (c *publishFlushConn) Subscribe(string, nats.MsgHandler) (*nats.Subscription, error) {
	return nil, errors.New("unexpected subscribe")
}

// Close 满足接口即可。
func (c *publishFlushConn) Close() {}

type subscribeConn struct {
	queueHandler nats.MsgHandler
	handler      nats.MsgHandler
	published    []publishedMessage
}

type publishedMessage struct {
	subject string
	data    []byte
}

// Publish 记录发布行为,供死信测试断言。
func (c *subscribeConn) Publish(subject string, data []byte) error {
	c.published = append(c.published, publishedMessage{subject: subject, data: append([]byte(nil), data...)})
	return nil
}

// FlushWithContext 模拟立即确认发布。
func (c *subscribeConn) FlushWithContext(context.Context) error { return nil }

// QueueSubscribe 记录队列订阅回调。
func (c *subscribeConn) QueueSubscribe(_ string, _ string, cb nats.MsgHandler) (*nats.Subscription, error) {
	c.queueHandler = cb
	return &nats.Subscription{}, nil
}

// Subscribe 记录普通订阅回调。
func (c *subscribeConn) Subscribe(_ string, cb nats.MsgHandler) (*nats.Subscription, error) {
	c.handler = cb
	return &nats.Subscription{}, nil
}

// Close 满足接口即可。
func (c *subscribeConn) Close() {}

// TestPublishReturnsFlushFailure 确认事件发布后仍会等待 NATS flush 确认。
func TestPublishReturnsFlushFailure(t *testing.T) {
	bus := &natsBus{conn: &publishFlushConn{flushErr: errors.New("flush failed")}}

	err := bus.Publish(context.Background(), "identity.audit", map[string]string{"id": "1"})

	if err == nil || !strings.Contains(err.Error(), "flush failed") {
		t.Fatalf("expected flush failure, got %v", err)
	}
}

// TestDispatchMessageLogsHandlerError 确认订阅处理失败不会被静默丢弃。
func TestDispatchMessageLogsHandlerError(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	bus := &natsBus{cfg: deliveryConfig{retryMax: 1}}
	bus.handleMessage(context.Background(), "identity.audit", "workers", []byte(`{}`), func(context.Context, []byte) error {
		return errors.New("handler failed")
	})

	got := buf.String()
	if !strings.Contains(got, "事件处理失败") || !strings.Contains(got, "identity.audit") {
		t.Fatalf("expected structured error log, got %q", got)
	}
}

// TestDispatchMessageMasksHandlerError 确认事件处理错误日志复用统一脱敏规则。
func TestDispatchMessageMasksHandlerError(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	bus := &natsBus{cfg: deliveryConfig{retryMax: 1}}
	bus.handleMessage(context.Background(), "identity.audit", "workers", []byte(`{}`), func(context.Context, []byte) error {
		return errors.New("handler failed password=secret")
	})

	got := buf.String()
	if strings.Contains(got, "secret") {
		t.Fatalf("event handler log leaked sensitive value: %q", got)
	}
	if !strings.Contains(got, "password=***") {
		t.Fatalf("event handler log did not keep masked diagnostic key: %q", got)
	}
}

// TestDecodeWrapsInvalidPayloadWithModuleError 确认事件解码复用统一入口并保留模块错误语义。
func TestDecodeWrapsInvalidPayloadWithModuleError(t *testing.T) {
	var payload struct {
		ID string `json:"id"`
	}

	err := Decode([]byte(`{`), &payload, apperr.ErrBadRequest)

	if got, ok := apperr.As(err); !ok || got.Code != apperr.ErrBadRequest.Code {
		t.Fatalf("expected wrapped module error, got %v", err)
	}
	if !strings.Contains(err.Error(), "unexpected end") {
		t.Fatalf("expected original JSON cause in error chain, got %v", err)
	}
}

// TestSubscribeRetriesHandlerAndPublishesDeadLetter 确认事件消费失败会按配置重试并在耗尽后进入死信。
func TestSubscribeRetriesHandlerAndPublishesDeadLetter(t *testing.T) {
	conn := &subscribeConn{}
	bus := &natsBus{
		conn: conn,
		cfg: deliveryConfig{
			retryMax:     3,
			retryDelay:   time.Millisecond,
			deadSubject:  "dead.identity.audit",
			publishFlush: true,
		},
	}

	attempts := 0
	_, err := bus.Subscribe("identity.audit", "workers", func(context.Context, []byte) error {
		attempts++
		return errors.New("handler failed")
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	if conn.queueHandler == nil {
		t.Fatalf("queue subscribe handler not registered")
	}

	conn.queueHandler(&nats.Msg{Subject: "identity.audit", Data: []byte(`{"id":"1"}`)})

	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if len(conn.published) != 1 {
		t.Fatalf("dead-letter publish count = %d, want 1", len(conn.published))
	}
	if conn.published[0].subject != "dead.identity.audit" {
		t.Fatalf("dead-letter subject = %q, want dead.identity.audit", conn.published[0].subject)
	}
	if !strings.Contains(string(conn.published[0].data), `"original_subject":"identity.audit"`) {
		t.Fatalf("dead-letter payload missing original subject: %s", conn.published[0].data)
	}
}

// TestNewBuildsDeadLetterSubjectFromConfig 确认事件总线会根据统一配置推导死信主题。
func TestNewBuildsDeadLetterSubjectFromConfig(t *testing.T) {
	cfg := config.NATSConfig{
		URL:                  "nats://127.0.0.1:4222",
		ReconnectWaitSeconds: 1,
		ConsumerRetryMax:     2,
		ConsumerRetryDelayMs: 50,
		DeadLetterPrefix:     "dead",
	}

	delivery := newDeliveryConfig(cfg)

	if delivery.retryMax != 2 {
		t.Fatalf("retryMax = %d, want 2", delivery.retryMax)
	}
	if delivery.retryDelay != 50*time.Millisecond {
		t.Fatalf("retryDelay = %v, want 50ms", delivery.retryDelay)
	}
	if got := delivery.deadLetterSubject("notify.push"); got != "dead.notify.push" {
		t.Fatalf("dead-letter subject = %q, want dead.notify.push", got)
	}
}
