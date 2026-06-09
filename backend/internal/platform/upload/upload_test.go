// Package upload 测试上传基础规则不携带任何业务模块语义。
package upload

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"testing"
)

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

// TestReadTarGzFilesAppliesCommonArchiveLimits 确认 tar.gz 安全读取统一限制文件数和展开字节数。
func TestReadTarGzFilesAppliesCommonArchiveLimits(t *testing.T) {
	raw := testTarGz(t, map[string]string{
		"src/a.js": "const a = 1;",
		"src/b.js": "const b = 2;",
	})
	if _, err := ReadTarGzFiles(raw, ArchiveLimits{MaxFiles: 1, MaxUnpackedBytes: 1024}); !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("too many files must return ErrArchiveTooLarge, got %v", err)
	}
	if _, err := ReadTarGzFiles(raw, ArchiveLimits{MaxFiles: 10, MaxUnpackedBytes: 4}); !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("too many bytes must return ErrArchiveTooLarge, got %v", err)
	}
	files, err := ReadTarGzFiles(raw, ArchiveLimits{MaxFiles: 10, MaxUnpackedBytes: 1024})
	if err != nil {
		t.Fatalf("safe tar.gz rejected: %v", err)
	}
	if string(files["src/a.js"]) != "const a = 1;" {
		t.Fatalf("unexpected file map: %#v", files)
	}
}

// TestReadZipFilesRejectsDuplicateAndTraversal 确认 zip 安全读取复用统一路径规则。
func TestReadZipFilesRejectsDuplicateAndTraversal(t *testing.T) {
	if _, err := ReadZipFiles(testZip(t, []archiveFile{
		{name: "src/a.js", body: "safe"},
		{name: "src/a.js", body: "overwrite"},
	}), ArchiveLimits{MaxFiles: 10, MaxUnpackedBytes: 1024}); !errors.Is(err, ErrArchiveInvalid) {
		t.Fatalf("duplicate zip entry must be invalid, got %v", err)
	}
	if _, err := ReadZipFiles(testZip(t, []archiveFile{{name: "../escape", body: "x"}}), ArchiveLimits{MaxFiles: 10, MaxUnpackedBytes: 1024}); !errors.Is(err, ErrArchiveInvalid) {
		t.Fatalf("traversal zip entry must be invalid, got %v", err)
	}
}

// TestRewriteTarGzNormalizesEntries 确认进入容器前的归档会被统一重打包为安全普通文件。
func TestRewriteTarGzNormalizesEntries(t *testing.T) {
	rewritten, err := RewriteTarGz(testTarGz(t, map[string]string{"src/main.sol": "contract C {}"}), ArchiveLimits{MaxFiles: 10, MaxUnpackedBytes: 1024})
	if err != nil {
		t.Fatalf("rewrite safe archive: %v", err)
	}
	files, err := ReadTarGzFiles(rewritten, ArchiveLimits{MaxFiles: 10, MaxUnpackedBytes: 1024})
	if err != nil {
		t.Fatalf("read rewritten archive: %v", err)
	}
	if string(files["src/main.sol"]) != "contract C {}" {
		t.Fatalf("unexpected rewritten archive files: %#v", files)
	}
}

type archiveFile struct {
	name string
	body string
}

func testTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		data := []byte(body)
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return raw.Bytes()
}

func testZip(t *testing.T, files []archiveFile) []byte {
	t.Helper()
	var raw bytes.Buffer
	zw := zip.NewWriter(&raw)
	for i, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			t.Fatalf("create zip entry %d: %v", i, err)
		}
		if _, err := fmt.Fprint(w, f.body); err != nil {
			t.Fatalf("write zip entry %d: %v", i, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return raw.Bytes()
}
