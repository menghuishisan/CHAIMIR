// M5 服务层:承载内容版本、答案隔离、共享克隆、分类、组卷与内部取用。
package content

import (
	"context"

	"chaimir/internal/contracts"
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

// CreateItem 创建教师手动维护的内容草稿。
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
	return s.createItemValidated(ctx, id.TenantID, req)
}

// SystemImportItem 固化系统/外部源内容,仅允许内部调用标记为系统或外部来源。
func (s *Service) SystemImportItem(ctx context.Context, req CreateItemRequest) (ItemDTO, error) {
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
		req.AuthorType = AuthorTypeSystem
	}
	if req.Visibility == 0 {
		req.Visibility = VisibilityPrivate
	}
	if err := validateSystemImportRequest(req); err != nil {
		return ItemDTO{}, err
	}
	return s.createItemValidated(ctx, id.TenantID, req)
}

// createItemValidated 写入已完成来源与字段校验的内容外壳和内容体。
func (s *Service) createItemValidated(ctx context.Context, tenantID int64, req CreateItemRequest) (ItemDTO, error) {
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
	out, err := s.repo.createItem(ctx, tenantID, itemID, authorID, req, body, status)
	if err != nil {
		return ItemDTO{}, apperr.ErrContentCodeConflict.WithCause(err)
	}
	if err := s.writeAudit(ctx, tenantID, auditActionItemCreate, auditTargetItem, itemID, createItemAuditDetail(req)); err != nil {
		return ItemDTO{}, err
	}
	return out, nil
}

// ListItems 检索本租户内容外壳。
func (s *Service) ListItems(ctx context.Context, req ListItemsRequest) ([]ItemDTO, int64, error) {
	page, size := pagex.Normalize(req.Page, req.Size)
	rows, total, err := s.repo.listItems(ctx, req, size, (page-1)*size)
	if err != nil {
		return nil, 0, apperr.ErrContentQueryFailed.WithCause(err)
	}
	return rows, total, nil
}

// BatchGetFace 批量读取题面,供内部 HTTP 入口复用 service 层失败策略。
func (s *Service) BatchGetFace(ctx context.Context, refs []ItemRef) ([]ItemDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return nil, apperr.ErrUnauthorized
	}
	return s.batchGetFaceInTenant(ctx, id.TenantID, refs)
}

// batchGetFaceInTenant 在指定租户下批量展开题面并沿用单题敏感字段过滤。
func (s *Service) batchGetFaceInTenant(ctx context.Context, tenantID int64, refs []ItemRef) ([]ItemDTO, error) {
	out := make([]ItemDTO, 0, len(refs))
	for _, ref := range refs {
		item, err := s.getContentFaceInTenant(ctx, tenantID, ref.Code, ref.Version)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
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
	if err := s.ensureCanManage(ctx, ids.ParseOrZero(item.AuthorID)); err != nil {
		return ItemDTO{}, err
	}
	return item, nil
}

// UpdateDraft 更新草稿题目外壳与正文,只允许作者或平台在草稿态编辑。
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
	out, err := s.repo.updateDraft(ctx, id.TenantID, itemID, req, body, func(current contentItemGuard) error {
		// 在更新事务内先做权限与草稿状态判断,避免并发下越权修改已发布版本。
		if err := s.ensureCanManage(ctx, current.AuthorID); err != nil {
			return err
		}
		return validateDraftEditable(current.Status)
	})
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ItemDTO{}, ae
		}
		return ItemDTO{}, apperr.ErrContentUpdateFailed.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionItemUpdate, auditTargetItem, itemID, map[string]any{"id": ids.Format(itemID)}); err != nil {
		return ItemDTO{}, err
	}
	return out, nil
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
	if err := s.repo.deleteDraft(ctx, id.TenantID, itemID, func(current contentItemGuard) error {
		if err := s.ensureCanManage(ctx, current.AuthorID); err != nil {
			return err
		}
		return validateDraftDeletable(current.Status, current.UsageCount)
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
	rows, err := s.repo.listVersions(ctx, code)
	if err != nil {
		return nil, apperr.ErrContentVersionQueryFailed.WithCause(err)
	}
	return rows, nil
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
	} else if err = s.ensureCanManage(ctx, ids.ParseOrZero(source.AuthorID)); err != nil {
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
	if err := s.writeAudit(ctx, id.TenantID, auditActionItemClone, auditTargetItem, ids.ParseOrZero(out.ID), map[string]any{"source": code + ":" + version}); err != nil {
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
	rows, err := s.repo.listShared(ctx, typ, keyword, size, (page-1)*size)
	if err != nil {
		return nil, apperr.ErrContentShareInvalid.WithCause(err)
	}
	return rows, nil
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
	parentID := ids.ParseOrZero(req.ParentID)
	categories, err := s.ListCategories(ctx)
	if err != nil {
		return CategoryDTO{}, err
	}
	if err := validateCategoryParent(0, parentID, categories); err != nil {
		return CategoryDTO{}, err
	}
	rowID := s.idgen.Generate()
	row, err := s.repo.createCategory(ctx, id.TenantID, rowID, parentID, req)
	if err != nil {
		return CategoryDTO{}, apperr.ErrContentInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionCategorySave, auditTargetCategory, rowID, map[string]any{"name": req.Name}); err != nil {
		return CategoryDTO{}, err
	}
	return row, nil
}

// ListCategories 查询分类树。
func (s *Service) ListCategories(ctx context.Context) ([]CategoryDTO, error) {
	rows, err := s.repo.listCategories(ctx)
	if err != nil {
		return nil, apperr.ErrContentCategoryQueryFailed.WithCause(err)
	}
	return rows, nil
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
	parentID := ids.ParseOrZero(req.ParentID)
	categories, err := s.ListCategories(ctx)
	if err != nil {
		return CategoryDTO{}, err
	}
	if err := validateCategoryParent(categoryID, parentID, categories); err != nil {
		return CategoryDTO{}, err
	}
	row, err := s.repo.updateCategory(ctx, id.TenantID, categoryID, parentID, req)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return CategoryDTO{}, ae
		}
		return CategoryDTO{}, apperr.ErrContentInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionCategorySave, auditTargetCategory, categoryID, map[string]any{"name": req.Name}); err != nil {
		return CategoryDTO{}, err
	}
	return row, nil
}

// DeleteCategory 软删分类节点。
func (s *Service) DeleteCategory(ctx context.Context, categoryID int64) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if err := s.repo.deleteCategory(ctx, id.TenantID, categoryID); err != nil {
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
	items, err := s.preparePaperItems(ctx, req)
	if err != nil {
		return PaperDTO{}, err
	}
	row, itemRows, err := s.repo.createPaperWithItems(ctx, id.TenantID, paperID, id.AccountID, req, criteria, items)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return PaperDTO{}, ae
		}
		return PaperDTO{}, apperr.ErrPaperInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionPaperSave, auditTargetPaper, paperID, map[string]any{"name": req.Name}); err != nil {
		return PaperDTO{}, err
	}
	row.Items = paperItemsDTOFromRepoRows(itemRows)
	return row, nil
}

// ListPapers 查询试卷列表。
func (s *Service) ListPapers(ctx context.Context, page, size int) ([]PaperDTO, error) {
	page, size = pagex.Normalize(page, size)
	rows, err := s.repo.listPapers(ctx, size, (page-1)*size)
	if err != nil {
		return nil, apperr.ErrPaperQueryFailed.WithCause(err)
	}
	return rows, nil
}

// GetPaper 查询试卷详情,并以题面视角展开锁定版本题目。
func (s *Service) GetPaper(ctx context.Context, paperID int64) (PaperDTO, error) {
	row, itemRows, err := s.repo.getPaperWithItems(ctx, paperID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return PaperDTO{}, ae
		}
		return PaperDTO{}, apperr.ErrPaperQueryFailed.WithCause(err)
	}
	items, err := s.expandPaperItems(ctx, itemRows)
	if err != nil {
		return PaperDTO{}, err
	}
	row.Items = items
	return row, nil
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
	items, err := s.preparePaperItems(ctx, req)
	if err != nil {
		return PaperDTO{}, err
	}
	row, itemRows, err := s.repo.replacePaperItems(ctx, id.TenantID, paperID, items)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return PaperDTO{}, ae
		}
		return PaperDTO{}, apperr.ErrPaperInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionPaperSave, auditTargetPaper, paperID, map[string]any{"regenerated": true}); err != nil {
		return PaperDTO{}, err
	}
	row.Items = paperItemsDTOFromRepoRows(itemRows)
	return row, nil
}

// IncrementUsage 记录内容被引用。
func (s *Service) IncrementUsage(ctx context.Context, tenantID int64, code, version string) error {
	if err := s.repo.incrementUsage(ctx, tenantID, code, version); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrContentUsageUpdateFailed.WithCause(err)
	}
	if err := s.writeAudit(ctx, tenantID, auditActionItemUsage, auditTargetItem, 0, map[string]any{"code": code, "version": version}); err != nil {
		return err
	}
	return nil
}

// getOwnContent 读取本租户内容。
func (s *Service) getOwnContent(ctx context.Context, code, version string, face bool) (ItemDTO, error) {
	row, err := s.repo.getOwnContent(ctx, code, version)
	if err != nil {
		if db.IsNoRows(err) {
			return ItemDTO{}, apperr.ErrContentNotFound
		}
		return ItemDTO{}, apperr.ErrContentReadFailed.WithCause(err)
	}
	return itemDTOFromRow(row, face)
}

// getSharedContent 读取共享库内容,仅用于浏览题面或克隆源。
func (s *Service) getSharedContent(ctx context.Context, code, version string, face bool) (ItemDTO, error) {
	if !s.repo.hasPrivileged() {
		return ItemDTO{}, apperr.ErrContentShareInvalid
	}
	row, err := s.repo.getSharedContent(ctx, code, version)
	if err != nil {
		if db.IsNoRows(err) {
			return ItemDTO{}, apperr.ErrContentShareInvalid
		}
		return ItemDTO{}, apperr.ErrContentShareReadFailed.WithCause(err)
	}
	return itemDTOFromRow(row, face)
}

// updateStatus 更新内容状态。
func (s *Service) updateStatus(ctx context.Context, itemID int64, status int16, action string) (ItemDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ItemDTO{}, apperr.ErrUnauthorized
	}
	row, err := s.repo.updateStatus(ctx, id.TenantID, itemID, status, func(current contentItemGuard) error {
		return s.ensureCanManage(ctx, current.AuthorID)
	})
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ItemDTO{}, ae
		}
		return ItemDTO{}, apperr.ErrContentUpdateFailed.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, action, auditTargetItem, itemID, map[string]any{"status": status}); err != nil {
		return ItemDTO{}, err
	}
	return row, nil
}

// updateVisibility 更新共享可见性。
func (s *Service) updateVisibility(ctx context.Context, itemID int64, visibility int16, action string) (ItemDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ItemDTO{}, apperr.ErrUnauthorized
	}
	row, err := s.repo.updateVisibility(ctx, id.TenantID, itemID, visibility, func(current contentItemGuard) error {
		return s.ensureCanManage(ctx, current.AuthorID)
	})
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ItemDTO{}, ae
		}
		return ItemDTO{}, apperr.ErrContentUpdateFailed.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, action, auditTargetItem, itemID, map[string]any{"visibility": visibility}); err != nil {
		return ItemDTO{}, err
	}
	return row, nil
}

// preparePaperItems 创建试卷题目快照参数,手动模式锁定请求版本,随机模式锁定抽中的当前版本。
func (s *Service) preparePaperItems(ctx context.Context, req PaperRequest) ([]paperItemInsert, error) {
	items := req.Items
	if req.GenMode == PaperGenRandom {
		// 随机组卷先按条件锁定当前发布版本,不足量时整体失败,避免生成半张试卷。
		selected, err := s.repo.listRandomPaperItems(ctx, req.GenCriteria)
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
		for _, item := range selected {
			items = append(items, PaperItemReq{Code: item.Code, Version: item.Version, Score: score, Seq: item.Seq})
		}
	}
	out := make([]paperItemInsert, 0, len(items))
	for idx, item := range items {
		if req.GenMode == PaperGenManual {
			// 手动组卷必须逐题确认请求版本仍是已发布内容,防止引用草稿或下架版本。
			current, err := s.getOwnContent(ctx, item.Code, item.Version, false)
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
		// service 只生成锁定快照参数,实际写入由 repo 在试卷事务里完成。
		out = append(out, paperItemInsert{ID: s.idgen.Generate(), Code: item.Code, Version: item.Version, Score: item.Score, Seq: seq})
	}
	return out, nil
}

// expandPaperItems 展开试卷题目题面。
func (s *Service) expandPaperItems(ctx context.Context, rows []paperItemRow) ([]PaperItemDTO, error) {
	out := make([]PaperItemDTO, 0, len(rows))
	for _, row := range rows {
		item, err := s.GetFace(ctx, row.ItemCode, row.ItemVersion)
		if err != nil {
			return nil, err
		}
		out = append(out, paperItemDTOFromRepoRow(row, item))
	}
	return out, nil
}

// paperItemsDTOFromRepoRows 转换未展开题面的试卷题目快照。
func paperItemsDTOFromRepoRows(rows []paperItemRow) []PaperItemDTO {
	out := make([]PaperItemDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, paperItemDTOFromRepoRow(row, ItemDTO{}))
	}
	return out
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
	if !canReadOwnContentFace(id.IsPlatform, account, ids.ParseOrZero(item.AuthorID), item.Visibility) {
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
	if req.GenMode == PaperGenRandom && jsonx.IntFromAny(req.GenCriteria["count"]) <= 0 {
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
