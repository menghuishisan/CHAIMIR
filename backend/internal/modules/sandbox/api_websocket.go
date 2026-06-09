// M2 WebSocket API:承接沙箱进度和终端连接升级,具体运行时执行委托 service。
package sandbox

import (
	"context"
	"errors"
	"io"
	"net/http"

	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"

	"github.com/gorilla/websocket"
)

// serveProgressWS 建立沙箱启动进度 WebSocket,连接建立前先完成归属校验并读取快照。
func (a *API) serveProgressWS(w http.ResponseWriter, r *http.Request, sandboxID int64) error {
	if a.svc == nil || a.svc.hub == nil {
		return apperr.ErrSandboxInvalidState
	}
	progress, err := a.svc.GetSandboxProgress(r.Context(), sandboxID)
	if err != nil {
		return err
	}
	return a.svc.hub.Serve(w, r, func(conn *ws.Conn) error {
		a.svc.hub.Subscribe(conn, progressTopic(sandboxID))
		select {
		case conn.SendChan() <- progressPayload(progress):
		default:
		}
		return nil
	})
}

// serveTerminalWS 建立终端 WebSocket,只负责协议桥接和连接生命周期。
func (a *API) serveTerminalWS(w http.ResponseWriter, r *http.Request, sandboxID int64, container string) (err error) {
	upgrader := websocket.Upgrader{CheckOrigin: a.svc.wsOrigin.Check}
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
	stdinR, stdinW := io.Pipe()
	defer func() {
		if closeErr := stdinW.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	// WebSocket 输入转为 exec stdin,终端执行和审计由 service 层负责。
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
	return a.svc.RunTerminalSession(ctx, sandboxID, container, stdinR, writer, writer)
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
