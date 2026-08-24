-- contest.sql 定义 M8 竞赛模块的 sqlc 查询,仅访问竞赛模块自有表。
-- name: CreateContest :one
INSERT INTO contest (id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, created_at, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 1, now(), now(), NULL)
RETURNING id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, archive_lease_token, archive_lease_until, created_at, updated_at, deleted_at;

-- name: CompleteAutoArchiveContest :execrows
UPDATE contest AS c
SET status = 6, archive_lease_token = '', archive_lease_until = NULL, updated_at = now()
WHERE c.tenant_id = sqlc.arg(tenant_id) AND c.id = sqlc.arg(contest_id) AND c.status = 5
  AND c.archive_lease_token = sqlc.arg(lease_token) AND c.archive_lease_until > now()
  AND NOT EXISTS (
      SELECT 1 FROM battle_match m
      WHERE m.tenant_id = c.tenant_id AND m.contest_id = c.id AND m.status IN (1, 2)
  );

-- name: ClaimManualArchiveContest :one
-- 已结束竞赛的人工归档和自动归档共用同一租约栅栏,避免并发生成不同最终快照。
UPDATE contest AS c
SET archive_lease_token = sqlc.arg(lease_token), archive_lease_until = sqlc.arg(lease_until)::timestamptz, updated_at = now()
WHERE c.tenant_id = sqlc.arg(tenant_id) AND c.id = sqlc.arg(contest_id) AND c.status = 5
  AND (c.archive_lease_until IS NULL OR c.archive_lease_until <= sqlc.arg(stale_before)::timestamptz)
  AND NOT EXISTS (
      SELECT 1 FROM battle_match m
      WHERE m.tenant_id = c.tenant_id AND m.contest_id = c.id AND m.status IN (1, 2)
  )
RETURNING c.id, c.tenant_id, c.created_at, c.archive_lease_token, c.archive_lease_until;

-- name: GetContest :one
SELECT id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, archive_lease_token, archive_lease_until, created_at, updated_at, deleted_at
FROM contest
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: FindPublishedContestTenant :one
-- 学生入口允许跨校发现已发布竞赛,但只返回竞赛所属租户;后续读写仍必须进入该租户事务。
SELECT tenant_id
FROM contest
WHERE id = $1 AND deleted_at IS NULL AND status BETWEEN 2 AND 6;

-- name: FindTeamTenant :one
SELECT tenant_id
FROM team
WHERE id = $1;

-- name: FindBattleMatchTenant :one
SELECT tenant_id
FROM battle_match
WHERE id = $1;

-- name: ListContests :many
SELECT id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, archive_lease_token, archive_lease_until, created_at, updated_at, deleted_at
FROM contest
WHERE tenant_id = $1 AND deleted_at IS NULL AND ($2::smallint = 0 OR status = $2)
ORDER BY updated_at DESC, id DESC
LIMIT $3 OFFSET $4;

-- name: CountContests :one
SELECT COUNT(*)::bigint
FROM contest
WHERE tenant_id = $1 AND deleted_at IS NULL AND ($2::smallint = 0 OR status = $2);

-- name: ListStudentContests :many
-- status 传 0 回学生可发现的全部赛事(草稿态不可见);传具体状态时仍受可见区间约束。
SELECT id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, archive_lease_token, archive_lease_until, created_at, updated_at, deleted_at
FROM contest
WHERE tenant_id = $1 AND deleted_at IS NULL AND status BETWEEN 2 AND 6
  AND (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountStudentContests :one
SELECT COUNT(*)::bigint
FROM contest
WHERE tenant_id = $1 AND deleted_at IS NULL AND status BETWEEN 2 AND 6
  AND (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint);

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
WHERE tenant_id = $1 AND id = $2 AND status IN (1, 2) AND deleted_at IS NULL
RETURNING id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, archive_lease_token, archive_lease_until, created_at, updated_at, deleted_at;

-- name: SetContestStatus :one
UPDATE contest
SET status = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, organizer_id, name, mode, match_mode, team_mode, signup_start, signup_end, start_at, end_at, freeze_minutes, rules, status, archive_lease_token, archive_lease_until, created_at, updated_at, deleted_at;

-- name: UpsertContestProblem :one
INSERT INTO contest_problem (id, tenant_id, contest_id, item_code, item_version, score, dynamic_score, battle_config, battle_rule, seq, composition_digest, composition_snapshot)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (tenant_id, contest_id, item_code, item_version) DO UPDATE
SET score = EXCLUDED.score,
    dynamic_score = EXCLUDED.dynamic_score,
    battle_config = EXCLUDED.battle_config,
    battle_rule = EXCLUDED.battle_rule,
    seq = EXCLUDED.seq,
    composition_digest = EXCLUDED.composition_digest,
    composition_snapshot = EXCLUDED.composition_snapshot
RETURNING id, tenant_id, contest_id, item_code, item_version, score, dynamic_score, battle_config, battle_rule, seq, composition_digest, composition_snapshot;

-- name: GetContestProblem :one
SELECT id, tenant_id, contest_id, item_code, item_version, score, dynamic_score, battle_config, battle_rule, seq, composition_digest, composition_snapshot
FROM contest_problem
WHERE tenant_id = $1 AND id = $2;

-- name: ListContestProblems :many
SELECT id, tenant_id, contest_id, item_code, item_version, score, dynamic_score, battle_config, battle_rule, seq, composition_digest, composition_snapshot
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

-- name: GetContestAccessGrant :one
SELECT id, tenant_id, contest_id, team_id, sandbox_id, member_tenant_id, member_account_id, capabilities, source_ref, grant_version, status, expires_at, created_at, updated_at
FROM contest_access_grant
WHERE tenant_id = $1 AND id = $2;

-- name: GetContestAccessGrantForSubject :one
SELECT id, tenant_id, contest_id, team_id, sandbox_id, member_tenant_id, member_account_id, capabilities, source_ref, grant_version, status, expires_at, created_at, updated_at
FROM contest_access_grant
WHERE tenant_id = $1 AND sandbox_id = $2 AND member_tenant_id = $3 AND member_account_id = $4;

-- name: FindContestAccessGrantForSubject :one
-- M8 网关跨租户读取当前成员的唯一沙箱授权；只在受控特权事务中使用。
SELECT id, tenant_id, contest_id, team_id, sandbox_id, member_tenant_id, member_account_id, capabilities, source_ref, grant_version, status, expires_at, created_at, updated_at
FROM contest_access_grant
WHERE sandbox_id = $1 AND member_tenant_id = $2 AND member_account_id = $3;

-- name: UpsertContestAccessGrant :one
INSERT INTO contest_access_grant (id, tenant_id, contest_id, team_id, sandbox_id, member_tenant_id, member_account_id, capabilities, source_ref, grant_version, status, expires_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1, 1, $10, now(), now())
ON CONFLICT (tenant_id, sandbox_id, member_tenant_id, member_account_id) DO UPDATE
SET capabilities = EXCLUDED.capabilities,
    source_ref = EXCLUDED.source_ref,
    grant_version = contest_access_grant.grant_version + 1,
    status = 1,
    expires_at = EXCLUDED.expires_at,
    updated_at = now()
RETURNING id, tenant_id, contest_id, team_id, sandbox_id, member_tenant_id, member_account_id, capabilities, source_ref, grant_version, status, expires_at, created_at, updated_at;

-- name: RevokeContestAccessGrantsForSandbox :exec
UPDATE contest_access_grant
SET status = 2, grant_version = grant_version + 1, updated_at = now()
WHERE tenant_id = $1 AND sandbox_id = $2 AND status = 1;

-- name: ListContestAccessGrantsForSandbox :many
SELECT id, tenant_id, contest_id, team_id, sandbox_id, member_tenant_id, member_account_id, capabilities, source_ref, grant_version, status, expires_at, created_at, updated_at
FROM contest_access_grant
WHERE tenant_id = $1 AND sandbox_id = $2
ORDER BY id;

-- name: ListContestAccessGrantsForContest :many
SELECT id, tenant_id, contest_id, team_id, sandbox_id, member_tenant_id, member_account_id, capabilities, source_ref, grant_version, status, expires_at, created_at, updated_at
FROM contest_access_grant
WHERE tenant_id = $1 AND contest_id = $2 AND status = 1
ORDER BY sandbox_id, id;

-- name: CreateSolveSubmission :one
INSERT INTO solve_submission (id, tenant_id, contest_id, problem_id, team_id, submitter_tenant_id, submitter_id, content_ref, source_ref, scope_ref, judge_task_ref, passed, score, sandbox_ref, submitted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, false, 0, $12, now())
RETURNING id, tenant_id, contest_id, problem_id, team_id, submitter_tenant_id, submitter_id, content_ref, source_ref, scope_ref, judge_task_ref, passed, score, sandbox_ref, submitted_at;

-- name: GetSolveSubmission :one
SELECT id, tenant_id, contest_id, problem_id, team_id, submitter_tenant_id, submitter_id, content_ref, source_ref, scope_ref, judge_task_ref, passed, score, sandbox_ref, submitted_at
FROM solve_submission
WHERE tenant_id = $1 AND id = $2;

-- name: FindSolveSubmissionTenant :one
SELECT tenant_id
FROM solve_submission
WHERE id = $1;

-- name: GetSolveSubmissionByJudgeTask :one
SELECT id, tenant_id, contest_id, problem_id, team_id, submitter_tenant_id, submitter_id, content_ref, source_ref, scope_ref, judge_task_ref, passed, score, sandbox_ref, submitted_at
FROM solve_submission
WHERE tenant_id = $1 AND judge_task_ref = $2;

-- name: UpdateSolveSubmissionResult :one
UPDATE solve_submission
SET passed = $3,
    score = $4
WHERE tenant_id = $1 AND id = $2
RETURNING id, tenant_id, contest_id, problem_id, team_id, submitter_tenant_id, submitter_id, content_ref, source_ref, scope_ref, judge_task_ref, passed, score, sandbox_ref, submitted_at;

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
SELECT lr.id, lr.tenant_id, lr.contest_id, lr.team_id, lr.score::float8 AS score, lr.solved_count, lr.last_solve_at, lr.rank, lr.updated_at
FROM ladder_rank lr
WHERE lr.tenant_id = $1 AND lr.contest_id = $2
  AND NOT EXISTS (
      SELECT 1 FROM cheat_record cr
      WHERE cr.tenant_id = lr.tenant_id
        AND cr.contest_id = lr.contest_id
        AND cr.team_id = lr.team_id
        AND cr.action = 3
  )
ORDER BY lr.rank ASC, lr.score DESC, lr.solved_count DESC, lr.last_solve_at ASC NULLS LAST, lr.team_id ASC
LIMIT $3 OFFSET $4;

-- name: GetLadderByTeam :one
SELECT id, tenant_id, contest_id, team_id, score::float8 AS score, solved_count, last_solve_at, rank, updated_at
FROM ladder_rank
WHERE tenant_id = $1 AND contest_id = $2 AND team_id = $3;

-- name: CountLadder :one
SELECT COUNT(*)::bigint
FROM ladder_rank lr
WHERE lr.tenant_id = $1 AND lr.contest_id = $2
  AND NOT EXISTS (
      SELECT 1 FROM cheat_record cr
      WHERE cr.tenant_id = lr.tenant_id
        AND cr.contest_id = lr.contest_id
        AND cr.team_id = lr.team_id
        AND cr.action = 3
  );

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
INSERT INTO battle_entry (id, tenant_id, contest_id, problem_id, team_id, role, artifact_ref, artifact_hash, version_no, is_active, submitted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, now())
RETURNING id, tenant_id, contest_id, problem_id, team_id, role, artifact_ref, artifact_hash, version_no, is_active, submitted_at;

-- name: ListBattleEntriesForTeam :many
SELECT id, tenant_id, contest_id, problem_id, team_id, role, artifact_ref, artifact_hash, version_no, is_active, submitted_at
FROM battle_entry
WHERE tenant_id = $1 AND contest_id = $2 AND team_id = $3
ORDER BY submitted_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountBattleEntriesForTeam :one
SELECT count(*)
FROM battle_entry
WHERE tenant_id = $1 AND contest_id = $2 AND team_id = $3;

-- name: GetBattleEntry :one
SELECT id, tenant_id, contest_id, problem_id, team_id, role, artifact_ref, artifact_hash, version_no, is_active, submitted_at
FROM battle_entry
WHERE tenant_id = $1 AND id = $2;

-- name: ListActiveBattleOpponents :many
WITH current_rank AS (
    SELECT COALESCE(score, $8::numeric) AS score
    FROM ladder_rank
    WHERE tenant_id = $1 AND contest_id = $2 AND team_id = $5
    LIMIT 1
)
SELECT be.id, be.tenant_id, be.contest_id, be.problem_id, be.team_id, be.role, be.artifact_ref, be.artifact_hash, be.version_no, be.is_active, be.submitted_at
FROM battle_entry be
LEFT JOIN ladder_rank lr ON lr.tenant_id = be.tenant_id AND lr.contest_id = be.contest_id AND lr.team_id = be.team_id
WHERE be.tenant_id = $1 AND be.contest_id = $2 AND be.problem_id = $3 AND be.is_active = true AND be.id <> $4 AND be.team_id <> $5
ORDER BY
    CASE WHEN $6::smallint = 2 THEN ABS(COALESCE(lr.score, $8::numeric) - COALESCE((SELECT score FROM current_rank), $8::numeric)) ELSE 0 END ASC,
    be.submitted_at ASC,
    be.id ASC
LIMIT $7;

-- name: CreateBattleMatch :one
INSERT INTO battle_match (id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, scope_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at, lease_token, lease_until)
SELECT $1, $2, $3, $4, $5, $6, $7, NULL, NULL, NULL, '{}'::jsonb, NULL, 1, now(), NULL, '', NULL
WHERE EXISTS (
    SELECT 1 FROM contest c
    WHERE c.tenant_id = $2 AND c.id = $3 AND c.status IN (3, 4) AND c.end_at > now()
)
RETURNING id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, scope_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at, lease_token, lease_until, attempt_count;

-- name: ExhaustUnstartedBattleMatches :many
-- 仅处理尚未提交 M3 的启动租约;已持有 judge_task_ref 的对局由 M3 生命周期收敛。
UPDATE battle_match AS m
SET status = 4, finished_at = now(), lease_token = '', lease_until = NULL
WHERE status = 2 AND COALESCE(judge_task_ref, '') = ''
  AND lease_until <= sqlc.arg(stale_before)::timestamptz
  AND attempt_count >= sqlc.arg(max_attempts)::int
RETURNING id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, scope_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at, lease_token, lease_until, attempt_count;

-- name: ClaimPendingBattleMatchesAcrossTenants :many
WITH candidates AS (
    SELECT m.id
    FROM battle_match m
    JOIN contest c ON c.tenant_id = m.tenant_id AND c.id = m.contest_id
    WHERE (c.status IN (3, 4) OR (c.status = 5 AND c.archive_lease_token = ''))
      AND m.attempt_count < sqlc.arg(max_attempts)::int
      AND (m.status = 1 OR (m.status = 2 AND COALESCE(m.judge_task_ref, '') = '' AND m.lease_until <= sqlc.arg(stale_before)::timestamptz))
    ORDER BY m.matched_at ASC
    LIMIT sqlc.arg(page_limit)::int
    FOR UPDATE OF m SKIP LOCKED
)
UPDATE battle_match m
SET status = 2,
    lease_token = sqlc.arg(lease_token),
    lease_until = sqlc.arg(lease_until)::timestamptz,
    attempt_count = m.attempt_count + 1
FROM candidates c
WHERE m.id = c.id
RETURNING m.id, m.tenant_id, m.contest_id, m.problem_id, m.entry_a_id, m.entry_b_id, m.source_ref, m.scope_ref, m.sandbox_ref, m.judge_task_ref, m.result, m.score_delta, m.replay_ref, m.status, m.matched_at, m.finished_at, m.lease_token, m.lease_until, m.attempt_count;

-- name: GetBattleMatch :one
SELECT id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, scope_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at, lease_token, lease_until, attempt_count
FROM battle_match
WHERE tenant_id = $1 AND id = $2;

-- name: GetBattleMatchByJudgeTask :one
SELECT id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, scope_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at, lease_token, lease_until, attempt_count
FROM battle_match
WHERE tenant_id = $1 AND judge_task_ref = $2;

-- name: ListRunningBattleMatchesWithJudgeTask :many
SELECT id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, scope_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at, lease_token, lease_until, attempt_count
FROM battle_match
WHERE status = 2 AND COALESCE(judge_task_ref, '') <> ''
ORDER BY matched_at ASC, id ASC
LIMIT $1;

-- name: StartBattleMatch :one
UPDATE battle_match AS m
SET sandbox_ref = $3,
    judge_task_ref = $4,
    status = 2,
    lease_token = '',
    lease_until = NULL
WHERE tenant_id = $1 AND id = $2 AND status = 2 AND lease_token = $5 AND lease_until > now()
RETURNING id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id, source_ref, scope_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref, status, matched_at, finished_at, lease_token, lease_until, attempt_count;

-- name: RenewBattleMatchStartLease :execrows
-- 启动沙箱和提交判题期间仅由当前 token 续租,避免长时准备被其他 worker 重领。
UPDATE battle_match
SET lease_until = sqlc.arg(lease_until)::timestamptz
WHERE tenant_id = $1
  AND id = $2
  AND status = 2
  AND COALESCE(judge_task_ref, '') = ''
  AND lease_token = $3
  AND lease_until > now();

-- name: ListBattleMatchesForTeam :many
-- 师生同一查询按视角过滤:传 team_id 只回该队参与的对局(学生视角),
-- 传 0 回本赛事全部对局(组织者监控视角)。不为教师另开一条同义查询。
SELECT m.id, m.tenant_id, m.contest_id, m.problem_id, m.entry_a_id, m.entry_b_id, m.source_ref, m.scope_ref, m.sandbox_ref, m.judge_task_ref, m.result, m.score_delta, m.replay_ref, m.status, m.matched_at, m.finished_at, m.lease_token, m.lease_until, m.attempt_count
FROM battle_match m
JOIN battle_entry a ON a.tenant_id = m.tenant_id AND a.id = m.entry_a_id
JOIN battle_entry b ON b.tenant_id = m.tenant_id AND b.id = m.entry_b_id
WHERE m.tenant_id = $1 AND m.contest_id = $2
  AND (sqlc.arg(team_id)::bigint = 0 OR a.team_id = sqlc.arg(team_id)::bigint OR b.team_id = sqlc.arg(team_id)::bigint)
ORDER BY m.matched_at DESC, m.id DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountBattleMatchesForTeam :one
SELECT count(*)::bigint
FROM battle_match m
JOIN battle_entry a ON a.tenant_id = m.tenant_id AND a.id = m.entry_a_id
JOIN battle_entry b ON b.tenant_id = m.tenant_id AND b.id = m.entry_b_id
WHERE m.tenant_id = $1 AND m.contest_id = $2
  AND (sqlc.arg(team_id)::bigint = 0 OR a.team_id = sqlc.arg(team_id)::bigint OR b.team_id = sqlc.arg(team_id)::bigint);

-- name: ListBattleReplayMatchesForTeam :many
-- 回放专用时间窗只读取已完成对局,并由服务端携带本队视角和当时生效的参战物。
WITH visible AS (
    SELECT
        m.id, m.tenant_id, m.contest_id, m.problem_id, m.entry_a_id, m.entry_b_id,
        m.source_ref, m.scope_ref, m.sandbox_ref, m.judge_task_ref, m.result, m.score_delta,
        m.replay_ref, m.status, m.matched_at, m.finished_at,
        ROW_NUMBER() OVER (ORDER BY m.finished_at ASC, m.id ASC)::bigint AS sequence_no,
        CASE WHEN a.team_id = sqlc.arg(team_id)::bigint THEN 'a' ELSE 'b' END AS my_side,
        active_entry.id AS active_entry_id,
        active_entry.role AS active_entry_role,
        active_entry.version_no AS active_entry_version_no,
        active_entry.submitted_at AS active_entry_submitted_at
    FROM battle_match m
    JOIN battle_entry a ON a.tenant_id = m.tenant_id AND a.id = m.entry_a_id
    JOIN battle_entry b ON b.tenant_id = m.tenant_id AND b.id = m.entry_b_id
    LEFT JOIN LATERAL (
        SELECT be.id, be.role, be.version_no, be.submitted_at
        FROM battle_entry be
        WHERE be.tenant_id = m.tenant_id
          AND be.contest_id = m.contest_id
          AND be.problem_id = m.problem_id
          AND be.team_id = sqlc.arg(team_id)::bigint
          AND be.submitted_at <= m.finished_at
        ORDER BY be.submitted_at DESC, be.version_no DESC, be.id DESC
        LIMIT 1
    ) active_entry ON true
    WHERE m.tenant_id = $1 AND m.contest_id = $2 AND m.status = 3
      AND (a.team_id = sqlc.arg(team_id)::bigint OR b.team_id = sqlc.arg(team_id)::bigint)
)
SELECT id, tenant_id, contest_id, problem_id, entry_a_id, entry_b_id,
       source_ref, scope_ref, sandbox_ref, judge_task_ref, result, score_delta, replay_ref,
       status, matched_at, finished_at, sequence_no, my_side,
       active_entry_id, active_entry_role, active_entry_version_no, active_entry_submitted_at
FROM visible
WHERE sequence_no > sqlc.arg(page_offset)::bigint
  AND sequence_no <= sqlc.arg(page_offset)::bigint + sqlc.arg(page_limit)::bigint
ORDER BY sequence_no ASC;

-- name: CountBattleReplayPendingForTeam :one
SELECT count(*)::bigint
FROM battle_match m
JOIN battle_entry a ON a.tenant_id = m.tenant_id AND a.id = m.entry_a_id
JOIN battle_entry b ON b.tenant_id = m.tenant_id AND b.id = m.entry_b_id
WHERE m.tenant_id = $1 AND m.contest_id = $2 AND m.status IN (1, 2)
  AND (a.team_id = sqlc.arg(team_id)::bigint OR b.team_id = sqlc.arg(team_id)::bigint);

-- name: CountBattleReplayCompletedForTeam :one
SELECT count(*)::bigint
FROM battle_match m
JOIN battle_entry a ON a.tenant_id = m.tenant_id AND a.id = m.entry_a_id
JOIN battle_entry b ON b.tenant_id = m.tenant_id AND b.id = m.entry_b_id
WHERE m.tenant_id = $1 AND m.contest_id = $2 AND m.status = 3
  AND (a.team_id = sqlc.arg(team_id)::bigint OR b.team_id = sqlc.arg(team_id)::bigint);

-- name: GetBattleReplayCheckpointForTeam :one
WITH visible AS (
    SELECT m.result, m.score_delta, m.finished_at, m.id,
           ROW_NUMBER() OVER (ORDER BY m.finished_at ASC, m.id ASC)::bigint AS sequence_no,
           CASE WHEN a.team_id = sqlc.arg(team_id)::bigint THEN 'a' ELSE 'b' END AS my_side
    FROM battle_match m
    JOIN battle_entry a ON a.tenant_id = m.tenant_id AND a.id = m.entry_a_id
    JOIN battle_entry b ON b.tenant_id = m.tenant_id AND b.id = m.entry_b_id
    WHERE m.tenant_id = $1 AND m.contest_id = $2 AND m.status = 3
      AND (a.team_id = sqlc.arg(team_id)::bigint OR b.team_id = sqlc.arg(team_id)::bigint)
)
SELECT
    count(*) FILTER (WHERE (my_side = 'a' AND result = 1) OR (my_side = 'b' AND result = 2))::int AS wins,
    count(*) FILTER (WHERE (my_side = 'a' AND result = 2) OR (my_side = 'b' AND result = 1))::int AS losses,
    count(*) FILTER (WHERE result = 3)::int AS draws,
    COALESCE(SUM(CASE
        WHEN my_side = 'a' THEN COALESCE((score_delta->>'delta_a')::float8, 0)
        ELSE COALESCE((score_delta->>'delta_b')::float8, 0)
    END), 0)::float8 AS rating_delta,
    COALESCE((
        SELECT CASE
            WHEN v.my_side = 'a' THEN COALESCE((v.score_delta->>'rating_a_after')::float8, 0)
            ELSE COALESCE((v.score_delta->>'rating_b_after')::float8, 0)
        END
        FROM visible v
        WHERE v.sequence_no <= sqlc.arg(page_offset)::bigint
        ORDER BY v.sequence_no DESC
        LIMIT 1
    ), 0)::float8 AS rating
FROM visible
WHERE sequence_no <= sqlc.arg(page_offset)::bigint;

-- name: ListActiveBattleSourceRefsForArchive :many
SELECT DISTINCT source_ref
FROM battle_match
WHERE tenant_id = $1
  AND contest_id = $2
  AND source_ref <> ''
  AND status IN (1, 2)
ORDER BY source_ref ASC;

-- name: FinishBattleMatch :one
UPDATE battle_match AS m
SET sandbox_ref = $3,
    judge_task_ref = $4,
    result = $5,
    score_delta = $6,
    replay_ref = $7,
    status = 3,
    finished_at = now(), lease_token = '', lease_until = NULL
WHERE m.tenant_id = $1 AND m.id = $2 AND m.status = 2 AND m.judge_task_ref = $4
  AND EXISTS (
      SELECT 1 FROM contest c
      WHERE c.tenant_id = m.tenant_id AND c.id = m.contest_id
        AND (c.status IN (3, 4) OR (c.status = 5 AND c.archive_lease_token = ''))
  )
RETURNING m.id, m.tenant_id, m.contest_id, m.problem_id, m.entry_a_id, m.entry_b_id, m.source_ref, m.scope_ref, m.sandbox_ref, m.judge_task_ref, m.result, m.score_delta, m.replay_ref, m.status, m.matched_at, m.finished_at, m.lease_token, m.lease_until, m.attempt_count;

-- name: FailBattleMatchStart :one
UPDATE battle_match AS m
SET status = 4, finished_at = now(), lease_token = '', lease_until = NULL
WHERE m.tenant_id = $1 AND m.id = $2 AND m.status = 2 AND m.lease_token = $3 AND m.lease_until > now()
  AND COALESCE(m.judge_task_ref, '') = ''
RETURNING m.id, m.tenant_id, m.contest_id, m.problem_id, m.entry_a_id, m.entry_b_id, m.source_ref, m.scope_ref, m.sandbox_ref, m.judge_task_ref, m.result, m.score_delta, m.replay_ref, m.status, m.matched_at, m.finished_at, m.lease_token, m.lease_until, m.attempt_count;

-- name: FailBattleMatchByJudgeTask :one
UPDATE battle_match AS m
SET status = 4, finished_at = now(), lease_token = '', lease_until = NULL
WHERE m.tenant_id = $1 AND m.id = $2 AND m.status = 2 AND m.judge_task_ref = $3
  AND EXISTS (
      SELECT 1 FROM contest c
      WHERE c.tenant_id = m.tenant_id AND c.id = m.contest_id
        AND (c.status IN (3, 4) OR (c.status = 5 AND c.archive_lease_token = ''))
  )
RETURNING m.id, m.tenant_id, m.contest_id, m.problem_id, m.entry_a_id, m.entry_b_id, m.source_ref, m.scope_ref, m.sandbox_ref, m.judge_task_ref, m.result, m.score_delta, m.replay_ref, m.status, m.matched_at, m.finished_at, m.lease_token, m.lease_until, m.attempt_count;

-- name: UpsertLadderSnapshot :one
INSERT INTO contest_ladder_snapshot (id, tenant_id, contest_id, snapshot_status, ranking, generated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (tenant_id, contest_id, snapshot_status) DO UPDATE
SET ranking = EXCLUDED.ranking,
    generated_at = now()
RETURNING id, tenant_id, contest_id, snapshot_status, ranking, generated_at;

-- name: GetLadderSnapshot :one
SELECT id, tenant_id, contest_id, snapshot_status, ranking, generated_at
FROM contest_ladder_snapshot
WHERE tenant_id = $1 AND contest_id = $2 AND snapshot_status = $3;

-- name: ListStudentContestRecords :many
SELECT c.id AS contest_id, t.id AS team_id, COALESCE(l.score::float8, 0)::float8 AS score, COALESCE(l.rank, 0)::int AS rank, c.name AS contest_name, c.status AS contest_status
FROM team_member tm
JOIN team t ON t.tenant_id = tm.tenant_id AND t.id = tm.team_id
JOIN contest c ON c.tenant_id = t.tenant_id AND c.id = t.contest_id
LEFT JOIN ladder_rank l ON l.tenant_id = t.tenant_id AND l.contest_id = t.contest_id AND l.team_id = t.id
WHERE tm.member_tenant_id = $1 AND tm.account_id = $2 AND c.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM cheat_record cr
      WHERE cr.tenant_id = t.tenant_id
        AND cr.contest_id = t.contest_id
        AND cr.team_id = t.id
        AND cr.action = 3
  )
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

-- name: CountCheatRecords :one
SELECT count(*)::bigint
FROM cheat_record
WHERE tenant_id = $1 AND contest_id = $2;

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
    prevalidate_status = 1,
    prevalidate_detail = '{}'::jsonb,
    composition_digest = NULL,
    composition_snapshot = NULL,
    init_code_ref = NULL,
    init_script_ref = NULL,
    content_item_code = NULL,
    content_item_version = NULL,
    status = 1,
    updated_at = now()
RETURNING id, tenant_id, source_id, external_ref, title, level, runtime_mode, draft_body, prevalidate_status, prevalidate_detail, content_item_code, content_item_version, status, created_at, updated_at, composition_digest, composition_snapshot, init_code_ref, init_script_ref;

-- name: GetVulnProblem :one
SELECT id, tenant_id, source_id, external_ref, title, level, runtime_mode, draft_body, prevalidate_status, prevalidate_detail, content_item_code, content_item_version, status, created_at, updated_at, composition_digest, composition_snapshot, init_code_ref, init_script_ref
FROM vuln_problem
WHERE tenant_id = $1 AND id = $2;

-- name: ListVulnProblems :many
SELECT id, tenant_id, source_id, external_ref, title, level, runtime_mode, draft_body, prevalidate_status, prevalidate_detail, content_item_code, content_item_version, status, created_at, updated_at, composition_digest, composition_snapshot, init_code_ref, init_script_ref
FROM vuln_problem
WHERE tenant_id = sqlc.arg(tenant_id)::bigint
  AND (sqlc.arg(source_id)::bigint = 0 OR source_id = sqlc.arg(source_id)::bigint)
  AND (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
  AND (sqlc.arg(prevalidate_status)::smallint = 0 OR prevalidate_status = sqlc.arg(prevalidate_status)::smallint)
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountVulnProblems :one
SELECT count(*)::bigint
FROM vuln_problem
WHERE tenant_id = sqlc.arg(tenant_id)::bigint
  AND (sqlc.arg(source_id)::bigint = 0 OR source_id = sqlc.arg(source_id)::bigint)
  AND (sqlc.arg(status)::smallint = 0 OR status = sqlc.arg(status)::smallint)
  AND (sqlc.arg(prevalidate_status)::smallint = 0 OR prevalidate_status = sqlc.arg(prevalidate_status)::smallint);

-- name: SetVulnProblemPrevalidate :one
UPDATE vuln_problem
SET prevalidate_status = $3,
    prevalidate_detail = $4,
    composition_digest = $5,
    composition_snapshot = $6,
    init_code_ref = $7,
    init_script_ref = $8,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status = 1
RETURNING id, tenant_id, source_id, external_ref, title, level, runtime_mode, draft_body, prevalidate_status, prevalidate_detail, content_item_code, content_item_version, status, created_at, updated_at, composition_digest, composition_snapshot, init_code_ref, init_script_ref;

-- name: FinalizeVulnProblem :one
UPDATE vuln_problem
SET content_item_code = $3,
    content_item_version = $4,
    status = 2,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND prevalidate_status = 2 AND status = 1
RETURNING id, tenant_id, source_id, external_ref, title, level, runtime_mode, draft_body, prevalidate_status, prevalidate_detail, content_item_code, content_item_version, status, created_at, updated_at, composition_digest, composition_snapshot, init_code_ref, init_script_ref;

-- name: ContestStats :one
SELECT
    COUNT(*)::bigint AS contest_count,
    COALESCE(SUM(CASE WHEN status IN (2, 3, 4) THEN 1 ELSE 0 END), 0)::bigint AS active_contest_count,
    COALESCE((SELECT COUNT(DISTINCT tm.member_tenant_id::text || ':' || tm.account_id::text)::bigint FROM team_member tm WHERE tm.tenant_id = $1), 0)::bigint AS participant_count
FROM contest c
WHERE c.tenant_id = $1 AND c.deleted_at IS NULL;

-- name: ClaimAutoArchiveContestsAcrossTenants :many
-- 归档只在没有待执行/执行中对局时取得租约;状态改为已结束后不再接受新对局。
WITH candidates AS (
    SELECT c.id
    FROM contest c
    WHERE c.deleted_at IS NULL
      AND ((c.status IN (3, 4) AND c.end_at <= now())
        OR (c.status = 5 AND (c.archive_lease_until IS NULL OR c.archive_lease_until <= sqlc.arg(stale_before)::timestamptz)))
      AND NOT EXISTS (
          SELECT 1 FROM battle_match m
          WHERE m.tenant_id = c.tenant_id AND m.contest_id = c.id AND m.status IN (1, 2)
      )
    ORDER BY c.end_at ASC
    LIMIT sqlc.arg(page_limit)::int
    FOR UPDATE OF c SKIP LOCKED
)
UPDATE contest c
SET status = 5,
    archive_lease_token = sqlc.arg(lease_token),
    archive_lease_until = sqlc.arg(lease_until)::timestamptz,
    updated_at = now()
FROM candidates x
WHERE c.id = x.id
RETURNING c.id, c.tenant_id, c.created_at, c.archive_lease_token, c.archive_lease_until;
