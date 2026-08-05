// contest service_battle_replay 文件负责把 M3 产出的链轨迹固化为 M8 回放归档。
package contest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

const battleReplayArchiveVersion = 1

type battleReplayArchive struct {
	Version      int                           `json:"version"`
	MatchID      string                        `json:"match_id"`
	TaskID       string                        `json:"task_id"`
	SourceRef    string                        `json:"source_ref"`
	InitialState battleReplayInitialState      `json:"initial_state"`
	Actions      []contracts.JudgeReplayAction `json:"actions"`
	Result       contracts.JudgeTaskResult     `json:"result"`
	FinishedAt   time.Time                     `json:"finished_at"`
}

type battleReplayInitialState struct {
	ContestID  int64                  `json:"contest_id"`
	ProblemID  int64                  `json:"problem_id"`
	BattleRule int16                  `json:"battle_rule"`
	EntryA     battleReplayEntryState `json:"entry_a"`
	EntryB     battleReplayEntryState `json:"entry_b"`
}

type battleReplayEntryState struct {
	Role         int16  `json:"role"`
	VersionNo    int32  `json:"version_no"`
	ArtifactHash string `json:"artifact_hash"`
}

// prepareBattleReplay 校验 M3 已生成完整轨迹并把回放写入统一对象存储。
func (s *Service) prepareBattleReplay(ctx context.Context, tenantID, taskID int64, result contracts.JudgeTaskResult, sourceRef string) (string, bool, error) {
	var match BattleMatch
	var problem ContestProblem
	var entryA, entryB BattleEntry
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		match, err = tx.GetBattleMatchByJudgeTask(ctx, tenantID, ids.Format(taskID))
		if err != nil {
			return err
		}
		if match.SourceRef != sourceRef {
			return apperr.ErrContestEventSourceMismatch
		}
		if match.Status == BattleMatchStatusDone && strings.TrimSpace(match.ReplayRef) != "" {
			return nil
		}
		if match.Status != BattleMatchStatusRunning {
			return apperr.ErrContestBattleMatchFailed
		}
		problem, err = tx.GetContestProblem(ctx, tenantID, match.ProblemID)
		if err != nil {
			return err
		}
		entryA, err = tx.GetBattleEntry(ctx, tenantID, match.EntryAID)
		if err != nil {
			return err
		}
		entryB, err = tx.GetBattleEntry(ctx, tenantID, match.EntryBID)
		return err
	}); err != nil {
		return "", false, err
	}
	if match.Status == BattleMatchStatusDone && strings.TrimSpace(match.ReplayRef) != "" {
		return match.ReplayRef, true, nil
	}
	if len(result.Replay.Actions) == 0 {
		return "", false, apperr.ErrContestReplayUnavailable
	}
	archive := battleReplayArchive{
		Version:   battleReplayArchiveVersion,
		MatchID:   ids.Format(match.ID),
		TaskID:    ids.Format(taskID),
		SourceRef: sourceRef,
		InitialState: battleReplayInitialState{
			ContestID:  match.ContestID,
			ProblemID:  match.ProblemID,
			BattleRule: problem.BattleRule,
			EntryA:     battleReplayEntryState{Role: entryA.Role, VersionNo: entryA.VersionNo, ArtifactHash: entryA.ArtifactHash},
			EntryB:     battleReplayEntryState{Role: entryB.Role, VersionNo: entryB.VersionNo, ArtifactHash: entryB.ArtifactHash},
		},
		Actions:    result.Replay.Actions,
		Result:     result,
		FinishedAt: timex.Now(),
	}
	raw, err := json.Marshal(archive)
	if err != nil {
		return "", false, apperr.ErrContestReplayUnavailable.WithCause(err)
	}
	archiveID := ids.Format(s.ids.Generate())
	key, err := storage.ObjectKey(tenantID, "contest", "replay", ids.Format(match.ID), ids.Format(taskID)+"-"+archiveID+".json")
	if err != nil {
		return "", false, apperr.ErrContestReplayUnavailable.WithCause(err)
	}
	if err := s.replayStore.Put(ctx, s.replayBucket, key, bytes.NewReader(raw), int64(len(raw)), "application/json"); err != nil {
		return "", false, apperr.ErrContestReplayUnavailable.WithCause(err)
	}
	objectRef, err := storage.ObjectRefString(s.replayBucket, key)
	if err != nil {
		if cleanupErr := s.replayStore.Delete(ctx, s.replayBucket, key); cleanupErr != nil {
			return "", false, apperr.ErrContestReplayUnavailable.WithCause(fmt.Errorf("生成回放引用失败: %w; 清理对象失败: %v", err, cleanupErr))
		}
		return "", false, apperr.ErrContestReplayUnavailable.WithCause(err)
	}
	return objectRef, false, nil
}

// deleteBattleReplay 清理事务失败后未被 battle_match 引用的对象。
func (s *Service) deleteBattleReplay(ctx context.Context, objectRef string) error {
	ref, err := storage.ParseObjectRef(objectRef)
	if err != nil {
		return err
	}
	return s.replayStore.Delete(ctx, ref.Bucket, ref.Key)
}
