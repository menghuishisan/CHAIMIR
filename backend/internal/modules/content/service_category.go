// content service_category 文件实现分类树维护。
package content

import (
	"context"

	"chaimir/pkg/apperr"
)

// ListCategories 查询当前租户分类树。
func (s *Service) ListCategories(ctx context.Context) ([]CategoryDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var items []Category
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListCategories(ctx, id.TenantID)
		return err
	}); err != nil {
		return nil, apperr.AsAppError(err)
	}
	out := make([]CategoryDTO, 0, len(items))
	for _, item := range items {
		out = append(out, categoryDTO(item))
	}
	return out, nil
}

// CreateCategory 创建分类。
func (s *Service) CreateCategory(ctx context.Context, actor auditActorSnapshot, req CategoryRequest) (CategoryDTO, error) {
	return s.saveCategory(ctx, actor, 0, req, true)
}

// UpdateCategory 更新分类。
func (s *Service) UpdateCategory(ctx context.Context, actor auditActorSnapshot, id int64, req CategoryRequest) (CategoryDTO, error) {
	return s.saveCategory(ctx, actor, id, req, false)
}

// DeleteCategory 软删分类。
func (s *Service) DeleteCategory(ctx context.Context, actor auditActorSnapshot, categoryID int64) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	if err := actor.validate(id); err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		deleted, err := tx.DeleteCategory(ctx, id.TenantID, categoryID)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, actor.accountID, actor.role, "content.category.delete", contentAuditTargetCategory, deleted.ID, map[string]any{"name": deleted.Name})
	}); err != nil {
		return apperr.AsAppError(err)
	}
	return nil
}

// saveCategory 创建或更新分类。
func (s *Service) saveCategory(ctx context.Context, actor auditActorSnapshot, categoryID int64, req CategoryRequest, create bool) (CategoryDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CategoryDTO{}, err
	}
	req, err = validateCategoryRequest(req)
	if err != nil {
		return CategoryDTO{}, err
	}
	if err := actor.validate(id); err != nil {
		return CategoryDTO{}, err
	}
	category := Category{ID: categoryID, TenantID: id.TenantID, ParentID: req.ParentID.Int64(), Name: req.Name, Sort: req.Sort}
	if create {
		category.ID = s.ids.Generate()
	}
	action := "content.category.update"
	if create {
		action = "content.category.create"
	}
	var out CategoryDTO
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var saved Category
		var err error
		if create {
			saved, err = tx.CreateCategory(ctx, category)
		} else {
			saved, err = tx.UpdateCategory(ctx, category)
		}
		if err != nil {
			return err
		}
		out = categoryDTO(saved)
		return s.writeAudit(ctx, id.TenantID, actor.accountID, actor.role, action, contentAuditTargetCategory, saved.ID, map[string]any{"name": saved.Name})
	}); err != nil {
		return CategoryDTO{}, apperr.AsAppError(err)
	}
	return out, nil
}
