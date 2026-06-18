// content rules 文件集中实现 M5 输入校验和状态规则。
package content

import (
	"regexp"
	"strings"

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

// validVisibility 校验可见性。
func validVisibility(value int16) bool {
	return value == VisibilityPrivate || value == VisibilityTenant || value == VisibilityShared
}

// validDraftVisibility 校验草稿可见性,跨租户共享只能在发布后通过 share 状态流转进入。
func validDraftVisibility(value int16) bool {
	return value == VisibilityPrivate || value == VisibilityTenant
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
