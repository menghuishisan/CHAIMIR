// logging 配置全平台结构化日志(slog),并提供统一日志上下文与脱敏入口。
package logging

import (
	"context"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

type attrsCtxKey struct{}

// Setup 按配置初始化全局 slog logger(json/text + level)。
func Setup(level, format string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl, ReplaceAttr: redactAttr}
	if strings.EqualFold(format, "text") {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, opts)))
		return
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, opts)))
}

// WithAttrs 把通用定位字段追加到 context,供响应层、平台层和业务层统一写日志。
func WithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	if len(attrs) == 0 {
		return ctx
	}
	existing := AttrsFromContext(ctx)
	merged := make([]slog.Attr, 0, len(existing)+len(attrs))
	merged = append(merged, existing...)
	merged = append(merged, attrs...)
	return context.WithValue(ctx, attrsCtxKey{}, merged)
}

// AttrsFromContext 读取已注入的日志字段,并可附加本次操作的局部字段。
func AttrsFromContext(ctx context.Context, extra ...slog.Attr) []slog.Attr {
	var base []slog.Attr
	if attrs, ok := ctx.Value(attrsCtxKey{}).([]slog.Attr); ok {
		base = attrs
	}
	out := make([]slog.Attr, 0, len(base)+len(extra))
	out = append(out, base...)
	out = append(out, extra...)
	return out
}

// ErrorContext 写统一错误日志:上下文字段来自 context,错误链先脱敏再落盘。
func ErrorContext(ctx context.Context, msg string, err string, attrs ...slog.Attr) {
	all := AttrsFromContext(ctx, attrs...)
	all = append(all, slog.String("error", SanitizeError(err)))
	slog.Default().LogAttrs(ctx, slog.LevelError, msg, all...)
}

type secretPattern struct {
	re          *regexp.Regexp
	replacement string
}

var sensitiveValuePatterns = []secretPattern{
	{regexp.MustCompile(`(?i)(password\s*[=:]\s*)([^,\s;]+)`), `${1}***`},
	{regexp.MustCompile(`(?i)("password"\s*:\s*")([^"]+)(")`), `${1}***${3}`},
	{regexp.MustCompile(`(?i)(token\s*[=:]\s*)([^,\s;]+)`), `${1}***`},
	{regexp.MustCompile(`(?i)("token"\s*:\s*")([^"]+)(")`), `${1}***${3}`},
	{regexp.MustCompile(`(?i)(authorization\s*[=:]\s*)([^,\s;]+(?:\s+[^,\s;]+)?)`), `${1}***`},
	{regexp.MustCompile(`(?i)("authorization"\s*:\s*")([^"]+)(")`), `${1}***${3}`},
	{regexp.MustCompile(`(?i)(secret\s*[=:]\s*)([^,\s;]+)`), `${1}***`},
	{regexp.MustCompile(`(?i)("secret"\s*:\s*")([^"]+)(")`), `${1}***${3}`},
	{regexp.MustCompile(`(?i)(access[_-]?key\s*[=:]\s*)([^,\s;]+)`), `${1}***`},
	{regexp.MustCompile(`(?i)("access[_-]?key"\s*:\s*")([^"]+)(")`), `${1}***${3}`},
	{regexp.MustCompile(`(?i)(postgres(?:ql)?://[^:\s/@]+:)([^@\s]+)(@)`), `${1}***${3}`},
	{regexp.MustCompile(`(?i)(redis://[^:\s/@]+:)([^@\s]+)(@)`), `${1}***${3}`},
}

var phonePattern = regexp.MustCompile(`\b1[3-9]\d{9}\b`)

// SanitizeError 对日志错误文本做强制脱敏,覆盖 key/value、JSON 和常见连接串凭据形态。
func SanitizeError(raw string) string {
	masked := raw
	for _, pattern := range sensitiveValuePatterns {
		masked = pattern.re.ReplaceAllString(masked, pattern.replacement)
	}
	masked = phonePattern.ReplaceAllStringFunc(masked, maskPhoneNumber)
	return masked
}

// maskPhoneNumber 按文档要求把手机号掩码为 138****1234 形态。
func maskPhoneNumber(phone string) string {
	if len(phone) != 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}

// redactAttr 在 handler 输出前按字段名兜底脱敏,避免结构化字段绕过字符串规则。
func redactAttr(_ []string, attr slog.Attr) slog.Attr {
	key := strings.ToLower(attr.Key)
	for _, marker := range []string{"password", "secret", "token", "key", "credential"} {
		if strings.Contains(key, marker) && attr.Value.Kind() == slog.KindString {
			attr.Value = slog.StringValue(SanitizeError(attr.Value.String()))
			return attr
		}
	}
	return attr
}
