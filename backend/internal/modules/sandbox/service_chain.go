// sandbox service_chain 文件实现跨运行时统一链部署、交易、查询和重置能力。
package sandbox

import (
	"context"
	"encoding/json"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

// ChainDeploy 调用统一链部署能力。
func (s *Service) chainDeployContract(ctx context.Context, req contracts.SandboxChainDeployRequest) (map[string]any, error) {
	if req.TenantID <= 0 || req.SandboxID <= 0 || strings.TrimSpace(req.RuntimeInstance) == "" || len(req.Payload) == 0 || !validSourceRef(req.SourceRef) {
		return nil, apperr.ErrSandboxDeployRequestInvalid
	}
	var out map[string]any
	err := s.withSandboxChainLock(ctx, req.TenantID, req.SandboxID, func() error {
		sb, runtime, cap, err := s.chainCapability(ctx, req.TenantID, req.SandboxID, req.SourceRef, req.RuntimeInstance)
		if err != nil {
			return err
		}
		if err := ensureRuntimeAction(runtime, ChainOperationDeploy); err != nil {
			return err
		}
		if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
			return err
		}
		out, err = cap.Deploy(ctx, sb, runtime, req.Payload)
		if err != nil {
			return apperr.ErrSandboxChainFailed.WithCause(err)
		}
		return nil
	})
	return out, err
}

// runtimeForSandboxInstance 从不可变组合快照选择指定运行时实例,并恢复编译后的适配器。
func (s *Service) runtimeForSandboxInstance(ctx context.Context, sb Sandbox, instanceCode string) (Runtime, error) {
	instanceCode = strings.TrimSpace(instanceCode)
	if instanceCode == "" {
		return Runtime{}, apperr.ErrSandboxContractRequestInvalid
	}
	var snapshot contracts.SandboxCompositionSnapshot
	if len(sb.CompositionSnapshot) == 0 {
		return Runtime{}, apperr.ErrSandboxCapabilityUnavailable
	}
	if err := json.Unmarshal(sb.CompositionSnapshot, &snapshot); err != nil {
		return Runtime{}, apperr.ErrSandboxCapabilityUnavailable.WithCause(err)
	}
	var frozen contracts.CompiledRuntimeSnapshot
	found := false
	for _, item := range snapshot.Runtimes {
		if strings.TrimSpace(item.InstanceCode) == instanceCode {
			if found {
				return Runtime{}, apperr.ErrSandboxCapabilityUnavailable
			}
			frozen = item
			found = true
		}
	}
	if !found || frozen.RuntimeID <= 0 || strings.TrimSpace(frozen.Code) == "" || len(frozen.AdapterSpec) == 0 {
		return Runtime{}, apperr.ErrSandboxCapabilityUnavailable
	}
	var runtime Runtime
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		runtime, err = tx.GetRuntimeByID(ctx, frozen.RuntimeID)
		return err
	}); err != nil {
		return Runtime{}, apperr.ErrSandboxRuntimeNotFound.WithCause(err)
	}
	if runtime.Code != frozen.Code || runtime.Status != RuntimeStatusAvailable || runtime.SelftestStatus != RuntimeSelftestPassed {
		return Runtime{}, apperr.ErrSandboxCapabilityUnavailable
	}
	var compiled AdapterSpec
	if err := json.Unmarshal(frozen.AdapterSpec, &compiled); err != nil {
		return Runtime{}, apperr.ErrSandboxCapabilityUnavailable.WithCause(err)
	}
	runtime.AdapterSpec = compiled
	runtime.AdapterLevel = frozen.AdapterLevel
	runtime.CapabilityImpl = frozen.CapabilityImpl
	return runtime, nil
}

// ChainSendTx 调用统一链交易能力。
func (s *Service) chainSendTxContract(ctx context.Context, req contracts.SandboxChainTxRequest) (map[string]any, error) {
	if req.TenantID <= 0 || req.SandboxID <= 0 || strings.TrimSpace(req.RuntimeInstance) == "" || len(req.Payload) == 0 || !validSourceRef(req.SourceRef) {
		return nil, apperr.ErrSandboxTxRequestInvalid
	}
	var out map[string]any
	err := s.withSandboxChainLock(ctx, req.TenantID, req.SandboxID, func() error {
		sb, runtime, cap, err := s.chainCapability(ctx, req.TenantID, req.SandboxID, req.SourceRef, req.RuntimeInstance)
		if err != nil {
			return err
		}
		if err := ensureRuntimeAction(runtime, ChainOperationTransaction); err != nil {
			return err
		}
		if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
			return err
		}
		out, err = cap.SendTx(ctx, sb, runtime, req.Payload)
		if err != nil {
			return apperr.ErrSandboxChainFailed.WithCause(err)
		}
		return nil
	})
	return out, err
}

// ChainQuery 调用统一链查询能力。
func (s *Service) chainQueryContract(ctx context.Context, req contracts.SandboxChainQueryRequest) (map[string]any, error) {
	if req.TenantID <= 0 || req.SandboxID <= 0 || strings.TrimSpace(req.RuntimeInstance) == "" || req.Target == "" || !validSourceRef(req.SourceRef) {
		return nil, apperr.ErrSandboxContractRequestInvalid
	}
	sb, runtime, cap, err := s.chainCapability(ctx, req.TenantID, req.SandboxID, req.SourceRef, req.RuntimeInstance)
	if err != nil {
		return nil, err
	}
	if err := ensureRuntimeAction(runtime, ChainOperationQuery); err != nil {
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
func (s *Service) chainResetContract(ctx context.Context, req contracts.SandboxChainResetRequest) error {
	if req.TenantID <= 0 || req.SandboxID <= 0 || strings.TrimSpace(req.RuntimeInstance) == "" || !validSourceRef(req.SourceRef) {
		return apperr.ErrSandboxContractRequestInvalid
	}
	return s.withSandboxChainLock(ctx, req.TenantID, req.SandboxID, func() error {
		sb, runtime, cap, err := s.chainCapability(ctx, req.TenantID, req.SandboxID, req.SourceRef, req.RuntimeInstance)
		if err != nil {
			return err
		}
		if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
			return err
		}
		if strings.TrimSpace(runtime.AdapterSpec.CapabilityCommands.ResetStrategy) == "recreate_runtime" {
			plan, planErr := s.planForExistingSandbox(ctx, sb)
			if planErr != nil {
				return planErr
			}
			if err := s.orchestrator.ResetSandboxRuntime(ctx, plan); err != nil {
				return apperr.ErrSandboxChainFailed.WithCause(err)
			}
			return nil
		}
		if len(runtime.AdapterSpec.CapabilityCommands.Reset.Command) == 0 {
			return apperr.ErrSandboxCapabilityUnavailable
		}
		if err := cap.Reset(ctx, sb, runtime); err != nil {
			return apperr.ErrSandboxChainFailed.WithCause(err)
		}
		return nil
	})
}

// withSandboxChainLock 持有数据库事务级 advisory lock,覆盖真实插件调用的整个生命周期。
func (s *Service) withSandboxChainLock(ctx context.Context, tenantID, sandboxID int64, fn func() error) error {
	return s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.LockSandboxChain(ctx, tenantID, sandboxID); err != nil {
			return apperr.ErrSandboxChainFailed.WithCause(err)
		}
		return fn()
	})
}

// ChainDeployForOwner 校验用户归属后调用统一链部署能力,供用户工作台使用。
func (s *Service) ChainDeployForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, runtimeInstance string, payload map[string]any) (map[string]any, error) {
	if tenantID <= 0 || accountID <= 0 || sandboxID <= 0 || strings.TrimSpace(runtimeInstance) == "" || len(payload) == 0 {
		return nil, apperr.ErrSandboxDeployRequestInvalid
	}
	var out map[string]any
	err := s.withSandboxChainLock(ctx, tenantID, sandboxID, func() error {
		sb, runtime, cap, err := s.chainCapabilityForOwner(ctx, tenantID, accountID, sandboxID, runtimeInstance)
		if err != nil {
			return err
		}
		if err := ensureRuntimeAction(runtime, ChainOperationDeploy); err != nil {
			return err
		}
		if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
			return err
		}
		out, err = cap.Deploy(ctx, sb, runtime, payload)
		if err != nil {
			return apperr.ErrSandboxChainFailed.WithCause(err)
		}
		return nil
	})
	return out, err
}

// ChainSendTxForOwner 校验用户归属后调用统一链交易能力,避免工具容器直连链节点。
func (s *Service) ChainSendTxForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, runtimeInstance string, payload map[string]any) (map[string]any, error) {
	if tenantID <= 0 || accountID <= 0 || sandboxID <= 0 || strings.TrimSpace(runtimeInstance) == "" || len(payload) == 0 {
		return nil, apperr.ErrSandboxTxRequestInvalid
	}
	var out map[string]any
	err := s.withSandboxChainLock(ctx, tenantID, sandboxID, func() error {
		sb, runtime, cap, err := s.chainCapabilityForOwner(ctx, tenantID, accountID, sandboxID, runtimeInstance)
		if err != nil {
			return err
		}
		if err := ensureRuntimeAction(runtime, ChainOperationTransaction); err != nil {
			return err
		}
		if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
			return err
		}
		out, err = cap.SendTx(ctx, sb, runtime, payload)
		if err != nil {
			return apperr.ErrSandboxChainFailed.WithCause(err)
		}
		return nil
	})
	return out, err
}

// ChainQueryForOwner 校验用户归属后调用统一链查询能力,返回前由能力实现负责结果脱敏。
func (s *Service) ChainQueryForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, runtimeInstance, target string) (map[string]any, error) {
	if tenantID <= 0 || accountID <= 0 || sandboxID <= 0 || strings.TrimSpace(runtimeInstance) == "" || target == "" {
		return nil, apperr.ErrSandboxContractRequestInvalid
	}
	sb, runtime, cap, err := s.chainCapabilityForOwner(ctx, tenantID, accountID, sandboxID, runtimeInstance)
	if err != nil {
		return nil, err
	}
	if err := ensureRuntimeAction(runtime, ChainOperationQuery); err != nil {
		return nil, err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return nil, err
	}
	out, err := cap.Query(ctx, sb, runtime, target)
	if err != nil {
		return nil, apperr.ErrSandboxChainFailed.WithCause(err)
	}
	return out, nil
}

// chainCapability 查询沙箱运行时并解析 L2 能力实现器。
func (s *Service) chainCapability(ctx context.Context, tenantID, sandboxID int64, sourceRef, runtimeInstance string) (Sandbox, Runtime, ChainCapability, error) {
	sb, _, err := s.sandboxRuntime(ctx, tenantID, sandboxID)
	if err != nil {
		return Sandbox{}, Runtime{}, nil, err
	}
	if sb.SourceRef != strings.TrimSpace(sourceRef) {
		return Sandbox{}, Runtime{}, nil, apperr.ErrSandboxOwnershipInvalid
	}
	if !sandboxExecAllowed(sb) {
		return Sandbox{}, Runtime{}, nil, apperr.ErrSandboxStateInvalid
	}
	runtime, err := s.runtimeForSandboxInstance(ctx, sb, runtimeInstance)
	if err != nil {
		return Sandbox{}, Runtime{}, nil, err
	}
	cap, err := s.resolveCapability(runtime)
	if err != nil {
		return Sandbox{}, Runtime{}, nil, err
	}
	return sb, runtime, cap, nil
}

// chainCapabilityForOwner 查询用户自己的沙箱运行时并解析 L2 能力实现器。
func (s *Service) chainCapabilityForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, runtimeInstance string) (Sandbox, Runtime, ChainCapability, error) {
	sb, _, err := s.sandboxRuntimeForOwner(ctx, tenantID, accountID, sandboxID)
	if err != nil {
		return Sandbox{}, Runtime{}, nil, err
	}
	if !sandboxExecAllowed(sb) {
		return Sandbox{}, Runtime{}, nil, apperr.ErrSandboxStateInvalid
	}
	runtime, err := s.runtimeForSandboxInstance(ctx, sb, runtimeInstance)
	if err != nil {
		return Sandbox{}, Runtime{}, nil, err
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
	if runtime.AdapterLevel == RuntimeAdapterLevelPlugin {
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

// ensureRuntimeAction enforces the runtime's declared native action boundary before invoking a capability.
func ensureRuntimeAction(runtime Runtime, action string) error {
	if runtime.AdapterLevel == RuntimeAdapterLevelPlugin {
		return nil
	}
	var command []string
	switch action {
	case ChainOperationDeploy:
		command = runtime.AdapterSpec.CapabilityCommands.Deploy.Command
	case ChainOperationTransaction:
		command = runtime.AdapterSpec.CapabilityCommands.Tx.Command
	case ChainOperationQuery:
		command = runtime.AdapterSpec.CapabilityCommands.Query.Command
	default:
		return apperr.ErrSandboxCapabilityUnavailable
	}
	if len(command) == 0 {
		return apperr.ErrSandboxCapabilityUnavailable
	}
	return nil
}
