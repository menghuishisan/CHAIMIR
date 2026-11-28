// Package logging 配置结构化日志并统一错误脱敏。
package logging

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"chaimir/pkg/privacy"
)

type attrsCtxKey struct{}

const traceKey = "trace_id"

// WithTrace 把追踪编号交给通用日志上下文保存。
func WithTrace(ctx context.Context, traceID string) context.Context {
	return WithAttrs(ctx, slog.String(traceKey, traceID))
}

// TraceFromContext 返回日志、事件和响应共用的追踪编号。
func TraceFromContext(ctx context.Context) string {
	attrs, _ := ctx.Value(attrsCtxKey{}).([]slog.Attr)
	for _, attr := range attrs {
		if attr.Key == traceKey && attr.Value.Kind() == slog.KindString {
			return attr.Value.String()
		}
	}
	return ""
}

type errorCloser interface {
	Close() error
}

type Settings struct {
	level slog.Level
	text  bool
}

// ParseSettings 校验各进程共用的日志配置。
func ParseSettings(level, format string) (Settings, error) {
	var cfg Settings
	var levelErr, formatErr error
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		cfg.level = slog.LevelDebug
	case "info":
		cfg.level = slog.LevelInfo
	case "warn":
		cfg.level = slog.LevelWarn
	case "error":
		cfg.level = slog.LevelError
	default:
		levelErr = errors.New("LOG_LEVEL 必须为 debug、info、warn 或 error")
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
	case "text":
		cfg.text = true
	default:
		formatErr = errors.New("LOG_FORMAT 必须为 json 或 text")
	}
	return cfg, errors.Join(levelErr, formatErr)
}

// Setup 使用已校验的配置初始化全局 slog logger。
func Setup(cfg Settings) {
	opts := &slog.HandlerOptions{Level: cfg.level, ReplaceAttr: redactAttr}
	if cfg.text {
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
	return context.WithValue(ctx, attrsCtxKey{}, AttrsFromContext(ctx, attrs...))
}

// AttrsFromContext 返回独立的属性切片，局部操作的同名字段覆盖继承值。
func AttrsFromContext(ctx context.Context, extra ...slog.Attr) []slog.Attr {
	base, _ := ctx.Value(attrsCtxKey{}).([]slog.Attr)
	out := make([]slog.Attr, len(base), len(base)+len(extra))
	copy(out, base)
	for _, attr := range extra {
		index := -1
		// 空名字用于展开 slog 的匿名分组，允许重复。
		if attr.Key != "" {
			for i := range out {
				if out[i].Key == attr.Key {
					index = i
					break
				}
			}
		}
		if index >= 0 {
			out[index] = attr
		} else {
			out = append(out, attr)
		}
	}
	return out
}

// ErrorContext 写统一错误日志:上下文字段来自 context,错误链先脱敏再落盘。
func ErrorContext(ctx context.Context, msg string, err string, attrs ...slog.Attr) {
	all := AttrsFromContext(ctx, attrs...)
	all = append(all, slog.String("error", SanitizeError(err)))
	slog.Default().LogAttrs(ctx, slog.LevelError, msg, all...)
}

// CloseContext 关闭带错误返回的资源并记录失败,避免 defer Close() 静默吞掉生产诊断信号。
func CloseContext(ctx context.Context, msg string, closer errorCloser, attrs ...slog.Attr) {
	if closer == nil {
		return
	}
	if err := closer.Close(); err != nil {
		ErrorContext(ctx, msg, err.Error(), attrs...)
	}
}

var (
	credentialAssignmentPattern = regexp.MustCompile(`(?i)("?([a-z][a-z0-9_-]*)"?\s*[=:]\s*)("(?:[^"\\\r\n]|\\[^\r\n])*"|(?:(?:bearer|basic)\s+)?[^,\s;]+)`)
	connectionSecretPattern     = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://[^:\s/@]*:)([^@/\s]+)(@)`)
)

var phonePattern = regexp.MustCompile(`\b1[3-9]\d{9}\b`)

// SanitizeError 对日志错误文本做强制脱敏,覆盖 key/value、JSON 和常见连接串凭据形态。
func SanitizeError(raw string) string {
	masked := credentialAssignmentPattern.ReplaceAllStringFunc(raw, redactCredentialAssignment)
	masked = connectionSecretPattern.ReplaceAllString(masked, `${1}***${3}`)
	return phonePattern.ReplaceAllStringFunc(masked, privacy.MaskPhone)
}

// redactCredentialAssignment 使用 privacy 的统一凭据字段口径替换错误文本中的 key/value 值。
func redactCredentialAssignment(assignment string) string {
	parts := credentialAssignmentPattern.FindStringSubmatch(assignment)
	if len(parts) != 4 || !privacy.IsCredentialKey(parts[2]) {
		return assignment
	}
	value := "***"
	if strings.HasPrefix(parts[3], `"`) {
		value = `"***"`
	}
	return parts[1] + value
}

// redactAttr 在 handler 输出前按字段名兜底脱敏,避免非字符串结构化字段绕过字符串规则。
func redactAttr(_ []string, attr slog.Attr) slog.Attr {
	if privacy.IsCredentialKey(attr.Key) {
		attr.Value = slog.StringValue("***")
		return attr
	}
	switch attr.Value.Kind() {
	case slog.KindString:
		attr.Value = slog.StringValue(SanitizeError(attr.Value.String()))
	case slog.KindAny:
		if err, ok := attr.Value.Any().(error); ok {
			attr.Value = slog.StringValue(SanitizeError(err.Error()))
		}
	}
	return attr
}
