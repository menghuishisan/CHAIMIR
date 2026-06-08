// M6 业务规则:状态机、校验、迟交策略与成绩计算。
package teaching

import (
	"math"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

// GradeWeightInput 是成绩权重配置输入。
type GradeWeightInput struct {
	SourceType int16   `json:"source_type"`
	SourceRef  string  `json:"source_ref"`
	Weight     float64 `json:"weight"`
}

// WeightedScore 是某个成绩来源的得分与权重。
type WeightedScore struct {
	Score         float64
	OverrideScore *float64
	Weight        float64
}

// LateResult 是迟交策略应用后的结果。
type LateResult struct {
	IsLate     bool
	FinalScore int
}

// normalizeCourseListRole 归一化课程列表视角;空值按文档默认教师视角,非法值显式拒绝。
func normalizeCourseListRole(role string) (string, error) {
	switch strings.TrimSpace(role) {
	case "":
		return contracts.RoleTeacher, nil
	case contracts.RoleTeacher, contracts.RoleStudent:
		return role, nil
	default:
		return "", apperr.ErrCourseInvalid
	}
}

// validateCourseTransition 校验课程生命周期流转。
func validateCourseTransition(from, to int16) error {
	allowed := map[int16][]int16{
		CourseStatusDraft:     {CourseStatusPublished, CourseStatusArchived},
		CourseStatusPublished: {CourseStatusRunning, CourseStatusEnded, CourseStatusArchived},
		CourseStatusRunning:   {CourseStatusEnded, CourseStatusArchived},
		CourseStatusEnded:     {CourseStatusArchived},
	}
	for _, next := range allowed[from] {
		if next == to {
			return nil
		}
	}
	return apperr.ErrCourseInvalidState
}

// validateLessonContentRef 校验课时内容类型和对应引用字段。
func validateLessonContentRef(contentType int16, ref map[string]any) error {
	switch contentType {
	case LessonContentVideo, LessonContentAttachment:
		if strings.TrimSpace(stringValue(ref["storage_key"])) == "" {
			return apperr.ErrCourseInvalid
		}
	case LessonContentMarkdown:
		if strings.TrimSpace(stringValue(ref["markdown"])) == "" {
			return apperr.ErrCourseInvalid
		}
	case LessonContentExperiment:
		if strings.TrimSpace(stringValue(ref["experiment_id"])) == "" {
			return apperr.ErrCourseInvalid
		}
	case LessonContentSimulation:
		if strings.TrimSpace(stringValue(ref["package_code"])) == "" || strings.TrimSpace(stringValue(ref["version"])) == "" {
			return apperr.ErrCourseInvalid
		}
	default:
		return apperr.ErrCourseInvalid
	}
	return nil
}

// validateGradeWeights 校验同课程权重合计为 100%。
func validateGradeWeights(items []GradeWeightInput) error {
	if len(items) == 0 {
		return apperr.ErrGradeWeightInvalid
	}
	total := 0.0
	for _, item := range items {
		if item.Weight <= 0 {
			return apperr.ErrGradeWeightInvalid
		}
		total += item.Weight
	}
	if math.Abs(total-100) > 0.001 {
		return apperr.ErrGradeWeightInvalid
	}
	return nil
}

// validateGradeWeightSources 校验生产权重配置的来源字段。
func validateGradeWeightSources(items []GradeWeightInput) error {
	for _, item := range items {
		if strings.TrimSpace(item.SourceRef) == "" ||
			(item.SourceType != GradeSourceAssignment && item.SourceType != GradeSourceExperiment && item.SourceType != GradeSourceExam) {
			return apperr.ErrGradeWeightInvalid
		}
	}
	return nil
}

// applyLatePolicy 根据截止时间与策略决定是否拒收或扣分。
func applyLatePolicy(dueAt, submittedAt time.Time, policy int16, penalty map[string]any, score int) (LateResult, error) {
	isLate := submittedAt.After(dueAt)
	if !isLate {
		return LateResult{FinalScore: score}, nil
	}
	switch policy {
	case LatePolicyReject:
		return LateResult{}, apperr.ErrSubmissionLateRejected
	case LatePolicyNoPenalty:
		return LateResult{IsLate: true, FinalScore: score}, nil
	case LatePolicyDeduct:
		rate := numberValue(penalty["deduct_percent"])
		if rate <= 0 {
			rate = numberValue(penalty["percent"])
		}
		if rate < 0 || rate > 100 {
			return LateResult{}, apperr.ErrAssignmentInvalid
		}
		final := int(math.Round(float64(score) * (100 - rate) / 100))
		return LateResult{IsLate: true, FinalScore: final}, nil
	default:
		return LateResult{}, apperr.ErrAssignmentInvalid
	}
}

// computeWeightedTotal 计算加权总评,手动覆盖分优先。
func computeWeightedTotal(items []WeightedScore) (float64, error) {
	if len(items) == 0 {
		return 0, apperr.ErrGradeInvalid
	}
	total := 0.0
	for _, item := range items {
		score := item.Score
		if item.OverrideScore != nil {
			score = *item.OverrideScore
		}
		if item.Weight < 0 {
			return 0, apperr.ErrGradeInvalid
		}
		total += score * item.Weight / 100
	}
	return math.Round(total*100) / 100, nil
}

// validateScore 校验百分制分数。
func validateScore(score float64) error {
	if score < 0 || score > 100 {
		return apperr.ErrGradeInvalid
	}
	return nil
}

// numberValue 从 JSON 数字字段中提取 float64。
func numberValue(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

// stringValue 从 JSON 字段中提取字符串。
func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
