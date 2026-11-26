// sim service_session 文件实现会话创建、操作序列、回放、分享和检查点能力。
package sim

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// CreateSessionFromHTTP 转换内部 HTTP 请求为跨模块契约调用。
func (s *Service) CreateSessionFromHTTP(ctx context.Context, tenantID int64, req CreateSessionRequest) (SimSessionCreateResponse, error) {
	authorizedAccountIDs := make([]int64, 0, len(req.AuthorizedAccountIDs))
	for _, accountID := range req.AuthorizedAccountIDs {
		authorizedAccountIDs = append(authorizedAccountIDs, accountID.Int64())
	}
	info, err := s.CreateSession(ctx, contracts.SimCreateSessionRequest{TenantID: tenantID, PackageCode: req.PackageCode, Version: req.Version, Seed: req.Seed, InitParams: req.InitParams, OwnerAccountID: req.OwnerAccountID.Int64(), AuthorizedAccountIDs: authorizedAccountIDs, SourceRef: req.SourceRef, ScopeRef: req.ScopeRef})
	if err != nil {
		return SimSessionCreateResponse{}, err
	}
	return SimSessionCreateResponse{SessionID: ids.ID(info.SessionID), Compute: info.Compute}, nil
}

// ReportAction 保存用户操作序列,同 seq 同内容幂等,同 seq 不同内容拒绝。
func (s *Service) ReportAction(ctx context.Context, tenantID, accountID, sessionID int64, req ReportActionRequest) (SimActionResponse, error) {
	if err := validateAction(req); err != nil {
		return SimActionResponse{}, err
	}
	req.EventType = strings.TrimSpace(req.EventType)
	if req.Payload == nil {
		req.Payload = map[string]any{}
	}
	var out Action
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.LockSessionActions(ctx, tenantID, sessionID); err != nil {
			return apperr.ErrSimActionSeqInvalid.WithCause(err)
		}
		session, err := tx.GetSessionWithPackage(ctx, tenantID, sessionID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		if !sessionAccountAuthorized(session.Session, accountID) {
			return apperr.ErrForbidden
		}
		if err := s.requireSessionModules(ctx, tenantID, session.SourceRef); err != nil {
			return err
		}
		if session.Status != SessionAvailable {
			return apperr.ErrSimSessionStateInvalid
		}
		if session.Compute == ComputeIsolated {
			return apperr.ErrSimActionSeqInvalid
		}
		if err := validateActionReplayPosition(req.Seq, req.AtTick, session.ScaleLimit); err != nil {
			return err
		}
		if err := validateActionAgainstSchema(session.InteractionSchema, req.EventType, req.Payload); err != nil {
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
		} else if req.Seq != last.Seq+1 || req.AtTick < last.AtTick {
			return apperr.ErrSimActionSeqInvalid
		}
		out, err = tx.CreateAction(ctx, Action{ID: s.ids.Generate(), TenantID: tenantID, SessionID: sessionID, Seq: req.Seq, AtTick: req.AtTick, EventType: req.EventType, Payload: req.Payload})
		if err != nil {
			return apperr.ErrSimActionSeqInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return SimActionResponse{}, err
	}
	return actionToResponse(out), nil
}

// GetReplayForUser 读取当前用户可见的回放。
func (s *Service) GetReplayForUser(ctx context.Context, tenantID, accountID, sessionID int64) (SimReplayResponse, error) {
	session, actions, err := s.loadReplay(ctx, tenantID, sessionID)
	if err != nil {
		return SimReplayResponse{}, err
	}
	if !sessionAccountAuthorized(session.Session, accountID) {
		return SimReplayResponse{}, apperr.ErrForbidden
	}
	if err := s.requireSessionModules(ctx, tenantID, session.SourceRef); err != nil {
		return SimReplayResponse{}, err
	}
	return replayToResponse(session, actions), nil
}

// ShareSession 为用户会话创建公开分享码。
func (s *Service) ShareSession(ctx context.Context, tenantID, accountID, sessionID int64, expireAt time.Time) (SimShareResponse, error) {
	if !expireAt.IsZero() && !expireAt.After(timex.Now()) {
		return SimShareResponse{}, apperr.ErrSimShareCodeInvalid
	}
	var share Share
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.LockSessionActions(ctx, tenantID, sessionID); err != nil {
			return apperr.ErrSimSessionStateInvalid.WithCause(err)
		}
		session, err := tx.GetSession(ctx, tenantID, sessionID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		if !sessionAccountAuthorized(session, accountID) {
			return apperr.ErrForbidden
		}
		if err := s.requireSessionModules(ctx, tenantID, session.SourceRef); err != nil {
			return err
		}
		if session.Status != SessionAvailable {
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
			if !isNoRows(err) {
				return apperr.ErrSimShareCodeInvalid.WithCause(err)
			}
			lastErr = err
		}
		return apperr.ErrSimShareCodeInvalid.WithCause(lastErr)
	}); err != nil {
		return SimShareResponse{}, err
	}
	return SimShareResponse{Code: share.Code, ExpireAt: share.ExpireAt, Status: "active"}, nil
}

// GetSharedReplay 按公开分享码读取可复现剧本,分享索引本身不存剧本正文。
func (s *Service) GetSharedReplay(ctx context.Context, code string) (SimReplayResponse, error) {
	if strings.TrimSpace(code) == "" || len(strings.TrimSpace(code)) > 48 {
		return SimReplayResponse{}, apperr.ErrSimShareCodeInvalid
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
		return SimReplayResponse{}, err
	}
	if !shareUsable(share, timex.Now()) {
		return SimReplayResponse{}, apperr.ErrSimShareCodeInvalid
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
		return SimReplayResponse{}, err
	}
	if err := s.requireSessionModules(ctx, share.TenantID, session.SourceRef); err != nil {
		return SimReplayResponse{}, err
	}
	return replayToPublicResponse(session, actions), nil
}

// requireSessionModules 同时检查仿真与稳定来源模块，匿名分享和业务内嵌会话共用。
func (s *Service) requireSessionModules(ctx context.Context, tenantID int64, sourceRef string) error {
	modules := []string{contracts.TenantModuleSim}
	if source := contracts.TenantModuleFromSourceRef(sourceRef); source != "" && source != contracts.TenantModuleSim {
		modules = append(modules, source)
	}
	return s.identity.RequireTenantModules(ctx, tenantID, modules...)
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

// releaseBackendSessions 释放已归档隔离执行会话占用的 Pod 与命名空间,并让出租户并发额度。
func (s *Service) releaseBackendSessions(ctx context.Context, tenantID int64, sessions []Session) error {
	var failures []error
	for _, archived := range sessions {
		if archived.Compute != ComputeIsolated {
			continue
		}
		if err := s.publishSessionChanged(ctx, tenantID, archived.ID); err != nil {
			failures = append(failures, fmt.Errorf("session_id=%d: %w", archived.ID, err))
		}
		session, err := s.loadBackendReleaseSession(ctx, tenantID, archived.ID)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		adapter := s.backends[strings.TrimSpace(session.BackendAdapter)]
		if err := validateBackendAdapterConfig(session.Compute, session.BackendAdapter, session.BackendConfig, s.backends); err != nil {
			failures = append(failures, fmt.Errorf("session_id=%d: %w", session.ID, err))
			continue
		}
		if err := adapter.Release(ctx, session); err != nil {
			failures = append(failures, fmt.Errorf("session_id=%d: %w", session.ID, apperr.ErrSimBackendComputeUnavailable.WithCause(err)))
		}
	}
	return errors.Join(failures...)
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
