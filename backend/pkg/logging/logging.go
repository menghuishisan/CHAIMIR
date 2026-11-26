// Package logging 配置结构化日志并统一错误脱敏。
package logging

import (
	"context"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"chaimir/pkg/privacy"
)

type attrsCtxKey struct{}

type errorCloser interface {
	Close() error
}

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

type secretPattern struct {
	re          *regexp.Regexp
	replacement string
}

var (
	credentialAssignmentPattern = regexp.MustCompile(`(?i)("?([a-z][a-z0-9_-]*)"?\s*[=:]\s*)("(?:[^"\\\r\n]|\\[^\r\n])*"|(?:(?:bearer|basic)\s+)?[^,\s;]+)`)
	connectionSecretPatterns    = []secretPattern{
		{regexp.MustCompile(`(?i)(postgres(?:ql)?://[^:\s/@]+:)([^@\s]+)(@)`), `${1}***${3}`},
		{regexp.MustCompile(`(?i)(redis://[^:\s/@]+:)([^@\s]+)(@)`), `${1}***${3}`},
	}
)

var phonePattern = regexp.MustCompile(`\b1[3-9]\d{9}\b`)

// SanitizeError 对日志错误文本做强制脱敏,覆盖 key/value、JSON 和常见连接串凭据形态。
func SanitizeError(raw string) string {
	masked := credentialAssignmentPattern.ReplaceAllStringFunc(raw, redactCredentialAssignment)
	for _, pattern := range connectionSecretPatterns {
		masked = pattern.re.ReplaceAllString(masked, pattern.replacement)
	}
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
	}
	return attr
}
