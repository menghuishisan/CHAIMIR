// sim compute 文件实现 compute=backend WebSocket 接入和 M4 自有适配器注册表调度。
package sim

import (
	"context"
	"strings"

	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
)

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
	adapter := s.backends[strings.TrimSpace(session.BackendAdapter)]
	if adapter == nil {
		return apperr.ErrSimBackendComputeUnavailable
	}
	if err := adapter.Serve(ctx, session, conn); err != nil {
		return apperr.ErrSimBackendComputeUnavailable.WithCause(err)
	}
	return nil
}
