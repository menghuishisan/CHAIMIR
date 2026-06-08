// M4 WebSocket 交互:为 compute=backend 的仿真会话提供事件输入与状态输出流。
package sim

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"chaimir/internal/modules/sim/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"

	"github.com/gorilla/websocket"
)

// ServeBackendStreamWS 建立后端计算仿真 WebSocket,客户端事件经适配器步进后返回状态。
func (s *Service) ServeBackendStreamWS(w http.ResponseWriter, r *http.Request, sessionID int64) (err error) {
	session, err := s.loadBackendSession(r.Context(), sessionID)
	if err != nil {
		return err
	}
	upgrader := websocket.Upgrader{CheckOrigin: s.wsOrigin.Check}
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := socket.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	state := map[string]any{}
	tick := int32(0)
	for {
		var event ReportActionRequest
		if err := socket.ReadJSON(&event); err != nil {
			return nil
		}
		// 每条客户端事件都通过已审核包声明的后端适配器执行,不在平台进程里动态执行包代码。
		out, err := s.backend.Step(r.Context(), session.PackageBackendAdapter.String, BackendStepInput{
			SessionID: session.ID, Tick: tick, EventType: event.EventType, Payload: event.Payload,
			State: state, Config: jsonx.ObjectMap(session.PackageBackendConfig),
		})
		if err != nil {
			return err
		}
		tick = out.Tick
		state = out.State
		payload, err := json.Marshal(out)
		if err != nil {
			return apperr.ErrSimBackendUnavailable.WithCause(err)
		}
		if err := socket.WriteMessage(websocket.TextMessage, payload); err != nil {
			return err
		}
	}
}

// loadBackendSession 校验会话存在、归属当前租户且运行位置为后端。
func (s *Service) loadBackendSession(ctx context.Context, sessionID int64) (sqlcgen.GetSimSessionWithPackageRow, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return sqlcgen.GetSimSessionWithPackageRow{}, apperr.ErrUnauthorized
	}
	var row sqlcgen.GetSimSessionWithPackageRow
	if err := s.repo.inTenantID(ctx, id.TenantID, func(q *sqlcgen.Queries) error {
		found, e := q.GetSimSessionWithPackage(ctx, sessionID)
		if db.IsNoRows(e) {
			return apperr.ErrSimSessionNotFound
		}
		row = found
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return sqlcgen.GetSimSessionWithPackageRow{}, ae
		}
		return sqlcgen.GetSimSessionWithPackageRow{}, apperr.ErrSimSessionNotFound.WithCause(err)
	}
	if row.Compute != ComputeBackend || !row.PackageBackendAdapter.Valid {
		return sqlcgen.GetSimSessionWithPackageRow{}, apperr.ErrSimBackendUnavailable
	}
	if err := authorizeSessionOwner(id, row.OwnerAccountID); err != nil {
		return sqlcgen.GetSimSessionWithPackageRow{}, err
	}
	return row, nil
}
