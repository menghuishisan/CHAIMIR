// sandbox service_contract 文件实现 M2 对聚合和业务模块开放的统一沙箱能力契约。
package sandbox

import (
	"context"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/storage"
	"chaimir/pkg/apperr"
)

// ValidateSandboxTemplate 校验沙箱模板可调度性,但不创建资源。
func (s *Service) ValidateSandboxTemplate(ctx context.Context, req contracts.SandboxCreateRequest) error {
	return s.validateSandboxTemplateContract(ctx, req)
}

// CreateSandbox 创建沙箱并返回控制面摘要。
func (s *Service) CreateSandbox(ctx context.Context, req contracts.SandboxCreateRequest) (contracts.SandboxInfo, error) {
	return s.createSandboxContract(ctx, req)
}

// GetSandbox 查询单个沙箱状态与工具接入信息。
func (s *Service) GetSandbox(ctx context.Context, tenantID, sandboxID int64) (contracts.SandboxInfo, error) {
	return s.getSandboxContract(ctx, tenantID, sandboxID)
}

// UpdateSandboxAuthorizedAccounts 原子替换沙箱共享账号集合。
func (s *Service) UpdateSandboxAuthorizedAccounts(ctx context.Context, req contracts.SandboxAuthorizedAccountsRequest) error {
	if req.TenantID <= 0 || req.SandboxID <= 0 || strings.TrimSpace(req.SourceRef) == "" {
		return apperr.ErrSandboxContractRequestInvalid
	}
	var revoked []int64
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		sb, err := tx.GetSandbox(ctx, req.TenantID, req.SandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if sb.SourceRef != strings.TrimSpace(req.SourceRef) {
			return apperr.ErrSandboxOwnershipInvalid
		}
		accountIDs := append([]int64(nil), req.AuthorizedAccountIDs...)
		if err := s.validateSandboxSharedAccounts(ctx, req.TenantID, accountIDs); err != nil {
			return err
		}
		accountIDs = append(accountIDs, sb.OwnerAccountID)
		accountIDs, err = normalizeSandboxSharedAccountIDs(sb.OwnerAccountID, accountIDs)
		if err != nil {
			return err
		}
		previous := make(map[int64]struct{})
		previous[sb.OwnerAccountID] = struct{}{}
		for _, accountID := range sb.SharedAccountIDs {
			previous[accountID] = struct{}{}
		}
		next := make(map[int64]struct{}, len(accountIDs))
		for _, accountID := range accountIDs {
			next[accountID] = struct{}{}
		}
		for accountID := range previous {
			if _, ok := next[accountID]; !ok {
				revoked = append(revoked, accountID)
			}
		}
		_, err = tx.UpdateSandboxAuthorizedAccounts(ctx, req.TenantID, req.SandboxID, accountIDs)
		return err
	}); err != nil {
		return err
	}
	for _, accountID := range revoked {
		if err := s.closeSandboxSessions(req.TenantID, accountID, req.SandboxID); err != nil {
			return apperr.ErrSandboxToolProxyUnavailable.WithCause(err)
		}
	}
	return nil
}

// GetSandboxWorkspaceArchive 返回已保存工作区的内部对象引用,仅供实例重建恢复使用。
func (s *Service) GetSandboxWorkspaceArchive(ctx context.Context, tenantID, sandboxID int64) (string, error) {
	if tenantID <= 0 || sandboxID <= 0 {
		return "", apperr.ErrSandboxContractRequestInvalid
	}
	var sb Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		return err
	}); err != nil {
		return "", apperr.ErrSandboxNotFound.WithCause(err)
	}
	if strings.TrimSpace(sb.CodeHash) == "" || strings.TrimSpace(sb.CodeStorageKey) == "" {
		return "", nil
	}
	ref, err := storage.ObjectRefString(s.minio.BucketCode(), sb.CodeStorageKey)
	if err != nil {
		return "", apperr.ErrSandboxFilePersistFailed.WithCause(err)
	}
	return ref, nil
}

// PauseSandbox 暂停单个沙箱。
func (s *Service) PauseSandbox(ctx context.Context, req contracts.SandboxControlRequest) error {
	return s.pauseSandboxContract(ctx, req)
}

// ResumeSandbox 恢复单个沙箱。
func (s *Service) ResumeSandbox(ctx context.Context, req contracts.SandboxControlRequest) error {
	return s.resumeSandboxContract(ctx, req)
}

// DestroySandbox 主动销毁单个沙箱。
func (s *Service) DestroySandbox(ctx context.Context, req contracts.SandboxControlRequest) error {
	return s.destroySandboxContract(ctx, req)
}

// RecycleByScopeRef 按生命周期作用域级联回收沙箱。
func (s *Service) RecycleByScopeRef(ctx context.Context, req contracts.SandboxRecycleRequest) error {
	return s.recycleByScopeRefContract(ctx, req)
}

// PutSandboxFile 把公开文件写入沙箱工作区。
func (s *Service) PutSandboxFile(ctx context.Context, req contracts.SandboxFileWriteRequest) (int64, error) {
	return s.putSandboxFileContract(ctx, req)
}

// PutSandboxPrivateArchive 把隐藏判题归档写入私有执行域。
func (s *Service) PutSandboxPrivateArchive(ctx context.Context, req contracts.SandboxPrivateArchiveInjectRequest) error {
	return s.putSandboxPrivateArchiveContract(ctx, req)
}

// RestoreSandboxArchive 把已授权对象归档恢复到沙箱工作区。
func (s *Service) RestoreSandboxArchive(ctx context.Context, req contracts.SandboxArchiveRestoreRequest) error {
	return s.restoreSandboxArchiveContract(ctx, req)
}

// SaveSandboxFiles 立即保存工作区并返回代码引用与哈希。
func (s *Service) SaveSandboxFiles(ctx context.Context, req contracts.SandboxSaveRequest) (string, string, error) {
	return s.saveSandboxFilesContract(ctx, req)
}

// ExecSandboxCommand 在沙箱内执行受限内部命令。
func (s *Service) ExecSandboxCommand(ctx context.Context, req contracts.SandboxExecRequest) (contracts.SandboxExecResult, error) {
	return s.execSandboxCommandContract(ctx, req)
}

// ChainDeploy 调用统一链部署能力。
func (s *Service) ChainDeploy(ctx context.Context, req contracts.SandboxChainDeployRequest) (map[string]any, error) {
	return s.chainDeployContract(ctx, req)
}

// ChainSendTx 调用统一链交易能力。
func (s *Service) ChainSendTx(ctx context.Context, req contracts.SandboxChainTxRequest) (map[string]any, error) {
	return s.chainSendTxContract(ctx, req)
}

// ChainQuery 调用统一链查询能力。
func (s *Service) ChainQuery(ctx context.Context, req contracts.SandboxChainQueryRequest) (map[string]any, error) {
	return s.chainQueryContract(ctx, req)
}

// ChainReset 调用统一链重置能力。
func (s *Service) ChainReset(ctx context.Context, req contracts.SandboxChainResetRequest) error {
	return s.chainResetContract(ctx, req)
}

// Stats 返回租户级沙箱资源统计。
func (s *Service) Stats(ctx context.Context, tenantID int64) (contracts.SandboxQuotaStats, error) {
	return s.statsContract(ctx, tenantID)
}
