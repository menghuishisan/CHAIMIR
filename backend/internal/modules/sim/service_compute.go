// sim service_compute 文件实现 compute=backend WebSocket 接入和 M4 自有适配器注册表调度。
package sim

import (
	"context"
	"strings"

	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
)

// validateBackendAdapterAvailable 确保 compute=backend 包只能使用已装配的 M4 自有适配器。
func validateBackendAdapterAvailable(compute int16, adapterCode string, registry BackendRegistry) error {
	if compute != ComputeBackend {
		return nil
	}
	adapterCode = strings.TrimSpace(adapterCode)
	if adapterCode == "" || registry == nil || registry[adapterCode] == nil {
		return apperr.ErrSimBackendComputeUnavailable
	}
	return nil
}

// ServeBackendStream 校验会话归属和适配器后,把 WebSocket 交给 M4 自有后端计算适配器。
func (s *Service) ServeBackendStream(ctx context.Context, conn *ws.Conn, tenantID, accountID, sessionID int64) error {
	var session SessionWithPackage
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		session, err = tx.GetSessionWithPackage(ctx, tenantID, sessionID)
		if err != nil {
			return apperr.ErrSimSessionNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	if session.OwnerAccountID != accountID {
		return apperr.ErrForbidden
	}
	if session.Compute != ComputeBackend || session.Status == SessionArchived || session.Status == SessionFailed || strings.TrimSpace(session.BackendAdapter) == "" {
		return apperr.ErrSimBackendComputeUnavailable
	}
	if err := validateBackendAdapterAvailable(session.Compute, session.BackendAdapter, s.backends); err != nil {
		return err
	}
	adapter := s.backends[strings.TrimSpace(session.BackendAdapter)]
	if err := adapter.Serve(ctx, session, conn); err != nil {
		return apperr.ErrSimBackendComputeUnavailable.WithCause(err)
	}
	return nil
}
