// contest service_sandbox 文件实现 M8 跨校竞赛沙箱授权核验与对 M2 受控主体契约的转发。
package contest

import (
	"context"
	"fmt"
	"io"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
)

const (
	contestSandboxCapabilityWorkspaceRead  = "workspace.read"
	contestSandboxCapabilityWorkspaceWrite = "workspace.write"
	contestSandboxCapabilityTerminal       = "terminal"
	contestSandboxCapabilityCommandTools   = "command_tools"
	contestSandboxCapabilityChain          = "chain"
	contestSandboxCapabilityWebTools       = "web_tools"
)

// contestSandboxGrantCapabilities 从 M2 返回的运行时能力快照生成最小授权集合。
// grant 不再因为业务路径固定而把未声明的终端、链或网页工具暴露给所有成员。
func contestSandboxGrantCapabilities(info contracts.SandboxInfo) []string {
	capabilities := make([]string, 0, 6)
	if info.Capabilities.FileWorkspace {
		capabilities = append(capabilities, contestSandboxCapabilityWorkspaceRead, contestSandboxCapabilityWorkspaceWrite)
	}
	if info.Capabilities.Terminal {
		capabilities = append(capabilities, contestSandboxCapabilityTerminal)
	}
	if info.Capabilities.CommandTools {
		capabilities = append(capabilities, contestSandboxCapabilityCommandTools)
	}
	if len(info.Capabilities.ChainOperations) > 0 {
		capabilities = append(capabilities, contestSandboxCapabilityChain)
	}
	for _, tool := range info.ToolAccess {
		if tool.Kind == contracts.SandboxToolKindWebEmbed {
			capabilities = append(capabilities, contestSandboxCapabilityWebTools)
			break
		}
	}
	return capabilities
}

// contestSandboxAccess 是 M8 验证 grant 后传给 M2 的最小受控访问上下文。
type contestSandboxAccess struct {
	grant  ContestAccessGrant
	access contracts.SandboxPrincipalRequest
}

// resolveContestSandboxAccess 以当前登录学生身份读取唯一 grant，并在每次操作前重新校验撤销、到期和能力。
func (s *Service) resolveContestSandboxAccess(ctx context.Context, sandboxID int64, requiredCapability string) (contestSandboxAccess, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return contestSandboxAccess{}, err
	}
	if sandboxID <= 0 || strings.TrimSpace(requiredCapability) == "" {
		return contestSandboxAccess{}, apperr.ErrContestTeamAccessDenied
	}
	var grant ContestAccessGrant
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		grant, err = tx.FindContestAccessGrantForSubject(ctx, sandboxID, id.TenantID, id.AccountID)
		return err
	}); err != nil {
		return contestSandboxAccess{}, err
	}
	if grant.Status != contestAccessGrantActive || !grant.ExpiresAt.After(timex.Now()) || !grantAllows(grant, requiredCapability) {
		return contestSandboxAccess{}, apperr.ErrContestTeamAccessDenied
	}
	return contestSandboxAccess{
		grant: grant,
		access: contracts.SandboxPrincipalRequest{
			TenantID: grant.TenantID, SandboxID: grant.SandboxID, SourceRef: grant.SourceRef,
			Principal: contracts.SandboxAccessPrincipal{AuthorizationID: grant.ID, AuthorizationRevision: grant.GrantVersion, SubjectTenantID: id.TenantID, SubjectAccountID: id.AccountID, SourceRef: grant.SourceRef},
		},
	}, nil
}

// grantAllows 判断已签发能力集合是否覆盖本次访问动作。
func grantAllows(grant ContestAccessGrant, required string) bool {
	for _, capability := range grant.Capabilities {
		if capability == required {
			return true
		}
	}
	return false
}

// contestSandboxSessionScope 让撤销精确关闭这个 grant 所属沙箱的长连接，而不是踢掉用户其他会话。
func contestSandboxSessionScope(grantID, sandboxID int64) string {
	return fmt.Sprintf("contest-sandbox:%d:grant:%d", sandboxID, grantID)
}

// GetContestSandbox 返回当前竞赛成员可访问的沙箱摘要。
func (s *Service) GetContestSandbox(ctx context.Context, sandboxID int64) (contracts.SandboxInfo, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityWorkspaceRead)
	if err != nil {
		return contracts.SandboxInfo{}, err
	}
	return s.sandbox.GetSandboxForPrincipal(ctx, resolved.access)
}

// ReadContestSandboxFile 读取当前竞赛成员可访问的工作区文件。
func (s *Service) ReadContestSandboxFile(ctx context.Context, sandboxID int64, relativePath string) (contracts.SandboxWorkspaceFileRead, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityWorkspaceRead)
	if err != nil {
		return contracts.SandboxWorkspaceFileRead{}, err
	}
	return s.sandbox.ReadSandboxFileForPrincipal(ctx, resolved.access, relativePath)
}

// ListContestSandboxFiles 列出当前竞赛成员可访问的工作区目录。
func (s *Service) ListContestSandboxFiles(ctx context.Context, sandboxID int64, relativePath string) (contracts.SandboxWorkspaceFileList, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityWorkspaceRead)
	if err != nil {
		return contracts.SandboxWorkspaceFileList{}, err
	}
	return s.sandbox.ListSandboxFilesForPrincipal(ctx, resolved.access, relativePath)
}

// WriteContestSandboxFile 写入当前竞赛成员可访问的工作区文件。
func (s *Service) WriteContestSandboxFile(ctx context.Context, sandboxID int64, relativePath, contentBase64 string, expectedRevision int64) (int64, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityWorkspaceWrite)
	if err != nil {
		return 0, err
	}
	return s.sandbox.WriteSandboxFileForPrincipal(ctx, contracts.SandboxWorkspaceFileWrite{Access: resolved.access, RelativePath: relativePath, ContentBase64: contentBase64, ExpectedRevision: expectedRevision})
}

// SaveContestSandboxFiles 立即持久化当前竞赛成员可访问的工作区。
func (s *Service) SaveContestSandboxFiles(ctx context.Context, sandboxID int64) (contracts.SandboxWorkspaceSave, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityWorkspaceWrite)
	if err != nil {
		return contracts.SandboxWorkspaceSave{}, err
	}
	return s.sandbox.SaveSandboxFilesForPrincipal(ctx, resolved.access)
}

// RunContestSandboxCommandTool 执行当前竞赛成员已授权的工具白名单命令。
func (s *Service) RunContestSandboxCommandTool(ctx context.Context, sandboxID int64, toolCode string, command []string, stdinBase64 string, timeoutSec int32) (contracts.SandboxCommandToolResult, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityCommandTools)
	if err != nil {
		return contracts.SandboxCommandToolResult{}, err
	}
	return s.sandbox.RunCommandToolForPrincipal(ctx, contracts.SandboxCommandToolRequest{Access: resolved.access, ToolCode: toolCode, Command: command, StdinBase64: stdinBase64, TimeoutSec: timeoutSec})
}

// ContestSandboxTerminalTarget 返回当前竞赛成员可连接的终端目标及绑定授权。
func (s *Service) ContestSandboxTerminalTarget(ctx context.Context, sandboxID int64, container string) (contracts.SandboxPrincipalRequest, contracts.SandboxTerminalTarget, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityTerminal)
	if err != nil {
		return contracts.SandboxPrincipalRequest{}, contracts.SandboxTerminalTarget{}, err
	}
	target, err := s.sandbox.TerminalTargetForPrincipal(ctx, resolved.access, container)
	if err != nil {
		return contracts.SandboxPrincipalRequest{}, contracts.SandboxTerminalTarget{}, err
	}
	return resolved.access, target, nil
}

// AttachContestSandboxTerminal 按已核验的 grant 和终端目标附加 PTY。
func (s *Service) AttachContestSandboxTerminal(ctx context.Context, access contracts.SandboxPrincipalRequest, target contracts.SandboxTerminalTarget, stdin io.Reader, stdout io.Writer) error {
	return s.sandbox.AttachSandboxTerminal(ctx, access, target, stdin, stdout)
}

// ContestSandboxProgressSubscription 返回当前成员可订阅的沙箱进度主题。
func (s *Service) ContestSandboxProgressSubscription(ctx context.Context, sandboxID int64) (contracts.SandboxPrincipalRequest, string, contracts.SandboxProgressMessage, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityWorkspaceRead)
	if err != nil {
		return contracts.SandboxPrincipalRequest{}, "", contracts.SandboxProgressMessage{}, err
	}
	topic, initial, err := s.sandbox.ProgressSubscriptionForPrincipal(ctx, resolved.access)
	if err != nil {
		return contracts.SandboxPrincipalRequest{}, "", contracts.SandboxProgressMessage{}, err
	}
	return resolved.access, topic, initial, nil
}

// ContestSandboxToolProxyTarget 返回当前成员可使用的 Web 工具代理目标及授权上下文。
func (s *Service) ContestSandboxToolProxyTarget(ctx context.Context, sandboxID int64, toolCode string) (contracts.SandboxPrincipalRequest, contracts.SandboxToolProxyTarget, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityWebTools)
	if err != nil {
		return contracts.SandboxPrincipalRequest{}, contracts.SandboxToolProxyTarget{}, err
	}
	target, err := s.sandbox.ToolProxyTargetForPrincipal(ctx, resolved.access, toolCode)
	if err != nil {
		return contracts.SandboxPrincipalRequest{}, contracts.SandboxToolProxyTarget{}, err
	}
	return resolved.access, target, nil
}

// ObserveContestSandboxToolAccess 延续经 M8 代理的 Web 工具活跃度和防抖保存。
func (s *Service) ObserveContestSandboxToolAccess(ctx context.Context, access contracts.SandboxPrincipalRequest) error {
	return s.sandbox.ObserveSandboxToolAccess(ctx, access)
}

// DeployContestSandboxChain 执行当前竞赛成员可用的链部署。
func (s *Service) DeployContestSandboxChain(ctx context.Context, sandboxID int64, payload map[string]any) (map[string]any, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityChain)
	if err != nil {
		return nil, err
	}
	return s.sandbox.ChainDeployForPrincipal(ctx, resolved.access, payload)
}

// SendContestSandboxChainTx 执行当前竞赛成员可用的链交易。
func (s *Service) SendContestSandboxChainTx(ctx context.Context, sandboxID int64, payload map[string]any) (map[string]any, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityChain)
	if err != nil {
		return nil, err
	}
	return s.sandbox.ChainSendTxForPrincipal(ctx, resolved.access, payload)
}

// QueryContestSandboxChain 执行当前竞赛成员可用的链查询。
func (s *Service) QueryContestSandboxChain(ctx context.Context, sandboxID int64, target string) (map[string]any, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityChain)
	if err != nil {
		return nil, err
	}
	return s.sandbox.ChainQueryForPrincipal(ctx, resolved.access, target)
}

// revokeContestSandboxGrants 在同一 M8 事务中使赛事全部活跃沙箱授权失效，并返回需关闭长连接的主体。
func (s *Service) revokeContestSandboxGrants(ctx context.Context, tx TxStore, tenantID, contestID int64) ([]ContestAccessGrant, error) {
	grants, err := tx.ListContestAccessGrantsForContest(ctx, tenantID, contestID)
	if err != nil {
		return nil, err
	}
	sandboxes := make(map[int64]struct{}, len(grants))
	for _, grant := range grants {
		sandboxes[grant.SandboxID] = struct{}{}
	}
	for sandboxID := range sandboxes {
		if err := tx.RevokeContestAccessGrantsForSandbox(ctx, tenantID, sandboxID); err != nil {
			return nil, err
		}
	}
	return grants, nil
}

// closeRevokedContestSandboxSessions 精确关闭被撤销 grant 对应的终端和进度连接。
func (s *Service) closeRevokedContestSandboxSessions(grants []ContestAccessGrant) error {
	for _, grant := range grants {
		if s.wsHub == nil {
			return apperr.ErrSandboxToolProxyUnavailable
		}
		terminal := ws.SessionKey{TenantID: grant.MemberTenantID, AccountID: grant.MemberAccountID, Scope: contestSandboxSessionScope(grant.ID, grant.SandboxID)}
		if err := s.wsHub.CloseSession(terminal); err != nil {
			return err
		}
		progress := terminal
		progress.Scope += ":progress"
		if err := s.wsHub.CloseSession(progress); err != nil {
			return err
		}
	}
	return nil
}
