// sim service_contract 文件实现 M4 对业务模块开放的会话、回放与检查点契约。
package sim

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
)

// CreateSession 创建仿真会话并锁定仿真包版本。
func (s *Service) CreateSession(ctx context.Context, req contracts.SimCreateSessionRequest) (contracts.SimSessionInfo, error) {
	req.SourceRef = strings.TrimSpace(req.SourceRef)
	req.ScopeRef = strings.TrimSpace(req.ScopeRef)
	initParams := req.InitParams
	if initParams == nil {
		initParams = map[string]any{}
	}
	create := CreateSessionRequest{PackageCode: req.PackageCode, Version: req.Version, Seed: req.Seed, InitParams: initParams, OwnerAccountID: ids.ID(req.OwnerAccountID), SourceRef: req.SourceRef, ScopeRef: req.ScopeRef}
	if err := validateCreateSession(create, req.TenantID); err != nil {
		return contracts.SimSessionInfo{}, err
	}
	if !auth.ServiceSourceRefAuthorized(ctx, req.SourceRef) {
		return contracts.SimSessionInfo{}, apperr.ErrServiceUnauthorized
	}
	sharedAccountIDs, err := normalizeSessionSharedAccountIDs(req.OwnerAccountID, req.AuthorizedAccountIDs)
	if err != nil {
		return contracts.SimSessionInfo{}, err
	}
	if err := s.validateSessionSharedAccounts(ctx, req.TenantID, sharedAccountIDs); err != nil {
		return contracts.SimSessionInfo{}, err
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
	session := Session{ID: s.ids.Generate(), TenantID: req.TenantID, PackageID: pkg.ID, SourceRef: req.SourceRef, ScopeRef: req.ScopeRef, OwnerAccountID: req.OwnerAccountID, SharedAccountIDs: sharedAccountIDs, Seed: req.Seed, InitParams: initParams, Compute: pkg.Compute, Status: SessionCreating}
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		// 隔离会话的并发闸门与建行必须处于同一事务,避免并发请求同时绕过配额。
		if pkg.Compute == ComputeIsolated {
			active, err := tx.CountActiveIsolatedSessions(ctx, req.TenantID)
			if err != nil {
				return apperr.ErrSimSessionQueryFailed.WithCause(err)
			}
			if active >= int64(s.maxIsolatedSessionsPerTenant) {
				return apperr.ErrSimBackendComputeUnavailable.WithCause(fmt.Errorf("租户 %d 隔离执行会话已达上限 %d", req.TenantID, s.maxIsolatedSessionsPerTenant))
			}
		}
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

// validateSessionSharedAccounts 确认共享账号真实存在且全部属于目标租户,弥补数组字段无法声明逐元素外键的限制。
func (s *Service) validateSessionSharedAccounts(ctx context.Context, tenantID int64, accountIDs []int64) error {
	if len(accountIDs) == 0 {
		return nil
	}
	accounts, err := s.identity.BatchGetAccounts(ctx, accountIDs)
	if err != nil {
		return apperr.ErrSimSessionInvalid.WithCause(err)
	}
	want := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		want[accountID] = struct{}{}
	}
	for _, account := range accounts {
		if account.TenantID != tenantID {
			return apperr.ErrSimSessionInvalid
		}
		delete(want, account.AccountID)
	}
	if len(want) != 0 {
		return apperr.ErrSimSessionInvalid
	}
	return nil
}

// GetReplay 返回可复现的 seed、参数与操作序列。
func (s *Service) GetReplay(ctx context.Context, tenantID, sessionID int64) (contracts.SimReplayInfo, error) {
	session, actions, err := s.loadReplay(ctx, tenantID, sessionID)
	if err != nil {
		return contracts.SimReplayInfo{}, err
	}
	return replayToContract(session, actions), nil
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

// UpdateSessionAuthorizedAccounts 原子替换仿真会话共享账号,并精确关闭已撤销账号的在线流。
func (s *Service) UpdateSessionAuthorizedAccounts(ctx context.Context, req contracts.SimAuthorizedAccountsRequest) error {
	req.SourceRef = strings.TrimSpace(req.SourceRef)
	if req.TenantID <= 0 || req.SessionID <= 0 || !auth.ValidSourceRef(req.SourceRef) {
		return apperr.ErrSimSessionInvalid
	}
	if !auth.ServiceSourceRefAuthorized(ctx, req.SourceRef) {
		return apperr.ErrServiceUnauthorized
	}
	var revoked []int64
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		session, err := tx.GetSession(ctx, req.TenantID, req.SessionID)
		if err != nil {
			return lookupError(err, apperr.ErrSimSessionNotFound, apperr.ErrSimSessionQueryFailed)
		}
		if session.SourceRef != req.SourceRef {
			return apperr.ErrServiceUnauthorized
		}
		if !canMutateSession(session.Status) {
			return apperr.ErrSimSessionStateInvalid
		}
		if err := s.validateSessionSharedAccounts(ctx, req.TenantID, req.AuthorizedAccountIDs); err != nil {
			return err
		}
		accountIDs, err := normalizeSessionSharedAccountIDs(session.OwnerAccountID, req.AuthorizedAccountIDs)
		if err != nil {
			return err
		}
		previous := make(map[int64]struct{}, len(session.SharedAccountIDs)+1)
		previous[session.OwnerAccountID] = struct{}{}
		for _, accountID := range session.SharedAccountIDs {
			previous[accountID] = struct{}{}
		}
		next := make(map[int64]struct{}, len(accountIDs)+1)
		next[session.OwnerAccountID] = struct{}{}
		for _, accountID := range accountIDs {
			next[accountID] = struct{}{}
		}
		for accountID := range previous {
			if _, ok := next[accountID]; !ok {
				revoked = append(revoked, accountID)
			}
		}
		_, err = tx.UpdateSessionAuthorizedAccounts(ctx, req.TenantID, req.SessionID, req.SourceRef, accountIDs)
		if err != nil {
			return apperr.ErrSimSessionStateInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	if s.wsHub != nil {
		for _, accountID := range revoked {
			if err := s.wsHub.CloseSession(ws.SessionKey{TenantID: req.TenantID, AccountID: accountID, Scope: simSessionSessionScope(req.SessionID)}); err != nil {
				return apperr.ErrSimBackendComputeUnavailable.WithCause(err)
			}
		}
	}
	return nil
}

// RecycleByScopeRef 按生命周期作用域归档仿真会话并释放后端计算资源。
func (s *Service) RecycleByScopeRef(ctx context.Context, req contracts.SimRecycleRequest) error {
	req.SourceRef = strings.TrimSpace(req.SourceRef)
	req.ScopeRef = strings.TrimSpace(req.ScopeRef)
	if req.TenantID <= 0 || !auth.ValidSourceRef(req.SourceRef) || !auth.ValidScopeRef(req.ScopeRef) {
		return apperr.ErrSimSessionInvalid
	}
	if !auth.ServiceSourceRefAuthorized(ctx, req.SourceRef) {
		return apperr.ErrServiceUnauthorized
	}
	var archived []Session
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		archived, err = tx.ArchiveSessionsByScopeRef(ctx, req.TenantID, req.ScopeRef, req.SourceRef)
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
