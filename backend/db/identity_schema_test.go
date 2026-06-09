// identity_schema_test 校验身份模块迁移中的多租户数据库约束,防止只靠 service 校验数据完整性。
package db_test

import (
	"os"
	"strings"
	"testing"
)

// TestIdentityMigrationDefinesTenantScopedForeignKeys 验证组织层级外键同时约束 tenant_id 和父级 ID。
func TestIdentityMigrationDefinesTenantScopedForeignKeys(t *testing.T) {
	source := readIdentityMigration(t)
	for _, want := range []string{
		"UNIQUE (tenant_id, id)",
		"FOREIGN KEY (tenant_id, department_id) REFERENCES department(tenant_id, id)",
		"FOREIGN KEY (tenant_id, major_id) REFERENCES major(tenant_id, id)",
		"FOREIGN KEY (tenant_id, account_id) REFERENCES account(tenant_id, id)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("identity migration 缺少租户一致性外键约束: %s", want)
		}
	}
}

// TestIdentityMigrationValidatesPolymorphicAccountProfileOrg 验证师生统一档案的多态 org_id 由数据库兜底约束。
func TestIdentityMigrationValidatesPolymorphicAccountProfileOrg(t *testing.T) {
	source := readIdentityMigration(t)
	for _, want := range []string{
		"validate_account_profile_org",
		"account_identity = 1",
		"FROM class c",
		"account_identity = 2",
		"FROM department d",
		"trg_account_profile_org_check",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("identity migration 缺少 account_profile.org_id 多态组织校验: %s", want)
		}
	}
}

// TestIdentityMigrationIndexesTenantScopedForeignKeys 验证外键关联字段有索引支撑级联校验和租户内查询。
func TestIdentityMigrationIndexesTenantScopedForeignKeys(t *testing.T) {
	source := readIdentityMigration(t)
	for _, want := range []string{
		"idx_major_tenant_department",
		"idx_class_tenant_major",
		"idx_account_role_tenant_account",
		"idx_account_profile_tenant_account",
		"idx_auth_session_tenant_account_status",
		"idx_activation_code_tenant_account",
		"idx_import_batch_tenant_operator",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("identity migration 缺少外键字段索引: %s", want)
		}
	}
}

// readIdentityMigration 读取未部署前的单一 identity 初始化迁移。
func readIdentityMigration(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("migrations/0001_identity.up.sql")
	if err != nil {
		t.Fatalf("读取 identity migration 失败: %v", err)
	}
	return string(raw)
}
