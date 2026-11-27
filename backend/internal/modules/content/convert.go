package content

import (
	"sort"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
	"chaimir/pkg/jsonvalue"
)

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
		CreatedAt:       timex.RFC3339OrEmpty(item.CreatedAt),
		UpdatedAt:       timex.RFC3339OrEmpty(item.UpdatedAt),
	}
}

// itemSnapshotDTO 转换内容快照为 HTTP DTO,full 响应可携带敏感路径清单。
func itemSnapshotDTO(item ItemWithBody, includeSensitivePaths bool) (ItemSnapshotDTO, error) {
	body, err := jsonx.CloneObjectStrict(item.Body)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	out := ItemSnapshotDTO{ItemDTO: itemDTO(item.Item), Body: body}
	if includeSensitivePaths {
		out.SensitiveFields = cloneStrings(item.SensitiveFields)
	}
	return out, nil
}

func contractSnapshot(item ItemWithBody) (contracts.ContentItemSnapshot, error) {
	body, err := jsonx.CloneObjectStrict(item.Body)
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

func categoryDTO(category Category) CategoryDTO {
	return CategoryDTO{ID: ids.ID(category.ID), ParentID: ids.ID(category.ParentID), Name: category.Name, Sort: category.Sort, CreatedAt: timex.RFC3339OrEmpty(category.CreatedAt), UpdatedAt: timex.RFC3339OrEmpty(category.UpdatedAt)}
}

func paperDTO(paper Paper) PaperDTO {
	return PaperDTO{ID: ids.ID(paper.ID), Name: paper.Name, AuthorID: ids.ID(paper.AuthorID), GenMode: paper.GenMode, GenCriteria: paper.GenCriteria, CreatedAt: timex.RFC3339OrEmpty(paper.CreatedAt), UpdatedAt: timex.RFC3339OrEmpty(paper.UpdatedAt)}
}

func paperDetailDTO(detail PaperWithItems) (PaperDetailDTO, error) {
	items := make([]PaperItemFaceDTO, 0, len(detail.Items))
	for _, item := range detail.Items {
		body, err := jsonx.CloneObjectStrict(item.Body)
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

// versionHash 对不可变版本内容生成摘要,可见范围单独鉴权。
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
		"body":             body,
		"sensitive_fields": normalizedStrings(sensitive),
	}
	raw, err := jsonvalue.Canonical(payload)
	if err != nil {
		return "", apperr.ErrInternal.WithCause(err)
	}
	return crypto.SHA256Hex(raw), nil
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

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
