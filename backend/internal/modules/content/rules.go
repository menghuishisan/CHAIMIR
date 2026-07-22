// content rules 文件集中实现 M5 输入校验和状态规则。
package content

import (
	"regexp"
	"slices"
	"strings"

	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/pkg/apperr"
)

const contentBodyMaxInlineStringBytes = 16 * 1024

var (
	contentCodeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{2,95}$`)
	semverRe      = regexp.MustCompile(`^v?[0-9]+(\.[0-9]+){1,2}(-[0-9A-Za-z.-]+)?$`)
)

// validateCreateRequest 校验教师创建草稿请求,共享库发布必须通过显式 share 流程。
func validateCreateRequest(req CreateItemRequest) (CreateItemRequest, error) {
	req.Code = strings.TrimSpace(req.Code)
	req.Version = strings.TrimSpace(req.Version)
	req.Title = strings.TrimSpace(req.Title)
	req.Tags = normalizedStrings(req.Tags)
	req.KnowledgePoints = normalizedStrings(req.KnowledgePoints)
	req.SensitiveFields = normalizedStrings(append(req.SensitiveFields, defaultSensitivePaths...))
	if !validCode(req.Code) || !validVersion(req.Version) || !validType(req.Type) || req.Title == "" || !validDifficulty(req.Difficulty) || !validDraftVisibility(req.Visibility) || req.Body == nil {
		return CreateItemRequest{}, apperr.ErrContentInvalid
	}
	body, err := jsonx.CloneObjectStrict(req.Body)
	if err != nil {
		return CreateItemRequest{}, apperr.ErrContentBodyInvalid.WithCause(err)
	}
	req.Body = body
	if err := validateContentBody(req.Type, req.Body); err != nil {
		return CreateItemRequest{}, err
	}
	if err := validateContentBodyRefs(req.Body); err != nil {
		return CreateItemRequest{}, err
	}
	return req, nil
}

// validateUpdateRequest 校验草稿编辑请求,草稿不能直接进入跨租户共享库。
func validateUpdateRequest(req UpdateItemRequest) (UpdateItemRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Tags = normalizedStrings(req.Tags)
	req.KnowledgePoints = normalizedStrings(req.KnowledgePoints)
	req.SensitiveFields = normalizedStrings(append(req.SensitiveFields, defaultSensitivePaths...))
	if req.Title == "" || !validDifficulty(req.Difficulty) || !validDraftVisibility(req.Visibility) || req.Body == nil {
		return UpdateItemRequest{}, apperr.ErrContentInvalid
	}
	body, err := jsonx.CloneObjectStrict(req.Body)
	if err != nil {
		return UpdateItemRequest{}, apperr.ErrContentBodyInvalid.WithCause(err)
	}
	req.Body = body
	if err := validateContentBodyRefs(req.Body); err != nil {
		return UpdateItemRequest{}, err
	}
	return req, nil
}

// validateSystemImport 校验内部系统建题请求,禁止伪装教师来源。
func validateSystemImport(req SystemImportRequest) (SystemImportRequest, error) {
	create, err := validateCreateRequest(CreateItemRequest{Code: req.Code, Version: req.Version, Type: req.Type, Title: req.Title, CategoryID: req.CategoryID, Difficulty: req.Difficulty, Tags: req.Tags, KnowledgePoints: req.KnowledgePoints, Visibility: req.Visibility, Body: req.Body, SensitiveFields: req.SensitiveFields})
	if err != nil {
		return SystemImportRequest{}, apperr.ErrContentSystemImportInvalid.WithCause(err)
	}
	authorType := req.AuthorType
	if authorType == 0 {
		authorType = AuthorSystem
	}
	if authorType != AuthorSystem && authorType != AuthorExternal {
		return SystemImportRequest{}, apperr.ErrContentSystemImportInvalid
	}
	if req.AuthorID <= 0 {
		return SystemImportRequest{}, apperr.ErrContentSystemImportInvalid
	}
	req.Code = create.Code
	req.Version = create.Version
	req.Type = create.Type
	req.Title = create.Title
	req.CategoryID = create.CategoryID
	req.Difficulty = create.Difficulty
	req.Tags = create.Tags
	req.KnowledgePoints = create.KnowledgePoints
	req.Visibility = create.Visibility
	req.Body = create.Body
	req.SensitiveFields = create.SensitiveFields
	req.AuthorType = authorType
	return req, nil
}

// validatePaperRequest 校验组卷请求。
func validatePaperRequest(req CreatePaperRequest) (CreatePaperRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || (req.GenMode != PaperModeManual && req.GenMode != PaperModeRandom) {
		return CreatePaperRequest{}, apperr.ErrPaperInvalid
	}
	if req.GenMode == PaperModeManual {
		if len(req.Items) == 0 {
			return CreatePaperRequest{}, apperr.ErrPaperItemInvalid
		}
		for i := range req.Items {
			req.Items[i].Code = strings.TrimSpace(req.Items[i].Code)
			req.Items[i].Version = strings.TrimSpace(req.Items[i].Version)
			if !validCode(req.Items[i].Code) || !validVersion(req.Items[i].Version) || req.Items[i].Score <= 0 {
				return CreatePaperRequest{}, apperr.ErrPaperItemInvalid
			}
		}
		return req, nil
	}
	req.GenCriteria.KnowledgePoints = normalizedStrings(req.GenCriteria.KnowledgePoints)
	if req.GenCriteria.Count <= 0 || req.GenCriteria.DefaultScore <= 0 {
		return CreatePaperRequest{}, apperr.ErrPaperInvalid
	}
	if req.GenCriteria.Type != 0 && !validType(req.GenCriteria.Type) {
		return CreatePaperRequest{}, apperr.ErrPaperInvalid
	}
	for _, difficulty := range req.GenCriteria.Difficulties {
		if !validDifficulty(difficulty) {
			return CreatePaperRequest{}, apperr.ErrPaperInvalid
		}
	}
	return req, nil
}

// validateCategoryRequest 校验分类请求。
func validateCategoryRequest(req CategoryRequest) (CategoryRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len([]rune(req.Name)) > 128 || req.ParentID < 0 {
		return CategoryRequest{}, apperr.ErrContentCategoryInvalid
	}
	return req, nil
}

// validCode 校验内容 code。
func validCode(value string) bool {
	return contentCodeRe.MatchString(strings.TrimSpace(value))
}

// validVersion 校验 semver 版本。
func validVersion(value string) bool {
	return semverRe.MatchString(strings.TrimSpace(value))
}

// validType 校验内容类型。
func validType(value int16) bool {
	return value == TypeExperimentTemplate || value == TypeContestProblem || value == TypeTheoryQuestion
}

// validDifficulty 校验难度。
func validDifficulty(value int16) bool {
	return value >= DifficultyIntro && value <= DifficultyChallenge
}

// validDraftVisibility 校验草稿可见性,跨租户共享只能在发布后通过 share 状态流转进入。
func validDraftVisibility(value int16) bool {
	return value == VisibilityPrivate || value == VisibilityTenant
}

// validateContentBody 按内容类型校验当前唯一结构，旧字段和未知顶层字段一律拒绝。
func validateContentBody(contentType int16, body map[string]any) error {
	var valid bool
	switch contentType {
	case TypeExperimentTemplate:
		valid = validateExperimentTemplateBody(body)
	case TypeContestProblem:
		valid = validateContestProblemBody(body)
	case TypeTheoryQuestion:
		valid = validateTheoryQuestionBody(body)
	}
	if !valid {
		return apperr.ErrContentBodyInvalid
	}
	return nil
}

// validateExperimentTemplateBody 校验实验模板专有字段。
func validateExperimentTemplateBody(body map[string]any) bool {
	if !jsonx.HasOnlyKeys(body, "runtime_code", "tools", "init_code_ref", "sim_package_ref", "judge_config", "description", "init_script") {
		return false
	}
	return nonEmptyString(body["runtime_code"]) && nonEmptyString(body["description"]) && stringValue(body["init_code_ref"]) && stringValue(body["sim_package_ref"]) && stringValue(body["init_script"]) && stringArray(body["tools"]) && validateJudgeConfig(jsonx.ObjectFromAny(body["judge_config"]))
}

// validateContestProblemBody 校验竞赛题专有字段。
func validateContestProblemBody(body map[string]any) bool {
	if !jsonx.HasOnlyKeys(body, "statement", "judge_config", "init_contracts", "ad_config") || !nonEmptyString(body["statement"]) || !stringArray(body["init_contracts"]) || !validateJudgeConfig(jsonx.ObjectFromAny(body["judge_config"])) {
		return false
	}
	if raw, exists := body["ad_config"]; exists && raw != nil {
		config := jsonx.ObjectFromAny(raw)
		return jsonx.HasOnlyKeys(config, "runtime_code", "runtime_image_version", "tool_codes") && nonEmptyString(config["runtime_code"]) && nonEmptyString(config["runtime_image_version"]) && stringArray(config["tool_codes"])
	}
	return true
}

// validateTheoryQuestionBody 校验理论题专有字段和题型答案结构。
func validateTheoryQuestionBody(body map[string]any) bool {
	if !jsonx.HasOnlyKeys(body, "statement", "q_type", "options", "answer", "explanation") || !nonEmptyString(body["statement"]) || !stringValue(body["explanation"]) {
		return false
	}
	qType, ok := body["q_type"].(string)
	if !ok || !slices.Contains([]string{"single_choice", "multiple_choice", "true_false", "fill_blank", "short_answer"}, qType) {
		return false
	}
	options, ok := body["options"].([]any)
	if !ok {
		return false
	}
	if (qType == "single_choice" || qType == "multiple_choice") && (len(options) < 2 || !stringArray(options)) {
		return false
	}
	if qType != "single_choice" && qType != "multiple_choice" && len(options) != 0 {
		return false
	}
	switch qType {
	case "multiple_choice":
		return stringArray(body["answer"])
	case "true_false":
		_, ok := body["answer"].(bool)
		return ok
	default:
		return nonEmptyString(body["answer"])
	}
}

// validateJudgeConfig 校验所有内容类型共用的判题配置外壳和受控 expectation 字段。
func validateJudgeConfig(config map[string]any) bool {
	maxScore, maxScoreOK := jsonx.Int32FromNumberOK(config["max_score"])
	if !jsonx.HasOnlyKeys(config, "judger_code", "suite_ref", "max_score", "expectation") || !nonEmptyString(config["judger_code"]) || !optionalString(config, "suite_ref") || !maxScoreOK || maxScore <= 0 {
		return false
	}
	expectation, ok := config["expectation"].(map[string]any)
	return ok && expectation != nil
}

// stringValue 判断值是否为字符串，允许当前契约中的空可选文本。
func stringValue(value any) bool {
	_, ok := value.(string)
	return ok
}

// nonEmptyString 判断值是否为非空字符串。
func nonEmptyString(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

// optionalString 校验可省略字符串字段。
func optionalString(value map[string]any, key string) bool {
	raw, exists := value[key]
	return !exists || raw == nil || stringValue(raw)
}

// stringArray 校验已归一化 JSON 字符串数组。
func stringArray(value any) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if !nonEmptyString(item) {
			return false
		}
	}
	return true
}

// validateContentBodyRefs 拒绝正文内联大文件、data URL 和外部直链,附件必须走统一文件服务对象引用。
func validateContentBodyRefs(body map[string]any) error {
	return walkContentBody(body)
}

// walkContentBody 递归检查 JSON 正文中的字符串字段是否越过文件服务边界。
func walkContentBody(value any) error {
	switch v := value.(type) {
	case map[string]any:
		for _, child := range v {
			if err := walkContentBody(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range v {
			if err := walkContentBody(child); err != nil {
				return err
			}
		}
	case string:
		if unsafeInlineBodyString(v) {
			return apperr.ErrContentBodyInvalid
		}
		if err := validateObjectRefString(v); err != nil {
			return err
		}
	}
	return nil
}

// unsafeInlineBodyString 判断字符串是否疑似把附件或外部资源直接塞进正文。
func unsafeInlineBodyString(value string) bool {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return true
	}
	return len([]byte(trimmed)) > contentBodyMaxInlineStringBytes
}

// validateObjectRefString 要求正文中的对象引用必须符合统一文件服务格式。
func validateObjectRefString(value string) error {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(trimmed), "minio://") {
		return nil
	}
	if _, err := storage.ParseObjectRef(trimmed); err != nil {
		return apperr.ErrContentBodyInvalid.WithCause(err)
	}
	return nil
}
