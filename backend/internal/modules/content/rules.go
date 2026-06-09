// M5 领域规则:集中维护校验、敏感字段过滤、克隆深拷贝与判题快照解析。
package content

import (
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// randomCriteria 是随机组卷条件的规范化结果。
type randomCriteria struct {
	Count           int
	Type            int16
	Difficulties    []int16
	KnowledgePoints []string
	Score           int32
}

// validateCreateItemRequest 校验内容创建请求。
func validateCreateItemRequest(req CreateItemRequest) error {
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Title) == "" || req.Body == nil {
		return apperr.ErrContentInvalid
	}
	if req.Version == "" {
		req.Version = initialVersion
	}
	if err := validateVersion(req.Version); err != nil {
		return err
	}
	if req.Type < ContentTypeExperimentTemplate || req.Type > ContentTypeTheoryQuestion ||
		req.Difficulty < DifficultyIntro || req.Difficulty > DifficultyResearch {
		return apperr.ErrContentInvalid
	}
	if req.Visibility < VisibilityPrivate || req.Visibility > VisibilityShared {
		return apperr.ErrContentInvalid
	}
	if req.AuthorType < AuthorTypeTeacher || req.AuthorType > AuthorTypeExternal {
		return apperr.ErrContentInvalid
	}
	return nil
}

// validateSystemImportRequest 校验内部系统建题来源,避免服务请求体伪装成教师手动创建。
func validateSystemImportRequest(req CreateItemRequest) error {
	if err := validateCreateItemRequest(req); err != nil {
		return err
	}
	if req.AuthorType != AuthorTypeSystem && req.AuthorType != AuthorTypeExternal {
		return apperr.ErrContentInvalid
	}
	if len(req.SystemImportNote) == 0 {
		return apperr.ErrContentInvalid
	}
	return nil
}

// canManageContent 判断当前服务端身份是否允许管理某内容。
func canManageContent(isPlatform bool, account contracts.AccountInfo, authorID int64) bool {
	if isPlatform {
		return true
	}
	if contracts.HasAnyRole(account.Roles, contracts.RoleSchoolAdmin) {
		return true
	}
	return account.AccountID == authorID
}

// canReadOwnContentFace 判断教师直连题库题面时是否满足内容可见性。
func canReadOwnContentFace(isPlatform bool, account contracts.AccountInfo, authorID int64, visibility int16) bool {
	if isPlatform || contracts.HasAnyRole(account.Roles, contracts.RoleSchoolAdmin) || account.AccountID == authorID {
		return true
	}
	return visibility == VisibilityTenant
}

// validateUpdateItemRequest 校验草稿编辑请求。
func validateUpdateItemRequest(req UpdateItemRequest) error {
	if strings.TrimSpace(req.Title) == "" || req.Body == nil {
		return apperr.ErrContentInvalid
	}
	if req.Difficulty < DifficultyIntro || req.Difficulty > DifficultyResearch ||
		req.Visibility < VisibilityPrivate || req.Visibility > VisibilityShared {
		return apperr.ErrContentInvalid
	}
	return nil
}

// validateDraftEditable 确认只有草稿版本允许直接编辑。
func validateDraftEditable(status int16) error {
	if status != ItemStatusDraft {
		return apperr.ErrContentImmutable
	}
	return nil
}

// validateDraftDeletable 区分草稿状态与引用占用,让删除失败能返回准确错误码。
func validateDraftDeletable(status int16, usageCount int32) error {
	if status != ItemStatusDraft {
		return apperr.ErrContentImmutable
	}
	if usageCount > 0 {
		return apperr.ErrContentDeleteBlocked
	}
	return nil
}

// validateVersion 校验版本号采用三段数字语义版本。
func validateVersion(version string) error {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return apperr.ErrContentVersionInvalid
	}
	for _, part := range parts {
		if part == "" {
			return apperr.ErrContentVersionInvalid
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return apperr.ErrContentVersionInvalid
			}
		}
	}
	return nil
}

// filterSensitiveBody 返回剥离敏感字段后的深拷贝内容体。
func filterSensitiveBody(body map[string]any, fields []string) map[string]any {
	out := jsonx.CloneObject(body)
	for _, field := range fields {
		removeJSONPath(out, field)
	}
	return out
}

// validateBodyHash 校验内容体哈希与外壳记录一致,防止版本内容被绕过服务层篡改。
func validateBodyHash(body []byte, expected string) error {
	if strings.TrimSpace(expected) == "" || bodyHash(body) != expected {
		return apperr.ErrContentIntegrity
	}
	return nil
}

// removeJSONPath 从对象中移除点分路径字段。
func removeJSONPath(body map[string]any, path string) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	if len(parts) == 0 || parts[0] == "" {
		return
	}
	current := body
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			return
		}
		current = next
	}
	delete(current, parts[len(parts)-1])
}

// buildCloneDraft 构造克隆草稿,重置归属、版本、可见性与复用统计。
func buildCloneDraft(source ItemDTO, authorID int64, newCode string) (ItemDTO, error) {
	if strings.TrimSpace(newCode) == "" || authorID <= 0 {
		return ItemDTO{}, apperr.ErrContentCloneInvalid
	}
	return ItemDTO{
		Code: newCode, Version: initialVersion, Type: source.Type, Title: source.Title,
		CategoryID: source.CategoryID, Difficulty: source.Difficulty, Tags: append([]string{}, source.Tags...),
		KnowledgePoints: append([]string{}, source.KnowledgePoints...), AuthorID: ids.Format(authorID), AuthorType: AuthorTypeTeacher,
		Visibility: VisibilityPrivate, Status: ItemStatusDraft, Body: jsonx.CloneObject(source.Body),
		SensitiveFields: append([]string{}, source.SensitiveFields...),
	}, nil
}

// judgeSpecFromItem 从已发布 full 内容解析 M3 判题快照。
func judgeSpecFromItem(item ItemDTO) (contracts.ContentJudgeSpec, error) {
	if item.Status != ItemStatusPublished {
		return contracts.ContentJudgeSpec{}, apperr.ErrContentUnavailable
	}
	raw, ok := item.Body["judge_config"].(map[string]any)
	if !ok {
		return contracts.ContentJudgeSpec{}, apperr.ErrContentInvalid
	}
	judgerCode, _ := raw["judger_code"].(string)
	suiteRef, _ := raw["suite_ref"].(string)
	if strings.TrimSpace(judgerCode) == "" {
		return contracts.ContentJudgeSpec{}, apperr.ErrContentInvalid
	}
	return contracts.ContentJudgeSpec{
		ItemCode: item.Code, ItemVersion: item.Version, JudgerCode: judgerCode,
		MaxScore: int32(jsonx.IntFromAny(raw["max_score"])), SuiteRef: suiteRef,
		Expectation: jsonx.ObjectFromAny(raw["expectation"]), VersionHash: item.BodyHash,
	}, nil
}

// normalizeRandomCriteria 把组卷条件规范化为 SQL 可直接使用的数组条件。
func normalizeRandomCriteria(criteria map[string]any) randomCriteria {
	out := randomCriteria{
		Count:           jsonx.IntFromAny(criteria["count"]),
		Type:            int16(jsonx.IntFromAny(criteria["type"])),
		Difficulties:    int16List(criteria["difficulty"]),
		KnowledgePoints: stringList(criteria["knowledge_points"]),
		Score:           int32(jsonx.IntFromAny(criteria["score"])),
	}
	return out
}

// int16List 支持单值或数组形式的 JSON 数字条件。
func int16List(v any) []int16 {
	switch x := v.(type) {
	case []any:
		out := make([]int16, 0, len(x))
		for _, item := range x {
			if n := int16(jsonx.IntFromAny(item)); n > 0 {
				out = append(out, n)
			}
		}
		return out
	case []int16:
		return append([]int16{}, x...)
	case []int:
		out := make([]int16, 0, len(x))
		for _, item := range x {
			if item > 0 {
				out = append(out, int16(item))
			}
		}
		return out
	default:
		if n := int16(jsonx.IntFromAny(v)); n > 0 {
			return []int16{n}
		}
		return nil
	}
}

// stringList 支持单值或数组形式的字符串条件。
func stringList(v any) []string {
	switch x := v.(type) {
	case string:
		if strings.TrimSpace(x) == "" {
			return nil
		}
		return []string{strings.TrimSpace(x)}
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if strings.TrimSpace(item) != "" {
				out = append(out, strings.TrimSpace(item))
			}
		}
		return out
	default:
		return nil
	}
}

// validateCategoryParent 校验分类父节点存在,且不会形成自引用或环。
func validateCategoryParent(categoryID, parentID int64, categories []CategoryDTO) error {
	if parentID <= 0 {
		return nil
	}
	if categoryID == parentID {
		return apperr.ErrContentCategoryInvalid
	}
	parentByID := make(map[int64]int64, len(categories))
	for _, category := range categories {
		id, ok := ids.Parse(category.ID)
		if !ok {
			return apperr.ErrContentCategoryInvalid
		}
		pid := int64(0)
		if category.ParentID != "" {
			parsed, ok := ids.Parse(category.ParentID)
			if !ok {
				return apperr.ErrContentCategoryInvalid
			}
			pid = parsed
		}
		parentByID[id] = pid
	}
	if _, ok := parentByID[parentID]; !ok {
		return apperr.ErrContentCategoryInvalid
	}
	for current := parentID; current > 0; current = parentByID[current] {
		if current == categoryID {
			return apperr.ErrContentCategoryInvalid
		}
		if _, ok := parentByID[current]; !ok {
			return apperr.ErrContentCategoryInvalid
		}
	}
	return nil
}

// createItemAuditDetail 构造内容创建审计详情,保留系统导入来源与预验证信息。
func createItemAuditDetail(req CreateItemRequest) map[string]any {
	detail := map[string]any{"code": req.Code, "version": req.Version}
	if len(req.SystemImportNote) > 0 {
		detail["system_import_note"] = jsonx.CloneObject(req.SystemImportNote)
	}
	return detail
}

// newVersionFrom 递增补丁版本号,用于未显式传版本时创建新版本草稿。
func newVersionFrom(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", apperr.ErrContentVersionInvalid
	}
	var nums [3]int
	for i, part := range parts {
		for _, r := range part {
			if r < '0' || r > '9' {
				return "", apperr.ErrContentVersionInvalid
			}
			nums[i] = nums[i]*10 + int(r-'0')
		}
	}
	nums[2]++
	return fmt.Sprintf("%d.%d.%d", nums[0], nums[1], nums[2]), nil
}

// resolveNewVersionPlan 从现有版本中确定发新版的源版本和目标版本,默认基于最高非草稿版本递增补丁号。
func resolveNewVersionPlan(versions []ItemDTO, req NewVersionRequest) (string, string, error) {
	sourceVersion := strings.TrimSpace(req.SourceVersion)
	targetVersion := strings.TrimSpace(req.Version)
	if sourceVersion == "" {
		latest, err := latestReleasedVersion(versions)
		if err != nil {
			return "", "", err
		}
		sourceVersion = latest
	}
	if err := validateVersion(sourceVersion); err != nil {
		return "", "", err
	}
	if targetVersion == "" {
		next, err := newVersionFrom(sourceVersion)
		if err != nil {
			return "", "", err
		}
		targetVersion = next
	}
	if err := validateVersion(targetVersion); err != nil {
		return "", "", err
	}
	return sourceVersion, targetVersion, nil
}

// latestReleasedVersion 选择当前最高已发布或已弃用版本,草稿不作为默认发新版基线。
func latestReleasedVersion(versions []ItemDTO) (string, error) {
	best := ""
	for _, item := range versions {
		if item.Status != ItemStatusPublished && item.Status != ItemStatusDeprecated {
			continue
		}
		if best == "" || compareVersion(item.Version, best) > 0 {
			best = item.Version
		}
	}
	if best == "" {
		return "", apperr.ErrContentVersionInvalid
	}
	return best, nil
}

// compareVersion 比较三段数字版本号,调用方先通过 validateVersion 保证格式。
func compareVersion(a, b string) int {
	ap := versionParts(a)
	bp := versionParts(b)
	for i := range ap {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	return 0
}

// versionParts 把已校验的三段数字版本转换为整数数组。
func versionParts(version string) [3]int {
	var out [3]int
	for i, part := range strings.Split(version, ".") {
		for _, r := range part {
			out[i] = out[i]*10 + int(r-'0')
		}
	}
	return out
}
