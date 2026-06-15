// storage 提供统一对象引用解析,确保模块间传递的 minio:// 引用语义一致。
package storage

import (
	"regexp"
	"strings"
)

var bucketNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// ObjectRef 表示一条 minio://bucket/key 形式的对象引用。
type ObjectRef struct {
	Bucket string
	Key    string
}

// ObjectRefString 构造 minio://bucket/key 引用,与解析校验共用同一安全规则。
func ObjectRefString(bucket, key string) (string, error) {
	if !safeObjectRefBucket(bucket) || !safeObjectRefKey(key) {
		return "", ErrObjectRefInvalid
	}
	return "minio://" + bucket + "/" + key, nil
}

// ParseObjectRef 解析 minio://bucket/key 引用,并拒绝会混淆 bucket/key 边界的非法片段。
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

// safeObjectRefBucket 限制 bucket 为 MinIO/S3 单段名称,禁止混入路径分隔或空白。
func safeObjectRefBucket(bucket string) bool {
	if bucket != strings.TrimSpace(bucket) || strings.Contains(bucket, "/") || strings.Contains(bucket, "\\") {
		return false
	}
	if !bucketNamePattern.MatchString(bucket) || strings.Contains(bucket, "..") {
		return false
	}
	return true
}

// safeObjectRefKey 校验 key 的每一段,禁止空段、首尾空白、当前目录和上级目录。
func safeObjectRefKey(key string) bool {
	for _, seg := range strings.Split(key, "/") {
		if seg != strings.TrimSpace(seg) || seg == "" || seg == "." || seg == ".." || strings.Contains(seg, "\\") {
			return false
		}
	}
	return true
}
