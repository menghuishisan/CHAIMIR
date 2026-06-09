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

// TestBackgroundWorkersUseProcessContext 确认模块后台任务随服务进程 context 统一停止。
func TestBackgroundWorkersUseProcessContext(t *testing.T) {
	for _, path := range []string{"sandbox.go", "judge.go", "teaching.go"} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		body := string(src)
		if strings.Contains(body, "context.Background()") {
			t.Fatalf("%s must not start module background workers with context.Background()", path)
		}
		if !strings.Contains(body, "d.ctx") {
			t.Fatalf("%s must use moduleDeps.ctx for background workers", path)
		}
	}
}

// TestAssembleModulesRequiresProcessContext 确认模块装配缺少进程 context 时 fail-fast。
func TestAssembleModulesRequiresProcessContext(t *testing.T) {
	err := assembleModules(&moduleDeps{})
	if err == nil {
		t.Fatalf("assembleModules must reject nil process context")
	}
	if !strings.Contains(err.Error(), "缺少进程 context") {
		t.Fatalf("unexpected error: %v", err)
	}
}
