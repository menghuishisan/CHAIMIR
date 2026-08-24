// sandbox service_principal 文件实现 M8 等业务网关完成业务授权后使用的受控沙箱访问契约。
package sandbox

import (
	"context"
	"encoding/base64"
	"io"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
)

// validatePrincipalAccess 保证业务网关传入的受控主体、目标沙箱和来源引用是完整且自洽的。
func validatePrincipalAccess(access contracts.SandboxPrincipalRequest) error {
	if access.TenantID <= 0 || access.SandboxID <= 0 || !validSourceRef(access.SourceRef) ||
		access.Principal.AuthorizationID <= 0 || access.Principal.AuthorizationRevision <= 0 ||
		access.Principal.SubjectTenantID <= 0 || access.Principal.SubjectAccountID <= 0 ||
		strings.TrimSpace(access.Principal.SourceRef) != strings.TrimSpace(access.SourceRef) {
		return apperr.ErrSandboxOwnershipInvalid
	}
	return nil
}

// sandboxForPrincipal 校验来源绑定后加载组织租户中的沙箱；业务授权是否有效由调用网关在每次请求前负责。
func (s *Service) sandboxForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest) (Sandbox, Runtime, error) {
	if err := validatePrincipalAccess(access); err != nil {
		return Sandbox{}, Runtime{}, err
	}
	return s.sandboxRuntimeForSource(ctx, access.TenantID, access.SandboxID, access.SourceRef)
}

// GetSandboxForPrincipal 返回经业务网关授权的沙箱摘要。
func (s *Service) GetSandboxForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest) (contracts.SandboxInfo, error) {
	if _, _, err := s.sandboxForPrincipal(ctx, access); err != nil {
		return contracts.SandboxInfo{}, err
	}
	return s.GetSandbox(ctx, access.TenantID, access.SandboxID)
}

// ReadSandboxFileForPrincipal 读取经业务授权的公开工作区文件。
func (s *Service) ReadSandboxFileForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, relativePath string) (contracts.SandboxWorkspaceFileRead, error) {
	if _, _, err := s.sandboxForPrincipal(ctx, access); err != nil {
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
	if _, _, err := s.sandboxForPrincipal(ctx, access); err != nil {
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
	if _, _, err := s.sandboxForPrincipal(ctx, req.Access); err != nil {
		return 0, err
	}
	return s.PutSandboxFile(ctx, contracts.SandboxFileWriteRequest{TenantID: req.Access.TenantID, SandboxID: req.Access.SandboxID, SourceRef: req.Access.SourceRef, RelativePath: req.RelativePath, ContentBase64: req.ContentBase64, ExpectedRevision: req.ExpectedRevision})
}

// SaveSandboxFilesForPrincipal 立即保存经业务授权的工作区。
func (s *Service) SaveSandboxFilesForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest) (contracts.SandboxWorkspaceSave, error) {
	if _, _, err := s.sandboxForPrincipal(ctx, access); err != nil {
		return contracts.SandboxWorkspaceSave{}, err
	}
	key, hash, err := s.SaveSandboxFiles(ctx, contracts.SandboxSaveRequest{TenantID: access.TenantID, SandboxID: access.SandboxID, SourceRef: access.SourceRef})
	if err != nil {
		return contracts.SandboxWorkspaceSave{}, err
	}
	info, err := s.GetSandboxForPrincipal(ctx, access)
	if err != nil {
		return contracts.SandboxWorkspaceSave{}, err
	}
	return contracts.SandboxWorkspaceSave{CodeStorageKey: key, CodeHash: hash, WorkspaceRevision: info.WorkspaceRevision}, nil
}

// commandToolTargetForPrincipal 解析经业务授权的命令工具，而不使用 M2 租户内账号归属集合。
func (s *Service) commandToolTargetForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, toolCode string) (Sandbox, SandboxTool, error) {
	sb, _, err := s.sandboxForPrincipal(ctx, access)
	if err != nil {
		return Sandbox{}, SandboxTool{}, err
	}
	var tools []SandboxTool
	if err := s.store.TenantTx(ctx, access.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		tools, err = tx.ListSandboxTools(ctx, access.TenantID, access.SandboxID)
		return err
	}); err != nil {
		return Sandbox{}, SandboxTool{}, apperr.ErrSandboxToolNotFound.WithCause(err)
	}
	for _, tool := range tools {
		if tool.Kind == SandboxToolKindCommand && tool.Status == SandboxToolStatusReady && strings.EqualFold(tool.ToolCode, strings.TrimSpace(toolCode)) {
			return sb, tool, nil
		}
	}
	return Sandbox{}, SandboxTool{}, apperr.ErrSandboxToolNotFound
}

// RunCommandToolForPrincipal 在业务网关已经校验授权的情况下执行工具白名单内命令。
func (s *Service) RunCommandToolForPrincipal(ctx context.Context, req contracts.SandboxCommandToolRequest) (contracts.SandboxCommandToolResult, error) {
	if strings.TrimSpace(req.ToolCode) == "" || !workload.ValidNonShellCommand(req.Command) {
		return contracts.SandboxCommandToolResult{}, apperr.ErrSandboxToolRunRequestInvalid
	}
	stdin, err := decodeOptionalBase64(req.StdinBase64)
	if err != nil {
		return contracts.SandboxCommandToolResult{}, apperr.ErrSandboxToolRunRequestInvalid.WithCause(err)
	}
	sb, tool, err := s.commandToolTargetForPrincipal(ctx, req.Access, req.ToolCode)
	if err != nil {
		return contracts.SandboxCommandToolResult{}, err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return contracts.SandboxCommandToolResult{}, err
	}
	target := commandToolExecTarget(tool)
	if target == "" || !commandToolCommandAllowed(tool.ResourceSpec.CommandPolicy, req.Command) {
		return contracts.SandboxCommandToolResult{}, apperr.ErrSandboxToolRunRequestInvalid
	}
	defaultTimeout := int32(s.cfg.ExecTimeoutSeconds)
	timeoutSec := commandToolTimeoutSeconds(tool.ResourceSpec.CommandPolicy, req.TimeoutSec, defaultTimeout)
	if defaultTimeout <= 0 || timeoutSec <= 0 {
		return contracts.SandboxCommandToolResult{}, apperr.ErrSandboxToolRunRequestInvalid
	}
	execCtx, cancel := context.WithTimeout(ctx, timeDurationSeconds(int(timeoutSec)))
	defer cancel()
	stdout, stderr, err := s.orchestrator.Exec(execCtx, sb.Namespace, target, req.Command, stdin, false)
	if err != nil {
		if exitCode, ok := commandToolExitCode(err); ok {
			if recordErr := s.recordCommandToolRun(ctx, sb, tool, req.Command); recordErr != nil {
				return contracts.SandboxCommandToolResult{}, recordErr
			}
			return contracts.SandboxCommandToolResult{StdoutBase64: base64.StdEncoding.EncodeToString(stdout), StderrBase64: base64.StdEncoding.EncodeToString(stderr), ExitCode: exitCode}, nil
		}
		return contracts.SandboxCommandToolResult{}, sandboxExecFailure(apperr.ErrSandboxExecFailed, err, stderr)
	}
	if err := s.recordCommandToolRun(ctx, sb, tool, req.Command); err != nil {
		return contracts.SandboxCommandToolResult{}, err
	}
	return contracts.SandboxCommandToolResult{StdoutBase64: base64.StdEncoding.EncodeToString(stdout), StderrBase64: base64.StdEncoding.EncodeToString(stderr)}, nil
}

// TerminalTargetForPrincipal 返回业务授权主体可进入的学生终端目标。
func (s *Service) TerminalTargetForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, container string) (contracts.SandboxTerminalTarget, error) {
	sb, runtime, err := s.sandboxForPrincipal(ctx, access)
	if err != nil {
		return contracts.SandboxTerminalTarget{}, err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return contracts.SandboxTerminalTarget{}, err
	}
	targetContainer := strings.TrimSpace(container)
	if targetContainer == "" {
		targetContainer = defaultTerminalContainer(runtime)
	}
	if targetContainer == "" || !runtimeContainerAllowed(runtime, targetContainer) {
		return contracts.SandboxTerminalTarget{}, apperr.ErrSandboxOwnershipInvalid
	}
	return contracts.SandboxTerminalTarget{TenantID: sb.TenantID, SandboxID: sb.ID, Namespace: sb.Namespace, Container: targetContainer, Command: append([]string(nil), runtime.AdapterSpec.WorkspaceOps.Terminal...)}, nil
}

// AttachSandboxTerminal 二次根据受控访问主体重算目标，避免调用方伪造命名空间或容器。
func (s *Service) AttachSandboxTerminal(ctx context.Context, access contracts.SandboxPrincipalRequest, target contracts.SandboxTerminalTarget, stdin io.Reader, stdout io.Writer) error {
	expected, err := s.TerminalTargetForPrincipal(ctx, access, target.Container)
	if err != nil {
		return err
	}
	if expected.TenantID != target.TenantID || expected.SandboxID != target.SandboxID || expected.Namespace != target.Namespace || expected.Container != target.Container || !sameStringSlice(expected.Command, target.Command) {
		return apperr.ErrSandboxOwnershipInvalid
	}
	return s.AttachTerminal(ctx, TerminalTarget{TenantID: expected.TenantID, SandboxID: expected.SandboxID, Namespace: expected.Namespace, Container: expected.Container, Command: expected.Command}, stdin, stdout)
}

// ProgressSubscriptionForPrincipal 返回经业务授权的进度主题及当前状态。
func (s *Service) ProgressSubscriptionForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest) (string, contracts.SandboxProgressMessage, error) {
	sb, _, err := s.sandboxForPrincipal(ctx, access)
	if err != nil {
		return "", contracts.SandboxProgressMessage{}, err
	}
	message := progressFromState(sb.Phase, sb.Status, "")
	return progressTopic(sb.TenantID, sb.ID), contracts.SandboxProgressMessage{Phase: message.Phase, Status: message.Status, Stage: message.Stage, Message: message.Message, TraceID: message.TraceID}, nil
}

// ToolProxyTargetForPrincipal 返回业务授权主体可代理的 Web 工具内部目标。
func (s *Service) ToolProxyTargetForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, toolCode string) (contracts.SandboxToolProxyTarget, error) {
	sb, _, err := s.sandboxForPrincipal(ctx, access)
	if err != nil {
		return contracts.SandboxToolProxyTarget{}, err
	}
	var tools []SandboxTool
	if err := s.store.TenantTx(ctx, access.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		tools, err = tx.ListSandboxTools(ctx, access.TenantID, access.SandboxID)
		return err
	}); err != nil {
		return contracts.SandboxToolProxyTarget{}, apperr.ErrSandboxToolNotFound.WithCause(err)
	}
	for _, tool := range tools {
		if tool.Kind == SandboxToolKindWebEmbed && tool.Status == SandboxToolStatusReady && strings.EqualFold(tool.ToolCode, strings.TrimSpace(toolCode)) {
			if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
				return contracts.SandboxToolProxyTarget{}, err
			}
			target := toolProxyURL(sb, tool)
			if target == nil || strings.TrimSpace(target.Host) == "" {
				return contracts.SandboxToolProxyTarget{}, apperr.ErrSandboxToolProxyUnavailable
			}
			return contracts.SandboxToolProxyTarget{TargetURL: target.String()}, nil
		}
	}
	return contracts.SandboxToolProxyTarget{}, apperr.ErrSandboxToolNotFound
}

// ObserveSandboxToolAccess 记录经业务网关代理的 Web 工具访问。
func (s *Service) ObserveSandboxToolAccess(ctx context.Context, access contracts.SandboxPrincipalRequest) error {
	sb, _, err := s.sandboxForPrincipal(ctx, access)
	if err != nil {
		return err
	}
	s.ObserveToolAccess(ctx, sb)
	return nil
}

// ChainDeployForPrincipal 执行经业务授权的链部署。
func (s *Service) ChainDeployForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, runtimeInstance string, payload map[string]any) (map[string]any, error) {
	if _, _, err := s.sandboxForPrincipal(ctx, access); err != nil {
		return nil, err
	}
	return s.ChainDeploy(ctx, contracts.SandboxChainDeployRequest{TenantID: access.TenantID, SandboxID: access.SandboxID, SourceRef: access.SourceRef, RuntimeInstance: runtimeInstance, Payload: payload})
}

// ChainSendTxForPrincipal 执行经业务授权的链交易。
func (s *Service) ChainSendTxForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, runtimeInstance string, payload map[string]any) (map[string]any, error) {
	if _, _, err := s.sandboxForPrincipal(ctx, access); err != nil {
		return nil, err
	}
	return s.ChainSendTx(ctx, contracts.SandboxChainTxRequest{TenantID: access.TenantID, SandboxID: access.SandboxID, SourceRef: access.SourceRef, RuntimeInstance: runtimeInstance, Payload: payload})
}

// ChainQueryForPrincipal 执行经业务授权的链查询。
func (s *Service) ChainQueryForPrincipal(ctx context.Context, access contracts.SandboxPrincipalRequest, runtimeInstance, target string) (map[string]any, error) {
	if _, _, err := s.sandboxForPrincipal(ctx, access); err != nil {
		return nil, err
	}
	return s.ChainQuery(ctx, contracts.SandboxChainQueryRequest{TenantID: access.TenantID, SandboxID: access.SandboxID, SourceRef: access.SourceRef, RuntimeInstance: runtimeInstance, Target: target})
}

// sameStringSlice 比较终端固定命令，避免 attach 阶段命令替换。
func sameStringSlice(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
