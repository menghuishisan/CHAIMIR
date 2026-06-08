// M3 提交指纹与相似度计算:提供查重能力,不做作弊业务判定。
package judge

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"math"
	"regexp"
	"strings"

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
func fingerprintVectorFromArchive(raw []byte) (map[string]float64, error) {
	// 第一步优先按 tar.gz 归档解析,符合 M2/M3 代码对象的默认保存格式。
	reader, err := gzip.NewReader(bytes.NewReader(raw))
	if err == nil {
		vector, tarErr := fingerprintVectorFromTar(reader)
		if closeErr := reader.Close(); closeErr != nil {
			return nil, apperr.ErrFingerprintInvalid.WithCause(errors.Join(tarErr, closeErr))
		}
		return vector, tarErr
	}
	// 第二步非归档内容按单文件源码处理,便于内部调用方上传单文件提交。
	return fingerprintVectorFromText(string(raw)), nil
}

// fingerprintVectorFromTar 遍历归档内源码文件并聚合 token 向量。
func fingerprintVectorFromTar(reader io.Reader) (map[string]float64, error) {
	tr := tar.NewReader(reader)
	var builder strings.Builder
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, apperr.ErrFingerprintInvalid.WithCause(err)
		}
		// 只处理普通文件,目录和特殊文件不参与相似度特征。
		if header.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, apperr.ErrFingerprintInvalid.WithCause(err)
		}
		builder.Write(data)
		builder.WriteByte('\n')
	}
	return fingerprintVectorFromText(builder.String()), nil
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
