// 本文件实现判题执行器共享的结果归一化命令,输出 M3 统一 JSON 契约。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	maxOutputSummaryRunes = 500
	maxOutputReadBytes    = 4096
)

type judgeDetail struct {
	Case   string `json:"case,omitempty"`
	Passed bool   `json:"passed"`
	Source string `json:"source,omitempty"`
	Actual string `json:"actual,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

type judgeResult struct {
	Passed   bool          `json:"passed"`
	Score    int           `json:"score"`
	MaxScore int           `json:"max_score"`
	Details  []judgeDetail `json:"details"`
}

// main 根据命令行模式归一化判题器输出,失败时只向 stderr 输出运维可见原因。
func main() {
	mode := flag.String("mode", "", "normalization mode")
	exitCode := flag.Int("exit-code", 0, "command exit code")
	source := flag.String("source", "testcase", "judge source")
	stdoutPath := flag.String("stdout", "", "captured stdout path")
	reportPath := flag.String("report", "", "tool report path")
	flag.Parse()

	var result judgeResult
	var err error
	switch *mode {
	case "exit-code":
		result, err = fromExitCode(*exitCode, *source, *stdoutPath)
	case "slither":
		result, err = fromSlither(*reportPath)
	default:
		err = fmt.Errorf("unsupported normalizer mode")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(64)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(64)
	}
}

// fromExitCode 将通用命令退出码转换为 M3 判题结果,stdout 仅作为脱敏摘要写入。
func fromExitCode(exitCode int, source string, stdoutPath string) (judgeResult, error) {
	passed := exitCode == 0
	actual := "判题命令通过"
	hint := ""
	if !passed {
		actual = fmt.Sprintf("判题命令退出码 %d", exitCode)
	}
	if stdoutPath != "" {
		content, truncated, err := readTextPrefix(stdoutPath, maxOutputReadBytes)
		if err != nil {
			return judgeResult{}, err
		}
		actual, hint = safeOutputSummary(content, truncated)
		if actual == "" {
			actual = "判题命令未输出可展示摘要"
		}
	}
	if strings.TrimSpace(source) == "" {
		source = "testcase"
	}
	return judgeResult{
		Passed:   passed,
		Score:    boolScore(passed),
		MaxScore: 1,
		Details:  []judgeDetail{detail("命令执行", passed, source, actual, hint)},
	}, nil
}

// fromSlither 将 Slither JSON 报告归一化为静态扫描结果,不回传源码或完整报告。
func fromSlither(reportPath string) (judgeResult, error) {
	if reportPath == "" {
		return judgeResult{}, fmt.Errorf("--report is required for slither mode")
	}
	content, err := os.ReadFile(reportPath)
	if err != nil {
		return judgeResult{}, err
	}
	var payload struct {
		Results struct {
			Detectors []map[string]any `json:"detectors"`
		} `json:"results"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return judgeResult{}, err
	}
	failed := len(payload.Results.Detectors) > 0
	details := make([]judgeDetail, 0, min(len(payload.Results.Detectors), 50))
	for _, detector := range payload.Results.Detectors {
		if len(details) >= 50 {
			break
		}
		check, _ := detector["check"].(string)
		description, _ := detector["description"].(string)
		if check == "" {
			check = "slither"
		}
		if description == "" {
			description = "发现静态检查风险"
		}
		description, hint := safeOutputSummary(description, false)
		if description == "" {
			description = "发现静态检查风险"
		}
		details = append(details, detail("静态扫描", false, safeSource(check), description, hint))
	}
	if len(details) == 0 {
		details = append(details, detail("静态扫描", true, "slither", "未发现阻断级静态检查风险", ""))
	}
	return judgeResult{
		Passed:   !failed,
		Score:    boolScore(!failed),
		MaxScore: 1,
		Details:  details,
	}, nil
}

// detail 构造一条满足后端校验字段的脱敏结果详情。
func detail(caseName string, passed bool, source string, actual string, hint string) judgeDetail {
	actual = clipRunes(strings.TrimSpace(actual), maxOutputSummaryRunes)
	hint = clipRunes(strings.TrimSpace(hint), maxOutputSummaryRunes)
	return judgeDetail{Case: caseName, Passed: passed, Source: safeSource(source), Actual: actual, Hint: hint}
}

// readTextPrefix 只读取文件前缀,避免超大 stdout 被整体载入归一化器内存。
func readTextPrefix(path string, limit int64) (string, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, fmt.Errorf("judge stdout file does not exist: %s", path)
		}
		return "", false, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return "", false, err
	}
	truncated := int64(len(content)) > limit
	if truncated {
		content = content[:limit]
	}
	return strings.ToValidUTF8(string(content), ""), truncated, nil
}

// safeOutputSummary 返回可展示输出摘要,遇到答案、flag、密钥等敏感线索时直接隐藏。
func safeOutputSummary(raw string, truncated bool) (string, string) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", ""
	}
	if containsResultSensitiveText(text) {
		return "判题输出包含敏感内容,已隐藏", "请查看教师侧判题套件或运维日志定位具体原因"
	}
	summary := clipRunes(text, maxOutputSummaryRunes)
	if truncated || len([]rune(text)) > maxOutputSummaryRunes {
		return summary, "判题输出较长,这里只展示前段摘要"
	}
	return summary, ""
}

// containsResultSensitiveText 使用与后端用户可见结果一致的保守敏感词边界。
func containsResultSensitiveText(value string) bool {
	lower := strings.ToLower(value)
	markers := []string{
		"flag{",
		"-----begin",
		"answer",
		"answers",
		"correct_answer",
		"solution",
		"judge_config",
		"testcases",
		"hidden_test",
		"hidden_testcases",
		"suite_source",
		"answer_source",
		"private_key",
		"privatekey",
		"secret",
		"token",
		"credential",
		"authorization",
		"api_key",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// safeSource 清理工具来源字段,避免外部报告把敏感内容塞进 source。
func safeSource(source string) string {
	value := strings.TrimSpace(source)
	if value == "" || containsResultSensitiveText(value) {
		return "judge"
	}
	return clipRunes(value, 120)
}

// clipRunes 按 rune 截断用户可见摘要,避免破坏 UTF-8。
func clipRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

// boolScore 将布尔通过状态映射为归一化的一分制分值。
func boolScore(passed bool) int {
	if passed {
		return 1
	}
	return 0
}
