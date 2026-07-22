// content service_item 文件实现内容创建、版本、共享、克隆与跨模块读取。
package content

import (
	"bytes"
	"context"
	"log/slog"
	"strconv"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
	"chaimir/pkg/logging"
)

// ListItems 查询教师侧内容分页。
func (s *Service) ListItems(ctx context.Context, filter ItemListFilter) ([]ItemDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	filter.ViewerID = id.AccountID
	filter.Page, filter.Size = pagex.Normalize(filter.Page, filter.Size)
	var items []Item
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListItems(ctx, id.TenantID, filter)
		return err
	}); err != nil {
		return nil, 0, 0, 0, apperr.ErrContentQueryInvalid.WithCause(err)
	}
	out := make([]ItemDTO, 0, len(items))
	for _, item := range items {
		out = append(out, itemDTO(item))
	}
	return out, total, filter.Page, filter.Size, nil
}

// CreateItem 创建教师草稿内容。
func (s *Service) CreateItem(ctx context.Context, req CreateItemRequest) (ItemSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	req, err = validateCreateRequest(req)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	item := ItemWithBody{Item: Item{ID: s.ids.Generate(), TenantID: id.TenantID, Code: req.Code, Version: req.Version, Type: req.Type, Title: req.Title, CategoryID: req.CategoryID.Int64(), Difficulty: req.Difficulty, Tags: req.Tags, KnowledgePoints: req.KnowledgePoints, AuthorID: id.AccountID, AuthorType: AuthorTeacher, Visibility: req.Visibility, Status: StatusDraft}, Body: req.Body, SensitiveFields: req.SensitiveFields}
	item.VersionHash, err = versionHash(item.Item, item.Body, item.SensitiveFields)
	if err != nil {
		return ItemSnapshotDTO{}, apperr.ErrContentBodyInvalid.WithCause(err)
	}
	var created ItemWithBody
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		created, err = tx.CreateItem(ctx, item)
		return err
	}); err != nil {
		return ItemSnapshotDTO{}, apperr.ErrContentVersionConflict.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "content.create", contentAuditTargetItem, created.ID, map[string]any{"code": created.Code, "version": created.Version}); err != nil {
		return ItemSnapshotDTO{}, err
	}
	return itemSnapshotDTO(created, true)
}

// GetItemFaceForUser 读取题面视角内容,教师侧不返回敏感字段。
func (s *Service) GetItemFaceForUser(ctx context.Context, code, version string) (ItemSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	item, err := s.getItemWithBody(ctx, id.TenantID, code, version)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	if item.Status != StatusPublished && item.Status != StatusDeprecated {
		return ItemSnapshotDTO{}, apperr.ErrContentVersionNotPublished
	}
	if item.Visibility == VisibilityPrivate && item.AuthorID != id.AccountID {
		return ItemSnapshotDTO{}, apperr.ErrContentNotFound
	}
	face, err := faceSnapshot(item)
	if err != nil {
		return ItemSnapshotDTO{}, apperr.ErrContentBodyInvalid.WithCause(err)
	}
	return itemSnapshotDTO(face, false)
}

// GetItemFullForUser 读取教师作者可见的全量内容,学校管理员也不得越过作者边界读取答案。
func (s *Service) GetItemFullForUser(ctx context.Context, code, version string) (ItemSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	item, err := s.getItemWithBody(ctx, id.TenantID, code, version)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	if item.TenantID != id.TenantID || item.AuthorID != id.AccountID {
		return ItemSnapshotDTO{}, apperr.ErrContentFullAccessDenied
	}
	return itemSnapshotDTO(item, true)
}

// UpdateDraftItem 编辑草稿内容,已发布版本不可变。
func (s *Service) UpdateDraftItem(ctx context.Context, itemID int64, req UpdateItemRequest) (ItemSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	req, err = validateUpdateRequest(req)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	var updated ItemWithBody
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetItemWithBodyByID(ctx, id.TenantID, itemID)
		if err != nil {
			return mapContentReadError(err)
		}
		if current.AuthorID != id.AccountID {
			return apperr.ErrContentForbidden
		}
		if current.Status != StatusDraft {
			return apperr.ErrContentVersionImmutable
		}
		if err := validateContentBody(current.Type, req.Body); err != nil {
			return err
		}
		current.Title = req.Title
		current.CategoryID = req.CategoryID.Int64()
		current.Difficulty = req.Difficulty
		current.Tags = req.Tags
		current.KnowledgePoints = req.KnowledgePoints
		current.Visibility = req.Visibility
		current.Body = req.Body
		current.SensitiveFields = req.SensitiveFields
		current.VersionHash, err = versionHash(current.Item, current.Body, current.SensitiveFields)
		if err != nil {
			return apperr.ErrContentBodyInvalid.WithCause(err)
		}
		updated, err = tx.UpdateDraftItem(ctx, current)
		return mapContentMutationError(err)
	}); err != nil {
		return ItemSnapshotDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "content.update", contentAuditTargetItem, updated.ID, map[string]any{"code": updated.Code, "version": updated.Version}); err != nil {
		return ItemSnapshotDTO{}, err
	}
	return itemSnapshotDTO(updated, true)
}

// PublishItem 发布草稿内容。
func (s *Service) PublishItem(ctx context.Context, itemID int64) (ItemDTO, error) {
	return s.itemStatusTransition(ctx, itemID, "content.publish", func(ctx context.Context, tx TxStore, tenantID, id int64) (Item, error) {
		return tx.PublishItem(ctx, tenantID, id)
	})
}

// DeprecateItem 弃用已发布内容。
func (s *Service) DeprecateItem(ctx context.Context, itemID int64) (ItemDTO, error) {
	return s.itemStatusTransition(ctx, itemID, "content.deprecate", func(ctx context.Context, tx TxStore, tenantID, id int64) (Item, error) {
		return tx.DeprecateItem(ctx, tenantID, id)
	})
}

// DeleteItem 删除无引用草稿。
func (s *Service) DeleteItem(ctx context.Context, itemID int64) error {
	_, err := s.itemStatusTransition(ctx, itemID, "content.delete", func(ctx context.Context, tx TxStore, tenantID, id int64) (Item, error) {
		item, err := tx.GetItemByID(ctx, tenantID, id)
		if err != nil {
			return Item{}, mapContentReadError(err)
		}
		if item.UsageCount > 0 {
			return Item{}, apperr.ErrContentDeleteBlocked
		}
		return tx.DeleteDraftItem(ctx, tenantID, id)
	})
	return err
}

// itemStatusTransition 执行需要作者归属校验的状态流转。
func (s *Service) itemStatusTransition(ctx context.Context, itemID int64, action string, fn func(context.Context, TxStore, int64, int64) (Item, error)) (ItemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemDTO{}, err
	}
	var out Item
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetItemByID(ctx, id.TenantID, itemID)
		if err != nil {
			return mapContentReadError(err)
		}
		if current.AuthorID != id.AccountID {
			return apperr.ErrContentForbidden
		}
		out, err = fn(ctx, tx, id.TenantID, itemID)
		return mapContentMutationError(err)
	}); err != nil {
		return ItemDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, action, contentAuditTargetItem, out.ID, map[string]any{"code": out.Code, "version": out.Version}); err != nil {
		return ItemDTO{}, err
	}
	return itemDTO(out), nil
}

// ListVersions 查询同 code 的版本列表。
func (s *Service) ListVersions(ctx context.Context, code string) ([]ItemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if !validCode(code) {
		return nil, apperr.ErrContentQueryInvalid
	}
	var items []Item
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListVersions(ctx, id.TenantID, code)
		return err
	}); err != nil {
		return nil, apperr.ErrContentQueryInvalid.WithCause(err)
	}
	out := make([]ItemDTO, 0, len(items))
	for _, item := range items {
		if item.Visibility == VisibilityPrivate && item.AuthorID != id.AccountID {
			continue
		}
		if item.TenantID != id.TenantID && item.Visibility != VisibilityShared {
			continue
		}
		out = append(out, itemDTO(item))
	}
	return out, nil
}

// CreateNewVersion 从既有版本复制出新草稿。
func (s *Service) CreateNewVersion(ctx context.Context, code string, req NewVersionRequest) (ItemSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	req.SourceVersion = stringsTrim(req.SourceVersion)
	req.NewVersion = stringsTrim(req.NewVersion)
	if !validCode(code) || !validVersion(req.SourceVersion) || !validVersion(req.NewVersion) || req.SourceVersion == req.NewVersion {
		return ItemSnapshotDTO{}, apperr.ErrContentVersionInvalid
	}
	var created ItemWithBody
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		source, err := tx.GetItemWithBodyByRef(ctx, id.TenantID, code, req.SourceVersion)
		if err != nil {
			return mapContentReadError(err)
		}
		if source.TenantID != id.TenantID || source.AuthorID != id.AccountID {
			return apperr.ErrContentForbidden
		}
		source.ID = s.ids.Generate()
		source.TenantID = id.TenantID
		source.Version = req.NewVersion
		source.Status = StatusDraft
		if source.Visibility == VisibilityShared {
			source.Visibility = VisibilityTenant
		}
		source.UsageCount = 0
		source.VersionHash, err = versionHash(source.Item, source.Body, source.SensitiveFields)
		if err != nil {
			return apperr.ErrContentBodyInvalid.WithCause(err)
		}
		created, err = tx.CreateItem(ctx, source)
		return err
	}); err != nil {
		return ItemSnapshotDTO{}, apperr.ErrContentVersionConflict.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "content.new_version", contentAuditTargetItem, created.ID, map[string]any{"code": created.Code, "version": created.Version, "source_version": req.SourceVersion}); err != nil {
		return ItemSnapshotDTO{}, err
	}
	return itemSnapshotDTO(created, true)
}

// CloneItem 克隆本租户或共享库内容为独立草稿。
func (s *Service) CloneItem(ctx context.Context, code, version string, req CloneItemRequest) (ItemSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	req.NewCode = stringsTrim(req.NewCode)
	req.NewVersion = stringsTrim(req.NewVersion)
	if !validCode(code) || !validVersion(version) || !validCode(req.NewCode) || !validVersion(req.NewVersion) {
		return ItemSnapshotDTO{}, apperr.ErrContentCloneInvalid
	}
	var source ItemWithBody
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		source, err = tx.GetItemWithBodyByRef(ctx, id.TenantID, code, version)
		return err
	}); err != nil {
		if isNoRows(err) {
			return ItemSnapshotDTO{}, apperr.ErrContentSharedNotFound
		}
		if ae, ok := apperr.As(err); ok {
			return ItemSnapshotDTO{}, ae
		}
		return ItemSnapshotDTO{}, apperr.ErrContentCloneInvalid.WithCause(err)
	}
	if source.TenantID != id.TenantID && source.Visibility != VisibilityShared {
		return ItemSnapshotDTO{}, apperr.ErrContentSharedNotFound
	}
	if source.Status != StatusPublished {
		return ItemSnapshotDTO{}, apperr.ErrContentCloneInvalid
	}
	sourceTenantID := source.TenantID
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
		if ae, ok := apperr.As(err); ok {
			return ItemSnapshotDTO{}, ae
		}
		return ItemSnapshotDTO{}, apperr.ErrContentCloneInvalid.WithCause(err)
	}
	source.VersionHash, err = versionHash(source.Item, source.Body, source.SensitiveFields)
	if err != nil {
		s.cleanupClonedAttachments(ctx, copied)
		return ItemSnapshotDTO{}, apperr.ErrContentBodyInvalid.WithCause(err)
	}
	var created ItemWithBody
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		created, err = tx.CreateItem(ctx, source)
		return err
	}); err != nil {
		s.cleanupClonedAttachments(ctx, copied)
		if ae, ok := apperr.As(err); ok {
			return ItemSnapshotDTO{}, ae
		}
		return ItemSnapshotDTO{}, apperr.ErrContentCloneInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "content.clone", contentAuditTargetItem, created.ID, map[string]any{"source_code": code, "source_version": version, "code": created.Code}); err != nil {
		return ItemSnapshotDTO{}, err
	}
	return itemSnapshotDTO(created, true)
}

// cloneBodyAttachments 把正文内属于 M5 附件前缀的对象引用复制到目标租户,保证克隆副本独立。
func (s *Service) cloneBodyAttachments(ctx context.Context, body map[string]any, sourceTenantID, targetTenantID, targetItemID int64, targetCode, targetVersion string) (map[string]any, []storage.ObjectRef, error) {
	cloned, err := jsonx.CloneObjectStrict(body)
	if err != nil {
		return nil, nil, apperr.ErrContentBodyInvalid.WithCause(err)
	}
	resourceID := targetCode + "-" + targetVersion
	copied := make([]storage.ObjectRef, 0)
	out, err := s.rewriteAttachmentRefs(ctx, cloned, sourceTenantID, targetTenantID, targetItemID, resourceID, &copied)
	if err != nil {
		s.cleanupClonedAttachments(ctx, copied)
		return nil, nil, err
	}
	if mapped, ok := out.(map[string]any); ok {
		return mapped, copied, nil
	}
	s.cleanupClonedAttachments(ctx, copied)
	return nil, nil, apperr.ErrContentBodyInvalid
}

// rewriteAttachmentRefs 递归复制并替换 JSON 正文中的附件 object_ref。
func (s *Service) rewriteAttachmentRefs(ctx context.Context, value any, sourceTenantID, targetTenantID, targetItemID int64, resourceID string, copied *[]storage.ObjectRef) (any, error) {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			rewritten, err := s.rewriteAttachmentRefs(ctx, child, sourceTenantID, targetTenantID, targetItemID, resourceID, copied)
			if err != nil {
				return nil, err
			}
			node[key] = rewritten
		}
		return node, nil
	case []any:
		for i, child := range node {
			rewritten, err := s.rewriteAttachmentRefs(ctx, child, sourceTenantID, targetTenantID, targetItemID, resourceID, copied)
			if err != nil {
				return nil, err
			}
			node[i] = rewritten
		}
		return node, nil
	case string:
		ref, ok, err := parseContentAttachmentRef(node, s.storage.BucketAttach(), sourceTenantID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return node, nil
		}
		newRef, copiedRef, err := s.copyAttachmentObject(ctx, ref, targetTenantID, targetItemID, resourceID)
		if err != nil {
			return nil, err
		}
		*copied = append(*copied, copiedRef)
		return newRef, nil
	default:
		return value, nil
	}
}

// parseContentAttachmentRef 识别正文中属于源租户 M5 附件边界的对象引用。
func parseContentAttachmentRef(raw, expectedBucket string, sourceTenantID int64) (storage.ObjectRef, bool, error) {
	ref, err := storage.ParseObjectRef(strings.TrimSpace(raw))
	if err != nil {
		if strings.HasPrefix(strings.TrimSpace(raw), "minio://") {
			return storage.ObjectRef{}, false, apperr.ErrContentBodyInvalid.WithCause(err)
		}
		return storage.ObjectRef{}, false, nil
	}
	if ref.Bucket != strings.TrimSpace(expectedBucket) {
		return ref, false, nil
	}
	expectedPrefix, err := storage.ObjectKey(sourceTenantID, contentModuleName, contentAttachmentResourceType)
	if err != nil {
		return storage.ObjectRef{}, false, apperr.ErrContentBodyInvalid.WithCause(err)
	}
	if ref.Key == expectedPrefix || strings.HasPrefix(ref.Key, expectedPrefix+"/") {
		return ref, true, nil
	}
	return ref, false, nil
}

// copyAttachmentObject 复制附件对象到目标租户的 M5 附件边界并返回新 object_ref。
func (s *Service) copyAttachmentObject(ctx context.Context, ref storage.ObjectRef, targetTenantID, targetItemID int64, resourceID string) (string, storage.ObjectRef, error) {
	reader, err := s.storage.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return "", storage.ObjectRef{}, apperr.ErrContentCloneInvalid.WithCause(err)
	}
	defer logging.CloseContext(ctx, "关闭题库克隆附件对象失败", reader)
	fileName := attachmentFileName(ref.Key)
	key, err := storage.ObjectKey(targetTenantID, contentModuleName, contentAttachmentResourceType, resourceID, attachmentCopyName(targetItemID, ref.Key, fileName))
	if err != nil {
		return "", storage.ObjectRef{}, apperr.ErrContentCloneInvalid.WithCause(err)
	}
	content, sizeResult, err := upload.ReadBounded(reader, s.contentAttachmentMaxBytes)
	if err != nil {
		return "", storage.ObjectRef{}, apperr.ErrContentCloneInvalid.WithCause(err)
	}
	if len(content) == 0 || sizeResult != upload.SizeOK {
		return "", storage.ObjectRef{}, apperr.ErrContentCloneInvalid
	}
	if err := s.storage.Put(ctx, s.storage.BucketAttach(), key, bytes.NewReader(content), int64(len(content)), "application/octet-stream"); err != nil {
		return "", storage.ObjectRef{}, apperr.ErrContentCloneInvalid.WithCause(err)
	}
	newRef, err := storage.ObjectRefString(s.storage.BucketAttach(), key)
	if err != nil {
		return "", storage.ObjectRef{}, apperr.ErrContentCloneInvalid.WithCause(err)
	}
	copiedRef := storage.ObjectRef{Bucket: s.storage.BucketAttach(), Key: key}
	return newRef, copiedRef, nil
}

// cleanupClonedAttachments 清理克隆失败时已复制的对象,清理失败只进结构化日志。
func (s *Service) cleanupClonedAttachments(ctx context.Context, refs []storage.ObjectRef) {
	for _, ref := range refs {
		if err := s.storage.Delete(ctx, ref.Bucket, ref.Key); err != nil {
			logging.ErrorContext(ctx, "清理题库克隆附件失败", err.Error(), slog.String("bucket", ref.Bucket), slog.String("key", ref.Key))
		}
	}
}

// attachmentFileName 从受控对象 key 中取单段文件名。
func attachmentFileName(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) == 0 {
		return "attachment"
	}
	name := strings.TrimSpace(parts[len(parts)-1])
	if name == "" {
		return "attachment"
	}
	return name
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
	return strconv.FormatInt(targetItemID, 10) + "-" + hash + "-" + name
}

// ShareItem 把已发布内容放入共享库。
func (s *Service) ShareItem(ctx context.Context, itemID int64) (ItemDTO, error) {
	return s.setItemVisibility(ctx, itemID, VisibilityShared, "content.share")
}

// UnshareItem 取消后续共享浏览和克隆。
func (s *Service) UnshareItem(ctx context.Context, itemID int64) (ItemDTO, error) {
	return s.setItemVisibility(ctx, itemID, VisibilityTenant, "content.unshare")
}

// setItemVisibility 执行共享状态流转。
func (s *Service) setItemVisibility(ctx context.Context, itemID int64, visibility int16, action string) (ItemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemDTO{}, err
	}
	var out Item
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetItemByID(ctx, id.TenantID, itemID)
		if err != nil {
			return mapContentReadError(err)
		}
		if current.AuthorID != id.AccountID {
			return apperr.ErrContentForbidden
		}
		if visibility == VisibilityShared && current.Status != StatusPublished {
			return apperr.ErrContentStateInvalid
		}
		if visibility == VisibilityTenant && current.Visibility != VisibilityShared {
			return apperr.ErrContentStateInvalid
		}
		out, err = tx.SetVisibility(ctx, id.TenantID, itemID, visibility)
		return mapContentMutationError(err)
	}); err != nil {
		return ItemDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, action, contentAuditTargetItem, out.ID, map[string]any{"code": out.Code, "version": out.Version}); err != nil {
		return ItemDTO{}, err
	}
	return itemDTO(out), nil
}

// ListShared 查询跨租户共享库,只返回已发布共享内容摘要。
func (s *Service) ListShared(ctx context.Context, filter ItemListFilter) ([]ItemDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	filter.Visibility = VisibilityShared
	filter.Status = StatusPublished
	filter.OnlyShared = true
	filter.PublishedShared = true
	filter.Page, filter.Size = pagex.Normalize(filter.Page, filter.Size)
	var items []Item
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListItems(ctx, id.TenantID, filter)
		return err
	}); err != nil {
		return nil, 0, 0, 0, apperr.ErrContentQueryInvalid.WithCause(err)
	}
	out := make([]ItemDTO, 0, len(items))
	for _, item := range items {
		out = append(out, itemDTO(item))
	}
	return out, total, filter.Page, filter.Size, nil
}

// GetContentFace 实现跨模块题面读取契约。
func (s *Service) GetContentFace(ctx context.Context, tenantID int64, ref contracts.ContentItemRef) (contracts.ContentItemSnapshot, error) {
	item, err := s.getItemWithBody(ctx, tenantID, ref.ItemCode, ref.ItemVersion)
	if err != nil {
		return contracts.ContentItemSnapshot{}, err
	}
	if item.TenantID != tenantID && item.Visibility != VisibilityShared {
		return contracts.ContentItemSnapshot{}, apperr.ErrContentNotFound
	}
	if item.Status != StatusPublished && item.Status != StatusDeprecated {
		return contracts.ContentItemSnapshot{}, apperr.ErrContentVersionNotPublished
	}
	face, err := faceSnapshot(item)
	if err != nil {
		return contracts.ContentItemSnapshot{}, apperr.ErrContentBodyInvalid.WithCause(err)
	}
	return contractSnapshot(face)
}

// GetContentFull 实现跨模块全量读取契约。
func (s *Service) GetContentFull(ctx context.Context, tenantID int64, ref contracts.ContentItemRef) (contracts.ContentItemSnapshot, error) {
	item, err := s.getItemWithBody(ctx, tenantID, ref.ItemCode, ref.ItemVersion)
	if err != nil {
		return contracts.ContentItemSnapshot{}, err
	}
	if item.TenantID != tenantID {
		return contracts.ContentItemSnapshot{}, apperr.ErrContentFullAccessDenied
	}
	if item.Status != StatusPublished && item.Status != StatusDeprecated {
		return contracts.ContentItemSnapshot{}, apperr.ErrContentVersionNotPublished
	}
	return contractSnapshot(item)
}

// BatchGetContentFace 实现跨模块批量题面读取契约。
func (s *Service) BatchGetContentFace(ctx context.Context, tenantID int64, refs []contracts.ContentItemRef) ([]contracts.ContentItemSnapshot, error) {
	if tenantID <= 0 || len(refs) == 0 || len(refs) > 100 {
		return nil, apperr.ErrContentQueryInvalid
	}
	out := make([]contracts.ContentItemSnapshot, 0, len(refs))
	for _, ref := range refs {
		item, err := s.GetContentFace(ctx, tenantID, ref)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// ReplaceUsageRefs 实现跨模块内容引用集合替换契约。
func (s *Service) ReplaceUsageRefs(ctx context.Context, tenantID int64, sourceScope, sourceRef string, refs []contracts.ContentItemRef) error {
	sourceScope = stringsTrim(sourceScope)
	sourceRef = stringsTrim(sourceRef)
	if tenantID <= 0 || sourceScope == "" || sourceRef == "" || len(sourceScope) > 32 || len(sourceRef) > 128 || len(refs) > 200 {
		return apperr.ErrContentInvalid
	}
	seen := map[string]struct{}{}
	next := make([]UsageRef, 0, len(refs))
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		for _, ref := range refs {
			if !validCode(ref.ItemCode) || !validVersion(ref.ItemVersion) {
				return apperr.ErrContentInvalid
			}
			key := ref.ItemCode + "\x00" + ref.ItemVersion
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			item, err := tx.GetPublishedItemForUsage(ctx, tenantID, ref.ItemCode, ref.ItemVersion)
			if isNoRows(err) {
				return apperr.ErrContentVersionNotPublished
			}
			if err != nil {
				return err
			}
			next = append(next, UsageRef{ID: s.ids.Generate(), TenantID: tenantID, ItemID: item.ID, ItemCode: item.Code, ItemVersion: item.Version, SourceScope: sourceScope, SourceRef: sourceRef})
		}
		return tx.ReplaceUsageRefs(ctx, tenantID, sourceScope, sourceRef, next)
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrContentInvalid.WithCause(err)
	}
	return nil
}

// getItemWithBody 统一读取完整内容并映射错误。
func (s *Service) getItemWithBody(ctx context.Context, tenantID int64, code, version string) (ItemWithBody, error) {
	if !validCode(code) || !validVersion(version) || tenantID <= 0 {
		return ItemWithBody{}, apperr.ErrContentQueryInvalid
	}
	var item ItemWithBody
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.GetItemWithBodyByRef(ctx, tenantID, code, version)
		return err
	}); err != nil {
		return ItemWithBody{}, mapContentReadError(err)
	}
	return item, nil
}

// stringsTrim 避免 service 文件散落 strings 依赖。
func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}
