// M7 业务规则:实验组件校验、实例状态机与得分汇总。
package experiment

import (
	"errors"
	"strings"

	"chaimir/pkg/apperr"
)

// validateExperimentComponents 校验实验至少可运行且检查点分值不超过 100。
func validateExperimentComponents(components ExperimentComponents) error {
	issues := validateExperimentComponentsDetailed(components)
	for _, issue := range issues {
		if issue.Level == "error" {
			return apperr.ErrExperimentInvalid.WithCause(errors.New(issue.Message))
		}
	}
	return nil
}

// validateExperimentComponentsDetailed 返回面向发布前校验接口的结构化问题列表。
func validateExperimentComponentsDetailed(components ExperimentComponents) []ValidationIssue {
	var issues []ValidationIssue
	if len(components.Envs) == 0 && len(components.Sims) == 0 {
		issues = append(issues, ValidationIssue{Level: "error", Message: "实验至少需要一个沙箱环境或仿真组件"})
	}
	for _, env := range components.Envs {
		if strings.TrimSpace(env.RuntimeCode) == "" {
			issues = append(issues, ValidationIssue{Level: "error", Message: "沙箱环境缺少运行时配置"})
		}
	}
	for _, sim := range components.Sims {
		if strings.TrimSpace(sim.PackageCode) == "" || strings.TrimSpace(sim.Version) == "" {
			issues = append(issues, ValidationIssue{Level: "error", Message: "仿真组件缺少仿真包或版本"})
		}
	}
	var total float64
	seen := map[string]bool{}
	for _, cp := range components.Checkpoints {
		if strings.TrimSpace(cp.ID) == "" || strings.TrimSpace(cp.JudgerCode) == "" || strings.TrimSpace(cp.ItemCode) == "" || strings.TrimSpace(cp.ItemVersion) == "" {
			issues = append(issues, ValidationIssue{Level: "error", Message: "检查点配置不完整"})
		}
		if seen[cp.ID] {
			issues = append(issues, ValidationIssue{Level: "error", Message: "检查点编号不能重复"})
		}
		seen[cp.ID] = true
		if cp.Score < 0 {
			issues = append(issues, ValidationIssue{Level: "error", Message: "检查点分值不能为负数"})
		}
		total += cp.Score
	}
	if total > 100 {
		issues = append(issues, ValidationIssue{Level: "error", Message: "检查点分值合计不能超过 100"})
	}
	if len(components.Checkpoints) > 0 && total != 100 {
		issues = append(issues, ValidationIssue{Level: "warning", Message: "检查点分值合计不是 100,请确认报告分或人工评分配置"})
	}
	return issues
}

// validateExperimentRequest 校验实验定义的基础字段和组件编排。
func validateExperimentRequest(req ExperimentRequest) error {
	if err := validateExperimentDraftRequest(req); err != nil {
		return err
	}
	return validateExperimentComponents(req.Components)
}

// validateExperimentDraftRequest 校验向导草稿保存所需的最小字段。
func validateExperimentDraftRequest(req ExperimentRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return apperr.ErrExperimentInvalid
	}
	if req.CollabMode == 0 {
		req.CollabMode = CollabModeSingle
	}
	if req.CollabMode != CollabModeSingle && req.CollabMode != CollabModeGroup {
		return apperr.ErrExperimentInvalid
	}
	if req.WizardStep < 1 || req.WizardStep > 6 {
		return apperr.ErrExperimentInvalid
	}
	return nil
}

// validateInstanceTransition 校验实例状态流转符合文档状态机。
func validateInstanceTransition(from, to int16) error {
	allowed := map[int16][]int16{
		InstanceStatusCreating:  {InstanceStatusRunning, InstanceStatusError},
		InstanceStatusRunning:   {InstanceStatusPaused, InstanceStatusCompleted, InstanceStatusRecycled, InstanceStatusReleased},
		InstanceStatusPaused:    {InstanceStatusRunning, InstanceStatusRecycled, InstanceStatusReleased},
		InstanceStatusCompleted: {InstanceStatusRecycled},
		InstanceStatusError:     {InstanceStatusRecycled},
		InstanceStatusReleased:  {InstanceStatusRunning, InstanceStatusRecycled},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return nil
		}
	}
	return apperr.ErrExperimentInstanceState
}

// computeExperimentScore 汇总检查点与报告分,并确保总分在 0..100 范围内。
func computeExperimentScore(parts []ScorePart, reportScore *float64) (float64, error) {
	var total float64
	for _, part := range parts {
		if part.Score < 0 {
			return 0, apperr.ErrExperimentScoreInvalid
		}
		total += part.Score
	}
	if reportScore != nil {
		if *reportScore < 0 {
			return 0, apperr.ErrExperimentScoreInvalid
		}
		total += *reportScore
	}
	if total > 100 {
		return 0, apperr.ErrExperimentScoreInvalid
	}
	return total, nil
}
