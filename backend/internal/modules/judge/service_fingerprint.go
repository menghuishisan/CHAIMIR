// judge service_fingerprint 文件实现提交代码特征提取与相似度计算,输入来自对象存储而不是前端。
package judge

import (
	"archive/tar"
	"archive/zip"
	"bytes"
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
	format, err := upload.DetectArchiveFormat(name, data)
	if err != nil {
		return nil, apperr.ErrFingerprintRequestInvalid.WithCause(err)
	}
	switch format {
	case upload.ArchiveFormatZIP:
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, apperr.ErrFingerprintRequestInvalid.WithCause(err)
		}
		if _, err := upload.ZIPEntryNames(zr, limits); err != nil {
			return nil, apperr.ErrJudgeInputArchiveInvalid.WithCause(err)
		}
		return fingerprintVectorFromZIP(zr)
	case upload.ArchiveFormatTAR:
		if _, err := upload.TAREntryNames(tar.NewReader(bytes.NewReader(data)), limits); err != nil {
			return nil, apperr.ErrJudgeInputArchiveInvalid.WithCause(err)
		}
		return fingerprintVectorFromTar(tar.NewReader(bytes.NewReader(data)))
	default:
		return nil, apperr.ErrFingerprintRequestInvalid
	}
}

// fingerprintVectorFromZIP 遍历 ZIP 普通源码文件。
func fingerprintVectorFromZIP(zr *zip.Reader) (map[string]float64, error) {
	vector := map[string]float64{}
	for _, file := range zr.File {
		if file.FileInfo().IsDir() || !fingerprintFileName(file.Name) {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, apperr.ErrFingerprintSimilarityFailed.WithCause(err)
		}
		if err := addTextTokens(vector, rc); err != nil {
			rc.Close()
			return nil, err
		}
		if err := rc.Close(); err != nil {
			return nil, apperr.ErrFingerprintSimilarityFailed.WithCause(err)
		}
	}
	normalizeVector(vector)
	return vector, nil
}

// fingerprintVectorFromTar 遍历 TAR 普通源码文件。
func fingerprintVectorFromTar(tr *tar.Reader) (map[string]float64, error) {
	vector := map[string]float64{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, apperr.ErrFingerprintSimilarityFailed.WithCause(err)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		if !fingerprintFileName(header.Name) {
			continue
		}
		if err := addTextTokens(vector, tr); err != nil {
			return nil, err
		}
	}
	normalizeVector(vector)
	return vector, nil
}

// addTextTokens 把源码文本归一化为 token 计数。
func addTextTokens(vector map[string]float64, r io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(r, 4<<20))
	if err != nil {
		return apperr.ErrFingerprintSimilarityFailed.WithCause(err)
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
