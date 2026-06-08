// M2 数据库迁移结构测试:守护沙箱表关系约束与 RLS 基线。
package sandbox

import (
	"os"
	"strings"
	"testing"
)

// TestSandboxMigrationDeclaresForeignKeys 确认 M2 自有表之间有外键约束,避免生产产生孤儿配置或事件。
func TestSandboxMigrationDeclaresForeignKeys(t *testing.T) {
	data, err := os.ReadFile("../../../db/migrations/0003_sandbox.up.sql")
	if err != nil {
		t.Fatalf("read sandbox migration: %v", err)
	}
	body := string(data)
	for _, required := range []string{
		"runtime_image_runtime_fk",
		"sandbox_runtime_fk",
		"sandbox_image_fk",
		"sandbox_tool_sandbox_fk",
		"sandbox_tool_tool_fk",
		"sandbox_event_sandbox_fk",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("sandbox migration missing foreign key %s", required)
		}
	}
}
