// M1 账号导入文件解析测试。
package identity

import (
	"bytes"
	"testing"

	"chaimir/pkg/apperr"

	"github.com/xuri/excelize/v2"
)

// TestParseImportFileCSVReadsRows 确认 CSV 上传文件按模板表头解析为导入行。
func TestParseImportFileCSVReadsRows(t *testing.T) {
	content := []byte("phone,name,no,org_id,enrollment_year,title\n13800138000,张三,S001,3001,2026,\n")

	rows, err := ParseImportFile("students.csv", content)
	if err != nil {
		t.Fatalf("parse csv import file: %v", err)
	}
	if len(rows) != 1 || rows[0].Phone != "13800138000" || rows[0].EnrollmentYear != 2026 {
		t.Fatalf("unexpected csv rows: %#v", rows)
	}
}

// TestParseImportFileXLSXReadsRows 确认 Excel 上传文件按模板表头解析为导入行。
func TestParseImportFileXLSXReadsRows(t *testing.T) {
	file := excelize.NewFile()
	defer file.Close()
	sheet := file.GetSheetName(0)
	values := []string{"phone", "name", "no", "org_id", "enrollment_year", "title"}
	for i, v := range values {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			t.Fatalf("cell name: %v", err)
		}
		if err := file.SetCellValue(sheet, cell, v); err != nil {
			t.Fatalf("set header: %v", err)
		}
	}
	row := []string{"13800138001", "李四", "T001", "2001", "", "讲师"}
	for i, v := range row {
		cell, err := excelize.CoordinatesToCellName(i+1, 2)
		if err != nil {
			t.Fatalf("cell name: %v", err)
		}
		if err := file.SetCellValue(sheet, cell, v); err != nil {
			t.Fatalf("set row: %v", err)
		}
	}
	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}

	rows, err := ParseImportFile("teachers.xlsx", buf.Bytes())
	if err != nil {
		t.Fatalf("parse xlsx import file: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "李四" || rows[0].Title != "讲师" {
		t.Fatalf("unexpected xlsx rows: %#v", rows)
	}
}

// TestParseImportFileRejectsWrongHeader 确认非模板表头不会被宽松解析。
func TestParseImportFileRejectsWrongHeader(t *testing.T) {
	_, err := ParseImportFile("bad.csv", []byte("phone,name\n13800138000,张三\n"))
	if err != apperr.ErrImportFormat {
		t.Fatalf("expected import format error, got %v", err)
	}
}
