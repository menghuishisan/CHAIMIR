// 管理内容创建、版本、题面读取与共享状态。
package content

import (
	"context"
	"strings"

	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
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
		return nil, 0, 0, 0, apperr.AsAppError(err)
	}
	out := make([]ItemDTO, 0, len(items))
	for _, item := range items {
		out = append(out, itemDTO(item))
	}
	return out, total, filter.Page, filter.Size, nil
}

// CreateItem 创建教师草稿内容。
func (s *Service) CreateItem(ctx context.Context, actor auditActorSnapshot, req CreateItemRequest) (ItemSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	req, err = validateCreateRequest(req)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	body, err := jsonx.CloneObjectStrict(req.Body)
	if err != nil {
		return ItemSnapshotDTO{}, apperr.ErrContentBodyInvalid.WithCause(err)
	}
	if err := actor.validate(id); err != nil {
		return ItemSnapshotDTO{}, err
	}
	item := ItemWithBody{Item: Item{ID: s.ids.Generate(), TenantID: id.TenantID, Code: req.Code, Version: req.Version, Type: req.Type, Title: req.Title, CategoryID: req.CategoryID.Int64(), Difficulty: req.Difficulty, Tags: req.Tags, KnowledgePoints: req.KnowledgePoints, AuthorID: id.AccountID, AuthorType: AuthorTeacher, Visibility: req.Visibility, Status: StatusDraft}, Body: body, SensitiveFields: req.SensitiveFields}
	item.VersionHash, err = versionHash(item.Item, item.Body, item.SensitiveFields)
	if err != nil {
		return ItemSnapshotDTO{}, apperr.ErrContentBodyInvalid.WithCause(err)
	}
	var out ItemSnapshotDTO
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		created, err := tx.CreateItem(ctx, item)
		if err != nil {
			return err
		}
		out, err = itemSnapshotDTO(created, true)
		if err != nil {
			return apperr.ErrContentBodyInvalid.WithCause(err)
		}
		return s.writeAudit(ctx, id.TenantID, actor.accountID, actor.role, "content.create", contentAuditTargetItem, created.ID, map[string]any{"code": created.Code, "version": created.Version})
	}); err != nil {
		return ItemSnapshotDTO{}, apperr.AsAppError(err)
	}
	return out, nil
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

// GetSharedItemFace 按所选 ID 读取仍已发布且共享的题面。
func (s *Service) GetSharedItemFace(ctx context.Context, itemID int64) (ItemSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	if itemID <= 0 {
		return ItemSnapshotDTO{}, apperr.ErrContentSharedNotFound
	}
	var item ItemWithBody
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.GetVisibleItemWithBodyByID(ctx, id.TenantID, itemID)
		return err
	}); err != nil {
		if isNoRows(err) {
			return ItemSnapshotDTO{}, apperr.ErrContentSharedNotFound
		}
		return ItemSnapshotDTO{}, apperr.AsAppError(err)
	}
	if item.Visibility != VisibilityShared || item.Status != StatusPublished {
		return ItemSnapshotDTO{}, apperr.ErrContentSharedNotFound
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
func (s *Service) UpdateDraftItem(ctx context.Context, actor auditActorSnapshot, itemID int64, req UpdateItemRequest) (ItemSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	req, err = validateUpdateRequest(req)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	if err := actor.validate(id); err != nil {
		return ItemSnapshotDTO{}, err
	}
	var out ItemSnapshotDTO
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
		if current.Type == TypeTheoryQuestion {
			if err := validateTheoryQuestionBody(req.Body); err != nil {
				return err
			}
		}
		current.Title = req.Title
		current.CategoryID = req.CategoryID.Int64()
		current.Difficulty = req.Difficulty
		current.Tags = req.Tags
		current.KnowledgePoints = req.KnowledgePoints
		current.Visibility = req.Visibility
		body, err := jsonx.CloneObjectStrict(req.Body)
		if err != nil {
			return apperr.ErrContentBodyInvalid.WithCause(err)
		}
		current.Body = body
		current.SensitiveFields = req.SensitiveFields
		current.VersionHash, err = versionHash(current.Item, current.Body, current.SensitiveFields)
		if err != nil {
			return apperr.ErrContentBodyInvalid.WithCause(err)
		}
		updated, err := tx.UpdateDraftItem(ctx, current)
		if err != nil {
			return mapContentMutationError(err)
		}
		out, err = itemSnapshotDTO(updated, true)
		if err != nil {
			return apperr.ErrContentBodyInvalid.WithCause(err)
		}
		return s.writeAudit(ctx, id.TenantID, actor.accountID, actor.role, "content.update", contentAuditTargetItem, updated.ID, map[string]any{"code": updated.Code, "version": updated.Version})
	}); err != nil {
		return ItemSnapshotDTO{}, err
	}
	return out, nil
}

// PublishItem 发布草稿内容。
func (s *Service) PublishItem(ctx context.Context, actor auditActorSnapshot, itemID int64) (ItemDTO, error) {
	return s.itemStatusTransition(ctx, actor, itemID, "content.publish", func(ctx context.Context, tx TxStore, tenantID, id int64) (Item, error) {
		item, err := tx.GetItemWithBodyByID(ctx, tenantID, id)
		if err != nil {
			return Item{}, mapContentReadError(err)
		}
		if err := validateContestSubmitKey(item); err != nil {
			return Item{}, err
		}
		if item.Type == TypeTheoryQuestion {
			if err := validateTheoryQuestionBody(item.Body); err != nil {
				return Item{}, err
			}
		}
		return tx.PublishItem(ctx, tenantID, id)
	})
}

// DeprecateItem 弃用已发布内容。
func (s *Service) DeprecateItem(ctx context.Context, actor auditActorSnapshot, itemID int64) (ItemDTO, error) {
	return s.itemStatusTransition(ctx, actor, itemID, "content.deprecate", func(ctx context.Context, tx TxStore, tenantID, id int64) (Item, error) {
		return tx.DeprecateItem(ctx, tenantID, id)
	})
}

// DeleteItem 删除无引用草稿。
func (s *Service) DeleteItem(ctx context.Context, actor auditActorSnapshot, itemID int64) error {
	_, err := s.itemStatusTransition(ctx, actor, itemID, "content.delete", func(ctx context.Context, tx TxStore, tenantID, id int64) (Item, error) {
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
func (s *Service) itemStatusTransition(ctx context.Context, actor auditActorSnapshot, itemID int64, action string, fn func(context.Context, TxStore, int64, int64) (Item, error)) (ItemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemDTO{}, err
	}
	if err := actor.validate(id); err != nil {
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
		if err != nil {
			return mapContentMutationError(err)
		}
		return s.writeAudit(ctx, id.TenantID, actor.accountID, actor.role, action, contentAuditTargetItem, out.ID, map[string]any{"code": out.Code, "version": out.Version})
	}); err != nil {
		return ItemDTO{}, err
	}
	return itemDTO(out), nil
}

// ListVersions 查询本校同 code 的可见版本。
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
		return nil, apperr.AsAppError(err)
	}
	out := make([]ItemDTO, 0, len(items))
	for _, item := range items {
		if item.Visibility == VisibilityPrivate && item.AuthorID != id.AccountID {
			continue
		}
		out = append(out, itemDTO(item))
	}
	return out, nil
}

// CreateNewVersion 从既有版本复制出新草稿。
func (s *Service) CreateNewVersion(ctx context.Context, actor auditActorSnapshot, code string, req NewVersionRequest) (ItemSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	req.SourceVersion = strings.TrimSpace(req.SourceVersion)
	req.NewVersion = strings.TrimSpace(req.NewVersion)
	if !validCode(code) || !validVersion(req.SourceVersion) || !validVersion(req.NewVersion) || req.SourceVersion == req.NewVersion {
		return ItemSnapshotDTO{}, apperr.ErrContentVersionInvalid
	}
	if err := actor.validate(id); err != nil {
		return ItemSnapshotDTO{}, err
	}
	var out ItemSnapshotDTO
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
		created, err := tx.CreateItem(ctx, source)
		if err != nil {
			return err
		}
		out, err = itemSnapshotDTO(created, true)
		if err != nil {
			return apperr.ErrContentBodyInvalid.WithCause(err)
		}
		return s.writeAudit(ctx, id.TenantID, actor.accountID, actor.role, "content.new_version", contentAuditTargetItem, created.ID, map[string]any{"code": created.Code, "version": created.Version, "source_version": req.SourceVersion})
	}); err != nil {
		return ItemSnapshotDTO{}, apperr.AsAppError(err)
	}
	return out, nil
}

// ShareItem 把已发布内容放入共享库。
func (s *Service) ShareItem(ctx context.Context, actor auditActorSnapshot, itemID int64) (ItemDTO, error) {
	return s.setItemVisibility(ctx, actor, itemID, VisibilityShared, "content.share")
}

// UnshareItem 取消后续共享浏览和克隆。
func (s *Service) UnshareItem(ctx context.Context, actor auditActorSnapshot, itemID int64) (ItemDTO, error) {
	return s.setItemVisibility(ctx, actor, itemID, VisibilityTenant, "content.unshare")
}

// setItemVisibility 执行共享状态流转。
func (s *Service) setItemVisibility(ctx context.Context, actor auditActorSnapshot, itemID int64, visibility int16, action string) (ItemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ItemDTO{}, err
	}
	if err := actor.validate(id); err != nil {
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
		if err != nil {
			return mapContentMutationError(err)
		}
		return s.writeAudit(ctx, id.TenantID, actor.accountID, actor.role, action, contentAuditTargetItem, out.ID, map[string]any{"code": out.Code, "version": out.Version})
	}); err != nil {
		return ItemDTO{}, err
	}
	return itemDTO(out), nil
}

// ListShared 查询跨租户共享库,只返回已发布共享内容摘要。
func (s *Service) ListShared(ctx context.Context, filter ItemListFilter) ([]ItemDTO, int64, int, int, pagex.Facets, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, nil, err
	}
	filter.Visibility = VisibilityShared
	filter.Status = StatusPublished
	filter.OnlyShared = true
	filter.Page, filter.Size = pagex.Normalize(filter.Page, filter.Size)
	var items []Item
	var total int64
	var byType pagex.FacetGroup
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListItems(ctx, id.TenantID, filter)
		if err != nil {
			return err
		}
		// 「按题型」分段计数:套用关键词,不套用题型筛选
		byType, err = tx.CountSharedItemsByType(ctx, filter.Keyword)
		return err
	}); err != nil {
		return nil, 0, 0, 0, nil, apperr.AsAppError(err)
	}
	out := make([]ItemDTO, 0, len(items))
	for _, item := range items {
		out = append(out, itemDTO(item))
	}
	return out, total, filter.Page, filter.Size, pagex.Facets{"type": byType}, nil
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
