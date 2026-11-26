package sandbox

import (
	"context"
	"log/slog"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

func progressTopic(tenantID, sandboxID int64) string {
	return "tenant:" + ids.Format(tenantID) + ":sandbox:" + ids.Format(sandboxID) + ":progress"
}

// ProgressSubscription 校验沙箱归属并返回进度主题和当前快照。
func (s *Service) ProgressSubscription(ctx context.Context, tenantID, accountID, sandboxID int64) (string, ProgressMessage, error) {
	sb, err := s.sandboxForOwner(ctx, tenantID, accountID, sandboxID)
	if err != nil {
		return "", ProgressMessage{}, err
	}
	return progressTopic(tenantID, sandboxID), progressFromState(sb.Phase, sb.Status, ""), nil
}

// broadcastProgress 经事件总线请求 M10 统一推送沙箱进度,由 notify 在边界做 topic 租户校验与信封封装;推送失败不影响主状态机。
func (s *Service) broadcastProgress(ctx context.Context, tenantID, sandboxID int64, phase, status int16, traceID string) {
	payload, err := jsonx.AnyBytes(progressFromState(phase, status, traceID), apperr.ErrInternal)
	if err != nil {
		logging.ErrorContext(ctx, "sandbox progress marshal failed", err.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("sandbox_id", sandboxID))
		return
	}
	evt := contracts.NotifyPushRequestedEvent{TenantID: tenantID, TraceID: traceID, Topic: progressTopic(tenantID, sandboxID), Payload: payload}
	if err := s.bus.Publish(ctx, contracts.SubjectNotifyPushRequested, evt); err != nil {
		logging.ErrorContext(ctx, "sandbox progress publish failed", err.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("sandbox_id", sandboxID))
	}
}

func progressFromState(phase, status int16, traceID string) ProgressMessage {
	switch status {
	case SandboxStatusFailed:
		return ProgressMessage{Phase: phase, Status: status, Stage: SandboxProgressStageFailed, Message: "实验环境准备失败,请稍后重试", TraceID: traceID}
	case SandboxStatusPaused:
		return ProgressMessage{Phase: phase, Status: status, Stage: SandboxProgressStagePaused, Message: "实验环境已暂停，恢复后可继续"}
	case SandboxStatusDestroyed:
		return ProgressMessage{Phase: phase, Status: status, Stage: SandboxProgressStageDestroyed, Message: "实验环境已释放"}
	case SandboxStatusRecycling:
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
