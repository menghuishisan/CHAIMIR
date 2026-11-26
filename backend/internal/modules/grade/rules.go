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
	ordered := orderedMapping(mapping)
	var totals gpaTotals
	for _, grade := range grades {
		if err := totals.add(grade, ordered); err != nil {
			return 0, 0, err
		}
	}
	return totals.gpa(), totals.credits, nil
}

// orderedMapping 返回按分数阈值降序排列的等级映射副本,供绩点与不及格判定共用同一套档位语义。
func orderedMapping(mapping []LevelRule) []LevelRule {
	ordered := append([]LevelRule(nil), mapping...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Min > ordered[j].Min })
	return ordered
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

// gpaTotals 供完整明细和流式读取共用同一加权规则，避免学期平均值再次平均。
type gpaTotals struct{ weighted, credits float64 }

// add 使用已验证并排序的等级映射累加一门课程。
func (t *gpaTotals) add(grade CourseGradeInput, mapping []LevelRule) error {
	if grade.Credits <= 0 {
		return nil
	}
	point, ok := gpaPointForScore(grade.FinalTotal, mapping)
	if !ok {
		return apperr.ErrGradeConfigInvalid
	}
	t.weighted += point * grade.Credits
	t.credits += grade.Credits
	return nil
}

// gpa 仅在完成加权后舍入，避免学期舍入误差进入累计值。
func (t gpaTotals) gpa() float64 {
	if t.credits == 0 {
		return 0
	}
	return round3(t.weighted / t.credits)
}
