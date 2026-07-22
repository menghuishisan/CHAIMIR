// content convert 文件负责领域模型、DTO 与 contracts 快照之间的纯转换。
package content

import (
	"sort"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
)

// itemDTO 转换内容外壳为 HTTP DTO。
func itemDTO(item Item) ItemDTO {
	return ItemDTO{
		ID:              ids.ID(item.ID),
		TenantID:        ids.ID(item.TenantID),
		Code:            item.Code,
		Version:         item.Version,
		Type:            item.Type,
		Title:           item.Title,
		CategoryID:      ids.ID(item.CategoryID),
		Difficulty:      item.Difficulty,
		Tags:            cloneStrings(item.Tags),
		KnowledgePoints: cloneStrings(item.KnowledgePoints),
		AuthorID:        ids.ID(item.AuthorID),
		AuthorType:      item.AuthorType,
		Visibility:      item.Visibility,
		Status:          item.Status,
		UsageCount:      item.UsageCount,
		VersionHash:     item.VersionHash,
		CreatedAt:       formatTime(item.CreatedAt),
		UpdatedAt:       formatTime(item.UpdatedAt),
	}
}

// itemSnapshotDTO 转换内容快照为 HTTP DTO,full 响应可携带敏感路径清单。
func itemSnapshotDTO(item ItemWithBody, includeSensitivePaths bool) (ItemSnapshotDTO, error) {
	body, err := cloneMapStrict(item.Body)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	out := ItemSnapshotDTO{ItemDTO: itemDTO(item.Item), Body: body}
	if includeSensitivePaths {
		out.SensitiveFields = cloneStrings(item.SensitiveFields)
	}
	return out, nil
}

// contractSnapshot 转换为跨模块内容快照。
func contractSnapshot(item ItemWithBody) (contracts.ContentItemSnapshot, error) {
	body, err := cloneMapStrict(item.Body)
	if err != nil {
		return contracts.ContentItemSnapshot{}, err
	}
	return contracts.ContentItemSnapshot{
		ItemCode:        item.Code,
		ItemVersion:     item.Version,
		Type:            item.Type,
		Title:           item.Title,
		Difficulty:      item.Difficulty,
		Visibility:      item.Visibility,
		Tags:            cloneStrings(item.Tags),
		KnowledgePoints: cloneStrings(item.KnowledgePoints),
		Body:            body,
		VersionHash:     item.VersionHash,
		Status:          item.Status,
	}, nil
}

// categoryDTO 转换分类响应。
func categoryDTO(category Category) CategoryDTO {
	return CategoryDTO{ID: ids.ID(category.ID), ParentID: ids.ID(category.ParentID), Name: category.Name, Sort: category.Sort, CreatedAt: formatTime(category.CreatedAt), UpdatedAt: formatTime(category.UpdatedAt)}
}

// paperDTO 转换试卷响应。
func paperDTO(paper Paper) PaperDTO {
	return PaperDTO{ID: ids.ID(paper.ID), Name: paper.Name, AuthorID: ids.ID(paper.AuthorID), GenMode: paper.GenMode, GenCriteria: paper.GenCriteria, CreatedAt: formatTime(paper.CreatedAt), UpdatedAt: formatTime(paper.UpdatedAt)}
}

// paperDetailDTO 转换试卷详情响应。
func paperDetailDTO(detail PaperWithItems) (PaperDetailDTO, error) {
	items := make([]PaperItemFaceDTO, 0, len(detail.Items))
	for _, item := range detail.Items {
		body, err := cloneMapStrict(item.Body)
		if err != nil {
			return PaperDetailDTO{}, err
		}
		items = append(items, PaperItemFaceDTO{
			ID:      ids.ID(item.ID),
			Code:    item.ItemCode,
			Version: item.ItemVersion,
			Score:   item.Score,
			Seq:     item.Seq,
			Item:    item.Item,
			Body:    body,
		})
	}
	return PaperDetailDTO{Paper: paperDTO(detail.Paper), Items: items}, nil
}

// versionHash 对外壳关键字段和正文生成稳定 SHA-256 摘要,用于发布版本完整性校验。
func versionHash(item Item, body map[string]any, sensitive []string) (string, error) {
	payload := map[string]any{
		"code":             item.Code,
		"version":          item.Version,
		"type":             item.Type,
		"title":            item.Title,
		"category_id":      item.CategoryID,
		"difficulty":       item.Difficulty,
		"tags":             normalizedStrings(item.Tags),
		"knowledge_points": normalizedStrings(item.KnowledgePoints),
		"visibility":       item.Visibility,
		"body":             body,
		"sensitive_fields": normalizedStrings(sensitive),
	}
	raw, err := jsonx.AnyBytes(payload, apperr.ErrInternal)
	if err != nil {
		return "", err
	}
	return crypto.SHA256Hex(raw), nil
}

// cloneMapStrict 深拷贝 JSON 对象,转换失败时显式返回错误避免坏正文伪装为空对象。
func cloneMapStrict(in map[string]any) (map[string]any, error) {
	return jsonx.CloneObjectStrict(in)
}

// cloneStrings 拷贝字符串切片。
func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

// normalizedStrings 去重排序后返回稳定字符串切片。
func normalizedStrings(in []string) []string {
	seen := map[string]struct{}{}
	for _, value := range in {
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// formatTime 输出统一 RFC3339 时间字符串。
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
