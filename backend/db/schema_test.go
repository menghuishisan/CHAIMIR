// Package dbschema 测试数据库迁移的多租户安全约束。
package dbschema

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	createTablePattern = regexp.MustCompile(`(?is)CREATE TABLE\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\((.*?)\);`)
	tenantArrayPattern = regexp.MustCompile(`(?is)tenant_tables\s+TEXT\[\]\s*:=\s*ARRAY\[(.*?)\]`)
	quotedNamePattern  = regexp.MustCompile(`'([a-zA-Z_][a-zA-Z0-9_]*)'`)
	selectStarPattern  = regexp.MustCompile(`(?i)\bSELECT\s+\*`)
)

// TestTenantTablesAreCoveredByRLSLoops 确认所有含 tenant_id 的租户表都进入本迁移 RLS 列表。
func TestTenantTablesAreCoveredByRLSLoops(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("migrations", "*.up.sql"))
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	for _, path := range files {
		text := readMigration(t, path)
		rlsTables := rlsLoopTables(text)
		for _, table := range createTenantTables(text) {
			if platformScopedTenantColumn(table.name) {
				continue
			}
			if _, ok := rlsTables[table.name]; !ok {
				t.Errorf("%s creates tenant table %s but does not include it in tenant_tables RLS loop", path, table.name)
			}
		}
	}
}

// TestQueriesUseExplicitColumns 防止新增字段或敏感字段被 SELECT * 默认带出。
func TestQueriesUseExplicitColumns(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("queries", "*.sql"))
	if err != nil {
		t.Fatalf("list query files: %v", err)
	}
	for _, path := range files {
		text := readMigration(t, path)
		if loc := selectStarPattern.FindStringIndex(text); loc != nil {
			t.Fatalf("%s must use explicit columns instead of SELECT * near: %q", path, text[loc[0]:min(len(text), loc[1]+80)])
		}
	}
}

type migrationTable struct {
	name string
	body string
}

// readMigration 读取单个迁移文件内容。
func readMigration(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// createTenantTables 提取迁移中包含 tenant_id 字段的建表语句。
func createTenantTables(text string) []migrationTable {
	var out []migrationTable
	for _, match := range createTablePattern.FindAllStringSubmatch(text, -1) {
		if strings.Contains(strings.ToLower(match[2]), "tenant_id") {
			out = append(out, migrationTable{name: match[1], body: match[2]})
		}
	}
	return out
}

// rlsLoopTables 提取迁移中批量启用 RLS 的 tenant_tables 列表。
func rlsLoopTables(text string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, block := range tenantArrayPattern.FindAllStringSubmatch(text, -1) {
		for _, match := range quotedNamePattern.FindAllStringSubmatch(block[1], -1) {
			out[match[1]] = struct{}{}
		}
	}
	return out
}

// platformScopedTenantColumn 判断 tenant_id 只是平台流程结果引用,不是租户隔离键。
func platformScopedTenantColumn(table string) bool {
	switch table {
	case "tenant_application":
		return true
	case "sim_share":
		return true
	default:
		return false
	}
}
