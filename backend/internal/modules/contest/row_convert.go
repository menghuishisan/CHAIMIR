// contest row_convert 文件负责 sqlc 行到 M8 领域模型的纯映射。
package contest

import (
	"encoding/json"
	"time"

	"chaimir/internal/modules/contest/internal/sqlcgen"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
)

// contestFromRow 转换竞赛定义行。
func contestFromRow(row sqlcgen.Contest) (Contest, error) {
	rules, err := decodeMap(row.Rules, apperr.ErrContestInvalid)
	if err != nil {
		return Contest{}, err
	}
	return Contest{ID: row.ID, TenantID: row.TenantID, OrganizerID: row.OrganizerID, Name: row.Name, Mode: row.Mode, MatchMode: int16FromPG(row.MatchMode), TeamMode: row.TeamMode, SignupStart: timeFromPG(row.SignupStart), SignupEnd: timeFromPG(row.SignupEnd), StartAt: timeFromPG(row.StartAt), EndAt: timeFromPG(row.EndAt), FreezeMinutes: row.FreezeMinutes, Rules: rules, Status: row.Status, CreatedAt: timeFromPG(row.CreatedAt), UpdatedAt: timeFromPG(row.UpdatedAt)}, nil
}

// problemFromRow 转换竞赛题目行。
func problemFromRow(row sqlcgen.ContestProblem) (ContestProblem, error) {
	dynamic, err := decodeMap(row.DynamicScore, apperr.ErrContestProblemInvalid)
	if err != nil {
		return ContestProblem{}, err
	}
	return ContestProblem{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, ItemCode: row.ItemCode, ItemVersion: row.ItemVersion, Score: row.Score, DynamicScore: dynamic, BattleRule: int16FromPG(row.BattleRule), Seq: row.Seq}, nil
}

// teamFromRows 组合队伍和成员。
func teamFromRows(row sqlcgen.Team, members []sqlcgen.TeamMember) Team {
	out := Team{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, Name: row.Name, InviteCode: textFromPG(row.InviteCode), Status: row.Status, CreatedAt: timeFromPG(row.CreatedAt)}
	out.Members = make([]TeamMember, 0, len(members))
	for _, member := range members {
		out.Members = append(out.Members, teamMemberFromRow(member))
	}
	return out
}

// teamMemberFromRow 转换队员行。
func teamMemberFromRow(row sqlcgen.TeamMember) TeamMember {
	return TeamMember{ID: row.ID, TenantID: row.TenantID, TeamID: row.TeamID, AccountID: row.AccountID, MemberTenantID: row.MemberTenantID, IsLeader: row.IsLeader, JoinedAt: timeFromPG(row.JoinedAt)}
}

// submissionFromRow 转换解题提交行。
func submissionFromRow(row sqlcgen.SolveSubmission) (SolveSubmission, error) {
	content, err := decodeMap(row.ContentRef, apperr.ErrContestSubmissionInvalid)
	if err != nil {
		return SolveSubmission{}, err
	}
	return SolveSubmission{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, ProblemID: row.ProblemID, TeamID: row.TeamID, SubmitterID: row.SubmitterID, ContentRef: content, SourceRef: row.SourceRef, JudgeTaskRef: textFromPG(row.JudgeTaskRef), Passed: row.Passed, Score: row.Score, SandboxRef: textFromPG(row.SandboxRef), SubmittedAt: timeFromPG(row.SubmittedAt)}, nil
}

// battleEntryFromRow 转换参战物行。
func battleEntryFromRow(row sqlcgen.BattleEntry) BattleEntry {
	return BattleEntry{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, ProblemID: row.ProblemID, TeamID: row.TeamID, Role: row.Role, ArtifactRef: row.ArtifactRef, VersionNo: row.VersionNo, IsActive: row.IsActive, SubmittedAt: timeFromPG(row.SubmittedAt)}
}

// battleMatchFromRow 转换对局行。
func battleMatchFromRow(row sqlcgen.BattleMatch) (BattleMatch, error) {
	delta, err := decodeMap(row.ScoreDelta, apperr.ErrContestBattleMatchFailed)
	if err != nil {
		return BattleMatch{}, err
	}
	return BattleMatch{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, ProblemID: row.ProblemID, EntryAID: row.EntryAID, EntryBID: row.EntryBID, SourceRef: row.SourceRef, SandboxRef: textFromPG(row.SandboxRef), JudgeTaskRef: textFromPG(row.JudgeTaskRef), Result: int16FromPG(row.Result), ScoreDelta: delta, ReplayRef: textFromPG(row.ReplayRef), Status: row.Status, MatchedAt: timeFromPG(row.MatchedAt), FinishedAt: timeFromPG(row.FinishedAt)}, nil
}

// ladderFromRow 转换排行榜行。
func ladderFromRow(row sqlcgen.ListLadderRow) LadderRank {
	return LadderRank{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, TeamID: row.TeamID, Score: row.Score, SolvedCount: row.SolvedCount, LastSolveAt: timeFromPG(row.LastSolveAt), Rank: row.Rank, UpdatedAt: timeFromPG(row.UpdatedAt)}
}

// ladderFromGetRow 转换按队伍读取的排行榜行。
func ladderFromGetRow(row sqlcgen.GetLadderByTeamRow) LadderRank {
	return LadderRank{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, TeamID: row.TeamID, Score: row.Score, SolvedCount: row.SolvedCount, LastSolveAt: timeFromPG(row.LastSolveAt), Rank: row.Rank, UpdatedAt: timeFromPG(row.UpdatedAt)}
}

// ladderFromUpsertRow 转换排行 upsert 返回行。
func ladderFromUpsertRow(row sqlcgen.CreateOrUpdateLadderRankRow) LadderRank {
	return LadderRank{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, TeamID: row.TeamID, Score: row.Score, SolvedCount: row.SolvedCount, LastSolveAt: timeFromPG(row.LastSolveAt), Rank: row.Rank, UpdatedAt: timeFromPG(row.UpdatedAt)}
}

// snapshotFromRow 转换成绩快照行。
func snapshotFromRow(row sqlcgen.ContestResultSnapshot) (ResultSnapshot, error) {
	items, err := decodeMapSlice(row.FinalRanking, apperr.ErrContestInvalid)
	if err != nil {
		return ResultSnapshot{}, err
	}
	return ResultSnapshot{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, FinalRanking: items, GeneratedAt: timeFromPG(row.GeneratedAt)}, nil
}

// cheatFromRow 转换违规记录行。
func cheatFromRow(row sqlcgen.CheatRecord) (CheatRecord, error) {
	evidence, err := decodeMap(row.Evidence, apperr.ErrContestCheatInvalid)
	if err != nil {
		return CheatRecord{}, err
	}
	return CheatRecord{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, TeamID: row.TeamID, Type: row.Type, Evidence: evidence, Action: row.Action, OperatorID: int64FromPG(row.OperatorID), CreatedAt: timeFromPG(row.CreatedAt)}, nil
}

// vulnSourceFromRow 转换漏洞源行。
func vulnSourceFromRow(row sqlcgen.VulnSource) (VulnSource, error) {
	cfg, err := decodeMap(row.Config, apperr.ErrContestVulnSourceInvalid)
	if err != nil {
		return VulnSource{}, err
	}
	return VulnSource{ID: row.ID, TenantID: int64FromPG(row.TenantID), Type: row.Type, Name: row.Name, Config: cfg, DefaultLevel: row.DefaultLevel, Enabled: row.Enabled, LastSyncAt: timeFromPG(row.LastSyncAt), CreatedAt: timeFromPG(row.CreatedAt), UpdatedAt: timeFromPG(row.UpdatedAt)}, nil
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
	return VulnProblem{ID: row.ID, TenantID: row.TenantID, SourceID: int64FromPG(row.SourceID), ExternalRef: textFromPG(row.ExternalRef), Title: row.Title, Level: row.Level, RuntimeMode: row.RuntimeMode, DraftBody: body, PrevalidateStatus: row.PrevalidateStatus, PrevalidateDetail: detail, ContentItemCode: textFromPG(row.ContentItemCode), ContentItemVersion: textFromPG(row.ContentItemVersion), Status: row.Status, CreatedAt: timeFromPG(row.CreatedAt), UpdatedAt: timeFromPG(row.UpdatedAt)}, nil
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
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, invalid.WithCause(err)
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
}

// decodeMapSlice 解析 JSONB 对象数组,空值按空数组处理。
func decodeMapSlice(raw []byte, invalid *apperr.Error) ([]map[string]any, error) {
	if len(raw) == 0 {
		return []map[string]any{}, nil
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, invalid.WithCause(err)
	}
	if out == nil {
		return []map[string]any{}, nil
	}
	return out, nil
}

// encodeJSON 将结构化字段序列化为 JSONB 字节。
func encodeJSON(v any, invalid *apperr.Error) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, invalid.WithCause(err)
	}
	return raw, nil
}

// timeFromPG 转换 pg 时间,空值返回零时间。
func timeFromPG(v pgtype.Timestamptz) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time
}

// pgTime 构造 nullable timestamptz。
func pgTime(v time.Time) pgtype.Timestamptz {
	if v.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: v, Valid: true}
}

// textFromPG 转换 pg nullable text。
func textFromPG(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

// int64FromPG 转换 pg nullable int8。
func int64FromPG(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

// int16FromPG 转换 pg nullable int2。
func int16FromPG(v pgtype.Int2) int16 {
	if !v.Valid {
		return 0
	}
	return v.Int16
}

// pgText 构造 nullable text。
func pgText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

// pgInt8 构造 nullable int8。
func pgInt8(v int64) pgtype.Int8 {
	if v == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: v, Valid: true}
}

// pgInt2 构造 nullable int2。
func pgInt2(v int16) pgtype.Int2 {
	if v == 0 {
		return pgtype.Int2{}
	}
	return pgtype.Int2{Int16: v, Valid: true}
}
