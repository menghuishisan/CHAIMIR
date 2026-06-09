// sandbox 装配测试:覆盖 M2 生产装配不能静默缺失 K8s 编排能力。
package main

import (
	"context"
	"strings"
	"testing"

	"chaimir/internal/platform/config"
)

// TestAssembleSandboxRequiresK8sClient 确认 M2 装配缺少 K8s 客户端时 fail-fast。
func TestAssembleSandboxRequiresK8sClient(t *testing.T) {
	server := newHTTPServer(config.ServerConfig{Addr: "127.0.0.1", Port: 0, AppEnv: "test"}, nil)
	err := assembleSandbox(&moduleDeps{
		ctx: context.Background(),
		cfg: &config.Config{},
		infra: &infra{
			server: server,
		},
	})
	if err == nil {
		t.Fatalf("expected sandbox assembly to fail without k8s client")
	}
	if !strings.Contains(err.Error(), "K8s 客户端不可用") {
		t.Fatalf("unexpected error: %v", err)
	}
}
