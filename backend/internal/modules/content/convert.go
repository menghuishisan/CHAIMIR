// M5 转换层:处理领域 DTO、contracts DTO 与 HTTP 输出结构之间的纯转换。
package content

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"crypto/sha256"
	"encoding/hex"
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
		Title: row.Title, CategoryID: pgtypex.IDString(row.CategoryID), Difficulty: row.Difficulty, Tags: row.Tags,
		KnowledgePoints: row.KnowledgePoints, AuthorID: ids.Format(row.AuthorID), AuthorType: row.AuthorType,
		Visibility: row.Visibility, Status: row.Status, UsageCount: row.UsageCount, BodyHash: row.BodyHash,
		Body: body, SensitiveFields: fields}
	return dto, nil
}

// paperItemDTOFromRepoRow 转换 repo 试卷题目最小行。
func paperItemDTOFromRepoRow(row paperItemRow, item ItemDTO) PaperItemDTO {
	return PaperItemDTO{ID: ids.Format(row.ID), Code: row.ItemCode, Version: row.ItemVersion, Score: row.Score, Seq: row.Seq, Item: item}
}

// contractSnapshotFromItem 把内容 DTO 转换为跨模块内容快照。
func contractSnapshotFromItem(dto ItemDTO) contracts.ContentItemSnapshot {
	return contracts.ContentItemSnapshot{ItemCode: dto.Code, ItemVersion: dto.Version, Type: dto.Type, Title: dto.Title,
		Difficulty: dto.Difficulty, Tags: dto.Tags, KnowledgePoints: dto.KnowledgePoints, Body: dto.Body,
		VersionHash: dto.BodyHash, Status: dto.Status}
}

// contractSnapshotsFromItems 批量转换内容 DTO 为跨模块内容快照。
func contractSnapshotsFromItems(items []ItemDTO) []contracts.ContentItemSnapshot {
	out := make([]contracts.ContentItemSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, contractSnapshotFromItem(item))
	}
	return out
}

// contractRefsToItemRefs 把跨模块内容引用转换为 M5 内部批量请求引用。
func contractRefsToItemRefs(refs []contracts.ContentItemRef) []ItemRef {
	out := make([]ItemRef, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ItemRef{Code: ref.ItemCode, Version: ref.ItemVersion})
	}
	return out
}

// bodyHash 计算内容体哈希,用于版本完整性校验。
func bodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
