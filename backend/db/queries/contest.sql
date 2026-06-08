-- M8 contest sqlc 查询:仅访问 contest 模块自有租户表。

-- name: CreateContest :one
INSERT INTO contest (id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status)
VALUES (@id, @tenant_id, @organizer_id, @name, @mode, @match_mode, @team_mode, @signup_start, @signup_end, @start_at, @end_at, @freeze_minutes, @rules, @status)
RETURNING *;

-- name: GetContestByID :one
SELECT * FROM contest WHERE id = @id AND deleted_at IS NULL;

-- name: ListContests :many
SELECT * FROM contest
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::smallint IS NULL OR status = sqlc.narg('status'))
ORDER BY start_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountContests :one
SELECT count(*)::bigint FROM contest WHERE deleted_at IS NULL AND (sqlc.narg('status')::smallint IS NULL OR status = sqlc.narg('status'));

-- name: UpdateContest :one
UPDATE contest SET name = @name, mode = @mode, match_mode = @match_mode, team_mode = @team_mode,
  signup_start = @signup_start, signup_end = @signup_end, start_at = @start_at, end_at = @end_at,
  freeze_minutes = @freeze_minutes, rules = @rules
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: UpdateContestStatus :one
UPDATE contest SET status = @status WHERE id = @id AND deleted_at IS NULL RETURNING *;

-- name: CreateContestProblem :one
INSERT INTO contest_problem (id, tenant_id, contest_id, item_code, item_version, score, dynamic_score, battle_rule, seq)
VALUES (@id, @tenant_id, @contest_id, @item_code, @item_version, @score, @dynamic_score, @battle_rule, @seq)
RETURNING *;

-- name: ListContestProblems :many
SELECT * FROM contest_problem WHERE contest_id = @contest_id ORDER BY seq ASC;

-- name: GetContestProblemByID :one
SELECT * FROM contest_problem WHERE id = @id;

-- name: CreateTeam :one
INSERT INTO team (id, tenant_id, contest_id, name, invite_code, status)
VALUES (@id, @tenant_id, @contest_id, @name, @invite_code, @status)
RETURNING *;

-- name: GetTeamByID :one
SELECT * FROM team WHERE id = @id;

-- name: GetTeamByContestAndAccount :one
SELECT t.* FROM team t
JOIN team_member tm ON tm.team_id = t.id
WHERE t.contest_id = @contest_id AND tm.account_id = @account_id;

-- name: AddTeamMember :one
INSERT INTO team_member (id, tenant_id, team_id, account_id, member_tenant_id, is_leader)
VALUES (@id, @tenant_id, @team_id, @account_id, @member_tenant_id, @is_leader)
ON CONFLICT (tenant_id, team_id, account_id) DO UPDATE SET is_leader = EXCLUDED.is_leader
RETURNING *;

-- name: ListTeamMembers :many
SELECT * FROM team_member WHERE team_id = @team_id ORDER BY is_leader DESC, id ASC;

-- name: GetTeamMember :one
SELECT * FROM team_member WHERE team_id = @team_id AND account_id = @account_id;

-- name: LockTeam :one
UPDATE team SET status = @status WHERE id = @id RETURNING *;

-- name: CreateSolveSubmission :one
INSERT INTO solve_submission (id, tenant_id, contest_id, problem_id, team_id, submitter_id, content_ref, source_ref, judge_task_ref, sandbox_ref)
VALUES (@id, @tenant_id, @contest_id, @problem_id, @team_id, @submitter_id, @content_ref, @source_ref, @judge_task_ref, @sandbox_ref)
RETURNING *;

-- name: GetSolveSubmissionByID :one
SELECT * FROM solve_submission WHERE id = @id;

-- name: GetSolveSubmissionByJudgeTask :one
SELECT s.*, p.score AS max_score FROM solve_submission s
JOIN contest_problem p ON p.id = s.problem_id
WHERE s.judge_task_ref = @judge_task_ref;

-- name: UpdateSolveSubmissionResult :one
UPDATE solve_submission SET passed = @passed, score = @score WHERE id = @id RETURNING *;

-- name: CreateBattleEntry :one
WITH disabled AS (
  UPDATE battle_entry SET is_active = false WHERE contest_id = @contest_id AND team_id = @team_id AND role = @role
)
INSERT INTO battle_entry (id, tenant_id, contest_id, team_id, role, artifact_ref, version_no, is_active)
VALUES (@id, @tenant_id, @contest_id, @team_id, @role, @artifact_ref, @version_no, true)
RETURNING *;

-- name: ListBattleEntries :many
SELECT * FROM battle_entry WHERE contest_id = @contest_id AND team_id = @team_id ORDER BY version_no DESC;

-- name: GetBattleEntryByID :one
SELECT * FROM battle_entry WHERE id = @id;

-- name: CreateBattleMatch :one
INSERT INTO battle_match (id, tenant_id, contest_id, entry_a_id, entry_b_id, sandbox_ref, result, score_delta, replay_ref, finished_at)
VALUES (@id, @tenant_id, @contest_id, @entry_a_id, @entry_b_id, @sandbox_ref, @result, @score_delta, @replay_ref, now())
RETURNING *;

-- name: ListBattleMatches :many
SELECT * FROM battle_match
WHERE contest_id = @contest_id AND (sqlc.narg('entry_id')::bigint IS NULL OR entry_a_id = sqlc.narg('entry_id') OR entry_b_id = sqlc.narg('entry_id'))
ORDER BY matched_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: ListBattleMatchesByTeam :many
SELECT m.* FROM battle_match m
JOIN battle_entry ea ON ea.id = m.entry_a_id
JOIN battle_entry eb ON eb.id = m.entry_b_id
WHERE m.contest_id = @contest_id AND (ea.team_id = @team_id OR eb.team_id = @team_id)
ORDER BY m.matched_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: GetBattleMatchByID :one
SELECT * FROM battle_match WHERE id = @id;

-- name: UpsertLadderRank :one
INSERT INTO ladder_rank (id, tenant_id, contest_id, team_id, score, solved_count, last_solve_at, rank)
VALUES (@id, @tenant_id, @contest_id, @team_id, @score, @solved_count, @last_solve_at, @rank)
ON CONFLICT (tenant_id, contest_id, team_id) DO UPDATE SET
  score = EXCLUDED.score,
  solved_count = EXCLUDED.solved_count,
  last_solve_at = COALESCE(EXCLUDED.last_solve_at, ladder_rank.last_solve_at),
  rank = EXCLUDED.rank,
  updated_at = now()
RETURNING *;

-- name: GetLadderRank :one
SELECT * FROM ladder_rank WHERE contest_id = @contest_id AND team_id = @team_id;

-- name: ListLadderRanks :many
SELECT * FROM ladder_rank WHERE contest_id = @contest_id ORDER BY rank ASC, score DESC LIMIT @limit_count OFFSET @offset_count;

-- name: CreateResultSnapshot :one
INSERT INTO contest_result_snapshot (id, tenant_id, contest_id, final_ranking)
VALUES (@id, @tenant_id, @contest_id, @final_ranking)
ON CONFLICT (tenant_id, contest_id) DO UPDATE SET final_ranking = EXCLUDED.final_ranking, generated_at = now()
RETURNING *;

-- name: GetResultSnapshot :one
SELECT * FROM contest_result_snapshot WHERE contest_id = @contest_id;

-- name: ListStudentAchievements :many
SELECT lr.* FROM ladder_rank lr
JOIN team_member tm ON tm.team_id = lr.team_id
WHERE tm.account_id = @student_id
ORDER BY lr.rank ASC, lr.score DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CreateCheatRecord :one
INSERT INTO cheat_record (id, tenant_id, contest_id, team_id, type, evidence, action, operator_id)
VALUES (@id, @tenant_id, @contest_id, @team_id, @type, @evidence, @action, @operator_id)
RETURNING *;

-- name: ListCheatRecordsByContest :many
SELECT * FROM cheat_record WHERE contest_id = @contest_id ORDER BY created_at DESC LIMIT @limit_count OFFSET @offset_count;

-- name: CreateVulnSource :one
INSERT INTO vuln_source (id, tenant_id, type, name, config, default_level, enabled)
VALUES (@id, @tenant_id, @type, @name, @config, @default_level, @enabled)
RETURNING *;

-- name: ListVulnSources :many
SELECT * FROM vuln_source ORDER BY id DESC LIMIT @limit_count OFFSET @offset_count;

-- name: GetVulnSourceByID :one
SELECT * FROM vuln_source WHERE id = @id;

-- name: MarkVulnSourceSynced :one
UPDATE vuln_source SET last_sync_at = now() WHERE id = @id RETURNING *;

-- name: CreateVulnProblem :one
INSERT INTO vuln_problem (id, tenant_id, source_id, external_ref, title, level, runtime_mode, draft_body, prevalidate_status, prevalidate_detail, status)
VALUES (@id, @tenant_id, @source_id, @external_ref, @title, @level, @runtime_mode, @draft_body, @prevalidate_status, @prevalidate_detail, @status)
RETURNING *;

-- name: GetVulnProblemByID :one
SELECT * FROM vuln_problem WHERE id = @id;

-- name: UpdateVulnProblemPrevalidate :one
UPDATE vuln_problem SET prevalidate_status = @prevalidate_status, prevalidate_detail = @prevalidate_detail
WHERE id = @id
RETURNING *;

-- name: FinalizeVulnProblem :one
UPDATE vuln_problem SET status = @status, content_item_code = @content_item_code, content_item_version = @content_item_version
WHERE id = @id
RETURNING *;

-- name: CountActiveContests :one
SELECT count(*)::bigint FROM contest WHERE deleted_at IS NULL AND status IN (2, 3, 4);

-- name: CountContestTeams :one
SELECT count(*)::bigint FROM team;
