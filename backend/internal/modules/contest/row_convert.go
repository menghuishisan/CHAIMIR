// contest row_convert 文件负责 sqlc 行到 M8 领域模型的纯映射。
package contest

import (
	"chaimir/internal/modules/contest/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// contestFromRow 转换竞赛定义行。
func contestFromRow(row sqlcgen.Contest) (Contest, error) {
	rules, err := decodeMap(row.Rules, apperr.ErrContestInvalid)
	if err != nil {
		return Contest{}, err
	}
	return Contest{ID: row.ID, TenantID: row.TenantID, OrganizerID: row.OrganizerID, Name: row.Name, Mode: row.Mode, MatchMode: pgtypex.Int2Value(row.MatchMode), TeamMode: row.TeamMode, SignupStart: timex.FromTimestamptz(row.SignupStart), SignupEnd: timex.FromTimestamptz(row.SignupEnd), StartAt: timex.FromTimestamptz(row.StartAt), EndAt: timex.FromTimestamptz(row.EndAt), FreezeMinutes: row.FreezeMinutes, Rules: rules, Status: row.Status, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}, nil
}

// problemFromRow 转换竞赛题目行。
func problemFromRow(row sqlcgen.ContestProblem) (ContestProblem, error) {
	dynamic, err := decodeMap(row.DynamicScore, apperr.ErrContestProblemInvalid)
	if err != nil {
		return ContestProblem{}, err
	}
	battleConfig, err := decodeMap(row.BattleConfig, apperr.ErrContestProblemInvalid)
	if err != nil {
		return ContestProblem{}, err
	}
	return ContestProblem{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, ItemCode: row.ItemCode, ItemVersion: row.ItemVersion, Score: row.Score, DynamicScore: dynamic, BattleConfig: battleConfig, BattleRule: pgtypex.Int2Value(row.BattleRule), Seq: row.Seq}, nil
}

// teamFromRows 组合队伍和成员。
func teamFromRows(row sqlcgen.Team, members []sqlcgen.TeamMember) Team {
	out := Team{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, Name: row.Name, InviteCode: pgtypex.TextValue(row.InviteCode), Status: row.Status, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
	out.Members = make([]TeamMember, 0, len(members))
	for _, member := range members {
		out.Members = append(out.Members, teamMemberFromRow(member))
	}
	return out
}

// teamMemberFromRow 转换队员行。
func teamMemberFromRow(row sqlcgen.TeamMember) TeamMember {
	return TeamMember{ID: row.ID, TenantID: row.TenantID, TeamID: row.TeamID, AccountID: row.AccountID, MemberTenantID: row.MemberTenantID, IsLeader: row.IsLeader, JoinedAt: timex.FromTimestamptz(row.JoinedAt)}
}

// submissionFromRow 转换解题提交行。
func submissionFromRow(row sqlcgen.SolveSubmission) (SolveSubmission, error) {
	content, err := decodeMap(row.ContentRef, apperr.ErrContestSubmissionInvalid)
	if err != nil {
		return SolveSubmission{}, err
	}
	return SolveSubmission{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, ProblemID: row.ProblemID, TeamID: row.TeamID, SubmitterID: row.SubmitterID, ContentRef: content, SourceRef: row.SourceRef, JudgeTaskRef: pgtypex.TextValue(row.JudgeTaskRef), Passed: row.Passed, Score: row.Score, SandboxRef: pgtypex.TextValue(row.SandboxRef), SubmittedAt: timex.FromTimestamptz(row.SubmittedAt)}, nil
}

// battleEntryFromRow 转换参战物行。
func battleEntryFromRow(row sqlcgen.BattleEntry) BattleEntry {
	return BattleEntry{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, ProblemID: row.ProblemID, TeamID: row.TeamID, Role: row.Role, ArtifactRef: row.ArtifactRef, ArtifactHash: row.ArtifactHash, VersionNo: row.VersionNo, IsActive: row.IsActive, SubmittedAt: timex.FromTimestamptz(row.SubmittedAt)}
}

// battleMatchFromRow 转换对局行。
func battleMatchFromRow(row sqlcgen.BattleMatch) (BattleMatch, error) {
	delta, err := decodeMap(row.ScoreDelta, apperr.ErrContestBattleMatchFailed)
	if err != nil {
		return BattleMatch{}, err
	}
	return BattleMatch{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, ProblemID: row.ProblemID, EntryAID: row.EntryAID, EntryBID: row.EntryBID, SourceRef: row.SourceRef, SandboxRef: pgtypex.TextValue(row.SandboxRef), JudgeTaskRef: pgtypex.TextValue(row.JudgeTaskRef), Result: pgtypex.Int2Value(row.Result), ScoreDelta: delta, ReplayRef: pgtypex.TextValue(row.ReplayRef), Status: row.Status, MatchedAt: timex.FromTimestamptz(row.MatchedAt), FinishedAt: timex.FromTimestamptz(row.FinishedAt)}, nil
}

// ladderFromRow 转换排行榜行。
func ladderFromRow(row sqlcgen.ListLadderRow) LadderRank {
	return LadderRank{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, TeamID: row.TeamID, Score: row.Score, SolvedCount: row.SolvedCount, LastSolveAt: timex.FromTimestamptz(row.LastSolveAt), Rank: row.Rank, UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// ladderFromGetRow 转换按队伍读取的排行榜行。
func ladderFromGetRow(row sqlcgen.GetLadderByTeamRow) LadderRank {
	return LadderRank{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, TeamID: row.TeamID, Score: row.Score, SolvedCount: row.SolvedCount, LastSolveAt: timex.FromTimestamptz(row.LastSolveAt), Rank: row.Rank, UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// ladderFromUpsertRow 转换排行 upsert 返回行。
func ladderFromUpsertRow(row sqlcgen.CreateOrUpdateLadderRankRow) LadderRank {
	return LadderRank{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, TeamID: row.TeamID, Score: row.Score, SolvedCount: row.SolvedCount, LastSolveAt: timex.FromTimestamptz(row.LastSolveAt), Rank: row.Rank, UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// ladderSnapshotFromRow 转换封榜或归档排行榜快照行。
func ladderSnapshotFromRow(row sqlcgen.ContestLadderSnapshot) (LadderSnapshot, error) {
	items, err := decodeMapSlice(row.Ranking, apperr.ErrContestInvalid)
	if err != nil {
		return LadderSnapshot{}, err
	}
	return LadderSnapshot{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, SnapshotStatus: row.SnapshotStatus, Ranking: items, GeneratedAt: timex.FromTimestamptz(row.GeneratedAt)}, nil
}

// cheatFromRow 转换违规记录行。
func cheatFromRow(row sqlcgen.CheatRecord) (CheatRecord, error) {
	evidence, err := decodeMap(row.Evidence, apperr.ErrContestCheatInvalid)
	if err != nil {
		return CheatRecord{}, err
	}
	return CheatRecord{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, TeamID: row.TeamID, Type: row.Type, Evidence: evidence, Action: row.Action, OperatorID: pgtypex.Int8Value(row.OperatorID), CreatedAt: timex.FromTimestamptz(row.CreatedAt)}, nil
}

// vulnSourceFromRow 转换漏洞源行。
func vulnSourceFromRow(row sqlcgen.VulnSource) (VulnSource, error) {
	cfg, err := decodeMap(row.Config, apperr.ErrContestVulnSourceInvalid)
	if err != nil {
		return VulnSource{}, err
	}
	return VulnSource{ID: row.ID, TenantID: pgtypex.Int8Value(row.TenantID), Type: row.Type, Name: row.Name, Config: cfg, DefaultLevel: row.DefaultLevel, Enabled: row.Enabled, LastSyncAt: timex.FromTimestamptz(row.LastSyncAt), CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}, nil
}

// vulnProblemFromRow 转换漏洞题草稿行。
func vulnProblemFromRow(row sqlcgen.VulnProblem) (VulnProblem, error) {
	body, err := decodeMap(row.DraftBody, apperr.ErrContestVulnProblemInvalid)
	if err != nil {
		return VulnProblem{}, err
	}
	detail, err := decodeMap(row.PrevalidateDetail, apperr.ErrContestVulnProblemInvalid)
	if err != nil {
		return VulnProblem{}, err
	}
	return VulnProblem{ID: row.ID, TenantID: row.TenantID, SourceID: pgtypex.Int8Value(row.SourceID), ExternalRef: pgtypex.TextValue(row.ExternalRef), Title: row.Title, Level: row.Level, RuntimeMode: row.RuntimeMode, DraftBody: body, PrevalidateStatus: row.PrevalidateStatus, PrevalidateDetail: detail, ContentItemCode: pgtypex.TextValue(row.ContentItemCode), ContentItemVersion: pgtypex.TextValue(row.ContentItemVersion), Status: row.Status, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}, nil
}

// recordFromRow 转换个人竞赛记录行。
func recordFromRow(row sqlcgen.ListStudentContestRecordsRow) StudentContestRecord {
	return StudentContestRecord{ContestID: row.ContestID, TeamID: row.TeamID, Score: row.Score, Rank: row.Rank, ContestName: row.ContestName, ContestStatus: row.ContestStatus}
}

// decodeMap 解析 JSONB 对象,空值按空对象处理。
func decodeMap(raw []byte, invalid *apperr.Error) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	out, err := jsonx.ObjectMapStrict(raw)
	if err != nil {
		return nil, invalid.WithCause(err)
	}
	return out, nil
}

// decodeMapSlice 解析 JSONB 对象数组,空值按空数组处理。
func decodeMapSlice(raw []byte, invalid *apperr.Error) ([]map[string]any, error) {
	if len(raw) == 0 {
		return []map[string]any{}, nil
	}
	var out []map[string]any
	if err := jsonx.DecodeStrict(raw, &out); err != nil {
		return nil, invalid.WithCause(err)
	}
	if out == nil {
		return []map[string]any{}, nil
	}
	return out, nil
}

// encodeJSON 将结构化字段序列化为 JSONB 字节。
func encodeJSON(v any, invalid *apperr.Error) ([]byte, error) {
	raw, err := jsonx.AnyBytes(v, invalid)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
