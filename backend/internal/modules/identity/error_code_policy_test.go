// identity error_code_policy_test 文件守护身份模块不得用动态文案复用业务错误码。
package identity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIdentityProductionCodeDoesNotUseWithMessage 验证身份模块业务错误必须在 pkg/apperr 中一错一码定义。
func TestIdentityProductionCodeDoesNotUseWithMessage(t *testing.T) {
	root := "."
	var offenders []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if strings.Contains(path, string(filepath.Separator)+"internal"+string(filepath.Separator)+"sqlcgen") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(raw), ".WithMessage(") {
			offenders = append(offenders, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan identity files: %v", err)
	}
	if len(offenders) > 0 {
		t.Fatalf("identity production code must use dedicated apperr codes instead of WithMessage: %s", strings.Join(offenders, ", "))
	}
}
