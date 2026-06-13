// sim service_bundle 文件负责仿真包上传、归档安全校验和危险调用静态扫描。
package sim

import (
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
	findings := []string{}
	err := upload.WalkArchiveFiles(name, data, limits, func(file upload.ArchiveFile) error {
		if !scanCandidate(file.Name) {
			return nil
		}
		content, err := upload.ReadArchiveFileContent(file, limits.MaxUnpackedBytes)
		if err != nil {
			return err
		}
		findings = append(findings, scanContent(file.Name, content)...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return findings, nil
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
