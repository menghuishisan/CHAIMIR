// sim service_compute 文件实现 compute=backend WebSocket 接入和 M4 自有适配器注册表调度。
package sim

import (
	"context"
	"encoding/json"
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
	guarded := &backendValidatedConn{ctx: ctx, svc: s, conn: conn, session: session, tenantID: tenantID}
	if err := adapter.Serve(ctx, session, guarded); err != nil {
		return apperr.ErrSimBackendComputeUnavailable.WithCause(err)
	}
	return nil
}

// backendValidatedConn 在 M4 边界统一校验并持久化后端计算客户端事件。
type backendValidatedConn struct {
	ctx      context.Context
	svc      *Service
	conn     *ws.Conn
	session  SessionWithPackage
	tenantID int64
	nextSeq  int32
}

// ReadJSON 只允许适配器读取已通过包内交互 schema 的 BackendEvent。
func (c *backendValidatedConn) ReadJSON(v any) error {
	var event BackendEvent
	if err := c.conn.ReadJSON(&event); err != nil {
		return err
	}
	req := ReportActionRequest{Seq: c.nextSeq + 1, EventType: event.EventType, Payload: event.Payload}
	if err := validateAction(req); err != nil {
		return err
	}
	if err := validateActionAgainstSchema(c.session.InteractionSchema, req); err != nil {
		return err
	}
	action, err := c.persist(event)
	if err != nil {
		return err
	}
	c.nextSeq = action.Seq
	if out, ok := v.(*BackendEvent); ok {
		*out = event
		return nil
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

// SendJSON 复用统一 WebSocket 发送队列输出后端状态。
func (c *backendValidatedConn) SendJSON(v any) error {
	return c.conn.SendJSON(v)
}

// persist 把后端计算事件写入同一条 sim_action_log 序列。
func (c *backendValidatedConn) persist(event BackendEvent) (Action, error) {
	var out Action
	err := c.svc.store.TenantTx(c.ctx, c.tenantID, func(ctx context.Context, tx TxStore) error {
		last, err := tx.GetLastAction(ctx, c.tenantID, c.session.ID)
		if err != nil && !isNoRows(err) {
			return apperr.ErrSimActionSeqInvalid.WithCause(err)
		}
		seq := int32(1)
		if !isNoRows(err) {
			seq = last.Seq + 1
		}
		created, err := tx.CreateAction(ctx, Action{ID: c.svc.ids.Generate(), TenantID: c.tenantID, SessionID: c.session.ID, Seq: seq, AtTick: int32(seq), EventType: strings.TrimSpace(event.EventType), Payload: event.Payload})
		if err != nil {
			return apperr.ErrSimActionSeqInvalid.WithCause(err)
		}
		out = created
		return nil
	})
	return out, err
}
