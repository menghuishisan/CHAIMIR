// file_access 提供迁移验收数据使用的根目录约束文件读取能力。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// readSeedFile 在目标文件所在目录内读取文件，阻止符号链接和路径组件逃逸到目录外。
func readSeedFile(path string) ([]byte, error) {
	file, err := os.OpenInRoot(filepath.Dir(path), filepath.Base(path))
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf("关闭文件失败: %w", closeErr)
	}
	return raw, nil
}
