// M2 终端交互服务:在已建立的输入输出流上执行沙箱终端并记录事件。
package sandbox

import (
	"context"
	"io"
	"log/slog"

	"chaimir/pkg/logging"
)

// RunTerminalSession 在已建立的终端输入输出流上执行运行时 shell。
func (s *Service) RunTerminalSession(ctx context.Context, sandboxID int64, container string, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	// 先复用 runtimeBindingForSandbox 完成权限、状态和工作区绑定校验。
	row, binding, err := s.runtimeBindingForSandbox(ctx, sandboxID)
	if err != nil {
		return err
	}
	if container != "" {
		binding.Container = container
	}
	if err := s.writeSandboxExecEvent(ctx, row.TenantID, row.ID, "terminal-open", map[string]any{"container": binding.Container}); err != nil {
		return err
	}

	// 在工作区内启动 shell,退出时无论成功失败都记录关闭事件。
	command := []string{"sh", "-lc", "cd " + shellQuote(binding.WorkspaceDir) + " && ${SHELL:-/bin/sh}"}
	err = s.orchestrator.Exec(ctx, binding, command, stdin, stdout, stderr, true)
	if eventErr := s.writeSandboxExecEvent(ctx, row.TenantID, row.ID, "terminal-close", map[string]any{"container": binding.Container}); eventErr != nil {
		logging.ErrorContext(ctx, "terminal close event failed", eventErr.Error(), slog.Int64("sandbox_id", row.ID))
	}
	return err
}
