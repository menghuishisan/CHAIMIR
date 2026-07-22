// config env 文件提供环境变量读取辅助。
package config

import (
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
