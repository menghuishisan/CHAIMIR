// M1 账号列表查询参数测试。
package identity

import (
	"testing"

	"chaimir/pkg/apperr"
)

// TestBuildAccountListFilterUsesDocumentedRoleAndClassID 确认账号列表使用文档规定的 role/class_id 过滤。
func TestBuildAccountListFilterUsesDocumentedRoleAndClassID(t *testing.T) {
	filter, err := buildAccountListFilter("teacher", "3001", "2", "张")
	if err != nil {
		t.Fatalf("build account list filter: %v", err)
	}
	if filter.Role != RoleTeacher || filter.ClassID != 3001 || filter.Status != AccountActive || filter.Keyword != "张" {
		t.Fatalf("unexpected account list filter: %#v", filter)
	}
}

// TestBuildAccountListFilterRejectsUnknownRole 确认未知角色不会被静默当作不过滤。
func TestBuildAccountListFilterRejectsUnknownRole(t *testing.T) {
	_, err := buildAccountListFilter("unknown-role", "", "", "")
	if err != apperr.ErrAccountQueryInvalid {
		t.Fatalf("expected account query error for unknown role, got %v", err)
	}
}

// TestBuildAccountListFilterRejectsInvalidClassAndStatus 确认列表查询的不同非法参数不会落到全局错误码。
func TestBuildAccountListFilterRejectsInvalidClassAndStatus(t *testing.T) {
	for _, tc := range []struct {
		name        string
		classIDText string
		statusText  string
	}{
		{name: "class id", classIDText: "abc"},
		{name: "status", statusText: "999"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildAccountListFilter("", tc.classIDText, tc.statusText, "")
			if err != apperr.ErrAccountQueryInvalid {
				t.Fatalf("expected account query error, got %v", err)
			}
		})
	}
}
