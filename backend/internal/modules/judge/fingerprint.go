// M3 提交指纹与相似度计算:提供查重能力,不做作弊业务判定。
package judge

import (
	"bytes"
	"math"
	"regexp"
	"strings"

	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
)

var tokenPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// cosineSimilarity 计算两个稀疏向量的余弦相似度;空向量返回 0,避免除零。
func cosineSimilarity(a, b map[string]float64) float64 {
	var dot, normA, normB float64
	for key, va := range a {
		dot += va * b[key]
		normA += va * va
	}
	for _, vb := range b {
		normB += vb * vb
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// fingerprintVectorFromArchive 从提交归档或源码字节提取归一化 token 频率向量。
func fingerprintVectorFromArchive(raw []byte, limits upload.ArchiveLimits) (map[string]float64, error) {
	// 第一步优先按 tar.gz 归档解析,符合 M2/M3 代码对象的默认保存格式。
	if bytes.HasPrefix(raw, []byte{0x1f, 0x8b}) {
		files, err := upload.ReadTarGzFiles(raw, limits)
		if err != nil {
			return nil, apperr.ErrFingerprintInvalid.WithCause(err)
		}
		return fingerprintVectorFromFiles(files), nil
	}
	// 第二步非归档内容按单文件源码处理,便于内部调用方上传单文件提交。
	return fingerprintVectorFromText(string(raw)), nil
}

// fingerprintVectorFromFiles 聚合归档内普通文件内容并提取 token 向量。
func fingerprintVectorFromFiles(files map[string][]byte) map[string]float64 {
	var builder strings.Builder
	for _, data := range files {
		builder.Write(data)
		builder.WriteByte('\n')
	}
	return fingerprintVectorFromText(builder.String())
}

// fingerprintVectorFromText 从源码文本提取小写 token 并做 L2 归一化。
func fingerprintVectorFromText(text string) map[string]float64 {
	counts := map[string]float64{}
	for _, token := range tokenPattern.FindAllString(text, -1) {
		counts[strings.ToLower(token)]++
	}
	var norm float64
	for _, count := range counts {
		norm += count * count
	}
	if norm == 0 {
		return map[string]float64{}
	}
	scale := math.Sqrt(norm)
	for token, count := range counts {
		counts[token] = count / scale
	}
	return counts
}
