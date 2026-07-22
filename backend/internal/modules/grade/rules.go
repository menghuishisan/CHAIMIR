// grade rules 文件集中实现 M11 GPA、申诉时效和输入校验规则。
package grade

import (
	"math"
	"sort"
	"strings"
	"time"

	"chaimir/pkg/apperr"
)

// ComputeGPA 按等级映射和学分计算学分加权 GPA。
func ComputeGPA(grades []CourseGradeInput, mapping []LevelRule) (float64, float64, error) {
	if err := validateGPAMapping(mapping); err != nil {
		return 0, 0, apperr.ErrGradeConfigInvalid
	}
	ordered := append([]LevelRule(nil), mapping...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Min > ordered[j].Min })
	var weighted float64
	var credits float64
	for _, grade := range grades {
		if grade.Credits <= 0 {
			continue
		}
		point, ok := gpaPointForScore(grade.FinalTotal, ordered)
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

// validateGPAMapping 拒绝缺档、重复阈值和非不及格档的零绩点配置。
func validateGPAMapping(mapping []LevelRule) error {
	if len(mapping) == 0 {
		return apperr.ErrGradeConfigInvalid
	}
	seen := make(map[float64]struct{}, len(mapping))
	hasFloor := false
	for _, rule := range mapping {
		if rule.Min < 0 || rule.Min > 100 || strings.TrimSpace(rule.Grade) == "" || rule.GPA < 0 || rule.GPA > 4 {
			return apperr.ErrGradeConfigInvalid
		}
		if rule.Min > 0 && rule.GPA == 0 {
			return apperr.ErrGradeConfigInvalid
		}
		if _, exists := seen[rule.Min]; exists {
			return apperr.ErrGradeConfigInvalid
		}
		seen[rule.Min] = struct{}{}
		hasFloor = hasFloor || rule.Min == 0
	}
	if !hasFloor {
		return apperr.ErrGradeConfigInvalid
	}
	return nil
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
