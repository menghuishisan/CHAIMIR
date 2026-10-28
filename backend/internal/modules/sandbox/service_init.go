// sandbox service_init 文件实现初始化归档恢复、私有脚本执行与开放前清理,复用对象存储和上传安全原语。
package sandbox

import (
	"context"
	"encoding/hex"
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

// applyInitAssetsIfNeeded 在个性化阶段注入运行时声明的已审核初始化资产。
func (s *Service) applyInitAssetsIfNeeded(ctx context.Context, sb Sandbox, runtime Runtime) error {
	for _, asset := range runtime.AdapterSpec.InitAssets {
		if strings.TrimSpace(asset.Name) == "" || strings.TrimSpace(asset.SourceRef) == "" || strings.TrimSpace(asset.TargetDomain) == "" {
			return apperr.ErrSandboxInitAssetConfigInvalid
		}
		domain, ok := volumeDomainByName(runtime.AdapterSpec, asset.TargetDomain)
		if !ok || (domain.Name != VolumeDomainPublicAssets && domain.Name != VolumeDomainRuntimeState) || domain.StudentAccess == VolumeAccessReadWrite {
			return apperr.ErrSandboxInitAssetConfigInvalid
		}
		if hash := strings.TrimSpace(asset.ExpectedSHA256); hash != "" {
			decoded, err := hex.DecodeString(hash)
			if err != nil || len(decoded) != 32 {
				return apperr.ErrSandboxInitAssetConfigInvalid
			}
		}
		phase := strings.TrimSpace(asset.ApplyPhase)
		if phase != "" &&
			!strings.EqualFold(phase, InitAssetApplyPhaseInit) &&
			!strings.EqualFold(phase, InitAssetApplyPhasePersonalization) {
			return apperr.ErrSandboxInitAssetConfigInvalid
		}
		if err := s.restoreArchiveToDomain(ctx, sb, runtime, asset.SourceRef, asset.ExpectedSHA256, domain); err != nil {
			return err
		}
	}
	return nil
}

// restoreArchiveToDomain 把审核归档解包到运行时声明的非学生安全域,不借用 workspace 文件接口。
func (s *Service) restoreArchiveToDomain(ctx context.Context, sb Sandbox, runtime Runtime, objectRef, expectedHash string, domain VolumeDomainSpec) error {
	ref, err := storage.ParseObjectRef(objectRef)
	if err != nil {
		return apperr.ErrSandboxInitObjectRefInvalid.WithCause(err)
	}
	if err := validateInitObjectRef(sb.TenantID, ref, s.objectStorage.BucketCode(), s.objectStorage.BucketAttach()); err != nil {
		return err
	}
	reader, err := s.objectStorage.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return apperr.ErrSandboxInitObjectReadFailed.WithCause(err)
	}
	defer logging.CloseContext(ctx, "关闭沙箱初始化归档读取器失败", reader)
	data, sizeResult, err := upload.ReadBounded(reader, s.cfg.InitArchiveMaxBytes)
	if err != nil {
		return apperr.ErrSandboxInitObjectReadFailed.WithCause(err)
	}
	if sizeResult == upload.SizeEmpty || sizeResult == upload.SizeTooLarge {
		return apperr.ErrSandboxInitArchiveTooLarge
	}
	if hash := strings.TrimSpace(expectedHash); hash != "" && !strings.EqualFold(hash, pkgcrypto.SHA256Hex(data)) {
		return apperr.ErrSandboxInitArchiveInvalid
	}
	tarball, err := upload.SafeArchiveTar(ref.Key, data, upload.ArchiveLimits{MaxFiles: s.cfg.InitArchiveMaxFiles, MaxUnpackedBytes: s.cfg.InitArchiveMaxUnpackedBytes})
	if err != nil {
		return apperr.ErrSandboxInitArchiveInvalid.WithCause(err)
	}
	ops := runtime.AdapterSpec.InitAssetOps
	if !componentCanWriteDomain(runtime.AdapterSpec, ops.ExecTarget, domain.Name) {
		return apperr.ErrSandboxInitAssetConfigInvalid
	}
	command := workspaceCommand(ops.UnpackTar, runtime.AdapterSpec.WorkspaceDir, domain.MountPath)
	if _, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, ops.ExecTarget, command, tarball, false); err != nil {
		return sandboxExecFailure(apperr.ErrSandboxInitExecFailed, err, stderr)
	}
	return nil
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

// restoreInitCodeIfNeeded 在个性化初始化阶段把已授权代码归档恢复到工作区。
func (s *Service) restoreInitCodeIfNeeded(ctx context.Context, sb Sandbox, runtime Runtime, initCodeRef string) error {
	return s.restoreArchiveToWorkspace(ctx, sb, runtime, initCodeRef)
}

// restoreArchiveToWorkspace 把对象存储中的已校验归档安全解包到沙箱工作区。
func (s *Service) restoreArchiveToWorkspace(ctx context.Context, sb Sandbox, runtime Runtime, objectRef string) error {
	return s.restoreArchiveToWorkspaceDir(ctx, sb, runtime, objectRef, ".")
}

// restoreArchiveToWorkspaceDir 把对象存储中的已校验归档安全解包到沙箱工作区指定子目录。
func (s *Service) restoreArchiveToWorkspaceDir(ctx context.Context, sb Sandbox, runtime Runtime, objectRef, targetDir string) error {
	return s.restoreArchiveToWorkspaceDirChecked(ctx, sb, runtime, objectRef, "", targetDir)
}

// restoreArchiveToWorkspaceDirChecked 校验对象归属、可选哈希和归档安全后恢复到工作区。
func (s *Service) restoreArchiveToWorkspaceDirChecked(ctx context.Context, sb Sandbox, runtime Runtime, objectRef, expectedHash, targetDir string) error {
	ref, err := storage.ParseObjectRef(objectRef)
	if err != nil {
		return apperr.ErrSandboxInitObjectRefInvalid.WithCause(err)
	}
	if err := validateInitObjectRef(sb.TenantID, ref, s.objectStorage.BucketCode(), s.objectStorage.BucketAttach()); err != nil {
		return err
	}
	reader, err := s.objectStorage.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return apperr.ErrSandboxInitObjectReadFailed.WithCause(err)
	}
	defer logging.CloseContext(ctx, "关闭沙箱初始化归档读取器失败", reader)
	data, sizeResult, err := upload.ReadBounded(reader, s.cfg.InitArchiveMaxBytes)
	if err != nil {
		return apperr.ErrSandboxInitObjectReadFailed.WithCause(err)
	}
	if sizeResult == upload.SizeEmpty || sizeResult == upload.SizeTooLarge {
		return apperr.ErrSandboxInitArchiveTooLarge
	}
	if strings.TrimSpace(expectedHash) != "" && !strings.EqualFold(strings.TrimSpace(expectedHash), pkgcrypto.SHA256Hex(data)) {
		return apperr.ErrSandboxInitArchiveInvalid
	}
	tarball, err := upload.SafeArchiveTar(ref.Key, data, upload.ArchiveLimits{MaxFiles: s.cfg.InitArchiveMaxFiles, MaxUnpackedBytes: s.cfg.InitArchiveMaxUnpackedBytes})
	if err != nil {
		return apperr.ErrSandboxInitArchiveInvalid.WithCause(err)
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

// RestoreSandboxArchive 将公开归档恢复到工作区子目录,供上层编排真实代码对局等场景使用。
func (s *Service) restoreSandboxArchiveContract(ctx context.Context, req contracts.SandboxArchiveRestoreRequest) error {
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
	return s.restoreArchiveToWorkspaceDirChecked(ctx, sb, runtime, req.ObjectRef, req.ExpectedHash, req.TargetDir)
}

// runInitScriptIfNeeded 仅向非学生容器传递脚本原文,不把私有初始化材料写入公开工作区。
func (s *Service) runInitScriptIfNeeded(ctx context.Context, sb Sandbox, runtime Runtime, initScriptRef string) error {
	target, err := initScriptExecTarget(runtime.AdapterSpec)
	if err != nil {
		return err
	}
	ref, err := storage.ParseObjectRef(initScriptRef)
	if err != nil {
		return apperr.ErrSandboxInitObjectRefInvalid.WithCause(err)
	}
	if err := validateInitObjectRef(sb.TenantID, ref, s.objectStorage.BucketCode(), s.objectStorage.BucketAttach()); err != nil {
		return err
	}
	reader, err := s.objectStorage.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return apperr.ErrSandboxInitObjectReadFailed.WithCause(err)
	}
	defer logging.CloseContext(ctx, "关闭沙箱初始化脚本读取器失败", reader)
	data, sizeResult, err := upload.ReadBounded(reader, s.cfg.InitArchiveMaxBytes)
	if err != nil {
		return apperr.ErrSandboxInitObjectReadFailed.WithCause(err)
	}
	if sizeResult == upload.SizeEmpty || sizeResult == upload.SizeTooLarge {
		return apperr.ErrSandboxInitArchiveTooLarge
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

// validateInitObjectRef 校验初始化对象只来自当前租户的代码或附件桶,防止内部调用误传跨租户对象。
func validateInitObjectRef(tenantID int64, ref storage.ObjectRef, codeBucket, attachBucket string) error {
	if tenantID <= 0 {
		return apperr.ErrSandboxInitObjectRefInvalid
	}
	if ref.Bucket != codeBucket && ref.Bucket != attachBucket {
		return apperr.ErrSandboxInitObjectRefInvalid
	}
	prefix := ids.Format(tenantID) + "/"
	if !strings.HasPrefix(ref.Key, prefix) {
		return apperr.ErrSandboxInitObjectRefInvalid
	}
	return nil
}
