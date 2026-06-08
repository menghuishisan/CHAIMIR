// M2 WebSocket 交互:progress 订阅与终端 exec 代理。
package sandbox

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

	"github.com/gorilla/websocket"
)

// ServeProgressWS 建立 progress WebSocket。
func (s *Service) ServeProgressWS(w http.ResponseWriter, r *http.Request, sandboxID int64) error {
	if s.hub == nil {
		return apperr.ErrSandboxInvalidState
	}
	progress, err := s.GetSandboxProgress(r.Context(), sandboxID)
	if err != nil {
		return err
	}
	return s.hub.Serve(w, r, func(conn *ws.Conn) error {
		s.hub.Subscribe(conn, progressTopic(sandboxID))
		select {
		case conn.SendChan() <- progressPayload(progress):
		default:
		}
		return nil
	})
}

// ServeTerminalWS 建立终端 exec WebSocket。
func (s *Service) ServeTerminalWS(w http.ResponseWriter, r *http.Request, sandboxID int64, container string) (err error) {
	row, binding, err := s.runtimeBindingForSandbox(r.Context(), sandboxID)
	if err != nil {
		return err
	}
	if container != "" {
		binding.Container = container
	}
	upgrader := websocket.Upgrader{CheckOrigin: s.wsOrigin.Check}
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := socket.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	if err := s.writeSandboxExecEvent(ctx, row.TenantID, row.ID, "terminal-open", map[string]any{"container": binding.Container}); err != nil {
		return err
	}

	stdinR, stdinW := io.Pipe()
	defer func() {
		if closeErr := stdinW.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	go func() {
		defer cancel()
		for {
			_, data, err := socket.ReadMessage()
			if err != nil {
				return
			}
			if _, err := stdinW.Write(data); err != nil {
				return
			}
			if _, err := stdinW.Write([]byte("\n")); err != nil {
				return
			}
		}
	}()

	writer := &wsSocketWriter{socket: socket}
	command := []string{"sh", "-lc", "cd " + shellQuote(binding.WorkspaceDir) + " && ${SHELL:-/bin/sh}"}
	err = s.orchestrator.Exec(ctx, binding, command, stdinR, writer, writer, true)
	if eventErr := s.writeSandboxExecEvent(r.Context(), row.TenantID, row.ID, "terminal-close", map[string]any{"container": binding.Container}); eventErr != nil {
		logging.ErrorContext(r.Context(), "terminal close event failed", eventErr.Error(), slog.Int64("sandbox_id", row.ID))
	}
	return err
}

// wsSocketWriter 把 exec 输出写回 WebSocket。
type wsSocketWriter struct {
	socket *websocket.Conn
}

// Write 把一段 exec 输出作为 WebSocket 文本帧发送。
func (w *wsSocketWriter) Write(p []byte) (int, error) {
	if err := w.socket.WriteMessage(websocket.TextMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}
