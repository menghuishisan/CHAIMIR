// upload 提供跨模块上传安全边界,只表达通用校验结果,不持有业务错误码或用户文案。
package upload

import (
	"bytes"
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
	if maxBytes <= 0 {
		return nil, SizeTooLarge, nil
	}
	limit := maxBytes
	limit++
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

// AttachmentKindValid 校验通用附件扩展名、声明 MIME 与内容签名是否一致。
func AttachmentKindValid(fileName, contentType string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	declared := baseContentType(contentType)
	if ext == "" || declared == "" || len(content) == 0 {
		return false
	}
	switch ext {
	case ".jpg", ".jpeg":
		return declared == "image/jpeg" && looksLikeJPEG(content)
	case ".png":
		return declared == "image/png" && looksLikePNG(content)
	case ".gif":
		return declared == "image/gif" && looksLikeGIF(content)
	case ".webp":
		return declared == "image/webp" && looksLikeWEBP(content)
	case ".pdf":
		return declared == "application/pdf" && bytes.HasPrefix(content, []byte("%PDF-"))
	case ".txt":
		return declared == "text/plain" && looksLikeText(content)
	case ".md", ".markdown":
		return declared == "text/markdown" && looksLikeText(content)
	case ".json":
		return declared == "application/json" && looksLikeText(content)
	case ".csv":
		return AllowedCSVContentType(declared) && looksLikeText(content)
	default:
		return false
	}
}

// MarkdownKindValid 校验实验报告等 Markdown 文档的扩展名、声明 MIME 与文本内容一致。
func MarkdownKindValid(fileName, contentType string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	declared := baseContentType(contentType)
	return (ext == ".md" || ext == ".markdown") &&
		(declared == "text/markdown" || declared == "text/plain") &&
		looksLikeText(content)
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

// looksLikeJPEG 识别 JPEG SOI 魔数。
func looksLikeJPEG(content []byte) bool {
	return len(content) >= 3 && content[0] == 0xff && content[1] == 0xd8 && content[2] == 0xff
}

// looksLikePNG 识别 PNG 文件签名。
func looksLikePNG(content []byte) bool {
	return len(content) >= 8 && bytes.Equal(content[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
}

// looksLikeGIF 识别 GIF87a/GIF89a 文件签名。
func looksLikeGIF(content []byte) bool {
	return len(content) >= 6 && (bytes.Equal(content[:6], []byte("GIF87a")) || bytes.Equal(content[:6], []byte("GIF89a")))
}

// looksLikeWEBP 识别 RIFF WEBP 容器签名。
func looksLikeWEBP(content []byte) bool {
	return len(content) >= 12 && bytes.Equal(content[:4], []byte("RIFF")) && bytes.Equal(content[8:12], []byte("WEBP"))
}

// looksLikeText 拒绝二进制控制字符,用于文本类附件的轻量内容校验。
func looksLikeText(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return false
		}
		if b < 0x20 && b != '\n' && b != '\r' && b != '\t' {
			return false
		}
	}
	return true
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

// SafeArchiveDirectoryName 校验归档目录条目,允许 tar -C dir . 产生的根目录占位。
func SafeArchiveDirectoryName(name string) bool {
	raw := strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	clean := path.Clean(raw)
	if raw == "" {
		return false
	}
	if clean == "." {
		return true
	}
	return clean != ".." && !path.IsAbs(clean) && !strings.HasPrefix(clean, "../") && !strings.Contains(clean, ":")
}

// baseContentType 去掉 MIME 参数并规整大小写。
func baseContentType(contentType string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
}
