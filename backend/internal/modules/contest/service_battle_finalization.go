// contest service_battle_finalization 可靠交付对局终态后的回收、推送和审计。
package contest

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
)

// finalizeBattleMatch 仅确认外部交付进度,失败或崩溃后按租约恢复,不重新结算积分。
func (s *Service) finalizeBattleMatch(ctx context.Context, match BattleMatch) error {
	if match.Status != BattleMatchStatusDone && match.Status != BattleMatchStatusFailed {
		return apperr.ErrContestBattleMatchFailed
	}
	duration := time.Duration(s.cfg.MatchmakerLeaseDurationMs) * time.Millisecond
	if duration <= 0 {
		return apperr.ErrContestBattleMatchFailed
	}
	token, err := pkgcrypto.RandomToken(48)
	if err != nil {
		return apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	until := timex.Now().Add(duration)
	var claimed bool
	if err := s.store.TenantTx(ctx, match.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		claimed, err = tx.ClaimBattleMatchFinalization(ctx, match.TenantID, match.ID, token, until)
		return err
	}); err != nil {
		return apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	if !claimed {
		return nil
	}
	// 所有外部动作共享租约截止时间;失效后由下一交付者接续。
	deliveryCtx, cancel := context.WithDeadline(ctx, until)
	defer cancel()
	reason := "battle_failed"
	if match.Status == BattleMatchStatusDone {
		reason = "battle_finished"
	}
	if err := s.sandbox.RecycleByScopeRef(deliveryCtx, contracts.SandboxRecycleRequest{TenantID: match.TenantID, ScopeRef: match.ScopeRef, SourceRef: match.SourceRef, Reason: reason}); err != nil {
		return apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	if match.Status == BattleMatchStatusDone {
		if err := s.pushLeaderboard(deliveryCtx, match.TenantID, match.ContestID); err != nil {
			return err
		}
		if err := s.writeAudit(deliveryCtx, match.TenantID, 0, audit.ActorRoleSystem, "contest.battle.finish", auditTargetBattleMatch, match.ID, battleScoreDeltaAuditDetail(match.ScoreDelta)); err != nil {
			return err
		}
	}
	var completed bool
	if err := s.store.TenantTx(deliveryCtx, match.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		completed, err = tx.CompleteBattleMatchFinalization(ctx, match.TenantID, match.ID, token)
		return err
	}); err != nil {
		return apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	if !completed {
		return apperr.ErrContestBattleMatchFailed.WithCause(fmt.Errorf("对局 %d 终态交付租约已失效", match.ID))
	}
	return nil
}
