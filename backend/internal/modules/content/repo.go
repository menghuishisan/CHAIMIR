// M5 数据访问层:只读写 content 模块自有表,跨校共享读取限定在受控特权查询。
package content

import (
	"context"

	"chaimir/internal/modules/content/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
)

// repo 封装 content 模块数据库事务入口。
type repo struct {
	db *db.DB
}

// contentItemGuard 暴露 service 做权限和状态判断所需的最小内容元信息。
type contentItemGuard struct {
	AuthorID   int64
	Status     int16
	UsageCount int32
}

// paperItemRow 是试卷题目行在 service 层展开题面所需的最小结构。
type paperItemRow struct {
	ID          int64
	ItemCode    string
	ItemVersion string
	Score       int32
	Seq         int32
}

// paperItemInsert 是 repo 写入试卷题目时需要的完整快照。
type paperItemInsert struct {
	ID      int64
	Code    string
	Version string
	Score   int32
	Seq     int32
}

// contentItemGuardFunc 让 service 在 repo 事务内执行纯权限和状态判断。
type contentItemGuardFunc func(contentItemGuard) error

// newRepo 构造 M5 repo。
func newRepo(database *db.DB) *repo {
	return &repo{db: database}
}

// queryFunc 是 M5 数据访问闭包,统一接收 sqlc 查询对象。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 从 ctx 取租户并注入 RLS 后执行查询。
func (r *repo) inTenant(ctx context.Context, fn queryFunc) error {
	return r.db.WithTenantTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inTenantID 用显式租户 ID 注入 RLS,供 contracts 内部调用使用。
func (r *repo) inTenantID(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inPrivileged 执行共享库受控跨租户读取,调用方必须在 SQL 层限定 shared+published。
func (r *repo) inPrivileged(ctx context.Context, fn queryFunc) error {
	return r.db.WithPrivilegedTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// hasPrivileged 返回是否配置特权池。
func (r *repo) hasPrivileged() bool { return r.db.HasPrivileged() }

// createItem 写入内容外壳和内容体,保证同一版本内容原子创建。
func (r *repo) createItem(ctx context.Context, tenantID, itemID, authorID int64, req CreateItemRequest, body []byte, status int16) (ItemDTO, error) {
	var row sqlcgen.ContentItem
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		created, e := q.CreateContentItem(ctx, sqlcgen.CreateContentItemParams{
			ID: itemID, TenantID: tenantID, Code: req.Code, Version: req.Version, Type: req.Type, Title: req.Title,
			CategoryID: pgtypex.Int8(ids.ParseOrZero(req.CategoryID)), Difficulty: req.Difficulty, Tags: req.Tags,
			KnowledgePoints: req.KnowledgePoints, AuthorID: authorID, AuthorType: req.AuthorType,
			Visibility: req.Visibility, Status: status, BodyHash: bodyHash(body),
		})
		if e != nil {
			return e
		}
		row = created
		_, e = q.CreateContentBody(ctx, sqlcgen.CreateContentBodyParams{ItemID: itemID, TenantID: tenantID, Body: body, SensitiveFields: req.SensitiveFields})
		return e
	})
	return itemDTOFromShell(row), err
}

// listItems 查询内容外壳列表与总数,用于 service 组装分页响应。
func (r *repo) listItems(ctx context.Context, req ListItemsRequest, size, offset int) ([]ItemDTO, int64, error) {
	var rows []sqlcgen.ContentItem
	var total int64
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListContentItems(ctx, listParams(req, size, offset))
		if e != nil {
			return e
		}
		rows = found
		total, e = q.CountContentItems(ctx, countParams(req))
		return e
	})
	return itemsDTOFromShell(rows), total, err
}

// updateDraft 在同一事务内读取当前内容、执行权限状态守卫并更新草稿正文。
func (r *repo) updateDraft(ctx context.Context, tenantID, itemID int64, req UpdateItemRequest, body []byte, guard contentItemGuardFunc) (ItemDTO, error) {
	var row sqlcgen.ContentItem
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		current, e := q.GetContentItemByID(ctx, itemID)
		if db.IsNoRows(e) {
			return apperr.ErrContentNotFound
		}
		if e != nil {
			return e
		}
		if e = guard(contentItemGuard{AuthorID: current.AuthorID, Status: current.Status, UsageCount: current.UsageCount}); e != nil {
			return e
		}
		row, e = q.UpdateContentDraft(ctx, sqlcgen.UpdateContentDraftParams{
			ID: itemID, Title: req.Title, CategoryID: pgtypex.Int8(ids.ParseOrZero(req.CategoryID)), Difficulty: req.Difficulty,
			Tags: req.Tags, KnowledgePoints: req.KnowledgePoints, Visibility: req.Visibility, BodyHash: bodyHash(body),
		})
		if e != nil {
			return e
		}
		_, e = q.UpdateContentBody(ctx, sqlcgen.UpdateContentBodyParams{ItemID: itemID, Body: body, SensitiveFields: req.SensitiveFields})
		return e
	})
	return itemDTOFromShell(row), err
}

// deleteDraft 软删符合业务守卫的草稿内容。
func (r *repo) deleteDraft(ctx context.Context, tenantID, itemID int64, guard contentItemGuardFunc) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		current, err := q.GetContentItemByID(ctx, itemID)
		if db.IsNoRows(err) {
			return apperr.ErrContentNotFound
		}
		if err != nil {
			return err
		}
		if err = guard(contentItemGuard{AuthorID: current.AuthorID, Status: current.Status, UsageCount: current.UsageCount}); err != nil {
			return err
		}
		_, err = q.SoftDeleteContentItem(ctx, itemID)
		if db.IsNoRows(err) {
			return apperr.ErrContentDeleteBlocked
		}
		return err
	})
}

// listVersions 查询同一内容 code 的所有版本外壳。
func (r *repo) listVersions(ctx context.Context, code string) ([]ItemDTO, error) {
	var rows []sqlcgen.ContentItem
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListContentVersions(ctx, code)
		rows = found
		return e
	})
	return itemsDTOFromShell(rows), err
}

// listShared 查询跨租户共享库中已发布内容摘要。
func (r *repo) listShared(ctx context.Context, typ int16, keyword string, size, offset int) ([]ItemDTO, error) {
	var rows []sqlcgen.ContentItem
	err := r.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListSharedContentItems(ctx, sqlcgen.ListSharedContentItemsParams{
			Limit: int32(size), Offset: int32(offset), Type: pgtypex.Int2(typ), Keyword: pgtypex.Text(keyword),
		})
		rows = found
		return e
	})
	return itemsDTOFromShell(rows), err
}

// createCategory 写入内容分类节点。
func (r *repo) createCategory(ctx context.Context, tenantID, rowID, parentID int64, req CategoryRequest) (CategoryDTO, error) {
	var row sqlcgen.ContentCategory
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		created, e := q.CreateContentCategory(ctx, sqlcgen.CreateContentCategoryParams{
			ID: rowID, TenantID: tenantID, ParentID: pgtypex.Int8(parentID), Name: req.Name, Sort: req.Sort,
		})
		row = created
		return e
	})
	return categoryDTOFromRow(row), err
}

// listCategories 查询当前租户分类树原始行。
func (r *repo) listCategories(ctx context.Context) ([]CategoryDTO, error) {
	var rows []sqlcgen.ContentCategory
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListContentCategories(ctx)
		rows = found
		return e
	})
	return categoriesDTOFromRows(rows), err
}

// updateCategory 更新分类节点并把缺失行转换为内容不存在错误。
func (r *repo) updateCategory(ctx context.Context, tenantID, categoryID, parentID int64, req CategoryRequest) (CategoryDTO, error) {
	var row sqlcgen.ContentCategory
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		updated, e := q.UpdateContentCategory(ctx, sqlcgen.UpdateContentCategoryParams{
			ID: categoryID, ParentID: pgtypex.Int8(parentID), Name: req.Name, Sort: req.Sort,
		})
		if db.IsNoRows(e) {
			return apperr.ErrContentNotFound
		}
		row = updated
		return e
	})
	return categoryDTOFromRow(row), err
}

// deleteCategory 软删分类节点并把缺失行转换为内容不存在错误。
func (r *repo) deleteCategory(ctx context.Context, tenantID, categoryID int64) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.DeleteContentCategory(ctx, categoryID)
		if db.IsNoRows(err) {
			return apperr.ErrContentNotFound
		}
		return err
	})
}

// listRandomPaperItems 按随机组卷条件查询可锁定的已发布内容版本。
func (r *repo) listRandomPaperItems(ctx context.Context, criteria map[string]any) ([]PaperItemReq, error) {
	var items []PaperItemReq
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		selected, e := q.ListPublishedItemsForRandomPaper(ctx, randomParams(criteria))
		if e != nil {
			return e
		}
		items = make([]PaperItemReq, 0, len(selected))
		for idx, item := range selected {
			items = append(items, PaperItemReq{Code: item.Code, Version: item.Version, Seq: int32(idx + 1)})
		}
		return nil
	})
	return items, err
}

// createPaperWithItems 创建试卷并写入锁定版本题目快照。
func (r *repo) createPaperWithItems(ctx context.Context, tenantID, paperID, authorID int64, req PaperRequest, criteria []byte, items []paperItemInsert) (PaperDTO, []paperItemRow, error) {
	var row sqlcgen.Paper
	var itemRows []paperItemRow
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		created, e := q.CreatePaper(ctx, sqlcgen.CreatePaperParams{
			ID: paperID, TenantID: tenantID, Name: req.Name, AuthorID: authorID, GenMode: req.GenMode, GenCriteria: criteria,
		})
		if e != nil {
			return e
		}
		row = created
		itemRows, e = r.createPaperItemRows(ctx, q, tenantID, paperID, items)
		return e
	})
	return paperDTOFromRow(row, nil), itemRows, err
}

// listPapers 查询试卷列表。
func (r *repo) listPapers(ctx context.Context, size, offset int) ([]PaperDTO, error) {
	var rows []sqlcgen.Paper
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListPapers(ctx, sqlcgen.ListPapersParams{Limit: int32(size), Offset: int32(offset)})
		rows = found
		return e
	})
	out := make([]PaperDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, paperDTOFromRow(row, nil))
	}
	return out, err
}

// getPaperWithItems 查询试卷及其锁定题目。
func (r *repo) getPaperWithItems(ctx context.Context, paperID int64) (PaperDTO, []paperItemRow, error) {
	var row sqlcgen.Paper
	var items []paperItemRow
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.GetPaperByID(ctx, paperID)
		if db.IsNoRows(e) {
			return apperr.ErrPaperNotFound
		}
		if e != nil {
			return e
		}
		row = found
		itemRows, e := q.ListPaperItems(ctx, paperID)
		if e != nil {
			return e
		}
		items = paperItemRowsFromSQLC(itemRows)
		return nil
	})
	return paperDTOFromRow(row, nil), items, err
}

// replacePaperItems 删除旧题目并写入新的锁定版本题目集合。
func (r *repo) replacePaperItems(ctx context.Context, tenantID, paperID int64, items []paperItemInsert) (PaperDTO, []paperItemRow, error) {
	var row sqlcgen.Paper
	var itemRows []paperItemRow
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
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
		itemRows, e = r.createPaperItemRows(ctx, q, tenantID, paperID, items)
		return e
	})
	return paperDTOFromRow(row, nil), itemRows, err
}

// incrementUsage 增加内容引用计数。
func (r *repo) incrementUsage(ctx context.Context, tenantID int64, code, version string) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.IncrementContentUsage(ctx, sqlcgen.IncrementContentUsageParams{Code: code, Version: version})
		if db.IsNoRows(err) {
			return apperr.ErrContentUnavailable
		}
		return err
	})
}

// getOwnContent 读取当前租户内容外壳与内容体。
func (r *repo) getOwnContent(ctx context.Context, code, version string) (itemRow, error) {
	var row itemRow
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.GetContentByCodeVersion(ctx, sqlcgen.GetContentByCodeVersionParams{Code: code, Version: version})
		row = contentRowFromOwn(found)
		return e
	})
	return row, err
}

// getContentInTenant 用显式租户读取内容外壳与内容体,供 contracts 入口使用。
func (r *repo) getContentInTenant(ctx context.Context, tenantID int64, code, version string) (itemRow, error) {
	var row itemRow
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, e := q.GetContentByCodeVersion(ctx, sqlcgen.GetContentByCodeVersionParams{Code: code, Version: version})
		row = contentRowFromOwn(found)
		return e
	})
	return row, err
}

// getSharedContent 读取共享库内容外壳与内容体。
func (r *repo) getSharedContent(ctx context.Context, code, version string) (itemRow, error) {
	var row itemRow
	err := r.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.GetSharedContentByCodeVersion(ctx, sqlcgen.GetSharedContentByCodeVersionParams{Code: code, Version: version})
		row = contentRowFromShared(found)
		return e
	})
	return row, err
}

// updateStatus 在事务内校验当前内容守卫后更新状态。
func (r *repo) updateStatus(ctx context.Context, tenantID, itemID int64, status int16, guard contentItemGuardFunc) (ItemDTO, error) {
	var row sqlcgen.ContentItem
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		current, e := q.GetContentItemByID(ctx, itemID)
		if db.IsNoRows(e) {
			return apperr.ErrContentNotFound
		}
		if e != nil {
			return e
		}
		if e = guard(contentItemGuard{AuthorID: current.AuthorID, Status: current.Status, UsageCount: current.UsageCount}); e != nil {
			return e
		}
		updated, e := q.UpdateContentStatus(ctx, sqlcgen.UpdateContentStatusParams{ID: itemID, Status: status})
		if db.IsNoRows(e) {
			return apperr.ErrContentNotFound
		}
		row = updated
		return e
	})
	return itemDTOFromShell(row), err
}

// updateVisibility 在事务内校验当前内容守卫后更新共享可见性。
func (r *repo) updateVisibility(ctx context.Context, tenantID, itemID int64, visibility int16, guard contentItemGuardFunc) (ItemDTO, error) {
	var row sqlcgen.ContentItem
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		current, e := q.GetContentItemByID(ctx, itemID)
		if db.IsNoRows(e) {
			return apperr.ErrContentNotFound
		}
		if e != nil {
			return e
		}
		if e = guard(contentItemGuard{AuthorID: current.AuthorID, Status: current.Status, UsageCount: current.UsageCount}); e != nil {
			return e
		}
		updated, e := q.UpdateContentVisibility(ctx, sqlcgen.UpdateContentVisibilityParams{ID: itemID, Visibility: visibility})
		if db.IsNoRows(e) {
			return apperr.ErrContentUnavailable
		}
		row = updated
		return e
	})
	return itemDTOFromShell(row), err
}

// createPaperItemRows 写入试卷题目快照,调用方负责提前完成业务校验与 ID 分配。
func (r *repo) createPaperItemRows(ctx context.Context, q *sqlcgen.Queries, tenantID, paperID int64, items []paperItemInsert) ([]paperItemRow, error) {
	out := make([]paperItemRow, 0, len(items))
	for _, item := range items {
		row, err := q.CreatePaperItem(ctx, sqlcgen.CreatePaperItemParams{
			ID: item.ID, TenantID: tenantID, PaperID: paperID, ItemCode: item.Code,
			ItemVersion: item.Version, Score: item.Score, Seq: item.Seq,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, paperItemRowFromSQLC(row))
	}
	return out, nil
}

// tenantFromContext 读取当前租户身份。
func tenantFromContext(ctx context.Context) (tenant.Identity, bool) {
	return tenant.FromContext(ctx)
}

// listParams 构造内容列表查询参数。
func listParams(req ListItemsRequest, size, offset int) sqlcgen.ListContentItemsParams {
	return sqlcgen.ListContentItemsParams{
		Limit: int32(size), Offset: int32(offset), Type: pgtypex.Int2(req.Type), CategoryID: pgtypex.Int8(ids.ParseOrZero(req.CategoryID)),
		Difficulty: pgtypex.Int2(req.Difficulty), Visibility: pgtypex.Int2(req.Visibility), Status: pgtypex.Int2(req.Status),
		Tag: pgtypex.Text(req.Tag), Kp: pgtypex.Text(req.KP), Keyword: pgtypex.Text(req.Keyword),
	}
}

// countParams 构造内容计数查询参数。
func countParams(req ListItemsRequest) sqlcgen.CountContentItemsParams {
	return sqlcgen.CountContentItemsParams{
		Type: pgtypex.Int2(req.Type), CategoryID: pgtypex.Int8(ids.ParseOrZero(req.CategoryID)), Difficulty: pgtypex.Int2(req.Difficulty),
		Visibility: pgtypex.Int2(req.Visibility), Status: pgtypex.Int2(req.Status), Tag: pgtypex.Text(req.Tag), Kp: pgtypex.Text(req.KP), Keyword: pgtypex.Text(req.Keyword),
	}
}

// randomParams 构造随机组卷查询参数。
func randomParams(criteria map[string]any) sqlcgen.ListPublishedItemsForRandomPaperParams {
	normalized := normalizeRandomCriteria(criteria)
	return sqlcgen.ListPublishedItemsForRandomPaperParams{
		Limit: int32(normalized.Count), Type: pgtypex.Int2(normalized.Type),
		Difficulties: normalized.Difficulties, KnowledgePoints: normalized.KnowledgePoints,
	}
}
