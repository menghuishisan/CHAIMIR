// bootstrap 配置读取测试。
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadReadsBootstrapConfig 确认私有化初始化入口从环境变量读取租户与首个学校管理员参数。
func TestLoadReadsBootstrapConfig(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SCHOOL_TENANT_ID", "1001")
	t.Setenv("BOOTSTRAP_SCHOOL_TENANT_CODE", "school-demo")
	t.Setenv("BOOTSTRAP_SCHOOL_NAME", "示例学校")
	t.Setenv("BOOTSTRAP_SCHOOL_TYPE", "1")
	t.Setenv("BOOTSTRAP_ADMIN_PHONE", "13800138000")
	t.Setenv("BOOTSTRAP_ADMIN_NAME", "首个管理员")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "AdminStrong123")
	t.Setenv("BOOTSTRAP_PLATFORM_ADMIN_USERNAME", "platform-admin")
	t.Setenv("BOOTSTRAP_PLATFORM_ADMIN_NAME", "平台管理员")
	t.Setenv("BOOTSTRAP_PLATFORM_ADMIN_PASSWORD", "PlatformStrong123")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Bootstrap.SchoolTenantID != 1001 ||
		cfg.Bootstrap.SchoolTenantCode != "school-demo" ||
		cfg.Bootstrap.SchoolName != "示例学校" ||
		cfg.Bootstrap.SchoolType != 1 ||
		cfg.Bootstrap.AdminPhone != "13800138000" ||
		cfg.Bootstrap.AdminName != "首个管理员" ||
		cfg.Bootstrap.AdminPassword != "AdminStrong123" ||
		cfg.Bootstrap.PlatformAdminUser != "platform-admin" ||
		cfg.Bootstrap.PlatformAdminName != "平台管理员" ||
		cfg.Bootstrap.PlatformAdminPassword != "PlatformStrong123" {
		t.Fatalf("unexpected bootstrap config: %#v", cfg.Bootstrap)
	}
}

// TestEnvExampleDocumentsBootstrapConfig 确认新增配置进入 .env.example 且每个变量前有说明注释。
func TestEnvExampleDocumentsBootstrapConfig(t *testing.T) {
	root := configRepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, ".env.example"))
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}
	requireDocumentedEnv(t, string(raw),
		"BOOTSTRAP_SCHOOL_TENANT_CODE",
		"BOOTSTRAP_SCHOOL_NAME",
		"BOOTSTRAP_SCHOOL_TYPE",
		"BOOTSTRAP_ADMIN_PHONE",
		"BOOTSTRAP_ADMIN_NAME",
		"BOOTSTRAP_ADMIN_PASSWORD",
		"BOOTSTRAP_PLATFORM_ADMIN_USERNAME",
		"BOOTSTRAP_PLATFORM_ADMIN_NAME",
		"BOOTSTRAP_PLATFORM_ADMIN_PASSWORD",
	)
}

// TestProdSchoolOverlayProvidesFixedTenantID 确认私有化 overlay 与启动配置校验一致。
func TestProdSchoolOverlayProvidesFixedTenantID(t *testing.T) {
	root := configRepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "..", "deploy", "overlays", "prod-school", "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read prod-school overlay: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "DEPLOY_MODE=school") {
		t.Fatalf("prod-school overlay must declare DEPLOY_MODE=school")
	}
	requireNonEmptyLiteral(t, content, "SCHOOL_TENANT_ID")
}

func configRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

func requireNonEmptyLiteral(t *testing.T, content, key string) {
	t.Helper()
	prefix := "- " + key + "="
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			if strings.TrimSpace(strings.TrimPrefix(trimmed, prefix)) == "" {
				t.Fatalf("%s must be non-empty in prod-school overlay", key)
			}
			return
		}
	}
	t.Fatalf("%s missing from prod-school overlay literals", key)
}

func requireDocumentedEnv(t *testing.T, content string, keys ...string) {
	t.Helper()
	lines := strings.Split(content, "\n")
	for _, key := range keys {
		found := false
		for i, line := range lines {
			if strings.HasPrefix(line, key+"=") {
				found = true
				if i == 0 || !strings.HasPrefix(strings.TrimSpace(lines[i-1]), "#") {
					t.Fatalf("%s must have a comment immediately above it in .env.example", key)
				}
			}
		}
		if !found {
			t.Fatalf("%s missing from .env.example", key)
		}
	}
}
