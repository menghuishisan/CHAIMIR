// sandbox service_init 文件实现初始化代码归档恢复,统一复用对象存储与上传安全原语。
package sandbox

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// applyInitAssetsIfNeeded 在个性化阶段注入运行时声明的已审核初始化资产。
func (s *Service) applyInitAssetsIfNeeded(ctx context.Context, sb Sandbox, runtime Runtime) error {
	for _, asset := range runtime.AdapterSpec.InitAssets {
		if strings.TrimSpace(asset.SourceRef) == "" {
			return apperr.ErrSandboxInitAssetConfigInvalid
		}
		phase := strings.TrimSpace(asset.ApplyPhase)
		if phase != "" &&
			!strings.EqualFold(phase, InitAssetApplyPhaseInit) &&
			!strings.EqualFold(phase, InitAssetApplyPhasePersonalization) {
			return apperr.ErrSandboxInitAssetConfigInvalid
		}
		if err := s.restoreArchiveToWorkspace(ctx, sb, runtime, asset.SourceRef); err != nil {
			return err
		}
	}
	return nil
}

// restoreInitCodeIfNeeded 在个性化初始化阶段把已授权代码归档恢复到工作区。
func (s *Service) restoreInitCodeIfNeeded(ctx context.Context, sb Sandbox, runtime Runtime, initCodeRef string) error {
	return s.restoreArchiveToWorkspace(ctx, sb, runtime, initCodeRef)
}

// restoreArchiveToWorkspace 把对象存储中的已校验归档安全解包到沙箱工作区。
func (s *Service) restoreArchiveToWorkspace(ctx context.Context, sb Sandbox, runtime Runtime, objectRef string) error {
	ref, err := storage.ParseObjectRef(objectRef)
	if err != nil {
		return apperr.ErrSandboxInitObjectRefInvalid.WithCause(err)
	}
	if err := validateInitObjectRef(sb.TenantID, ref, s.minio.BucketCode(), s.minio.BucketAttach()); err != nil {
		return err
	}
	reader, err := s.minio.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return apperr.ErrSandboxInitObjectReadFailed.WithCause(err)
	}
	defer logging.CloseContext(ctx, "关闭沙箱初始化归档读取器失败", reader)
	data, err := io.ReadAll(io.LimitReader(reader, s.cfg.InitArchiveMaxBytes+1))
	if err != nil {
		return apperr.ErrSandboxInitObjectReadFailed.WithCause(err)
	}
	if int64(len(data)) > s.cfg.InitArchiveMaxBytes {
		return apperr.ErrSandboxInitArchiveTooLarge
	}
	tarball, err := upload.SafeArchiveTar(ref.Key, data, upload.ArchiveLimits{MaxFiles: s.cfg.InitArchiveMaxFiles, MaxUnpackedBytes: s.cfg.InitArchiveMaxUnpackedBytes})
	if err != nil {
		return apperr.ErrSandboxInitArchiveInvalid.WithCause(err)
	}
	command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.UnpackTar, runtime.AdapterSpec.WorkspaceDir, runtime.AdapterSpec.WorkspaceDir, "")
	if _, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, runtimeExecTarget(runtime), command, tarball, false); err != nil {
		return apperr.ErrSandboxInitExecFailed.WithCause(fmt.Errorf("%w: %s", err, string(stderr)))
	}
	return nil
}

// runInitScriptIfNeeded 在沙箱内部执行已授权初始化脚本引用,用于重放部署动作。
func (s *Service) runInitScriptIfNeeded(ctx context.Context, sb Sandbox, runtime Runtime, initScriptRef string) error {
	ref, err := storage.ParseObjectRef(initScriptRef)
	if err != nil {
		return apperr.ErrSandboxInitObjectRefInvalid.WithCause(err)
	}
	if err := validateInitObjectRef(sb.TenantID, ref, s.minio.BucketCode(), s.minio.BucketAttach()); err != nil {
		return err
	}
	reader, err := s.minio.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return apperr.ErrSandboxInitObjectReadFailed.WithCause(err)
	}
	defer logging.CloseContext(ctx, "关闭沙箱初始化脚本读取器失败", reader)
	data, err := io.ReadAll(io.LimitReader(reader, s.cfg.InitArchiveMaxBytes+1))
	if err != nil {
		return apperr.ErrSandboxInitObjectReadFailed.WithCause(err)
	}
	if int64(len(data)) > s.cfg.InitArchiveMaxBytes {
		return apperr.ErrSandboxInitArchiveTooLarge
	}
	scriptPath := path.Join(runtime.AdapterSpec.WorkspaceDir, ".chaimir-init-script")
	writeCommand := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.WriteFile, runtime.AdapterSpec.WorkspaceDir, scriptPath, "")
	if _, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, runtimeExecTarget(runtime), writeCommand, data, false); err != nil {
		return apperr.ErrSandboxInitExecFailed.WithCause(fmt.Errorf("%w: %s", err, string(stderr)))
	}
	runCommand := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.RunScript, runtime.AdapterSpec.WorkspaceDir, runtime.AdapterSpec.WorkspaceDir, scriptPath)
	stdin := []byte(base64.StdEncoding.EncodeToString(data))
	if _, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, runtimeExecTarget(runtime), runCommand, stdin, false); err != nil {
		return apperr.ErrSandboxInitExecFailed.WithCause(fmt.Errorf("%w: %s", err, string(stderr)))
	}
	return s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		detail, err := jsonBytes(map[string]any{"script_ref": initScriptRef})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeExec, detail)
	})
}

// validateInitObjectRef 校验初始化对象只来自当前租户的代码或附件桶,防止内部调用误传跨租户对象。
func validateInitObjectRef(tenantID int64, ref storage.ObjectRef, codeBucket, attachBucket string) error {
	if tenantID <= 0 {
		return apperr.ErrSandboxInitObjectRefInvalid
	}
	if ref.Bucket != codeBucket && ref.Bucket != attachBucket {
		return apperr.ErrSandboxInitObjectRefInvalid
	}
	prefix := strconv.FormatInt(tenantID, 10) + "/"
	if !strings.HasPrefix(ref.Key, prefix) {
		return apperr.ErrSandboxInitObjectRefInvalid
	}
	return nil
}
