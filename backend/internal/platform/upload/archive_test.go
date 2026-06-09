// upload 测试归档校验共享安全边界,防止沙箱和判题各自分叉实现。
package upload

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"testing"
)

// TestZIPEntryNamesRejectsTraversalAndDuplicates 确认 ZIP 成员会拒绝路径逃逸和重复覆盖。
func TestZIPEntryNamesRejectsTraversalAndDuplicates(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	zw := zip.NewWriter(buf)
	for _, name := range []string{"../bad.txt", "dup.txt", "dup.txt"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := w.Write([]byte("a")); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip reader: %v", err)
	}
	if _, err := ZIPEntryNames(reader, ArchiveLimits{MaxFiles: 10, MaxUnpackedBytes: 1024}); err == nil {
		t.Fatalf("unsafe zip archive should fail")
	}
}

// TestZIPEntryNamesAcceptsSafeEntries 确认安全 ZIP 能返回规范化成员名。
func TestZIPEntryNamesAcceptsSafeEntries(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	zw := zip.NewWriter(buf)
	w, err := zw.Create("dir\\a.txt")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := w.Write([]byte("abc")); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip reader: %v", err)
	}
	names, err := ZIPEntryNames(reader, ArchiveLimits{MaxFiles: 10, MaxUnpackedBytes: 1024})
	if err != nil {
		t.Fatalf("safe zip should pass: %v", err)
	}
	if len(names) != 1 || names[0] != "dir/a.txt" {
		t.Fatalf("unexpected safe names: %#v", names)
	}
}

// TestTAREntryNamesRejectsUnsupportedType 确认 TAR 中的符号链接等特殊成员会被统一拒绝。
func TestTAREntryNamesRejectsUnsupportedType(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(buf)
	if err := tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if _, err := TAREntryNames(tar.NewReader(bytes.NewReader(buf.Bytes())), ArchiveLimits{MaxFiles: 10, MaxUnpackedBytes: 1024}); err == nil {
		t.Fatalf("tar symlink should fail")
	}
}

// TestTAREntryNamesRejectsTooManyFiles 确认归档成员数量上限在平台层统一生效。
func TestTAREntryNamesRejectsTooManyFiles(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(buf)
	for _, name := range []string{"a.txt", "b.txt"} {
		content := []byte("x")
		if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Size: int64(len(content))}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if _, err := TAREntryNames(tar.NewReader(bytes.NewReader(buf.Bytes())), ArchiveLimits{MaxFiles: 1, MaxUnpackedBytes: 1024}); err == nil {
		t.Fatalf("tar with too many files should fail")
	}
}
