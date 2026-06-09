// upload 提供 ZIP/TAR 归档成员的统一安全校验,供沙箱初始化包和判题输入包复用。
package upload

import (
	"archive/tar"
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
)

// ArchiveLimits 描述归档校验时允许的文件数和展开总大小上限。
type ArchiveLimits struct {
	MaxFiles         int
	MaxUnpackedBytes int64
}

// ZIPEntryNames 校验 ZIP 成员路径、数量、大小和类型,返回规范化后的成员名列表。
func ZIPEntryNames(r *zip.Reader, limits ArchiveLimits) ([]string, error) {
	if r == nil {
		return nil, fmt.Errorf("ZIP 归档为空")
	}
	return walkEntries(len(r.File), limits, func(visit func(name string, size int64) error) error {
		for _, file := range r.File {
			// 第一步:目录只校验路径安全,不计入实际文件数量。
			if file.FileInfo().IsDir() {
				if _, ok := SafeArchiveEntryName(file.Name, map[string]struct{}{}); !ok {
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
				if _, ok := SafeArchiveEntryName(header.Name, map[string]struct{}{}); !ok {
					return fmt.Errorf("TAR 归档成员路径非法: %s", header.Name)
				}
				continue
			case tar.TypeReg, tar.TypeRegA:
				if err := visit(header.Name, header.Size); err != nil {
					return err
				}
			default:
				return fmt.Errorf("TAR 归档包含不受支持的成员类型: %s", header.Name)
			}
		}
	})
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
