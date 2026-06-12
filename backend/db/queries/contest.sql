-- contest.sql 定义 M8 竞赛模块的 sqlc 查询,仅访问竞赛模块自有表。
-- name: CreateContest :one
INSERT INTO contest (id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, created_at, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 1, now(), now(), NULL)
RETURNING id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, created_at, updated_at, deleted_at;

-- name: GetContest :one
SELECT id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, created_at, updated_at, deleted_at
FROM contest
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: ListContests :many
SELECT id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, created_at, updated_at, deleted_at
FROM contest
WHERE tenant_id = $1 AND deleted_at IS NULL AND ($2::smallint = 0 OR status = $2)
ORDER BY updated_at DESC, id DESC
LIMIT $3 OFFSET $4;

-- name: CountContests :one
SELECT COUNT(*)::bigint
FROM contest
WHERE tenant_id = $1 AND deleted_at IS NULL AND ($2::smallint = 0 OR status = $2);

-- name: UpdateContest :one
UPDATE contest
SET name = $3,
    mode = $4,
    match_mode = $5,
    team_mode = $6,
    signup_start = $7,
    signup_end = $8,
    start_at = $9,
    end_at = $10,
    freeze_minutes = $11,
    rules = $12,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 1 AND deleted_at IS NULL
RETURNING id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, created_at, updated_at, deleted_at;

-- name: SetContestStatus :one
UPDATE contest
SET status = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, created_at, updated_at, deleted_at;

-- name: UpsertContestProblem :one
INSERT INTO contest_problem (id, tenant_id, contest_id, item_code, item_version, score, dynamic_score, battle_rule, seq)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (tenant_id, contest_id, item_code, item_version) DO UPDATE
SET score = EXCLUDED.score,
    dynamic_score = EXCLUDED.dynamic_score,
    battle_rule = EXCLUDED.battle_rule,
    seq = EXCLUDED.seq
RETURNING id, tenant_id, contest_id, item_code, item_version, score, dynamic_score, battle_rule, seq;

-- name: GetContestProblem :one
SELECT id, tenant_id, contest_id, item_code, item_version, score, dynamic_score, battle_rule, seq
FROM contest_problem
WHERE tenant_id = $1 AND id = $2;

-- name: ListContestProblems :many
SELECT id, tenant_id, contest_id, item_code, item_version, score, dynamic_score, battle_rule, seq
FROM contest_problem
WHERE tenant_id = $1 AND contest_id = $2
ORDER BY seq ASC, id ASC;

-- name: CreateTeam :one
INSERT INTO team (id, tenant_id, contest_id, name, invite_code, status, created_at)
VALUES ($1, $2, $3, $4, $5, 1, now())
RETURNING id, tenant_id, contest_id, name, invite_code, status, created_at;

-- name: GetTeam :one
SELECT id, tenant_id, contest_id, name, invite_code, status, created_at
FROM team
WHERE tenant_id = $1 AND id = $2;

-- name: GetTeamByInviteCode :one
SELECT id, tenant_id, contest_id, name, invite_code, status, created_at
FROM team
WHERE tenant_id = $1 AND invite_code = $2;

-- name: GetTeamForAccount :one
SELECT t.id, t.tenant_id, t.contest_id, t.name, t.invite_code, t.status, t.created_at
FROM team t
JOIN team_member m ON m.tenant_id = t.tenant_id AND m.team_id = t.id
WHERE t.tenant_id = $1 AND t.contest_id = $2 AND m.member_tenant_id = $3 AND m.account_id = $4
ORDER BY t.created_at DESC, t.id DESC
LIMIT 1;

-- name: LockTeam :one
UPDATE team
SET status = 2
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, contest_id, name, invite_code, status, created_at;

-- name: LockContestTeams :exec
UPDATE team
SET status = 2
WHERE tenant_id = $1 AND contest_id = $2 AND status = 1;

-- name: AddTeamMember :one
INSERT INTO team_member (id, tenant_id, team_id, account_id, member_tenant_id, is_leader, joined_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (tenant_id, team_id, member_tenant_id, account_id) DO UPDATE SET is_leader = team_member.is_leader OR EXCLUDED.is_leader
RETURNING id, tenant_id, team_id, account_id, member_tenant_id, is_leader, joined_at;

-- name: ListTeamMembers :many
SELECT id, tenant_id, team_id, account_id, member_tenant_id, is_leader, joined_at
FROM team_member
WHERE tenant_id = $1 AND team_id = $2
ORDER BY is_leader DESC, joined_at ASC, id ASC;

-- name: AccountTeamIDs :many
SELECT t.id
FROM team t
JOIN team_member m ON m.tenant_id = t.tenant_id AND m.team_id = t.id
WHERE t.tenant_id = $1 AND t.contest_id = $2 AND m.member_tenant_id = $3 AND m.account_id = $4;

-- name: CreateSolveSubmission :one
INSERT INTO solve_submission (id, tenant_id, contest_id, problem_id, team_id, submitter_id, content_ref, source_ref, judge_task_ref, passed, score, sandbox_ref, submitted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, false, 0, $10, now())
RETURNING id, tenant_id, contest_id, problem_id, team_id, submitter_id, content_ref, source_ref, judge_task_ref, passed, score, sandbox_ref, submitted_at;

-- name: GetSolveSubmission :one
SELECT id, tenant_id, contest_id, problem_id, team_id, submitter_id, content_ref, source_ref, judge_task_ref, passed, score, sandbox_ref, submitted_at
FROM solve_submission
WHERE tenant_id = $1 AND id = $2;

-- name: GetSolveSubmissionByJudgeTask :one
SELECT id, tenant_id, contest_id, problem_id, team_id, submitter_id, content_ref, source_ref, judge_task_ref, passed, score, sandbox_ref, submitted_at
FROM solve_submission
WHERE tenant_id = $1 AND judge_task_ref = $2;

-- name: UpdateSolveSubmissionResult :one
UPDATE solve_submission
SET passed = $3,
    score = $4
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, contest_id, problem_id, team_id, submitter_id, content_ref, source_ref, judge_task_ref, passed, score, sandbox_ref, submitted_at;

-- name: RecentFailedSolveCount :one
SELECT COUNT(*)::bigint
FROM solve_submission
WHERE tenant_id = $1 AND contest_id = $2 AND problem_id = $3 AND team_id = $4 AND passed = false AND submitted_at >= now() - ($5::text || ' seconds')::interval;

-- name: RecentSolveCount :one
SELECT COUNT(*)::bigint
FROM solve_submission
WHERE tenant_id = $1 AND contest_id = $2 AND problem_id = $3 AND team_id = $4 AND submitted_at >= now() - ($5::text || ' seconds')::interval;

-- name: CreateOrUpdateLadderRank :one
INSERT INTO ladder_rank (id, tenant_id, contest_id, team_id, score, solved_count, last_solve_at, rank, updated_at)
VALUES ($1, $2, $3, $4, $5::text::numeric, $6, $7, 0, now())
ON CONFLICT (tenant_id, contest_id, team_id) DO UPDATE
SET score = EXCLUDED.score,
    solved_count = EXCLUDED.solved_count,
    last_solve_at = EXCLUDED.last_solve_at,
    updated_at = now()
RETURNING id, tenant_id, contest_id, team_id, score::float8 AS score, solved_count, last_solve_at, rank, updated_at;

-- name: SumTeamSolvedScore :one
SELECT COALESCE(SUM(best.score), 0)::float8 AS score, COUNT(*)::int AS solved_count, MAX(best.submitted_at)::timestamptz AS last_solve_at
FROM (
    SELECT DISTINCT ON (problem_id) problem_id, score, submitted_at
    FROM solve_submission
    WHERE tenant_id = $1 AND contest_id = $2 AND team_id = $3 AND passed = true
    ORDER BY problem_id, score DESC, submitted_at ASC
) best;

-- name: CountProblemSolvedTeams :one
SELECT COUNT(DISTINCT team_id)::bigint
FROM solve_submission
WHERE tenant_id = $1 AND contest_id = $2 AND problem_id = $3 AND passed = true;

-- name: ListLadder :many
SELECT id, tenant_id, contest_id, team_id, score::float8 AS score, solved_count, last_solve_at, rank, updated_at
FROM ladder_rank
WHERE tenant_id = $1 AND contest_id = $2
ORDER BY rank ASC, score DESC, solved_count DESC, last_solve_at ASC NULLS LAST, team_id ASC
LIMIT $3 OFFSET $4;

-- name: GetLadderByTeam :one
SELECT id, tenant_id, contest_id, team_id, score::float8 AS score, solved_count, last_solve_at, rank, updated_at
FROM ladder_rank
WHERE tenant_id = $1 AND contest_id = $2 AND team_id = $3;

-- name: CountLadder :one
SELECT COUNT(*)::bigint
FROM ladder_rank
WHERE tenant_id = $1 AND contest_id = $2;

-- name: RefreshContestRanks :exec
WITH ranked AS (
    SELECT lr0.id, ROW_NUMBER() OVER (ORDER BY lr0.score DESC, lr0.solved_count DESC, lr0.last_solve_at ASC NULLS LAST, lr0.team_id ASC)::int AS new_rank
    FROM ladder_rank lr0
    WHERE lr0.tenant_id = $1 AND lr0.contest_id = $2
)
UPDATE ladder_rank lr
SET rank = ranked.new_rank, updated_at = now()
FROM ranked
WHERE lr.id = ranked.id;

-- name: DeactivateBattleEntries :exec
UPDATE battle_entry
SET is_active = false
WHERE tenant_id = $1 AND contest_id = $2 AND problem_id = $3 AND team_id = $4 AND role = $5;

-- name: NextBattleVersion :one
SELECT COALESCE(MAX(version_no), 0)::int + 1
FROM battle_entry
WHERE tenant_id = $1 AND contest_id = $2 AND problem_id = $3 AND team_id = $4 AND role = $5;

-- name: CreateBattleEntry :one
INSERT INTO battle_entry (id, tenant_id, contest_id, problem_id, team_id, role, artifact_ref, version_no, is_active, submitted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true, now())
RETURNING id, tenant_id, contest_id, problem_id, team_id, role, artifact_ref, version_no, is_active, submitted_at;

-- name: ListBattleEntriesForTeam :many
SELECT id, tenant_id, contest_id, problem_id, team_id, role, artifact_ref, version_no, is_active, submitted_at
FROM battle_entry
WHERE tenant_id = $1 AND contest_id = $2 AND team_id = $3
ORDER BY submitted_at DESC, id DESC;

-- name: GetBattleEntry :one
SELECT id, tenant_id, contest_id, problem_id, team_id, role, artifact_ref, version_no, is_active, submitted_at
FROM battle_entry
WHERE tenant_id = $1 AND id = $2;

-- name: ListActiveBattleOpponents :many
SELECT id, tenant_id, contest_id, problem_id, team_id, role, artifact_ref, version_no, is_active, submitted_at
FROM battle_entry
WHERE tenant_id = $1 AND contest_id = $2 AND problem_id = $3 AND is_active = true AND id <> $4 AND team_id <> $5
ORDER BY submitted_at ASC, id ASC
LIMIT $6;

-- name: CreateBattleMatch :one
INSERT INTO battle_match (id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, NULL, NULL, '{}'::jsonb, NULL, 1, now(), NULL)
RETURNING id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at;

-- name: ClaimPendingBattleMatchesAcrossTenants :many
UPDATE battle_match
SET status = 2
WHERE id IN (
    SELECT id FROM battle_match
    WHERE status = 1
    ORDER BY matched_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at;

-- name: GetBattleMatch :one
SELECT id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at
FROM battle_match
WHERE tenant_id = $1 AND id = $2;

-- name: GetBattleMatchByJudgeTask :one
SELECT id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at
FROM battle_match
WHERE tenant_id = $1 AND judge_task_ref = $2;

-- name: StartBattleMatch :one
UPDATE battle_match
SET sandbox_ref = $3,
    judge_task_ref = $4,
    status = 2
WHERE tenant_id = $1 AND id = $2 AND status = 2
RETURNING id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at;

-- name: ListBattleMatchesForTeam :many
SELECT m.id, m.tenant_id, m.contest_id, m.problem_id, m.entry_a_id, m.entry_b_id, m.source_ref, m.sandbox_ref, m.judge_task_ref, m.result, m.score_delta, m.replay_ref, m.status, m.matched_at, m.finished_at
FROM battle_match m
JOIN battle_entry a ON a.tenant_id = m.tenant_id AND a.id = m.entry_a_id
JOIN battle_entry b ON b.tenant_id = m.tenant_id AND b.id = m.entry_b_id
WHERE m.tenant_id = $1 AND m.contest_id = $2 AND (a.team_id = $3 OR b.team_id = $3)
ORDER BY m.matched_at DESC, m.id DESC
LIMIT $4 OFFSET $5;

-- name: FinishBattleMatch :one
UPDATE battle_match
SET sandbox_ref = $3,
    judge_task_ref = $4,
    result = $5,
    score_delta = $6,
    replay_ref = $7,
    status = 3,
    finished_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at;

-- name: FailBattleMatch :one
UPDATE battle_match
SET status = 4, finished_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at;

-- name: CreateResultSnapshot :one
INSERT INTO contest_result_snapshot (id, tenant_id, contest_id, final_ranking, generated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (tenant_id, contest_id) DO UPDATE
SET final_ranking = EXCLUDED.final_ranking,
    generated_at = now()
RETURNING id, tenant_id, contest_id, final_ranking, generated_at;

-- name: GetResultSnapshot :one
SELECT id, tenant_id, contest_id, final_ranking, generated_at
FROM contest_result_snapshot
WHERE tenant_id = $1 AND contest_id = $2;

-- name: ListStudentContestRecords :many
SELECT c.id AS contest_id, t.id AS team_id, COALESCE(l.score::float8, 0)::float8 AS score, COALESCE(l.rank, 0)::int AS rank, c.name AS contest_name, c.status AS contest_status
FROM team_member tm
JOIN team t ON t.tenant_id = tm.tenant_id AND t.id = tm.team_id
JOIN contest c ON c.tenant_id = t.tenant_id AND c.id = t.contest_id
LEFT JOIN ladder_rank l ON l.tenant_id = t.tenant_id AND l.contest_id = t.contest_id AND l.team_id = t.id
WHERE tm.tenant_id = $1 AND tm.member_tenant_id = $1 AND tm.account_id = $2 AND c.deleted_at IS NULL
ORDER BY c.end_at DESC, c.id DESC;

-- name: CreateCheatRecord :one
INSERT INTO cheat_record (id, tenant_id, contest_id, team_id, type, evidence, action, operator_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
RETURNING id, tenant_id, contest_id, team_id, type, evidence, action, operator_id, created_at;

-- name: ListCheatRecords :many
SELECT id, tenant_id, contest_id, team_id, type, evidence, action, operator_id, created_at
FROM cheat_record
WHERE tenant_id = $1 AND contest_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3 OFFSET $4;

-- name: UpsertVulnSource :one
INSERT INTO vuln_source (id, tenant_id, type, name, config, default_level, enabled, last_sync_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, now(), now())
ON CONFLICT (tenant_id, id) DO UPDATE
SET type = EXCLUDED.type,
    name = EXCLUDED.name,
    config = EXCLUDED.config,
    default_level = EXCLUDED.default_level,
    enabled = EXCLUDED.enabled,
    updated_at = now()
RETURNING id, tenant_id, type, name, config, default_level, enabled, last_sync_at, created_at, updated_at;

-- name: ListVulnSources :many
SELECT id, tenant_id, type, name, config, default_level, enabled, last_sync_at, created_at, updated_at
FROM vuln_source
WHERE tenant_id IS NULL OR tenant_id = $1
ORDER BY tenant_id NULLS FIRST, id DESC;

-- name: GetVulnSource :one
SELECT id, tenant_id, type, name, config, default_level, enabled, last_sync_at, created_at, updated_at
FROM vuln_source
WHERE (tenant_id = $1 OR tenant_id IS NULL) AND id = $2;

-- name: MarkVulnSourceSynced :one
UPDATE vuln_source
SET last_sync_at = now(), updated_at = now()
WHERE (tenant_id = $1 OR tenant_id IS NULL) AND id = $2
RETURNING id, tenant_id, type, name, config, default_level, enabled, last_sync_at, created_at, updated_at;

-- name: UpsertVulnProblem :one
INSERT INTO vuln_problem (id, tenant_id, source_id, external_ref, title, level, runtime_mode, draft_body, prevalidate_status, prevalidate_detail, content_item_code, content_item_version, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, '{}'::jsonb, NULL, NULL, 1, now(), now())
ON CONFLICT (tenant_id, COALESCE(source_id, 0), external_ref) DO UPDATE
SET title = EXCLUDED.title,
    level = EXCLUDED.level,
    runtime_mode = EXCLUDED.runtime_mode,
    draft_body = EXCLUDED.draft_body,
    updated_at = now()
RETURNING id, tenant_id, source_id, external_ref, title, level, runtime_mode, draft_body, prevalidate_status, prevalidate_detail, content_item_code, content_item_version, status, created_at, updated_at;

-- name: GetVulnProblem :one
SELECT id, tenant_id, source_id, external_ref, title, level, runtime_mode, draft_body, prevalidate_status, prevalidate_detail, content_item_code, content_item_version, status, created_at, updated_at
FROM vuln_problem
WHERE tenant_id = $1 AND id = $2;

-- name: ListVulnProblems :many
SELECT id, tenant_id, source_id, external_ref, title, level, runtime_mode, draft_body, prevalidate_status, prevalidate_detail, content_item_code, content_item_version, status, created_at, updated_at
FROM vuln_problem
WHERE tenant_id = $1 AND ($2::bigint = 0 OR source_id = $2) AND ($3::smallint = 0 OR status = $3)
ORDER BY updated_at DESC, id DESC
LIMIT $4 OFFSET $5;

-- name: SetVulnProblemPrevalidate :one
UPDATE vuln_problem
SET prevalidate_status = $3,
    prevalidate_detail = $4,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 1
RETURNING id, tenant_id, source_id, external_ref, title, level, runtime_mode, draft_body, prevalidate_status, prevalidate_detail, content_item_code, content_item_version, status, created_at, updated_at;

-- name: FinalizeVulnProblem :one
UPDATE vuln_problem
SET content_item_code = $3,
    content_item_version = $4,
    status = 2,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND prevalidate_status = 2 AND status = 1
RETURNING id, tenant_id, source_id, external_ref, title, level, runtime_mode, draft_body, prevalidate_status, prevalidate_detail, content_item_code, content_item_version, status, created_at, updated_at;

-- name: ContestStats :one
SELECT
    COUNT(*)::bigint AS contest_count,
    COALESCE(SUM(CASE WHEN status IN (2, 3, 4) THEN 1 ELSE 0 END), 0)::bigint AS active_contest_count,
    COALESCE((SELECT COUNT(DISTINCT tm.member_tenant_id::text || ':' || tm.account_id::text)::bigint FROM team_member tm WHERE tm.tenant_id = $1), 0)::bigint AS participant_count
FROM contest c
WHERE c.tenant_id = $1 AND c.deleted_at IS NULL;

-- name: ClaimAutoArchiveContestsAcrossTenants :many
UPDATE contest
SET status = 5, updated_at = now()
WHERE id IN (
    SELECT id FROM contest
    WHERE status IN (3, 4) AND end_at <= now() AND deleted_at IS NULL
    ORDER BY end_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, created_at, updated_at, deleted_at;
