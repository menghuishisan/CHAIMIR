// 导入模板生成:为账号批量导入提供 CSV 与 Excel 模板。
package identity

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"

	"github.com/xuri/excelize/v2"
)

// ImportTemplate 是导入模板下载结果。
type ImportTemplate struct {
	FileName    string
	ContentType string
	Content     []byte
}

var importTemplateHeaders = []string{"phone", "name", "no", "org_id", "enrollment_year", "title"}

// BuildImportTemplate 生成账号导入模板;format 支持 csv/xlsx。
func BuildImportTemplate(targetType int16, format string) (*ImportTemplate, error) {
	if targetType != ImportTargetTeacher && targetType != ImportTargetStudent {
		return nil, apperr.ErrImportTargetInvalid
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "csv":
		return buildCSVImportTemplate(targetType)
	case "xlsx", "excel":
		return buildXLSXImportTemplate(targetType)
	default:
		return nil, apperr.ErrImportFormat
	}
}

// buildCSVImportTemplate 使用标准库生成 CSV 模板。
func buildCSVImportTemplate(targetType int16) (*ImportTemplate, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write(importTemplateHeaders); err != nil {
		return nil, apperr.ErrImportTemplateBuildFailed.WithCause(err)
	}
	if err := writer.Write(importTemplateSample(targetType)); err != nil {
		return nil, apperr.ErrImportTemplateBuildFailed.WithCause(err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, apperr.ErrImportTemplateBuildFailed.WithCause(err)
	}
	return &ImportTemplate{
		FileName:    importTemplateFileName(targetType, "csv"),
		ContentType: "text/csv; charset=utf-8",
		Content:     buf.Bytes(),
	}, nil
}

// buildXLSXImportTemplate 使用 excelize 生成 Excel 模板。
func buildXLSXImportTemplate(targetType int16) (tpl *ImportTemplate, err error) {
	file := excelize.NewFile()
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, apperr.ErrImportTemplateBuildFailed.WithCause(closeErr))
		}
	}()
	sheet := file.GetSheetName(0)
	for i, header := range importTemplateHeaders {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, apperr.ErrImportTemplateBuildFailed.WithCause(err)
		}
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			return nil, apperr.ErrImportTemplateBuildFailed.WithCause(err)
		}
	}
	for i, value := range importTemplateSample(targetType) {
		cell, err := excelize.CoordinatesToCellName(i+1, 2)
		if err != nil {
			return nil, apperr.ErrImportTemplateBuildFailed.WithCause(err)
		}
		if err := file.SetCellValue(sheet, cell, value); err != nil {
			return nil, apperr.ErrImportTemplateBuildFailed.WithCause(err)
		}
	}
	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		return nil, apperr.ErrImportTemplateBuildFailed.WithCause(err)
	}
	return &ImportTemplate{
		FileName:    importTemplateFileName(targetType, "xlsx"),
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		Content:     buf.Bytes(),
	}, nil
}

// importTemplateSample 返回一行示例数据,帮助管理员按目标身份填写。
func importTemplateSample(targetType int16) []string {
	if targetType == ImportTargetTeacher {
		return []string{"13800138000", "教师姓名", "T2026001", "院系ID", "", "讲师"}
	}
	return []string{"13800138000", "学生姓名", "S2026001", "班级ID", "2026", ""}
}

// importTemplateFileName 生成下载文件名。
func importTemplateFileName(targetType int16, ext string) string {
	kind := contracts.RoleStudent
	if targetType == ImportTargetTeacher {
		kind = contracts.RoleTeacher
	}
	return fmt.Sprintf("chaimir-%s-import-template.%s", kind, ext)
}
