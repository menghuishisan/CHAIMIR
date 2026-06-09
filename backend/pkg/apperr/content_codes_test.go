// Package apperr 测试 M5 错误码分段与唯一性。
package apperr

import "testing"

// TestContentErrorCodesAreUniqueAndInRange 确认 M5 错误码不复用且保持 5xxxx 分段。
func TestContentErrorCodesAreUniqueAndInRange(t *testing.T) {
	codes := []*Error{
		ErrContentNotFound,
		ErrContentInvalid,
		ErrContentForbidden,
		ErrContentImmutable,
		ErrContentDeleteBlocked,
		ErrContentUnavailable,
		ErrContentRequestInvalid,
		ErrContentIDInvalid,
		ErrContentCategoryInvalid,
		ErrContentQueryFailed,
		ErrContentCategoryQueryFailed,
		ErrContentIntegrity,
		ErrContentReadFailed,
		ErrContentUpdateFailed,
		ErrContentUsageUpdateFailed,
		ErrContentAuditFailed,
		ErrContentTenantInvalid,
		ErrContentCodeConflict,
		ErrContentVersionInvalid,
		ErrContentVersionRequestInvalid,
		ErrContentVersionQueryFailed,
		ErrContentShareInvalid,
		ErrContentCloneInvalid,
		ErrContentCloneRequestInvalid,
		ErrContentShareReadFailed,
		ErrPaperInvalid,
		ErrPaperNotFound,
		ErrPaperRandomNotEnough,
		ErrPaperRequestInvalid,
		ErrPaperIDInvalid,
		ErrPaperQueryFailed,
	}
	seen := map[string]bool{}
	for _, err := range codes {
		if err == nil {
			t.Fatalf("content error code entry is nil")
		}
		if len(err.Code) != 5 || err.Code[0] != '5' {
			t.Fatalf("content error code %s must be in 5xxxx range", err.Code)
		}
		if seen[err.Code] {
			t.Fatalf("duplicate content error code %s", err.Code)
		}
		seen[err.Code] = true
	}
}
