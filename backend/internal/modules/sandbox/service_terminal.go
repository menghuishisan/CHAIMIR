package sandbox

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/workload"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/terminal"

	k8sexec "k8s.io/client-go/util/exec"
)

const studentAccessLabel = "chaimir.io/student-access"

func sandboxTerminalSessionScope(sandboxID int64) string {
	return "sandbox:" + ids.Format(sandboxID) + ":terminal"
}

func sandboxProgressSessionScope(sandboxID int64) string {
	return "sandbox:" + ids.Format(sandboxID) + ":progress"
}

func (s *Service) closeSandboxSessions(tenantID, accountID, sandboxID int64) error {
	if s.wsHub == nil {
		return nil
	}
	return closeSandboxSessionsFor(s.wsHub, tenantID, accountID, sandboxID)
}

type sandboxSessionCloser interface {
	CloseSession(ws.SessionKey) error
}

func closeSandboxSessionsFor(hub sandboxSessionCloser, tenantID, accountID, sandboxID int64) error {
	if tenantID <= 0 || accountID <= 0 || sandboxID <= 0 {
		return apperr.ErrSandboxContractRequestInvalid
	}
	if hub == nil {
		return apperr.ErrSandboxToolProxyUnavailable
	}
	return errors.Join(
		hub.CloseSession(ws.SessionKey{TenantID: tenantID, AccountID: accountID, Scope: sandboxTerminalSessionScope(sandboxID)}),
		hub.CloseSession(ws.SessionKey{TenantID: tenantID, AccountID: accountID, Scope: sandboxProgressSessionScope(sandboxID)}),
	)
}

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
	if !sandboxExecAllowed(sb) {
		return TerminalTarget{}, apperr.ErrSandboxStateInvalid
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
	command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.Terminal, runtime.AdapterSpec.WorkspaceDir, runtime.AdapterSpec.WorkspaceDir)
	return TerminalTarget{TenantID: sb.TenantID, SandboxID: sb.ID, Namespace: sb.Namespace, Container: targetContainer, Command: command}, nil
}

// AttachTerminal 把已鉴权输入输出流代理到 Kubernetes exec PTY。
func (s *Service) AttachTerminal(ctx context.Context, target TerminalTarget, stdin io.Reader, stdout io.Writer, sizes terminal.SizeQueue) error {
	if target.TenantID <= 0 || target.SandboxID <= 0 || strings.TrimSpace(target.Namespace) == "" || strings.TrimSpace(target.Container) == "" || len(target.Command) == 0 {
		return apperr.ErrSandboxToolProxyUnavailable
	}
	if err := s.recordTerminalOpen(ctx, target); err != nil {
		return err
	}
	return s.orchestrator.ExecStream(ctx, target.Namespace, target.Container, target.Command, stdin, stdout, stdout, true, sizes)
}

func (s *Service) AttachTerminalForOwner(ctx context.Context, tenantID, accountID int64, target TerminalTarget, stdin io.Reader, stdout io.Writer, sizes terminal.SizeQueue) error {
	expected, err := s.TerminalTargetForOwner(ctx, tenantID, accountID, target.SandboxID, target.Container)
	if err != nil {
		return err
	}
	if expected.TenantID != target.TenantID || expected.SandboxID != target.SandboxID || expected.Namespace != target.Namespace || expected.Container != target.Container || !slices.Equal(expected.Command, target.Command) {
		return apperr.ErrSandboxOwnershipInvalid
	}
	return s.AttachTerminal(ctx, expected, stdin, stdout, sizes)
}

// ToolProxyTargetForOwner 校验用户归属并解析 Web 工具代理目标。
func (s *Service) ToolProxyTargetForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, toolCode, proxyPath string) (Sandbox, *url.URL, error) {
	sb, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID)
	if err != nil {
		return Sandbox{}, nil, err
	}
	target, err := s.toolProxyTarget(ctx, sb, toolCode, proxyPath)
	return sb, target, err
}

func (s *Service) toolProxyTarget(ctx context.Context, sb Sandbox, toolCode, proxyPath string) (*url.URL, error) {
	tool, err := s.readySandboxTool(ctx, sb, SandboxToolKindWebEmbed, toolCode)
	if err != nil {
		return nil, err
	}
	target, err := toolProxyURL(sb, tool, proxyPath)
	if err != nil {
		return nil, err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return nil, err
	}
	return target, nil
}

// toolProxyURL 保持最长路径段匹配；不得因声明顺序把请求发往其他组件。
func toolProxyURL(sb Sandbox, tool SandboxTool, proxyPath string) (*url.URL, error) {
	if err := validateToolServicesAndRoutes(&tool.ResourceSpec); err != nil {
		return nil, apperr.ErrSandboxToolProxyUnavailable.WithCause(err)
	}
	proxyPath = "/" + strings.TrimPrefix(proxyPath, "/")
	var selected *workload.RouteSpec
	for i := range tool.ResourceSpec.Routes {
		route := &tool.ResourceSpec.Routes[i]
		prefix := route.PathPrefix
		if prefix != "/" && proxyPath != prefix && !strings.HasPrefix(proxyPath, prefix+"/") {
			continue
		}
		if selected == nil || len(prefix) > len(selected.PathPrefix) {
			selected = route
		}
	}
	if selected == nil || strings.TrimSpace(sb.Namespace) == "" {
		return nil, apperr.ErrSandboxToolProxyUnavailable.WithCause(fmt.Errorf("工具 %s 的路径 %q 无有效 Service 目标", tool.ToolCode, proxyPath))
	}
	for _, service := range tool.ResourceSpec.Services {
		if service.Name != selected.Service {
			continue
		}
		for _, port := range service.Ports {
			if port.Name == selected.Port {
				return &url.URL{Scheme: "http", Host: fmt.Sprintf("%s.%s.svc.cluster.local:%d", service.Name, sb.Namespace, port.Port)}, nil
			}
		}
	}
	return nil, apperr.ErrSandboxToolProxyUnavailable
}

// RunCommandToolForOwner 在命令工具容器中执行一次受控 argv 命令。
func (s *Service) RunCommandToolForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, toolCode string, req ToolRunRequest) (ToolRunResponse, error) {
	if tenantID <= 0 || accountID <= 0 || sandboxID <= 0 || strings.TrimSpace(toolCode) == "" || !workload.ValidNonShellCommand(req.Command) {
		return ToolRunResponse{}, apperr.ErrSandboxToolRunRequestInvalid
	}
	sb, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID)
	if err != nil {
		return ToolRunResponse{}, err
	}
	tool, err := s.readySandboxTool(ctx, sb, SandboxToolKindCommand, toolCode)
	if err != nil {
		return ToolRunResponse{}, err
	}
	return s.runCommandTool(ctx, sb, tool, req)
}

func (s *Service) runCommandTool(ctx context.Context, sb Sandbox, tool SandboxTool, req ToolRunRequest) (ToolRunResponse, error) {
	stdin, err := decodeOptionalBase64(req.StdinBase64)
	if err != nil {
		return ToolRunResponse{}, apperr.ErrSandboxToolRunRequestInvalid.WithCause(err)
	}
	target := commandToolExecTarget(tool)
	if target == "" {
		return ToolRunResponse{}, apperr.ErrSandboxToolProxyUnavailable
	}
	if !commandToolCommandAllowed(tool.ResourceSpec.CommandPolicy, req.Command) {
		return ToolRunResponse{}, apperr.ErrSandboxToolRunRequestInvalid
	}
	defaultTimeout, ok := intx.Int32(s.cfg.ExecTimeoutSeconds)
	if !ok || defaultTimeout <= 0 {
		return ToolRunResponse{}, apperr.ErrSandboxToolRunRequestInvalid
	}
	timeoutSec := commandToolTimeoutSeconds(tool.ResourceSpec.CommandPolicy, req.TimeoutSec, defaultTimeout)
	if timeoutSec <= 0 {
		return ToolRunResponse{}, apperr.ErrSandboxToolRunRequestInvalid
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return ToolRunResponse{}, err
	}
	execCtx, cancel := context.WithTimeout(ctx, timeDurationSeconds(int(timeoutSec)))
	defer cancel()
	stdout, stderr, execErr := s.orchestrator.Exec(execCtx, sb.Namespace, target, req.Command, stdin, false)
	exitCode := 0
	if execErr != nil {
		var exited bool
		exitCode, exited = commandToolExitCode(execErr)
		if !exited {
			return ToolRunResponse{}, sandboxExecFailure(apperr.ErrSandboxExecFailed, execErr, stderr)
		}
	}
	if err := s.recordCommandToolRun(ctx, sb, tool, req.Command); err != nil {
		return ToolRunResponse{}, err
	}
	return ToolRunResponse{StdoutBase64: base64.StdEncoding.EncodeToString(stdout), StderrBase64: base64.StdEncoding.EncodeToString(stderr), ExitCode: exitCode}, nil
}

func commandToolExitCode(err error) (int, bool) {
	var exitErr k8sexec.CodeExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus(), true
	}
	return 0, false
}

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

func commandToolCommandAllowed(policy CommandToolPolicy, command []string) bool {
	if !workload.ValidNonShellCommand(command) {
		return false
	}
	for _, allowed := range policy.AllowedArgv {
		if slices.Equal(command, allowed) {
			return true
		}
	}
	return false
}

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

func decodeOptionalBase64(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(raw)
}

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

func (s *Service) recordTerminalOpen(ctx context.Context, target TerminalTarget) error {
	return s.store.TenantTx(ctx, target.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.MarkSandboxActive(ctx, target.TenantID, target.SandboxID); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
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

func workspaceExecTarget(runtime Runtime) string {
	return strings.TrimSpace(runtime.AdapterSpec.WorkspaceOps.ExecTarget)
}

func podGroupForRuntime(runtime Runtime) []workload.PodSpec {
	return podTopologyForAdapter(runtime.AdapterSpec)
}
