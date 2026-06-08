// 导入模板生成测试。
package identity

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// TestBuildImportTemplateCSV 保障 CSV 模板包含导入字段。
func TestBuildImportTemplateCSV(t *testing.T) {
	tpl, err := BuildImportTemplate(ImportTargetStudent, "csv")
	if err != nil {
		t.Fatalf("build csv template: %v", err)
	}
	if tpl.ContentType != "text/csv; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", tpl.ContentType)
	}
	if !strings.Contains(string(tpl.Content), "phone,name,no,org_id,enrollment_year,title") {
		t.Fatalf("missing csv header: %s", string(tpl.Content))
	}
}

// TestBuildImportTemplateXLSX 保障 Excel 模板可由成熟库读取。
func TestBuildImportTemplateXLSX(t *testing.T) {
	tpl, err := BuildImportTemplate(ImportTargetTeacher, "xlsx")
	if err != nil {
		t.Fatalf("build xlsx template: %v", err)
	}
	if tpl.ContentType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("unexpected content type: %s", tpl.ContentType)
	}
	file, err := excelize.OpenReader(bytes.NewReader(tpl.Content))
	if err != nil {
		t.Fatalf("open xlsx template: %v", err)
	}
	defer file.Close()
	value, err := file.GetCellValue("Sheet1", "A1")
	if err != nil {
		t.Fatalf("read xlsx cell: %v", err)
	}
	if value != "phone" {
		t.Fatalf("expected A1 phone, got %q", value)
	}
}
