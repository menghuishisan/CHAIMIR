// config 提供环境变量读取辅助和本地 .env 加载能力。
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// getCSV 读取逗号分隔配置并清理空白项。
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

// getKeyValueMap 读取 key=value 逗号分隔配置,用于 Kubernetes selector 这类小型结构化环境变量。
func getKeyValueMap(key string, errs *[]string) map[string]string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	items := getCSV(key)
	out := make(map[string]string, len(items))
	for _, item := range items {
		k, v, ok := strings.Cut(item, "=")
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if !ok || k == "" || v == "" {
			*errs = append(*errs, fmt.Sprintf("环境变量 %s 需为 key=value 逗号分隔格式,非法项=%q", key, item))
			continue
		}
		out[k] = v
	}
	return out
}

// LoadDotEnv 在本地开发时加载 .env;已有环境变量优先,避免覆盖外部注入。
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
