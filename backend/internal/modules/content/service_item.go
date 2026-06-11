// content service_item 文件实现内容创建、版本、共享、克隆与跨模块读取。
package content

import (
	"context"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

// ListItems 查询教师侧内容分页。
func (s *Service) ListItems(ctx context.Context, filter ItemListFilter) ([]ItemDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	normalizePage(&filter.Page, &filter.Size)
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
	item := ItemWithBody{Item: Item{ID: s.ids.Generate(), TenantID: id.TenantID, Code: req.Code, Version: req.Version, Type: req.Type, Title: req.Title, CategoryID: req.CategoryID, Difficulty: req.Difficulty, Tags: req.Tags, KnowledgePoints: req.KnowledgePoints, AuthorID: id.AccountID, AuthorType: AuthorTeacher, Visibility: req.Visibility, Status: StatusDraft}, Body: cloneMap(req.Body), SensitiveFields: req.SensitiveFields}
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
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "content.create", contentAuditTargetItem, created.ID, map[string]any{"code": created.Code, "version": created.Version}); err != nil {
		return ItemSnapshotDTO{}, err
	}
	return itemSnapshotDTO(created, true), nil
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
	return itemSnapshotDTO(faceSnapshot(item), false), nil
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
	return itemSnapshotDTO(item, true), nil
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
		current.Title = req.Title
		current.CategoryID = req.CategoryID
		current.Difficulty = req.Difficulty
		current.Tags = req.Tags
		current.KnowledgePoints = req.KnowledgePoints
		current.Visibility = req.Visibility
		current.Body = cloneMap(req.Body)
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
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "content.update", contentAuditTargetItem, updated.ID, map[string]any{"code": updated.Code, "version": updated.Version}); err != nil {
		return ItemSnapshotDTO{}, err
	}
	return itemSnapshotDTO(updated, true), nil
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
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, action, contentAuditTargetItem, out.ID, map[string]any{"code": out.Code, "version": out.Version}); err != nil {
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
		source.Version = req.NewVersion
		source.Status = StatusDraft
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
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "content.new_version", contentAuditTargetItem, created.ID, map[string]any{"code": created.Code, "version": created.Version, "source_version": req.SourceVersion}); err != nil {
		return ItemSnapshotDTO{}, err
	}
	return itemSnapshotDTO(created, true), nil
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
	var created ItemWithBody
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		source, err := tx.GetItemWithBodyByRef(ctx, id.TenantID, code, version)
		if err != nil {
			return apperr.ErrContentSharedNotFound.WithCause(err)
		}
		if source.TenantID != id.TenantID && source.Visibility != VisibilityShared {
			return apperr.ErrContentSharedNotFound
		}
		if source.Status != StatusPublished {
			return apperr.ErrContentCloneInvalid
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
		source.VersionHash, err = versionHash(source.Item, source.Body, source.SensitiveFields)
		if err != nil {
			return apperr.ErrContentBodyInvalid.WithCause(err)
		}
		created, err = tx.CreateItem(ctx, source)
		return err
	}); err != nil {
		return ItemSnapshotDTO{}, apperr.ErrContentCloneInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "content.clone", contentAuditTargetItem, created.ID, map[string]any{"source_code": code, "source_version": version, "code": created.Code}); err != nil {
		return ItemSnapshotDTO{}, err
	}
	return itemSnapshotDTO(created, true), nil
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
		out, err = tx.SetVisibility(ctx, id.TenantID, itemID, visibility)
		return mapContentMutationError(err)
	}); err != nil {
		return ItemDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, action, contentAuditTargetItem, out.ID, map[string]any{"code": out.Code, "version": out.Version}); err != nil {
		return ItemDTO{}, err
	}
	return itemDTO(out), nil
}

// ListShared 查询跨租户共享库,只返回已发布共享内容摘要。
func (s *Service) ListShared(ctx context.Context, filter ItemListFilter) ([]ItemDTO, int64, int, int, error) {
	filter.Visibility = VisibilityShared
	filter.Status = StatusPublished
	filter.PublishedShared = true
	return s.ListItems(ctx, filter)
}

// GetContentFace 实现跨模块题面读取契约。
func (s *Service) GetContentFace(ctx context.Context, tenantID int64, ref contracts.ContentItemRef) (contracts.ContentItemSnapshot, error) {
	item, err := s.getItemWithBody(ctx, tenantID, ref.ItemCode, ref.ItemVersion)
	if err != nil {
		return contracts.ContentItemSnapshot{}, err
	}
	if item.Status != StatusPublished && item.Status != StatusDeprecated {
		return contracts.ContentItemSnapshot{}, apperr.ErrContentVersionNotPublished
	}
	return contractSnapshot(faceSnapshot(item)), nil
}

// GetContentFull 实现跨模块全量读取契约。
func (s *Service) GetContentFull(ctx context.Context, tenantID int64, ref contracts.ContentItemRef) (contracts.ContentItemSnapshot, error) {
	item, err := s.getItemWithBody(ctx, tenantID, ref.ItemCode, ref.ItemVersion)
	if err != nil {
		return contracts.ContentItemSnapshot{}, err
	}
	if item.Status != StatusPublished && item.Status != StatusDeprecated {
		return contracts.ContentItemSnapshot{}, apperr.ErrContentVersionNotPublished
	}
	return contractSnapshot(item), nil
}

// BatchGetContentFace 实现跨模块批量题面读取契约。
func (s *Service) BatchGetContentFace(ctx context.Context, tenantID int64, refs []contracts.ContentItemRef) ([]contracts.ContentItemSnapshot, error) {
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

// IncrementUsage 实现跨模块引用计数契约。
func (s *Service) IncrementUsage(ctx context.Context, tenantID int64, ref contracts.ContentItemRef) error {
	if !validCode(ref.ItemCode) || !validVersion(ref.ItemVersion) || tenantID <= 0 {
		return apperr.ErrContentInvalid
	}
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		_, err := tx.IncrementUsage(ctx, tenantID, ref.ItemCode, ref.ItemVersion)
		if isNoRows(err) {
			return apperr.ErrContentVersionNotPublished
		}
		return err
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
