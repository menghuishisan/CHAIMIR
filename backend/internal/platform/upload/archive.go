// Package upload 的归档辅助统一处理上传包路径、展开大小和文件数安全边界。
package upload

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
)

// 归档通用错误只表达安全边界,由业务模块映射到各自错误码。
var (
	ErrArchiveInvalid  = errors.New("archive invalid")
	ErrArchiveTooLarge = errors.New("archive too large")
)

// ArchiveLimits 是归档展开的通用安全上限。
type ArchiveLimits struct {
	MaxFiles         int
	MaxUnpackedBytes int64
}

// ReadTarGzFiles 安全读取 tar.gz 普通文件,拒绝路径逃逸、重复覆盖和超限展开。
func ReadTarGzFiles(raw []byte, limits ArchiveLimits) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrArchiveInvalid
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	files := map[string][]byte{}
	existing := map[string]struct{}{}
	var total int64
	for {
		// 逐条读取归档头,目录只参与路径占用校验,实际内容只接受普通文件。
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, ErrArchiveInvalid
		}
		if header.Typeflag == tar.TypeDir {
			if _, ok := safeArchiveName(header.Name, existing); !ok {
				return nil, ErrArchiveInvalid
			}
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, ErrArchiveInvalid
		}
		name, ok := safeArchiveName(header.Name, existing)
		if !ok {
			return nil, ErrArchiveInvalid
		}
		// 普通文件按剩余展开预算读取,防止压缩包通过声明大小或实际内容突破上限。
		content, nextTotal, err := readArchiveFile(tr, header.Size, total, limits)
		if err != nil {
			return nil, err
		}
		if exceedsArchiveFileLimit(len(files)+1, limits) {
			return nil, ErrArchiveTooLarge
		}
		existing[name] = struct{}{}
		files[name] = content
		total = nextTotal
	}
	return files, nil
}

// ReadZipFiles 安全读取 zip 普通文件,拒绝路径逃逸、重复覆盖和超限展开。
func ReadZipFiles(raw []byte, limits ArchiveLimits) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, ErrArchiveInvalid
	}
	files := map[string][]byte{}
	existing := map[string]struct{}{}
	var total int64
	for _, file := range reader.File {
		// Zip 目录同样只参与路径占用校验,避免后续文件覆盖同名目录语义。
		if file.FileInfo().IsDir() {
			if _, ok := safeArchiveName(file.Name, existing); !ok {
				return nil, ErrArchiveInvalid
			}
			continue
		}
		name, ok := safeArchiveName(file.Name, existing)
		if !ok {
			return nil, ErrArchiveInvalid
		}
		if exceedsArchiveFileLimit(len(files)+1, limits) {
			return nil, ErrArchiveTooLarge
		}
		// 文件内容必须通过统一预算读取,Zip64 解包大小也不能绕过平台限制。
		rc, err := file.Open()
		if err != nil {
			return nil, ErrArchiveInvalid
		}
		content, nextTotal, readErr := readArchiveFile(rc, int64(file.UncompressedSize64), total, limits)
		closeErr := rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, ErrArchiveInvalid
		}
		existing[name] = struct{}{}
		files[name] = content
		total = nextTotal
	}
	return files, nil
}

// RewriteTarGz 安全重打包 tar.gz,只保留目录和普通文件并重置权限。
func RewriteTarGz(raw []byte, limits ArchiveLimits) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrArchiveInvalid
	}
	defer gz.Close()

	var out bytes.Buffer
	outGz := gzip.NewWriter(&out)
	tw := tar.NewWriter(outGz)
	tr := tar.NewReader(gz)
	existing := map[string]struct{}{}
	var total int64
	var fileCount int
	for {
		// 重打包时先统一路径校验,剔除符号链接、设备文件等可逃逸或不可控条目。
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, ErrArchiveInvalid
		}
		name, ok := safeArchiveName(header.Name, existing)
		if !ok {
			return nil, ErrArchiveInvalid
		}
		existing[name] = struct{}{}
		switch header.Typeflag {
		case tar.TypeDir:
			// 目录只保留规范化名称和固定权限,不继承上传包里的权限位。
			if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
				return nil, ErrArchiveInvalid
			}
		case tar.TypeReg, tar.TypeRegA:
			// 普通文件重新写入固定权限,调用方拿到的是已净化的 tar.gz。
			fileCount++
			if exceedsArchiveFileLimit(fileCount, limits) {
				return nil, ErrArchiveTooLarge
			}
			content, nextTotal, err := readArchiveFile(tr, header.Size, total, limits)
			if err != nil {
				return nil, err
			}
			if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(content))}); err != nil {
				return nil, ErrArchiveInvalid
			}
			if _, err := tw.Write(content); err != nil {
				return nil, ErrArchiveInvalid
			}
			total = nextTotal
		default:
			return nil, ErrArchiveInvalid
		}
	}
	if err := tw.Close(); err != nil {
		return nil, ErrArchiveInvalid
	}
	if err := outGz.Close(); err != nil {
		return nil, ErrArchiveInvalid
	}
	return out.Bytes(), nil
}

// safeArchiveName 复用归档条目路径规则并为目录/文件同名覆盖提供统一判定。
func safeArchiveName(name string, existing map[string]struct{}) (string, bool) {
	clean, ok := SafeArchiveEntryName(name, existing)
	if !ok {
		return "", false
	}
	return clean, true
}

// readArchiveFile 按剩余展开预算读取普通文件内容。
func readArchiveFile(r io.Reader, declaredSize, current int64, limits ArchiveLimits) ([]byte, int64, error) {
	if declaredSize < 0 {
		return nil, current, ErrArchiveInvalid
	}
	remaining := remainingArchiveBytes(current, limits)
	if declaredSize > remaining {
		return nil, current, ErrArchiveTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(r, remaining+1))
	if err != nil {
		return nil, current, ErrArchiveInvalid
	}
	if int64(len(data)) > remaining {
		return nil, current, ErrArchiveTooLarge
	}
	return data, current + int64(len(data)), nil
}

// remainingArchiveBytes 返回当前归档还能展开的字节数。
func remainingArchiveBytes(current int64, limits ArchiveLimits) int64 {
	if limits.MaxUnpackedBytes <= 0 {
		return 1<<63 - 1
	}
	return limits.MaxUnpackedBytes - current
}

// exceedsArchiveFileLimit 判断文件数量是否超过配置边界。
func exceedsArchiveFileLimit(count int, limits ArchiveLimits) bool {
	return limits.MaxFiles > 0 && count > limits.MaxFiles
}
