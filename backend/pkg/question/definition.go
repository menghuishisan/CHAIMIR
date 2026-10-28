// Package question 定义题库与评测共用的理论题形状校验,不实现评分或读取持久化答案。
package question

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Kind 是理论题的唯一题型枚举,与M5正文q_type保持一致。
type Kind string

const (
	SingleChoice   Kind = "single_choice"
	MultipleChoice Kind = "multiple_choice"
	TrueFalse      Kind = "true_false"
	FillBlank      Kind = "fill_blank"
	ShortAnswer    Kind = "short_answer"
)

// Definition 只承载理论题定义,Answer只能供M5作者与M3私有执行边界使用。
type Definition struct {
	Kind       Kind            `json:"q_type"`
	Options    []string        `json:"options"`
	BlankCount int             `json:"blank_count"`
	MaxScore   int32           `json:"max_score"`
	Answer     json.RawMessage `json:"answer"`
}

// Decode 读取已登记字段并校验题面及可选黑盒答案,不猜测缺失题型或满分。
func Decode(body map[string]any, requireAnswer bool) (Definition, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return Definition{}, fmt.Errorf("encode theory definition: %w", err)
	}
	var d Definition
	if err := json.Unmarshal(raw, &d); err != nil {
		return Definition{}, fmt.Errorf("decode theory definition: %w", err)
	}
	if d.MaxScore <= 0 {
		return Definition{}, fmt.Errorf("theory max_score must be positive")
	}
	switch d.Kind {
	case SingleChoice, MultipleChoice:
		if len(d.Options) < 2 {
			return Definition{}, fmt.Errorf("choice question requires options")
		}
		seen := make(map[string]bool, len(d.Options))
		for _, option := range d.Options {
			if strings.TrimSpace(option) == "" || strings.TrimSpace(option) != option || seen[option] {
				return Definition{}, fmt.Errorf("choice options must be unique nonempty trimmed text")
			}
			seen[option] = true
		}
		if d.BlankCount != 0 {
			return Definition{}, fmt.Errorf("choice question cannot declare blank_count")
		}
	case FillBlank:
		if d.BlankCount <= 0 || len(d.Options) > 0 {
			return Definition{}, fmt.Errorf("fill question requires positive blank_count and no options")
		}
	case TrueFalse, ShortAnswer:
		if len(d.Options) > 0 || d.BlankCount != 0 {
			return Definition{}, fmt.Errorf("nonchoice question cannot declare options or blank_count")
		}
	default:
		return Definition{}, fmt.Errorf("unsupported theory q_type %q", d.Kind)
	}
	if requireAnswer {
		if _, err := d.Normalize(d.Answer, false); err != nil {
			return Definition{}, fmt.Errorf("invalid private theory answer: %w", err)
		}
	}
	return d, nil
}

// Automatic 判断该明确题型是否支持确定性的自动判分,简答不进入自动链。
func (d Definition) Automatic() bool {
	switch d.Kind {
	case SingleChoice, MultipleChoice, TrueFalse, FillBlank:
		return true
	default:
		return false
	}
}

// Face 返回不含黑盒答案的定义,供输入快照与公开题面结构使用。
func (d Definition) Face() map[string]any {
	out := map[string]any{"q_type": string(d.Kind), "max_score": d.MaxScore}
	if len(d.Options) > 0 {
		out["options"] = append([]string(nil), d.Options...)
	}
	if d.BlankCount > 0 {
		out["blank_count"] = d.BlankCount
	}
	return out
}

// Normalize 校验作答形状并生成确定性表示;allowEmpty仅用于学生漏答,不允许空标准答案。
func (d Definition) Normalize(value any, allowEmpty bool) ([]string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode theory response: %w", err)
	}
	if bytes.Equal(raw, []byte("null")) || len(raw) == 0 {
		if allowEmpty {
			return nil, nil
		}
		return nil, fmt.Errorf("theory answer is missing")
	}
	switch d.Kind {
	case SingleChoice, ShortAnswer:
		var answer string
		if err := json.Unmarshal(raw, &answer); err != nil {
			return nil, fmt.Errorf("theory answer must be text: %w", err)
		}
		answer = strings.TrimSpace(answer)
		if answer == "" {
			if allowEmpty {
				return nil, nil
			}
			return nil, fmt.Errorf("theory answer is empty")
		}
		if d.Kind == SingleChoice && !d.hasOption(answer) {
			return nil, fmt.Errorf("choice answer is not a declared option")
		}
		return []string{answer}, nil
	case TrueFalse:
		var answer bool
		if err := json.Unmarshal(raw, &answer); err != nil {
			return nil, fmt.Errorf("judgment answer must be boolean: %w", err)
		}
		if answer {
			return []string{"true"}, nil
		}
		return []string{"false"}, nil
	case MultipleChoice, FillBlank:
		var answers []string
		if err := json.Unmarshal(raw, &answers); err != nil {
			return nil, fmt.Errorf("theory answer must be text array: %w", err)
		}
		if d.Kind == FillBlank && len(answers) != d.BlankCount {
			return nil, fmt.Errorf("fill answer count differs from blank_count")
		}
		if len(answers) == 0 && !allowEmpty {
			return nil, fmt.Errorf("theory answer is empty")
		}
		seen := make(map[string]bool, len(answers))
		for i, answer := range answers {
			answer = strings.TrimSpace(answer)
			if answer == "" && !allowEmpty {
				return nil, fmt.Errorf("theory answer contains empty text")
			}
			if d.Kind == MultipleChoice {
				if !d.hasOption(answer) || seen[answer] {
					return nil, fmt.Errorf("multiple choice contains invalid or repeated option")
				}
				seen[answer] = true
			}
			answers[i] = answer
		}
		if d.Kind == MultipleChoice {
			sort.Strings(answers)
		}
		return answers, nil
	default:
		return nil, fmt.Errorf("unsupported theory q_type %q", d.Kind)
	}
}

// hasOption 判断作答是否精确引用当前冻结题面的一项。
func (d Definition) hasOption(answer string) bool {
	for _, option := range d.Options {
		if answer == option {
			return true
		}
	}
	return false
}
