// NATS 事件总线错误处理测试。
package eventbus

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"chaimir/pkg/apperr"

	"github.com/nats-io/nats.go"
)

type publishFlushConn struct {
	publishErr error
	flushErr   error
}

func (c *publishFlushConn) Publish(string, []byte) error { return c.publishErr }
func (c *publishFlushConn) FlushWithContext(context.Context) error {
	return c.flushErr
}
func (c *publishFlushConn) QueueSubscribe(string, string, nats.MsgHandler) (*nats.Subscription, error) {
	return nil, errors.New("unexpected queue subscribe")
}
func (c *publishFlushConn) Subscribe(string, nats.MsgHandler) (*nats.Subscription, error) {
	return nil, errors.New("unexpected subscribe")
}
func (c *publishFlushConn) Close() {}

// TestPublishReturnsFlushFailure 确认事件写入客户端缓冲后仍会等待 NATS 确认,避免发布失败被延迟吞掉。
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

	dispatchMessage(context.Background(), "identity.audit", "workers", []byte(`{}`), func(context.Context, []byte) error {
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

	dispatchMessage(context.Background(), "identity.audit", "workers", []byte(`{}`), func(context.Context, []byte) error {
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

// TestDecodeWrapsInvalidPayloadWithModuleError 确认事件解码复用统一入口且保留模块错误语义。
func TestDecodeWrapsInvalidPayloadWithModuleError(t *testing.T) {
	var payload struct {
		ID string `json:"id"`
	}

	err := Decode([]byte(`{`), &payload, apperr.ErrContestEventInvalid)

	if got, ok := apperr.As(err); !ok || got.Code != apperr.ErrContestEventInvalid.Code {
		t.Fatalf("expected contest event error, got %v", err)
	}
	if !strings.Contains(err.Error(), "unexpected end") {
		t.Fatalf("expected original JSON cause in error chain, got %v", err)
	}
}
