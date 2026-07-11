// sim service_compute 文件实现 compute=backend WebSocket 接入和 M4 自有适配器注册表调度。
package sim

import (
	"context"
	"sort"
	"strings"

	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
)

// BackendCapabilities 返回当前组合根真实注册的后端计算适配器。
func (s *Service) BackendCapabilities() BackendCapabilitiesDTO {
	items := make([]BackendAdapterDescriptor, 0, len(s.backends))
	for _, adapter := range s.backends {
		if adapter == nil {
			continue
		}
		descriptor := adapter.Descriptor()
		items = append(items, descriptor)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Code < items[j].Code })
	return BackendCapabilitiesDTO{BackendCompute: len(items) > 0, Adapters: items}
}

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

// validateBackendAdapterConfig 在 M4 边界统一校验适配器存在性和包配置。
func validateBackendAdapterConfig(compute int16, adapterCode string, backendConfig map[string]any, registry BackendRegistry) error {
	if err := validateBackendAdapterAvailable(compute, adapterCode, registry); err != nil {
		return err
	}
	if compute != ComputeBackend {
		return nil
	}
	if err := registry[strings.TrimSpace(adapterCode)].ValidateConfig(backendConfig); err != nil {
		return apperr.ErrSimPackageInvalid.WithCause(err)
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
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
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
	if err := validateBackendAdapterConfig(session.Compute, session.BackendAdapter, session.BackendConfig, s.backends); err != nil {
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
	req := ReportActionRequest{Seq: c.nextSeq + 1, AtTick: c.nextSeq + 1, EventType: event.EventType, Payload: event.Payload}
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
	raw, err := jsonx.AnyBytes(event, apperr.ErrSimActionSeqInvalid)
	if err != nil {
		return err
	}
	return jsonx.DecodeStrict(raw, v)
}

// SendJSON 复用统一 WebSocket 发送队列输出后端状态。
func (c *backendValidatedConn) SendJSON(v any) error {
	return c.conn.SendJSON(v)
}

// persist 把后端计算事件写入同一条 sim_action_log 序列。
func (c *backendValidatedConn) persist(event BackendEvent) (Action, error) {
	var out Action
	err := c.svc.store.TenantTx(c.ctx, c.tenantID, func(ctx context.Context, tx TxStore) error {
		session, err := tx.GetSession(ctx, c.tenantID, c.session.ID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		if !canMutateSession(session.Status) {
			return apperr.ErrSimSessionStateInvalid
		}
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
