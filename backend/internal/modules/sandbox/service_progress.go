// sandbox service_progress 文件实现沙箱启动进度的统一 WebSocket 推送编排。
package sandbox

import (
	"context"
	"fmt"
	"log/slog"

	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// progressTopic 生成租户内单沙箱进度主题,确保不同租户和沙箱不会串线。
func progressTopic(tenantID, sandboxID int64) string {
	return fmt.Sprintf("sandbox:%d:%d:progress", tenantID, sandboxID)
}

// ProgressSubscription 校验沙箱归属并返回进度主题和当前快照。
func (s *Service) ProgressSubscription(ctx context.Context, tenantID, accountID, sandboxID int64) (string, ProgressMessage, error) {
	sb, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID)
	if err != nil {
		return "", ProgressMessage{}, err
	}
	return progressTopic(tenantID, sandboxID), progressFromState(sb.Phase, sb.Status, ""), nil
}

// broadcastProgress 向已订阅前端广播沙箱进度,广播失败不影响主状态机但必须由 Hub 统一丢弃慢连接。
func (s *Service) broadcastProgress(tenantID, sandboxID int64, phase, status int16, traceID string) {
	if s.wsHub == nil {
		return
	}
	data, err := jsonx.AnyBytes(progressFromState(phase, status, traceID), apperr.ErrInternal)
	if err != nil {
		logging.ErrorContext(context.Background(), "sandbox progress marshal failed", err.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("sandbox_id", sandboxID))
		return
	}
	s.wsHub.Broadcast(progressTopic(tenantID, sandboxID), data)
}

// progressFromState 把内部状态转换为用户可理解的阶段文案,不暴露 Pod、Namespace 或镜像错误。
func progressFromState(phase, status int16, traceID string) ProgressMessage {
	if status == SandboxStatusFailed {
		return ProgressMessage{Phase: phase, Status: status, Stage: SandboxProgressStageFailed, Message: "实验环境准备失败,请稍后重试", TraceID: traceID}
	}
	if status == SandboxStatusRecycling || status == SandboxStatusDestroyed {
		return ProgressMessage{Phase: phase, Status: status, Stage: SandboxProgressStageRecycling, Message: "实验环境正在释放"}
	}
	switch phase {
	case SandboxPhaseReady:
		return ProgressMessage{Phase: phase, Status: status, Stage: SandboxProgressStageReady, Message: "节点已就绪,可进入"}
	case SandboxPhaseInitializing:
		return ProgressMessage{Phase: phase, Status: status, Stage: SandboxProgressStageInitializing, Message: "实验环境正在初始化"}
	case SandboxPhaseFullyReady:
		return ProgressMessage{Phase: phase, Status: status, Stage: SandboxProgressStageReady, Message: "实验环境已就绪"}
	default:
		return ProgressMessage{Phase: phase, Status: status, Stage: SandboxProgressStageAllocating, Message: "实验环境正在准备"}
	}
}
