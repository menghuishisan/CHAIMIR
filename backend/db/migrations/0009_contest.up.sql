-- 迁移 0009:M8 竞赛 —— 赛事、报名队伍、解题/对抗、排行、快照、作弊与漏洞源租户表。
-- 依据 docs/08-竞赛/03-数据模型.md。M8 只编排 M2/M3/M5,不保存题目正本和引擎正文。

CREATE TABLE contest (
    id             BIGINT PRIMARY KEY,
    tenant_id      BIGINT       NOT NULL,
    organizer_id   BIGINT       NOT NULL,
    name           VARCHAR(255) NOT NULL,
    mode           SMALLINT     NOT NULL,
    match_mode     SMALLINT     NULL,
    team_mode      SMALLINT     NOT NULL,
    signup_start   TIMESTAMPTZ  NOT NULL,
    signup_end     TIMESTAMPTZ  NOT NULL,
    start_at       TIMESTAMPTZ  NOT NULL,
    end_at         TIMESTAMPTZ  NOT NULL,
    freeze_minutes INT          NOT NULL DEFAULT 0,
    rules          JSONB        NOT NULL DEFAULT '{}'::jsonb,
    status         SMALLINT     NOT NULL,
    deleted_at     TIMESTAMPTZ  NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_contest_status ON contest (tenant_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_contest_organizer ON contest (tenant_id, organizer_id) WHERE deleted_at IS NULL;

CREATE TABLE contest_problem (
    id            BIGINT PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL,
    contest_id    BIGINT      NOT NULL,
    item_code     VARCHAR(96) NOT NULL,
    item_version  VARCHAR(32) NOT NULL,
    score         INT         NOT NULL,
    dynamic_score JSONB       NOT NULL DEFAULT '{}'::jsonb,
    battle_rule   SMALLINT    NULL,
    seq           INT         NOT NULL
);
CREATE INDEX idx_contest_problem_contest ON contest_problem (tenant_id, contest_id);

CREATE TABLE team (
    id          BIGINT PRIMARY KEY,
    tenant_id   BIGINT       NOT NULL,
    contest_id  BIGINT       NOT NULL,
    name        VARCHAR(128) NOT NULL,
    invite_code VARCHAR(16)  NULL,
    status      SMALLINT     NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_team_contest ON team (tenant_id, contest_id);

CREATE TABLE team_member (
    id               BIGINT PRIMARY KEY,
    tenant_id        BIGINT  NOT NULL,
    team_id          BIGINT  NOT NULL,
    account_id       BIGINT  NOT NULL,
    member_tenant_id BIGINT  NOT NULL,
    is_leader        BOOLEAN NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX uk_team_member_account ON team_member (tenant_id, team_id, account_id);
CREATE INDEX idx_team_member_account ON team_member (tenant_id, account_id);

CREATE TABLE solve_submission (
    id             BIGINT PRIMARY KEY,
    tenant_id      BIGINT      NOT NULL,
    contest_id     BIGINT      NOT NULL,
    problem_id     BIGINT      NOT NULL,
    team_id        BIGINT      NOT NULL,
    submitter_id   BIGINT      NOT NULL,
    content_ref    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    source_ref     VARCHAR(96) NOT NULL,
    judge_task_ref VARCHAR(64) NULL,
    passed         BOOLEAN     NOT NULL DEFAULT false,
    score          INT         NOT NULL DEFAULT 0,
    sandbox_ref    VARCHAR(64) NULL,
    submitted_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_solve_submission_problem ON solve_submission (tenant_id, contest_id, team_id, problem_id);
CREATE UNIQUE INDEX uk_solve_submission_source_ref ON solve_submission (tenant_id, source_ref);
CREATE INDEX idx_solve_submission_judge_task ON solve_submission (tenant_id, judge_task_ref);

CREATE TABLE battle_entry (
    id           BIGINT PRIMARY KEY,
    tenant_id    BIGINT       NOT NULL,
    contest_id   BIGINT       NOT NULL,
    team_id      BIGINT       NOT NULL,
    role         SMALLINT     NOT NULL,
    artifact_ref VARCHAR(255) NOT NULL,
    version_no   INT          NOT NULL,
    is_active    BOOLEAN      NOT NULL DEFAULT true,
    submitted_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_battle_entry_active ON battle_entry (tenant_id, contest_id, team_id, is_active);

CREATE TABLE battle_match (
    id           BIGINT PRIMARY KEY,
    tenant_id    BIGINT       NOT NULL,
    contest_id   BIGINT       NOT NULL,
    entry_a_id   BIGINT       NOT NULL,
    entry_b_id   BIGINT       NOT NULL,
    sandbox_ref  VARCHAR(64)  NOT NULL DEFAULT '',
    result       SMALLINT     NOT NULL,
    score_delta  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    replay_ref   VARCHAR(255) NOT NULL DEFAULT '',
    matched_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    finished_at  TIMESTAMPTZ  NULL
);
CREATE INDEX idx_battle_match_contest ON battle_match (tenant_id, contest_id);

CREATE TABLE ladder_rank (
    id            BIGINT PRIMARY KEY,
    tenant_id     BIGINT        NOT NULL,
    contest_id    BIGINT        NOT NULL,
    team_id       BIGINT        NOT NULL,
    score         NUMERIC(10,2) NOT NULL DEFAULT 0,
    solved_count  INT           NOT NULL DEFAULT 0,
    last_solve_at TIMESTAMPTZ   NULL,
    rank          INT           NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_ladder_rank_team ON ladder_rank (tenant_id, contest_id, team_id);
CREATE INDEX idx_ladder_rank_rank ON ladder_rank (tenant_id, contest_id, rank);

CREATE TABLE contest_result_snapshot (
    id            BIGINT PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL,
    contest_id    BIGINT      NOT NULL,
    final_ranking JSONB       NOT NULL DEFAULT '[]'::jsonb,
    generated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_contest_result_snapshot ON contest_result_snapshot (tenant_id, contest_id);

CREATE TABLE cheat_record (
    id          BIGINT PRIMARY KEY,
    tenant_id   BIGINT      NOT NULL,
    contest_id  BIGINT      NOT NULL,
    team_id     BIGINT      NOT NULL,
    type        SMALLINT    NOT NULL,
    evidence    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    action      SMALLINT    NOT NULL,
    operator_id BIGINT      NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_cheat_record_contest ON cheat_record (tenant_id, contest_id);

CREATE TABLE vuln_source (
    id            BIGINT PRIMARY KEY,
    tenant_id     BIGINT       NULL,
    type          SMALLINT     NOT NULL,
    name          VARCHAR(128) NOT NULL,
    config        JSONB        NOT NULL DEFAULT '{}'::jsonb,
    default_level SMALLINT     NOT NULL,
    enabled       BOOLEAN      NOT NULL DEFAULT true,
    last_sync_at  TIMESTAMPTZ  NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_vuln_source_enabled ON vuln_source (tenant_id, enabled);

CREATE TABLE vuln_problem (
    id                 BIGINT PRIMARY KEY,
    tenant_id          BIGINT       NOT NULL,
    source_id          BIGINT       NULL,
    external_ref       VARCHAR(128) NULL,
    title              VARCHAR(255) NOT NULL,
    level              SMALLINT     NOT NULL,
    runtime_mode       SMALLINT     NOT NULL,
    draft_body         JSONB        NOT NULL DEFAULT '{}'::jsonb,
    prevalidate_status SMALLINT     NOT NULL,
    prevalidate_detail JSONB        NOT NULL DEFAULT '{}'::jsonb,
    content_item_code  VARCHAR(96)  NULL,
    content_item_version VARCHAR(32) NULL,
    status             SMALLINT     NOT NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_vuln_problem_source ON vuln_problem (tenant_id, source_id);
CREATE INDEX idx_vuln_problem_status ON vuln_problem (tenant_id, status, prevalidate_status);

CREATE TRIGGER trg_contest_updated BEFORE UPDATE ON contest FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_vuln_source_updated BEFORE UPDATE ON vuln_source FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_vuln_problem_updated BEFORE UPDATE ON vuln_problem FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DO $$
DECLARE
    t TEXT;
    tenant_tables TEXT[] := ARRAY[
        'contest','contest_problem','team','team_member','solve_submission','battle_entry','battle_match',
        'ladder_rank','contest_result_snapshot','cheat_record','vuln_source','vuln_problem'
    ];
BEGIN
    FOREACH t IN ARRAY tenant_tables LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        IF t = 'vuln_source' THEN
            EXECUTE format($f$
                CREATE POLICY tenant_isolation ON %I
                    USING (tenant_id IS NULL OR tenant_id = current_setting('app.tenant_id')::BIGINT)
                    WITH CHECK (tenant_id IS NULL OR tenant_id = current_setting('app.tenant_id')::BIGINT)
            $f$, t);
        ELSE
            EXECUTE format($f$
                CREATE POLICY tenant_isolation ON %I
                    USING (tenant_id = current_setting('app.tenant_id')::BIGINT)
                    WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT)
            $f$, t);
        END IF;
    END LOOP;
END $$;
