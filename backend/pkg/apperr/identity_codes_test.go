// Package apperr 测试身份错误码语义不保留废弃登录路径。
package apperr

import (
	"os"
	"strings"
	"testing"
)

// TestMustChangePasswordIsLoginResultFlag 确认首登改密不再作为登录失败错误码暴露。
func TestMustChangePasswordIsLoginResultFlag(t *testing.T) {
	data, err := os.ReadFile("identity_codes.go")
	if err != nil {
		t.Fatalf("read identity codes: %v", err)
	}
	if strings.Contains(string(data), "ErrMustChangePassword") {
		t.Fatalf("must_change_pwd is a login result flag, not a standalone login failure error")
	}
}

// TestIdentitySpecificCodesCoverAccountAndImportConflicts 确认 M1 不用通用冲突/未命中码承载账号和导入语义。
func TestIdentitySpecificCodesCoverAccountAndImportConflicts(t *testing.T) {
	codes := []*Error{
		ErrAccountNoAlreadyExists,
		ErrAccountStatusTransitionInvalid,
		ErrBatchAccountArchiveInvalid,
		ErrImportPreviewNotFound,
	}
	seen := map[string]bool{}
	for _, item := range codes {
		if item.Code == "" || item.Code[0] != '1' {
			t.Fatalf("identity code must be in 1xxxx segment, got %q", item.Code)
		}
		if seen[item.Code] {
			t.Fatalf("duplicate identity code %s", item.Code)
		}
		seen[item.Code] = true
	}
}
