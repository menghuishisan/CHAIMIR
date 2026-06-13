// content row_convert 文件负责 sqlc 行到 M5 领域模型的纯转换。
package content

import (
	"chaimir/internal/modules/content/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// itemFromRow 转换内容外壳行。
func itemFromRow(row sqlcgen.ContentItem) Item {
	return Item{
		ID:              row.ID,
		TenantID:        row.TenantID,
		Code:            row.Code,
		Version:         row.Version,
		Type:            row.Type,
		Title:           row.Title,
		CategoryID:      pgtypex.Int8Value(row.CategoryID),
		Difficulty:      row.Difficulty,
		Tags:            cloneStrings(row.Tags),
		KnowledgePoints: cloneStrings(row.KnowledgePoints),
		AuthorID:        row.AuthorID,
		AuthorType:      row.AuthorType,
		Visibility:      row.Visibility,
		Status:          row.Status,
		UsageCount:      row.UsageCount,
		VersionHash:     row.VersionHash,
		CreatedAt:       timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:       timex.FromTimestamptz(row.UpdatedAt),
	}
}

// itemWithBodyFromRefRow 转换按引用读取的内容快照。
func itemWithBodyFromRefRow(row sqlcgen.GetContentItemWithBodyByRefRow) (ItemWithBody, error) {
	body, err := decodeMap(row.Body)
	if err != nil {
		return ItemWithBody{}, err
	}
	return ItemWithBody{Item: itemFromRow(sqlcgen.ContentItem{ID: row.ID, TenantID: row.TenantID, Code: row.Code, Version: row.Version, Type: row.Type, Title: row.Title, CategoryID: row.CategoryID, Difficulty: row.Difficulty, Tags: row.Tags, KnowledgePoints: row.KnowledgePoints, AuthorID: row.AuthorID, AuthorType: row.AuthorType, Visibility: row.Visibility, Status: row.Status, UsageCount: row.UsageCount, VersionHash: row.VersionHash, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt}), Body: body, SensitiveFields: cloneStrings(row.SensitiveFields)}, nil
}

// itemWithBodyFromIDRow 转换按 ID 读取的内容快照。
func itemWithBodyFromIDRow(row sqlcgen.GetContentItemWithBodyByIDRow) (ItemWithBody, error) {
	body, err := decodeMap(row.Body)
	if err != nil {
		return ItemWithBody{}, err
	}
	return ItemWithBody{Item: itemFromRow(sqlcgen.ContentItem{ID: row.ID, TenantID: row.TenantID, Code: row.Code, Version: row.Version, Type: row.Type, Title: row.Title, CategoryID: row.CategoryID, Difficulty: row.Difficulty, Tags: row.Tags, KnowledgePoints: row.KnowledgePoints, AuthorID: row.AuthorID, AuthorType: row.AuthorType, Visibility: row.Visibility, Status: row.Status, UsageCount: row.UsageCount, VersionHash: row.VersionHash, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt}), Body: body, SensitiveFields: cloneStrings(row.SensitiveFields)}, nil
}

// categoryFromRow 转换分类行。
func categoryFromRow(row sqlcgen.ContentCategory) Category {
	return Category{ID: row.ID, TenantID: row.TenantID, ParentID: pgtypex.Int8Value(row.ParentID), Name: row.Name, Sort: row.Sort, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// paperFromRow 转换试卷行并解析组卷条件。
func paperFromRow(row sqlcgen.Paper) (Paper, error) {
	criteria, err := criteriaFromJSON(row.GenCriteria)
	if err != nil {
		return Paper{}, err
	}
	return Paper{ID: row.ID, TenantID: row.TenantID, Name: row.Name, AuthorID: row.AuthorID, GenMode: row.GenMode, GenCriteria: criteria, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}, nil
}

// paperItemFromRow 转换卷题关联行。
func paperItemFromRow(row sqlcgen.PaperItem) PaperItem {
	return PaperItem{ID: row.ID, TenantID: row.TenantID, PaperID: row.PaperID, ItemCode: row.ItemCode, ItemVersion: row.ItemVersion, Score: row.Score, Seq: row.Seq, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// criteriaJSON 序列化随机组卷条件。
func criteriaJSON(criteria PaperCriteria) ([]byte, error) {
	if criteria.KnowledgePoints == nil {
		criteria.KnowledgePoints = []string{}
	}
	if criteria.Difficulties == nil {
		criteria.Difficulties = []int16{}
	}
	return jsonx.AnyBytes(criteria, apperr.ErrPaperInvalid)
}

// criteriaFromJSON 反序列化随机组卷条件。
func criteriaFromJSON(raw []byte) (PaperCriteria, error) {
	if len(raw) == 0 {
		return PaperCriteria{}, nil
	}
	var out PaperCriteria
	if err := jsonx.DecodeStrict(raw, &out); err != nil {
		return PaperCriteria{}, err
	}
	return out, nil
}

// decodeMap 解码 JSONB 对象为空 map。
func decodeMap(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	return jsonx.ObjectMapStrict(raw)
}

// encodeMap 编码 JSONB 对象。
func encodeMap(value map[string]any) ([]byte, error) {
	if value == nil {
		value = map[string]any{}
	}
	return jsonx.ObjectBytes(value, apperr.ErrContentBodyInvalid)
}
