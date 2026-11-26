package sandbox

import (
	"context"
	"io"
	"net/url"
	"slices"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/terminal"
)

func validatePrincipalAccess(access contracts.SandboxPrincipalRequest) error {
	if access.TenantID <= 0 || access.SandboxID <= 0 || !validSourceRef(access.SourceRef) ||
		access.Principal.AuthorizationID <= 0 || access.Principal.AuthorizationRevision <= 0 ||
		access.Principal.SubjectTenantID <= 0 || access.Principal.SubjectAccountID <= 0 ||
		strings.TrimSpace(access.Principal.SourceRef) != strings.TrimSpace(access.SourceRef) {
		return apperr.ErrSandboxOwnershipInvalid
	}
	return nil
}

// 业务授权由调用网关逐请求校验；状态观察不要求当前目录允许执行。
func (s *Service) sandboxForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest) (Sandbox, error) {
	if err := validatePrincipalAccess(access); err != nil {
		return Sandbox{}, err
	}
	var sb Sandbox
	err := s.store.TenantTx(ctx, access.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, access.TenantID, access.SandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if sb.SourceRef != strings.TrimSpace(access.SourceRef) {
			return apperr.ErrSandboxOwnershipInvalid
		}
		return nil
	})
	return sb, err
}

func (s *Service) sandboxRuntimeForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest) (Sandbox, Runtime, error) {
	sb, err := s.sandboxForPrincipal(ctx, access)
	if err != nil {
		return Sandbox{}, Runtime{}, err
	}
	runtime, _, err := s.workspaceRuntimeForSandbox(ctx, sb)
	if err != nil {
		return Sandbox{}, Runtime{}, apperr.ErrSandboxRuntimeNotFound.WithCause(err)
	}
	return sb, runtime, nil
}

// GetSandboxForPrincipal 返回经业务网关授权的沙箱摘要。
func (s *Service) GetSandboxForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest) (contracts.SandboxInfo, error) {
	sb, err := s.sandboxForPrincipal(ctx, access)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	return s.sandboxInfo(ctx, sb)
}

// ReadSandboxFileForPrincipal 读取经业务授权的公开工作区文件。
func (s *Service) ReadSandboxFileForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, relativePath string) (contracts.SandboxWorkspaceFileRead, error) {
	if _, _, err := s.sandboxRuntimeForPrincipal(ctx, access); err != nil {
		return contracts.SandboxWorkspaceFileRead{}, err
	}
	out, err := s.ReadSandboxFile(ctx, access.TenantID, access.SandboxID, relativePath)
	if err != nil {
		return contracts.SandboxWorkspaceFileRead{}, err
	}
	return contracts.SandboxWorkspaceFileRead{RelativePath: out.RelativePath, ContentBase64: out.ContentBase64, ContentSHA256: out.ContentSHA256, ContentSize: out.ContentSize, WorkspaceRevision: out.WorkspaceRevision}, nil
}

// ListSandboxFilesForPrincipal 列出经业务授权的公开工作区目录。
func (s *Service) ListSandboxFilesForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, relativePath string) (contracts.SandboxWorkspaceFileList, error) {
	if _, _, err := s.sandboxRuntimeForPrincipal(ctx, access); err != nil {
		return contracts.SandboxWorkspaceFileList{}, err
	}
	out, err := s.ListSandboxFiles(ctx, access.TenantID, access.SandboxID, relativePath)
	if err != nil {
		return contracts.SandboxWorkspaceFileList{}, err
	}
	entries := make([]contracts.SandboxWorkspaceFileEntry, 0, len(out.Entries))
	for _, entry := range out.Entries {
		entries = append(entries, contracts.SandboxWorkspaceFileEntry{Name: entry.Name, RelativePath: entry.RelativePath, IsDir: entry.IsDir, Size: entry.Size})
	}
	return contracts.SandboxWorkspaceFileList{RelativePath: out.RelativePath, Entries: entries}, nil
}

// WriteSandboxFileForPrincipal 写入经业务授权的公开工作区文件。
func (s *Service) WriteSandboxFileForPrincipal(ctx context.Context, req contracts.SandboxWorkspaceFileWrite) (int64, error) {
	if _, _, err := s.sandboxRuntimeForPrincipal(ctx, req.Access); err != nil {
		return 0, err
	}
	return s.PutSandboxFile(ctx, contracts.SandboxFileWriteRequest{TenantID: req.Access.TenantID, SandboxID: req.Access.SandboxID, SourceRef: req.Access.SourceRef, RelativePath: req.RelativePath, ContentBase64: req.ContentBase64, ExpectedRevision: req.ExpectedRevision})
}

// SaveSandboxFilesForPrincipal 立即保存经业务授权的工作区。
func (s *Service) SaveSandboxFilesForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest) (contracts.SandboxWorkspaceSave, error) {
	sb, _, err := s.sandboxRuntimeForPrincipal(ctx, access)
	if err != nil {
		return contracts.SandboxWorkspaceSave{}, err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return contracts.SandboxWorkspaceSave{}, err
	}
	return s.saveSandboxFiles(ctx, sb)
}

// RunCommandToolForPrincipal 在业务网关已经校验授权的情况下执行工具白名单内命令。
func (s *Service) RunCommandToolForPrincipal(ctx context.Context, req contracts.SandboxCommandToolRequest) (contracts.SandboxCommandToolResult, error) {
	if strings.TrimSpace(req.ToolCode) == "" || !workload.ValidNonShellCommand(req.Command) {
		return contracts.SandboxCommandToolResult{}, apperr.ErrSandboxToolRunRequestInvalid
	}
	sb, _, err := s.sandboxRuntimeForPrincipal(ctx, req.Access)
	if err != nil {
		return contracts.SandboxCommandToolResult{}, err
	}
	tool, err := s.readySandboxTool(ctx, sb, SandboxToolKindCommand, req.ToolCode)
	if err != nil {
		return contracts.SandboxCommandToolResult{}, err
	}
	out, err := s.runCommandTool(ctx, sb, tool, ToolRunRequest{Command: req.Command, StdinBase64: req.StdinBase64, TimeoutSec: req.TimeoutSec})
	if err != nil {
		return contracts.SandboxCommandToolResult{}, err
	}
	return contracts.SandboxCommandToolResult{StdoutBase64: out.StdoutBase64, StderrBase64: out.StderrBase64, ExitCode: out.ExitCode}, nil
}

// TerminalTargetForPrincipal 返回业务授权主体可进入的学生终端目标。
func (s *Service) TerminalTargetForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, container string) (contracts.SandboxTerminalTarget, error) {
	sb, runtime, err := s.sandboxRuntimeForPrincipal(ctx, access)
	if err != nil {
		return contracts.SandboxTerminalTarget{}, err
	}
	if !sandboxExecAllowed(sb) {
		return contracts.SandboxTerminalTarget{}, apperr.ErrSandboxStateInvalid
	}
	targetContainer := strings.TrimSpace(container)
	if targetContainer == "" {
		targetContainer = defaultTerminalContainer(runtime)
	}
	if targetContainer == "" || !runtimeContainerAllowed(runtime, targetContainer) {
		return contracts.SandboxTerminalTarget{}, apperr.ErrSandboxOwnershipInvalid
	}
	command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.Terminal, runtime.AdapterSpec.WorkspaceDir, runtime.AdapterSpec.WorkspaceDir)
	return contracts.SandboxTerminalTarget{TenantID: sb.TenantID, SandboxID: sb.ID, Namespace: sb.Namespace, Container: targetContainer, Command: command}, nil
}

// AttachSandboxTerminal 二次根据受控访问主体重算目标，避免调用方伪造命名空间或容器。
func (s *Service) AttachSandboxTerminal(ctx context.Context, access contracts.SandboxPrincipalRequest, target contracts.SandboxTerminalTarget, stdin io.Reader, stdout io.Writer, sizes terminal.SizeQueue) error {
	expected, err := s.TerminalTargetForPrincipal(ctx, access, target.Container)
	if err != nil {
		return err
	}
	if expected.TenantID != target.TenantID || expected.SandboxID != target.SandboxID || expected.Namespace != target.Namespace || expected.Container != target.Container || !slices.Equal(expected.Command, target.Command) {
		return apperr.ErrSandboxOwnershipInvalid
	}
	return s.AttachTerminal(ctx, TerminalTarget{TenantID: expected.TenantID, SandboxID: expected.SandboxID, Namespace: expected.Namespace, Container: expected.Container, Command: expected.Command}, stdin, stdout, sizes)
}

// ProgressSubscriptionForPrincipal 返回经业务授权的进度主题及当前状态。
func (s *Service) ProgressSubscriptionForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest) (string, contracts.SandboxProgressMessage, error) {
	sb, err := s.sandboxForPrincipal(ctx, access)
	if err != nil {
		return "", contracts.SandboxProgressMessage{}, err
	}
	message := progressFromState(sb.Phase, sb.Status, "")
	return progressTopic(sb.TenantID, sb.ID), contracts.SandboxProgressMessage{Phase: message.Phase, Status: message.Status, Stage: message.Stage, Message: message.Message, TraceID: message.TraceID}, nil
}

// ToolProxyTargetForPrincipal 返回业务授权主体可代理的 Web 工具内部目标。
func (s *Service) ToolProxyTargetForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, toolCode, proxyPath string) (*url.URL, error) {
	sb, _, err := s.sandboxRuntimeForPrincipal(ctx, access)
	if err != nil {
		return nil, err
	}
	return s.toolProxyTarget(ctx, sb, toolCode, proxyPath)
}

// ObserveSandboxToolAccess 记录经业务网关代理的 Web 工具访问。
func (s *Service) ObserveSandboxToolAccess(ctx context.Context, access contracts.SandboxPrincipalRequest) error {
	sb, _, err := s.sandboxRuntimeForPrincipal(ctx, access)
	if err != nil {
		return err
	}
	s.ObserveToolAccess(ctx, sb)
	return nil
}

// ChainDeployForPrincipal 执行经业务授权的链部署。
func (s *Service) ChainDeployForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, runtimeInstance string, payload map[string]any) (map[string]any, error) {
	if err := validatePrincipalAccess(access); err != nil {
		return nil, err
	}
	return s.ChainDeploy(ctx, contracts.SandboxChainDeployRequest{TenantID: access.TenantID, SandboxID: access.SandboxID, SourceRef: access.SourceRef, RuntimeInstance: runtimeInstance, Payload: payload})
}

// ChainSendTxForPrincipal 执行经业务授权的链交易。
func (s *Service) ChainSendTxForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, runtimeInstance string, payload map[string]any) (map[string]any, error) {
	if err := validatePrincipalAccess(access); err != nil {
		return nil, err
	}
	return s.ChainSendTx(ctx, contracts.SandboxChainTxRequest{TenantID: access.TenantID, SandboxID: access.SandboxID, SourceRef: access.SourceRef, RuntimeInstance: runtimeInstance, Payload: payload})
}

// ChainQueryForPrincipal 执行经业务授权的链查询。
func (s *Service) ChainQueryForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, runtimeInstance, target string) (map[string]any, error) {
	if err := validatePrincipalAccess(access); err != nil {
		return nil, err
	}
	return s.ChainQuery(ctx, contracts.SandboxChainQueryRequest{TenantID: access.TenantID, SandboxID: access.SandboxID, SourceRef: access.SourceRef, RuntimeInstance: runtimeInstance, Target: target})
}
