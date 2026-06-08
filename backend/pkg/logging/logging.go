// Package logging 配置全平台结构化日志(slog),并提供统一日志上下文与脱敏入口。
// 依据 CLAUDE.md §5/§8:错误日志必须结构化、含 trace/tenant 等定位字段,敏感值不能落盘。
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
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lv}
	var h slog.Handler
	if strings.ToLower(format) == "text" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts) // 生产 json,便于集中采集。
	}
	slog.SetDefault(slog.New(h))
}

// WithAttrs 把通用定位字段追加到 context,供响应层、平台层和业务层统一写日志。
func WithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	if len(attrs) == 0 {
		return ctx
	}

	// pkg 层只保存 slog.Attr,不认识 tenant/auth 等 internal 类型,避免通用日志包反向耦合平台上下文。
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

	// 返回新切片,避免调用方追加字段时改到 context 内已保存的共享字段集合。
	out := make([]slog.Attr, 0, len(base)+len(extra))
	out = append(out, base...)
	out = append(out, extra...)
	return out
}

// ErrorContext 写统一错误日志:上下文字段来自 context,错误链先脱敏再落盘。
func ErrorContext(ctx context.Context, msg string, err string, attrs ...slog.Attr) {
	all := AttrsFromContext(ctx, attrs...)

	// 错误链保留给运维排查,但密钥、令牌等敏感值必须在进入 handler 前脱敏。
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

// SanitizeError 对日志错误文本做强制脱敏,覆盖 key/value、JSON 和常见连接串凭据形态。
func SanitizeError(raw string) string {
	masked := raw
	for _, pattern := range sensitiveValuePatterns {
		// 日志入口必须先脱敏再落盘;无法识别的格式由后续规则继续覆盖。
		masked = pattern.re.ReplaceAllString(masked, pattern.replacement)
	}
	return masked
}
