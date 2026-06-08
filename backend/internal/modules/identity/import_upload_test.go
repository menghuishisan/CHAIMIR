// M1 导入上传边界测试。
package identity

import (
	"testing"

	"chaimir/pkg/apperr"
)

// TestEnsureImportUploadSizeRejectsOversizedFile 确认上传文件大小有服务端边界。
func TestEnsureImportUploadSizeRejectsOversizedFile(t *testing.T) {
	if err := ensureImportUploadSize(1025, 1024); err != apperr.ErrImportTooLarge {
		t.Fatalf("expected import too large error, got %v", err)
	}
}

// TestEnsureImportUploadTypeRejectsMismatchedContent 确认上传导入文件不只依赖扩展名。
func TestEnsureImportUploadTypeRejectsMismatchedContent(t *testing.T) {
	if err := ensureImportUploadType("students.csv", "text/csv", []byte("PK\x03\x04fake")); err != apperr.ErrImportFormat {
		t.Fatalf("expected csv content mismatch to be rejected, got %v", err)
	}
	if err := ensureImportUploadType("students.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", []byte("phone,name\n")); err != apperr.ErrImportFormat {
		t.Fatalf("expected xlsx content mismatch to be rejected, got %v", err)
	}
}

// TestEnsureImportUploadTypeRejectsUnexpectedMIME 确认上传导入文件校验客户端声明类型。
func TestEnsureImportUploadTypeRejectsUnexpectedMIME(t *testing.T) {
	if err := ensureImportUploadType("students.csv", "application/octet-stream", []byte("phone,name,no,org_id,enrollment_year,title\n")); err != apperr.ErrImportFormat {
		t.Fatalf("expected unexpected csv mime to be rejected, got %v", err)
	}
}
