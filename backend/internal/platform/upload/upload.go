// upload 提供跨模块上传安全边界,只表达通用校验结果,不持有业务错误码或用户文案。
package upload

import (
	"io"
	"path"
	"path/filepath"
	"strings"
)

// SizeResult 是上传大小校验的通用结果,由调用模块映射为自己的错误码。
type SizeResult int

const (
	// SizeOK 表示上传大小在允许范围内。
	SizeOK SizeResult = iota
	// SizeEmpty 表示上传内容为空。
	SizeEmpty
	// SizeTooLarge 表示上传内容超过配置上限。
	SizeTooLarge
)

// FileKind 是基础层可识别的通用上传文件类型。
type FileKind int

const (
	// KindInvalid 表示扩展名、MIME 或魔数不匹配。
	KindInvalid FileKind = iota
	// KindCSV 表示普通 CSV 文本。
	KindCSV
	// KindXLSX 表示 Office OpenXML 工作簿。
	KindXLSX
)

// XLSXContentType 是 Office OpenXML 工作簿的标准 MIME。
const XLSXContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// CheckSize 校验上传大小,只返回通用原因,不绑定任何业务模块错误码。
func CheckSize(size, maxBytes int64) SizeResult {
	if size <= 0 {
		return SizeEmpty
	}
	if maxBytes > 0 && size > maxBytes {
		return SizeTooLarge
	}
	return SizeOK
}

// ReadBounded 最多读取 maxBytes+1 字节,让调用方能判定超限而不会无界占用内存。
func ReadBounded(r io.Reader, maxBytes int64) ([]byte, SizeResult, error) {
	if r == nil {
		return nil, SizeEmpty, nil
	}
	limit := maxBytes
	if limit > 0 {
		limit++
	}
	data, err := io.ReadAll(io.LimitReader(r, limit))
	if err != nil {
		return nil, SizeOK, err
	}
	return data, CheckSize(int64(len(data)), maxBytes), nil
}

// CSVOrXLSXKind 校验 CSV/XLSX 的扩展名、声明 MIME 与内容签名是否一致。
func CSVOrXLSXKind(fileName, contentType string, content []byte) FileKind {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".csv":
		if !AllowedCSVContentType(contentType) || LooksLikeZip(content) {
			return KindInvalid
		}
		return KindCSV
	case ".xlsx":
		if baseContentType(contentType) != XLSXContentType || !LooksLikeZip(content) {
			return KindInvalid
		}
		return KindXLSX
	default:
		return KindInvalid
	}
}

// AllowedCSVContentType 判断 CSV 上传允许的常见 MIME 类型。
func AllowedCSVContentType(contentType string) bool {
	switch baseContentType(contentType) {
	case "text/csv", "application/csv", "application/vnd.ms-excel":
		return true
	default:
		return false
	}
}

// LooksLikeZip 通过 ZIP 文件头识别 XLSX/ZIP 容器。
func LooksLikeZip(content []byte) bool {
	return len(content) >= 4 && content[0] == 'P' && content[1] == 'K' && content[2] == 0x03 && content[3] == 0x04
}

// SafeArchiveEntryName 标准化归档条目路径,拒绝目录穿越、绝对路径、Windows 盘符和同名覆盖。
func SafeArchiveEntryName(name string, existing map[string]struct{}) (string, bool) {
	raw := strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	clean := path.Clean(raw)
	if raw == "" || clean == "." || clean == ".." || path.IsAbs(clean) ||
		strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
		return "", false
	}
	if _, ok := existing[clean]; ok {
		return "", false
	}
	return clean, true
}

// baseContentType 去掉 MIME 参数并规整大小写。
func baseContentType(contentType string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
}
