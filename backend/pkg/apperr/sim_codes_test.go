// Package apperr 测试 M4 错误码分段与唯一性。
package apperr

import "testing"

// TestSimErrorCodesAreUniqueAndInRange 确认 M4 错误码不复用且保持 4xxxx 分段。
func TestSimErrorCodesAreUniqueAndInRange(t *testing.T) {
	codes := []*Error{
		ErrSimPackageNotFound,
		ErrSimPackageInvalid,
		ErrSimPackageVersionConflict,
		ErrSimPackageUnavailable,
		ErrSimPackageValidationFail,
		ErrSimBundleReadFail,
		ErrSimBundleTooLarge,
		ErrSimPackageQueryFailed,
		ErrSimPackageUpdateFailed,
		ErrSimSessionNotFound,
		ErrSimSessionInvalid,
		ErrSimSessionInvalidState,
		ErrSimActionInvalid,
		ErrSimBackendUnavailable,
		ErrSimCheckpointInvalid,
		ErrSimShareInvalid,
		ErrSimAccessDenied,
		ErrSimEventPublish,
		ErrSimShareCodeGenerate,
		ErrSimReplayReadFailed,
		ErrSimAuditFailed,
		ErrSimShareCreateFailed,
		ErrSimShareReadFailed,
		ErrSimReviewNotFound,
		ErrSimReviewInvalidState,
		ErrSimReviewQueryFailed,
		ErrSimReviewUpdateFailed,
	}
	seen := map[string]bool{}
	for _, err := range codes {
		if err == nil {
			t.Fatalf("sim error code entry is nil")
		}
		if len(err.Code) != 5 || err.Code[0] != '4' {
			t.Fatalf("sim error code %s must be in 4xxxx range", err.Code)
		}
		if seen[err.Code] {
			t.Fatalf("duplicate sim error code %s", err.Code)
		}
		seen[err.Code] = true
	}
}
