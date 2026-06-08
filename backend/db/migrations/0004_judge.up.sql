-- 迁移 0004:M3 评测引擎 —— 判题器平台配置 + 判题任务/结果/指纹租户表。
-- 依据 docs/03-评测引擎/03-数据模型.md。
-- M3 只保存判题输入快照、结果与查重指纹;题目内容和答案正本归 M5。

CREATE TABLE judger (
    id                  BIGINT PRIMARY KEY,
    code                VARCHAR(64)  NOT NULL UNIQUE,
    name                VARCHAR(128) NOT NULL,
    type                SMALLINT     NOT NULL,
    executor_ref        VARCHAR(128) NOT NULL,
    runtime_required    BOOLEAN      NOT NULL DEFAULT true,
    default_timeout_sec INT          NOT NULL,
    resource_spec       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    selftest_status     SMALLINT     NOT NULL DEFAULT 1,
    selftest_detail     JSONB        NOT NULL DEFAULT '{}'::jsonb,
    status              SMALLINT     NOT NULL DEFAULT 2,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_judger_code ON judger (code);
CREATE INDEX idx_judger_status ON judger (status);

CREATE TABLE judge_task (
    id                 BIGINT PRIMARY KEY,
    tenant_id          BIGINT       NOT NULL,
    judger_id          BIGINT       NOT NULL REFERENCES judger(id),
    source_ref         VARCHAR(128) NOT NULL,
    submitter_id       BIGINT       NOT NULL,
    problem_ref        VARCHAR(128) NOT NULL,
    code_storage_key   VARCHAR(255) NOT NULL,
    code_hash          VARCHAR(64)  NOT NULL,
    input_snapshot     JSONB        NOT NULL DEFAULT '{}'::jsonb,
    sandbox_mode       SMALLINT     NOT NULL,
    target_sandbox_ref VARCHAR(64)  NULL,
    priority           SMALLINT     NOT NULL DEFAULT 2,
    status             SMALLINT     NOT NULL DEFAULT 1,
    retry_count        INT          NOT NULL DEFAULT 0,
    max_retries        INT          NOT NULL DEFAULT 2,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_judge_task_queue ON judge_task (tenant_id, status, priority, created_at);
CREATE UNIQUE INDEX uk_judge_task_source_ref ON judge_task (tenant_id, source_ref);
CREATE INDEX idx_judge_task_source ON judge_task (tenant_id, source_ref);
CREATE INDEX idx_judge_task_submitter ON judge_task (tenant_id, submitter_id);

CREATE TABLE judge_result (
    task_id           BIGINT      PRIMARY KEY REFERENCES judge_task(id),
    tenant_id         BIGINT      NOT NULL,
    passed            BOOLEAN     NOT NULL,
    score             INT         NOT NULL,
    max_score         INT         NOT NULL,
    details           JSONB       NOT NULL DEFAULT '{}'::jsonb,
    judge_sandbox_ref VARCHAR(128) NULL,
    judged_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_rejudge        BOOLEAN     NOT NULL DEFAULT false
);
CREATE INDEX idx_judge_result_tenant_time ON judge_result (tenant_id, judged_at DESC);

CREATE TABLE submission_fingerprint (
    id           BIGINT      PRIMARY KEY,
    tenant_id    BIGINT      NOT NULL,
    source_ref   VARCHAR(128) NOT NULL,
    problem_ref  VARCHAR(128) NOT NULL,
    submitter_id BIGINT      NOT NULL,
    code_hash    VARCHAR(64) NOT NULL,
    sim_vector   JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_submission_fingerprint_exact ON submission_fingerprint (tenant_id, problem_ref, code_hash);
CREATE INDEX idx_submission_fingerprint_source ON submission_fingerprint (tenant_id, source_ref);

CREATE TRIGGER trg_judger_updated BEFORE UPDATE ON judger
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_judge_task_updated BEFORE UPDATE ON judge_task
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DO $$
DECLARE
    t TEXT;
    tenant_tables TEXT[] := ARRAY[
        'judge_task','judge_result','submission_fingerprint'
    ];
BEGIN
    FOREACH t IN ARRAY tenant_tables LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format($f$
            CREATE POLICY tenant_isolation ON %I
                USING (tenant_id = current_setting('app.tenant_id')::BIGINT)
                WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT)
        $f$, t);
    END LOOP;
END $$;
