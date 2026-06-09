// identity service_org_test 文件校验组织结构导入模板和 CSV/XLSX 解析规则。
package identity

import (
	"bytes"
	"testing"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/upload"

	"github.com/xuri/excelize/v2"
)

// TestOrgImportTemplateProducesXLSXWorkbook 验证组织导入模板默认可生成真实 Excel 工作簿。
func TestOrgImportTemplateProducesXLSXWorkbook(t *testing.T) {
	s := &Service{cfg: config.IdentityConfig{ImportMaxRows: 10}}

	tpl, err := s.OrgImportTemplate("xlsx")
	if err != nil {
		t.Fatalf("期望生成组织 Excel 模板: %v", err)
	}
	if tpl.FileName != "org_import_template.xlsx" {
		t.Fatalf("模板文件名不正确: %s", tpl.FileName)
	}
	if tpl.ContentType != upload.XLSXContentType {
		t.Fatalf("模板 MIME 不正确: %s", tpl.ContentType)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(tpl.Content))
	if err != nil {
		t.Fatalf("模板不是可打开的 xlsx: %v", err)
	}
	defer func() {
		if err := workbook.Close(); err != nil {
			t.Fatalf("关闭模板工作簿失败: %v", err)
		}
	}()
	rows, err := workbook.GetRows("org")
	if err != nil {
		t.Fatalf("读取模板工作表失败: %v", err)
	}
	if len(rows) < 4 || rows[0][0] != "kind" || rows[0][3] != "enrollment_year" {
		t.Fatalf("组织模板表头不符合规范: %#v", rows)
	}
}

// TestParseOrgImportFileAcceptsXLSX 验证组织导入预览支持按文档上传 XLSX。
func TestParseOrgImportFileAcceptsXLSX(t *testing.T) {
	s := &Service{cfg: config.IdentityConfig{ImportMaxRows: 10}}
	raw := buildOrgImportXLSX(t, [][]string{
		{"kind", "name", "code_or_parent_id", "enrollment_year"},
		{"department", "计算机学院", "CS", ""},
		{"major", "区块链工程", "1001", ""},
		{"class", "区块链 2024-1 班", "2001", "2024"},
		{"class", "", "bad", "abc"},
	})

	rows, results, err := s.parseOrgImportFile(raw, "org.xlsx", upload.XLSXContentType)
	if err != nil {
		t.Fatalf("期望 XLSX 可解析: %v", err)
	}
	if len(rows) != 4 || len(results) != 4 {
		t.Fatalf("期望解析四行, rows=%d results=%d", len(rows), len(results))
	}
	if rows[0].Kind != "department" || rows[1].ParentID != 1001 || rows[2].EnrollmentYear != 2024 {
		t.Fatalf("组织导入字段解析不正确: %#v", rows[:3])
	}
	if results[3].Error == "" {
		t.Fatalf("期望非法行返回用户向错误")
	}
}

// TestParseOrgImportFileRejectsMismatchedUploadKind 验证组织导入拒绝扩展名、MIME 与魔数不一致的文件。
func TestParseOrgImportFileRejectsMismatchedUploadKind(t *testing.T) {
	s := &Service{cfg: config.IdentityConfig{ImportMaxRows: 10}}
	raw := []byte("kind,name,code_or_parent_id,enrollment_year\ndepartment,计算机学院,CS,\n")

	if _, _, err := s.parseOrgImportFile(raw, "org.xlsx", upload.XLSXContentType); err == nil {
		t.Fatalf("期望拒绝伪装成 xlsx 的 CSV 内容")
	}
}

// buildOrgImportXLSX 构造测试用组织导入工作簿。
func buildOrgImportXLSX(t *testing.T, data [][]string) []byte {
	t.Helper()
	workbook := excelize.NewFile()
	defer func() {
		if err := workbook.Close(); err != nil {
			t.Fatalf("关闭测试工作簿失败: %v", err)
		}
	}()
	if err := workbook.SetSheetName("Sheet1", "org"); err != nil {
		t.Fatalf("设置测试工作表失败: %v", err)
	}
	for rowIndex, row := range data {
		for colIndex, value := range row {
			cell, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
			if err != nil {
				t.Fatalf("生成测试单元格坐标失败: %v", err)
			}
			if err := workbook.SetCellValue("org", cell, value); err != nil {
				t.Fatalf("写入测试单元格失败: %v", err)
			}
		}
	}
	var buf bytes.Buffer
	if err := workbook.Write(&buf); err != nil {
		t.Fatalf("写入测试 xlsx 失败: %v", err)
	}
	return buf.Bytes()
}
