// apperr 测试:验证应用错误不会向响应层泄露内部错误链。
package apperr

import (
	"errors"
	"fmt"
	"testing"
)

// TestAppErrorKeepsCauseForLogsButHidesDefaultString 确认内部原因只通过 Unwrap/%+v 给日志使用。
func TestAppErrorKeepsCauseForLogsButHidesDefaultString(t *testing.T) {
	cause := errors.New("sql: password leaked")
	err := ErrBadRequest.WithCause(cause)

	if err.Error() == "" || err.Error() == ErrBadRequest.Message {
		t.Fatalf("default error string leaked cause: %q", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Fatalf("wrapped cause should remain available for logs")
	}
	if got := fmt.Sprintf("%+v", err); got == err.Message || got == "" {
		t.Fatalf("verbose format should include diagnostic chain, got %q", got)
	}
}

// TestAsAppErrorWrapsUnknownError 确认未知错误统一收敛为内部错误码。
func TestAsAppErrorWrapsUnknownError(t *testing.T) {
	err := AsAppError(errors.New("driver failure"))
	if err.UserCode() != CodeInternal {
		t.Fatalf("unknown error code = %s", err.UserCode())
	}
}

// TestPlatformGenericErrorCodesStayUnique 确认 pkg 层平台通用错误码一错一码,不共享编码。
func TestPlatformGenericErrorCodesStayUnique(t *testing.T) {
	codes := []*Error{
		ErrInternal,
		ErrUnauthorized,
		ErrForbidden,
		ErrCrossTenant,
		ErrBadRequest,
		ErrNotFound,
		ErrConflict,
		ErrRateLimited,
		ErrServiceUnauthorized,
		ErrAuditActorResolveFailed,
		ErrPathIDInvalid,
		ErrRequestBodyInvalid,
		ErrQueryParamInvalid,
		ErrUnhandledFailure,
		ErrPanicRecovered,
	}
	seen := map[string]bool{}
	for _, item := range codes {
		if item == nil || item.Code == "" {
			t.Fatalf("platform error entry is empty")
		}
		if seen[item.Code] {
			t.Fatalf("duplicate platform error code %s", item.Code)
		}
		seen[item.Code] = true
	}
}

// TestAsAppErrorExposesStableChineseFields 确认应用错误读取接口保持中文文案和稳定错误码。
func TestAsAppErrorExposesStableChineseFields(t *testing.T) {
	err := ErrForbidden.WithCause(errors.New("driver failed"))
	if err.UserCode() != ErrForbidden.Code {
		t.Fatalf("code accessor mismatch: %s vs %s", err.UserCode(), ErrForbidden.Code)
	}
	if err.UserMessage() != ErrForbidden.Message {
		t.Fatalf("message accessor mismatch: %s vs %s", err.UserMessage(), ErrForbidden.Message)
	}
}
