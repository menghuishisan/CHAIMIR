// sandbox service_chain 文件实现跨运行时统一链部署、交易、查询和重置能力。
package sandbox

import (
	"context"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

// ChainDeploy 调用统一链部署能力。
func (s *Service) ChainDeploy(ctx context.Context, req contracts.SandboxChainDeployRequest) (map[string]any, error) {
	if req.TenantID <= 0 || req.SandboxID <= 0 || len(req.Payload) == 0 || !validSourceRef(req.SourceRef) {
		return nil, apperr.ErrSandboxDeployRequestInvalid
	}
	sb, runtime, cap, err := s.chainCapability(ctx, req.TenantID, req.SandboxID, req.SourceRef)
	if err != nil {
		return nil, err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return nil, err
	}
	out, err := cap.Deploy(ctx, sb, runtime, req.Payload)
	if err != nil {
		return nil, apperr.ErrSandboxChainFailed.WithCause(err)
	}
	return out, nil
}

// ChainSendTx 调用统一链交易能力。
func (s *Service) ChainSendTx(ctx context.Context, req contracts.SandboxChainTxRequest) (map[string]any, error) {
	if req.TenantID <= 0 || req.SandboxID <= 0 || len(req.Payload) == 0 || !validSourceRef(req.SourceRef) {
		return nil, apperr.ErrSandboxTxRequestInvalid
	}
	sb, runtime, cap, err := s.chainCapability(ctx, req.TenantID, req.SandboxID, req.SourceRef)
	if err != nil {
		return nil, err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return nil, err
	}
	out, err := cap.SendTx(ctx, sb, runtime, req.Payload)
	if err != nil {
		return nil, apperr.ErrSandboxChainFailed.WithCause(err)
	}
	return out, nil
}

// ChainQuery 调用统一链查询能力。
func (s *Service) ChainQuery(ctx context.Context, req contracts.SandboxChainQueryRequest) (map[string]any, error) {
	if req.TenantID <= 0 || req.SandboxID <= 0 || req.Target == "" || !validSourceRef(req.SourceRef) {
		return nil, apperr.ErrSandboxContractRequestInvalid
	}
	sb, runtime, cap, err := s.chainCapability(ctx, req.TenantID, req.SandboxID, req.SourceRef)
	if err != nil {
		return nil, err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return nil, err
	}
	out, err := cap.Query(ctx, sb, runtime, req.Target)
	if err != nil {
		return nil, apperr.ErrSandboxChainFailed.WithCause(err)
	}
	return out, nil
}

// ChainReset 调用统一链重置能力。
func (s *Service) ChainReset(ctx context.Context, req contracts.SandboxChainResetRequest) error {
	if req.TenantID <= 0 || req.SandboxID <= 0 || !validSourceRef(req.SourceRef) {
		return apperr.ErrSandboxContractRequestInvalid
	}
	sb, runtime, cap, err := s.chainCapability(ctx, req.TenantID, req.SandboxID, req.SourceRef)
	if err != nil {
		return err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return err
	}
	if err := cap.Reset(ctx, sb, runtime); err != nil {
		return apperr.ErrSandboxChainFailed.WithCause(err)
	}
	return nil
}

// chainCapability 查询沙箱运行时并解析 L2 能力实现器。
func (s *Service) chainCapability(ctx context.Context, tenantID, sandboxID int64, sourceRef string) (Sandbox, Runtime, ChainCapability, error) {
	sb, runtime, err := s.sandboxRuntime(ctx, tenantID, sandboxID)
	if err != nil {
		return Sandbox{}, Runtime{}, nil, err
	}
	if sb.SourceRef != strings.TrimSpace(sourceRef) {
		return Sandbox{}, Runtime{}, nil, apperr.ErrSandboxOwnershipInvalid
	}
	if !sandboxExecAllowed(sb) {
		return Sandbox{}, Runtime{}, nil, apperr.ErrSandboxStateInvalid
	}
	cap, err := s.resolveCapability(runtime)
	if err != nil {
		return Sandbox{}, Runtime{}, nil, err
	}
	return sb, runtime, cap, nil
}

// markSandboxExecutionActive 记录链能力调用活跃度,并把 ready/idle 沙箱切回 running。
func (s *Service) markSandboxExecutionActive(ctx context.Context, sb Sandbox) error {
	if !sandboxExecAllowed(sb) {
		return apperr.ErrSandboxStateInvalid
	}
	return s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.MarkSandboxActive(ctx, sb.TenantID, sb.ID); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	})
}

// resolveCapability 只从服务端注册表解析 L2/L3 能力,禁止按 plugin_ref 动态加载任意代码。
func (s *Service) resolveCapability(runtime Runtime) (ChainCapability, error) {
	key := strings.TrimSpace(runtime.CapabilityImpl)
	if runtime.AdapterLevel == 3 {
		key = strings.TrimSpace(runtime.PluginRef)
	}
	if key == "" {
		return nil, apperr.ErrSandboxCapabilityUnavailable
	}
	cap, ok := s.capabilities[key]
	if !ok || cap == nil {
		return nil, apperr.ErrSandboxCapabilityUnavailable
	}
	return cap, nil
}
