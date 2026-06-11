// content rules 文件集中实现 M5 输入校验和状态规则。
package content

import (
	"regexp"
	"strings"

	"chaimir/pkg/apperr"
)

var (
	contentCodeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{2,95}$`)
	semverRe      = regexp.MustCompile(`^v?[0-9]+(\.[0-9]+){1,2}(-[0-9A-Za-z.-]+)?$`)
)

// validateCreateRequest 校验教师创建草稿请求。
func validateCreateRequest(req CreateItemRequest) (CreateItemRequest, error) {
	req.Code = strings.TrimSpace(req.Code)
	req.Version = strings.TrimSpace(req.Version)
	req.Title = strings.TrimSpace(req.Title)
	req.Tags = normalizedStrings(req.Tags)
	req.KnowledgePoints = normalizedStrings(req.KnowledgePoints)
	req.SensitiveFields = normalizedStrings(append(req.SensitiveFields, defaultSensitivePaths...))
	if !validCode(req.Code) || !validVersion(req.Version) || !validType(req.Type) || req.Title == "" || !validDifficulty(req.Difficulty) || !validVisibility(req.Visibility) || req.Body == nil {
		return CreateItemRequest{}, apperr.ErrContentInvalid
	}
	return req, nil
}

// validateUpdateRequest 校验草稿编辑请求。
func validateUpdateRequest(req UpdateItemRequest) (UpdateItemRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Tags = normalizedStrings(req.Tags)
	req.KnowledgePoints = normalizedStrings(req.KnowledgePoints)
	req.SensitiveFields = normalizedStrings(append(req.SensitiveFields, defaultSensitivePaths...))
	if req.Title == "" || !validDifficulty(req.Difficulty) || !validVisibility(req.Visibility) || req.Body == nil {
		return UpdateItemRequest{}, apperr.ErrContentInvalid
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

// normalizePage 将分页限制在平台统一的常用范围内。
func normalizePage(page, size *int) {
	if *page <= 0 {
		*page = 1
	}
	if *size <= 0 {
		*size = 20
	}
	if *size > 100 {
		*size = 100
	}
}
