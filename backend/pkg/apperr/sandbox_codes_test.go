// M2 错误码测试:确保沙箱错误码覆盖已实现的运行时、沙箱、工具与配额细分场景。
package apperr

import "testing"

// TestSandboxErrorCodesAreUniqueAndSegmented 确认 M2 错误码不重复且落在 2xxxx 段。
func TestSandboxErrorCodesAreUniqueAndSegmented(t *testing.T) {
	codes := []*Error{
		ErrRuntimeNotFound,
		ErrRuntimeUnavailable,
		ErrRuntimeInvalid,
		ErrRuntimeImageNotFound,
		ErrRuntimeSelftestFailed,
		ErrRuntimePrepullFailed,
		ErrRuntimeCapabilityUnavailable,
		ErrRuntimePersistenceFail,
		ErrRuntimeSelftestConfigInvalid,
		ErrSandboxNotFound,
		ErrSandboxCreateFail,
		ErrSandboxRecycleFail,
		ErrSandboxInvalidState,
		ErrSandboxTimeout,
		ErrSandboxFileInvalid,
		ErrSandboxFileNotFound,
		ErrSandboxFileSaveFail,
		ErrSandboxInitFail,
		ErrSandboxChainOperationFail,
		ErrRuntimeCreateInvalid,
		ErrRuntimeUpdateInvalid,
		ErrRuntimeImageCreateInvalid,
		ErrRuntimeImagePrepullInvalid,
		ErrToolCreateInvalid,
		ErrSandboxCreateRequestInvalid,
		ErrSandboxOwnerInvalid,
		ErrSandboxRecycleRequestInvalid,
		ErrSandboxChainDeployInvalid,
		ErrSandboxChainTxInvalid,
		ErrSandboxFileWriteInvalid,
		ErrSandboxAccessDenied,
		ErrSandboxPersistenceFail,
		ErrSandboxAuditFail,
		ErrSandboxSnapshotUnavailable,
		ErrSandboxSchedulerConfigInvalid,
		ErrSandboxRecycleScanFail,
		ErrSandboxRecycleFinalizeFail,
		ErrSandboxSnapshotCleanupFail,
		ErrToolNotFound,
		ErrToolNotFitRuntime,
		ErrToolProxyFail,
		ErrToolPersistenceFail,
		ErrQuotaExceeded,
		ErrQuotaInvalid,
		ErrQuotaUpdateInvalid,
		ErrQuotaResourceBusy,
		ErrQuotaPersistenceFail,
	}
	seen := map[string]bool{}
	for _, item := range codes {
		if item.Code == "" || item.Code[0] != '2' {
			t.Fatalf("sandbox code must be in 2xxxx segment, got %q", item.Code)
		}
		if seen[item.Code] {
			t.Fatalf("duplicate sandbox code %s", item.Code)
		}
		seen[item.Code] = true
	}
}
