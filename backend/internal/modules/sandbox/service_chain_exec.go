// sandbox service_chain_exec 文件实现声明式命令驱动的 L2 链能力。
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// execChainCapability 通过运行时镜像内受控 helper 执行 deploy/tx/query/reset,避免动态加载任意插件代码。
type execChainCapability struct {
	orchestrator   Orchestrator
	timeoutSeconds int
}

// Deploy 调用运行时声明的部署命令,stdin/stdout 均为 JSON 对象。
func (c execChainCapability) Deploy(ctx context.Context, sb Sandbox, runtime Runtime, payload map[string]any) (map[string]any, error) {
	return c.runJSON(ctx, sb, runtime, runtime.AdapterSpec.CapabilityCommands.Deploy, payload)
}

// SendTx 调用运行时声明的交易命令,用于链上交易或链码调用。
func (c execChainCapability) SendTx(ctx context.Context, sb Sandbox, runtime Runtime, payload map[string]any) (map[string]any, error) {
	return c.runJSON(ctx, sb, runtime, runtime.AdapterSpec.CapabilityCommands.Tx, payload)
}

// Query 调用运行时声明的查询命令,只把目标标识传入 helper,避免 HTTP 层拼接链细节。
func (c execChainCapability) Query(ctx context.Context, sb Sandbox, runtime Runtime, target string) (map[string]any, error) {
	return c.runJSON(ctx, sb, runtime, runtime.AdapterSpec.CapabilityCommands.Query, map[string]any{"target": target})
}

// Reset 调用运行时声明的重置命令,把链恢复到创世就绪态。
func (c execChainCapability) Reset(ctx context.Context, sb Sandbox, runtime Runtime) error {
	_, err := c.runJSON(ctx, sb, runtime, runtime.AdapterSpec.CapabilityCommands.Reset, map[string]any{})
	return err
}

// runJSON 统一执行 capability 命令,限制超时并把 stderr 仅作为内部错误链保存。
func (c execChainCapability) runJSON(ctx context.Context, sb Sandbox, runtime Runtime, spec CapabilityCommandSpec, payload map[string]any) (map[string]any, error) {
	if c.orchestrator == nil || !safeCommand(spec.Command) {
		return nil, apperr.ErrSandboxCapabilityUnavailable
	}
	stdin, err := jsonx.AnyBytes(payload, apperr.ErrSandboxContractRequestInvalid)
	if err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithTimeout(ctx, c.commandTimeout(spec))
	defer cancel()
	stdout, stderr, err := c.orchestrator.Exec(runCtx, sb.Namespace, runtimeExecTarget(runtime), spec.Command, stdin, false)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, string(stderr))
	}
	if len(bytes.TrimSpace(stdout)) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := jsonx.DecodeStrict(stdout, &out); err != nil {
		return nil, fmt.Errorf("链能力输出不是 JSON 对象: %w", err)
	}
	return out, nil
}

// commandTimeout 计算单个链能力命令超时,优先使用动作声明,否则使用平台环境配置。
func (c execChainCapability) commandTimeout(spec CapabilityCommandSpec) time.Duration {
	seconds := spec.TimeoutSeconds
	if seconds <= 0 {
		seconds = int32(c.timeoutSeconds)
	}
	if seconds <= 0 {
		seconds = 1
	}
	return time.Duration(seconds) * time.Second
}
