// identity service_tenant_provision 文件实现新租户初始化事件的事务 outbox 与可靠发布。
package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/intx"

	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"
	"chaimir/pkg/textx"

	"github.com/google/uuid"
)

const (
	tenantProvisionFailureDetailLimit = 255 // tenant_provision_outbox.last_error
	tenantProvisionOutboxExhausted    = 5
)

// enqueueTenantProvision 在租户创建事务内保存跨模块初始化事件。
func enqueueTenantProvision(ctx context.Context, tx TxStore, ids snowflake.Generator, item Tenant) error {
	if item.ID <= 0 || item.DeployMode <= 0 {
		return apperr.ErrInternal
	}
	traceID := strings.TrimSpace(logging.TraceFromContext(ctx))
	if traceID == "" {
		traceID = uuid.NewString()
	}
	err := tx.CreateTenantProvisionOutbox(ctx, TenantProvisionOutbox{
		ID: ids.Generate(), TenantID: item.ID, DeployMode: item.DeployMode,
		TraceID: traceID, ProvisionedAt: timex.Now(),
	})
	if err != nil {
		return apperr.ErrInternal.WithCause(fmt.Errorf("保存新租户初始化事件失败: %w", err))
	}
	return nil
}

// RunTenantProvisionOutboxOnce 领取并发布一批新租户初始化事件。
func (s *Service) RunTenantProvisionOutboxOnce(ctx context.Context) error {
	maxAttempts, ok := intx.Int32(s.cfg.TenantProvisionOutboxMaxAttempts)
	if !ok || maxAttempts <= 0 {
		return apperr.ErrInternal
	}
	lease := time.Duration(s.cfg.TenantProvisionOutboxStaleMs) * time.Millisecond
	var failures []error
	for range s.cfg.TenantProvisionOutboxBatch {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		var item TenantProvisionOutbox
		if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
			var err error
			item, err = tx.ClaimTenantProvisionOutbox(ctx, maxAttempts, lease)
			return err
		}); err != nil {
			return errors.Join(append(failures, fmt.Errorf("领取新租户初始化事件失败: %w", err))...)
		}
		if item.ID == 0 {
			break
		}
		if item.Status == tenantProvisionOutboxExhausted {
			logging.ErrorContext(logging.WithTrace(ctx, item.TraceID), "tenant provision outbox exhausted", item.LastError,
				slog.Int64("tenant_id", item.TenantID), slog.Int64("outbox_id", item.ID))
			continue
		}
		if err := s.publishTenantProvisionOutbox(ctx, item, maxAttempts, lease); err != nil {
			failures = append(failures, err)
			logging.ErrorContext(ctx, "tenant provision outbox publish failed", err.Error(), slog.Int64("tenant_id", item.TenantID), slog.Int64("outbox_id", item.ID))
		}
	}
	return errors.Join(failures...)
}

// publishTenantProvisionOutbox 发布单条事件并更新 outbox 终态。
func (s *Service) publishTenantProvisionOutbox(ctx context.Context, item TenantProvisionOutbox, maxAttempts int32, retryDelay time.Duration) error {
	eventCtx := logging.WithTrace(ctx, item.TraceID)
	event := contracts.TenantProvisionedEvent{
		EventID: strconv.FormatInt(item.ID, 10), TenantID: item.TenantID, TraceID: item.TraceID,
		DeployMode: item.DeployMode, ProvisionedAt: item.ProvisionedAt,
	}
	if err := s.bus.Publish(eventCtx, contracts.SubjectTenantProvisioned, event); err != nil {
		return apperr.ErrInternal.WithCause(errors.Join(
			fmt.Errorf("发布新租户初始化事件失败: %w", err),
			s.recordTenantProvisionFailure(eventCtx, item, err, maxAttempts, retryDelay),
		))
	}
	return s.store.PrivilegedTx(eventCtx, func(ctx context.Context, tx TxStore) error {
		return tx.MarkTenantProvisionOutboxPublished(ctx, item.ID, item.LeaseToken)
	})
}

// recordTenantProvisionFailure 持久化发布失败原因，记录失败本身也必须可定位。
func (s *Service) recordTenantProvisionFailure(ctx context.Context, item TenantProvisionOutbox, cause error, maxAttempts int32, retryDelay time.Duration) error {
	detail := textx.TruncateRunes(logging.SanitizeError(cause.Error()), tenantProvisionFailureDetailLimit)
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		return tx.MarkTenantProvisionOutboxFailed(ctx, item.ID, detail, item.LeaseToken, maxAttempts, retryDelay)
	}); err != nil {
		return fmt.Errorf("保存新租户初始化事件失败状态: %w", err)
	}
	return nil
}

// drainTenantProvisionOutboxBestEffort 在租户事务提交后立即尝试发布，失败由后台任务继续重试。
func (s *Service) drainTenantProvisionOutboxBestEffort(ctx context.Context) {
	if err := s.RunTenantProvisionOutboxOnce(ctx); err != nil {
		logging.ErrorContext(ctx, "tenant provision outbox immediate drain failed", err.Error())
	}
}
