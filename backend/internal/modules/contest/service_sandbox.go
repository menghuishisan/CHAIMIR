// contest service_sandbox 文件实现 M8 沙箱就绪等待、跨校授权核验及 M2 受控主体契约转发。
package contest

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/terminal"
)

const (
	contestSandboxCapabilityWorkspaceRead  = "workspace.read"
	contestSandboxCapabilityWorkspaceWrite = "workspace.write"
	contestSandboxCapabilityTerminal       = "terminal"
	contestSandboxCapabilityCommandTools   = "command_tools"
	contestSandboxCapabilityChain          = "chain"
	contestSandboxCapabilityWebTools       = "web_tools"
)

// waitSandboxReady 用同一有界等待保证对局和预验证只在 M2 权威状态可执行后操作。
func (s *Service) waitSandboxReady(ctx context.Context, tenantID, sandboxID int64) error {
	timeout := time.Duration(s.cfg.BattleSandboxReadyTimeoutSeconds) * time.Second
	interval := time.Duration(s.cfg.BattleSandboxReadyPollIntervalMs) * time.Millisecond
	if timeout <= 0 || interval <= 0 {
		return apperr.ErrContestSandboxUnavailable
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		info, err := s.sandbox.GetSandbox(waitCtx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrContestSandboxUnavailable.WithCause(err)
		}
		if info.Status == contracts.SandboxStatusFailed || info.Status == contracts.SandboxStatusDestroyed {
			return apperr.ErrContestSandboxUnavailable
		}
		if contestSubmissionSandboxReady(info) {
			return nil
		}
		select {
		case <-waitCtx.Done():
			return apperr.ErrContestSandboxUnavailable.WithCause(waitCtx.Err())
		case <-ticker.C:
		}
	}
}

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

// contestSandboxGrantScopeMatches 确认授权精确绑定组织租户、竞赛、题目、队伍、成员和环境。
func contestSandboxGrantScopeMatches(grant ContestAccessGrant, contest Contest, problem ContestProblem, team Team, sandboxID, memberTenantID, accountID int64) bool {
	if contest.ID <= 0 || contest.TenantID != grant.TenantID || problem.ID <= 0 || problem.TenantID != contest.TenantID ||
		problem.ContestID != contest.ID || team.ID <= 0 || team.TenantID != contest.TenantID || team.ContestID != contest.ID ||
		grant.TenantID != contest.TenantID || grant.ContestID != contest.ID || grant.TeamID != team.ID ||
		grant.SandboxID != sandboxID || grant.MemberTenantID != memberTenantID || grant.MemberAccountID != accountID {
		return false
	}
	prefix := contestEnvSourceRefPrefix(contest.CreatedAt.Year(), problem.ID, team.ID)
	if !strings.HasPrefix(grant.SourceRef, prefix) {
		return false
	}
	digest := strings.TrimPrefix(grant.SourceRef, prefix)
	if len(digest) != 16 {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}

// contestSubmissionSandbox 为代码采集与现场判题核验同一归属和就绪规则。
func (s *Service) contestSubmissionSandbox(ctx context.Context, contest Contest, problem ContestProblem, team Team, sandboxID, memberTenantID, accountID int64, capability string) (contestSandboxAccess, contracts.SandboxInfo, error) {
	if sandboxID <= 0 || problem.CompositionDigest == "" {
		return contestSandboxAccess{}, contracts.SandboxInfo{}, apperr.ErrContestSubmissionInvalid
	}
	access, err := s.resolveContestSandboxAccess(ctx, sandboxID, capability)
	if err != nil {
		return contestSandboxAccess{}, contracts.SandboxInfo{}, err
	}
	if !contestSandboxGrantScopeMatches(access.grant, contest, problem, team, sandboxID, memberTenantID, accountID) {
		return contestSandboxAccess{}, contracts.SandboxInfo{}, apperr.ErrContestTeamAccessDenied
	}
	info, err := s.sandbox.GetSandboxForPrincipal(ctx, access.access)
	if err != nil {
		return contestSandboxAccess{}, contracts.SandboxInfo{}, err
	}
	if info.TenantID != contest.TenantID || info.SandboxID != sandboxID || info.SourceRef != access.grant.SourceRef ||
		info.CompositionDigest != problem.CompositionDigest || !contestSubmissionSandboxReady(info) {
		return contestSandboxAccess{}, contracts.SandboxInfo{}, apperr.ErrContestSandboxUnavailable
	}
	return access, info, nil
}

// saveContestSubmissionWorkspace 从当前成员获授权的题目环境保存不可变代码输入。
func (s *Service) saveContestSubmissionWorkspace(ctx context.Context, contest Contest, problem ContestProblem, team Team, sandboxID, memberTenantID, accountID int64) (contracts.SandboxWorkspaceSave, error) {
	access, _, err := s.contestSubmissionSandbox(ctx, contest, problem, team, sandboxID, memberTenantID, accountID, contestSandboxCapabilityWorkspaceWrite)
	if err != nil {
		return contracts.SandboxWorkspaceSave{}, err
	}
	saved, err := s.sandbox.SaveSandboxFilesForPrincipal(ctx, access.access)
	if err != nil {
		return contracts.SandboxWorkspaceSave{}, err
	}
	if strings.TrimSpace(saved.CodeStorageKey) == "" || len(saved.CodeStorageKey) > 255 || !pkgcrypto.ValidSHA256Hex(saved.CodeHash) {
		return contracts.SandboxWorkspaceSave{}, apperr.ErrContestSandboxUnavailable
	}

	latest, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityWorkspaceWrite)
	if err != nil {
		return contracts.SandboxWorkspaceSave{}, err
	}
	if latest.grant.ID != access.grant.ID || latest.grant.GrantVersion != access.grant.GrantVersion ||
		!contestSandboxGrantScopeMatches(latest.grant, contest, problem, team, sandboxID, memberTenantID, accountID) {
		return contracts.SandboxWorkspaceSave{}, apperr.ErrContestTeamAccessDenied
	}
	latestInfo, err := s.sandbox.GetSandboxForPrincipal(ctx, latest.access)
	if err != nil {
		return contracts.SandboxWorkspaceSave{}, err
	}
	if latestInfo.TenantID != contest.TenantID || latestInfo.SandboxID != sandboxID || latestInfo.SourceRef != latest.grant.SourceRef ||
		latestInfo.CompositionDigest != problem.CompositionDigest || !contestSubmissionSandboxReady(latestInfo) ||
		latestInfo.WorkspaceRevision != saved.WorkspaceRevision {
		return contracts.SandboxWorkspaceSave{}, apperr.ErrContestSandboxUnavailable
	}
	return saved, nil
}

// contestSubmissionSandboxReady 接受已可编辑或因空闲进入待回收状态的环境,拒绝准备中和终止环境。
func contestSubmissionSandboxReady(info contracts.SandboxInfo) bool {
	statusReady := info.Status == contracts.SandboxStatusReady || info.Status == contracts.SandboxStatusRunning || info.Status == contracts.SandboxStatusIdle
	phaseReady := info.Phase == contracts.SandboxPhaseReady || info.Phase == contracts.SandboxPhaseFullyReady
	return statusReady && phaseReady
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
func (s *Service) SaveContestSandboxFiles(ctx context.Context, sandboxID int64) (ContestSandboxFileSaveResponse, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityWorkspaceWrite)
	if err != nil {
		return ContestSandboxFileSaveResponse{}, err
	}
	saved, err := s.sandbox.SaveSandboxFilesForPrincipal(ctx, resolved.access)
	if err != nil {
		return ContestSandboxFileSaveResponse{}, err
	}
	return ContestSandboxFileSaveResponse{WorkspaceRevision: saved.WorkspaceRevision}, nil
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
func (s *Service) AttachContestSandboxTerminal(ctx context.Context, access contracts.SandboxPrincipalRequest, target contracts.SandboxTerminalTarget, stdin io.Reader, stdout io.Writer, sizes terminal.SizeQueue) error {
	current, err := s.resolveContestSandboxAccess(ctx, access.SandboxID, contestSandboxCapabilityTerminal)
	if err != nil {
		return err
	}
	if current.access != access {
		return apperr.ErrContestTeamAccessDenied
	}
	return s.sandbox.AttachSandboxTerminal(ctx, current.access, target, stdin, stdout, sizes)
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
func (s *Service) ContestSandboxToolProxyTarget(ctx context.Context, sandboxID int64, toolCode, proxyPath string) (contracts.SandboxPrincipalRequest, *url.URL, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityWebTools)
	if err != nil {
		return contracts.SandboxPrincipalRequest{}, nil, err
	}
	target, err := s.sandbox.ToolProxyTargetForPrincipal(ctx, resolved.access, toolCode, proxyPath)
	if err != nil {
		return contracts.SandboxPrincipalRequest{}, nil, err
	}
	return resolved.access, target, nil
}

// ObserveContestSandboxToolAccess 延续经 M8 代理的 Web 工具活跃度和防抖保存。
func (s *Service) ObserveContestSandboxToolAccess(ctx context.Context, access contracts.SandboxPrincipalRequest) error {
	return s.sandbox.ObserveSandboxToolAccess(ctx, access)
}

// DeployContestSandboxChain 执行当前竞赛成员可用的链部署。
func (s *Service) DeployContestSandboxChain(ctx context.Context, sandboxID int64, runtimeInstance string, payload map[string]any) (map[string]any, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityChain)
	if err != nil {
		return nil, err
	}
	return s.sandbox.ChainDeployForPrincipal(ctx, resolved.access, runtimeInstance, payload)
}

// SendContestSandboxChainTx 执行当前竞赛成员可用的链交易。
func (s *Service) SendContestSandboxChainTx(ctx context.Context, sandboxID int64, runtimeInstance string, payload map[string]any) (map[string]any, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityChain)
	if err != nil {
		return nil, err
	}
	return s.sandbox.ChainSendTxForPrincipal(ctx, resolved.access, runtimeInstance, payload)
}

// QueryContestSandboxChain 执行当前竞赛成员可用的链查询。
func (s *Service) QueryContestSandboxChain(ctx context.Context, sandboxID int64, runtimeInstance, target string) (map[string]any, error) {
	resolved, err := s.resolveContestSandboxAccess(ctx, sandboxID, contestSandboxCapabilityChain)
	if err != nil {
		return nil, err
	}
	return s.sandbox.ChainQueryForPrincipal(ctx, resolved.access, runtimeInstance, target)
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

// closeRevokedContestSandboxSessions 尝试关闭全部被撤销 grant 的终端和进度连接,最后汇总清理错误。
func (s *Service) closeRevokedContestSandboxSessions(ctx context.Context, tenantID int64, grants []ContestAccessGrant) error {
	refs := make([]contracts.ContestSandboxSessionRef, 0, len(grants))
	seen := make(map[contracts.ContestSandboxSessionRef]struct{}, len(grants))
	var failures []error
	for _, grant := range grants {
		ref := contracts.ContestSandboxSessionRef{
			GrantID: grant.ID, SandboxID: grant.SandboxID,
			MemberTenantID: grant.MemberTenantID, MemberAccountID: grant.MemberAccountID,
		}
		if ref.GrantID <= 0 || ref.SandboxID <= 0 || ref.MemberTenantID <= 0 || ref.MemberAccountID <= 0 {
			failures = append(failures, fmt.Errorf("竞赛授权连接引用非法: grant_id=%d sandbox_id=%d", grant.ID, grant.SandboxID))
			continue
		}
		if _, ok := seen[ref]; !ok {
			seen[ref] = struct{}{}
			refs = append(refs, ref)
		}
	}
	if len(refs) == 0 {
		return errors.Join(failures...)
	}
	if tenantID <= 0 {
		failures = append(failures, fmt.Errorf("竞赛撤权租户非法: tenant_id=%d", tenantID))
		return errors.Join(failures...)
	}
	if s.wsHub == nil {
		failures = append(failures, apperr.ErrSandboxToolProxyUnavailable)
	} else if err := closeContestSandboxSessions(s.wsHub, refs); err != nil {
		failures = append(failures, err)
	}
	fanout, ok := s.bus.(eventbus.FanoutBus)
	if !ok {
		failures = append(failures, apperr.ErrEventBusMissing)
	} else if err := fanout.PublishFanout(context.WithoutCancel(ctx), contracts.SubjectContestSandboxSessionsRevoked, contracts.ContestSandboxSessionsRevokedEvent{
		TenantID: tenantID, Sessions: refs,
	}); err != nil {
		failures = append(failures, fmt.Errorf("广播竞赛授权撤销失败: %w", err))
	}
	return errors.Join(failures...)
}

type contestSessionCloser interface {
	CloseSession(ws.SessionKey) error
}

// closeContestSandboxSessions 尝试关闭每项授权对应的终端和进度连接。
func closeContestSandboxSessions(hub contestSessionCloser, refs []contracts.ContestSandboxSessionRef) error {
	if len(refs) == 0 {
		return nil
	}
	if hub == nil {
		return apperr.ErrSandboxToolProxyUnavailable
	}
	var failures []error
	for _, ref := range refs {
		if ref.GrantID <= 0 || ref.SandboxID <= 0 || ref.MemberTenantID <= 0 || ref.MemberAccountID <= 0 {
			failures = append(failures, fmt.Errorf("竞赛授权会话引用非法: grant_id=%d sandbox_id=%d", ref.GrantID, ref.SandboxID))
			continue
		}
		terminal := ws.SessionKey{
			TenantID: ref.MemberTenantID, AccountID: ref.MemberAccountID,
			Scope: contestSandboxSessionScope(ref.GrantID, ref.SandboxID),
		}
		progress := terminal
		progress.Scope += ":progress"
		if err := hub.CloseSession(terminal); err != nil {
			failures = append(failures, fmt.Errorf("关闭竞赛终端失败: grant_id=%d sandbox_id=%d: %w", ref.GrantID, ref.SandboxID, err))
		}
		if err := hub.CloseSession(progress); err != nil {
			failures = append(failures, fmt.Errorf("关闭竞赛进度连接失败: grant_id=%d sandbox_id=%d: %w", ref.GrantID, ref.SandboxID, err))
		}
	}
	return errors.Join(failures...)
}
