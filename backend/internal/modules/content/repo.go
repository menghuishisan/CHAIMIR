// content repo 文件定义 M5 持久化接口和数据库事务边界,是 service 访问数据库的唯一入口。
package content

import (
	"context"
	"errors"
	"fmt"

	"chaimir/internal/modules/content/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/pgtypex"

	"github.com/jackc/pgx/v5"
)

// Store 定义 service 所需的 content 持久化能力,不暴露 sqlc 行类型。
type Store interface {
	TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error
}

// TxStore 定义单个事务内可调用的数据访问能力。
type TxStore interface {
	CreateItem(ctx context.Context, item ItemWithBody) (ItemWithBody, error)
	UpdateDraftItem(ctx context.Context, item ItemWithBody) (ItemWithBody, error)
	GetItemByID(ctx context.Context, tenantID, id int64) (Item, error)
	GetItemWithBodyByID(ctx context.Context, tenantID, id int64) (ItemWithBody, error)
	GetItemWithBodyByRef(ctx context.Context, tenantID int64, code, version string) (ItemWithBody, error)
	ListItems(ctx context.Context, tenantID int64, filter ItemListFilter) ([]Item, int64, error)
	ListVersions(ctx context.Context, tenantID int64, code string) ([]Item, error)
	PublishItem(ctx context.Context, tenantID, id int64) (Item, error)
	DeprecateItem(ctx context.Context, tenantID, id int64) (Item, error)
	DeleteDraftItem(ctx context.Context, tenantID, id int64) (Item, error)
	SetVisibility(ctx context.Context, tenantID, id int64, visibility int16) (Item, error)
	GetPublishedItemForUsage(ctx context.Context, tenantID int64, code, version string) (Item, error)
	ReplaceUsageRefs(ctx context.Context, tenantID int64, sourceScope, sourceRef string, refs []UsageRef) error
	CreateCategory(ctx context.Context, category Category) (Category, error)
	UpdateCategory(ctx context.Context, category Category) (Category, error)
	DeleteCategory(ctx context.Context, tenantID, id int64) (Category, error)
	ListCategories(ctx context.Context, tenantID int64) ([]Category, error)
	CreatePaper(ctx context.Context, paper Paper) (Paper, error)
	ReplacePaperItems(ctx context.Context, tenantID, paperID int64, items []PaperItem) ([]PaperItem, error)
	GetPaper(ctx context.Context, tenantID, id int64) (Paper, error)
	ListPapers(ctx context.Context, tenantID int64, page, size int) ([]Paper, int64, error)
	ListPaperItems(ctx context.Context, tenantID, paperID int64) ([]PaperItem, error)
	RandomPickItems(ctx context.Context, tenantID int64, criteria PaperCriteria) ([]Item, error)
}

type store struct {
	database *db.DB
}

type txStore struct {
	q *sqlcgen.Queries
}

// NewStore 创建 content 模块持久化入口,仅装配层应调用。
func NewStore(database *db.DB) Store {
	return &store{database: database}
}

// TenantTx 在注入 RLS 租户变量后访问 M5 租户表。
func (s *store) TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("content store 未初始化")
	}
	return s.database.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// isNoRows 统一识别未命中错误,让 service 不直接依赖 pgx。
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// CreateItem 创建内容外壳与正文。
func (s *txStore) CreateItem(ctx context.Context, item ItemWithBody) (ItemWithBody, error) {
	body, err := encodeMap(item.Body)
	if err != nil {
		return ItemWithBody{}, err
	}
	row, err := s.q.CreateContentItem(ctx, sqlcgen.CreateContentItemParams{ID: item.ID, TenantID: item.TenantID, Code: item.Code, Version: item.Version, Type: item.Type, Title: item.Title, CategoryID: pgtypex.Int8(item.CategoryID), Difficulty: item.Difficulty, Tags: item.Tags, KnowledgePoints: item.KnowledgePoints, AuthorID: item.AuthorID, AuthorType: item.AuthorType, Visibility: item.Visibility, Status: item.Status, VersionHash: item.VersionHash})
	if err != nil {
		return ItemWithBody{}, err
	}
	if _, err := s.q.CreateContentBody(ctx, sqlcgen.CreateContentBodyParams{ItemID: row.ID, TenantID: row.TenantID, Body: body, SensitiveFields: item.SensitiveFields}); err != nil {
		return ItemWithBody{}, err
	}
	created := item
	created.Item = itemFromRow(row)
	return created, nil
}

// UpdateDraftItem 更新草稿外壳与正文。
func (s *txStore) UpdateDraftItem(ctx context.Context, item ItemWithBody) (ItemWithBody, error) {
	body, err := encodeMap(item.Body)
	if err != nil {
		return ItemWithBody{}, err
	}
	row, err := s.q.UpdateDraftContentItem(ctx, sqlcgen.UpdateDraftContentItemParams{TenantID: item.TenantID, ID: item.ID, Title: item.Title, CategoryID: pgtypex.Int8(item.CategoryID), Difficulty: item.Difficulty, Tags: item.Tags, KnowledgePoints: item.KnowledgePoints, Visibility: item.Visibility, VersionHash: item.VersionHash})
	if err != nil {
		return ItemWithBody{}, err
	}
	if _, err := s.q.UpdateContentBody(ctx, sqlcgen.UpdateContentBodyParams{TenantID: item.TenantID, ItemID: item.ID, Body: body, SensitiveFields: item.SensitiveFields}); err != nil {
		return ItemWithBody{}, err
	}
	return ItemWithBody{Item: itemFromRow(row), Body: item.Body, SensitiveFields: item.SensitiveFields}, nil
}

// GetItemByID 按 ID 查询内容外壳。
func (s *txStore) GetItemByID(ctx context.Context, tenantID, id int64) (Item, error) {
	row, err := s.q.GetContentItemByID(ctx, sqlcgen.GetContentItemByIDParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Item{}, err
	}
	return itemFromRow(row), nil
}

// GetItemWithBodyByID 按 ID 查询完整内容。
func (s *txStore) GetItemWithBodyByID(ctx context.Context, tenantID, id int64) (ItemWithBody, error) {
	row, err := s.q.GetContentItemWithBodyByID(ctx, sqlcgen.GetContentItemWithBodyByIDParams{TenantID: tenantID, ID: id})
	if err != nil {
		return ItemWithBody{}, err
	}
	return itemWithBodyFromIDRow(row)
}

// GetItemWithBodyByRef 按 code/version 查询完整内容。
func (s *txStore) GetItemWithBodyByRef(ctx context.Context, tenantID int64, code, version string) (ItemWithBody, error) {
	row, err := s.q.GetContentItemWithBodyByRef(ctx, sqlcgen.GetContentItemWithBodyByRefParams{Code: code, Version: version, TenantID: tenantID})
	if err != nil {
		return ItemWithBody{}, err
	}
	return itemWithBodyFromRefRow(row)
}

// ListItems 查询内容分页。
func (s *txStore) ListItems(ctx context.Context, tenantID int64, filter ItemListFilter) ([]Item, int64, error) {
	params := sqlcgen.ListContentItemsParams{TenantID: tenantID, Column2: filter.Type, Column3: filter.CategoryID, Column4: filter.Difficulty, Column5: filter.Tag, Column6: filter.KnowledgePoint, Column7: filter.Keyword, Column8: filter.Visibility, Column9: filter.Status, Column10: filter.AuthorID, Column11: filter.OnlyShared, AuthorID: filter.ViewerID, Limit: int32(filter.Size), Offset: int32((filter.Page - 1) * filter.Size)}
	rows, err := s.q.ListContentItems(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountContentItems(ctx, sqlcgen.CountContentItemsParams{TenantID: tenantID, Column2: filter.Type, Column3: filter.CategoryID, Column4: filter.Difficulty, Column5: filter.Tag, Column6: filter.KnowledgePoint, Column7: filter.Keyword, Column8: filter.Visibility, Column9: filter.Status, Column10: filter.AuthorID, Column11: filter.OnlyShared, AuthorID: filter.ViewerID})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Item, 0, len(rows))
	for _, row := range rows {
		item := itemFromRow(row)
		if filter.PublishedShared && (item.Visibility != VisibilityShared || item.Status != StatusPublished) {
			continue
		}
		if item.Visibility == VisibilityPrivate && item.AuthorID != filter.ViewerID {
			continue
		}
		out = append(out, item)
	}
	return out, total, nil
}

// ListVersions 查询同 code 下版本。
func (s *txStore) ListVersions(ctx context.Context, tenantID int64, code string) ([]Item, error) {
	rows, err := s.q.ListContentVersions(ctx, sqlcgen.ListContentVersionsParams{TenantID: tenantID, Code: code})
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(rows))
	for _, row := range rows {
		out = append(out, itemFromRow(row))
	}
	return out, nil
}

// PublishItem 发布草稿。
func (s *txStore) PublishItem(ctx context.Context, tenantID, id int64) (Item, error) {
	row, err := s.q.PublishContentItem(ctx, sqlcgen.PublishContentItemParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Item{}, err
	}
	return itemFromRow(row), nil
}

// DeprecateItem 弃用已发布内容。
func (s *txStore) DeprecateItem(ctx context.Context, tenantID, id int64) (Item, error) {
	row, err := s.q.DeprecateContentItem(ctx, sqlcgen.DeprecateContentItemParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Item{}, err
	}
	return itemFromRow(row), nil
}

// DeleteDraftItem 软删未引用草稿。
func (s *txStore) DeleteDraftItem(ctx context.Context, tenantID, id int64) (Item, error) {
	row, err := s.q.SoftDeleteDraftContentItem(ctx, sqlcgen.SoftDeleteDraftContentItemParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Item{}, err
	}
	return itemFromRow(row), nil
}

// SetVisibility 更新已发布内容可见性。
func (s *txStore) SetVisibility(ctx context.Context, tenantID, id int64, visibility int16) (Item, error) {
	row, err := s.q.SetContentVisibility(ctx, sqlcgen.SetContentVisibilityParams{TenantID: tenantID, ID: id, Visibility: visibility})
	if err != nil {
		return Item{}, err
	}
	return itemFromRow(row), nil
}

// GetPublishedItemForUsage 读取可被业务引用的已发布内容版本。
func (s *txStore) GetPublishedItemForUsage(ctx context.Context, tenantID int64, code, version string) (Item, error) {
	row, err := s.q.GetPublishedContentItemForUsage(ctx, sqlcgen.GetPublishedContentItemForUsageParams{TenantID: tenantID, Code: code, Version: version})
	if err != nil {
		return Item{}, err
	}
	return itemFromRow(row), nil
}

// ReplaceUsageRefs 替换业务来源持有的内容引用集合并刷新计数。
func (s *txStore) ReplaceUsageRefs(ctx context.Context, tenantID int64, sourceScope, sourceRef string, refs []UsageRef) error {
	oldItemIDs, err := s.q.ListContentUsageItemIDsBySource(ctx, sqlcgen.ListContentUsageItemIDsBySourceParams{TenantID: tenantID, SourceScope: sourceScope, SourceRef: sourceRef})
	if err != nil {
		return err
	}
	if err := s.q.DeleteContentUsageRefsBySource(ctx, sqlcgen.DeleteContentUsageRefsBySourceParams{TenantID: tenantID, SourceScope: sourceScope, SourceRef: sourceRef}); err != nil {
		return err
	}
	changed := map[int64]struct{}{}
	for _, itemID := range oldItemIDs {
		changed[itemID] = struct{}{}
	}
	for _, ref := range refs {
		if _, err := s.q.CreateContentUsageRef(ctx, sqlcgen.CreateContentUsageRefParams{ID: ref.ID, TenantID: tenantID, ItemID: ref.ItemID, ItemCode: ref.ItemCode, ItemVersion: ref.ItemVersion, SourceScope: sourceScope, SourceRef: sourceRef}); err != nil {
			return err
		}
		changed[ref.ItemID] = struct{}{}
	}
	for itemID := range changed {
		if _, err := s.q.RefreshContentUsageCount(ctx, sqlcgen.RefreshContentUsageCountParams{TenantID: tenantID, ID: itemID}); err != nil {
			return err
		}
	}
	return nil
}

// CreateCategory 创建分类。
func (s *txStore) CreateCategory(ctx context.Context, category Category) (Category, error) {
	row, err := s.q.CreateContentCategory(ctx, sqlcgen.CreateContentCategoryParams{ID: category.ID, TenantID: category.TenantID, ParentID: pgtypex.Int8(category.ParentID), Name: category.Name, Sort: category.Sort})
	if err != nil {
		return Category{}, err
	}
	return categoryFromRow(row), nil
}

// UpdateCategory 更新分类。
func (s *txStore) UpdateCategory(ctx context.Context, category Category) (Category, error) {
	row, err := s.q.UpdateContentCategory(ctx, sqlcgen.UpdateContentCategoryParams{TenantID: category.TenantID, ID: category.ID, ParentID: pgtypex.Int8(category.ParentID), Name: category.Name, Sort: category.Sort})
	if err != nil {
		return Category{}, err
	}
	return categoryFromRow(row), nil
}

// DeleteCategory 软删分类。
func (s *txStore) DeleteCategory(ctx context.Context, tenantID, id int64) (Category, error) {
	row, err := s.q.DeleteContentCategory(ctx, sqlcgen.DeleteContentCategoryParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Category{}, err
	}
	return categoryFromRow(row), nil
}

// ListCategories 查询分类树。
func (s *txStore) ListCategories(ctx context.Context, tenantID int64) ([]Category, error) {
	rows, err := s.q.ListContentCategories(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]Category, 0, len(rows))
	for _, row := range rows {
		out = append(out, categoryFromRow(row))
	}
	return out, nil
}

// CreatePaper 创建试卷外壳。
func (s *txStore) CreatePaper(ctx context.Context, paper Paper) (Paper, error) {
	raw, err := criteriaJSON(paper.GenCriteria)
	if err != nil {
		return Paper{}, err
	}
	row, err := s.q.CreatePaper(ctx, sqlcgen.CreatePaperParams{ID: paper.ID, TenantID: paper.TenantID, Name: paper.Name, AuthorID: paper.AuthorID, GenMode: paper.GenMode, GenCriteria: raw})
	if err != nil {
		return Paper{}, err
	}
	return paperFromRow(row)
}

// ReplacePaperItems 覆盖试卷题目集合。
func (s *txStore) ReplacePaperItems(ctx context.Context, tenantID, paperID int64, items []PaperItem) ([]PaperItem, error) {
	if err := s.q.DeletePaperItems(ctx, sqlcgen.DeletePaperItemsParams{TenantID: tenantID, PaperID: paperID}); err != nil {
		return nil, err
	}
	out := make([]PaperItem, 0, len(items))
	for _, item := range items {
		row, err := s.q.CreatePaperItem(ctx, sqlcgen.CreatePaperItemParams{ID: item.ID, TenantID: tenantID, PaperID: paperID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, Seq: item.Seq})
		if err != nil {
			return nil, err
		}
		out = append(out, paperItemFromRow(row))
	}
	return out, nil
}

// GetPaper 按 ID 查询试卷。
func (s *txStore) GetPaper(ctx context.Context, tenantID, id int64) (Paper, error) {
	row, err := s.q.GetPaper(ctx, sqlcgen.GetPaperParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Paper{}, err
	}
	return paperFromRow(row)
}

// ListPapers 查询试卷分页。
func (s *txStore) ListPapers(ctx context.Context, tenantID int64, page, size int) ([]Paper, int64, error) {
	rows, err := s.q.ListPapers(ctx, sqlcgen.ListPapersParams{TenantID: tenantID, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountPapers(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Paper, 0, len(rows))
	for _, row := range rows {
		paper, err := paperFromRow(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, paper)
	}
	return out, total, nil
}

// ListPaperItems 查询卷题集合。
func (s *txStore) ListPaperItems(ctx context.Context, tenantID, paperID int64) ([]PaperItem, error) {
	rows, err := s.q.ListPaperItems(ctx, sqlcgen.ListPaperItemsParams{TenantID: tenantID, PaperID: paperID})
	if err != nil {
		return nil, err
	}
	out := make([]PaperItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, paperItemFromRow(row))
	}
	return out, nil
}

// RandomPickItems 按条件随机抽取已发布内容。
func (s *txStore) RandomPickItems(ctx context.Context, tenantID int64, criteria PaperCriteria) ([]Item, error) {
	rows, err := s.q.RandomPickContentItems(ctx, sqlcgen.RandomPickContentItemsParams{TenantID: tenantID, Column2: criteria.Type, Column3: criteria.Difficulties, Column4: criteria.KnowledgePoints, Limit: criteria.Count})
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(rows))
	for _, row := range rows {
		out = append(out, itemFromRow(row))
	}
	return out, nil
}
