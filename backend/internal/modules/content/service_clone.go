package content

import (
	"context"
	"path"
	"strings"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
)

// CloneItem 克隆本租户或共享库内容为独立草稿。
func (s *Service) CloneItem(ctx context.Context, actor auditActorSnapshot, itemID int64, req CloneItemRequest) (ItemSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	req.NewCode = strings.TrimSpace(req.NewCode)
	req.NewVersion = strings.TrimSpace(req.NewVersion)
	if itemID <= 0 || !validCode(req.NewCode) || !validVersion(req.NewVersion) {
		return ItemSnapshotDTO{}, apperr.ErrContentCloneInvalid
	}
	if err := actor.validate(id); err != nil {
		return ItemSnapshotDTO{}, err
	}
	var source ItemWithBody
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		source, err = tx.GetVisibleItemWithBodyByID(ctx, id.TenantID, itemID)
		return err
	}); err != nil {
		if isNoRows(err) {
			return ItemSnapshotDTO{}, apperr.ErrContentSharedNotFound
		}
		return ItemSnapshotDTO{}, apperr.AsAppError(err)
	}
	if source.TenantID != id.TenantID && source.Visibility != VisibilityShared {
		return ItemSnapshotDTO{}, apperr.ErrContentSharedNotFound
	}
	if source.Visibility == VisibilityPrivate && source.AuthorID != id.AccountID {
		return ItemSnapshotDTO{}, apperr.ErrContentNotFound
	}
	if source.Status != StatusPublished {
		return ItemSnapshotDTO{}, apperr.ErrContentCloneInvalid
	}
	sourceTenantID := source.TenantID
	sourceCode, sourceVersion := source.Code, source.Version
	if sourceTenantID != id.TenantID {
		source.CategoryID = 0
	}
	source.ID = s.ids.Generate()
	source.TenantID = id.TenantID
	source.Code = req.NewCode
	source.Version = req.NewVersion
	source.AuthorID = id.AccountID
	source.AuthorType = AuthorTeacher
	source.Visibility = VisibilityPrivate
	source.Status = StatusDraft
	source.UsageCount = 0
	var copied []storage.ObjectRef
	source.Body, copied, err = s.cloneBodyAttachments(ctx, source.Body, sourceTenantID, id.TenantID, source.ID, source.Code, source.Version)
	if err != nil {
		return ItemSnapshotDTO{}, apperr.AsAppError(err)
	}
	source.VersionHash, err = versionHash(source.Item, source.Body, source.SensitiveFields)
	if err != nil {
		return ItemSnapshotDTO{}, apperr.ErrContentBodyInvalid.WithCause(storage.CompensateUpload(ctx, s.cleanupTimeout, err, false, func(ctx context.Context) error { return s.cleanupAttachments(ctx, copied) }))
	}
	var out ItemSnapshotDTO
	commitAttempted := false
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		created, err := tx.CreateItem(ctx, source)
		if err != nil {
			return err
		}
		out, err = itemSnapshotDTO(created, true)
		if err != nil {
			return apperr.ErrContentBodyInvalid.WithCause(err)
		}
		if err := s.writeAudit(ctx, id.TenantID, actor.accountID, actor.role, "content.clone", contentAuditTargetItem, created.ID, map[string]any{"source_id": ids.Format(itemID), "source_tenant_id": ids.Format(sourceTenantID), "source_code": sourceCode, "source_version": sourceVersion, "code": created.Code}); err != nil {
			return err
		}
		commitAttempted = true
		return nil
	}); err != nil {
		err = storage.CompensateUpload(ctx, s.cleanupTimeout, err, commitAttempted, func(ctx context.Context) error { return s.cleanupAttachments(ctx, copied) })
		return ItemSnapshotDTO{}, apperr.AsAppError(err)
	}
	return out, nil
}

// cloneBodyAttachments 复制正文及隐藏配置中的租户对象，平台素材保留原引用。
func (s *Service) cloneBodyAttachments(ctx context.Context, body map[string]any, sourceTenantID, targetTenantID, targetItemID int64, targetCode, targetVersion string) (map[string]any, []storage.ObjectRef, error) {
	cloned, err := jsonx.CloneObjectStrict(body)
	if err != nil {
		return nil, nil, apperr.ErrContentBodyInvalid.WithCause(err)
	}
	var copied []storage.ObjectRef
	refs := make(map[storage.ObjectRef]string)
	_, err = storage.RewriteObjectRefs(cloned, func(ref storage.ObjectRef) (string, error) {
		if ref.Bucket != s.storage.BucketAttach() && (s.codeBucket == "" || ref.Bucket != s.codeBucket) {
			return "", apperr.ErrContentCloneInvalid
		}
		if strings.HasPrefix(ref.Key, "0/") {
			return storage.ObjectRefString(ref.Bucket, ref.Key)
		}
		if !strings.HasPrefix(ref.Key, ids.Format(sourceTenantID)+"/") {
			return "", apperr.ErrContentCloneInvalid
		}
		if target, ok := refs[ref]; ok {
			return target, nil
		}
		target, object, err := s.copyAttachmentObject(ctx, ref, targetTenantID, targetItemID, targetCode+"-"+targetVersion)
		if err != nil {
			return "", err
		}
		copied = append(copied, object)
		refs[ref] = target
		return target, nil
	})
	if err != nil {
		err = storage.CompensateUpload(ctx, s.cleanupTimeout, err, false, func(ctx context.Context) error { return s.cleanupAttachments(ctx, copied) })
		return nil, nil, apperr.AsAppError(err)
	}
	return cloned, copied, nil
}

func (s *Service) copyAttachmentObject(ctx context.Context, ref storage.ObjectRef, targetTenantID, targetItemID int64, resourceID string) (string, storage.ObjectRef, error) {
	fileName := path.Base(ref.Key)
	key, err := storage.ObjectKey(targetTenantID, contentModuleName, contentAttachmentResourceType, resourceID, attachmentCopyName(targetItemID, ref.Bucket+"/"+ref.Key, fileName))
	if err != nil {
		return "", storage.ObjectRef{}, apperr.ErrContentCloneInvalid.WithCause(err)
	}
	newRef, err := storage.ObjectRefString(s.storage.BucketAttach(), key)
	if err != nil {
		return "", storage.ObjectRef{}, apperr.ErrContentCloneInvalid.WithCause(err)
	}
	copiedRef := storage.ObjectRef{Bucket: s.storage.BucketAttach(), Key: key}
	size, err := s.storage.CopyObject(ctx, ref.Bucket, ref.Key, copiedRef.Bucket, copiedRef.Key)
	if err == nil && (size <= 0 || size > s.contentAttachmentMaxBytes) {
		err = apperr.ErrContentCloneInvalid
	}
	if err != nil {
		return "", storage.ObjectRef{}, apperr.ErrContentCloneInvalid.WithCause(storage.CompensateUpload(ctx, s.cleanupTimeout, err, false, func(ctx context.Context) error { return s.cleanupAttachments(ctx, []storage.ObjectRef{copiedRef}) }))
	}
	return newRef, copiedRef, nil
}

// attachmentCopyName 为克隆附件生成稳定且不会与源文件同名覆盖的单段文件名。
func attachmentCopyName(targetItemID int64, sourceKey, fileName string) string {
	name := strings.TrimSpace(fileName)
	if name == "" {
		name = "attachment"
	}
	hash := crypto.SHA256Hex([]byte(sourceKey))
	if len(hash) > 12 {
		hash = hash[:12]
	}
	return ids.Format(targetItemID) + "-" + hash + "-" + name
}
