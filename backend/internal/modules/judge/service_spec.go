// judge service_spec 文件解析并校验判题器 resource_spec 与提交契约。
package judge

import (
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
)

// JudgerExecutionSpec 描述平台管理员提交的受控执行策略，不包含持久化组合快照。
type JudgerExecutionSpec struct {
	GenesisRef        string                   `json:"genesis_ref,omitempty"`
	InitScriptRef     string                   `json:"init_script_ref,omitempty"`
	Command           []string                 `json:"command,omitempty"`
	ExecTarget        string                   `json:"exec_target,omitempty"`
	ExecutionSidecars []workload.ComponentSpec `json:"execution_sidecars,omitempty"`
	TimeoutSec        int32                    `json:"timeout_sec,omitempty"`
	MaxRetries        int32                    `json:"max_retries,omitempty"`
	SuiteArchiveName  string                   `json:"suite_archive_name,omitempty"`
	Selftest          map[string]any           `json:"selftest,omitempty"`
}

// JudgerResourceSpec 描述数据库中唯一持久化的判题执行事实。
type JudgerResourceSpec struct {
	CompositionSnapshot contracts.SandboxCompositionSnapshot `json:"composition_snapshot,omitempty"`
	GenesisRef          string                               `json:"genesis_ref,omitempty"`
	InitScriptRef       string                               `json:"init_script_ref,omitempty"`
	Command             []string                             `json:"command,omitempty"`
	ExecTarget          string                               `json:"exec_target,omitempty"`
	ExecutionSidecars   []workload.ComponentSpec             `json:"execution_sidecars,omitempty"`
	TimeoutSec          int32                                `json:"timeout_sec,omitempty"`
	MaxRetries          int32                                `json:"max_retries,omitempty"`
	SuiteArchiveName    string                               `json:"suite_archive_name,omitempty"`
	Selftest            map[string]any                       `json:"selftest,omitempty"`
}

// parseJudgerExecutionSpec 解析并校验平台管理员提交的执行策略。
func parseJudgerExecutionSpec(raw []byte, typ int16, runtimeRequired bool) (JudgerExecutionSpec, error) {
	spec := JudgerExecutionSpec{}
	if len(raw) > 0 {
		if err := jsonx.DecodeStrictKnownFields(raw, &spec); err != nil {
			return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid.WithCause(err)
		}
	}
	if spec.TimeoutSec < 0 || spec.MaxRetries < 0 {
		return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid
	}
	if runtimeRequired || typ == JudgerTypeTestcase || typ == JudgerTypeOnchainAssert || typ == JudgerTypeStaticScan {
		if strings.TrimSpace(spec.GenesisRef) == "" {
			return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid
		}
	}
	if typ == JudgerTypeTestcase || typ == JudgerTypeStaticScan {
		if !workload.ValidNonShellCommand(spec.Command) {
			return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid
		}
		if strings.TrimSpace(spec.ExecTarget) == "" || len(spec.ExecutionSidecars) == 0 {
			return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid
		}
		if !safeExecTarget(spec.ExecTarget) {
			return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid
		}
	}
	if typ == JudgerTypeManual && len(spec.Command) > 0 {
		return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid
	}
	return spec, nil
}

// parseJudgerResourceSpec 解析数据库中的判题执行事实，并拒绝旧声明或摘要字段。
func parseJudgerResourceSpec(raw []byte, typ int16, runtimeRequired bool) (JudgerResourceSpec, error) {
	spec := JudgerResourceSpec{}
	if len(raw) == 0 {
		return spec, nil
	}
	if err := jsonx.DecodeStrictKnownFields(raw, &spec); err != nil {
		return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid.WithCause(err)
	}
	if spec.TimeoutSec < 0 || spec.MaxRetries < 0 {
		return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid
	}
	if !judgerNeedsSandbox(typ, runtimeRequired) {
		if typ == JudgerTypeManual && (len(spec.Command) > 0 || len(spec.ExecutionSidecars) > 0 || spec.CompositionSnapshot.Digest != "") {
			return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid
		}
		return spec, nil
	}
	if strings.TrimSpace(spec.CompositionSnapshot.Digest) == "" {
		return spec, nil
	}
	digest, err := contracts.CanonicalSnapshotDigest(spec.CompositionSnapshot)
	if err != nil || digest != spec.CompositionSnapshot.Digest || spec.CompositionSnapshot.Spec.AccessProfile != contracts.SandboxAccessJudgePrivate || strings.TrimSpace(spec.GenesisRef) == "" {
		return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid
	}
	if typ == JudgerTypeTestcase || typ == JudgerTypeStaticScan {
		if !workload.ValidNonShellCommand(spec.Command) || !safeExecTarget(spec.ExecTarget) || len(spec.ExecutionSidecars) == 0 {
			return JudgerResourceSpec{}, apperr.ErrJudgerConfigInvalid
		}
	}
	return spec, nil
}

// validateJudgerRequest 校验判题器注册请求，并返回已解析的执行策略。
func validateJudgerRequest(req JudgerRequest) (JudgerExecutionSpec, error) {
	if !codePattern.MatchString(strings.TrimSpace(req.Code)) || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.ExecutorRef) == "" {
		return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid
	}
	if req.Type < JudgerTypeTestcase || req.Type > JudgerTypeManual {
		return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid
	}
	if req.DefaultTimeoutSec <= 0 {
		return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid
	}
	if req.Status != JudgerStatusAvailable && req.Status != JudgerStatusDisabled {
		return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid
	}
	if judgerNeedsSandbox(req.Type, req.RuntimeRequired) {
		if strings.TrimSpace(req.Composition.ID) == "" || len(req.Composition.Runtimes) == 0 || req.Composition.AccessProfile != contracts.SandboxAccessJudgePrivate {
			return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid
		}
		for _, runtime := range req.Composition.Runtimes {
			if strings.TrimSpace(runtime.InstanceCode) == "" || strings.TrimSpace(runtime.Code) == "" || strings.TrimSpace(runtime.ImageVersion) == "" {
				return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid
			}
		}
	} else if strings.TrimSpace(req.Composition.ID) != "" || len(req.Composition.Runtimes) != 0 {
		return JudgerExecutionSpec{}, apperr.ErrJudgerConfigInvalid
	}
	return parseJudgerExecutionSpec(req.ResourceSpec, req.Type, req.RuntimeRequired)
}

// judgerNeedsSandbox 统一判定判题器是否必须冻结 M2 judge-private 环境快照。
func judgerNeedsSandbox(typ int16, runtimeRequired bool) bool {
	return runtimeRequired || typ == JudgerTypeTestcase || typ == JudgerTypeOnchainAssert || typ == JudgerTypeStaticScan
}

// validateSubmitRequest 校验内部判题提交契约。
func validateSubmitRequest(req contracts.JudgeSubmitRequest) error {
	if req.TenantID <= 0 || req.SubmitterTenantID <= 0 || req.SubmitterID <= 0 ||
		strings.TrimSpace(req.ItemCode) == "" || strings.TrimSpace(req.ItemVersion) == "" ||
		!auth.ValidSourceRef(req.SourceRef) {
		return apperr.ErrJudgeSubmitInvalid
	}
	if strings.TrimSpace(req.CodeHash) != "" && !pkgcrypto.ValidSHA256Hex(req.CodeHash) {
		return apperr.ErrJudgeSubmitInvalid
	}
	mode, err := normalizedSandboxMode(req.SandboxMode)
	if err != nil {
		return err
	}
	if mode == JudgeSandboxModeReuse && strings.TrimSpace(req.TargetSandboxRef) == "" {
		return apperr.ErrJudgeSubmitInvalid
	}
	if mode == JudgeSandboxModeReuse && strings.TrimSpace(req.SandboxSourceRef) == "" {
		return apperr.ErrJudgeSubmitInvalid
	}
	if mode == JudgeSandboxModeFresh && strings.TrimSpace(req.TargetSandboxRef) != "" {
		return apperr.ErrJudgeSubmitInvalid
	}
	if mode == JudgeSandboxModeFresh && strings.TrimSpace(req.SandboxSourceRef) != "" {
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

// judgerRequiresCode 统一判定判题任务是否必须携带提交代码对象。
func judgerRequiresCode(typ int16, mode int16) bool {
	switch typ {
	case JudgerTypeTestcase, JudgerTypeStaticScan:
		return true
	case JudgerTypeOnchainAssert:
		return mode == JudgeSandboxModeFresh
	default:
		return false
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
	maxRetries, ok := intx.Int32(defaultMax)
	if !ok {
		return 0
	}
	return maxRetries
}

// timeoutForSnapshot 选择判题器配置或默认超时。
func timeoutForSnapshot(j Judger) int32 {
	if j.ResourceSpec.TimeoutSec > 0 {
		return j.ResourceSpec.TimeoutSec
	}
	return j.DefaultTimeoutSec
}

// safeExecTarget 校验内部执行目标是 pod/container 形式的受控 DNS 标签。
func safeExecTarget(target string) bool {
	parts := strings.Split(strings.TrimSpace(target), "/")
	if len(parts) != 2 {
		return false
	}
	return codePattern.MatchString(parts[0]) && codePattern.MatchString(parts[1])
}
