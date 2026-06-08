// Package upload 测试上传基础规则不携带任何业务模块语义。
package upload

import "testing"

// TestCheckSizeReturnsGenericResult 确认大小校验只返回通用原因,不绑定模块错误码。
func TestCheckSizeReturnsGenericResult(t *testing.T) {
	if got := CheckSize(0, 10); got != SizeEmpty {
		t.Fatalf("empty upload reason = %v", got)
	}
	if got := CheckSize(11, 10); got != SizeTooLarge {
		t.Fatalf("oversized upload reason = %v", got)
	}
	if got := CheckSize(10, 10); got != SizeOK {
		t.Fatalf("valid upload reason = %v", got)
	}
}

// TestCSVOrXLSXKindValidatesExtensionMIMEAndSignature 确认文件类型判断统一校验扩展名、MIME 与魔数。
func TestCSVOrXLSXKindValidatesExtensionMIMEAndSignature(t *testing.T) {
	if got := CSVOrXLSXKind("users.csv", "text/csv", []byte("a,b\n1,2")); got != KindCSV {
		t.Fatalf("csv kind = %v", got)
	}
	if got := CSVOrXLSXKind("users.xlsx", XLSXContentType, []byte{'P', 'K', 0x03, 0x04}); got != KindXLSX {
		t.Fatalf("xlsx kind = %v", got)
	}
	for _, tc := range []struct {
		name        string
		contentType string
		content     []byte
	}{
		{"users.csv", "text/csv", []byte{'P', 'K', 0x03, 0x04}},
		{"users.xlsx", "text/plain", []byte{'P', 'K', 0x03, 0x04}},
		{"users.exe", "application/octet-stream", []byte("x")},
	} {
		if got := CSVOrXLSXKind(tc.name, tc.contentType, tc.content); got != KindInvalid {
			t.Fatalf("mismatched upload kind = %v", got)
		}
	}
}

// TestSafeArchiveEntryNameRejectsTraversal 确认归档条目规则只表达通用路径安全。
func TestSafeArchiveEntryNameRejectsTraversal(t *testing.T) {
	existing := map[string]struct{}{"src/main.js": {}}
	for _, name := range []string{"../x", "/abs", "a/../../x", "C:/x", "", "src/main.js"} {
		if _, ok := SafeArchiveEntryName(name, existing); ok {
			t.Fatalf("unsafe archive entry should fail: %q", name)
		}
	}
	if got, ok := SafeArchiveEntryName(`src\app.js`, existing); !ok || got != "src/app.js" {
		t.Fatalf("safe archive entry should normalize, got=%q ok=%v", got, ok)
	}
}
