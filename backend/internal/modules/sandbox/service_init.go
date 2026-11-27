package sandbox

import (
	"context"
	"fmt"
	"path"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/logging"
)

// validateInitChainSteps 在编译时拒绝无效动作和运行时无法执行的初始化。
func validateInitChainSteps(steps []contracts.CompositionChainStep, runtimes []RuntimePlan) error {
	plan := CreateSandboxPlan{Runtimes: runtimes}
	for i, step := range steps {
		selected, ok := plan.RuntimeInstance(step.RuntimeInstance)
		if !ok || step.RuntimeInstance != selected.InstanceCode {
			return fmt.Errorf("链初始化步骤 %d 引用无效运行时: %w", i+1, apperr.ErrSandboxCreateRequestInvalid)
		}
		var action string
		switch step.Op {
		case "deploy", "tx":
			if len(step.Payload) == 0 || step.Target != "" {
				return apperr.ErrSandboxCreateRequestInvalid
			}
			action = ChainOperationDeploy
			if step.Op == "tx" {
				action = ChainOperationTransaction
			}
		case "query":
			if strings.TrimSpace(step.Target) == "" || len(step.Payload) > 0 {
				return apperr.ErrSandboxCreateRequestInvalid
			}
			action = ChainOperationQuery
		case "reset":
			commands := selected.Runtime.AdapterSpec.CapabilityCommands
			if len(step.Payload) > 0 || step.Target != "" ||
				(selected.Runtime.AdapterLevel != RuntimeAdapterLevelPlugin && commands.ResetStrategy != "recreate_runtime" && len(commands.Reset.Command) == 0) {
				return apperr.ErrSandboxCreateRequestInvalid
			}
			continue
		default:
			return apperr.ErrSandboxCreateRequestInvalid
		}
		if err := ensureRuntimeAction(selected.Runtime, action); err != nil {
			return fmt.Errorf("链初始化步骤 %d 无可用能力: %w", i+1, err)
		}
	}
	return nil
}

// initializeChains 仅在启动资源租约内执行，不能调用面向用户的就绪门禁入口。
func (s *Service) initializeChains(ctx context.Context, plan CreateSandboxPlan) error {
	if err := validateInitChainSteps(plan.InitChainSteps, plan.Runtimes); err != nil {
		return err
	}
	for i, step := range plan.InitChainSteps {
		selected, _ := plan.RuntimeInstance(step.RuntimeInstance)
		capability, err := s.resolveCapability(selected.Runtime)
		if err != nil {
			return err
		}
		switch step.Op {
		case "deploy":
			_, err = capability.Deploy(ctx, plan.Sandbox, selected.Runtime, step.Payload)
		case "tx":
			_, err = capability.SendTx(ctx, plan.Sandbox, selected.Runtime, step.Payload)
		case "query":
			_, err = capability.Query(ctx, plan.Sandbox, selected.Runtime, step.Target)
		case "reset":
			err = s.resetRuntimeForPlan(ctx, plan, step.RuntimeInstance, capability)
		}
		if err != nil {
			return apperr.ErrSandboxInitExecFailed.WithCause(fmt.Errorf("链初始化步骤 %d (%s/%s) 失败: %w", i+1, step.RuntimeInstance, step.Op, err))
		}
	}
	return nil
}

func (s *Service) prepareRuntimeInitArchives(ctx context.Context, sb Sandbox, runtimes []RuntimePlan) ([]RuntimeInitArchive, error) {
	if sb.TenantID < 0 {
		return nil, apperr.ErrSandboxInitObjectRefInvalid
	}
	type archiveSource struct {
		ref  storage.ObjectRef
		hash string
	}
	prepared := make(map[archiveSource][]byte)
	var archives []RuntimeInitArchive
	for _, runtime := range runtimes {
		if err := validateInitAssets(runtime.Runtime.AdapterSpec); err != nil {
			return nil, err
		}
		for _, asset := range runtime.Runtime.AdapterSpec.InitAssets {
			ref, err := storage.ParseObjectRef(asset.SourceRef)
			if err != nil {
				return nil, apperr.ErrSandboxInitObjectRefInvalid.WithCause(err)
			}
			ownerID := sb.TenantID
			// 共享素材由已自检目录带入；教师追加引用在组合编译时限定为本租户。
			if strings.HasPrefix(ref.Key, "0/") {
				ownerID = 0
			}
			source := archiveSource{ref: ref, hash: strings.ToLower(strings.TrimSpace(asset.ExpectedSHA256))}
			tarball, exists := prepared[source]
			if !exists {
				tarball, err = s.readInitArchive(ctx, ownerID, ref, asset.ExpectedSHA256)
				if err != nil {
					return nil, err
				}
				prepared[source] = tarball
			}
			archives = append(archives, RuntimeInitArchive{RuntimeInstance: runtime.InstanceCode, TargetDomain: asset.TargetDomain, Tarball: tarball})
		}
	}
	return archives, nil
}

func (s *Service) readInitObject(ctx context.Context, ownerID int64, ref storage.ObjectRef) ([]byte, error) {
	if err := validateInitObjectRef(ownerID, ref, s.objectStorage.BucketCode(), s.objectStorage.BucketAttach()); err != nil {
		return nil, err
	}
	reader, err := s.objectStorage.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return nil, apperr.ErrSandboxInitObjectReadFailed.WithCause(err)
	}
	defer logging.CloseContext(ctx, "关闭沙箱初始化对象读取器失败", reader)
	data, sizeResult, err := upload.ReadBounded(reader, s.cfg.InitArchiveMaxBytes)
	if err != nil {
		return nil, apperr.ErrSandboxInitObjectReadFailed.WithCause(err)
	}
	if sizeResult == upload.SizeEmpty || sizeResult == upload.SizeTooLarge {
		return nil, apperr.ErrSandboxInitArchiveTooLarge
	}
	return data, nil
}

func (s *Service) readInitArchive(ctx context.Context, ownerID int64, ref storage.ObjectRef, expectedHash string) ([]byte, error) {
	data, err := s.readInitObject(ctx, ownerID, ref)
	if err != nil {
		return nil, err
	}
	if hash := strings.TrimSpace(expectedHash); hash != "" && !strings.EqualFold(hash, pkgcrypto.SHA256Hex(data)) {
		return nil, apperr.ErrSandboxInitArchiveInvalid
	}
	tarball, err := upload.SafeArchiveTar(ref.Key, data, upload.ArchiveLimits{MaxFiles: s.cfg.InitArchiveMaxFiles, MaxUnpackedBytes: s.cfg.InitArchiveMaxUnpackedBytes})
	if err != nil {
		return nil, apperr.ErrSandboxInitArchiveInvalid.WithCause(err)
	}
	return tarball, nil
}

// componentCanWriteDomain 确认初始化归档的执行目标显式拥有目标域写权限,避免借用学生容器或只读挂载。
func componentCanWriteDomain(adapter AdapterSpec, target, domainName string) bool {
	parts := strings.SplitN(strings.TrimSpace(target), "/", 2)
	if len(parts) != 2 {
		return false
	}
	for _, pod := range podTopologyForAdapter(adapter) {
		if pod.Name != strings.TrimSpace(parts[0]) {
			continue
		}
		for _, component := range pod.Containers {
			if component.Name != strings.TrimSpace(parts[1]) || studentAccessibleContainer(component) {
				continue
			}
			for _, raw := range component.WritableMountDomains {
				if strings.TrimSpace(raw) == strings.TrimSpace(domainName) {
					return true
				}
			}
		}
	}
	return false
}

func (s *Service) restoreArchiveToWorkspace(ctx context.Context, sb Sandbox, runtime Runtime, objectRef, expectedHash, targetDir string) error {
	if sb.TenantID <= 0 {
		return apperr.ErrSandboxInitObjectRefInvalid
	}
	ref, err := storage.ParseObjectRef(objectRef)
	if err != nil {
		return apperr.ErrSandboxInitObjectRefInvalid.WithCause(err)
	}
	tarball, err := s.readInitArchive(ctx, sb.TenantID, ref, expectedHash)
	if err != nil {
		return err
	}
	target := runtime.AdapterSpec.WorkspaceDir
	if strings.TrimSpace(targetDir) != "" && strings.TrimSpace(targetDir) != "." {
		relative, err := validateWorkspaceListPath(targetDir)
		if err != nil {
			return err
		}
		target = path.Join(runtime.AdapterSpec.WorkspaceDir, relative)
	}
	tarball, err = workload.SanitizeWorkspaceArchive(tarball)
	if err != nil {
		return apperr.ErrSandboxInitArchiveInvalid.WithCause(err)
	}
	command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.UnpackTar, runtime.AdapterSpec.WorkspaceDir, target)
	if _, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, workspaceExecTarget(runtime), command, tarball, false); err != nil {
		return sandboxExecFailure(apperr.ErrSandboxInitExecFailed, err, stderr)
	}
	return nil
}

// 公开归档只恢复到当前租户的工作区，不承载判题私有材料。
func (s *Service) RestoreSandboxArchive(ctx context.Context, req contracts.SandboxArchiveRestoreRequest) error {
	if req.TenantID <= 0 || req.SandboxID <= 0 || !validSourceRef(req.SourceRef) || strings.TrimSpace(req.ObjectRef) == "" {
		return apperr.ErrSandboxCreateRequestInvalid
	}
	sb, runtime, err := s.sandboxRuntimeForSource(ctx, req.TenantID, req.SandboxID, req.SourceRef)
	if err != nil {
		return err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return err
	}
	return s.restoreArchiveToWorkspace(ctx, sb, runtime, req.ObjectRef, req.ExpectedHash, req.TargetDir)
}

// runInitScriptIfNeeded 仅向非学生容器传递脚本原文,不把私有初始化材料写入公开工作区。
func (s *Service) runInitScriptIfNeeded(ctx context.Context, sb Sandbox, runtime Runtime, initScriptRef string) error {
	if sb.TenantID <= 0 {
		return apperr.ErrSandboxInitObjectRefInvalid
	}
	target, err := initScriptExecTarget(runtime.AdapterSpec)
	if err != nil {
		return err
	}
	ref, err := storage.ParseObjectRef(initScriptRef)
	if err != nil {
		return apperr.ErrSandboxInitObjectRefInvalid.WithCause(err)
	}
	data, err := s.readInitObject(ctx, sb.TenantID, ref)
	if err != nil {
		return err
	}
	runCommand := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.RunScript, runtime.AdapterSpec.WorkspaceDir, runtime.AdapterSpec.WorkspaceDir)
	if _, _, err := s.orchestrator.Exec(ctx, sb.Namespace, target, runCommand, data, false); err != nil {
		// shell 语法错误和脚本自带输出可能回显私有源码,只保留执行错误链,不持久化标准流。
		return sandboxExecFailure(apperr.ErrSandboxInitExecFailed, err, nil)
	}
	return s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		detail, err := jsonBytes(map[string]any{"script_ref": initScriptRef})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeExec, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	})
}

// initScriptExecTarget 要求脚本在独立非学生容器执行,且临时文件不能进入共享卷。
func initScriptExecTarget(adapter AdapterSpec) (string, error) {
	target := strings.TrimSpace(adapter.WorkspaceOps.ScriptExecTarget)
	if !explicitExecTarget(target) || target == strings.TrimSpace(adapter.WorkspaceOps.ExecTarget) {
		return "", apperr.ErrSandboxWorkspaceOpsInvalid
	}
	if !workload.ValidNonShellCommand(adapter.WorkspaceOps.RunScript) || !workload.ValidNonShellCommand(adapter.WorkspaceOps.CleanWorkspace) {
		return "", apperr.ErrSandboxWorkspaceOpsInvalid
	}
	for _, pod := range podTopologyForAdapter(adapter) {
		for _, component := range pod.Containers {
			if pod.Name+"/"+component.Name != target {
				continue
			}
			if studentAccessibleContainer(component) || !shouldMountWorkspace(component) || len(component.SecretEnv) > 0 || len(component.WritableMountDomains) > 0 {
				return "", apperr.ErrSandboxWorkspaceOpsInvalid
			}
			for _, domain := range component.MountDomains {
				if strings.TrimSpace(domain) != VolumeDomainPublicAssets {
					return "", apperr.ErrSandboxWorkspaceOpsInvalid
				}
			}
			for _, mount := range component.EphemeralMounts {
				if path.Clean(mount.MountPath) == "/tmp" {
					return target, nil
				}
			}
			return "", apperr.ErrSandboxWorkspaceOpsInvalid
		}
	}
	return "", apperr.ErrSandboxWorkspaceOpsInvalid
}

// cleanWorkspaceBeforeAccess 在工作台开放前经非学生容器清除历史平台副本,失败保持不可操作。
func (s *Service) cleanWorkspaceBeforeAccess(ctx context.Context, sb Sandbox, runtime Runtime) error {
	target, err := initScriptExecTarget(runtime.AdapterSpec)
	if err != nil {
		return err
	}
	command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.CleanWorkspace, runtime.AdapterSpec.WorkspaceDir, runtime.AdapterSpec.WorkspaceDir)
	if _, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, target, command, nil, false); err != nil {
		return sandboxExecFailure(apperr.ErrSandboxInitExecFailed, err, stderr)
	}
	return nil
}

// validateInitObjectRef 匹配明确所有者的代码或附件对象；0 仅由平台素材路径使用。
func validateInitObjectRef(ownerID int64, ref storage.ObjectRef, codeBucket, attachBucket string) error {
	if ownerID < 0 {
		return apperr.ErrSandboxInitObjectRefInvalid
	}
	if ref.Bucket != codeBucket && ref.Bucket != attachBucket {
		return apperr.ErrSandboxInitObjectRefInvalid
	}
	prefix := ids.Format(ownerID) + "/"
	if !strings.HasPrefix(ref.Key, prefix) {
		return apperr.ErrSandboxInitObjectRefInvalid
	}
	return nil
}

// 缺失工作区实例时交由执行目标校验拒绝，不按运行时顺序回退。
func workspaceRuntimeForPlan(plan CreateSandboxPlan) Runtime {
	runtime, ok := plan.RuntimeInstance(plan.WorkspaceRuntimeInstance)
	if !ok {
		return Runtime{}
	}
	return runtime.Runtime
}
