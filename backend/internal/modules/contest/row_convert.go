// M8 行转换层:集中处理 sqlc 行到领域投影和响应 DTO 的纯转换。
package contest

import (
	"chaimir/internal/modules/contest/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// contestDTOFromRow 转换竞赛定义行。
func contestDTOFromRow(row sqlcgen.Contest) ContestDTO {
	return ContestDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), OrganizerID: ids.Format(row.OrganizerID),
		Name: row.Name, Mode: row.Mode, MatchMode: pgtypex.Int2Value(row.MatchMode), TeamMode: row.TeamMode,
		SignupStart: timex.FromTimestamptz(row.SignupStart), SignupEnd: timex.FromTimestamptz(row.SignupEnd), StartAt: timex.FromTimestamptz(row.StartAt),
		EndAt: timex.FromTimestamptz(row.EndAt), FreezeMinutes: row.FreezeMinutes, Rules: jsonx.ObjectMap(row.Rules),
		Status: row.Status, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt),
	}
}

// contestProblemDTOFromRow 转换竞赛题目引用行。
func contestProblemDTOFromRow(row sqlcgen.ContestProblem) ContestProblemDTO {
	return ContestProblemDTO{
		ID: ids.Format(row.ID), ContestID: ids.Format(row.ContestID), ItemCode: row.ItemCode, ItemVersion: row.ItemVersion,
		Score: row.Score, DynamicScore: jsonx.ObjectMap(row.DynamicScore), BattleRule: pgtypex.Int2Value(row.BattleRule), Seq: row.Seq,
	}
}

// teamDTOFromRows 转换队伍与成员行。
func teamDTOFromRows(row sqlcgen.Team, members []sqlcgen.TeamMember) TeamDTO {
	out := TeamDTO{ID: ids.Format(row.ID), ContestID: ids.Format(row.ContestID), Name: row.Name, InviteCode: pgtypex.TextValue(row.InviteCode), Status: row.Status, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
	out.Members = make([]TeamMemberDTO, 0, len(members))
	for _, member := range members {
		out.Members = append(out.Members, teamMemberDTOFromRow(member))
	}
	return out
}

// teamMemberDTOFromRow 转换队员行。
func teamMemberDTOFromRow(row sqlcgen.TeamMember) TeamMemberDTO {
	return TeamMemberDTO{ID: ids.Format(row.ID), TeamID: ids.Format(row.TeamID), AccountID: ids.Format(row.AccountID), MemberTenantID: ids.Format(row.MemberTenantID), IsLeader: row.IsLeader}
}

// solveSubmissionDTOFromRow 转换解题提交行。
func solveSubmissionDTOFromRow(row sqlcgen.SolveSubmission) SolveSubmissionDTO {
	return SolveSubmissionDTO{
		ID: ids.Format(row.ID), ContestID: ids.Format(row.ContestID), ProblemID: ids.Format(row.ProblemID), TeamID: ids.Format(row.TeamID),
		SubmitterID: ids.Format(row.SubmitterID), ContentRef: jsonx.ObjectMap(row.ContentRef), SourceRef: row.SourceRef, JudgeTaskRef: pgtypex.TextValue(row.JudgeTaskRef),
		Passed: row.Passed, Score: row.Score, SandboxRef: pgtypex.TextValue(row.SandboxRef), SubmittedAt: timex.FromTimestamptz(row.SubmittedAt),
	}
}

// battleEntryDTOFromRow 转换对抗参战物行。
func battleEntryDTOFromRow(row sqlcgen.BattleEntry) BattleEntryDTO {
	return BattleEntryDTO{
		ID: ids.Format(row.ID), ContestID: ids.Format(row.ContestID), TeamID: ids.Format(row.TeamID), Role: row.Role,
		ArtifactRef: row.ArtifactRef, VersionNo: row.VersionNo, IsActive: row.IsActive, SubmittedAt: timex.FromTimestamptz(row.SubmittedAt),
	}
}

// battleMatchDTOFromRow 转换对抗对局行。
func battleMatchDTOFromRow(row sqlcgen.BattleMatch) BattleMatchDTO {
	return BattleMatchDTO{
		ID: ids.Format(row.ID), ContestID: ids.Format(row.ContestID), EntryAID: ids.Format(row.EntryAID), EntryBID: ids.Format(row.EntryBID),
		SandboxRef: row.SandboxRef, Result: row.Result, ScoreDelta: jsonx.ObjectMap(row.ScoreDelta), ReplayRef: row.ReplayRef,
		MatchedAt: timex.FromTimestamptz(row.MatchedAt), FinishedAt: timex.FromTimestamptz(row.FinishedAt),
	}
}

// ladderRankDTOFromRow 转换排行榜行。
func ladderRankDTOFromRow(row sqlcgen.LadderRank) LadderRankDTO {
	return LadderRankDTO{ID: ids.Format(row.ID), ContestID: ids.Format(row.ContestID), TeamID: ids.Format(row.TeamID), Score: pgtypex.NumericValue(row.Score), SolvedCount: row.SolvedCount, Rank: row.Rank, UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// snapshotDTOFromRow 转换成绩快照行。
func snapshotDTOFromRow(row sqlcgen.ContestResultSnapshot) ResultSnapshotDTO {
	return ResultSnapshotDTO{ID: ids.Format(row.ID), ContestID: ids.Format(row.ContestID), FinalRanking: ladderRanksValue(row.FinalRanking), GeneratedAt: timex.FromTimestamptz(row.GeneratedAt)}
}

// cheatRecordDTOFromRow 转换作弊记录行。
func cheatRecordDTOFromRow(row sqlcgen.CheatRecord) CheatRecordDTO {
	return CheatRecordDTO{ID: ids.Format(row.ID), ContestID: ids.Format(row.ContestID), TeamID: ids.Format(row.TeamID), Type: row.Type, Evidence: jsonx.ObjectMap(row.Evidence), Action: row.Action, OperatorID: pgtypex.IDString(row.OperatorID), CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// vulnSourceDTOFromRow 转换漏洞源配置行。
func vulnSourceDTOFromRow(row sqlcgen.VulnSource) VulnSourceDTO {
	return VulnSourceDTO{ID: ids.Format(row.ID), TenantID: pgtypex.IDString(row.TenantID), Type: row.Type, Name: row.Name, Config: jsonx.ObjectMap(row.Config), DefaultLevel: row.DefaultLevel, Enabled: row.Enabled, LastSyncAt: timex.FromTimestamptz(row.LastSyncAt)}
}

// vulnProblemDTOFromRow 转换漏洞题草稿行。
func vulnProblemDTOFromRow(row sqlcgen.VulnProblem) VulnProblemDTO {
	return VulnProblemDTO{
		ID: ids.Format(row.ID), SourceID: pgtypex.IDString(row.SourceID), ExternalRef: pgtypex.TextValue(row.ExternalRef), Title: row.Title,
		Level: row.Level, RuntimeMode: row.RuntimeMode, DraftBody: jsonx.ObjectMap(row.DraftBody), PrevalidateStatus: row.PrevalidateStatus,
		PrevalidateDetail: jsonx.ObjectMap(row.PrevalidateDetail), ContentItemCode: pgtypex.TextValue(row.ContentItemCode),
		ContentItemVersion: pgtypex.TextValue(row.ContentItemVersion), Status: row.Status,
	}
}
