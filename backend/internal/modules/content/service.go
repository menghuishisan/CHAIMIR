// M5 服务层:承载内容版本、答案隔离、共享克隆、分类、组卷与内部取用。
package content

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/content/internal/sqlcgen"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

// Service 是 M5 题库与模板中心服务。
type Service struct {
	repo     *repo
	idgen    *snowflake.Node
	auditor  audit.Writer
	identity contracts.IdentityService
}

// NewService 构造 M5 服务。
func NewService(database *db.DB, idgen *snowflake.Node, auditor audit.Writer, identity contracts.IdentityService) *Service {
	return &Service{repo: newRepo(database), idgen: idgen, auditor: auditor, identity: identity}
}

// CreateItem 创建内容草稿或系统导入内容。
func (s *Service) CreateItem(ctx context.Context, req CreateItemRequest) (ItemDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ItemDTO{}, apperr.ErrUnauthorized
	}
	if req.Version == "" {
		req.Version = initialVersion
	}
	if req.AuthorID == "" {
		req.AuthorID = ids.Format(id.AccountID)
	}
	if req.AuthorType == 0 {
		req.AuthorType = AuthorTypeTeacher
	}
	if req.Visibility == 0 {
		req.Visibility = VisibilityPrivate
	}
	if err := validateCreateItemRequest(req); err != nil {
		return ItemDTO{}, err
	}
	authorID, ok := ids.Parse(req.AuthorID)
	if !ok {
		return ItemDTO{}, apperr.ErrContentInvalid
	}
	body, err := jsonx.ObjectBytes(req.Body, apperr.ErrContentInvalid)
	if err != nil {
		return ItemDTO{}, err
	}
	status := ItemStatusDraft
	if req.AutoPublish {
		status = ItemStatusPublished
	}
	itemID := s.idgen.Generate()
	var row sqlcgen.ContentItem
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		created, e := q.CreateContentItem(ctx, sqlcgen.CreateContentItemParams{
			ID: itemID, TenantID: id.TenantID, Code: req.Code, Version: req.Version, Type: req.Type, Title: req.Title,
			CategoryID: pgInt8(mustOptionalID(req.CategoryID)), Difficulty: req.Difficulty, Tags: req.Tags,
			KnowledgePoints: req.KnowledgePoints, AuthorID: authorID, AuthorType: req.AuthorType,
			Visibility: req.Visibility, Status: status, BodyHash: bodyHash(body),
		})
		if e != nil {
			return e
		}
		row = created
		_, e = q.CreateContentBody(ctx, sqlcgen.CreateContentBodyParams{ItemID: itemID, TenantID: id.TenantID, Body: body, SensitiveFields: req.SensitiveFields})
		return e
	}); err != nil {
		return ItemDTO{}, apperr.ErrContentCodeConflict.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionItemCreate, auditTargetItem, itemID, createItemAuditDetail(req)); err != nil {
		return ItemDTO{}, err
	}
	return itemDTOFromShell(row), nil
}

// ListItems 检索本租户内容外壳。
func (s *Service) ListItems(ctx context.Context, req ListItemsRequest) ([]ItemDTO, int64, error) {
	page, size := pagex.Normalize(req.Page, req.Size)
	params := listParams(req, size, (page-1)*size)
	var rows []sqlcgen.ContentItem
	var total int64
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListContentItems(ctx, params)
		if err != nil {
			return err
		}
		rows = found
		total, err = q.CountContentItems(ctx, countParams(req))
		return err
	}); err != nil {
		return nil, 0, apperr.ErrContentQueryFailed.WithCause(err)
	}
	return itemsDTOFromShell(rows), total, nil
}

// GetFace 读取题面内容并过滤敏感字段。
func (s *Service) GetFace(ctx context.Context, code, version string) (ItemDTO, error) {
	item, err := s.getOwnContent(ctx, code, version, true)
	if err != nil {
		return ItemDTO{}, err
	}
	if item.Status != ItemStatusPublished {
		return ItemDTO{}, apperr.ErrContentUnavailable
	}
	if err := s.ensureCanReadFace(ctx, item); err != nil {
		return ItemDTO{}, err
	}
	return item, nil
}

// GetFull 读取本租户全量内容,供教师与内部调用使用。
func (s *Service) GetFull(ctx context.Context, code, version string) (ItemDTO, error) {
	item, err := s.getOwnContent(ctx, code, version, false)
	if err != nil {
		return ItemDTO{}, err
	}
	if err := s.ensureCanManage(ctx, mustID(item.AuthorID)); err != nil {
		return ItemDTO{}, err
	}
	return item, nil
}

// UpdateDraft 更新草稿内容。
func (s *Service) UpdateDraft(ctx context.Context, itemID int64, req UpdateItemRequest) (ItemDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ItemDTO{}, apperr.ErrUnauthorized
	}
	if err := validateUpdateItemRequest(req); err != nil {
		return ItemDTO{}, err
	}
	body, err := jsonx.ObjectBytes(req.Body, apperr.ErrContentInvalid)
	if err != nil {
		return ItemDTO{}, err
	}
	var row sqlcgen.ContentItem
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		current, e := q.GetContentItemByID(ctx, itemID)
		if db.IsNoRows(e) {
			return apperr.ErrContentNotFound
		}
		if e != nil {
			return e
		}
		if e = s.ensureCanManage(ctx, current.AuthorID); e != nil {
			return e
		}
		if e = validateDraftEditable(current.Status); e != nil {
			return e
		}
		row, e = q.UpdateContentDraft(ctx, sqlcgen.UpdateContentDraftParams{
			ID: itemID, Title: req.Title, CategoryID: pgInt8(mustOptionalID(req.CategoryID)), Difficulty: req.Difficulty,
			Tags: req.Tags, KnowledgePoints: req.KnowledgePoints, Visibility: req.Visibility, BodyHash: bodyHash(body),
		})
		if e != nil {
			return e
		}
		_, e = q.UpdateContentBody(ctx, sqlcgen.UpdateContentBodyParams{ItemID: itemID, Body: body, SensitiveFields: req.SensitiveFields})
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ItemDTO{}, ae
		}
		return ItemDTO{}, apperr.ErrContentInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionItemUpdate, auditTargetItem, itemID, map[string]any{"id": ids.Format(itemID)}); err != nil {
		return ItemDTO{}, err
	}
	return itemDTOFromShell(row), nil
}

// Publish 发布草稿内容并冻结版本。
func (s *Service) Publish(ctx context.Context, itemID int64) (ItemDTO, error) {
	return s.updateStatus(ctx, itemID, ItemStatusPublished, auditActionItemPublish)
}

// Deprecate 弃用已发布内容,旧引用仍可读取。
func (s *Service) Deprecate(ctx context.Context, itemID int64) (ItemDTO, error) {
	return s.updateStatus(ctx, itemID, ItemStatusDeprecated, auditActionItemDeprecate)
}

// DeleteDraft 软删无引用草稿。
func (s *Service) DeleteDraft(ctx context.Context, itemID int64) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		current, err := q.GetContentItemByID(ctx, itemID)
		if db.IsNoRows(err) {
			return apperr.ErrContentNotFound
		}
		if err != nil {
			return err
		}
		if err = s.ensureCanManage(ctx, current.AuthorID); err != nil {
			return err
		}
		if err = validateDraftDeletable(current.Status, current.UsageCount); err != nil {
			return err
		}
		_, err = q.SoftDeleteContentItem(ctx, itemID)
		if db.IsNoRows(err) {
			return apperr.ErrContentDeleteBlocked
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrContentDeleteBlocked.WithCause(err)
	}
	return s.writeAudit(ctx, id.TenantID, auditActionItemDelete, auditTargetItem, itemID, map[string]any{"id": ids.Format(itemID)})
}

// ListVersions 查询某内容 code 下全部版本。
func (s *Service) ListVersions(ctx context.Context, code string) ([]ItemDTO, error) {
	var rows []sqlcgen.ContentItem
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListContentVersions(ctx, code)
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrContentVersionQueryFailed.WithCause(err)
	}
	return itemsDTOFromShell(rows), nil
}

// NewVersion 基于指定源版本或当前最高正式版本创建独立草稿。
func (s *Service) NewVersion(ctx context.Context, code string, req NewVersionRequest) (ItemDTO, error) {
	versions, err := s.ListVersions(ctx, code)
	if err != nil {
		return ItemDTO{}, err
	}
	sourceVersion, next, err := resolveNewVersionPlan(versions, req)
	if err != nil {
		return ItemDTO{}, err
	}
	latest, err := s.GetFull(ctx, code, sourceVersion)
	if err != nil {
		return ItemDTO{}, err
	}
	return s.CreateItem(ctx, CreateItemRequest{
		Code: code, Version: next, Type: latest.Type, Title: latest.Title, CategoryID: latest.CategoryID,
		Difficulty: latest.Difficulty, Tags: latest.Tags, KnowledgePoints: latest.KnowledgePoints,
		Visibility: VisibilityPrivate, Body: latest.Body, SensitiveFields: latest.SensitiveFields,
	})
}

// Clone 克隆自己的或共享库内容为本租户独立草稿。
func (s *Service) Clone(ctx context.Context, code, version, newCode string) (ItemDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ItemDTO{}, apperr.ErrUnauthorized
	}
	source, err := s.getOwnContent(ctx, code, version, false)
	if err != nil {
		source, err = s.getSharedContent(ctx, code, version, false)
	} else if err = s.ensureCanManage(ctx, mustID(source.AuthorID)); err != nil {
		return ItemDTO{}, err
	}
	if err != nil {
		return ItemDTO{}, err
	}
	draft, err := buildCloneDraft(source, id.AccountID, newCode)
	if err != nil {
		return ItemDTO{}, err
	}
	out, err := s.CreateItem(ctx, CreateItemRequest{
		Code: draft.Code, Version: draft.Version, Type: draft.Type, Title: draft.Title, CategoryID: draft.CategoryID,
		Difficulty: draft.Difficulty, Tags: draft.Tags, KnowledgePoints: draft.KnowledgePoints, Visibility: draft.Visibility,
		Body: draft.Body, SensitiveFields: draft.SensitiveFields,
	})
	if err != nil {
		return ItemDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionItemClone, auditTargetItem, mustID(out.ID), map[string]any{"source": code + ":" + version}); err != nil {
		return ItemDTO{}, err
	}
	return out, nil
}

// Share 把已发布内容加入共享库。
func (s *Service) Share(ctx context.Context, itemID int64) (ItemDTO, error) {
	return s.updateVisibility(ctx, itemID, VisibilityShared, auditActionItemShare)
}

// Unshare 取消共享,不影响已克隆副本。
func (s *Service) Unshare(ctx context.Context, itemID int64) (ItemDTO, error) {
	return s.updateVisibility(ctx, itemID, VisibilityTenant, auditActionItemUnshare)
}

// ListShared 浏览跨校共享库题面摘要。
func (s *Service) ListShared(ctx context.Context, typ int16, keyword string, page, size int) ([]ItemDTO, error) {
	if !s.repo.hasPrivileged() {
		return nil, apperr.ErrContentShareInvalid
	}
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.ContentItem
	if err := s.repo.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListSharedContentItems(ctx, sqlcgen.ListSharedContentItemsParams{
			Limit: int32(size), Offset: int32((page - 1) * size), Type: pgInt2(typ), Keyword: pgText(keyword),
		})
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrContentShareInvalid.WithCause(err)
	}
	return itemsDTOFromShell(rows), nil
}

// CreateCategory 创建内容分类。
func (s *Service) CreateCategory(ctx context.Context, req CategoryRequest) (CategoryDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return CategoryDTO{}, apperr.ErrUnauthorized
	}
	if req.Name == "" {
		return CategoryDTO{}, apperr.ErrContentInvalid
	}
	parentID := mustOptionalID(req.ParentID)
	categories, err := s.ListCategories(ctx)
	if err != nil {
		return CategoryDTO{}, err
	}
	if err := validateCategoryParent(0, parentID, categories); err != nil {
		return CategoryDTO{}, err
	}
	rowID := s.idgen.Generate()
	var row sqlcgen.ContentCategory
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		created, err := q.CreateContentCategory(ctx, sqlcgen.CreateContentCategoryParams{
			ID: rowID, TenantID: id.TenantID, ParentID: pgInt8(parentID), Name: req.Name, Sort: req.Sort,
		})
		row = created
		return err
	}); err != nil {
		return CategoryDTO{}, apperr.ErrContentInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionCategorySave, auditTargetCategory, rowID, map[string]any{"name": req.Name}); err != nil {
		return CategoryDTO{}, err
	}
	return categoryDTOFromRow(row), nil
}

// ListCategories 查询分类树。
func (s *Service) ListCategories(ctx context.Context) ([]CategoryDTO, error) {
	var rows []sqlcgen.ContentCategory
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListContentCategories(ctx)
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrContentCategoryQueryFailed.WithCause(err)
	}
	return categoriesDTOFromRows(rows), nil
}

// UpdateCategory 更新分类节点。
func (s *Service) UpdateCategory(ctx context.Context, categoryID int64, req CategoryRequest) (CategoryDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return CategoryDTO{}, apperr.ErrUnauthorized
	}
	if req.Name == "" {
		return CategoryDTO{}, apperr.ErrContentInvalid
	}
	parentID := mustOptionalID(req.ParentID)
	categories, err := s.ListCategories(ctx)
	if err != nil {
		return CategoryDTO{}, err
	}
	if err := validateCategoryParent(categoryID, parentID, categories); err != nil {
		return CategoryDTO{}, err
	}
	var row sqlcgen.ContentCategory
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		updated, err := q.UpdateContentCategory(ctx, sqlcgen.UpdateContentCategoryParams{
			ID: categoryID, ParentID: pgInt8(parentID), Name: req.Name, Sort: req.Sort,
		})
		if db.IsNoRows(err) {
			return apperr.ErrContentNotFound
		}
		row = updated
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return CategoryDTO{}, ae
		}
		return CategoryDTO{}, apperr.ErrContentInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionCategorySave, auditTargetCategory, categoryID, map[string]any{"name": req.Name}); err != nil {
		return CategoryDTO{}, err
	}
	return categoryDTOFromRow(row), nil
}

// DeleteCategory 软删分类节点。
func (s *Service) DeleteCategory(ctx context.Context, categoryID int64) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		_, err := q.DeleteContentCategory(ctx, categoryID)
		if db.IsNoRows(err) {
			return apperr.ErrContentNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrContentInvalid.WithCause(err)
	}
	return s.writeAudit(ctx, id.TenantID, auditActionCategorySave, auditTargetCategory, categoryID, map[string]any{"deleted": true})
}

// CreatePaper 创建试卷并锁定题目版本。
func (s *Service) CreatePaper(ctx context.Context, req PaperRequest) (PaperDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return PaperDTO{}, apperr.ErrUnauthorized
	}
	if err := validatePaperRequest(req); err != nil {
		return PaperDTO{}, err
	}
	criteria, err := jsonx.ObjectBytes(req.GenCriteria, apperr.ErrPaperInvalid)
	if err != nil {
		return PaperDTO{}, err
	}
	paperID := s.idgen.Generate()
	var row sqlcgen.Paper
	var items []PaperItemDTO
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		created, e := q.CreatePaper(ctx, sqlcgen.CreatePaperParams{
			ID: paperID, TenantID: id.TenantID, Name: req.Name, AuthorID: id.AccountID, GenMode: req.GenMode, GenCriteria: criteria,
		})
		if e != nil {
			return e
		}
		row = created
		items, e = s.createPaperItems(ctx, q, id.TenantID, paperID, req)
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return PaperDTO{}, ae
		}
		return PaperDTO{}, apperr.ErrPaperInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionPaperSave, auditTargetPaper, paperID, map[string]any{"name": req.Name}); err != nil {
		return PaperDTO{}, err
	}
	return paperDTOFromRow(row, items), nil
}

// ListPapers 查询试卷列表。
func (s *Service) ListPapers(ctx context.Context, page, size int) ([]PaperDTO, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.Paper
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListPapers(ctx, sqlcgen.ListPapersParams{Limit: int32(size), Offset: int32((page - 1) * size)})
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrPaperQueryFailed.WithCause(err)
	}
	out := make([]PaperDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, paperDTOFromRow(row, nil))
	}
	return out, nil
}

// GetPaper 查询试卷详情,并以题面视角展开锁定版本题目。
func (s *Service) GetPaper(ctx context.Context, paperID int64) (PaperDTO, error) {
	var row sqlcgen.Paper
	var itemRows []sqlcgen.PaperItem
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.GetPaperByID(ctx, paperID)
		if db.IsNoRows(err) {
			return apperr.ErrPaperNotFound
		}
		if err != nil {
			return err
		}
		row = found
		itemRows, err = q.ListPaperItems(ctx, paperID)
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return PaperDTO{}, ae
		}
		return PaperDTO{}, apperr.ErrPaperQueryFailed.WithCause(err)
	}
	items, err := s.expandPaperItems(ctx, itemRows)
	if err != nil {
		return PaperDTO{}, err
	}
	return paperDTOFromRow(row, items), nil
}

// RegeneratePaper 按原试卷或新请求重新抽题。
func (s *Service) RegeneratePaper(ctx context.Context, paperID int64, req PaperRequest) (PaperDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return PaperDTO{}, apperr.ErrUnauthorized
	}
	if req.GenMode == 0 {
		req.GenMode = PaperGenRandom
	}
	if err := validatePaperRequest(req); err != nil {
		return PaperDTO{}, err
	}
	var row sqlcgen.Paper
	var items []PaperItemDTO
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		found, e := q.GetPaperByID(ctx, paperID)
		if db.IsNoRows(e) {
			return apperr.ErrPaperNotFound
		}
		if e != nil {
			return e
		}
		row = found
		if e = q.DeletePaperItems(ctx, paperID); e != nil {
			return e
		}
		items, e = s.createPaperItems(ctx, q, id.TenantID, paperID, req)
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return PaperDTO{}, ae
		}
		return PaperDTO{}, apperr.ErrPaperInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionPaperSave, auditTargetPaper, paperID, map[string]any{"regenerated": true}); err != nil {
		return PaperDTO{}, err
	}
	return paperDTOFromRow(row, items), nil
}

// IncrementUsage 记录内容被引用。
func (s *Service) IncrementUsage(ctx context.Context, tenantID int64, code, version string) error {
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.IncrementContentUsage(ctx, sqlcgen.IncrementContentUsageParams{Code: code, Version: version})
		if db.IsNoRows(err) {
			return apperr.ErrContentUnavailable
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrContentUnavailable.WithCause(err)
	}
	if err := s.writeAudit(ctx, tenantID, auditActionItemUsage, auditTargetItem, 0, map[string]any{"code": code, "version": version}); err != nil {
		return err
	}
	return nil
}

// getOwnContent 读取本租户内容。
func (s *Service) getOwnContent(ctx context.Context, code, version string, face bool) (ItemDTO, error) {
	var row sqlcgen.GetContentByCodeVersionRow
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.GetContentByCodeVersion(ctx, sqlcgen.GetContentByCodeVersionParams{Code: code, Version: version})
		row = found
		return err
	}); err != nil {
		return ItemDTO{}, apperr.ErrContentNotFound.WithCause(err)
	}
	return itemDTOFromRow(contentRowFromOwn(row), face)
}

// getSharedContent 读取共享库内容,仅用于浏览题面或克隆源。
func (s *Service) getSharedContent(ctx context.Context, code, version string, face bool) (ItemDTO, error) {
	if !s.repo.hasPrivileged() {
		return ItemDTO{}, apperr.ErrContentShareInvalid
	}
	var row sqlcgen.GetSharedContentByCodeVersionRow
	if err := s.repo.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.GetSharedContentByCodeVersion(ctx, sqlcgen.GetSharedContentByCodeVersionParams{Code: code, Version: version})
		row = found
		return err
	}); err != nil {
		return ItemDTO{}, apperr.ErrContentShareInvalid.WithCause(err)
	}
	return itemDTOFromRow(contentRowFromShared(row), face)
}

// updateStatus 更新内容状态。
func (s *Service) updateStatus(ctx context.Context, itemID int64, status int16, action string) (ItemDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ItemDTO{}, apperr.ErrUnauthorized
	}
	var row sqlcgen.ContentItem
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		current, err := q.GetContentItemByID(ctx, itemID)
		if db.IsNoRows(err) {
			return apperr.ErrContentNotFound
		}
		if err != nil {
			return err
		}
		if err = s.ensureCanManage(ctx, current.AuthorID); err != nil {
			return err
		}
		updated, err := q.UpdateContentStatus(ctx, sqlcgen.UpdateContentStatusParams{ID: itemID, Status: status})
		if db.IsNoRows(err) {
			return apperr.ErrContentNotFound
		}
		row = updated
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ItemDTO{}, ae
		}
		return ItemDTO{}, apperr.ErrContentInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, action, auditTargetItem, itemID, map[string]any{"status": status}); err != nil {
		return ItemDTO{}, err
	}
	return itemDTOFromShell(row), nil
}

// updateVisibility 更新共享可见性。
func (s *Service) updateVisibility(ctx context.Context, itemID int64, visibility int16, action string) (ItemDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ItemDTO{}, apperr.ErrUnauthorized
	}
	var row sqlcgen.ContentItem
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		current, err := q.GetContentItemByID(ctx, itemID)
		if db.IsNoRows(err) {
			return apperr.ErrContentNotFound
		}
		if err != nil {
			return err
		}
		if err = s.ensureCanManage(ctx, current.AuthorID); err != nil {
			return err
		}
		updated, err := q.UpdateContentVisibility(ctx, sqlcgen.UpdateContentVisibilityParams{ID: itemID, Visibility: visibility})
		if db.IsNoRows(err) {
			return apperr.ErrContentUnavailable
		}
		row = updated
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ItemDTO{}, ae
		}
		return ItemDTO{}, apperr.ErrContentUnavailable.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, action, auditTargetItem, itemID, map[string]any{"visibility": visibility}); err != nil {
		return ItemDTO{}, err
	}
	return itemDTOFromShell(row), nil
}

// createPaperItems 创建试卷题目,手动模式锁定请求版本,随机模式锁定抽中的当前版本。
func (s *Service) createPaperItems(ctx context.Context, q *sqlcgen.Queries, tenantID, paperID int64, req PaperRequest) ([]PaperItemDTO, error) {
	items := req.Items
	if req.GenMode == PaperGenRandom {
		selected, err := q.ListPublishedItemsForRandomPaper(ctx, randomParams(req.GenCriteria))
		if err != nil {
			return nil, err
		}
		criteria := normalizeRandomCriteria(req.GenCriteria)
		want := criteria.Count
		if len(selected) < want {
			return nil, apperr.ErrPaperRandomNotEnough
		}
		items = make([]PaperItemReq, 0, len(selected))
		score := criteria.Score
		if score <= 0 {
			score = 1
		}
		for idx, item := range selected {
			items = append(items, PaperItemReq{Code: item.Code, Version: item.Version, Score: score, Seq: int32(idx + 1)})
		}
	}
	out := make([]PaperItemDTO, 0, len(items))
	for idx, item := range items {
		if req.GenMode == PaperGenManual {
			current, err := q.GetContentByCodeVersion(ctx, sqlcgen.GetContentByCodeVersionParams{Code: item.Code, Version: item.Version})
			if db.IsNoRows(err) {
				return nil, apperr.ErrContentNotFound
			}
			if err != nil {
				return nil, err
			}
			if err := validatePaperItemReference(current.Status); err != nil {
				return nil, err
			}
		}
		seq := item.Seq
		if seq <= 0 {
			seq = int32(idx + 1)
		}
		row, err := q.CreatePaperItem(ctx, sqlcgen.CreatePaperItemParams{
			ID: s.idgen.Generate(), TenantID: tenantID, PaperID: paperID, ItemCode: item.Code,
			ItemVersion: item.Version, Score: item.Score, Seq: seq,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, paperItemDTOFromRow(row, ItemDTO{}))
	}
	return out, nil
}

// expandPaperItems 展开试卷题目题面。
func (s *Service) expandPaperItems(ctx context.Context, rows []sqlcgen.PaperItem) ([]PaperItemDTO, error) {
	out := make([]PaperItemDTO, 0, len(rows))
	for _, row := range rows {
		item, err := s.GetFace(ctx, row.ItemCode, row.ItemVersion)
		if err != nil {
			return nil, err
		}
		out = append(out, paperItemDTOFromRow(row, item))
	}
	return out, nil
}

// ensureCanManage 校验当前账号可管理指定作者的内容。
func (s *Service) ensureCanManage(ctx context.Context, authorID int64) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return nil
	}
	if s.identity == nil {
		return apperr.ErrForbidden
	}
	account, err := s.identity.GetAccount(ctx, id.AccountID)
	if err != nil {
		return apperr.ErrForbidden.WithCause(err)
	}
	if !canManageContent(id.IsPlatform, account, authorID) {
		return apperr.ErrContentForbidden
	}
	return nil
}

// ensureCanReadFace 校验教师直连题库题面时符合 private/tenant 可见性边界。
func (s *Service) ensureCanReadFace(ctx context.Context, item ItemDTO) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return nil
	}
	if s.identity == nil {
		return apperr.ErrForbidden
	}
	account, err := s.identity.GetAccount(ctx, id.AccountID)
	if err != nil {
		return apperr.ErrForbidden.WithCause(err)
	}
	if !canReadOwnContentFace(id.IsPlatform, account, mustID(item.AuthorID), item.Visibility) {
		return apperr.ErrContentForbidden
	}
	return nil
}

// validatePaperRequest 校验组卷请求。
func validatePaperRequest(req PaperRequest) error {
	if req.Name == "" || (req.GenMode != PaperGenManual && req.GenMode != PaperGenRandom) {
		return apperr.ErrPaperInvalid
	}
	if req.GenMode == PaperGenManual && len(req.Items) == 0 {
		return apperr.ErrPaperInvalid
	}
	if req.GenMode == PaperGenManual {
		for _, item := range req.Items {
			if item.Code == "" || item.Version == "" || item.Score <= 0 {
				return apperr.ErrPaperInvalid
			}
		}
	}
	if req.GenMode == PaperGenRandom && numberValue(req.GenCriteria["count"]) <= 0 {
		return apperr.ErrPaperInvalid
	}
	return nil
}

// validatePaperItemReference 确认新组卷引用只能锁定已发布内容版本。
func validatePaperItemReference(status int16) error {
	if status != ItemStatusPublished {
		return apperr.ErrContentUnavailable
	}
	return nil
}

// listParams 构造内容列表查询参数。
func listParams(req ListItemsRequest, size, offset int) sqlcgen.ListContentItemsParams {
	return sqlcgen.ListContentItemsParams{
		Limit: int32(size), Offset: int32(offset), Type: pgInt2(req.Type), CategoryID: pgInt8(mustOptionalID(req.CategoryID)),
		Difficulty: pgInt2(req.Difficulty), Visibility: pgInt2(req.Visibility), Status: pgInt2(req.Status),
		Tag: pgText(req.Tag), Kp: pgText(req.KP), Keyword: pgText(req.Keyword),
	}
}

// countParams 构造内容计数查询参数。
func countParams(req ListItemsRequest) sqlcgen.CountContentItemsParams {
	return sqlcgen.CountContentItemsParams{
		Type: pgInt2(req.Type), CategoryID: pgInt8(mustOptionalID(req.CategoryID)), Difficulty: pgInt2(req.Difficulty),
		Visibility: pgInt2(req.Visibility), Status: pgInt2(req.Status), Tag: pgText(req.Tag), Kp: pgText(req.KP), Keyword: pgText(req.Keyword),
	}
}

// randomParams 构造随机组卷查询参数。
func randomParams(criteria map[string]any) sqlcgen.ListPublishedItemsForRandomPaperParams {
	normalized := normalizeRandomCriteria(criteria)
	return sqlcgen.ListPublishedItemsForRandomPaperParams{
		Limit: int32(normalized.Count), Type: pgInt2(normalized.Type),
		Difficulties: normalized.Difficulties, KnowledgePoints: normalized.KnowledgePoints,
	}
}

// mustOptionalID 解析可选 ID,空值返回 0。
func mustOptionalID(v string) int64 {
	id, _ := ids.Parse(v)
	return id
}

// mustID 解析已知由本服务输出的 ID。
func mustID(v string) int64 {
	id, _ := ids.Parse(v)
	return id
}
