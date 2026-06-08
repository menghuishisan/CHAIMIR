// M2 沙箱文件与对象存储辅助。
// 文件接口统一通过运行时主容器工作目录读写,并按文档要求持久化到 MinIO。
package sandbox

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/sandbox/internal/sqlcgen"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
)

// GetFile 读取目录或文件。
func (s *Service) GetFile(ctx context.Context, sandboxID int64, relPath string) (SandboxFilePayload, error) {
	row, binding, err := s.runtimeBindingForSandbox(ctx, sandboxID)
	if err != nil {
		return SandboxFilePayload{}, err
	}
	target, err := sandboxWorkspacePath(binding.WorkspaceDir, relPath)
	if err != nil {
		return SandboxFilePayload{}, err
	}
	command := workspaceReadCommand(binding.WorkspaceDir, target)
	var stdout bytes.Buffer
	if err := s.orchestrator.Exec(ctx, binding, command, nil, &stdout, nil, false); err != nil {
		return SandboxFilePayload{}, apperr.ErrSandboxFileNotFound.WithCause(err)
	}

	raw := strings.TrimSpace(stdout.String())
	if strings.Contains(raw, "\t") {
		payload := SandboxFilePayload{Path: relPath, IsDir: true}
		for _, line := range strings.Split(raw, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 4)
			if len(parts) != 4 {
				continue
			}
			// 目录列表来自沙箱内 find 输出;格式异常说明执行结果不可相信,必须显式失败而不是展示半截数据。
			size, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				return SandboxFilePayload{}, apperr.ErrSandboxFileInvalid.WithCause(err)
			}
			ts, err := time.Parse(time.RFC3339Nano, parts[3])
			if err != nil {
				return SandboxFilePayload{}, apperr.ErrSandboxFileInvalid.WithCause(err)
			}
			payload.Entries = append(payload.Entries, SandboxFileEntry{
				Name:      parts[0],
				Path:      path.Join(relPath, parts[0]),
				IsDir:     parts[1] == "d",
				Size:      size,
				UpdatedAt: ts,
			})
		}
		return payload, nil
	}
	return SandboxFilePayload{
		Path:     relPath,
		IsDir:    false,
		Content:  raw,
		Encoding: "base64",
	}, s.markSandboxActive(ctx, row.TenantID, row.ID)
}

// PutFile 写文件并触发保存。
func (s *Service) PutFile(ctx context.Context, sandboxID int64, relPath, contentBase64 string) error {
	row, binding, err := s.runtimeBindingForSandbox(ctx, sandboxID)
	if err != nil {
		return err
	}
	if _, err := base64.StdEncoding.DecodeString(contentBase64); err != nil {
		return apperr.ErrSandboxFileInvalid
	}
	target, err := sandboxWorkspacePath(binding.WorkspaceDir, relPath)
	if err != nil {
		return err
	}
	command := workspaceWriteCommand(binding.WorkspaceDir, target)
	if err := s.orchestrator.Exec(ctx, binding, command, bytes.NewReader([]byte(contentBase64)), nil, nil, false); err != nil {
		return apperr.ErrSandboxFileSaveFail.WithCause(err)
	}
	if err := s.markSandboxActive(ctx, row.TenantID, row.ID); err != nil {
		return err
	}
	return s.SaveFiles(ctx, sandboxID)
}

// PutSandboxFile 实现 contracts.SandboxService,供 M3 等内部模块向沙箱注入文件。
func (s *Service) PutSandboxFile(ctx context.Context, req contracts.SandboxFileWrite) error {
	return s.PutFile(ctx, req.SandboxID, req.RelativePath, req.ContentBase64)
}

// SaveFiles 把整个工作目录归档并持久化到 MinIO。
func (s *Service) SaveFiles(ctx context.Context, sandboxID int64) error {
	row, binding, err := s.runtimeBindingForSandbox(ctx, sandboxID)
	if err != nil {
		return err
	}
	return s.saveFilesForSandbox(ctx, row, binding)
}

// SaveSandboxFiles 实现 contracts.SandboxService,持久化后返回最新代码哈希。
func (s *Service) SaveSandboxFiles(ctx context.Context, sandboxID int64) (string, error) {
	if err := s.SaveFiles(ctx, sandboxID); err != nil {
		return "", err
	}
	row, _, err := s.runtimeBindingForSandbox(ctx, sandboxID)
	if err != nil {
		return "", err
	}
	return textValue(row.CodeHash), nil
}

// ExecSandboxCommand 实现 contracts.SandboxService,供 M3 在 judge 沙箱内执行受限判题命令。
func (s *Service) ExecSandboxCommand(ctx context.Context, req contracts.SandboxExecRequest) (contracts.SandboxExecResult, error) {
	// 第一步校验跨模块输入,命令和超时必须由判题器配置明确给出。
	if req.SandboxID <= 0 || len(req.Command) == 0 || req.TimeoutSec <= 0 {
		return contracts.SandboxExecResult{}, apperr.ErrSandboxCommandFail
	}
	// 第二步解析租户可访问的沙箱运行时绑定,复用 M2 原有权限检查。
	_, binding, err := s.runtimeBindingForSandbox(ctx, req.SandboxID)
	if err != nil {
		return contracts.SandboxExecResult{}, err
	}
	// 第三步按判题器超时限制执行命令,避免学生代码或测试套件长期占用资源。
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(req.TimeoutSec)*time.Second)
	defer cancel()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := s.orchestrator.Exec(execCtx, binding, req.Command, bytes.NewReader(req.Stdin), &stdout, &stderr, false); err != nil {
		if execCtx.Err() != nil {
			return contracts.SandboxExecResult{}, apperr.ErrSandboxTimeout.WithCause(execCtx.Err())
		}
		return contracts.SandboxExecResult{}, apperr.ErrSandboxCommandFail.WithCause(err)
	}
	return contracts.SandboxExecResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, nil
}

// saveFilesForSandbox 把指定沙箱工作目录归档到 MinIO,供用户保存和回收流程复用。
func (s *Service) saveFilesForSandbox(ctx context.Context, row sqlcgen.Sandbox, binding SandboxRuntimeBinding) error {
	if s.store == nil {
		return apperr.ErrSandboxFileSaveFail
	}
	command := []string{"sh", "-lc", "cd " + shellQuote(binding.WorkspaceDir) + " && tar -czf - ."}
	var stdout bytes.Buffer
	if err := s.orchestrator.Exec(ctx, binding, command, nil, &stdout, nil, false); err != nil {
		return apperr.ErrSandboxFileSaveFail.WithCause(err)
	}
	if err := s.store.Put(ctx, s.store.BucketCode(), row.CodeStorageKey+".tgz", bytes.NewReader(stdout.Bytes()), int64(stdout.Len()), "application/gzip"); err != nil {
		return apperr.ErrSandboxFileSaveFail.WithCause(err)
	}
	sum := sha256.Sum256(stdout.Bytes())
	codeHash := hex.EncodeToString(sum[:])
	if err := s.repo.inTenantID(ctx, row.TenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.UpdateSandboxCodeHash(ctx, sqlcgen.UpdateSandboxCodeHashParams{ID: row.ID, CodeHash: pgText(codeHash)}); err != nil {
			return err
		}
		return s.writeEvent(ctx, q, row.TenantID, row.ID, SandboxEventSaveFiles, map[string]any{
			"storage_key": row.CodeStorageKey + ".tgz",
			"code_hash":   codeHash,
		})
	}); err != nil {
		return apperr.ErrSandboxFileSaveFail.WithCause(err)
	}
	return nil
}

// workspaceReadCommand 构造带 realpath 边界校验的读取命令,阻断 symlink 指向工作区外。
func workspaceReadCommand(workspaceDir, target string) []string {
	ws := shellQuote(workspaceDir)
	dst := shellQuote(target)
	script := "ws=$(realpath -m " + ws + ") && target=" + dst + " && resolved=$(realpath -m \"$target\") && " +
		"case \"$resolved\" in \"$ws\"|\"$ws\"/*) ;; *) exit 23 ;; esac && " +
		"if [ -d \"$resolved\" ]; then find \"$resolved\" -maxdepth 1 -mindepth 1 -printf '%f\\t%y\\t%s\\t%TY-%Tm-%TdT%TH:%TM:%TS\\n'; else base64 -w0 \"$resolved\"; fi"
	return []string{"sh", "-lc", script}
}

// workspaceWriteCommand 构造带父目录与目标 symlink 校验的写入命令,防止覆盖工作区外文件。
func workspaceWriteCommand(workspaceDir, target string) []string {
	ws := shellQuote(workspaceDir)
	dst := shellQuote(target)
	script := "ws=$(realpath -m " + ws + ") && target=" + dst + " && parent=$(dirname \"$target\") && mkdir -p \"$parent\" && " +
		"resolved_parent=$(realpath -m \"$parent\") && case \"$resolved_parent\" in \"$ws\"|\"$ws\"/*) ;; *) exit 23 ;; esac && " +
		"if [ -L \"$target\" ]; then resolved_target=$(realpath -m \"$target\") && case \"$resolved_target\" in \"$ws\"|\"$ws\"/*) ;; *) exit 23 ;; esac; fi && " +
		"base64 -d > \"$target\""
	return []string{"sh", "-lc", script}
}

// safeSandboxInitArchive 校验并重打初始代码归档,避免恶意 tar 成员在容器内解包时逃逸工作区。
func safeSandboxInitArchive(raw []byte, cfg config.SandboxConfig) (outBytes []byte, err error) {
	if cfg.InitArchiveMaxFiles <= 0 || cfg.InitArchiveMaxUnpackedBytes <= 0 {
		return nil, apperr.ErrSandboxFileInvalid
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, apperr.ErrSandboxFileInvalid.WithCause(err)
	}
	defer func() {
		if closeErr := gzipReader.Close(); closeErr != nil {
			err = errors.Join(err, apperr.ErrSandboxFileInvalid.WithCause(closeErr))
		}
	}()

	tarReader := tar.NewReader(gzipReader)
	var out bytes.Buffer
	gzipWriter := gzip.NewWriter(&out)
	tarWriter := tar.NewWriter(gzipWriter)
	existing := map[string]struct{}{}
	var unpackedBytes int64
	var fileCount int
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, apperr.ErrSandboxFileInvalid.WithCause(err)
		}
		name, ok := upload.SafeArchiveEntryName(header.Name, existing)
		if !ok {
			return nil, apperr.ErrSandboxFileInvalid
		}
		existing[name] = struct{}{}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := tarWriter.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
				return nil, apperr.ErrSandboxFileInvalid.WithCause(err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 {
				return nil, apperr.ErrSandboxFileInvalid
			}
			fileCount++
			if fileCount > cfg.InitArchiveMaxFiles {
				return nil, apperr.ErrSandboxFileInvalid
			}
			if header.Size > cfg.InitArchiveMaxUnpackedBytes-unpackedBytes {
				return nil, apperr.ErrSandboxFileInvalid
			}
			unpackedBytes += header.Size
			if err := tarWriter.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: header.Size}); err != nil {
				return nil, apperr.ErrSandboxFileInvalid.WithCause(err)
			}
			if _, err := io.CopyN(tarWriter, tarReader, header.Size); err != nil {
				return nil, apperr.ErrSandboxFileInvalid.WithCause(err)
			}
		default:
			return nil, apperr.ErrSandboxFileInvalid
		}
	}
	if err := tarWriter.Close(); err != nil {
		return nil, apperr.ErrSandboxFileInvalid.WithCause(err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, apperr.ErrSandboxFileInvalid.WithCause(err)
	}
	return out.Bytes(), nil
}

// runtimeBindingForSandbox 校验租户权限并解析运行时绑定。
func (s *Service) runtimeBindingForSandbox(ctx context.Context, sandboxID int64) (sqlcgen.Sandbox, SandboxRuntimeBinding, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return sqlcgen.Sandbox{}, SandboxRuntimeBinding{}, apperr.ErrUnauthorized
	}
	var row sqlcgen.Sandbox
	var runtime sqlcgen.Runtime
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		current, err := q.GetSandboxByID(ctx, sandboxID)
		if err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrSandboxNotFound
			}
			return err
		}
		if err := authorizeSandboxRowAccess(ctx, id, current); err != nil {
			return err
		}
		if err := ensureSandboxInteractive(current.Status); err != nil {
			return err
		}
		row = current
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return sqlcgen.Sandbox{}, SandboxRuntimeBinding{}, ae
		}
		return sqlcgen.Sandbox{}, SandboxRuntimeBinding{}, apperr.ErrSandboxPersistenceFail.WithCause(err)
	}
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		runtime, err = q.GetRuntimeByID(ctx, row.RuntimeID)
		return err
	}); err != nil {
		return sqlcgen.Sandbox{}, SandboxRuntimeBinding{}, apperr.ErrRuntimeUnavailable.WithCause(err)
	}
	spec, err := parseRuntimeAdapterSpec(runtime.AdapterSpec)
	if err != nil {
		return sqlcgen.Sandbox{}, SandboxRuntimeBinding{}, err
	}
	binding, err := s.orchestrator.RuntimeBinding(ctx, row.Namespace)
	if err != nil {
		return sqlcgen.Sandbox{}, SandboxRuntimeBinding{}, apperr.ErrSandboxInvalidState.WithCause(err)
	}
	if binding.WorkspaceDir == "" {
		binding.WorkspaceDir = spec.WorkspaceDir
	}
	return row, binding, nil
}

// runtimeBindingForSandboxRow 使用已验证的沙箱行解析运行时绑定,供内部回收任务使用。
func (s *Service) runtimeBindingForSandboxRow(ctx context.Context, row sqlcgen.Sandbox) (SandboxRuntimeBinding, error) {
	var runtime sqlcgen.Runtime
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		runtime, err = q.GetRuntimeByID(ctx, row.RuntimeID)
		return err
	}); err != nil {
		return SandboxRuntimeBinding{}, apperr.ErrRuntimeUnavailable.WithCause(err)
	}
	spec, err := parseRuntimeAdapterSpec(runtime.AdapterSpec)
	if err != nil {
		return SandboxRuntimeBinding{}, err
	}
	binding, err := s.orchestrator.RuntimeBinding(ctx, row.Namespace)
	if err != nil {
		return SandboxRuntimeBinding{}, apperr.ErrSandboxInvalidState.WithCause(err)
	}
	if binding.WorkspaceDir == "" {
		binding.WorkspaceDir = spec.WorkspaceDir
	}
	return binding, nil
}

// markSandboxActive 更新最近活跃时间,供空闲回收使用。
func (s *Service) markSandboxActive(ctx context.Context, tenantID, sandboxID int64) error {
	return s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		return q.MarkSandboxActive(ctx, sandboxID)
	})
}

// writeSandboxExecEvent 记录文件/终端/初始化等执行事件。
func (s *Service) writeSandboxExecEvent(ctx context.Context, tenantID, sandboxID int64, action string, detail map[string]any) error {
	return s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		return s.writeEvent(ctx, q, tenantID, sandboxID, "exec", map[string]any{
			"action": action,
			"detail": detail,
		})
	})
}

// sandboxWorkspacePath 把对外相对路径收敛到工作目录下,防止目录穿越。
func sandboxWorkspacePath(workspaceDir, rel string) (string, error) {
	if path.IsAbs(rel) {
		return "", apperr.ErrSandboxFileInvalid
	}
	for _, part := range strings.Split(rel, "/") {
		if part == ".." {
			return "", apperr.ErrSandboxFileInvalid
		}
	}
	clean := path.Clean("/" + rel)
	if strings.HasPrefix(clean, "/..") {
		return "", apperr.ErrSandboxFileInvalid
	}
	trimmed := strings.TrimPrefix(clean, "/")
	if trimmed == "." || trimmed == "" {
		return workspaceDir, nil
	}
	return path.Join(workspaceDir, trimmed), nil
}

// shellQuote 对 shell 参数做最小转义,避免把用户路径直接拼进命令。
func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'"'"'`) + "'"
}
