package storage

import "strings"

// RewriteObjectRefs 改写已深拷贝 JSON 内的对象引用；解析和改写失败均中止。
func RewriteObjectRefs(value any, rewrite func(ObjectRef) (string, error)) (any, error) {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			out, err := RewriteObjectRefs(child, rewrite)
			if err != nil {
				return nil, err
			}
			node[key] = out
		}
	case []any:
		for i, child := range node {
			out, err := RewriteObjectRefs(child, rewrite)
			if err != nil {
				return nil, err
			}
			node[i] = out
		}
	case string:
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(node)), "s3://") {
			ref, err := ParseObjectRef(strings.TrimSpace(node))
			if err != nil {
				return nil, err
			}
			return rewrite(ref)
		}
	}
	return value, nil
}
