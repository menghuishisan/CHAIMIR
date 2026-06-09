// M4 WebSocket API:承接后端计算仿真流连接升级和帧协议转换。
package sim

import (
	"encoding/json"
	"errors"
	"net/http"

	"chaimir/pkg/apperr"

	"github.com/gorilla/websocket"
)

// serveBackendStreamWS 建立后端计算仿真 WebSocket,客户端事件经适配器步进后返回状态。
func (a *API) serveBackendStreamWS(w http.ResponseWriter, r *http.Request, sessionID int64) (err error) {
	session, err := a.svc.loadBackendSession(r.Context(), sessionID)
	if err != nil {
		return err
	}
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

	state := map[string]any{}
	tick := int32(0)
	for {
		var event ReportActionRequest
		if err := socket.ReadJSON(&event); err != nil {
			return nil
		}
		// 每条客户端事件都通过已审核包声明的后端适配器执行,不在平台进程里动态执行包代码。
		out, err := a.svc.backend.Step(r.Context(), session.BackendAdapter, BackendStepInput{
			SessionID: session.ID, Tick: tick, EventType: event.EventType, Payload: event.Payload,
			State: state, Config: session.BackendConfig,
		})
		if err != nil {
			return err
		}
		tick = out.Tick
		state = out.State
		payload, err := json.Marshal(out)
		if err != nil {
			return apperr.ErrSimBackendUnavailable.WithCause(err)
		}
		if err := socket.WriteMessage(websocket.TextMessage, payload); err != nil {
			return err
		}
	}
}
