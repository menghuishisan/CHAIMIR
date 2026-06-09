// upload 测试跨模块统一上传与归档安全边界。
package upload

import "testing"

// TestCheckSize 确认空文件、超限和正常大小走统一结果码。
func TestCheckSize(t *testing.T) {
	if got := CheckSize(0, 10); got != SizeEmpty {
		t.Fatalf("CheckSize(empty) = %v, want SizeEmpty", got)
	}
	if got := CheckSize(11, 10); got != SizeTooLarge {
		t.Fatalf("CheckSize(too large) = %v, want SizeTooLarge", got)
	}
	if got := CheckSize(10, 10); got != SizeOK {
		t.Fatalf("CheckSize(ok) = %v, want SizeOK", got)
	}
}

// TestCSVOrXLSXKind 确认 CSV/XLSX 类型校验不依赖业务模块重复实现。
func TestCSVOrXLSXKind(t *testing.T) {
	if got := CSVOrXLSXKind("a.csv", "text/csv", []byte("id,name\n1,a")); got != KindCSV {
		t.Fatalf("CSV kind = %v, want KindCSV", got)
	}
	if got := CSVOrXLSXKind("a.xlsx", XLSXContentType, []byte{'P', 'K', 0x03, 0x04}); got != KindXLSX {
		t.Fatalf("XLSX kind = %v, want KindXLSX", got)
	}
	if got := CSVOrXLSXKind("a.csv", "text/csv", []byte{'P', 'K', 0x03, 0x04}); got != KindInvalid {
		t.Fatalf("zip masquerading as csv should be invalid, got %v", got)
	}
}

// TestSafeArchiveEntryName 确认归档成员路径会拒绝逃逸和重复覆盖。
func TestSafeArchiveEntryName(t *testing.T) {
	existing := map[string]struct{}{"dup.txt": {}}
	if got, ok := SafeArchiveEntryName("../a.txt", nil); ok || got != "" {
		t.Fatalf("parent traversal should fail, got=%q ok=%v", got, ok)
	}
	if got, ok := SafeArchiveEntryName("C:\\a.txt", nil); ok || got != "" {
		t.Fatalf("windows drive path should fail, got=%q ok=%v", got, ok)
	}
	if got, ok := SafeArchiveEntryName("dup.txt", existing); ok || got != "" {
		t.Fatalf("duplicate archive entry should fail, got=%q ok=%v", got, ok)
	}
	if got, ok := SafeArchiveEntryName("dir\\a.txt", nil); !ok || got != "dir/a.txt" {
		t.Fatalf("safe archive entry = (%q,%v), want (dir/a.txt,true)", got, ok)
	}
}
