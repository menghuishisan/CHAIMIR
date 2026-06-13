// sim service_bundle 文件负责仿真包上传、归档安全校验和危险调用静态扫描。
package sim

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
)

// BundleInput 是 API 边界读取 multipart 后交给 service 的仿真包正文。
type BundleInput struct {
	FileName    string
	ContentType string
	Data        []byte
}

var dangerousBundlePatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{name: "eval", re: regexp.MustCompile(`\beval\s*\(`)},
	{name: "function-constructor", re: regexp.MustCompile(`\bFunction\s*\(`)},
	{name: "network-fetch", re: regexp.MustCompile(`\bfetch\s*\(`)},
	{name: "network-xhr", re: regexp.MustCompile(`\bXMLHttpRequest\b`)},
	{name: "dynamic-import", re: regexp.MustCompile(`\bimport\s*\(`)},
	{name: "dom-document", re: regexp.MustCompile(`\bdocument\s*\.`)},
	{name: "dom-window", re: regexp.MustCompile(`\bwindow\s*\.`)},
	{name: "storage-local", re: regexp.MustCompile(`\blocalStorage\b`)},
	{name: "storage-session", re: regexp.MustCompile(`\bsessionStorage\b`)},
	{name: "cookie", re: regexp.MustCompile(`\bcookie\b`)},
	{name: "websocket", re: regexp.MustCompile(`\bWebSocket\b`)},
}

// analyzeBundle 校验归档结构、计算 SHA-256 并执行危险调用静态扫描。
func analyzeBundle(input BundleInput, limits upload.ArchiveLimits) (string, StaticScanReport, error) {
	if strings.TrimSpace(input.FileName) == "" || len(input.Data) == 0 {
		return "", StaticScanReport{}, apperr.ErrSimBundleUnreadable
	}
	if len(input.Data) > 0 {
		hash := crypto.SHA256Hex(input.Data)
		findings, err := scanBundleEntries(input.FileName, input.Data, limits)
		if err != nil {
			return "", StaticScanReport{}, apperr.ErrSimBundleUnreadable.WithCause(err)
		}
		if len(findings) > 0 {
			return hash, StaticScanReport{Status: validationFailed, Findings: findings}, nil
		}
		return hash, StaticScanReport{Status: validationPassed}, nil
	}
	return "", StaticScanReport{}, apperr.ErrSimBundleUnreadable
}

// scanBundleEntries 遍历 ZIP/TAR 普通文件,对代码和 JSON 契约文件执行保守静态扫描。
func scanBundleEntries(name string, data []byte, limits upload.ArchiveLimits) ([]string, error) {
	format, err := upload.DetectArchiveFormat(name, data)
	if err != nil {
		return nil, err
	}
	switch format {
	case upload.ArchiveFormatZIP:
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		if _, err := upload.ZIPEntryNames(zr, limits); err != nil {
			return nil, err
		}
		return scanZIP(zr, limits)
	case upload.ArchiveFormatTAR:
		if _, err := upload.TAREntryNames(tar.NewReader(bytes.NewReader(data)), limits); err != nil {
			return nil, err
		}
		return scanTAR(tar.NewReader(bytes.NewReader(data)), limits)
	default:
		return nil, fmt.Errorf("归档格式不支持")
	}
}

// scanZIP 扫描 ZIP 成员内容。
func scanZIP(zr *zip.Reader, limits upload.ArchiveLimits) ([]string, error) {
	findings := []string{}
	for _, file := range zr.File {
		if file.FileInfo().IsDir() || !scanCandidate(file.Name) {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		content, err := readLimitedEntry(rc, limits.MaxUnpackedBytes)
		if closeErr := rc.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, err
		}
		findings = append(findings, scanContent(file.Name, content)...)
	}
	return findings, nil
}

// scanTAR 扫描 TAR 成员内容。
func scanTAR(tr *tar.Reader, limits upload.ArchiveLimits) ([]string, error) {
	findings := []string{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return findings, nil
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA || !scanCandidate(header.Name) {
			continue
		}
		content, err := readLimitedEntry(tr, minPositive(header.Size, limits.MaxUnpackedBytes))
		if err != nil {
			return nil, err
		}
		findings = append(findings, scanContent(header.Name, content)...)
	}
}

// scanCandidate 仅扫描可执行/契约文本文件,避免对图片等资产误报。
func scanCandidate(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".json":
		return true
	default:
		return false
	}
}

// scanContent 查找危险调用模式并返回可审计的命中项。
func scanContent(name string, content []byte) []string {
	text := string(content)
	findings := []string{}
	for _, item := range dangerousBundlePatterns {
		if item.re.MatchString(text) {
			findings = append(findings, name+":"+item.name)
		}
	}
	return findings
}

// readLimitedEntry 读取单个归档成员并限制上限。
func readLimitedEntry(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("归档展开大小上限必须大于 0")
	}
	out, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(out)) > maxBytes {
		return nil, fmt.Errorf("归档成员大小超出上限")
	}
	return out, nil
}

// minPositive 取正数上限中的较小值,用于防止 TAR 声明大小越过全局展开限制。
func minPositive(a, b int64) int64 {
	if a > 0 && a < b {
		return a
	}
	return b
}
