// upload 提供 ZIP/TAR 归档成员的统一安全校验,供沙箱初始化包和判题输入包复用。
package upload

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
)

// ArchiveLimits 描述归档校验时允许的文件数和展开总大小上限。
type ArchiveLimits struct {
	MaxFiles         int
	MaxUnpackedBytes int64
}

// ArchiveFile 表示已通过路径、类型和配额校验的普通归档成员。
type ArchiveFile struct {
	Name   string
	Size   int64
	Reader io.Reader
}

// ArchiveFormat 表示平台归档安全原语支持的文件格式。
type ArchiveFormat int

const (
	// ArchiveFormatUnknown 表示无法识别或不允许的归档格式。
	ArchiveFormatUnknown ArchiveFormat = iota
	// ArchiveFormatZIP 表示 ZIP 归档。
	ArchiveFormatZIP
	// ArchiveFormatTAR 表示 TAR 归档。
	ArchiveFormatTAR
)

// ZIPEntryNames 校验 ZIP 成员路径、数量、大小和类型,返回规范化后的成员名列表。
func ZIPEntryNames(r *zip.Reader, limits ArchiveLimits) ([]string, error) {
	if r == nil {
		return nil, fmt.Errorf("ZIP 归档为空")
	}
	return walkEntries(len(r.File), limits, func(visit func(name string, size int64) error) error {
		for _, file := range r.File {
			// 第一步:目录只校验路径安全,不计入实际文件数量。
			if file.FileInfo().IsDir() {
				if !SafeArchiveDirectoryName(file.Name) {
					return fmt.Errorf("ZIP 归档成员路径非法: %s", file.Name)
				}
				continue
			}
			// 第二步:拒绝链接等非普通文件,避免解包后逃逸或覆盖。
			mode := file.Mode()
			if mode&fs.ModeSymlink != 0 || !mode.IsRegular() {
				return fmt.Errorf("ZIP 归档包含不受支持的成员类型: %s", file.Name)
			}
			if err := visit(file.Name, int64(file.UncompressedSize64)); err != nil {
				return err
			}
		}
		return nil
	})
}

// TAREntryNames 校验 TAR 成员路径、数量、大小和类型,返回规范化后的成员名列表。
func TAREntryNames(r *tar.Reader, limits ArchiveLimits) ([]string, error) {
	if r == nil {
		return nil, fmt.Errorf("TAR 归档为空")
	}
	return walkEntries(0, limits, func(visit func(name string, size int64) error) error {
		for {
			header, err := r.Next()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("读取 TAR 归档失败: %w", err)
			}
			switch header.Typeflag {
			case tar.TypeDir:
				if !SafeArchiveDirectoryName(header.Name) {
					return fmt.Errorf("TAR 归档成员路径非法: %s", header.Name)
				}
				continue
			case tar.TypeReg:
				if err := visit(header.Name, header.Size); err != nil {
					return err
				}
			default:
				return fmt.Errorf("TAR 归档包含不受支持的成员类型: %s", header.Name)
			}
		}
	})
}

// SafeArchiveTar 校验 ZIP/TAR 归档并重打为只含普通文件的安全 TAR。
func SafeArchiveTar(name string, data []byte, limits ArchiveLimits) ([]byte, error) {
	format, err := DetectArchiveFormat(name, data)
	if err != nil {
		return nil, err
	}
	switch format {
	case ArchiveFormatZIP:
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		if _, err := ZIPEntryNames(zr, limits); err != nil {
			return nil, err
		}
		return zipToSafeTar(zr, limits)
	case ArchiveFormatTAR:
		if _, err := TAREntryNames(tar.NewReader(bytes.NewReader(data)), limits); err != nil {
			return nil, err
		}
		return tarToSafeTar(tar.NewReader(bytes.NewReader(data)), limits)
	default:
		return nil, fmt.Errorf("归档格式不支持")
	}
}

// WalkArchiveFiles 校验 ZIP/TAR 后按普通文件逐个回调,供模块复用统一归档安全边界。
func WalkArchiveFiles(name string, data []byte, limits ArchiveLimits, visit func(ArchiveFile) error) error {
	if visit == nil {
		return fmt.Errorf("归档文件访问回调不能为空")
	}
	format, err := DetectArchiveFormat(name, data)
	if err != nil {
		return err
	}
	switch format {
	case ArchiveFormatZIP:
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return err
		}
		names, err := ZIPEntryNames(zr, limits)
		if err != nil {
			return err
		}
		return walkSafeZIPFiles(zr, names, visit)
	case ArchiveFormatTAR:
		if _, err := TAREntryNames(tar.NewReader(bytes.NewReader(data)), limits); err != nil {
			return err
		}
		return walkSafeTARFiles(tar.NewReader(bytes.NewReader(data)), visit)
	default:
		return fmt.Errorf("归档格式不支持")
	}
}

// DetectArchiveFormat 根据文件名和魔数识别 ZIP/TAR 归档。
func DetectArchiveFormat(name string, data []byte) (ArchiveFormat, error) {
	lower := strings.ToLower(strings.TrimSpace(name))
	if strings.HasSuffix(lower, ".zip") || bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return ArchiveFormatZIP, nil
	}
	if strings.HasSuffix(lower, ".tar") || looksLikeTar(data) {
		return ArchiveFormatTAR, nil
	}
	return ArchiveFormatUnknown, fmt.Errorf("归档格式不支持")
}

// ReadArchiveFileContent 读取已校验归档成员内容并执行单成员上限,供模块扫描逻辑复用。
func ReadArchiveFileContent(file ArchiveFile, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("归档展开大小上限必须大于 0")
	}
	if file.Reader == nil {
		return nil, fmt.Errorf("归档成员读取器为空")
	}
	out, err := io.ReadAll(io.LimitReader(file.Reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(out)) > maxBytes {
		return nil, fmt.Errorf("归档成员大小超出上限")
	}
	return out, nil
}

// zipToSafeTar 把 ZIP 重打为最小普通文件 TAR,统一解包前的安全形态。
func zipToSafeTar(zr *zip.Reader, limits ArchiveLimits) ([]byte, error) {
	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	seen := map[string]struct{}{}
	for _, file := range zr.File {
		name, ok := SafeArchiveEntryName(file.Name, seen)
		if !ok || file.FileInfo().IsDir() {
			continue
		}
		seen[name] = struct{}{}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(file.UncompressedSize64), Typeflag: tar.TypeReg}); err != nil {
			return nil, err
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		if err := copyExactArchiveEntry(tw, rc, int64(file.UncompressedSize64), limits.MaxUnpackedBytes); err != nil {
			return nil, errors.Join(err, closeArchiveReader(rc))
		}
		if err := rc.Close(); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// walkSafeZIPFiles 在 ZIP 已通过安全校验后遍历普通文件内容。
func walkSafeZIPFiles(zr *zip.Reader, safeNames []string, visit func(ArchiveFile) error) error {
	allowed := make(map[string]struct{}, len(safeNames))
	for _, name := range safeNames {
		allowed[name] = struct{}{}
	}
	seen := map[string]struct{}{}
	for _, file := range zr.File {
		name, ok := SafeArchiveEntryName(file.Name, seen)
		if !ok || file.FileInfo().IsDir() {
			continue
		}
		if _, ok := allowed[name]; !ok {
			continue
		}
		seen[name] = struct{}{}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		err = visit(ArchiveFile{Name: name, Size: int64(file.UncompressedSize64), Reader: io.LimitReader(rc, int64(file.UncompressedSize64)+1)})
		if closeErr := rc.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// walkSafeTARFiles 在 TAR 已通过安全校验后遍历普通文件内容。
func walkSafeTARFiles(tr *tar.Reader, visit func(ArchiveFile) error) error {
	seen := map[string]struct{}{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		name, ok := SafeArchiveEntryName(header.Name, seen)
		if !ok {
			return fmt.Errorf("TAR 归档成员路径非法: %s", header.Name)
		}
		seen[name] = struct{}{}
		if err := visit(ArchiveFile{Name: name, Size: header.Size, Reader: io.LimitReader(tr, header.Size+1)}); err != nil {
			return err
		}
	}
}

// tarToSafeTar 把 TAR 重打为最小普通文件 TAR,剥离 owner、链接和特殊模式。
func tarToSafeTar(tr *tar.Reader, limits ArchiveLimits) ([]byte, error) {
	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	seen := map[string]struct{}{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("TAR 归档包含不受支持的成员类型: %s", header.Name)
		}
		name, ok := SafeArchiveEntryName(header.Name, seen)
		if !ok {
			return nil, fmt.Errorf("TAR 归档成员路径非法: %s", header.Name)
		}
		seen[name] = struct{}{}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: header.Size, Typeflag: tar.TypeReg}); err != nil {
			return nil, err
		}
		if err := copyExactArchiveEntry(tw, tr, header.Size, limits.MaxUnpackedBytes); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// closeArchiveReader 关闭归档成员读取流，并为调用方保留可定位的错误上下文。
func closeArchiveReader(reader io.Closer) error {
	if err := reader.Close(); err != nil {
		return fmt.Errorf("关闭归档成员读取流失败: %w", err)
	}
	return nil
}

// copyExactArchiveEntry 按归档头声明大小精确复制内容,防止重打包阶段绕过展开大小限制。
func copyExactArchiveEntry(dst io.Writer, src io.Reader, declaredSize, maxBytes int64) error {
	if declaredSize < 0 || maxBytes <= 0 || declaredSize > maxBytes {
		return fmt.Errorf("归档成员大小超出上限")
	}
	if _, err := io.CopyN(dst, src, declaredSize); err != nil {
		return fmt.Errorf("归档成员内容长度不足: %w", err)
	}
	var extra [1]byte
	n, err := src.Read(extra[:])
	if n > 0 || err == nil {
		return fmt.Errorf("归档成员内容超过声明大小")
	}
	if err != io.EOF {
		return fmt.Errorf("读取归档成员内容失败: %w", err)
	}
	return nil
}

// walkEntries 复用统一配额和重复路径校验,保证不同归档格式只有一套安全口径。
func walkEntries(hintCount int, limits ArchiveLimits, iter func(func(name string, size int64) error) error) ([]string, error) {
	if limits.MaxFiles <= 0 {
		return nil, fmt.Errorf("归档文件数上限必须大于 0")
	}
	if limits.MaxUnpackedBytes <= 0 {
		return nil, fmt.Errorf("归档展开大小上限必须大于 0")
	}

	seen := make(map[string]struct{}, max(hintCount, 1))
	names := make([]string, 0, max(hintCount, 1))
	var total int64
	count := 0
	err := iter(func(name string, size int64) error {
		clean, ok := SafeArchiveEntryName(name, seen)
		if !ok {
			return fmt.Errorf("归档成员路径非法或重复: %s", name)
		}
		if size < 0 {
			return fmt.Errorf("归档成员大小非法: %s", name)
		}
		count++
		if count > limits.MaxFiles {
			return fmt.Errorf("归档成员数量超出上限")
		}
		total += size
		if total > limits.MaxUnpackedBytes {
			return fmt.Errorf("归档展开大小超出上限")
		}
		seen[clean] = struct{}{}
		names = append(names, clean)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return names, nil
}

// max 返回两个整数中的较大值,用于避免空容量切片和 map。
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// looksLikeTar 通过 ustar 标记识别 TAR 归档,避免把任意二进制当成归档。
func looksLikeTar(data []byte) bool {
	return len(data) > 265 && string(data[257:262]) == "ustar"
}
