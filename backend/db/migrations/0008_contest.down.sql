-- 0008_contest.down.sql 回滚 M8 竞赛模块自有表和 RLS 策略。
DROP POLICY IF EXISTS vuln_problem_tenant_rls ON vuln_problem;
DROP POLICY IF EXISTS vuln_source_tenant_rls ON vuln_source;
DROP POLICY IF EXISTS cheat_record_tenant_rls ON cheat_record;
DROP POLICY IF EXISTS contest_result_snapshot_tenant_rls ON contest_result_snapshot;
DROP POLICY IF EXISTS ladder_rank_tenant_rls ON ladder_rank;
DROP POLICY IF EXISTS battle_match_tenant_rls ON battle_match;
DROP POLICY IF EXISTS battle_entry_tenant_rls ON battle_entry;
DROP POLICY IF EXISTS solve_submission_tenant_rls ON solve_submission;
DROP POLICY IF EXISTS team_member_tenant_rls ON team_member;
DROP POLICY IF EXISTS team_tenant_rls ON team;
DROP POLICY IF EXISTS contest_problem_tenant_rls ON contest_problem;
DROP POLICY IF EXISTS contest_tenant_rls ON contest;

DROP INDEX IF EXISTS uniq_vuln_problem_source_external;

DROP TABLE IF EXISTS vuln_problem;
DROP TABLE IF EXISTS vuln_source;
DROP TABLE IF EXISTS cheat_record;
DROP TABLE IF EXISTS contest_result_snapshot;
DROP TABLE IF EXISTS ladder_rank;
DROP TABLE IF EXISTS battle_match;
DROP TABLE IF EXISTS battle_entry;
DROP TABLE IF EXISTS solve_submission;
DROP TABLE IF EXISTS team_member;
DROP TABLE IF EXISTS team;
DROP TABLE IF EXISTS contest_problem;
DROP TABLE IF EXISTS contest;
