// contest convert 文件负责 M8 领域模型与 HTTP/contract DTO 的纯转换。
package contest

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/secretmap"
	"chaimir/pkg/apperr"
)

// contestDTOFromModel 转换竞赛定义为 HTTP 输出。
func contestDTOFromModel(item Contest) ContestDTO {
	return ContestDTO{ID: ids.ID(item.ID), OrganizerID: ids.ID(item.OrganizerID), Name: item.Name, Mode: item.Mode, MatchMode: item.MatchMode, TeamMode: item.TeamMode, SignupStart: item.SignupStart, SignupEnd: item.SignupEnd, StartAt: item.StartAt, EndAt: item.EndAt, FreezeMinutes: item.FreezeMinutes, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

// problemDTOFromModel 转换竞赛题引用为 HTTP 输出。
func problemDTOFromModel(item ContestProblem) ProblemDTO {
	return ProblemDTO{ID: ids.ID(item.ID), ContestID: ids.ID(item.ContestID), ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, DynamicScore: item.DynamicScore, BattleConfig: item.BattleConfig, BattleRule: item.BattleRule, Seq: item.Seq}
}

// teamDTOFromModel 转换队伍及成员为 HTTP 输出。
func teamDTOFromModel(item Team) TeamDTO {
	out := TeamDTO{ID: ids.ID(item.ID), ContestID: ids.ID(item.ContestID), Name: item.Name, InviteCode: item.InviteCode, Status: item.Status, CreatedAt: item.CreatedAt}
	out.Members = make([]TeamMemberDTO, 0, len(item.Members))
	for _, member := range item.Members {
		out.Members = append(out.Members, TeamMemberDTO{ID: ids.ID(member.ID), TeamID: ids.ID(member.TeamID), AccountID: ids.ID(member.AccountID), MemberTenantID: ids.ID(member.MemberTenantID), IsLeader: member.IsLeader, JoinedAt: member.JoinedAt})
	}
	return out
}

// submissionDTOFromModel 转换解题提交为 HTTP 输出。
func submissionDTOFromModel(item SolveSubmission) SubmissionDTO {
	return SubmissionDTO{ID: ids.ID(item.ID), ContestID: ids.ID(item.ContestID), ProblemID: ids.ID(item.ProblemID), TeamID: ids.ID(item.TeamID), SubmitterID: ids.ID(item.SubmitterID), ContentRef: item.ContentRef, SourceRef: item.SourceRef, JudgeTaskRef: item.JudgeTaskRef, Passed: item.Passed, Score: item.Score, SandboxRef: item.SandboxRef, SubmittedAt: item.SubmittedAt}
}

// battleEntryDTOFromModel 转换对抗参战物为 HTTP 输出。
func battleEntryDTOFromModel(item BattleEntry) BattleEntryDTO {
	return BattleEntryDTO{ID: ids.ID(item.ID), ContestID: ids.ID(item.ContestID), ProblemID: ids.ID(item.ProblemID), TeamID: ids.ID(item.TeamID), Role: item.Role, ArtifactRef: item.ArtifactRef, CodeHash: item.ArtifactHash, VersionNo: item.VersionNo, IsActive: item.IsActive, SubmittedAt: item.SubmittedAt}
}

// battleMatchDTOFromModel 转换对局为 HTTP 输出。
func battleMatchDTOFromModel(item BattleMatch) BattleMatchDTO {
	return BattleMatchDTO{ID: ids.ID(item.ID), ContestID: ids.ID(item.ContestID), ProblemID: ids.ID(item.ProblemID), EntryAID: ids.ID(item.EntryAID), EntryBID: ids.ID(item.EntryBID), SourceRef: item.SourceRef, SandboxRef: item.SandboxRef, JudgeTaskRef: item.JudgeTaskRef, Result: item.Result, ScoreDelta: item.ScoreDelta, ReplayAvailable: len(item.Replay) > 0, Status: item.Status, MatchedAt: item.MatchedAt, FinishedAt: item.FinishedAt}
}

// ladderDTOFromModel 转换排行投影为 HTTP 输出。
func ladderDTOFromModel(item LadderRank) LadderDTO {
	return LadderDTO{TeamID: ids.ID(item.TeamID), TeamName: item.TeamName, Score: item.Score, SolvedCount: item.SolvedCount, LastSolveAt: item.LastSolveAt, Rank: item.Rank, UpdatedAt: item.UpdatedAt}
}

// resultSnapshotDTOFromModel 严格解析归档榜单并转换为最终结果输出。
func resultSnapshotDTOFromModel(item LadderSnapshot) (ResultSnapshotDTO, error) {
	ranking, err := ladderDTOsFromSnapshot(item)
	if err != nil {
		return ResultSnapshotDTO{}, err
	}
	return ResultSnapshotDTO{ID: ids.ID(item.ID), TenantID: ids.ID(item.TenantID), ContestID: ids.ID(item.ContestID), FinalRanking: ranking, GeneratedAt: item.GeneratedAt}, nil
}

// cheatDTOFromModel 转换违规记录为 HTTP 输出。
func cheatDTOFromModel(item CheatRecord) CheatRecordDTO {
	return CheatRecordDTO{ID: ids.ID(item.ID), ContestID: ids.ID(item.ContestID), TeamID: ids.ID(item.TeamID), Type: item.Type, Evidence: item.Evidence, Action: item.Action, OperatorID: ids.ID(item.OperatorID), CreatedAt: item.CreatedAt}
}

// vulnSourceDTOFromModel 转换漏洞源为 HTTP 输出。
func vulnSourceDTOFromModel(item VulnSource) (VulnSourceDTO, error) {
	raw, err := jsonx.AnyBytes(secretmap.Mask(item.Config), apperr.ErrContestVulnSourceInvalid)
	if err != nil {
		return VulnSourceDTO{}, err
	}
	var config VulnSourceConfig
	if err := jsonx.DecodeStrictKnownFields(raw, &config); err != nil {
		return VulnSourceDTO{}, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	return VulnSourceDTO{ID: ids.ID(item.ID), Type: item.Type, Name: item.Name, Config: config, DefaultLevel: item.DefaultLevel, Enabled: item.Enabled, LastSyncAt: item.LastSyncAt}, nil
}

// vulnProblemDTOFromModel 转换漏洞题草稿为 HTTP 输出。
func vulnProblemDTOFromModel(item VulnProblem) VulnProblemDTO {
	return VulnProblemDTO{ID: ids.ID(item.ID), SourceID: ids.ID(item.SourceID), ExternalRef: item.ExternalRef, Title: item.Title, Level: item.Level, RuntimeMode: item.RuntimeMode, DraftBody: item.DraftBody, PrevalidateStatus: item.PrevalidateStatus, PrevalidateDetail: item.PrevalidateDetail, ContentItemCode: item.ContentItemCode, ContentItemVersion: item.ContentItemVersion, Status: item.Status}
}

// contestAchievementFromRecord 转换个人战绩为跨模块竞赛成就契约。
func contestAchievementFromRecord(tenantID, studentID int64, item StudentContestRecord) contracts.ContestAchievement {
	return contracts.ContestAchievement{TenantID: tenantID, StudentID: studentID, ContestID: item.ContestID, TeamID: item.TeamID, Score: item.Score, Rank: item.Rank}
}
