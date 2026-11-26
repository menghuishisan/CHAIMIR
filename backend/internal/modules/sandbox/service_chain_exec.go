package sandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
)

type execChainCapability struct {
	orchestrator Orchestrator
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
	if c.orchestrator == nil || !workload.ValidNonShellCommand(spec.Command) || spec.TimeoutSeconds <= 0 {
		return nil, apperr.ErrSandboxCapabilityUnavailable
	}
	stdin, err := jsonx.AnyBytes(payload, apperr.ErrSandboxContractRequestInvalid)
	if err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(spec.TimeoutSeconds)*time.Second)
	defer cancel()
	stdout, stderr, err := c.orchestrator.Exec(runCtx, sb.Namespace, strings.TrimSpace(spec.ExecTarget), spec.Command, stdin, false)
	if err != nil {
		return nil, sandboxExecFailure(apperr.ErrSandboxChainFailed, err, stderr)
	}
	var out map[string]any
	if err := jsonx.DecodeStrict(stdout, &out); err != nil {
		return nil, fmt.Errorf("链能力输出不是 JSON 对象: %w", err)
	}
	if out == nil {
		return nil, errors.New("链能力未返回 JSON 对象")
	}
	return out, nil
}
