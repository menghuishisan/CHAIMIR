// judge service_theory 复用理论题契约执行无容器判分,不把答案写入公开结果。
package judge

import (
	"chaimir/pkg/apperr"
	"chaimir/pkg/question"
	"slices"
)

// judgeTheoryQuestion 使用冻结定义比较答案,未作答及答错均为合法零分。
func judgeTheoryQuestion(task JudgeTask) (JudgeExecutionResult, error) {
	definition, err := question.Decode(task.InputSnapshot.Expectation, true)
	if err != nil || !definition.Automatic() || definition.MaxScore != task.InputSnapshot.MaxScore {
		return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid.WithCause(err)
	}
	expected, err := definition.Normalize(definition.Answer, false)
	if err != nil {
		return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid.WithCause(err)
	}
	actual, err := definition.Normalize(task.InputSnapshot.ExtraInput["student_answer"], true)
	if err != nil {
		return JudgeExecutionResult{}, apperr.ErrJudgeSubmitInvalid.WithCause(err)
	}
	passed := len(actual) > 0 && slices.Equal(expected, actual)
	score := int32(0)
	hint := "答案不正确"
	if passed {
		score = definition.MaxScore
		hint = "答案正确"
	}
	return JudgeExecutionResult{
		Passed: passed, Score: score, MaxScore: definition.MaxScore,
		Details: []JudgeResultDetail{{Case: "理论题", Passed: passed, ExpectedLabel: "按题目类型核对答案", Hint: hint}},
	}, nil
}

// SelftestBuiltinTheoryJudger 检验实际判分路径的四种客观题、漏答与错误答案。
func SelftestBuiltinTheoryJudger() error {
	tests := []struct {
		kind    string
		answer  any
		student any
		passed  bool
	}{
		{"single_choice", "a", " a ", true},
		{"single_choice", "a", "b", false},
		{"multiple_choice", []string{"a", "b"}, []string{"b", "a"}, true},
		{"true_false", false, false, true},
		{"true_false", true, nil, false},
		{"fill_blank", []string{"a", "b"}, []string{" a ", "b"}, true},
		{"fill_blank", []string{"a", "b"}, []string{"", "b"}, false},
	}
	for _, item := range tests {
		definition := map[string]any{"q_type": item.kind, "answer": item.answer, "max_score": int32(10)}
		if item.kind == "single_choice" || item.kind == "multiple_choice" {
			definition["options"] = []string{"a", "b"}
		}
		if item.kind == "fill_blank" {
			definition["blank_count"] = 2
		}
		result, err := judgeTheoryQuestion(JudgeTask{InputSnapshot: JudgeInputSnapshot{
			JudgerType: JudgerTypeTheoryQuestion, MaxScore: 10, Expectation: definition,
			ExtraInput: map[string]any{"student_answer": item.student},
		}})
		if err != nil || result.Passed != item.passed || result.MaxScore != 10 {
			return apperr.ErrJudgerSelftestFailed.WithCause(err)
		}
	}
	return nil
}
