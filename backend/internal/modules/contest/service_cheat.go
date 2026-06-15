// contest service_cheat 文件实现防作弊线索查询和人工处理记录。
package contest

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

// CreateCheatRecord 创建教师确认后的违规处理记录。
func (s *Service) CreateCheatRecord(ctx context.Context, contestID int64, req CheatRecordRequest) (CheatRecordDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CheatRecordDTO{}, err
	}
	req, err = validateCheatRequest(req)
	if err != nil {
		return CheatRecordDTO{}, err
	}
	item := CheatRecord{ID: s.ids.Generate(), TenantID: id.TenantID, ContestID: contestID, TeamID: req.TeamID, Type: req.Type, Evidence: req.Evidence, Action: req.Action, OperatorID: id.AccountID}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := s.loadContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID); err != nil {
			return err
		}
		team, err := tx.GetTeam(ctx, id.TenantID, req.TeamID)
		if err != nil {
			return err
		}
		if team.ContestID != contestID {
			return validateCheatTeamError()
		}
		item, err = tx.CreateCheatRecord(ctx, item)
		if err != nil {
			return err
		}
		if item.Action == CheatActionPenalty {
			if err := s.applyCheatPenalty(ctx, tx, id.TenantID, contestID, item.TeamID, float64FromMap(item.Evidence, "penalty_score", 0)); err != nil {
				return err
			}
		}
		if item.Action == CheatActionDisqualify {
			return tx.RefreshContestRanks(ctx, id.TenantID, contestID)
		}
		return err
	}); err != nil {
		return CheatRecordDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.cheat.record", auditTargetCheatRecord, item.ID, map[string]any{"contest_id": contestID, "team_id": req.TeamID}); err != nil {
		return CheatRecordDTO{}, err
	}
	return cheatDTOFromModel(item), nil
}

// ListCheatRecords 查询教师可见的违规处理记录。
func (s *Service) ListCheatRecords(ctx context.Context, contestID int64, page, size int) ([]CheatRecordDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var items []CheatRecord
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := s.loadContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID); err != nil {
			return err
		}
		items, err = tx.ListCheatRecords(ctx, id.TenantID, contestID, page, size)
		return err
	}); err != nil {
		return nil, err
	}
	out := make([]CheatRecordDTO, 0, len(items))
	for _, item := range items {
		out = append(out, cheatDTOFromModel(item))
	}
	return out, nil
}

// ListCheatSuspects 按 M3 指纹服务读取疑似相似提交线索。
func (s *Service) ListCheatSuspects(ctx context.Context, contestID, problemID int64, codeHash, excludeSourceRef string, threshold float64) ([]CheatSuspectDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	codeHash = strings.TrimSpace(codeHash)
	excludeSourceRef = strings.TrimSpace(excludeSourceRef)
	var problem ContestProblem
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := s.loadContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID); err != nil {
			return err
		}
		problem, err = tx.GetContestProblem(ctx, id.TenantID, problemID)
		if err != nil {
			return err
		}
		if problem.ContestID != contestID {
			return validateCheatTeamError()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	matches, err := s.fingerprint.FindSimilarity(ctx, contracts.FingerprintSimilarityRequest{TenantID: id.TenantID, ProblemRef: fmt.Sprintf("%s:%s", problem.ItemCode, problem.ItemVersion), CodeHash: codeHash, ExcludeSourceRef: excludeSourceRef, Threshold: threshold})
	if err != nil {
		return nil, err
	}
	out := make([]CheatSuspectDTO, 0, len(matches))
	for _, item := range matches {
		out = append(out, CheatSuspectDTO{SourceRef: item.SourceRef, SubmitterID: item.SubmitterID, Score: item.Score, CodeHash: item.CodeHash})
	}
	return out, nil
}

// validateCheatTeamError 保持违规上下文校验错误码统一。
func validateCheatTeamError() error {
	return apperr.ErrContestCheatInvalid
}

// applyCheatPenalty 将人工确认的扣分处罚写入排行榜投影。
func (s *Service) applyCheatPenalty(ctx context.Context, tx TxStore, tenantID, contestID, teamID int64, penalty float64) error {
	if penalty <= 0 {
		return apperr.ErrContestCheatInvalid
	}
	rank, err := tx.GetLadderByTeam(ctx, tenantID, contestID, teamID)
	if err != nil {
		if isNoRows(err) {
			rank = LadderRank{ID: s.ids.Generate(), TenantID: tenantID, ContestID: contestID, TeamID: teamID}
		} else {
			return err
		}
	}
	rank.ID = s.ids.Generate()
	rank.Score -= penalty
	if rank.Score < 0 {
		rank.Score = 0
	}
	if _, err := tx.UpsertLadder(ctx, rank); err != nil {
		return err
	}
	return tx.RefreshContestRanks(ctx, tenantID, contestID)
}
