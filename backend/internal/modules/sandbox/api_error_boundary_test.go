// M2 API 边界测试:守护沙箱 HTTP 参数错误使用模块专属错误码。
package sandbox

import (
	"os"
	"strings"
	"testing"
)

// TestSandboxAPIErrorsUseDedicatedCodes 防止 M2 API 退回平台通用 ErrBadRequest。
func TestSandboxAPIErrorsUseDedicatedCodes(t *testing.T) {
	src, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read sandbox api: %v", err)
	}
	text := string(src)
	if strings.Contains(text, "ErrBadRequest") {
		t.Fatalf("M2 API must use dedicated sandbox error codes instead of ErrBadRequest")
	}
	for _, required := range []string{
		"ErrRuntimeCreateInvalid",
		"ErrRuntimeUpdateInvalid",
		"ErrRuntimeImageCreateInvalid",
		"ErrRuntimeImagePrepullInvalid",
		"ErrToolCreateInvalid",
		"ErrSandboxCreateRequestInvalid",
		"ErrSandboxOwnerInvalid",
		"ErrSandboxRecycleRequestInvalid",
		"ErrSandboxChainDeployInvalid",
		"ErrSandboxChainTxInvalid",
		"ErrSandboxFileWriteInvalid",
		"ErrQuotaUpdateInvalid",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("M2 API missing dedicated error %s", required)
		}
	}
}

// TestFileWriteEncodingRejectsUnknownValue 确认文件写入只接受 utf-8 或 base64 两种外部协议编码。
func TestFileWriteEncodingRejectsUnknownValue(t *testing.T) {
	if _, err := normalizeFileWriteContent(FileWriteRequest{Content: "YQ==", Encoding: "bad"}); err == nil {
		t.Fatalf("unknown file encoding must be rejected")
	}
}

// TestProgressWSAuthorizesBeforeUpgrade 确认 progress WS 在升级连接前完成沙箱归属校验。
func TestProgressWSAuthorizesBeforeUpgrade(t *testing.T) {
	src, err := os.ReadFile("websocket.go")
	if err != nil {
		t.Fatalf("read websocket.go: %v", err)
	}
	text := string(src)
	progressIdx := strings.Index(text, "progress, err := s.GetSandboxProgress")
	serveIdx := strings.Index(text, "s.hub.Serve")
	if progressIdx < 0 || serveIdx < 0 {
		t.Fatalf("ServeProgressWS structure not found")
	}
	if progressIdx > serveIdx {
		t.Fatalf("ServeProgressWS must authorize and load current progress before WebSocket upgrade")
	}
}

// TestSandboxProductionCodeDoesNotUseGenericInternalError 防止 M2 业务错误退回平台通用 11500。
func TestSandboxProductionCodeDoesNotUseGenericInternalError(t *testing.T) {
	files := []string{"audit.go", "files.go", "websocket.go"}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if strings.Contains(string(src), "ErrInternal") {
			t.Fatalf("%s must use M2 dedicated errors instead of ErrInternal", file)
		}
	}
}
