// pkg 边界测试:守护通用库不依赖 internal,并符合中文职责注释规范。
package pkg_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPkgDoesNotDependOnInternal 确认 pkg 可外部复用,不反向依赖 internal 目录。
func TestPkgDoesNotDependOnInternal(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), `"chaimir/internal/`) {
			t.Fatalf("%s imports internal package from pkg", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk pkg: %v", err)
	}
}

// TestPkgProductionFilesHaveChineseComments 确认 pkg 生产文件具备中文文件职责和函数职责注释。
func TestPkgProductionFilesHaveChineseComments(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Split(string(data), "\n")
		assertHeaderComment(t, path, lines)
		assertFunctionComments(t, path, lines)
		return nil
	})
	if err != nil {
		t.Fatalf("walk pkg: %v", err)
	}
}

// assertHeaderComment 检查文件第一条非空行是中文职责注释。
func assertHeaderComment(t *testing.T, path string, lines []string) {
	t.Helper()
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !isCommentLine(trimmed) || !hasChinese(trimmed) {
			t.Fatalf("%s:%d lacks Chinese header responsibility comment", path, i+1)
		}
		return
	}
}

// assertFunctionComments 检查每个函数声明前都有中文职责注释。
func assertFunctionComments(t *testing.T, path string, lines []string) {
	t.Helper()
	for i, line := range lines {
		if !isFunctionLine(line) {
			continue
		}
		j := i - 1
		for j >= 0 && strings.TrimSpace(lines[j]) == "" {
			j--
		}
		if j < 0 || !isCommentLine(lines[j]) || !hasChinese(lines[j]) {
			t.Fatalf("%s:%d lacks Chinese function responsibility comment", path, i+1)
		}
	}
}

// isFunctionLine 识别顶层函数或方法声明。
func isFunctionLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "func ") && strings.Contains(trimmed, "(")
}

// isCommentLine 判断一行是否为 Go 注释。
func isCommentLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*")
}

// hasChinese 判断注释中是否包含中文字符。
func hasChinese(line string) bool {
	for _, r := range line {
		if r >= '\u4e00' && r <= '\u9fff' {
			return true
		}
	}
	return false
}
