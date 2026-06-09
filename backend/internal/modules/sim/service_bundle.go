// M4 bundle 上传处理:解析上传包、执行安全扫描、生成后端 hash 并写入对象存储。
package sim

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
)

var forbiddenBundleTokens = []string{
	"fetch(", "xmlhttprequest", "eval(", "newfunction(", "document.", "window.",
	"localstorage", "sessionstorage", ".cookie", "import(", "importscripts(", "websocket(", "eventsource(",
	"navigator.", "indexeddb", "globalthis.", "self.location", "location.href",
}

var forbiddenBundleCaseTokens = []string{
	"Function(",
}

// scanBundleWithLimits 扫描上传 bundle,并限制归档展开后的文件数与总字节数。
func scanBundleWithLimits(data []byte, filename string, limits config.UploadConfig) (map[string]any, error) {
	// 第一步根据扩展名选择归档解析方式;未知类型按单文件源码扫描。
	entries, err := bundleEntries(data, filename, limits)
	if err != nil {
		return nil, err
	}
	// 第二步逐文件扫描危险能力,命中即拒绝进入审核流程。
	for name, content := range entries {
		if !isScannableSource(name) {
			continue
		}
		if token := firstForbiddenToken(content); token != "" {
			return map[string]any{"static_scan": "failed", "file": name}, apperr.ErrSimPackageValidationFail
		}
	}
	return map[string]any{"static_scan": "passed", "file_count": len(entries)}, nil
}

// bundleHash 计算 bundle sha256,后端以此为准而不信任客户端提交值。
func bundleHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// uploadedBundle 是服务层完成扫描、hash 和对象存储后的 bundle 摘要。
type uploadedBundle struct {
	Key        string
	Hash       string
	ScanReport map[string]any
}

// StoreUploadedBundle 扫描上传包、计算后端权威 hash,并写入统一对象存储。
func (s *Service) StoreUploadedBundle(ctx context.Context, tenantID int64, code, version, filename, contentType string, data []byte, upload config.UploadConfig) (uploadedBundle, error) {
	report, err := scanBundleWithLimits(data, filename, upload)
	if err != nil {
		return uploadedBundle{}, err
	}
	if s.store == nil {
		return uploadedBundle{}, apperr.ErrSimBundleReadFail
	}
	key, err := simBundleObjectKey(tenantID, code, version, filename)
	if err != nil {
		return uploadedBundle{}, apperr.ErrSimBundleReadFail.WithCause(err)
	}
	if err := s.store.Put(ctx, s.store.BucketAttach(), key, bytes.NewReader(data), int64(len(data)), contentType); err != nil {
		return uploadedBundle{}, apperr.ErrSimBundleReadFail.WithCause(err)
	}
	return uploadedBundle{Key: key, Hash: bundleHash(data), ScanReport: report}, nil
}

// bundleEntries 把单文件或归档展开为待扫描文件表。
func bundleEntries(data []byte, filename string, limits config.UploadConfig) (map[string][]byte, error) {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".zip") {
		return simArchiveEntries(upload.ReadZipFiles(data, simArchiveLimits(limits)))
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return simArchiveEntries(upload.ReadTarGzFiles(data, simArchiveLimits(limits)))
	}
	if exceedsBundleUnpackedLimit(int64(len(data)), limits) {
		return nil, apperr.ErrSimBundleTooLarge
	}
	return map[string][]byte{filename: data}, nil
}

// simArchiveLimits 把 M4 上传配置转换为平台归档安全边界。
func simArchiveLimits(limits config.UploadConfig) upload.ArchiveLimits {
	return upload.ArchiveLimits{MaxFiles: limits.SimBundleMaxFiles, MaxUnpackedBytes: limits.SimBundleMaxUnpackedBytes}
}

// simArchiveEntries 将平台归档错误映射为 M4 业务错误码。
func simArchiveEntries(files map[string][]byte, err error) (map[string][]byte, error) {
	if err == nil {
		return files, nil
	}
	if errors.Is(err, upload.ErrArchiveTooLarge) {
		return nil, apperr.ErrSimBundleTooLarge
	}
	return nil, apperr.ErrSimPackageInvalid
}

// exceedsBundleUnpackedLimit 判断归档展开总大小是否超过配置边界。
func exceedsBundleUnpackedLimit(total int64, limits config.UploadConfig) bool {
	return limits.SimBundleMaxUnpackedBytes > 0 && total > limits.SimBundleMaxUnpackedBytes
}

// isScannableSource 判断文件是否属于需要扫描的源码/声明文件。
func isScannableSource(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".js", ".mjs", ".cjs", ".ts", ".tsx", ".jsx", ".json", ".html":
		return true
	default:
		return filepath.Ext(name) == ""
	}
}

// firstForbiddenToken 返回源码中第一个命中的危险 token。
func firstForbiddenToken(content []byte) string {
	text := normalizeBundleSource(content, true)
	for _, token := range forbiddenBundleTokens {
		if strings.Contains(text, token) {
			return token
		}
	}
	caseText := normalizeBundleSource(content, false)
	for _, token := range forbiddenBundleCaseTokens {
		if strings.Contains(caseText, token) {
			return token
		}
	}
	return ""
}

// normalizeBundleSource 去掉注释和空白,按需折叠大小写,用于发现被拆分的危险能力调用。
func normalizeBundleSource(content []byte, foldCase bool) string {
	src := string(content)
	var out strings.Builder
	out.Grow(len(src))
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if ch == '/' && i+1 < len(src) {
			switch src[i+1] {
			case '/':
				i += 2
				for i < len(src) && src[i] != '\n' && src[i] != '\r' {
					i++
				}
				i--
				continue
			case '*':
				i += 2
				for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
					i++
				}
				if i+1 < len(src) {
					i++
				}
				continue
			}
		}
		if isBundleIgnoredByte(ch) {
			continue
		}
		if foldCase {
			ch = asciiLower(ch)
		}
		out.WriteByte(ch)
	}
	return out.String()
}

// isBundleIgnoredByte 判断源码规范化时可忽略的 ASCII 空白。
func isBundleIgnoredByte(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\f'
}

// asciiLower 避免按 Unicode 逐字符转换源码,只处理危险 token 所需的 ASCII 范围。
func asciiLower(ch byte) byte {
	if ch >= 'A' && ch <= 'Z' {
		return ch + ('a' - 'A')
	}
	return ch
}

// simBundleObjectKey 生成仿真包 bundle 对象存储路径,复用平台对象 key 安全段规则。
func simBundleObjectKey(tenantID int64, code, version, filename string) (string, error) {
	if filename != filepath.Base(filename) || strings.Contains(filename, "\\") {
		return "", storage.ErrObjectRefInvalid
	}
	return storage.ObjectKey(tenantID, "sim", "package", code, version, filename)
}
