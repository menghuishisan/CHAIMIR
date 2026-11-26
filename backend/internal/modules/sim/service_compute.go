package sim

import (
	"context"
	"math"
	"strings"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
)

func simSessionSessionScope(sessionID int64) string {
	return "sim:" + ids.Format(sessionID)
}

func simReplaySessionScope(sessionID int64) string {
	return simSessionSessionScope(sessionID) + ":replay"
}

func validateBackendAdapterAvailable(compute int16, adapterCode string, registry BackendRegistry) error {
	if compute != ComputeIsolated {
		return nil
	}
	adapterCode = strings.TrimSpace(adapterCode)
	if adapterCode == "" || registry == nil || registry[adapterCode] == nil {
		return apperr.ErrSimBackendComputeUnavailable
	}
	return nil
}

func validateBackendAdapterConfig(compute int16, adapterCode string, backendConfig map[string]any, registry BackendRegistry) error {
	if err := validateBackendAdapterAvailable(compute, adapterCode, registry); err != nil {
		return err
	}
	if compute != ComputeIsolated {
		return nil
	}
	if err := registry[strings.TrimSpace(adapterCode)].ValidateConfig(backendConfig); err != nil {
		return apperr.ErrSimPackageInvalid.WithCause(err)
	}
	return nil
}

// ServeBackendStream 校验会话归属和能力后,把 WebSocket 交给 M4 自有隔离执行适配器。
func (s *Service) ServeBackendStream(ctx context.Context, conn *ws.Conn, tenantID, accountID, sessionID int64) error {
	var session SessionWithPackage
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		session, err = tx.GetSessionWithPackage(ctx, tenantID, sessionID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		if !sessionAccountAuthorized(session.Session, accountID) {
			return apperr.ErrForbidden
		}
		if err := s.requireSessionModules(ctx, tenantID, session.SourceRef); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if s.wsHub != nil {
		scope := simSessionSessionScope(sessionID)
		if session.Status == SessionArchived {
			scope = simReplaySessionScope(sessionID)
		}
		if err := conn.BindSession(ctx, ws.SessionKey{TenantID: tenantID, AccountID: accountID, Scope: scope}); err != nil {
			return apperr.ErrSimBackendComputeUnavailable.WithCause(err)
		}
	}
	if session.Compute != ComputeIsolated || (session.Status != SessionAvailable && session.Status != SessionArchived) || strings.TrimSpace(session.BackendAdapter) == "" {
		return apperr.ErrSimBackendComputeUnavailable
	}
	if err := validateBackendAdapterConfig(session.Compute, session.BackendAdapter, session.BackendConfig, s.backends); err != nil {
		return err
	}
	adapter := s.backends[strings.TrimSpace(session.BackendAdapter)]
	bundle, err := s.loadBundleForExecution(ctx, session)
	if err != nil {
		return err
	}
	guarded := &backendValidatedConn{svc: s, conn: conn, session: session, tenantID: tenantID, accountID: accountID}
	if err := adapter.Serve(ctx, session, bundle, guarded); err != nil {
		return apperr.ErrSimBackendComputeUnavailable.WithCause(err)
	}
	return nil
}

type backendValidatedConn struct {
	svc       *Service
	conn      *ws.Conn
	session   SessionWithPackage
	tenantID  int64
	accountID int64
	// tick 是最近一帧的推演时刻,用作用户操作入库时的 at_tick ——
	// 回放要靠它把动作放回正确的时刻上(见 frontend features/sim/replayMoves.ts)。
	tick int64
	// diverged 记录本连接是否已回退或重来过;此后不再登记新操作。
	diverged bool
	lastSeq  int32
}

// LoadReplay 必须在取得执行权后读取,以免连接排队时会话已归档或操作记录已推进。
func (c *backendValidatedConn) LoadReplay(ctx context.Context) ([]Action, error) {
	session, actions, err := c.svc.loadReplay(ctx, c.tenantID, c.session.ID)
	if err != nil {
		return nil, err
	}
	if err := c.authorizeExecution(ctx, session.Session); err != nil {
		return nil, err
	}
	c.session = session
	c.lastSeq = 0
	if len(actions) > 0 {
		c.lastSeq = actions[len(actions)-1].Seq
	}
	return actions, nil
}

func (c *backendValidatedConn) authorizeExecution(ctx context.Context, session Session) error {
	if !sessionAccountAuthorized(session, c.accountID) {
		return apperr.ErrForbidden
	}
	// 连接模式在建立时固定；执行中归档必须断开后重新进入只读回放。
	if session.Status != c.session.Status || (session.Status != SessionAvailable && session.Status != SessionArchived) {
		return apperr.ErrSimSessionStateInvalid
	}
	return c.svc.requireSessionModules(ctx, c.tenantID, session.SourceRef)
}

// ReadCommand 复核当前访问权并校验命令；执行成功后才允许登记，避免污染回放。
func (c *backendValidatedConn) ReadCommand(ctx context.Context) (BackendCommand, error) {
	var message BackendClientMessage
	if err := c.conn.ReadJSONContext(ctx, &message); err != nil {
		return BackendCommand{}, err
	}
	if err := c.svc.store.TenantTx(ctx, c.tenantID, func(ctx context.Context, tx TxStore) error {
		session, err := tx.GetSession(ctx, c.tenantID, c.session.ID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		return c.authorizeExecution(ctx, session)
	}); err != nil {
		return BackendCommand{}, err
	}
	switch BackendCommandKind(strings.TrimSpace(message.Type)) {
	case BackendCommandStep, BackendCommandBack, BackendCommandRestart:
		return BackendCommand{Kind: BackendCommandKind(strings.TrimSpace(message.Type))}, nil
	case BackendCommandEvent:
		if c.session.Status == SessionArchived {
			return BackendCommand{}, apperr.ErrSimSessionStateInvalid
		}
		event := BackendEvent{EventType: strings.TrimSpace(message.EventType), Payload: message.Payload}
		if err := validateActionContent(event.EventType, event.Payload); err != nil {
			return BackendCommand{}, err
		}
		if err := validateActionAgainstSchema(c.session.InteractionSchema, event.EventType, event.Payload); err != nil {
			return BackendCommand{}, err
		}
		return BackendCommand{Kind: BackendCommandEvent, Event: event}, nil
	default:
		return BackendCommand{}, apperr.ErrSimActionSeqInvalid
	}
}

// RecordExecuted 只登记成功的用户操作，确定性时刻推进由回放重算。
// 回退或重来后停止追加，保留原过程；浏览器 Worker 遵循同一规则。
func (c *backendValidatedConn) RecordExecuted(ctx context.Context, command BackendCommand) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch command.Kind {
	case BackendCommandBack, BackendCommandRestart:
		c.diverged = true
		return nil
	case BackendCommandEvent:
		if c.diverged {
			return nil
		}
		_, err := c.persist(ctx, command.Event)
		return err
	default:
		return nil
	}
}

// SendFrame 在服务端校验不可信包生成的快照及声明，再推送给前端。
func (c *backendValidatedConn) SendFrame(ctx context.Context, message BackendStreamMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateBackendSnapshot(message.Snapshot, c.session.ScaleLimit); err != nil {
		return err
	}
	if message.Descriptor != nil {
		if err := validateBackendDescriptor(*message.Descriptor, c.session.PackageCode, c.session.PackageVersion, c.session.InteractionSchema); err != nil {
			return err
		}
	}
	message.ReadOnly = c.session.Status == SessionArchived
	c.tick = message.Snapshot.Tick
	return c.conn.SendJSON(message)
}

func (c *backendValidatedConn) persist(ctx context.Context, event BackendEvent) (Action, error) {
	if c.session.Status != SessionAvailable {
		return Action{}, apperr.ErrSimSessionStateInvalid
	}
	var out Action
	atTick, tickOK := intx.Int64ToInt32(c.tick)
	if !tickOK || atTick < 0 {
		return Action{}, apperr.ErrSimActionSeqInvalid
	}
	err := c.svc.store.TenantTx(ctx, c.tenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.LockSessionActions(ctx, c.tenantID, c.session.ID); err != nil {
			return apperr.ErrSimActionSeqInvalid.WithCause(err)
		}
		session, err := tx.GetSession(ctx, c.tenantID, c.session.ID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		if err := c.authorizeExecution(ctx, session); err != nil {
			return err
		}
		last, err := tx.GetLastAction(ctx, c.tenantID, c.session.ID)
		if err != nil && !isNoRows(err) {
			return apperr.ErrSimActionSeqInvalid.WithCause(err)
		}
		lastSeq := int32(0)
		if !isNoRows(err) {
			lastSeq = last.Seq
		}
		if lastSeq != c.lastSeq || lastSeq == math.MaxInt32 || atTick < last.AtTick {
			return apperr.ErrSimActionSeqInvalid
		}
		if err := validateActionReplayPosition(lastSeq+1, atTick, c.session.ScaleLimit); err != nil {
			return err
		}
		created, err := tx.CreateAction(ctx, Action{ID: c.svc.ids.Generate(), TenantID: c.tenantID, SessionID: c.session.ID, Seq: lastSeq + 1, AtTick: atTick, EventType: strings.TrimSpace(event.EventType), Payload: event.Payload})
		if err != nil {
			return apperr.ErrSimActionSeqInvalid.WithCause(err)
		}
		out = created
		return nil
	})
	if err == nil {
		c.lastSeq = out.Seq
	}
	return out, err
}
