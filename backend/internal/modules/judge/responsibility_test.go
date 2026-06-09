// M3 职责守护测试:防止 service/worker 绕过 repo 与转换边界直接承担数据访问职责。
package judge

import (
	"os"
	"strings"
	"testing"
)

// TestJudgeServiceAndWorkerDoNotUsePersistenceTypes 确认 M3 编排层不直接依赖 sqlc、事务入口或数据库列类型。
func TestJudgeServiceAndWorkerDoNotUsePersistenceTypes(t *testing.T) {
	for _, file := range []string{"service.go", "service_worker.go"} {
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
