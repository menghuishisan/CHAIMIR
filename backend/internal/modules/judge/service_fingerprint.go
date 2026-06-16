// judge service_fingerprint 文件实现提交代码特征提取与相似度计算,输入来自对象存储而不是前端。
package judge

import (
	"io"
	"math"
	"path/filepath"
	"regexp"
	"strings"

	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
)

var tokenPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*|[0-9]+|==|!=|<=|>=|&&|\|\||[{}()[\];,.:+\-*/%<>]`)

// fingerprintVectorFromArchive 从提交归档中生成归一化 token 频率向量。
func fingerprintVectorFromArchive(name string, data []byte, limits upload.ArchiveLimits) (map[string]float64, error) {
	vector := map[string]float64{}
	if err := upload.WalkArchiveFiles(name, data, limits, func(file upload.ArchiveFile) error {
		if !fingerprintFileName(file.Name) {
			return nil
		}
		if err := addTextTokens(vector, file.Reader, limits.MaxUnpackedBytes); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if _, formatErr := upload.DetectArchiveFormat(name, data); formatErr != nil {
			return nil, apperr.ErrFingerprintRequestInvalid.WithCause(formatErr)
		}
		return nil, apperr.ErrJudgeInputArchiveInvalid.WithCause(err)
	}
	normalizeVector(vector)
	return vector, nil
}

// addTextTokens 把源码文本归一化为 token 计数。
func addTextTokens(vector map[string]float64, r io.Reader, maxBytes int64) error {
	data, sizeResult, err := upload.ReadBounded(r, maxBytes)
	if err != nil {
		return apperr.ErrFingerprintSimilarityFailed.WithCause(err)
	}
	if sizeResult != upload.SizeOK {
		return apperr.ErrJudgeInputArchiveInvalid
	}
	tokens := tokenPattern.FindAllString(string(data), -1)
	for _, token := range tokens {
		token = strings.ToLower(token)
		if token == "" {
			continue
		}
		vector[token]++
	}
	return nil
}

// fingerprintFileName 判断文件是否适合进入代码特征。
func fingerprintFileName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".go", ".js", ".ts", ".sol", ".rs", ".py", ".java", ".c", ".cc", ".cpp", ".h", ".hpp", ".move":
		return true
	default:
		return false
	}
}

// normalizeVector 把计数归一化为单位向量。
func normalizeVector(vector map[string]float64) {
	var sum float64
	for _, v := range vector {
		sum += v * v
	}
	if sum == 0 {
		return
	}
	norm := math.Sqrt(sum)
	for key, value := range vector {
		vector[key] = value / norm
	}
}

// cosineSimilarity 计算两个归一化向量的余弦相似度。
func cosineSimilarity(a, b map[string]float64) float64 {
	var score float64
	if len(a) > len(b) {
		a, b = b, a
	}
	for key, av := range a {
		score += av * b[key]
	}
	return score
}
