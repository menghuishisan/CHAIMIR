// sandbox service_files 文件实现沙箱工作区文件写入和统一对象存储持久化流程。
package sandbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
	"chaimir/pkg/logging"
)

// PutSandboxFile 把提交代码或公开脚本写入沙箱工作区,隐藏判题资产必须走私有域接口。
func (s *Service) PutSandboxFile(ctx context.Context, req contracts.SandboxFileWriteRequest) error {
	if req.TenantID <= 0 || req.SandboxID <= 0 || req.ContentBase64 == "" || !validSourceRef(req.SourceRef) {
		return apperr.ErrSandboxFileWriteRequestInvalid
	}
	relative, err := validateWorkspacePath(req.RelativePath)
	if err != nil {
		return err
	}
	content, err := base64.StdEncoding.DecodeString(req.ContentBase64)
	if err != nil {
		return apperr.ErrSandboxFileWriteRequestInvalid.WithCause(err)
	}
	sb, runtime, err := s.sandboxRuntimeForSource(ctx, req.TenantID, req.SandboxID, req.SourceRef)
	if err != nil {
		return err
	}
	target := path.Join(runtime.AdapterSpec.WorkspaceDir, relative)
	command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.WriteFile, runtime.AdapterSpec.WorkspaceDir, target, "")
	if _, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, runtimeExecTarget(runtime), command, content, false); err != nil {
		return apperr.ErrSandboxFileInvalid.WithCause(fmt.Errorf("%w: %s", err, string(stderr)))
	}
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.MarkSandboxActive(ctx, req.TenantID, req.SandboxID); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		detail, err := jsonBytes(map[string]any{"path": relative, "mode": "write"})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), req.TenantID, req.SandboxID, EventTypeFileSave, detail)
	}); err != nil {
		return err
	}
	s.scheduleDebouncedSave(ctx, req.TenantID, req.SandboxID)
	return nil
}

// PutSandboxPrivateArchive 将隐藏测试、答案或评分脚本安全解包到私有判题域。
func (s *Service) PutSandboxPrivateArchive(ctx context.Context, req contracts.SandboxPrivateArchiveInjectRequest) error {
	if req.TenantID <= 0 || req.SandboxID <= 0 || req.ContentBase64 == "" || !validSourceRef(req.SourceRef) {
		return apperr.ErrSandboxPrivateArchiveInvalid
	}
	if strings.TrimSpace(req.Domain) != VolumeDomainJudgePrivate || strings.TrimSpace(req.ArchiveName) == "" {
		return apperr.ErrSandboxPrivateArchiveInvalid
	}
	content, err := base64.StdEncoding.DecodeString(req.ContentBase64)
	if err != nil {
		return apperr.ErrSandboxPrivateArchiveInvalid.WithCause(err)
	}
	if int64(len(content)) > s.cfg.InitArchiveMaxBytes {
		return apperr.ErrSandboxPrivateArchiveInvalid
	}
	tarball, err := upload.SafeArchiveTar(req.ArchiveName, content, upload.ArchiveLimits{MaxFiles: s.cfg.InitArchiveMaxFiles, MaxUnpackedBytes: s.cfg.InitArchiveMaxUnpackedBytes})
	if err != nil {
		return apperr.ErrSandboxPrivateArchiveInvalid.WithCause(err)
	}
	sb, runtime, err := s.sandboxRuntimeForSource(ctx, req.TenantID, req.SandboxID, req.SourceRef)
	if err != nil {
		return err
	}
	domain, ok := volumeDomainByName(runtime.AdapterSpec, req.Domain)
	if !ok || domain.StudentAccess != VolumeAccessNone || domain.SnapshotScope != VolumeSnapshotNever {
		return apperr.ErrSandboxPrivateDomainInvalid
	}
	command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.UnpackTar, domain.MountPath, domain.MountPath, "")
	if _, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, runtimeExecTarget(runtime), command, tarball, false); err != nil {
		return apperr.ErrSandboxPrivateArchiveInvalid.WithCause(fmt.Errorf("%w: %s", err, string(stderr)))
	}
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		detail, err := jsonBytes(map[string]any{"domain": req.Domain, "archive": req.ArchiveName})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), req.TenantID, req.SandboxID, EventTypeFileSave, detail)
	}); err != nil {
		return err
	}
	return nil
}

// PutSandboxFileForOwner 校验操作者归属后写入工作区文件。
func (s *Service) PutSandboxFileForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, req FileWriteRequest) error {
	sb, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID)
	if err != nil {
		return err
	}
	return s.PutSandboxFile(ctx, contracts.SandboxFileWriteRequest{
		TenantID:      tenantID,
		SandboxID:     sandboxID,
		SourceRef:     sb.SourceRef,
		RelativePath:  req.RelativePath,
		ContentBase64: req.ContentBase64,
	})
}

// ReadSandboxFile 从工作区读取普通文件,同时防止符号链接逃逸到 workspace 外。
func (s *Service) ReadSandboxFile(ctx context.Context, tenantID, sandboxID int64, relativePath string) (FileReadResponse, error) {
	relative, err := validateWorkspacePath(relativePath)
	if err != nil {
		return FileReadResponse{}, err
	}
	sb, runtime, err := s.sandboxRuntime(ctx, tenantID, sandboxID)
	if err != nil {
		return FileReadResponse{}, err
	}
	target := path.Join(runtime.AdapterSpec.WorkspaceDir, relative)
	command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.ReadFile, runtime.AdapterSpec.WorkspaceDir, target, "")
	stdout, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, runtimeExecTarget(runtime), command, nil, false)
	if err != nil {
		return FileReadResponse{}, apperr.ErrSandboxFileReadFailed.WithCause(fmt.Errorf("%w: %s", err, string(stderr)))
	}
	return FileReadResponse{
		RelativePath:  relative,
		ContentBase64: base64.StdEncoding.EncodeToString(stdout),
		ContentSHA256: crypto.SHA256Hex(stdout),
		ContentSize:   int64(len(stdout)),
	}, nil
}

// ReadSandboxFileForOwner 校验操作者归属后读取工作区文件。
func (s *Service) ReadSandboxFileForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, relativePath string) (FileReadResponse, error) {
	if _, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID); err != nil {
		return FileReadResponse{}, err
	}
	return s.ReadSandboxFile(ctx, tenantID, sandboxID, relativePath)
}

// ListSandboxFiles 从工作区列出目录条目,目录穿越和符号链接策略由运行时受控 helper 执行。
func (s *Service) ListSandboxFiles(ctx context.Context, tenantID, sandboxID int64, relativePath string) (FileListResponse, error) {
	relative, err := validateWorkspaceListPath(relativePath)
	if err != nil {
		return FileListResponse{}, err
	}
	sb, runtime, err := s.sandboxRuntime(ctx, tenantID, sandboxID)
	if err != nil {
		return FileListResponse{}, err
	}
	target := path.Join(runtime.AdapterSpec.WorkspaceDir, relative)
	command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.ListFiles, runtime.AdapterSpec.WorkspaceDir, target, "")
	stdout, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, runtimeExecTarget(runtime), command, nil, false)
	if err != nil {
		return FileListResponse{}, apperr.ErrSandboxFileListFailed.WithCause(fmt.Errorf("%w: %s", err, string(stderr)))
	}
	var entries []FileEntryResponse
	if err := jsonx.DecodeStrict(stdout, &entries); err != nil {
		return FileListResponse{}, apperr.ErrSandboxFileListDecodeFailed.WithCause(err)
	}
	for _, entry := range entries {
		if strings.TrimSpace(entry.Name) == "" || entry.Size < 0 {
			return FileListResponse{}, apperr.ErrSandboxFileEntryInvalid
		}
		if _, err := validateWorkspacePath(entry.RelativePath); err != nil {
			return FileListResponse{}, apperr.ErrSandboxFileEntryInvalid.WithCause(err)
		}
	}
	return FileListResponse{RelativePath: relative, Entries: entries}, nil
}

// ListSandboxFilesForOwner 校验操作者归属后列出工作区目录。
func (s *Service) ListSandboxFilesForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, relativePath string) (FileListResponse, error) {
	if _, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID); err != nil {
		return FileListResponse{}, err
	}
	return s.ListSandboxFiles(ctx, tenantID, sandboxID, relativePath)
}

// SaveSandboxFiles 校验来源归属后立即持久化当前工作区,返回保存后的代码引用与哈希。
func (s *Service) SaveSandboxFiles(ctx context.Context, req contracts.SandboxSaveRequest) (string, string, error) {
	if req.TenantID <= 0 || req.SandboxID <= 0 || !validSourceRef(req.SourceRef) {
		return "", "", apperr.ErrSandboxContractRequestInvalid
	}
	return s.saveSandboxFilesForSource(ctx, req.TenantID, req.SandboxID, req.SourceRef)
}

// saveSandboxFilesForSource 在跨模块调用场景校验 source_ref 与沙箱归属一致。
func (s *Service) saveSandboxFilesForSource(ctx context.Context, tenantID, sandboxID int64, sourceRef string) (string, string, error) {
	if _, _, err := s.sandboxRuntimeForSource(ctx, tenantID, sandboxID, sourceRef); err != nil {
		return "", "", err
	}
	return s.saveSandboxFiles(ctx, tenantID, sandboxID)
}

// saveSandboxFiles 执行工作区打包上传,供已完成归属校验的模块内部流程复用。
func (s *Service) saveSandboxFiles(ctx context.Context, tenantID, sandboxID int64) (string, string, error) {
	s.cancelDebouncedSave(sandboxID)
	sb, runtime, err := s.sandboxRuntime(ctx, tenantID, sandboxID)
	if err != nil {
		return "", "", err
	}
	command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.PackTar, runtime.AdapterSpec.WorkspaceDir, runtime.AdapterSpec.WorkspaceDir, "")
	stdout, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, runtimeExecTarget(runtime), command, nil, false)
	if err != nil {
		return "", "", apperr.ErrSandboxFilePersistFailed.WithCause(fmt.Errorf("%w: %s", err, string(stderr)))
	}
	hash := crypto.SHA256Hex(stdout)
	if err := s.minio.Put(ctx, s.minio.BucketCode(), sb.CodeStorageKey, bytes.NewReader(stdout), int64(len(stdout)), "application/x-tar"); err != nil {
		return "", "", apperr.ErrSandboxFilePersistFailed.WithCause(err)
	}
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.UpdateSandboxCode(ctx, tenantID, sandboxID, sb.CodeStorageKey, hash); err != nil {
			return apperr.ErrSandboxFilePersistFailed.WithCause(err)
		}
		detail, err := jsonBytes(map[string]any{"code_hash": hash})
		if err != nil {
			return apperr.ErrSandboxFilePersistFailed.WithCause(err)
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), tenantID, sandboxID, EventTypeFileSave, detail)
	}); err != nil {
		return "", "", err
	}
	ref, err := storage.ObjectRefString(s.minio.BucketCode(), sb.CodeStorageKey)
	if err != nil {
		return "", "", apperr.ErrSandboxFilePersistFailed.WithCause(err)
	}
	return ref, hash, nil
}

// SaveSandboxFilesForOwner 校验操作者归属后立即持久化工作区。
func (s *Service) SaveSandboxFilesForOwner(ctx context.Context, tenantID, accountID, sandboxID int64) (FileSaveResponse, error) {
	if _, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID); err != nil {
		return FileSaveResponse{}, err
	}
	key, hash, err := s.saveSandboxFiles(ctx, tenantID, sandboxID)
	if err != nil {
		return FileSaveResponse{}, err
	}
	return FileSaveResponse{CodeStorageKey: key, CodeHash: hash}, nil
}

// ExecSandboxCommand 在沙箱内执行受限命令,供判题 worker 运行套件。
func (s *Service) ExecSandboxCommand(ctx context.Context, req contracts.SandboxExecRequest) (contracts.SandboxExecResult, error) {
	if req.TenantID <= 0 || req.SandboxID <= 0 || len(req.Command) == 0 || !validSourceRef(req.SourceRef) {
		return contracts.SandboxExecResult{}, apperr.ErrSandboxContractRequestInvalid
	}
	sb, runtime, err := s.sandboxRuntimeForSource(ctx, req.TenantID, req.SandboxID, req.SourceRef)
	if err != nil {
		return contracts.SandboxExecResult{}, err
	}
	execCtx := ctx
	cancel := func() {}
	if req.TimeoutSec > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeDurationSeconds(req.TimeoutSec))
	}
	defer cancel()
	stdout, stderr, err := s.orchestrator.Exec(execCtx, sb.Namespace, runtimeExecTarget(runtime), req.Command, req.Stdin, false)
	if err != nil {
		return contracts.SandboxExecResult{}, apperr.ErrSandboxExecFailed.WithCause(err)
	}
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		detail, err := jsonBytes(map[string]any{"command": req.Command[0]})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), req.TenantID, req.SandboxID, EventTypeExec, detail)
	}); err != nil {
		return contracts.SandboxExecResult{}, apperr.ErrSandboxStatePersistFailed.WithCause(err)
	}
	return contracts.SandboxExecResult{Stdout: stdout, Stderr: stderr}, nil
}

// ObserveToolAccess 记录工具访问活跃度并安排工作区防抖保存,覆盖经平台代理的 IDE/浏览器写入路径。
func (s *Service) ObserveToolAccess(ctx context.Context, sb Sandbox, tool SandboxTool) {
	if sb.TenantID <= 0 || sb.ID <= 0 {
		return
	}
	if err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.MarkSandboxActive(ctx, sb.TenantID, sb.ID); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		logging.ErrorContext(ctx, "sandbox tool access activity mark failed", err.Error(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID), slog.String("tool_code", tool.ToolCode))
		return
	}
	s.scheduleDebouncedSave(ctx, sb.TenantID, sb.ID)
}

// sandboxRuntime 查询沙箱及其运行时定义。
func (s *Service) sandboxRuntime(ctx context.Context, tenantID, sandboxID int64) (Sandbox, Runtime, error) {
	var sb Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return Sandbox{}, Runtime{}, err
	}
	var runtime Runtime
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		runtime, err = tx.GetRuntimeByID(ctx, sb.RuntimeID)
		return err
	}); err != nil {
		return Sandbox{}, Runtime{}, apperr.ErrSandboxRuntimeNotFound.WithCause(err)
	}
	return sb, runtime, nil
}

// sandboxForOwner 只校验租户内沙箱归属,不读取 metrics,供终端、文件和工具等交互入口使用。
func (s *Service) sandboxForOwner(ctx context.Context, tenantID, accountID, sandboxID int64) (Sandbox, error) {
	var sb Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return Sandbox{}, err
	}
	if sb.OwnerAccountID != accountID {
		return Sandbox{}, apperr.ErrSandboxOwnershipInvalid
	}
	return sb, nil
}

// sandboxRuntimeForOwner 校验用户归属并加载运行时声明,避免交互路径被实时资源指标依赖阻断。
func (s *Service) sandboxRuntimeForOwner(ctx context.Context, tenantID, accountID, sandboxID int64) (Sandbox, Runtime, error) {
	sb, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID)
	if err != nil {
		return Sandbox{}, Runtime{}, err
	}
	var runtime Runtime
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		runtime, err = tx.GetRuntimeByID(ctx, sb.RuntimeID)
		return err
	}); err != nil {
		return Sandbox{}, Runtime{}, apperr.ErrSandboxRuntimeNotFound.WithCause(err)
	}
	return sb, runtime, nil
}

// sandboxRuntimeForSource 查询沙箱运行时并确认调用来源与创建来源一致。
func (s *Service) sandboxRuntimeForSource(ctx context.Context, tenantID, sandboxID int64, sourceRef string) (Sandbox, Runtime, error) {
	sb, runtime, err := s.sandboxRuntime(ctx, tenantID, sandboxID)
	if err != nil {
		return Sandbox{}, Runtime{}, err
	}
	if sb.SourceRef != strings.TrimSpace(sourceRef) {
		return Sandbox{}, Runtime{}, apperr.ErrSandboxOwnershipInvalid
	}
	return sb, runtime, nil
}

// timeDurationSeconds 把正整数秒转换为 duration。
func timeDurationSeconds(sec int32) time.Duration {
	return time.Duration(sec) * time.Second
}

// volumeDomainByName 查找运行时声明的卷安全域,用于内部私有资产注入等受控路径。
func volumeDomainByName(spec AdapterSpec, name string) (VolumeDomainSpec, bool) {
	for _, domain := range spec.VolumeDomains {
		if domain.Name == strings.TrimSpace(name) {
			return domain, true
		}
	}
	return VolumeDomainSpec{}, false
}

// workspaceCommand 根据运行时声明模板替换受控变量,不拼接 shell 片段。
func workspaceCommand(template []string, workspaceDir, targetPath, scriptPath string) []string {
	out := make([]string, 0, len(template))
	for _, part := range template {
		part = strings.ReplaceAll(part, WorkspacePlaceholderRoot, workspaceDir)
		part = strings.ReplaceAll(part, WorkspacePlaceholderPath, targetPath)
		part = strings.ReplaceAll(part, WorkspacePlaceholderScript, scriptPath)
		out = append(out, part)
	}
	return out
}

// scheduleDebouncedSave 为沙箱写入安排一次延迟持久化,重复写入只保留最后一次。
func (s *Service) scheduleDebouncedSave(ctx context.Context, tenantID, sandboxID int64) {
	delay := time.Duration(s.cfg.FileSaveDebounceMs) * time.Millisecond
	if delay <= 0 {
		return
	}
	traceAttrs := logging.AttrsFromContext(ctx)
	s.saveMu.Lock()
	if timer := s.saveTimers[sandboxID]; timer != nil {
		timer.Stop()
	}
	s.saveTimers[sandboxID] = time.AfterFunc(delay, func() {
		saveCtx := logging.WithAttrs(context.Background(), traceAttrs...)
		if _, _, err := s.saveSandboxFiles(saveCtx, tenantID, sandboxID); err != nil {
			s.recordDebouncedSaveFailure(saveCtx, tenantID, sandboxID, err)
		}
	})
	s.saveMu.Unlock()
}

// recordDebouncedSaveFailure 记录防抖保存失败的结构化日志和沙箱技术事件。
func (s *Service) recordDebouncedSaveFailure(ctx context.Context, tenantID, sandboxID int64, cause error) {
	logging.ErrorContext(ctx, "sandbox debounced file save failed", cause.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("sandbox_id", sandboxID))
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		detail, err := jsonBytes(map[string]any{"stage": "debounced_file_save", "error": logging.SanitizeError(cause.Error())})
		if err != nil {
			return err
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), tenantID, sandboxID, EventTypeError, detail)
	}); err != nil {
		logging.ErrorContext(ctx, "sandbox debounced file save event failed", err.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("sandbox_id", sandboxID))
	}
}

// cancelDebouncedSave 取消指定沙箱尚未执行的防抖保存任务。
func (s *Service) cancelDebouncedSave(sandboxID int64) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	if timer := s.saveTimers[sandboxID]; timer != nil {
		timer.Stop()
		delete(s.saveTimers, sandboxID)
	}
}
