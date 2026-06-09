// k8s 测试 Kubernetes 基础配置装载边界。
package k8s

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildRestConfigRejectsMissingFile 确认 kubeconfig 路径非法时不会静默回退。
func TestBuildRestConfigRejectsMissingFile(t *testing.T) {
	_, err := buildRestConfig(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatalf("expected missing kubeconfig to fail")
	}
}

// TestBuildRestConfigLoadsKubeconfig 确认本地 kubeconfig 能被正确解析。
func TestBuildRestConfigLoadsKubeconfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := `
apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: local
  context:
    cluster: local
    user: local
current-context: local
users:
- name: local
  user:
    token: test-token
`
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	cfg, err := buildRestConfig(path)
	if err != nil {
		t.Fatalf("buildRestConfig() error = %v", err)
	}
	if cfg.Host != "https://127.0.0.1:6443" {
		t.Fatalf("unexpected kube host: %s", cfg.Host)
	}
}
