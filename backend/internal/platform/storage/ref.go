// Package storage 的对象引用辅助统一解析持久化对象地址。
package storage

import "strings"

// ObjectRef 表示一个 minio://bucket/key 对象引用。
type ObjectRef struct {
	Bucket string
	Key    string
}

// ParseObjectRef 解析 minio://bucket/key 格式对象引用,并拒绝会混淆 bucket/key 边界的路径段。
func ParseObjectRef(raw string) (ObjectRef, error) {
	if !strings.HasPrefix(raw, "minio://") {
		return ObjectRef{}, ErrObjectRefInvalid
	}
	trimmed := strings.TrimPrefix(raw, "minio://")
	idx := strings.Index(trimmed, "/")
	if idx <= 0 || idx == len(trimmed)-1 {
		return ObjectRef{}, ErrObjectRefInvalid
	}
	bucket, key := trimmed[:idx], trimmed[idx+1:]
	if !safeObjectRefBucket(bucket) || !safeObjectRefKey(key) {
		return ObjectRef{}, ErrObjectRefInvalid
	}
	return ObjectRef{Bucket: bucket, Key: key}, nil
}

// safeObjectRefBucket 限制 bucket 为单段名称,防止引用里混入路径分隔。
func safeObjectRefBucket(bucket string) bool {
	return strings.TrimSpace(bucket) != "" && !strings.Contains(bucket, "/") && !strings.Contains(bucket, "\\")
}

// safeObjectRefKey 校验 key 的每一段,允许层级但不允许空段、当前目录或上级目录。
func safeObjectRefKey(key string) bool {
	for _, seg := range strings.Split(key, "/") {
		if strings.TrimSpace(seg) == "" || seg == "." || seg == ".." || strings.Contains(seg, "\\") {
			return false
		}
	}
	return true
}
