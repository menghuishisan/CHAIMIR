package sandbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"path"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/upload"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
	"chaimir/pkg/logging"
)

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
	return s.sandboxWorkspaceArchiveRef(sb)
}

// FindSavedSandboxWorkspaceArchiveBySourceRef 在每次显式重建时重取持久化指针，不依赖前一请求内存。
func (s *Service) FindSavedSandboxWorkspaceArchiveBySourceRef(ctx context.Context, tenantID int64, sourceRef string) (string, error) {
	sourceRef = strings.TrimSpace(sourceRef)
	if tenantID <= 0 || !auth.ValidSourceRef(sourceRef) {
		return "", apperr.ErrSandboxContractRequestInvalid
	}
	var sb Sandbox
	err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSavedSandboxBySourceRef(ctx, tenantID, sourceRef)
		return err
	})
	if errors.Is(err, apperr.ErrSandboxNotFound) {
		return "", nil
	}
	if err != nil {
		return "", apperr.ErrSandboxFilePersistFailed.WithCause(err)
	}
	return s.sandboxWorkspaceArchiveRef(sb)
}

// sandboxWorkspaceArchiveRef 统一构造已保存公开归档引用，不返回初始化私有材料。
func (s *Service) sandboxWorkspaceArchiveRef(sb Sandbox) (string, error) {
	if strings.TrimSpace(sb.CodeHash) == "" || strings.TrimSpace(sb.CodeStorageKey) == "" {
		return "", nil
	}
	ref, err := storage.ObjectRefString(s.objectStorage.BucketCode(), sb.CodeStorageKey)
	if err != nil {
		return "", apperr.ErrSandboxFilePersistFailed.WithCause(err)
	}
	return ref, nil
}

// PutSandboxFile 把公开文件写入沙箱工作区。
func (s *Service) PutSandboxFile(ctx context.Context, req contracts.SandboxFileWriteRequest) (int64, error) {
	if req.TenantID <= 0 || req.SandboxID <= 0 || req.ExpectedRevision <= 0 || !validSourceRef(req.SourceRef) {
		return 0, apperr.ErrSandboxFileWriteRequestInvalid
	}
	relative, err := validateWorkspacePath(req.RelativePath)
	if err != nil {
		return 0, err
	}
	content, err := base64.StdEncoding.DecodeString(req.ContentBase64)
	if err != nil {
		return 0, apperr.ErrSandboxFileWriteRequestInvalid.WithCause(err)
	}
	sb, runtime, err := s.sandboxRuntimeForSource(ctx, req.TenantID, req.SandboxID, req.SourceRef)
	if err != nil {
		return 0, err
	}
	if !sandboxExecAllowed(sb) {
		return 0, apperr.ErrSandboxStateInvalid
	}
	target := path.Join(runtime.AdapterSpec.WorkspaceDir, relative)
	command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.WriteFile, runtime.AdapterSpec.WorkspaceDir, target)
	var revision int64
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		revision, err = tx.ClaimSandboxWorkspaceRevision(ctx, req.TenantID, req.SandboxID, req.ExpectedRevision)
		if err != nil {
			return err
		}
		if _, err := tx.MarkSandboxActive(ctx, req.TenantID, req.SandboxID); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		detail, err := jsonBytes(map[string]any{"path": relative, "mode": "write"})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), req.TenantID, req.SandboxID, EventTypeFileSave, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if _, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, workspaceExecTarget(runtime), command, content, false); err != nil {
			return sandboxExecFailure(apperr.ErrSandboxFileInvalid, err, stderr)
		}
		return nil
	}); err != nil {
		return 0, err
	}
	s.scheduleDebouncedSave(ctx, req.TenantID, req.SandboxID)
	return revision, nil
}

// PutSandboxPrivateArchive 把隐藏判题归档写入私有执行域。
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
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return err
	}
	domain, ok := volumeDomainByName(runtime.AdapterSpec, req.Domain)
	if !ok || domain.StudentAccess != VolumeAccessNone || domain.SnapshotScope != VolumeSnapshotNever {
		return apperr.ErrSandboxPrivateDomainInvalid
	}
	command := workspaceCommand(runtime.AdapterSpec.PrivateArchiveOps.UnpackTar, domain.MountPath, domain.MountPath)
	if _, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, strings.TrimSpace(runtime.AdapterSpec.PrivateArchiveOps.ExecTarget), command, tarball, false); err != nil {
		return sandboxExecFailure(apperr.ErrSandboxPrivateArchiveInvalid, err, stderr)
	}
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		detail, err := jsonBytes(map[string]any{"domain": req.Domain, "archive": req.ArchiveName})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), req.TenantID, req.SandboxID, EventTypeFileSave, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// PutSandboxFileForOwner 校验操作者归属后写入工作区文件。
func (s *Service) PutSandboxFileForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, req FileWriteRequest) (int64, error) {
	if req.ContentBase64 == nil {
		return 0, apperr.ErrSandboxFileWriteRequestInvalid
	}
	sb, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID)
	if err != nil {
		return 0, err
	}
	return s.PutSandboxFile(ctx, contracts.SandboxFileWriteRequest{
		TenantID:         tenantID,
		SandboxID:        sandboxID,
		SourceRef:        sb.SourceRef,
		RelativePath:     req.RelativePath,
		ContentBase64:    *req.ContentBase64,
		ExpectedRevision: req.ExpectedRevision,
	})
}

// ReadSandboxFile 从工作区读取普通文件,同时防止符号链接逃逸到 workspace 外。
func (s *Service) ReadSandboxFile(ctx context.Context, tenantID, sandboxID int64, relativePath string) (FileReadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ExecTimeoutSeconds))
	defer cancel()
	relative, err := validateWorkspacePath(relativePath)
	if err != nil {
		return FileReadResponse{}, err
	}
	sb, runtime, err := s.sandboxRuntime(ctx, tenantID, sandboxID)
	if err != nil {
		return FileReadResponse{}, err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return FileReadResponse{}, err
	}
	target := path.Join(runtime.AdapterSpec.WorkspaceDir, relative)
	command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.ReadFile, runtime.AdapterSpec.WorkspaceDir, target)
	var out FileReadResponse
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		// 与公开写入和归档保存共用行锁,避免文件字节与先前读到的版本错配。
		locked, err := tx.GetSandboxForUpdate(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxFileReadFailed.WithCause(err)
		}
		if locked.Namespace != sb.Namespace || locked.CompositionDigest != sb.CompositionDigest || !sandboxExecAllowed(locked) {
			return apperr.ErrConflict
		}
		stdout, stderr, err := s.orchestrator.Exec(ctx, locked.Namespace, workspaceExecTarget(runtime), command, nil, false)
		if err != nil {
			return sandboxExecFailure(apperr.ErrSandboxFileReadFailed, err, stderr)
		}
		out = FileReadResponse{
			RelativePath:      relative,
			ContentBase64:     base64.StdEncoding.EncodeToString(stdout),
			ContentSHA256:     crypto.SHA256Hex(stdout),
			ContentSize:       int64(len(stdout)),
			WorkspaceRevision: locked.WorkspaceRevision,
		}
		return nil
	}); err != nil {
		return FileReadResponse{}, err
	}
	return out, nil
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
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return FileListResponse{}, err
	}
	target := path.Join(runtime.AdapterSpec.WorkspaceDir, relative)
	command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.ListFiles, runtime.AdapterSpec.WorkspaceDir, target)
	stdout, stderr, err := s.orchestrator.Exec(ctx, sb.Namespace, workspaceExecTarget(runtime), command, nil, false)
	if err != nil {
		return FileListResponse{}, sandboxExecFailure(apperr.ErrSandboxFileListFailed, err, stderr)
	}
	var entries []FileEntryResponse
	if err := jsonx.DecodeStrictKnownFields(stdout, &entries); err != nil {
		return FileListResponse{}, apperr.ErrSandboxFileListDecodeFailed.WithCause(err)
	}
	visible := make([]FileEntryResponse, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Name) == "" || entry.Size < 0 {
			return FileListResponse{}, apperr.ErrSandboxFileEntryInvalid
		}
		if workload.IsPrivateWorkspacePath(entry.RelativePath) {
			continue
		}
		if _, err := validateWorkspacePath(entry.RelativePath); err != nil {
			return FileListResponse{}, apperr.ErrSandboxFileEntryInvalid.WithCause(err)
		}
		visible = append(visible, entry)
	}
	return FileListResponse{RelativePath: relative, Entries: visible}, nil
}

// ListSandboxFilesForOwner 校验操作者归属后列出工作区目录。
func (s *Service) ListSandboxFilesForOwner(ctx context.Context, tenantID, accountID, sandboxID int64, relativePath string) (FileListResponse, error) {
	if _, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID); err != nil {
		return FileListResponse{}, err
	}
	return s.ListSandboxFiles(ctx, tenantID, sandboxID, relativePath)
}

// SaveSandboxFiles 立即保存工作区并返回代码引用与哈希。
func (s *Service) SaveSandboxFiles(ctx context.Context, req contracts.SandboxSaveRequest) (string, string, error) {
	if req.TenantID <= 0 || req.SandboxID <= 0 || !validSourceRef(req.SourceRef) {
		return "", "", apperr.ErrSandboxContractRequestInvalid
	}
	return s.saveSandboxFilesForSource(ctx, req.TenantID, req.SandboxID, req.SourceRef)
}

func (s *Service) saveSandboxFilesForSource(ctx context.Context, tenantID, sandboxID int64, sourceRef string) (string, string, error) {
	sb, _, err := s.sandboxRuntimeForSource(ctx, tenantID, sandboxID, sourceRef)
	if err != nil {
		return "", "", err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return "", "", err
	}
	saved, err := s.saveSandboxFiles(ctx, sb)
	return saved.CodeStorageKey, saved.CodeHash, err
}

func (s *Service) saveSandboxFiles(ctx context.Context, sb Sandbox) (contracts.SandboxWorkspaceSave, error) {
	s.cancelDebouncedSave(sb.ID)
	return s.persistSandboxFiles(ctx, sb)
}

func (s *Service) persistSandboxFiles(ctx context.Context, sb Sandbox) (contracts.SandboxWorkspaceSave, error) {
	ctx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ExecTimeoutSeconds))
	defer cancel()
	tenantID, sandboxID := sb.TenantID, sb.ID
	var runtime Runtime
	var err error
	if sb.resourceLeaseToken == "" {
		runtime, _, err = s.workspaceRuntimeForSandbox(ctx, sb)
	} else {
		runtime, _, err = workspaceRuntimeSnapshot(sb)
	}
	if err != nil {
		return contracts.SandboxWorkspaceSave{}, err
	}
	var saved contracts.SandboxWorkspaceSave
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		// 第一阶段:锁定当前沙箱,与其它保存及公开文件revision更新跨副本串行。
		locked, err := tx.GetSandboxForUpdate(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxFilePersistFailed.WithCause(err)
		}
		if locked.Namespace != sb.Namespace || locked.CompositionDigest != sb.CompositionDigest {
			return apperr.ErrConflict
		}
		if !sandboxWorkspaceSaveAllowed(locked, sb.resourceLeaseToken) {
			return apperr.ErrSandboxStateInvalid
		}
		command := workspaceCommand(runtime.AdapterSpec.WorkspaceOps.PackTar, runtime.AdapterSpec.WorkspaceDir, runtime.AdapterSpec.WorkspaceDir)
		target := workspaceExecTarget(runtime)
		if sb.resourceLeaseToken != "" {
			target = workspaceArchiveExecTarget(sb)
		}
		stdout, stderr, err := s.orchestrator.Exec(ctx, locked.Namespace, target, command, nil, false)
		if err != nil {
			return sandboxExecFailure(apperr.ErrSandboxFilePersistFailed, err, stderr)
		}
		stdout, err = workload.SanitizeWorkspaceArchive(stdout)
		if err != nil {
			return apperr.ErrSandboxFilePersistFailed.WithCause(err)
		}
		// 第二阶段:内容哈希决定唯一归档路径,不覆盖先前提交引用的不同内容。
		hash := crypto.SHA256Hex(stdout)
		key, err := storage.ObjectKey(tenantID, "sandbox", "code", ids.Format(sandboxID), hash+".tar")
		if err != nil {
			return apperr.ErrSandboxFilePersistFailed.WithCause(err)
		}
		ref, err := storage.ObjectRefString(s.objectStorage.BucketCode(), key)
		if err != nil {
			return apperr.ErrSandboxFilePersistFailed.WithCause(err)
		}
		if err := s.objectStorage.Put(ctx, s.objectStorage.BucketCode(), key, bytes.NewReader(stdout), int64(len(stdout)), "application/x-tar"); err != nil {
			return apperr.ErrSandboxFilePersistFailed.WithCause(err)
		}
		// 第三阶段:上传成功后同事务写当前指针和事件,返回该次持锁版本。
		if _, err := tx.UpdateSandboxCode(ctx, tenantID, sandboxID, key, hash); err != nil {
			return apperr.ErrSandboxFilePersistFailed.WithCause(err)
		}
		detail, err := jsonBytes(map[string]any{"code_hash": hash})
		if err != nil {
			return apperr.ErrSandboxFilePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), tenantID, sandboxID, EventTypeFileSave, detail); err != nil {
			return apperr.ErrSandboxFilePersistFailed.WithCause(err)
		}
		saved = contracts.SandboxWorkspaceSave{CodeStorageKey: ref, CodeHash: hash, WorkspaceRevision: locked.WorkspaceRevision}
		return nil
	}); err != nil {
		return contracts.SandboxWorkspaceSave{}, err
	}
	return saved, nil
}

// SaveSandboxFilesForOwner 校验操作者归属后立即持久化工作区。
func (s *Service) SaveSandboxFilesForOwner(ctx context.Context, tenantID, accountID, sandboxID int64) (FileSaveResponse, error) {
	sb, _, err := s.sandboxRuntimeForOwner(ctx, tenantID, accountID, sandboxID)
	if err != nil {
		return FileSaveResponse{}, err
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return FileSaveResponse{}, err
	}
	saved, err := s.saveSandboxFiles(ctx, sb)
	if err != nil {
		return FileSaveResponse{}, err
	}
	return FileSaveResponse{CodeStorageKey: saved.CodeStorageKey, CodeHash: saved.CodeHash, WorkspaceRevision: saved.WorkspaceRevision}, nil
}

// ExecSandboxCommand 在沙箱内执行受限内部命令。
func (s *Service) ExecSandboxCommand(ctx context.Context, req contracts.SandboxExecRequest) (contracts.SandboxExecResult, error) {
	if req.TenantID <= 0 || req.SandboxID <= 0 || !workload.ValidNonShellCommand(req.Command) || !validSourceRef(req.SourceRef) {
		return contracts.SandboxExecResult{}, apperr.ErrSandboxContractRequestInvalid
	}
	sb, runtime, err := s.sandboxRuntimeForSource(ctx, req.TenantID, req.SandboxID, req.SourceRef)
	if err != nil {
		return contracts.SandboxExecResult{}, err
	}
	timeoutSec := req.TimeoutSec
	if timeoutSec <= 0 {
		var ok bool
		timeoutSec, ok = intx.Int32(s.cfg.ExecTimeoutSeconds)
		if !ok {
			return contracts.SandboxExecResult{}, apperr.ErrSandboxContractRequestInvalid
		}
	}
	if timeoutSec <= 0 {
		return contracts.SandboxExecResult{}, apperr.ErrSandboxContractRequestInvalid
	}
	execCtx, cancel := context.WithTimeout(ctx, timeDurationSeconds(int(timeoutSec)))
	defer cancel()
	target := runtimeExecTarget(runtime)
	if strings.TrimSpace(req.Container) != "" {
		target = strings.TrimSpace(req.Container)
		if !safeExecTarget(target) {
			return contracts.SandboxExecResult{}, apperr.ErrSandboxContractRequestInvalid
		}
	}
	if err := s.markSandboxExecutionActive(ctx, sb); err != nil {
		return contracts.SandboxExecResult{}, err
	}
	stdout, stderr, err := s.orchestrator.Exec(execCtx, sb.Namespace, target, req.Command, req.Stdin, false)
	if err != nil {
		return contracts.SandboxExecResult{}, sandboxExecFailure(apperr.ErrSandboxExecFailed, err, stderr)
	}
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		detail, err := jsonBytes(map[string]any{"command": req.Command[0]})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), req.TenantID, req.SandboxID, EventTypeExec, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return contracts.SandboxExecResult{}, apperr.ErrSandboxStatePersistFailed.WithCause(err)
	}
	return contracts.SandboxExecResult{Stdout: stdout, Stderr: stderr}, nil
}

func safeExecTarget(target string) bool {
	podName, containerName := splitExecTarget(target)
	return mountNamePattern.MatchString(podName) && mountNamePattern.MatchString(containerName)
}

// ObserveToolAccess 安排工作区防抖保存,覆盖经平台代理的 IDE/浏览器写入路径。
func (s *Service) ObserveToolAccess(ctx context.Context, sb Sandbox) {
	if sb.TenantID <= 0 || sb.ID <= 0 {
		return
	}
	s.scheduleDebouncedSave(ctx, sb.TenantID, sb.ID)
}

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
	runtime, _, err := s.workspaceRuntimeForSandbox(ctx, sb)
	if err != nil {
		return Sandbox{}, Runtime{}, apperr.ErrSandboxRuntimeNotFound.WithCause(err)
	}
	return sb, runtime, nil
}

func (s *Service) sandboxForOwner(ctx context.Context, tenantID, accountID, sandboxID int64) (Sandbox, error) {
	var sb Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if !sandboxAccountAuthorized(sb, accountID) {
			return apperr.ErrSandboxOwnershipInvalid
		}
		return nil
	}); err != nil {
		return Sandbox{}, err
	}
	if err := s.requireSandboxModule(ctx, sb); err != nil {
		return Sandbox{}, err
	}
	return sb, nil
}

func (s *Service) requireSandboxModule(ctx context.Context, sb Sandbox) error {
	module := contracts.TenantModuleFromSourceRef(sb.SourceRef)
	if module == "" {
		return apperr.ErrSandboxOwnershipInvalid
	}
	return s.identity.RequireTenantModules(ctx, sb.TenantID, module)
}

func (s *Service) sandboxRuntimeForOwner(ctx context.Context, tenantID, accountID, sandboxID int64) (Sandbox, Runtime, error) {
	sb, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID)
	if err != nil {
		return Sandbox{}, Runtime{}, err
	}
	runtime, _, err := s.workspaceRuntimeForSandbox(ctx, sb)
	if err != nil {
		return Sandbox{}, Runtime{}, apperr.ErrSandboxRuntimeNotFound.WithCause(err)
	}
	return sb, runtime, nil
}

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

func timeDurationSeconds(sec int) time.Duration {
	return time.Duration(sec) * time.Second
}

func sandboxExecAllowed(sb Sandbox) bool {
	phaseReady := sb.Phase == SandboxPhaseReady || sb.Phase == SandboxPhaseFullyReady
	statusReady := sb.Status == SandboxStatusReady || sb.Status == SandboxStatusRunning || sb.Status == SandboxStatusIdle
	return phaseReady && statusReady && sb.resourceLeaseToken == ""
}

func sandboxWorkspaceSaveAllowed(sb Sandbox, leaseToken string) bool {
	if leaseToken == "" {
		return sandboxExecAllowed(sb)
	}
	phaseReady := sb.Phase == SandboxPhaseReady || sb.Phase == SandboxPhaseFullyReady
	statusReady := sb.Status == SandboxStatusReady || sb.Status == SandboxStatusRunning || sb.Status == SandboxStatusIdle || sb.Status == SandboxStatusRecycling
	return phaseReady && statusReady && sb.resourceLeaseToken == leaseToken && sb.resourceLeaseUntil.After(timex.Now())
}

func volumeDomainByName(spec AdapterSpec, name string) (VolumeDomainSpec, bool) {
	for _, domain := range spec.VolumeDomains {
		if domain.Name == strings.TrimSpace(name) {
			return domain, true
		}
	}
	return VolumeDomainSpec{}, false
}

func workspaceCommand(template []string, workspaceDir, targetPath string) []string {
	out := make([]string, 0, len(template))
	for _, part := range template {
		part = strings.ReplaceAll(part, WorkspacePlaceholderRoot, workspaceDir)
		part = strings.ReplaceAll(part, WorkspacePlaceholderPath, targetPath)
		part = strings.ReplaceAll(part, WorkspacePlaceholderDomain, targetPath)
		out = append(out, part)
	}
	return out
}

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
	saveBase := logging.WithAttrs(context.WithoutCancel(ctx), traceAttrs...)
	var timer *time.Timer
	timer = time.AfterFunc(delay, func() {
		s.saveMu.Lock()
		if s.saveTimers[sandboxID] != timer {
			s.saveMu.Unlock()
			return
		}
		delete(s.saveTimers, sandboxID)
		s.saveMu.Unlock()
		saveCtx, cancel := context.WithTimeout(saveBase, timeDurationSeconds(s.cfg.ExecTimeoutSeconds))
		defer cancel()
		sb, _, err := s.sandboxRuntime(saveCtx, tenantID, sandboxID)
		if err == nil {
			// 防抖任务不是资源执行者，不能借用暂停或回收租约。
			sb.resourceLeaseToken = ""
			_, err = s.persistSandboxFiles(saveCtx, sb)
		}
		if err != nil {
			s.recordDebouncedSaveFailure(saveCtx, tenantID, sandboxID, err)
		}
	})
	s.saveTimers[sandboxID] = timer
	s.saveMu.Unlock()
}

func (s *Service) recordDebouncedSaveFailure(ctx context.Context, tenantID, sandboxID int64, cause error) {
	logging.ErrorContext(ctx, "sandbox debounced file save failed", apperr.AsAppError(cause).LogString(), slog.Int64("tenant_id", tenantID), slog.Int64("sandbox_id", sandboxID))
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		detail, err := jsonBytes(map[string]any{"stage": "debounced_file_save", "error": logging.SanitizeError(cause.Error())})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), tenantID, sandboxID, EventTypeError, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		logging.ErrorContext(ctx, "sandbox debounced file save event failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", tenantID), slog.Int64("sandbox_id", sandboxID))
	}
}

func (s *Service) cancelDebouncedSave(sandboxID int64) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	if timer := s.saveTimers[sandboxID]; timer != nil {
		timer.Stop()
		delete(s.saveTimers, sandboxID)
	}
}
