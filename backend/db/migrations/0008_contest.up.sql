-- 0008_contest.up.sql 创建 M8 竞赛模块自有表、索引和租户级 RLS 策略。
CREATE TABLE IF NOT EXISTS contest (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    organizer_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    mode SMALLINT NOT NULL CHECK (mode > 0),
    match_mode SMALLINT CHECK (match_mode IS NULL OR match_mode IN (1, 2)),
    team_mode SMALLINT NOT NULL CHECK (team_mode IN (1, 2)),
    signup_start TIMESTAMPTZ NOT NULL,
    signup_end TIMESTAMPTZ NOT NULL,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL,
    freeze_minutes INT NOT NULL DEFAULT 0 CHECK (freeze_minutes >= 0),
    rules JSONB NOT NULL DEFAULT '{}'::jsonb,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3, 4, 5, 6)),
    archive_lease_token VARCHAR(64) NOT NULL DEFAULT '',
    archive_lease_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, organizer_id) REFERENCES account(tenant_id, id),
    CHECK (signup_start < signup_end AND signup_end <= start_at AND start_at < end_at)
);

CREATE TABLE IF NOT EXISTS contest_problem (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    contest_id BIGINT NOT NULL,
    item_code VARCHAR(96) NOT NULL,
    item_version VARCHAR(32) NOT NULL,
    score INT NOT NULL CHECK (score > 0),
    dynamic_score JSONB CHECK (
        dynamic_score IS NULL OR (
            jsonb_typeof(dynamic_score) = 'object'
            AND dynamic_score ?& ARRAY['min_score', 'decay_per_solve']
            AND dynamic_score - ARRAY['min_score', 'decay_per_solve'] = '{}'::jsonb
            AND jsonb_typeof(dynamic_score->'min_score') = 'number'
            AND jsonb_typeof(dynamic_score->'decay_per_solve') = 'number'
        )
    ),
    battle_config JSONB CHECK (
        battle_config IS NULL OR (
            jsonb_typeof(battle_config) = 'object'
            AND battle_config ?& ARRAY['execution_profile', 'entry_roles', 'replay_profile']
            AND battle_config - ARRAY['execution_profile', 'entry_roles', 'replay_profile'] = '{}'::jsonb
            AND jsonb_typeof(battle_config->'execution_profile') = 'object'
            AND jsonb_typeof(battle_config->'entry_roles') = 'array'
            AND jsonb_typeof(battle_config->'replay_profile') = 'object'
        )
    ),
    battle_rule SMALLINT CHECK (battle_rule IS NULL OR battle_rule > 0),
    seq INT NOT NULL DEFAULT 0,
    composition_digest VARCHAR(128),
    composition_snapshot JSONB,
    CHECK ((composition_digest IS NULL AND composition_snapshot IS NULL) OR (composition_digest IS NOT NULL AND composition_snapshot IS NOT NULL)),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, contest_id, item_code, item_version),
    FOREIGN KEY (tenant_id, contest_id) REFERENCES contest(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS team (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    contest_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    invite_code VARCHAR(16),
    status SMALLINT NOT NULL CHECK (status IN (1, 2)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, contest_id, invite_code),
    FOREIGN KEY (tenant_id, contest_id) REFERENCES contest(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS team_member (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    team_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    member_tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    is_leader BOOLEAN NOT NULL DEFAULT false,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, team_id, member_tenant_id, account_id),
    FOREIGN KEY (tenant_id, team_id) REFERENCES team(tenant_id, id),
    FOREIGN KEY (member_tenant_id, account_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS contest_access_grant (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    contest_id BIGINT NOT NULL,
    team_id BIGINT NOT NULL,
    sandbox_id BIGINT NOT NULL,
    member_tenant_id BIGINT NOT NULL,
    member_account_id BIGINT NOT NULL,
    capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    source_ref VARCHAR(128) NOT NULL,
    grant_version BIGINT NOT NULL DEFAULT 1 CHECK (grant_version > 0),
    status SMALLINT NOT NULL CHECK (status IN (1, 2)),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, sandbox_id, member_tenant_id, member_account_id),
    FOREIGN KEY (tenant_id, contest_id) REFERENCES contest(tenant_id, id),
    FOREIGN KEY (tenant_id, team_id) REFERENCES team(tenant_id, id),
    FOREIGN KEY (tenant_id, team_id, member_tenant_id, member_account_id) REFERENCES team_member(tenant_id, team_id, member_tenant_id, account_id)
);

CREATE TABLE IF NOT EXISTS solve_submission (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    contest_id BIGINT NOT NULL,
    problem_id BIGINT NOT NULL,
    team_id BIGINT NOT NULL,
    submitter_tenant_id BIGINT NOT NULL,
    submitter_id BIGINT NOT NULL,
    content_ref JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_ref VARCHAR(128) NOT NULL,
    scope_ref VARCHAR(128) NOT NULL,
    judge_task_ref VARCHAR(64),
    passed BOOLEAN NOT NULL DEFAULT false,
    score INT NOT NULL DEFAULT 0 CHECK (score >= 0),
    sandbox_ref VARCHAR(64),
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, source_ref),
    UNIQUE (tenant_id, scope_ref),
    FOREIGN KEY (tenant_id, contest_id) REFERENCES contest(tenant_id, id),
    FOREIGN KEY (tenant_id, problem_id) REFERENCES contest_problem(tenant_id, id),
    FOREIGN KEY (tenant_id, team_id) REFERENCES team(tenant_id, id),
    FOREIGN KEY (submitter_tenant_id, submitter_id) REFERENCES account(tenant_id, id),
    FOREIGN KEY (tenant_id, team_id, submitter_tenant_id, submitter_id) REFERENCES team_member(tenant_id, team_id, member_tenant_id, account_id)
);

CREATE TABLE IF NOT EXISTS battle_entry (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    contest_id BIGINT NOT NULL,
    problem_id BIGINT NOT NULL,
    team_id BIGINT NOT NULL,
    role SMALLINT NOT NULL CHECK (role IN (0, 1, 2)),
    artifact_ref VARCHAR(255) NOT NULL,
    artifact_hash VARCHAR(64) NOT NULL,
    version_no INT NOT NULL CHECK (version_no > 0),
    is_active BOOLEAN NOT NULL DEFAULT true,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, contest_id, problem_id, team_id, role, version_no),
    FOREIGN KEY (tenant_id, contest_id) REFERENCES contest(tenant_id, id),
    FOREIGN KEY (tenant_id, problem_id) REFERENCES contest_problem(tenant_id, id),
    FOREIGN KEY (tenant_id, team_id) REFERENCES team(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS battle_match (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    contest_id BIGINT NOT NULL,
    problem_id BIGINT NOT NULL,
    entry_a_id BIGINT NOT NULL,
    entry_b_id BIGINT NOT NULL,
    source_ref VARCHAR(128) NOT NULL,
    scope_ref VARCHAR(128) NOT NULL,
    sandbox_ref VARCHAR(64),
    judge_task_ref VARCHAR(64),
    result SMALLINT CHECK (result IS NULL OR result IN (1, 2, 3)),
    score_delta JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (
        score_delta = '{}'::jsonb OR (
            jsonb_typeof(score_delta) = 'object'
            AND score_delta ?& ARRAY[
                'team_a', 'team_b', 'rating_a_before', 'rating_b_before', 'rating_a_after',
                'rating_b_after', 'delta_a', 'delta_b', 'k_factor', 'result'
            ]
            AND score_delta - ARRAY[
                'team_a', 'team_b', 'rating_a_before', 'rating_b_before', 'rating_a_after',
                'rating_b_after', 'delta_a', 'delta_b', 'k_factor', 'result'
            ] = '{}'::jsonb
            AND jsonb_typeof(score_delta->'team_a') = 'string'
            AND jsonb_typeof(score_delta->'team_b') = 'string'
            AND score_delta->>'team_a' ~ '^[1-9][0-9]*$'
            AND score_delta->>'team_b' ~ '^[1-9][0-9]*$'
            AND jsonb_typeof(score_delta->'rating_a_before') = 'number'
            AND jsonb_typeof(score_delta->'rating_b_before') = 'number'
            AND jsonb_typeof(score_delta->'rating_a_after') = 'number'
            AND jsonb_typeof(score_delta->'rating_b_after') = 'number'
            AND jsonb_typeof(score_delta->'delta_a') = 'number'
            AND jsonb_typeof(score_delta->'delta_b') = 'number'
            AND jsonb_typeof(score_delta->'k_factor') = 'number'
            AND jsonb_typeof(score_delta->'result') = 'number'
            AND score_delta->>'result' IN ('1', '2', '3')
        )
    ),
    replay_ref VARCHAR(255),
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3, 4)),
    matched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    lease_token VARCHAR(64) NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ,
    attempt_count INT NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, source_ref),
    UNIQUE (tenant_id, scope_ref),
    FOREIGN KEY (tenant_id, contest_id) REFERENCES contest(tenant_id, id),
    FOREIGN KEY (tenant_id, problem_id) REFERENCES contest_problem(tenant_id, id),
    FOREIGN KEY (tenant_id, entry_a_id) REFERENCES battle_entry(tenant_id, id),
    FOREIGN KEY (tenant_id, entry_b_id) REFERENCES battle_entry(tenant_id, id)
);
CREATE INDEX IF NOT EXISTS idx_battle_match_lease ON battle_match(status, lease_until) WHERE status = 2;
CREATE INDEX IF NOT EXISTS idx_contest_archive_lease ON contest(status, archive_lease_until) WHERE status = 5;

CREATE TABLE IF NOT EXISTS ladder_rank (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    contest_id BIGINT NOT NULL,
    team_id BIGINT NOT NULL,
    score NUMERIC(10,2) NOT NULL DEFAULT 0,
    solved_count INT NOT NULL DEFAULT 0 CHECK (solved_count >= 0),
    last_solve_at TIMESTAMPTZ,
    rank INT NOT NULL DEFAULT 0 CHECK (rank >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, contest_id, team_id),
    FOREIGN KEY (tenant_id, contest_id) REFERENCES contest(tenant_id, id),
    FOREIGN KEY (tenant_id, team_id) REFERENCES team(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS contest_ladder_snapshot (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    contest_id BIGINT NOT NULL,
    snapshot_status SMALLINT NOT NULL CHECK (snapshot_status IN (4, 6)),
    ranking JSONB NOT NULL DEFAULT '[]'::jsonb,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, contest_id, snapshot_status),
    FOREIGN KEY (tenant_id, contest_id) REFERENCES contest(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS cheat_record (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    contest_id BIGINT NOT NULL,
    team_id BIGINT NOT NULL,
    type SMALLINT NOT NULL CHECK (type IN (1, 2, 3)),
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    action SMALLINT NOT NULL CHECK (action IN (1, 2, 3)),
    operator_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, contest_id) REFERENCES contest(tenant_id, id),
    FOREIGN KEY (tenant_id, team_id) REFERENCES team(tenant_id, id),
    FOREIGN KEY (tenant_id, operator_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS vuln_source (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT REFERENCES tenant(id),
    type SMALLINT NOT NULL CHECK (type IN (1, 2, 3)),
    name VARCHAR(128) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    default_level SMALLINT NOT NULL CHECK (default_level IN (1, 2, 3)),
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_sync_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS vuln_problem (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    source_id BIGINT,
    external_ref VARCHAR(128),
    title VARCHAR(255) NOT NULL,
    level SMALLINT NOT NULL CHECK (level IN (1, 2, 3)),
    runtime_mode SMALLINT NOT NULL CHECK (runtime_mode IN (1, 2)),
    draft_body JSONB NOT NULL DEFAULT '{}'::jsonb,
    prevalidate_status SMALLINT NOT NULL CHECK (prevalidate_status IN (1, 2, 3)),
    prevalidate_detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    composition_digest VARCHAR(128),
    composition_snapshot JSONB,
    init_code_ref VARCHAR(255),
    init_script_ref VARCHAR(255),
    CHECK ((composition_digest IS NULL AND composition_snapshot IS NULL) OR (composition_digest IS NOT NULL AND composition_snapshot IS NOT NULL)),
    content_item_code VARCHAR(96),
    content_item_version VARCHAR(32),
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_contest_status ON contest(tenant_id, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_contest_problem_contest ON contest_problem(tenant_id, contest_id);
CREATE INDEX IF NOT EXISTS idx_team_contest ON team(tenant_id, contest_id);
CREATE INDEX IF NOT EXISTS idx_team_invite ON team(tenant_id, invite_code);
CREATE INDEX IF NOT EXISTS idx_team_member_account ON team_member(tenant_id, member_tenant_id, account_id);
CREATE INDEX IF NOT EXISTS idx_contest_access_grant_subject ON contest_access_grant(tenant_id, member_tenant_id, member_account_id, status);
CREATE INDEX IF NOT EXISTS idx_contest_access_grant_sandbox ON contest_access_grant(tenant_id, sandbox_id, status);
CREATE INDEX IF NOT EXISTS idx_solve_submission_scope ON solve_submission(tenant_id, contest_id, team_id, problem_id);
CREATE INDEX IF NOT EXISTS idx_battle_entry_active ON battle_entry(tenant_id, contest_id, problem_id, team_id, is_active);
CREATE INDEX IF NOT EXISTS idx_battle_match_contest ON battle_match(tenant_id, contest_id, status);
CREATE INDEX IF NOT EXISTS idx_ladder_rank_order ON ladder_rank(tenant_id, contest_id, rank);
CREATE INDEX IF NOT EXISTS idx_cheat_record_contest ON cheat_record(tenant_id, contest_id, team_id);
CREATE INDEX IF NOT EXISTS idx_vuln_problem_source ON vuln_problem(tenant_id, source_id);
CREATE INDEX IF NOT EXISTS idx_vuln_problem_status ON vuln_problem(tenant_id, status, prevalidate_status);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_vuln_problem_source_external ON vuln_problem(tenant_id, COALESCE(source_id, 0), external_ref);

ALTER TABLE contest ENABLE ROW LEVEL SECURITY;
ALTER TABLE contest_problem ENABLE ROW LEVEL SECURITY;
ALTER TABLE team ENABLE ROW LEVEL SECURITY;
ALTER TABLE team_member ENABLE ROW LEVEL SECURITY;
ALTER TABLE contest_access_grant ENABLE ROW LEVEL SECURITY;
ALTER TABLE solve_submission ENABLE ROW LEVEL SECURITY;
ALTER TABLE battle_entry ENABLE ROW LEVEL SECURITY;
ALTER TABLE battle_match ENABLE ROW LEVEL SECURITY;
ALTER TABLE ladder_rank ENABLE ROW LEVEL SECURITY;
ALTER TABLE contest_ladder_snapshot ENABLE ROW LEVEL SECURITY;
ALTER TABLE cheat_record ENABLE ROW LEVEL SECURITY;
ALTER TABLE vuln_source ENABLE ROW LEVEL SECURITY;
ALTER TABLE vuln_problem ENABLE ROW LEVEL SECURITY;

CREATE POLICY contest_tenant_rls ON contest USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY contest_problem_tenant_rls ON contest_problem USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY team_tenant_rls ON team USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY team_member_tenant_rls ON team_member USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY contest_access_grant_tenant_rls ON contest_access_grant USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY solve_submission_tenant_rls ON solve_submission USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY battle_entry_tenant_rls ON battle_entry USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY battle_match_tenant_rls ON battle_match USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY ladder_rank_tenant_rls ON ladder_rank USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY contest_ladder_snapshot_tenant_rls ON contest_ladder_snapshot USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY cheat_record_tenant_rls ON cheat_record USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY vuln_source_tenant_rls ON vuln_source USING (tenant_id IS NULL OR tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY vuln_problem_tenant_rls ON vuln_problem USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
