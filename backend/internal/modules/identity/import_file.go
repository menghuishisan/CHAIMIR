// M1 账号导入文件解析:统一支持 CSV 与 Excel 模板。
package identity

import (
	"bytes"
	"encoding/csv"
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"

	"github.com/xuri/excelize/v2"
)

// ParseImportFile 解析账号导入上传文件,只接受模板表头顺序。
func ParseImportFile(fileName string, content []byte) ([]ImportRowInput, error) {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".csv":
		return parseCSVImportFile(content)
	case ".xlsx":
		return parseXLSXImportFile(content)
	default:
		return nil, apperr.ErrImportFormat
	}
}

// ensureImportUploadSize 校验导入文件大小,边界来自启动配置而不是模块常量。
func ensureImportUploadSize(size, maxBytes int64) error {
	switch upload.CheckSize(size, maxBytes) {
	case upload.SizeOK:
		return nil
	case upload.SizeEmpty:
		return apperr.ErrImportEmpty
	case upload.SizeTooLarge:
		return apperr.ErrImportTooLarge
	default:
		return apperr.ErrImportFormat
	}
}

// ensureImportUploadType 校验导入文件扩展名、声明类型与内容签名一致。
func ensureImportUploadType(fileName, contentType string, content []byte) error {
	switch upload.CSVOrXLSXKind(fileName, contentType, content) {
	case upload.KindCSV, upload.KindXLSX:
		return nil
	default:
		return apperr.ErrImportFormat
	}
}

// parseCSVImportFile 使用标准库读取 CSV,避免手写分隔符解析造成转义问题。
func parseCSVImportFile(content []byte) ([]ImportRowInput, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, apperr.ErrImportFormat.WithCause(err)
	}
	return importRowsFromCells(records)
}

// parseXLSXImportFile 使用 excelize 读取首个工作表数据。
func parseXLSXImportFile(content []byte) (rows []ImportRowInput, err error) {
	file, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, apperr.ErrImportFormat.WithCause(err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, apperr.ErrImportFormat.WithCause(closeErr))
		}
	}()
	sheet := file.GetSheetName(0)
	rawRows, err := file.GetRows(sheet)
	if err != nil {
		return nil, apperr.ErrImportFormat.WithCause(err)
	}
	return importRowsFromCells(rawRows)
}

// importRowsFromCells 校验模板表头并把二维单元格数据转换为导入行。
func importRowsFromCells(records [][]string) ([]ImportRowInput, error) {
	if len(records) < 2 || !importHeaderMatches(records[0]) {
		return nil, apperr.ErrImportFormat
	}
	rows := make([]ImportRowInput, 0, len(records)-1)
	for _, record := range records[1:] {
		normalized := normalizeImportRecord(record)
		if importRecordEmpty(normalized) {
			continue
		}
		year, err := parseImportYear(normalized[4])
		if err != nil {
			return nil, apperr.ErrImportFormat.WithCause(err)
		}
		rows = append(rows, ImportRowInput{
			Phone:          normalized[0],
			Name:           normalized[1],
			No:             normalized[2],
			OrgID:          normalized[3],
			EnrollmentYear: year,
			Title:          normalized[5],
		})
	}
	if len(rows) == 0 {
		return nil, apperr.ErrImportEmpty
	}
	return rows, nil
}

// importHeaderMatches 检查表头与模板完全一致,防止列错位导致错误导入。
func importHeaderMatches(header []string) bool {
	normalized := normalizeImportRecord(header)
	for i, want := range importTemplateHeaders {
		if normalized[i] != want {
			return false
		}
	}
	return true
}

// normalizeImportRecord 把行补齐到模板列数并去掉单元格首尾空白。
func normalizeImportRecord(record []string) []string {
	out := make([]string, len(importTemplateHeaders))
	for i := range out {
		if i < len(record) {
			out[i] = strings.TrimSpace(record[i])
		}
	}
	return out
}

// importRecordEmpty 判断整行是否为空,允许 Excel 尾部空行存在。
func importRecordEmpty(record []string) bool {
	for _, v := range record {
		if v != "" {
			return false
		}
	}
	return true
}

// parseImportYear 解析学生入学年份;教师行可为空。
func parseImportYear(v string) (int16, error) {
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, err
	}
	return int16(n), nil
}
