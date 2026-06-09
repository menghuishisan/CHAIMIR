// M5 行转换层:集中处理 sqlc 行到领域投影和响应 DTO 的纯转换。
package content

import (
	"chaimir/internal/modules/content/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// contentRowFromOwn 把本租户内容联查行转成通用结构。
func contentRowFromOwn(row sqlcgen.GetContentByCodeVersionRow) itemRow {
	return itemRow{ID: row.ID, TenantID: row.TenantID, Code: row.Code, Version: row.Version, Type: row.Type, Title: row.Title,
		CategoryID: row.CategoryID, Difficulty: row.Difficulty, Tags: row.Tags, KnowledgePoints: row.KnowledgePoints,
		AuthorID: row.AuthorID, AuthorType: row.AuthorType, Visibility: row.Visibility, Status: row.Status,
		UsageCount: row.UsageCount, BodyHash: row.BodyHash, Body: row.Body, SensitiveFields: row.SensitiveFields}
}

// contentRowFromShared 把共享库联查行转成通用结构。
func contentRowFromShared(row sqlcgen.GetSharedContentByCodeVersionRow) itemRow {
	return itemRow{ID: row.ID, TenantID: row.TenantID, Code: row.Code, Version: row.Version, Type: row.Type, Title: row.Title,
		CategoryID: row.CategoryID, Difficulty: row.Difficulty, Tags: row.Tags, KnowledgePoints: row.KnowledgePoints,
		AuthorID: row.AuthorID, AuthorType: row.AuthorType, Visibility: row.Visibility, Status: row.Status,
		UsageCount: row.UsageCount, BodyHash: row.BodyHash, Body: row.Body, SensitiveFields: row.SensitiveFields}
}

// itemDTOFromShell 转换内容外壳摘要,不携带内容体。
func itemDTOFromShell(row sqlcgen.ContentItem) ItemDTO {
	return ItemDTO{ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), Code: row.Code, Version: row.Version, Type: row.Type,
		Title: row.Title, CategoryID: pgtypex.IDString(row.CategoryID), Difficulty: row.Difficulty, Tags: row.Tags,
		KnowledgePoints: row.KnowledgePoints, AuthorID: ids.Format(row.AuthorID), AuthorType: row.AuthorType,
		Visibility: row.Visibility, Status: row.Status, UsageCount: row.UsageCount, BodyHash: row.BodyHash,
		CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// itemsDTOFromShell 批量转换内容外壳摘要。
func itemsDTOFromShell(rows []sqlcgen.ContentItem) []ItemDTO {
	out := make([]ItemDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, itemDTOFromShell(row))
	}
	return out
}

// categoryDTOFromRow 转换分类行。
func categoryDTOFromRow(row sqlcgen.ContentCategory) CategoryDTO {
	return CategoryDTO{ID: ids.Format(row.ID), ParentID: pgtypex.IDString(row.ParentID), Name: row.Name, Sort: row.Sort, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// categoriesDTOFromRows 批量转换分类行。
func categoriesDTOFromRows(rows []sqlcgen.ContentCategory) []CategoryDTO {
	out := make([]CategoryDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, categoryDTOFromRow(row))
	}
	return out
}

// paperDTOFromRow 转换试卷行。
func paperDTOFromRow(row sqlcgen.Paper, items []PaperItemDTO) PaperDTO {
	return PaperDTO{ID: ids.Format(row.ID), Name: row.Name, AuthorID: ids.Format(row.AuthorID), GenMode: row.GenMode,
		GenCriteria: jsonx.ObjectMap(row.GenCriteria), Items: items, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// paperItemDTOFromRow 转换试卷题目行。
func paperItemDTOFromRow(row sqlcgen.PaperItem, item ItemDTO) PaperItemDTO {
	return PaperItemDTO{ID: ids.Format(row.ID), Code: row.ItemCode, Version: row.ItemVersion, Score: row.Score, Seq: row.Seq, Item: item}
}

// paperItemRowFromSQLC 把 sqlc 试卷题目行压缩为 service 需要的最小结构。
func paperItemRowFromSQLC(row sqlcgen.PaperItem) paperItemRow {
	return paperItemRow{ID: row.ID, ItemCode: row.ItemCode, ItemVersion: row.ItemVersion, Score: row.Score, Seq: row.Seq}
}

// paperItemRowsFromSQLC 批量压缩 sqlc 试卷题目行。
func paperItemRowsFromSQLC(rows []sqlcgen.PaperItem) []paperItemRow {
	out := make([]paperItemRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, paperItemRowFromSQLC(row))
	}
	return out
}
