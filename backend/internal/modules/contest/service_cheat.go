// contest service_cheat 文件实现防作弊线索查询和人工处理记录。
package contest

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
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
		return err
	}); err != nil {
		return CheatRecordDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "contest.cheat.record", auditTargetCheatRecord, item.ID, map[string]any{"contest_id": contestID, "team_id": req.TeamID}); err != nil {
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
