// 后端启动入口测试。
package main

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadRuntimeConfigReturnsDotEnvError 确认 .env 加载失败会阻止启动。
func TestLoadRuntimeConfigReturnsDotEnvError(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("=bad\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	_, err := loadRuntimeConfig(envPath)
	if err == nil {
		t.Fatalf("expected invalid .env to fail")
	}
}

// TestLogStartupFailureMasksSensitiveCause 确认致命启动错误也复用统一日志脱敏规则。
func TestLogStartupFailureMasksSensitiveCause(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	logStartupFailure(errors.New("open db password=secret"))

	got := buf.String()
	if strings.Contains(got, "secret") {
		t.Fatalf("startup log leaked sensitive value: %q", got)
	}
	if !strings.Contains(got, "password=***") || !strings.Contains(got, "服务启动失败") {
		t.Fatalf("startup log missing masked diagnostic context: %q", got)
	}
}
