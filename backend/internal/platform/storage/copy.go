package storage

import (
	"context"
	"errors"
	"fmt"
	"path"

	"chaimir/pkg/crypto"
)

type objectCopyStore interface {
	CopyObject(context.Context, string, string, string, string) (int64, error)
	Delete(context.Context, string, string) error
}

// CopyBatch 复制对象到独立前缀并去重；来源授权由所属业务模块负责。
// 未知提交结果时不得调用 Discard，部分复制失败仍须补偿。
type CopyBatch struct {
	store    objectCopyStore
	prefix   string
	maxBytes int64
	copied   map[ObjectRef]string
	objects  []ObjectRef
}

// NewCopyBatch 要求调用方提供本次操作独占的目标前缀及单个对象上限。
func NewCopyBatch(store objectCopyStore, prefix string, maxBytes int64) (*CopyBatch, error) {
	if store == nil || !safeObjectRefKey(prefix) || maxBytes <= 0 {
		return nil, fmt.Errorf("对象复制参数无效")
	}
	return &CopyBatch{store: store, prefix: prefix, maxBytes: maxBytes, copied: make(map[ObjectRef]string)}, nil
}

// Copy 保留来源桶，以桶名和完整来源键生成不冲突的目标名，不读取对象到应用内存。
func (b *CopyBatch) Copy(ctx context.Context, ref ObjectRef) (string, error) {
	if target, ok := b.copied[ref]; ok {
		return target, nil
	}
	name := crypto.SHA256Hex([]byte(ref.Bucket + "/" + ref.Key))[:16] + "-" + path.Base(ref.Key)
	target := ObjectRef{Bucket: ref.Bucket, Key: b.prefix + "/" + name}
	out, err := ObjectRefString(target.Bucket, target.Key)
	if err != nil {
		return "", err
	}
	b.objects = append(b.objects, target)
	size, err := b.store.CopyObject(ctx, ref.Bucket, ref.Key, target.Bucket, target.Key)
	if err != nil {
		return "", err
	}
	if size <= 0 || size > b.maxBytes {
		return "", fmt.Errorf("对象复制大小超出范围: %d", size)
	}
	b.copied[ref] = out
	return out, nil
}

// Discard 只删除该批次产生的目标，不修改来源对象。
func (b *CopyBatch) Discard(ctx context.Context) error {
	var errs []error
	for _, object := range b.objects {
		if err := b.store.Delete(ctx, object.Bucket, object.Key); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
