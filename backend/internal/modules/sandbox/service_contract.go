// sandbox service_contract 文件实现 M2 对聚合和业务模块开放的统一沙箱能力契约。
package sandbox

import (
	"context"

	"chaimir/internal/contracts"
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

// RecycleBySourceRef 按来源标识级联回收沙箱。
func (s *Service) RecycleBySourceRef(ctx context.Context, req contracts.SandboxRecycleRequest) error {
	return s.recycleBySourceRefContract(ctx, req)
}

// PutSandboxFile 把公开文件写入沙箱工作区。
func (s *Service) PutSandboxFile(ctx context.Context, req contracts.SandboxFileWriteRequest) error {
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
