// M3 判题进度服务:生成进度 topic 并向 WebSocket Hub 发布 worker 状态变化。
package judge

import (
	"context"
	"encoding/json"
	"log/slog"

	"chaimir/internal/platform/ids"
	"chaimir/pkg/logging"
)

// publishProgress 向任务进度 topic 推送状态变化;WS 是体验通道,主状态以数据库为准。
func (s *Service) publishProgress(taskID int64, status int16, message string) {
	if s.hub == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{"task_id": ids.Format(taskID), "status": status, "message": message})
	if err != nil {
		logging.ErrorContext(context.Background(), "judge progress marshal failed", err.Error(), slog.Int64("task_id", taskID))
		return
	}
	s.hub.Broadcast(judgeProgressTopic(taskID), payload)
}

// judgeProgressTopic 生成判题进度 WebSocket topic。
func judgeProgressTopic(taskID int64) string {
	return "judge:task:" + ids.Format(taskID) + ":progress"
}
