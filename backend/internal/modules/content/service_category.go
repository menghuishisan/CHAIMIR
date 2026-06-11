// content service_category 文件实现分类树维护。
package content

import (
	"context"

	"chaimir/internal/platform/audit"
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
		return nil, apperr.ErrContentCategoryInvalid.WithCause(err)
	}
	out := make([]CategoryDTO, 0, len(items))
	for _, item := range items {
		out = append(out, categoryDTO(item))
	}
	return out, nil
}

// CreateCategory 创建分类。
func (s *Service) CreateCategory(ctx context.Context, req CategoryRequest) (CategoryDTO, error) {
	return s.saveCategory(ctx, 0, req, true)
}

// UpdateCategory 更新分类。
func (s *Service) UpdateCategory(ctx context.Context, id int64, req CategoryRequest) (CategoryDTO, error) {
	return s.saveCategory(ctx, id, req, false)
}

// DeleteCategory 软删分类。
func (s *Service) DeleteCategory(ctx context.Context, categoryID int64) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	var deleted Category
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		deleted, err = tx.DeleteCategory(ctx, id.TenantID, categoryID)
		return err
	}); err != nil {
		return apperr.ErrContentCategoryInvalid.WithCause(err)
	}
	return s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "content.category.delete", contentAuditTargetCategory, deleted.ID, map[string]any{"name": deleted.Name})
}

// saveCategory 创建或更新分类。
func (s *Service) saveCategory(ctx context.Context, categoryID int64, req CategoryRequest, create bool) (CategoryDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CategoryDTO{}, err
	}
	req, err = validateCategoryRequest(req)
	if err != nil {
		return CategoryDTO{}, err
	}
	category := Category{ID: categoryID, TenantID: id.TenantID, ParentID: req.ParentID, Name: req.Name, Sort: req.Sort}
	if create {
		category.ID = s.ids.Generate()
	}
	var saved Category
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		if create {
			saved, err = tx.CreateCategory(ctx, category)
		} else {
			saved, err = tx.UpdateCategory(ctx, category)
		}
		return err
	}); err != nil {
		return CategoryDTO{}, apperr.ErrContentCategoryInvalid.WithCause(err)
	}
	action := "content.category.update"
	if create {
		action = "content.category.create"
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, action, contentAuditTargetCategory, saved.ID, map[string]any{"name": saved.Name}); err != nil {
		return CategoryDTO{}, err
	}
	return categoryDTO(saved), nil
}
