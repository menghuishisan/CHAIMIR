// M1 账号导入预览持久化测试。
package identity

import (
	"testing"

	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
)

// TestImportPreviewCommitRequestRequiresPreviewID 确认提交导入只引用服务端预览结果。
func TestImportPreviewCommitRequestRequiresPreviewID(t *testing.T) {
	if _, ok := ids.Parse(ImportCommitRequest{}.PreviewID); ok {
		t.Fatalf("empty preview id must not be accepted")
	}
}

// TestEnsureImportPreviewRowsLimitRejectsTooLarge 确认文件解析后仍统一执行单次导入上限。
func TestEnsureImportPreviewRowsLimitRejectsTooLarge(t *testing.T) {
	rows := make([]ImportRowInput, 3)
	if err := ensureImportRowsLimit(rows, 2); err != apperr.ErrImportTooLarge {
		t.Fatalf("expected too large error, got %v", err)
	}
}

// TestImportPreviewRowsRoundTrip 确认导入预览行持久化后可无损读回。
func TestImportPreviewRowsRoundTrip(t *testing.T) {
	rows := []ImportRowInput{{
		Phone: "13800138000", Name: "张三", No: "S001", OrgID: "3001", EnrollmentYear: 2026,
	}}

	encoded, err := marshalImportRows(rows)
	if err != nil {
		t.Fatalf("marshal import rows: %v", err)
	}
	decoded, err := unmarshalImportRows(encoded)
	if err != nil {
		t.Fatalf("unmarshal import rows: %v", err)
	}
	if len(decoded) != 1 || decoded[0].No != "S001" || decoded[0].EnrollmentYear != 2026 {
		t.Fatalf("unexpected decoded rows: %#v", decoded)
	}
}

// TestImportPreviewResultRoundTrip 确认预览结果持久化后保留逐行错误。
func TestImportPreviewResultRoundTrip(t *testing.T) {
	result := &ImportPreviewResult{Total: 1, Invalid: 1, Rows: []ImportPreviewRow{{Line: 1, Error: "手机号格式不正确"}}}

	encoded, err := marshalImportPreviewResult(result)
	if err != nil {
		t.Fatalf("marshal import preview result: %v", err)
	}
	decoded, err := unmarshalImportPreviewResult(encoded)
	if err != nil {
		t.Fatalf("unmarshal import preview result: %v", err)
	}
	if decoded.Invalid != 1 || decoded.Rows[0].Error == "" {
		t.Fatalf("unexpected decoded result: %#v", decoded)
	}
}
