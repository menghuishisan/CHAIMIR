// M3 WebSocket API:承接判题进度连接升级和 topic 订阅,不处理判题状态机。
package judge

import (
	"net/http"

	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
)

// serveProgressWS 建立判题进度 WebSocket 并订阅指定任务 topic。
func (a *API) serveProgressWS(w http.ResponseWriter, r *http.Request, taskID int64) error {
	if a.svc == nil || a.svc.hub == nil {
		return apperr.ErrJudgeConfigUnavailable
	}
	// 建连时只订阅 M3 任务 topic,历史进度由任务查询接口补齐。
	return a.svc.hub.Serve(w, r, func(c *ws.Conn) error {
		a.svc.hub.Subscribe(c, judgeProgressTopic(taskID))
		return nil
	})
}
