// identity service_tenant_provision 文件实现新租户初始化事件的事务 outbox 与可靠发布。
package identity

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/textx"

	"github.com/google/uuid"
)

// tenantProvisionFailureDetailLimit 是失败原因入库的字符上限,与 tenant_provision_outbox.detail 列宽一致。
const tenantProvisionFailureDetailLimit = 255

// enqueueTenantProvision 在租户创建事务内保存跨模块初始化事件。
func (s *Service) enqueueTenantProvision(ctx context.Context, tx TxStore, item Tenant) error {
	if item.ID <= 0 || item.DeployMode <= 0 {
		return apperr.ErrInternal
	}
	traceID := strings.TrimSpace(response.TraceFromContext(ctx))
	if traceID == "" {
		traceID = uuid.NewString()
	}
	_, err := tx.CreateTenantProvisionOutbox(ctx, TenantProvisionOutbox{
		ID: s.ids.Generate(), TenantID: item.ID, DeployMode: item.DeployMode,
		TraceID: traceID, ProvisionedAt: timex.Now(),
	})
	if err != nil {
		return apperr.ErrInternal.WithCause(fmt.Errorf("保存新租户初始化事件失败: %w", err))
	}
	return nil
}

// RunTenantProvisionOutboxOnce 领取并发布一批新租户初始化事件。
func (s *Service) RunTenantProvisionOutboxOnce(ctx context.Context) error {
	if s.bus == nil {
		return apperr.ErrInternal.WithCause(fmt.Errorf("新租户初始化事件总线未装配"))
	}
	limit, ok := intx.Int32(s.cfg.TenantProvisionOutboxBatch)
	if !ok || limit <= 0 {
		return apperr.ErrInternal
	}
	now := timex.Now()
	staleBefore := now
	leaseUntil := now.Add(time.Duration(s.cfg.TenantProvisionOutboxStaleMs) * time.Millisecond)
	var items []TenantProvisionOutbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		maxAttempts, ok := intx.Int32(s.cfg.TenantProvisionOutboxMaxAttempts)
		if !ok || maxAttempts <= 0 {
			return apperr.ErrInternal
		}
		items, err = tx.ClaimTenantProvisionOutbox(ctx, limit, maxAttempts, staleBefore, leaseUntil)
		return err
	}); err != nil {
		return apperr.ErrInternal.WithCause(fmt.Errorf("领取新租户初始化事件失败: %w", err))
	}
	var firstErr error
	for _, item := range items {
		if err := s.publishTenantProvisionOutbox(ctx, item); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			logging.ErrorContext(ctx, "tenant provision outbox publish failed", err.Error(), slog.Int64("tenant_id", item.TenantID), slog.Int64("outbox_id", item.ID))
		}
	}
	return firstErr
}

// publishTenantProvisionOutbox 发布单条事件并更新 outbox 终态。
func (s *Service) publishTenantProvisionOutbox(ctx context.Context, item TenantProvisionOutbox) error {
	eventCtx := response.WithTrace(ctx, item.TraceID)
	event := contracts.TenantProvisionedEvent{TenantID: item.TenantID, TraceID: item.TraceID, DeployMode: item.DeployMode, ProvisionedAt: item.ProvisionedAt}
	if err := s.bus.Publish(eventCtx, contracts.SubjectTenantProvisioned, event); err != nil {
		s.recordTenantProvisionFailure(eventCtx, item, err)
		return apperr.ErrInternal.WithCause(fmt.Errorf("发布新租户初始化事件失败: %w", err))
	}
	return s.store.PrivilegedTx(eventCtx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkTenantProvisionOutboxPublished(ctx, item.ID, item.LeaseToken)
		return err
	})
}

// recordTenantProvisionFailure 持久化发布失败原因，记录失败本身也必须可定位。
func (s *Service) recordTenantProvisionFailure(ctx context.Context, item TenantProvisionOutbox, cause error) {
	detail := textx.TruncateRunes(cause.Error(), tenantProvisionFailureDetailLimit)
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkTenantProvisionOutboxFailed(ctx, item.ID, detail, item.LeaseToken)
		return err
	}); err != nil {
		logging.ErrorContext(ctx, "tenant provision outbox failure persist failed", err.Error(), slog.Int64("outbox_id", item.ID))
	}
}

// drainTenantProvisionOutboxBestEffort 在租户事务提交后立即尝试发布，失败由后台任务继续重试。
func (s *Service) drainTenantProvisionOutboxBestEffort(ctx context.Context) {
	if s.bus == nil {
		return
	}
	if err := s.RunTenantProvisionOutboxOnce(ctx); err != nil {
		logging.ErrorContext(ctx, "tenant provision outbox immediate drain failed", err.Error())
	}
}
