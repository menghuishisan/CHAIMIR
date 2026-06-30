// sandbox service_terminal 文件实现终端和 Web 工具接入前的归属、容器与工具校验。
package sandbox

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
	k8sexec "k8s.io/client-go/util/exec"
)

const studentAccessLabel = "chaimir.io/student-access"

// TerminalTarget 描述一次终端连接允许进入的沙箱命名空间和容器。
type TerminalTarget struct {
	TenantID  int64
	SandboxID int64
	Namespace string
	Container string
	Command   []string
}

// TerminalTargetForOwner 校验用户归属并解析允许进入的终端容器。
func (s *Service) TerminalTargetForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, container string) (TerminalTarget, error) {
	sb, runtime, err := s.sandboxRuntimeForOwner(ctx, tenantID, accountID, sandboxID)
	if err != nil {
		return TerminalTarget{}, err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return TerminalTarget{}, err
	}
	targetContainer := strings.TrimSpace(container)
	if targetContainer == "" {
		targetContainer = defaultTerminalContainer(runtime)
		if targetContainer == "" {
			return TerminalTarget{}, apperr.ErrSandboxOwnershipInvalid
		}
	}
	if !runtimeContainerAllowed(runtime, targetContainer) {
		return TerminalTarget{}, apperr.ErrSandboxOwnershipInvalid
	}
	return TerminalTarget{TenantID: sb.TenantID, SandboxID: sb.ID, Namespace: sb.Namespace, Container: targetContainer, Command: runtime.AdapterSpec.WorkspaceOps.Terminal}, nil
}

// AttachTerminal 把已鉴权输入输出流代理到 Kubernetes exec PTY。
func (s *Service) AttachTerminal(ctx context.Context, target TerminalTarget, stdin io.Reader, stdout io.Writer) error {
	if target.TenantID <= 0 || target.SandboxID <= 0 || strings.TrimSpace(target.Namespace) == "" || strings.TrimSpace(target.Container) == "" || len(target.Command) == 0 {
		return apperr.ErrSandboxToolProxyUnavailable
	}
	if err := s.recordTerminalOpen(ctx, target); err != nil {
		return err
	}
	return s.orchestrator.ExecStream(ctx, target.Namespace, target.Container, target.Command, stdin, stdout, stdout, true)
}

// ToolProxyTargetForOwner 校验用户归属并解析 Web 工具代理目标。
func (s *Service) ToolProxyTargetForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, toolCode string) (Sandbox, SandboxTool, error) {
	var sb Sandbox
	var tools []SandboxTool
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if sb.OwnerAccountID != accountID {
			return apperr.ErrSandboxOwnershipInvalid
		}
		tools, err = tx.ListSandboxTools(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxToolNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return Sandbox{}, SandboxTool{}, err
	}
	for _, tool := range tools {
		if tool.Kind == SandboxToolKindWebEmbed && tool.Status == SandboxToolStatusReady && strings.EqualFold(tool.ToolCode, strings.TrimSpace(toolCode)) {
			if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
				return Sandbox{}, SandboxTool{}, err
			}
			return sb, tool, nil
		}
	}
	return Sandbox{}, SandboxTool{}, apperr.ErrSandboxToolNotFound
}

// RunCommandToolForOwner 在命令工具容器中执行一次受控 argv 命令。
func (s *Service) RunCommandToolForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, toolCode string, req ToolRunRequest) (ToolRunResponse, error) {
	if tenantID <= 0 || accountID <= 0 || sandboxID <= 0 || strings.TrimSpace(toolCode) == "" || !safeNonShellCommand(req.Command) {
		return ToolRunResponse{}, apperr.ErrSandboxToolRunRequestInvalid
	}
	stdin, err := decodeOptionalBase64(req.StdinBase64)
	if err != nil {
		return ToolRunResponse{}, apperr.ErrSandboxToolRunRequestInvalid.WithCause(err)
	}
	sb, tool, err := s.commandToolTargetForOwner(ctx, tenantID, accountID, sandboxID, toolCode)
	if err != nil {
		return ToolRunResponse{}, err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return ToolRunResponse{}, err
	}
	target := commandToolExecTarget(tool)
	if target == "" {
		return ToolRunResponse{}, apperr.ErrSandboxToolProxyUnavailable
	}
	if !commandToolCommandAllowed(tool.ResourceSpec.CommandPolicy, req.Command[0]) {
		return ToolRunResponse{}, apperr.ErrSandboxToolRunRequestInvalid
	}
	timeoutSec := commandToolTimeoutSeconds(tool.ResourceSpec.CommandPolicy, req.TimeoutSec, int32(s.cfg.ExecTimeoutSeconds))
	if timeoutSec <= 0 {
		return ToolRunResponse{}, apperr.ErrSandboxToolRunRequestInvalid
	}
	execCtx, cancel := context.WithTimeout(ctx, timeDurationSeconds(int(timeoutSec)))
	defer cancel()
	stdout, stderr, err := s.orchestrator.Exec(execCtx, sb.Namespace, target, req.Command, stdin, false)
	if err != nil {
		if exitCode, ok := commandToolExitCode(err); ok {
			if recordErr := s.recordCommandToolRun(ctx, sb, tool, req.Command); recordErr != nil {
				return ToolRunResponse{}, recordErr
			}
			return ToolRunResponse{
				StdoutBase64: base64.StdEncoding.EncodeToString(stdout),
				StderrBase64: base64.StdEncoding.EncodeToString(stderr),
				ExitCode:     exitCode,
			}, nil
		}
		return ToolRunResponse{}, apperr.ErrSandboxExecFailed.WithCause(fmt.Errorf("%w: %s", err, string(stderr)))
	}
	if err := s.recordCommandToolRun(ctx, sb, tool, req.Command); err != nil {
		return ToolRunResponse{}, err
	}
	return ToolRunResponse{
		StdoutBase64: base64.StdEncoding.EncodeToString(stdout),
		StderrBase64: base64.StdEncoding.EncodeToString(stderr),
		ExitCode:     0,
	}, nil
}

// commandToolExitCode 从 Kubernetes exec 错误链中提取容器进程退出码。
func commandToolExitCode(err error) (int, bool) {
	var exitErr k8sexec.CodeExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus(), true
	}
	return 0, false
}

// commandToolTargetForOwner 校验用户归属并解析已挂载的命令工具。
func (s *Service) commandToolTargetForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, toolCode string) (Sandbox, SandboxTool, error) {
	var sb Sandbox
	var tools []SandboxTool
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if sb.OwnerAccountID != accountID {
			return apperr.ErrSandboxOwnershipInvalid
		}
		tools, err = tx.ListSandboxTools(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxToolNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return Sandbox{}, SandboxTool{}, err
	}
	for _, tool := range tools {
		if tool.Kind == SandboxToolKindCommand && tool.Status == SandboxToolStatusReady && strings.EqualFold(tool.ToolCode, strings.TrimSpace(toolCode)) {
			return sb, tool, nil
		}
	}
	return Sandbox{}, SandboxTool{}, apperr.ErrSandboxToolNotFound
}

// commandToolExecTarget 返回命令工具唯一组件的 exec 目标。
func commandToolExecTarget(tool SandboxTool) string {
	if len(tool.ResourceSpec.Components) == 0 {
		return ""
	}
	component := tool.ResourceSpec.Components[0]
	if strings.TrimSpace(component.Name) == "" {
		return ""
	}
	return toolComponentPodName(tool.ToolCode, component.Name) + "/" + component.Name
}

// commandToolCommandAllowed 判断请求命令是否命中工具白名单。
func commandToolCommandAllowed(policy CommandToolPolicy, command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}
	for _, allowed := range policy.AllowedCommands {
		if command == strings.TrimSpace(allowed) {
			return true
		}
	}
	return false
}

// commandToolTimeoutSeconds 计算命令工具实际执行超时。
func commandToolTimeoutSeconds(policy CommandToolPolicy, requested, platformMax int32) int32 {
	if requested <= 0 {
		requested = policy.DefaultTimeoutSeconds
	}
	limit := policy.MaxTimeoutSeconds
	if platformMax > 0 && (limit <= 0 || platformMax < limit) {
		limit = platformMax
	}
	if limit <= 0 {
		return 0
	}
	if requested > limit {
		return limit
	}
	return requested
}

// decodeOptionalBase64 解码可选 stdin,空字符串表示无输入。
func decodeOptionalBase64(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(raw)
}

// recordCommandToolRun 写入命令工具执行技术事件,不记录输入输出内容。
func (s *Service) recordCommandToolRun(ctx context.Context, sb Sandbox, tool SandboxTool, command []string) error {
	return s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		detail, err := jsonBytes(map[string]any{"tool_code": tool.ToolCode, "command": command[0]})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeExec, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	})
}

// runtimeContainerAllowed 判断终端是否允许进入运行时声明中的容器。
func runtimeContainerAllowed(runtime Runtime, container string) bool {
	podName, containerName := splitExecTarget(container)
	for _, pod := range podGroupForRuntime(runtime) {
		if pod.Name != podName {
			continue
		}
		for _, candidate := range pod.Containers {
			if candidate.Name == containerName && studentAccessibleContainer(candidate) {
				return true
			}
		}
	}
	return false
}

// defaultTerminalContainer 选择第一个显式允许学生进入的容器,避免默认进入链节点主容器。
func defaultTerminalContainer(runtime Runtime) string {
	for _, pod := range podGroupForRuntime(runtime) {
		for _, container := range pod.Containers {
			if studentAccessibleContainer(container) {
				return pod.Name + "/" + container.Name
			}
		}
	}
	return ""
}

// recordTerminalOpen 写入终端打开技术事件,只记录容器目标,不保存终端输入内容。
func (s *Service) recordTerminalOpen(ctx context.Context, target TerminalTarget) error {
	return s.store.TenantTx(ctx, target.TenantID, func(ctx context.Context, tx TxStore) error {
		detail, err := jsonBytes(map[string]any{"container": target.Container})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), target.TenantID, target.SandboxID, EventTypeExec, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	})
}

// runtimeExecTarget 返回平台内部执行 helper 的运行时主容器目标。
func runtimeExecTarget(runtime Runtime) string {
	for _, pod := range podGroupForRuntime(runtime) {
		for _, container := range pod.Containers {
			if container.Name == runtime.AdapterSpec.RuntimeContainer.Name {
				return pod.Name + "/" + container.Name
			}
		}
	}
	return "sandbox/" + runtime.AdapterSpec.RuntimeContainer.Name
}

// podGroupForRuntime 复用编排拓扑,让终端和内部 exec 解析同一组 Pod。
func podGroupForRuntime(runtime Runtime) []workload.PodSpec {
	return podTopologyForAdapter(runtime.AdapterSpec)
}
