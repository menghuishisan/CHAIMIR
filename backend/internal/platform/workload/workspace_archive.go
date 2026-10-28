// workload workspace_archive 为沙箱保存/恢复和评测提交/注入统一剔除平台私有副本。
package workload

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"
)

// IsPrivateWorkspacePath 只匹配根保留项及其子路径,不占用学生其他目录的同名文件。
func IsPrivateWorkspacePath(name string) bool {
	name = path.Clean(name)
	const initScriptCopy = ".chaimir-init-script"
	return name == initScriptCopy || strings.HasPrefix(name, initScriptCopy+"/")
}

// SanitizeWorkspaceArchive 移除已安全重打包或受控 helper 生成的 TAR 中的平台私有副本。
// 不含保留项时返回原归档,避免改写既有学生代码哈希;不打印或保存被移除的正文。
func SanitizeWorkspaceArchive(data []byte) ([]byte, error) {
	reader := tar.NewReader(bytes.NewReader(data))
	// 常规归档只扫描成员头,没有历史副本时不额外复制整份代码归档。
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return data, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read workspace archive header: %w", err)
		}
		if IsPrivateWorkspacePath(header.Name) {
			break
		}
	}
	var out bytes.Buffer
	reader = tar.NewReader(bytes.NewReader(data))
	writer := tar.NewWriter(&out)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read workspace archive header: %w", err)
		}
		if IsPrivateWorkspacePath(header.Name) {
			continue
		}
		if err := writer.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("write workspace archive header: %w", err)
		}
		if _, err := io.Copy(writer, reader); err != nil {
			return nil, fmt.Errorf("copy workspace archive entry: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close workspace archive: %w", err)
	}
	return out.Bytes(), nil
}
