// 配置环境变量加载测试。
package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadDotEnvRejectsEmptyKey 确认非法 key 不会被静默跳过。
func TestLoadDotEnvRejectsEmptyKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("=bad\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	err := LoadDotEnv(path)
	if err == nil {
		t.Fatalf("expected empty key to fail")
	}
}
