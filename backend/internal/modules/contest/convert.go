// contest convert 文件负责 M8 领域模型与 HTTP/contract DTO 的纯转换。
package contest

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/secretmap"
)

// contestDTOFromModel 转换竞赛定义为 HTTP 输出。
func contestDTOFromModel(item Contest) ContestDTO {
	return ContestDTO{ID: item.ID, OrganizerID: item.OrganizerID, Name: item.Name, Mode: item.Mode, MatchMode: item.MatchMode, TeamMode: item.TeamMode, SignupStart: item.SignupStart, SignupEnd: item.SignupEnd, StartAt: item.StartAt, EndAt: item.EndAt, FreezeMinutes: item.FreezeMinutes, Rules: item.Rules, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

// problemDTOFromModel 转换竞赛题引用为 HTTP 输出。
func problemDTOFromModel(item ContestProblem) ProblemDTO {
	return ProblemDTO{ID: item.ID, ContestID: item.ContestID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, DynamicScore: item.DynamicScore, BattleRule: item.BattleRule, Seq: item.Seq}
}

// teamDTOFromModel 转换队伍及成员为 HTTP 输出。
func teamDTOFromModel(item Team) TeamDTO {
	out := TeamDTO{ID: item.ID, ContestID: item.ContestID, Name: item.Name, InviteCode: item.InviteCode, Status: item.Status, CreatedAt: item.CreatedAt}
	out.Members = make([]TeamMemberDTO, 0, len(item.Members))
	for _, member := range item.Members {
		out.Members = append(out.Members, TeamMemberDTO{ID: member.ID, TeamID: member.TeamID, AccountID: member.AccountID, MemberTenantID: member.MemberTenantID, IsLeader: member.IsLeader, JoinedAt: member.JoinedAt})
	}
	return out
}

// submissionDTOFromModel 转换解题提交为 HTTP 输出。
func submissionDTOFromModel(item SolveSubmission) SubmissionDTO {
	return SubmissionDTO{ID: item.ID, ContestID: item.ContestID, ProblemID: item.ProblemID, TeamID: item.TeamID, SubmitterID: item.SubmitterID, ContentRef: item.ContentRef, SourceRef: item.SourceRef, JudgeTaskRef: item.JudgeTaskRef, Passed: item.Passed, Score: item.Score, SandboxRef: item.SandboxRef, SubmittedAt: item.SubmittedAt}
}

// battleEntryDTOFromModel 转换对抗参战物为 HTTP 输出。
func battleEntryDTOFromModel(item BattleEntry) BattleEntryDTO {
	return BattleEntryDTO{ID: item.ID, ContestID: item.ContestID, ProblemID: item.ProblemID, TeamID: item.TeamID, Role: item.Role, ArtifactRef: item.ArtifactRef, CodeHash: item.ArtifactHash, VersionNo: item.VersionNo, IsActive: item.IsActive, SubmittedAt: item.SubmittedAt}
}

// battleMatchDTOFromModel 转换对局为 HTTP 输出。
func battleMatchDTOFromModel(item BattleMatch) BattleMatchDTO {
	return BattleMatchDTO{ID: item.ID, ContestID: item.ContestID, ProblemID: item.ProblemID, EntryAID: item.EntryAID, EntryBID: item.EntryBID, SourceRef: item.SourceRef, SandboxRef: item.SandboxRef, JudgeTaskRef: item.JudgeTaskRef, Result: item.Result, ScoreDelta: item.ScoreDelta, ReplayRef: item.ReplayRef, Status: item.Status, MatchedAt: item.MatchedAt, FinishedAt: item.FinishedAt}
}

// ladderDTOFromModel 转换排行投影为 HTTP 输出。
func ladderDTOFromModel(item LadderRank) LadderDTO {
	return LadderDTO{TeamID: item.TeamID, Score: item.Score, SolvedCount: item.SolvedCount, LastSolveAt: item.LastSolveAt, Rank: item.Rank, UpdatedAt: item.UpdatedAt}
}

// cheatDTOFromModel 转换违规记录为 HTTP 输出。
func cheatDTOFromModel(item CheatRecord) CheatRecordDTO {
	return CheatRecordDTO{ID: item.ID, ContestID: item.ContestID, TeamID: item.TeamID, Type: item.Type, Evidence: item.Evidence, Action: item.Action, OperatorID: item.OperatorID, CreatedAt: item.CreatedAt}
}

// vulnSourceDTOFromModel 转换漏洞源为 HTTP 输出。
func vulnSourceDTOFromModel(item VulnSource) VulnSourceDTO {
	return VulnSourceDTO{ID: item.ID, Type: item.Type, Name: item.Name, Config: secretmap.Mask(item.Config), DefaultLevel: item.DefaultLevel, Enabled: item.Enabled, LastSyncAt: item.LastSyncAt}
}

// vulnProblemDTOFromModel 转换漏洞题草稿为 HTTP 输出。
func vulnProblemDTOFromModel(item VulnProblem) VulnProblemDTO {
	return VulnProblemDTO{ID: item.ID, SourceID: item.SourceID, ExternalRef: item.ExternalRef, Title: item.Title, Level: item.Level, RuntimeMode: item.RuntimeMode, DraftBody: item.DraftBody, PrevalidateStatus: item.PrevalidateStatus, PrevalidateDetail: item.PrevalidateDetail, ContentItemCode: item.ContentItemCode, ContentItemVersion: item.ContentItemVersion, Status: item.Status}
}

// contestAchievementFromRecord 转换个人战绩为跨模块竞赛成就契约。
func contestAchievementFromRecord(tenantID, studentID int64, item StudentContestRecord) contracts.ContestAchievement {
	return contracts.ContestAchievement{TenantID: tenantID, StudentID: studentID, ContestID: item.ContestID, TeamID: item.TeamID, Score: item.Score, Rank: item.Rank}
}
