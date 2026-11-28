package experiment

import (
	"context"
	"encoding/json"
	"log/slog"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"

	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// AuthorizeProgressSubscription 复用实例访问边界，不向订阅者返回实例数据。
func (s *Service) AuthorizeProgressSubscription(ctx context.Context, tenantID, instanceID int64) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	if tenantID != id.TenantID || instanceID <= 0 {
		return apperr.ErrExperimentInstanceAccessDenied
	}
	return s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		inst, err := tx.GetInstance(ctx, tenantID, instanceID)
		if err != nil {
			return err
		}
		return ensureInstanceAccess(ctx, tx, id.AccountID, inst)
	})
}

func instanceProgressTopic(tenantID, instanceID int64) string {
	return "tenant:" + ids.Format(tenantID) + ":experiment:" + ids.Format(instanceID) + ":" + progressChannelName
}

// 只在业务事务提交后发送刷新信号；丢失提示不改变权威记录。
func (s *Service) publishInstanceChanged(ctx context.Context, tenantID, instanceID int64) {
	if s.bus == nil {
		return
	}
	event := contracts.NotifyPushRequestedEvent{TenantID: tenantID, TraceID: logging.TraceFromContext(ctx), Topic: instanceProgressTopic(tenantID, instanceID), Payload: json.RawMessage(`{"type":"instance.changed"}`)}
	if err := s.bus.Publish(ctx, contracts.SubjectNotifyPushRequested, event); err != nil {
		logging.ErrorContext(ctx, "experiment progress publish failed", err.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("instance_id", instanceID))
	}
}
