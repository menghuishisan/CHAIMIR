// sim service_compute 文件实现隔离执行 WebSocket 接入和 M4 自有能力注册表调度。
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

// simSessionSessionScope 为每个仿真会话建立稳定的 WebSocket 撤销范围。
func simSessionSessionScope(sessionID int64) string {
	return "sim:" + ids.Format(sessionID)
}

// validateBackendAdapterAvailable 确保隔离执行包只能使用已装配的 M4 自有能力。
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

// validateBackendAdapterConfig 在 M4 边界统一校验能力存在性和包配置。
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
		return nil
	}); err != nil {
		return err
	}
	if s.wsHub != nil {
		if err := conn.BindSession(ws.SessionKey{TenantID: tenantID, AccountID: accountID, Scope: simSessionSessionScope(sessionID)}); err != nil {
			return apperr.ErrSimBackendComputeUnavailable.WithCause(err)
		}
	}
	if session.Compute != ComputeIsolated || session.Status == SessionArchived || session.Status == SessionFailed || strings.TrimSpace(session.BackendAdapter) == "" {
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
	// 断线重连、刷新页面都要回到上次离开的位置(FE-9):容器不常驻状态,过程由已登记的操作序列决定,
	// 故连接建立时先取回本会话已有操作,交适配器在容器内重放到当前位置 ——
	// 否则学生刷新一次就从头开始,而操作记录里还留着之前的 10 步,那份记录描述的过程再也复现不出来。
	var recorded []Action
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		items, err := tx.ListActions(ctx, tenantID, sessionID)
		if err != nil {
			return apperr.ErrSimSessionQueryFailed.WithCause(err)
		}
		recorded = items
		return nil
	}); err != nil {
		return err
	}
	guarded := &backendValidatedConn{ctx: ctx, svc: s, conn: conn, session: session, tenantID: tenantID}
	if err := adapter.Serve(ctx, session, bundle, recorded, guarded); err != nil {
		return apperr.ErrSimBackendComputeUnavailable.WithCause(err)
	}
	return nil
}

// backendValidatedConn 在 M4 边界统一校验客户端命令、持久化动作,并校验容器回传的教学帧。
type backendValidatedConn struct {
	ctx      context.Context
	svc      *Service
	conn     *ws.Conn
	session  SessionWithPackage
	tenantID int64
	// tick 是最近一帧的推演时刻,用作用户操作入库时的 at_tick ——
	// 回放要靠它把动作放回正确的时刻上(见 frontend features/sim/replayMoves.ts)。
	tick int64
	// diverged 记录本连接是否已回退或重来过;此后不再登记新操作。
	diverged bool
}

// ReadCommand 只允许适配器拿到已过包内交互 schema 的受控命令。
//
// 这里只做校验不落库:操作要在容器执行成功后才登记(见 RecordExecuted),
// 否则一条被包拒绝的事件也会留在只追加的操作序列里,那份记录描述的过程再也复现不出来。
func (c *backendValidatedConn) ReadCommand() (BackendCommand, error) {
	var message BackendClientMessage
	if err := c.conn.ReadJSON(&message); err != nil {
		return BackendCommand{}, err
	}
	switch BackendCommandKind(strings.TrimSpace(message.Type)) {
	case BackendCommandStep, BackendCommandBack, BackendCommandRestart:
		return BackendCommand{Kind: BackendCommandKind(strings.TrimSpace(message.Type))}, nil
	case BackendCommandEvent:
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

// RecordExecuted 在容器执行成功后登记这条命令的效果。
//
// 时刻推进不入操作记录:它由 seed 决定可复算,与内置包在浏览器 Worker 内的语义一致;
// 把它写进只追加的操作序列会让记录里塞满与用户无关的条目,也会让 seq 失去"第几次操作"的含义。
//
// 回退与重来之后不再登记新操作:操作序列只追加,表达不了"撤回一步",
// 继续追加会让这条记录描述一个从未发生过的过程(与浏览器内置包同一条规则)。
func (c *backendValidatedConn) RecordExecuted(command BackendCommand) error {
	switch command.Kind {
	case BackendCommandBack, BackendCommandRestart:
		c.diverged = true
		return nil
	case BackendCommandEvent:
		if c.diverged {
			return nil
		}
		_, err := c.persist(command.Event)
		return err
	default:
		return nil
	}
}

// SendFrame 校验容器回传的教学帧与包自描述信息后再推给前端。
//
// 容器输出是不可信输入:它由外部提交的仿真包代码算出,可能给出越界模式数、未登记 mode、
// 悬空 layout 引用或超规模状态。前端有一份等价校验,但**后端必须自己也有一份** ——
// 只靠前端等于把协议边界交给不可信来源的下游(见 docs/04-仿真可视化引擎/07-安全设计.md §3)。
func (c *backendValidatedConn) SendFrame(message BackendStreamMessage) error {
	if err := validateBackendSnapshot(message.Snapshot, c.session.ScaleLimit); err != nil {
		return err
	}
	if message.Descriptor != nil {
		if err := validateBackendDescriptor(*message.Descriptor, c.session); err != nil {
			return err
		}
	}
	c.tick = message.Snapshot.Tick
	return c.conn.SendJSON(message)
}

// persist 把隔离执行的用户操作写入同一条 sim_action_log 序列。
func (c *backendValidatedConn) persist(event BackendEvent) (Action, error) {
	var out Action
	atTick, tickOK := intx.Int64ToInt32(c.tick)
	if !tickOK || atTick < 0 {
		return Action{}, apperr.ErrSimActionSeqInvalid
	}
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
			if last.Seq == math.MaxInt32 {
				return apperr.ErrSimActionSeqInvalid
			}
			seq = last.Seq + 1
		}
		created, err := tx.CreateAction(ctx, Action{ID: c.svc.ids.Generate(), TenantID: c.tenantID, SessionID: c.session.ID, Seq: seq, AtTick: atTick, EventType: strings.TrimSpace(event.EventType), Payload: event.Payload})
		if err != nil {
			return apperr.ErrSimActionSeqInvalid.WithCause(err)
		}
		out = created
		return nil
	})
	return out, err
}
