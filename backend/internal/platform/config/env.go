// 配置读取辅助 + 本地 .env 加载(仅开发便利,生产用环境变量注入)。
package config

import (
	"bufio"
	"errors"
	"fmt"

	"os"
	"strings"
)

// getCSV 读取逗号分隔配置并去除空白项。
func getCSV(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}

// LoadDotEnv 加载本地 .env(若存在)到环境变量;已存在的不覆盖(注入优先)。
func LoadDotEnv(path string) (err error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("关闭 %s 失败: %w", path, closeErr))
		}
	}()

	scanner := bufio.NewScanner(f)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("加载 %s 失败: 第 %d 行环境变量名为空", path, lineNumber)
		}
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, val); err != nil {
				return fmt.Errorf("设置环境变量 %s 失败: %w", key, err)
			}
		}
	}
	return scanner.Err()
}
