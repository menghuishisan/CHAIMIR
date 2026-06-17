// judge service_spec 文件解析并校验判题器 resource_spec 与提交契约。
package judge

import (
	"path"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// JudgerResourceSpec 描述判题器固定执行环境、自检样例和命令策略。
type JudgerResourceSpec struct {
	RuntimeCode         string         `json:"runtime_code,omitempty"`
	RuntimeImageVersion string         `json:"runtime_image_version,omitempty"`
	GenesisRef          string         `json:"genesis_ref,omitempty"`
	ToolCodes           []string       `json:"tool_codes,omitempty"`
	InitScriptRef       string         `json:"init_script_ref,omitempty"`
	Command             []string       `json:"command,omitempty"`
	TimeoutSec          int32          `json:"timeout_sec,omitempty"`
	MaxRetries          int32          `json:"max_retries,omitempty"`
	SuiteArchiveName    string         `json:"suite_archive_name,omitempty"`
	Selftest            map[string]any `json:"selftest,omitempty"`
}

// parseJudgerResourceSpec 解析并校验平台级判题器资源声明。
func parseJudgerResourceSpec(raw []byte, typ int16, runtimeRequired bool) (JudgerResourceSpec, error) {
	spec := JudgerResourceSpec{}
	if len(raw) > 0 {
		if err := jsonx.DecodeStrict(raw, &spec); err != nil {
			return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid.WithCause(err)
		}
	}
	if spec.TimeoutSec < 0 || spec.MaxRetries < 0 {
		return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid
	}
	if runtimeRequired || typ == JudgerTypeTestcase || typ == JudgerTypeOnchainAssert || typ == JudgerTypeStaticScan {
		if strings.TrimSpace(spec.RuntimeCode) == "" || strings.TrimSpace(spec.RuntimeImageVersion) == "" || strings.TrimSpace(spec.GenesisRef) == "" {
			return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid
		}
	}
	if typ == JudgerTypeTestcase || typ == JudgerTypeStaticScan {
		if !safeNonShellCommand(spec.Command) {
			return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid
		}
	}
	if typ == JudgerTypeManual && len(spec.Command) > 0 {
		return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid
	}
	return spec, nil
}

// validateJudgerRequest 校验判题器注册请求,并返回已解析的资源配置。
func validateJudgerRequest(req CreateJudgerRequest) (JudgerResourceSpec, error) {
	if !codePattern.MatchString(strings.TrimSpace(req.Code)) || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.ExecutorRef) == "" {
		return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid
	}
	if req.Type < JudgerTypeTestcase || req.Type > JudgerTypeManual {
		return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid
	}
	if req.DefaultTimeoutSec <= 0 {
		return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid
	}
	if req.Status != JudgerStatusAvailable && req.Status != JudgerStatusDisabled {
		return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid
	}
	return parseJudgerResourceSpec(req.ResourceSpec, req.Type, req.RuntimeRequired)
}

// validateSubmitRequest 校验内部判题提交契约。
func validateSubmitRequest(req contracts.JudgeSubmitRequest) error {
	if req.TenantID <= 0 || req.SubmitterID <= 0 || strings.TrimSpace(req.JudgerCode) == "" ||
		strings.TrimSpace(req.ItemCode) == "" || strings.TrimSpace(req.ItemVersion) == "" ||
		strings.TrimSpace(req.CodeStorageKey) == "" || !isSHA256Hex(req.CodeHash) ||
		!auth.ValidSourceRef(req.SourceRef) {
		return apperr.ErrJudgeSubmitInvalid
	}
	mode, err := normalizedSandboxMode(req.SandboxMode)
	if err != nil {
		return err
	}
	if mode == JudgeSandboxModeReuse && strings.TrimSpace(req.TargetSandboxRef) == "" {
		return apperr.ErrJudgeSubmitInvalid
	}
	if mode == JudgeSandboxModeFresh && strings.TrimSpace(req.TargetSandboxRef) != "" {
		return apperr.ErrJudgeSubmitInvalid
	}
	return nil
}

// normalizedSandboxMode 统一解析 fresh/reuse 文本为内部枚举。
func normalizedSandboxMode(mode string) (int16, error) {
	switch strings.TrimSpace(mode) {
	case "", contracts.JudgeSandboxModeFresh:
		return JudgeSandboxModeFresh, nil
	case contracts.JudgeSandboxModeReuse:
		return JudgeSandboxModeReuse, nil
	default:
		return 0, apperr.ErrJudgeSubmitInvalid
	}
}

// maxRetriesForJudger 选择判题器配置或全局默认重试次数。
func maxRetriesForJudger(j Judger, defaultMax int) int32 {
	if j.ResourceSpec.MaxRetries > 0 {
		return j.ResourceSpec.MaxRetries
	}
	if defaultMax < 0 {
		return 0
	}
	return int32(defaultMax)
}

// timeoutForSnapshot 选择判题器配置或默认超时。
func timeoutForSnapshot(j Judger) int32 {
	if j.ResourceSpec.TimeoutSec > 0 {
		return j.ResourceSpec.TimeoutSec
	}
	return j.DefaultTimeoutSec
}

// safeCommand 校验命令数组显式且不为空。
func safeCommand(command []string) bool {
	if len(command) == 0 {
		return false
	}
	for _, item := range command {
		if strings.TrimSpace(item) == "" {
			return false
		}
		if strings.ContainsAny(item, "\x00\r\n") {
			return false
		}
	}
	return true
}

// safeNonShellCommand 禁止判题器通过 shell 解释器执行字符串脚本,让命令边界保持 argv 级别。
func safeNonShellCommand(command []string) bool {
	if !safeCommand(command) {
		return false
	}
	blocked := map[string]struct{}{
		"sh": {}, "bash": {}, "dash": {}, "ash": {}, "zsh": {}, "ksh": {}, "csh": {},
		"cmd": {}, "cmd.exe": {}, "powershell": {}, "powershell.exe": {}, "pwsh": {}, "pwsh.exe": {},
	}
	_, ok := blocked[strings.ToLower(path.Base(strings.TrimSpace(command[0])))]
	return !ok
}
