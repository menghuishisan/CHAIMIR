// content convert_sensitive 文件实现题面敏感字段剥离,确保答案和判题配置不下发。
package content

import (
	"strings"

	"chaimir/internal/platform/jsonx"
)

var defaultSensitivePaths = []string{
	"answer",
	"answers",
	"correct_answer",
	"solution",
	"analysis.answer",
	"judge_config",
	"judge_config.testcases",
	"judge_config.hidden_cases",
	"testcases",
	"hidden_testcases",
	"flag",
	"flags",
}

// faceSnapshot 返回剥离敏感字段后的题面快照。
func faceSnapshot(item ItemWithBody) (ItemWithBody, error) {
	out := item
	body, err := stripSensitiveFields(item.Body, item.SensitiveFields)
	if err != nil {
		return ItemWithBody{}, err
	}
	out.Body = body
	out.SensitiveFields = nil
	return out, nil
}

// stripSensitiveFields 递归删除显式和默认敏感路径。
func stripSensitiveFields(body map[string]any, fields []string) (map[string]any, error) {
	out, err := jsonx.CloneObjectStrict(body)
	if err != nil {
		return nil, err
	}
	for _, path := range defaultSensitivePaths {
		deletePath(out, path)
	}
	for _, path := range fields {
		deletePath(out, path)
	}
	return out, nil
}

// deletePath 按点分路径删除字段,并对数组内对象递归应用同一路径。
func deletePath(value any, path string) {
	parts := splitPath(path)
	if len(parts) == 0 {
		return
	}
	deletePathParts(value, parts)
}

// deletePathParts 执行递归删除。
func deletePathParts(value any, parts []string) {
	switch node := value.(type) {
	case map[string]any:
		if len(parts) == 1 {
			delete(node, parts[0])
		} else if child, ok := node[parts[0]]; ok {
			deletePathParts(child, parts[1:])
		}
		for _, child := range node {
			deletePathParts(child, parts)
		}
	case []any:
		for _, child := range node {
			deletePathParts(child, parts)
		}
	}
}

// splitPath 清理敏感字段路径。
func splitPath(path string) []string {
	raw := strings.Split(strings.TrimSpace(path), ".")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
