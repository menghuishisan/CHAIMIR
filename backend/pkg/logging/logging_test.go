// Package logging 的测试覆盖全平台日志上下文字段与敏感信息脱敏约束。
package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// TestAttrsFromContextMergesRequestFields 确认日志字段可以沿 context 逐层合并。
func TestAttrsFromContextMergesRequestFields(t *testing.T) {
	ctx := WithAttrs(context.Background(), slog.String("trace_id", "trace-001"))
	ctx = WithAttrs(ctx, slog.Int64("tenant_id", 10), slog.Int64("account_id", 20))

	attrs := AttrsFromContext(ctx, slog.String("operation", "login"))

	got := map[string]string{}
	for _, attr := range attrs {
		got[attr.Key] = attr.Value.String()
	}
	if got["trace_id"] != "trace-001" || got["tenant_id"] != "10" || got["account_id"] != "20" || got["operation"] != "login" {
		t.Fatalf("unexpected attrs: %#v", got)
	}
}

// TestSanitizeErrorMasksSensitiveValues 确认日志中的内部错误链不会泄漏常见密钥字段。
func TestSanitizeErrorMasksSensitiveValues(t *testing.T) {
	raw := "connect failed password=secret token: abc Authorization=Bearer xyz"

	masked := SanitizeError(raw)

	for _, leaked := range []string{"secret", "abc", "Bearer xyz"} {
		if strings.Contains(masked, leaked) {
			t.Fatalf("masked error still contains sensitive value %q: %s", leaked, masked)
		}
	}
	for _, marker := range []string{"password=", "token:", "Authorization="} {
		if !strings.Contains(masked, marker) {
			t.Fatalf("masked error lost diagnostic key %q: %s", marker, masked)
		}
	}
}

// TestSanitizeErrorMasksStructuredSecrets 确认 JSON/URL/DSN 形态的敏感值也不会进入日志。
func TestSanitizeErrorMasksStructuredSecrets(t *testing.T) {
	raw := `{"password":"secret","token":"abc","nested":{"secret":"value"}} postgres://owner:p@ss@db/chaimir?sslmode=disable`

	masked := SanitizeError(raw)

	for _, leaked := range []string{`"password":"secret"`, `"token":"abc"`, `"secret":"value"`, "p@ss"} {
		if strings.Contains(masked, leaked) {
			t.Fatalf("masked structured error still contains %q: %s", leaked, masked)
		}
	}
}

// TestErrorContextWritesMergedSanitizedAttrs 确认统一错误日志入口同时输出上下文与脱敏错误。
func TestErrorContextWritesMergedSanitizedAttrs(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	ctx := WithAttrs(context.Background(), slog.String("trace_id", "trace-001"), slog.Int64("tenant_id", 10))
	ErrorContext(ctx, "request failed", "db password=secret", slog.String("operation", "login"))

	out := buf.String()
	for _, want := range []string{`"trace_id":"trace-001"`, `"tenant_id":10`, `"operation":"login"`, `"error":"db password=***"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("log missing %s: %s", want, out)
		}
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("log leaked sensitive value: %s", out)
	}
}
