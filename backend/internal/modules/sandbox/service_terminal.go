// sandbox service_terminal 文件实现终端和 Web 工具接入前的归属、容器与工具校验。
package sandbox

import (
	"context"
	"io"
	"strings"

	"chaimir/pkg/apperr"
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
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), target.TenantID, target.SandboxID, EventTypeExec, detail)
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
func podGroupForRuntime(runtime Runtime) []PodSpec {
	return podTopologyForAdapter(runtime.AdapterSpec)
}
