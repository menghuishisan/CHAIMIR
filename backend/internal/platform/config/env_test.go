// config_test 校验环境变量辅助加载逻辑,避免本地开发路径静默吞错。
package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadDotEnvRejectsEmptyKey 确认非法 key 不会被静默跳过。
func TestLoadDotEnvRejectsEmptyKey(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", ".tmp-test", "dotenv")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir test root: %v", err)
	}
	path := filepath.Join(root, "invalid.env")
	t.Cleanup(func() { _ = os.Remove(path) })
	if err := os.WriteFile(path, []byte("=bad\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	err := LoadDotEnv(path)
	if err == nil {
		t.Fatalf("expected empty key to fail")
	}
}
