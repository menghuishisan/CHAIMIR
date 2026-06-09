// M3 判题器资源配置解析:把 judger.resource_spec JSONB 转为可执行的强类型配置。
package judge

import (
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// JudgerResourceSpec 描述 M3 调 M2 创建 judge 沙箱与执行判题命令所需的配置。
type JudgerResourceSpec struct {
	RuntimeCode         string         `json:"runtime_code"`
	RuntimeImageVersion string         `json:"runtime_image_version"`
	GenesisRef          string         `json:"genesis_ref"`
	ToolCodes           []string       `json:"tool_codes"`
	InitScriptRef       string         `json:"init_script_ref"`
	Command             []string       `json:"command"`
	ResultFile          string         `json:"result_file"`
	MaxRetries          int32          `json:"max_retries"`
	TimeoutSec          int32          `json:"timeout_sec"`
	Metadata            map[string]any `json:"metadata"`
}

// parseJudgerResourceSpec 校验判题器执行配置,避免坏配置进入 worker 后才失败。
func parseJudgerResourceSpec(raw []byte) (JudgerResourceSpec, error) {
	return parseJudgerResourceSpecForType(raw, JudgerTypeTestcase)
}

// parseJudgerResourceSpecForType 按判题器类型校验资源配置;无沙箱判题器不强制 runtime/command。
func parseJudgerResourceSpecForType(raw []byte, judgerType int16) (JudgerResourceSpec, error) {
	var spec JudgerResourceSpec
	// 第一步拒绝空配置,避免 worker 运行时才发现缺少执行参数。
	if len(raw) == 0 {
		return spec, apperr.ErrJudgerInvalid
	}
	// 第二步解析 JSONB,解析失败统一返回判题器配置错误。
	if err := jsonx.DecodeStrict(raw, &spec); err != nil {
		return spec, apperr.ErrJudgerInvalid.WithCause(err)
	}
	// 第三步对无沙箱判题器只校验可选重试配置,不强制声明 runtime/command。
	if judgerType == JudgerTypeManual || judgerType == JudgerTypeFlag || judgerType == JudgerTypeSimCheckpoint {
		if spec.MaxRetries < 0 {
			return spec, apperr.ErrJudgerInvalid
		}
		return spec, nil
	}
	// 第四步校验运行时和命令,这是 fresh 沙箱判题的最小执行条件。
	if strings.TrimSpace(spec.RuntimeCode) == "" || len(spec.Command) == 0 {
		return spec, apperr.ErrJudgerInvalid
	}
	// 第五步固定判题沙箱镜像与创世状态,保证提交快照可复现。
	if strings.TrimSpace(spec.RuntimeImageVersion) == "" || strings.TrimSpace(spec.GenesisRef) == "" {
		return spec, apperr.ErrJudgerInvalid
	}
	// 第六步补齐文档默认值,让后续 worker 不需要重复处理空值。
	if len(spec.ToolCodes) == 0 {
		spec.ToolCodes = []string{"terminal"}
	}
	if strings.TrimSpace(spec.ResultFile) == "" {
		spec.ResultFile = "judge-result.json"
	}
	// 第七步拒绝非法重试配置,避免负数破坏 worker 重试判断。
	if spec.MaxRetries < 0 {
		return spec, apperr.ErrJudgerInvalid
	}
	return spec, nil
}

// maxRetriesForJudger 从判题器资源配置读取任务重试策略,未配置时使用 M3 全局默认值。
func maxRetriesForJudger(raw []byte, judgerType int16, defaultMaxRetries int) (int32, error) {
	spec, err := parseJudgerResourceSpecForType(raw, judgerType)
	if err != nil {
		return 0, err
	}
	if spec.MaxRetries > 0 {
		return spec.MaxRetries, nil
	}
	if defaultMaxRetries < 0 {
		return 0, apperr.ErrJudgerInvalid.WithCause(fmt.Errorf("negative default max retries"))
	}
	return int32(defaultMaxRetries), nil
}

// validateSubmitRequest 校验跨模块判题提交请求的业务边界。
func validateSubmitRequest(req contracts.JudgeSubmitRequest) error {
	// 第一步校验跨模块提交所需的不可变身份、题目和代码字段。
	if req.TenantID <= 0 || req.SubmitterID <= 0 ||
		strings.TrimSpace(req.JudgerCode) == "" ||
		strings.TrimSpace(req.ItemCode) == "" ||
		strings.TrimSpace(req.ItemVersion) == "" ||
		strings.TrimSpace(req.CodeStorageKey) == "" ||
		strings.TrimSpace(req.CodeHash) == "" {
		return apperr.ErrJudgeTaskInvalid
	}
	// 第二步校验来源格式,但不在 M3 内解析上游业务语义。
	if !auth.ValidSourceRef(req.SourceRef) {
		return apperr.ErrJudgeTaskInvalid
	}
	// 第三步规范化沙箱模式,未知模式直接拒绝。
	mode := normalizedSandboxMode(req.SandboxMode)
	if mode == 0 {
		return apperr.ErrJudgeTaskInvalid
	}
	// 第四步 reuse 模式必须明确目标沙箱,否则无法绑定现场上下文。
	if mode == SandboxModeReuse && strings.TrimSpace(req.TargetSandboxRef) == "" {
		return apperr.ErrJudgeTaskInvalid
	}
	return nil
}

// normalizedSandboxMode 把对外文本模式转内部枚举,空值按文档默认 fresh。
func normalizedSandboxMode(mode string) int16 {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", SandboxModeFreshText:
		return SandboxModeFresh
	case SandboxModeReuseText:
		return SandboxModeReuse
	default:
		return 0
	}
}
