// M2 职责守护测试:防止沙箱编排层绕过 repo 直接承担 sqlc 数据访问职责。
package sandbox

import (
	"os"
	"strings"
	"testing"
)

// TestSandboxOrchestrationFilesDoNotUsePersistenceTypes 确认 M2 编排文件不直接依赖 sqlc、事务入口或数据库列类型。
func TestSandboxOrchestrationFilesDoNotUsePersistenceTypes(t *testing.T) {
	for _, file := range []string{
		"service.go",
		"service_files.go",
		"service_interaction.go",
		"service_runtime_admin.go",
		"service_selftest.go",
		"service_scheduler.go",
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		content := string(data)
		for _, forbidden := range []string{
			"/internal/sqlcgen",
			"sqlcgen.",
			"repo.inTenant",
			"repo.inTenantID",
			"repo.inApp",
			"repo.inMaintenancePrivileged",
			"db.IsNoRows",
			"pgtypex.",
			"pgtype.",
		} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s mixes orchestration with persistence boundary via %s", file, forbidden)
			}
		}
	}
}
