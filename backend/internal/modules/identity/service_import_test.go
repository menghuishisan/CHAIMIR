// identity service_import_test 文件校验账号导入模板和 CSV/XLSX 解析规则。
package identity

import (
	"bytes"
	"testing"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/upload"

	"github.com/xuri/excelize/v2"
)

// TestImportTemplateProducesXLSXWorkbook 验证模板下载默认可生成真实 Excel 工作簿。
func TestImportTemplateProducesXLSXWorkbook(t *testing.T) {
	s := &Service{cfg: config.IdentityConfig{ImportMaxRows: 10}}

	tpl, err := s.ImportTemplate(ImportTargetStudent, "xlsx")
	if err != nil {
		t.Fatalf("期望生成学生 Excel 模板: %v", err)
	}
	if tpl.FileName != "student_import_template.xlsx" {
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
	rows, err := workbook.GetRows("accounts")
	if err != nil {
		t.Fatalf("读取模板工作表失败: %v", err)
	}
	if len(rows) < 2 || rows[0][0] != "phone" || rows[0][4] != "enrollment_year" {
		t.Fatalf("学生模板表头不符合规范: %#v", rows)
	}
	if rows[0][5] != "initial_password" {
		t.Fatalf("学生模板必须包含初始密码列, got %#v", rows[0])
	}
}

// TestParseImportFileAcceptsXLSX 验证账号导入预览支持按文档上传 XLSX。
func TestParseImportFileAcceptsXLSX(t *testing.T) {
	s := &Service{cfg: config.IdentityConfig{ImportMaxRows: 10}}
	raw := buildAccountImportXLSX(t, [][]string{
		{"phone", "name", "no", "org_id", "enrollment_year", "initial_password"},
		{"13800138000", "张三", "2024001", "123", "2024", "Initpass1"},
		{"123", "李四", "2024002", "abc", "2024", ""},
	})

	rows, results, err := s.parseImportFile(raw, "students.xlsx", upload.XLSXContentType, ImportTargetStudent)
	if err != nil {
		t.Fatalf("期望 XLSX 可解析: %v", err)
	}
	if len(rows) != 2 || len(results) != 2 {
		t.Fatalf("期望解析两行, rows=%d results=%d", len(rows), len(results))
	}
	if rows[0].Error != "" {
		t.Fatalf("期望第一行合法, got %q", rows[0].Error)
	}
	if rows[0].EnrollmentYear != 2024 {
		t.Fatalf("期望读取学生入学年份, got %d", rows[0].EnrollmentYear)
	}
	if rows[0].InitialPassword != "Initpass1" {
		t.Fatalf("期望读取初始密码列")
	}
	if rows[1].Error == "" {
		t.Fatalf("期望第二行报告错误")
	}
}

// TestParseImportFileRejectsMismatchedUploadKind 验证扩展名、MIME 与魔数不一致时拒绝导入。
func TestParseImportFileRejectsMismatchedUploadKind(t *testing.T) {
	s := &Service{cfg: config.IdentityConfig{ImportMaxRows: 10}}
	raw := []byte("phone,name,no,org_id\n13800138000,张三,2024001,123\n")

	if _, _, err := s.parseImportFile(raw, "students.xlsx", upload.XLSXContentType, ImportTargetStudent); err == nil {
		t.Fatalf("期望拒绝伪装成 xlsx 的 CSV 内容")
	}
}

// TestParseImportCSVValidatesRows 验证 CSV 行级校验会保留合法行并返回非法行错误。
func TestParseImportCSVValidatesRows(t *testing.T) {
	s := &Service{cfg: config.IdentityConfig{ImportMaxRows: 10}}
	raw := []byte("phone,name,no,org_id\n13800138000,张三,2024001,123\n123,李四,2024002,abc\n")
	rows, results, err := s.parseImportFile(raw, "students.csv", "text/csv", ImportTargetStudent)
	if err != nil {
		t.Fatalf("期望 CSV 可解析: %v", err)
	}
	if len(rows) != 2 || len(results) != 2 {
		t.Fatalf("期望解析两行, rows=%d results=%d", len(rows), len(results))
	}
	if rows[0].Error != "" {
		t.Fatalf("期望第一行合法, got %q", rows[0].Error)
	}
	if rows[1].Error == "" {
		t.Fatalf("期望第二行报告错误")
	}
}

// buildAccountImportXLSX 构造测试用账号导入工作簿。
func buildAccountImportXLSX(t *testing.T, data [][]string) []byte {
	t.Helper()
	workbook := excelize.NewFile()
	defer func() {
		if err := workbook.Close(); err != nil {
			t.Fatalf("关闭测试工作簿失败: %v", err)
		}
	}()
	if err := workbook.SetSheetName("Sheet1", "accounts"); err != nil {
		t.Fatalf("设置测试工作表失败: %v", err)
	}
	for rowIndex, row := range data {
		for colIndex, value := range row {
			cell, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
			if err != nil {
				t.Fatalf("生成测试单元格坐标失败: %v", err)
			}
			if err := workbook.SetCellValue("accounts", cell, value); err != nil {
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
