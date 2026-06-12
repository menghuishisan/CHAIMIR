// grade rules 文件集中实现 M11 GPA、申诉时效和输入校验规则。
package grade

import (
	"math"
	"sort"
	"time"

	"chaimir/pkg/apperr"
)

// ComputeGPA 按等级映射和学分计算学分加权 GPA。
func ComputeGPA(grades []CourseGradeInput, mapping []LevelRule) (float64, float64, error) {
	if len(mapping) == 0 {
		return 0, 0, apperr.ErrGradeConfigInvalid
	}
	sort.Slice(mapping, func(i, j int) bool { return mapping[i].Min > mapping[j].Min })
	var weighted float64
	var credits float64
	for _, grade := range grades {
		if grade.Credits <= 0 {
			continue
		}
		point, ok := gpaPointForScore(grade.FinalTotal, mapping)
		if !ok {
			return 0, 0, apperr.ErrGradeConfigInvalid
		}
		weighted += point * grade.Credits
		credits += grade.Credits
	}
	if credits == 0 {
		return 0, 0, nil
	}
	return round3(weighted / credits), credits, nil
}

// EnsureAppealWithinWindow 校验申诉是否仍在受理期限内。
func EnsureAppealWithinWindow(reviewedAt, now time.Time, windowDays int) error {
	if windowDays <= 0 {
		return apperr.ErrGradeConfigInvalid
	}
	if reviewedAt.IsZero() || now.After(reviewedAt.AddDate(0, 0, windowDays)) {
		return apperr.ErrGradeAppealExpired
	}
	return nil
}

// gpaPointForScore 找出分数命中的绩点规则。
func gpaPointForScore(score float64, mapping []LevelRule) (float64, bool) {
	for _, rule := range mapping {
		if score >= rule.Min {
			return rule.GPA, true
		}
	}
	return 0, false
}

// round3 把 GPA 统一保留三位小数。
func round3(value float64) float64 {
	return math.Round(value*1000) / 1000
}
