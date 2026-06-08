// M5 转换工具:在 sqlc 行、HTTP DTO 与 contracts DTO 之间做稳定转换。
package content

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/content/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"

	"github.com/jackc/pgx/v5/pgtype"
)

// itemRow 是内容外壳与内容体联查行的最小共同结构。
type itemRow struct {
	ID              int64
	TenantID        int64
	Code            string
	Version         string
	Type            int16
	Title           string
	CategoryID      pgtype.Int8
	Difficulty      int16
	Tags            []string
	KnowledgePoints []string
	AuthorID        int64
	AuthorType      int16
	Visibility      int16
	Status          int16
	UsageCount      int32
	BodyHash        string
	Body            []byte
	SensitiveFields []string
}

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

// itemDTOFromRow 转换完整内容行,face=true 时剥离敏感字段。
func itemDTOFromRow(row itemRow, face bool) (ItemDTO, error) {
	if err := validateBodyHash(row.Body, row.BodyHash); err != nil {
		return ItemDTO{}, err
	}
	body := jsonx.ObjectMap(row.Body)
	fields := append([]string{}, row.SensitiveFields...)
	if face {
		body = filterSensitiveBody(body, fields)
		fields = nil
	}
	dto := ItemDTO{ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), Code: row.Code, Version: row.Version, Type: row.Type,
		Title: row.Title, CategoryID: fmtOptionalID(row.CategoryID), Difficulty: row.Difficulty, Tags: row.Tags,
		KnowledgePoints: row.KnowledgePoints, AuthorID: ids.Format(row.AuthorID), AuthorType: row.AuthorType,
		Visibility: row.Visibility, Status: row.Status, UsageCount: row.UsageCount, BodyHash: row.BodyHash,
		Body: body, SensitiveFields: fields}
	return dto, nil
}

// itemDTOFromShell 转换内容外壳摘要,不携带内容体。
func itemDTOFromShell(row sqlcgen.ContentItem) ItemDTO {
	return ItemDTO{ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), Code: row.Code, Version: row.Version, Type: row.Type,
		Title: row.Title, CategoryID: fmtOptionalID(row.CategoryID), Difficulty: row.Difficulty, Tags: row.Tags,
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
	return CategoryDTO{ID: ids.Format(row.ID), ParentID: fmtOptionalID(row.ParentID), Name: row.Name, Sort: row.Sort, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
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

// contractSnapshotFromItem 把内容 DTO 转换为跨模块内容快照。
func contractSnapshotFromItem(dto ItemDTO) contracts.ContentItemSnapshot {
	return contracts.ContentItemSnapshot{ItemCode: dto.Code, ItemVersion: dto.Version, Type: dto.Type, Title: dto.Title,
		Difficulty: dto.Difficulty, Tags: dto.Tags, KnowledgePoints: dto.KnowledgePoints, Body: dto.Body,
		VersionHash: dto.BodyHash, Status: dto.Status}
}

// bodyHash 计算内容体哈希,用于版本完整性校验。
func bodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// fmtOptionalID 转换可空雪花 ID。
func fmtOptionalID(id pgtype.Int8) string {
	if !id.Valid {
		return ""
	}
	return ids.Format(id.Int64)
}

// pgInt8 把可选 int64 转为 pgtype.Int8。
func pgInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: v > 0}
}

// pgInt2 把可选 int16 转为 pgtype.Int2。
func pgInt2(v int16) pgtype.Int2 {
	return pgtype.Int2{Int16: v, Valid: v > 0}
}

// pgText 把可选字符串转换为 pgtype.Text。
func pgText(v string) pgtype.Text {
	v = strings.TrimSpace(v)
	return pgtype.Text{String: v, Valid: v != ""}
}
