// admin convert 文件负责 M9 领域快照与 HTTP DTO 之间的纯转换。
package admin

import "chaimir/internal/platform/transfer"

// exportTaskDTO 将统一导入导出中心任务快照转换为管理后台导出响应。
func exportTaskDTO(task transfer.Task) transfer.TaskDTO {
	return transfer.TaskToDTO(task)
}
