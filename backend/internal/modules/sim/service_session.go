// sim service_session 文件实现会话创建、操作序列、回放、分享和检查点能力。
package sim

import (
	"context"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// CreateSession 创建仿真会话并锁定仿真包版本。
func (s *Service) CreateSession(ctx context.Context, req contracts.SimCreateSessionRequest) (contracts.SimSessionInfo, error) {
	req.SourceRef = strings.TrimSpace(req.SourceRef)
	initParams := req.InitParams
	if initParams == nil {
		initParams = map[string]any{}
	}
	create := CreateSessionRequest{PackageCode: req.PackageCode, Version: req.Version, Seed: req.Seed, InitParams: initParams, OwnerAccountID: ids.ID(req.OwnerAccountID), SourceRef: req.SourceRef}
	if err := validateCreateSession(create, req.TenantID); err != nil {
		return contracts.SimSessionInfo{}, err
	}
	if !auth.ServiceSourceRefAuthorized(ctx, req.SourceRef) {
		return contracts.SimSessionInfo{}, apperr.ErrServiceUnauthorized
	}
	pkg, err := s.loadPackage(ctx, req.PackageCode, req.Version)
	if err != nil {
		return contracts.SimSessionInfo{}, err
	}
	if pkg.Status != PackageStatusPublished {
		return contracts.SimSessionInfo{}, apperr.ErrSimPackageUnavailable
	}
	if err := validateBackendAdapterConfig(pkg.Compute, pkg.BackendAdapter, pkg.BackendConfig, s.backends); err != nil {
		return contracts.SimSessionInfo{}, err
	}
	session := Session{ID: s.ids.Generate(), TenantID: req.TenantID, PackageID: pkg.ID, SourceRef: req.SourceRef, OwnerAccountID: req.OwnerAccountID, Seed: req.Seed, InitParams: initParams, Compute: pkg.Compute, Status: SessionCreating}
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		session, err = tx.CreateSession(ctx, session)
		if err != nil {
			return apperr.ErrSimSessionInvalid.WithCause(err)
		}
		session, err = tx.UpdateSessionStatus(ctx, req.TenantID, session.ID, SessionRunning)
		if err != nil {
			return apperr.ErrSimSessionStateInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return contracts.SimSessionInfo{}, err
	}
	if err := s.writeSystemAudit(ctx, req.TenantID, "sim.session.create", "sim_session", session.ID, map[string]any{"source_ref": session.SourceRef, "owner_account_id": req.OwnerAccountID, "package": pkg.Code + ":" + pkg.Version}); err != nil {
		return contracts.SimSessionInfo{}, err
	}
	return sessionToContract(session, pkg)
}

// CreateSessionFromHTTP 转换内部 HTTP 请求为跨模块契约调用。
func (s *Service) CreateSessionFromHTTP(ctx context.Context, tenantID int64, req CreateSessionRequest) (map[string]any, error) {
	info, err := s.CreateSession(ctx, contracts.SimCreateSessionRequest{TenantID: tenantID, PackageCode: req.PackageCode, Version: req.Version, Seed: req.Seed, InitParams: req.InitParams, OwnerAccountID: req.OwnerAccountID.Int64(), SourceRef: req.SourceRef})
	if err != nil {
		return nil, err
	}
	return map[string]any{"session_id": ids.Format(info.SessionID), "compute": info.Compute, "bundle_ref": info.BundleRef}, nil
}

// ReportAction 保存用户操作序列,同 seq 同内容幂等,同 seq 不同内容拒绝。
func (s *Service) ReportAction(ctx context.Context, tenantID, accountID, sessionID int64, req ReportActionRequest) (map[string]any, error) {
	if err := validateAction(req); err != nil {
		return nil, err
	}
	req.EventType = strings.TrimSpace(req.EventType)
	if req.Payload == nil {
		req.Payload = map[string]any{}
	}
	var out Action
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		session, err := tx.GetSessionWithPackage(ctx, tenantID, sessionID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		if session.OwnerAccountID != accountID {
			return apperr.ErrForbidden
		}
		if !canMutateSession(session.Status) {
			return apperr.ErrSimSessionStateInvalid
		}
		if err := validateActionAgainstSchema(session.InteractionSchema, req); err != nil {
			return err
		}
		existing, err := tx.GetActionBySeq(ctx, tenantID, sessionID, req.Seq)
		if err == nil {
			same, err := actionEqual(existing, req)
			if err != nil {
				return err
			}
			if same {
				out = existing
				return nil
			}
			return apperr.ErrSimActionSeqInvalid
		}
		if !isNoRows(err) {
			return apperr.ErrSimActionSeqInvalid.WithCause(err)
		}
		last, err := tx.GetLastAction(ctx, tenantID, sessionID)
		if err != nil && !isNoRows(err) {
			return apperr.ErrSimActionSeqInvalid.WithCause(err)
		}
		if isNoRows(err) {
			if req.Seq != 1 {
				return apperr.ErrSimActionSeqInvalid
			}
		} else if req.Seq != last.Seq+1 {
			return apperr.ErrSimActionSeqInvalid
		}
		out, err = tx.CreateAction(ctx, Action{ID: s.ids.Generate(), TenantID: tenantID, SessionID: sessionID, Seq: req.Seq, AtTick: req.AtTick, EventType: req.EventType, Payload: req.Payload})
		if err != nil {
			return apperr.ErrSimActionSeqInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return actionToMap(out), nil
}

// GetReplay 返回可复现的 seed、参数与操作序列。
func (s *Service) GetReplay(ctx context.Context, tenantID, sessionID int64) (contracts.SimReplayInfo, error) {
	session, actions, err := s.loadReplay(ctx, tenantID, sessionID)
	if err != nil {
		return contracts.SimReplayInfo{}, err
	}
	return replayToContract(session, actions), nil
}

// GetReplayForUser 读取当前用户可见的回放。
func (s *Service) GetReplayForUser(ctx context.Context, tenantID, accountID, sessionID int64) (map[string]any, error) {
	session, actions, err := s.loadReplay(ctx, tenantID, sessionID)
	if err != nil {
		return nil, err
	}
	if session.OwnerAccountID != accountID {
		return nil, apperr.ErrForbidden
	}
	return replayToMap(session, actions), nil
}

// DestroySession 回收单个仿真会话,并强制来源标识与目标会话一致。
func (s *Service) DestroySession(ctx context.Context, req contracts.SimDestroySessionRequest) error {
	req.SourceRef = strings.TrimSpace(req.SourceRef)
	if req.TenantID <= 0 || req.SessionID <= 0 || !auth.ValidSourceRef(req.SourceRef) {
		return apperr.ErrSimSessionInvalid
	}
	if !auth.ServiceSourceRefAuthorized(ctx, req.SourceRef) {
		return apperr.ErrServiceUnauthorized
	}
	var archived Session
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		session, err := tx.GetSession(ctx, req.TenantID, req.SessionID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		if session.SourceRef != req.SourceRef {
			return apperr.ErrServiceUnauthorized
		}
		if !canArchiveSession(session.Status) {
			return apperr.ErrSimSessionStateInvalid
		}
		archived, err = tx.UpdateSessionStatus(ctx, req.TenantID, req.SessionID, SessionArchived)
		if err != nil {
			return apperr.ErrSimSessionStateInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := s.releaseBackendSessions(ctx, req.TenantID, []Session{archived}); err != nil {
		return err
	}
	return s.writeSystemAudit(ctx, req.TenantID, "sim.session.archive", "sim_session", archived.ID, map[string]any{"source_ref": archived.SourceRef})
}

// RecycleBySourceRef 按来源标识归档仿真会话并释放后端计算资源。
func (s *Service) RecycleBySourceRef(ctx context.Context, req contracts.SimRecycleRequest) error {
	req.SourceRef = strings.TrimSpace(req.SourceRef)
	if req.TenantID <= 0 || !auth.ValidSourceRef(req.SourceRef) {
		return apperr.ErrSimSessionInvalid
	}
	if !auth.ServiceSourceRefAuthorized(ctx, req.SourceRef) {
		return apperr.ErrServiceUnauthorized
	}
	var archived []Session
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		archived, err = tx.ArchiveSessionsBySourceRef(ctx, req.TenantID, req.SourceRef)
		if err != nil {
			return apperr.ErrSimSessionStateInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := s.releaseBackendSessions(ctx, req.TenantID, archived); err != nil {
		return err
	}
	for _, session := range archived {
		if err := s.writeSystemAudit(ctx, req.TenantID, "sim.session.archive", "sim_session", session.ID, map[string]any{"source_ref": session.SourceRef, "reason": strings.TrimSpace(req.Reason)}); err != nil {
			return err
		}
	}
	return nil
}

// ReportCheckpoint 保存仿真检查点结果快照,供 M3 后续判分读取。
func (s *Service) ReportCheckpoint(ctx context.Context, req contracts.SimCheckpointRequest) error {
	sourceRef := strings.TrimSpace(req.SourceRef)
	if !auth.ValidSourceRef(sourceRef) {
		return apperr.ErrSimCheckpointInvalid
	}
	if !auth.ServiceSourceRefAuthorized(ctx, sourceRef) {
		return apperr.ErrServiceUnauthorized
	}
	return s.reportCheckpointRaw(ctx, req.TenantID, req.SessionID, sourceRef, req.CheckpointID, req.Answer, req.Achieved)
}

// ReportCheckpointFromHTTP 保存 HTTP 内部接口上报的检查点。
func (s *Service) ReportCheckpointFromHTTP(ctx context.Context, tenantID, sessionID int64, req ReportCheckpointRequest) error {
	sourceRef, ok := auth.ServiceSourceRefFromContext(ctx)
	if !ok {
		return apperr.ErrServiceUnauthorized
	}
	if !auth.ValidSourceRef(sourceRef) {
		return apperr.ErrSimCheckpointInvalid
	}
	if !auth.ServiceSourceRefAuthorized(ctx, sourceRef) {
		return apperr.ErrServiceUnauthorized
	}
	return s.reportCheckpointRaw(ctx, tenantID, sessionID, sourceRef, req.CheckpointID, req.Answer, req.Achieved)
}

// ShareSession 为用户会话创建公开分享码。
func (s *Service) ShareSession(ctx context.Context, tenantID, accountID, sessionID int64, expireAt time.Time) (map[string]any, error) {
	if !expireAt.IsZero() && !expireAt.After(timex.Now()) {
		return nil, apperr.ErrSimShareCodeInvalid
	}
	var share Share
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		session, err := tx.GetSession(ctx, tenantID, sessionID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		if session.OwnerAccountID != accountID {
			return apperr.ErrForbidden
		}
		if session.Status == SessionFailed || session.Status == SessionArchived {
			return apperr.ErrSimSessionStateInvalid
		}
		var lastErr error
		for attempt := 0; attempt < 5; attempt++ {
			code, err := newShareCode()
			if err != nil {
				return apperr.ErrSimShareCodeInvalid.WithCause(err)
			}
			share, err = tx.CreateShare(ctx, Share{ID: s.ids.Generate(), TenantID: tenantID, SessionID: sessionID, Code: code, CreatedBy: accountID, ExpireAt: expireAt})
			if err == nil {
				return nil
			}
			if !isUniqueViolation(err) {
				return apperr.ErrSimShareCodeInvalid.WithCause(err)
			}
			lastErr = err
		}
		return apperr.ErrSimShareCodeInvalid.WithCause(lastErr)
	}); err != nil {
		return nil, err
	}
	return map[string]any{"code": share.Code, "expire_at": share.ExpireAt, "status": "active"}, nil
}

// GetSharedReplay 按公开分享码读取可复现剧本,分享索引本身不存剧本正文。
func (s *Service) GetSharedReplay(ctx context.Context, code string) (map[string]any, error) {
	if strings.TrimSpace(code) == "" || len(strings.TrimSpace(code)) > 48 {
		return nil, apperr.ErrSimShareCodeInvalid
	}
	var share Share
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		share, err = tx.GetShareByCode(ctx, strings.TrimSpace(code))
		if err != nil {
			return lookupError(err, apperr.ErrSimShareCodeInvalid, apperr.ErrSimShareQueryFailed)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if !shareUsable(share, timex.Now()) {
		return nil, apperr.ErrSimShareCodeInvalid
	}
	var (
		session SessionWithPackage
		actions []Action
	)
	if err := s.store.TenantTx(ctx, share.TenantID, func(ctx context.Context, tx TxStore) error {
		tenantShare, err := tx.GetShareByCode(ctx, strings.TrimSpace(code))
		if err != nil {
			return lookupError(err, apperr.ErrSimShareCodeInvalid, apperr.ErrSimShareQueryFailed)
		}
		if tenantShare.ID != share.ID || tenantShare.SessionID != share.SessionID || !shareUsable(tenantShare, timex.Now()) {
			return apperr.ErrSimShareCodeInvalid
		}
		session, err = tx.GetSessionWithPackage(ctx, share.TenantID, share.SessionID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		actions, err = tx.ListActions(ctx, share.TenantID, share.SessionID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionStateInvalid, apperr.ErrSimSessionQueryFailed)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return replayToMapPublic(session, actions), nil
}

// loadReplay 读取会话和有序操作序列。
func (s *Service) loadReplay(ctx context.Context, tenantID, sessionID int64) (SessionWithPackage, []Action, error) {
	var session SessionWithPackage
	var actions []Action
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		session, err = tx.GetSessionWithPackage(ctx, tenantID, sessionID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		actions, err = tx.ListActions(ctx, tenantID, sessionID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionStateInvalid, apperr.ErrSimSessionQueryFailed)
		}
		return nil
	}); err != nil {
		return SessionWithPackage{}, nil, err
	}
	return session, actions, nil
}

// reportCheckpointRaw 在租户事务内保存检查点,不保存正确答案或判分规则。
func (s *Service) reportCheckpointRaw(ctx context.Context, tenantID, sessionID int64, sourceRef, checkpointID string, answer []byte, achieved bool) error {
	if err := validateCheckpoint(sessionID, checkpointID, answer); err != nil {
		return err
	}
	return s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		session, err := tx.GetSession(ctx, tenantID, sessionID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		if strings.TrimSpace(sourceRef) != "" && session.SourceRef != strings.TrimSpace(sourceRef) {
			return apperr.ErrServiceUnauthorized
		}
		if !auth.ServiceSourceRefAuthorized(ctx, session.SourceRef) {
			return apperr.ErrServiceUnauthorized
		}
		if !canMutateSession(session.Status) {
			return apperr.ErrSimSessionStateInvalid
		}
		_, err = tx.UpsertCheckpoint(ctx, Checkpoint{ID: s.ids.Generate(), TenantID: tenantID, SessionID: sessionID, CheckpointID: strings.TrimSpace(checkpointID), Answer: answer, Achieved: achieved})
		if err != nil {
			return lookupError(err, apperr.ErrSimCheckpointInvalid, apperr.ErrSimSessionQueryFailed)
		}
		return nil
	})
}

// releaseBackendSessions 释放已归档 compute=backend 会话的 M4 自有适配器资源。
func (s *Service) releaseBackendSessions(ctx context.Context, tenantID int64, sessions []Session) error {
	for _, archived := range sessions {
		if archived.Compute != ComputeBackend {
			continue
		}
		session, err := s.loadBackendReleaseSession(ctx, tenantID, archived.ID)
		if err != nil {
			return err
		}
		adapter := s.backends[strings.TrimSpace(session.BackendAdapter)]
		if err := validateBackendAdapterConfig(session.Compute, session.BackendAdapter, session.BackendConfig, s.backends); err != nil {
			return err
		}
		if err := adapter.Release(ctx, session); err != nil {
			return apperr.ErrSimBackendComputeUnavailable.WithCause(err)
		}
	}
	return nil
}

// loadBackendReleaseSession 读取后端适配器释放资源所需的会话与包配置。
func (s *Service) loadBackendReleaseSession(ctx context.Context, tenantID, sessionID int64) (SessionWithPackage, error) {
	var session SessionWithPackage
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		session, err = tx.GetSessionWithPackage(ctx, tenantID, sessionID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		return nil
	}); err != nil {
		return SessionWithPackage{}, err
	}
	if strings.TrimSpace(session.BackendAdapter) == "" {
		return SessionWithPackage{}, apperr.ErrSimBackendComputeUnavailable
	}
	return session, nil
}
