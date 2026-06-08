-- 迁移 0008:M7 实验 —— 实验定义、实例、协作、检查点与报告租户表。
-- 依据 docs/07-实验/02-数据模型.md。M7 只编排 M2/M3/M4/M5,不保存引擎正本数据。

CREATE TABLE experiment (
    id             BIGINT PRIMARY KEY,
    tenant_id      BIGINT       NOT NULL,
    course_id      BIGINT       NULL,
    author_id      BIGINT       NOT NULL,
    template_ref   VARCHAR(96)  NULL,
    template_version VARCHAR(32) NULL,
    name           VARCHAR(255) NOT NULL,
    description    TEXT         NOT NULL DEFAULT '',
    components     JSONB        NOT NULL DEFAULT '{}'::jsonb,
    collab_mode    SMALLINT     NOT NULL,
    group_config   JSONB        NOT NULL DEFAULT '{}'::jsonb,
    require_report BOOLEAN      NOT NULL DEFAULT false,
    wizard_step    SMALLINT     NOT NULL DEFAULT 1,
    status         SMALLINT     NOT NULL,
    deleted_at     TIMESTAMPTZ  NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_experiment_course_status ON experiment (tenant_id, course_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_experiment_author ON experiment (tenant_id, author_id) WHERE deleted_at IS NULL;

CREATE TABLE experiment_instance (
    id               BIGINT PRIMARY KEY,
    tenant_id        BIGINT      NOT NULL,
    experiment_id    BIGINT      NOT NULL,
    owner_account_id BIGINT      NOT NULL,
    group_id         BIGINT      NULL,
    source_ref       VARCHAR(96) NOT NULL,
    sandbox_refs     JSONB       NOT NULL DEFAULT '[]'::jsonb,
    sim_session_refs JSONB       NOT NULL DEFAULT '[]'::jsonb,
    status           SMALLINT    NOT NULL,
    score            NUMERIC(5,2) NULL,
    started_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at      TIMESTAMPTZ NULL,
    last_active_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_experiment_instance_owner ON experiment_instance (tenant_id, experiment_id, owner_account_id);
CREATE UNIQUE INDEX uk_experiment_instance_source_ref ON experiment_instance (tenant_id, source_ref);
CREATE INDEX idx_experiment_instance_status ON experiment_instance (tenant_id, status, last_active_at);
CREATE INDEX idx_experiment_instance_group ON experiment_instance (tenant_id, group_id);

CREATE TABLE experiment_group (
    id            BIGINT PRIMARY KEY,
    tenant_id     BIGINT       NOT NULL,
    experiment_id BIGINT       NOT NULL,
    name          VARCHAR(128) NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_experiment_group_experiment ON experiment_group (tenant_id, experiment_id);

CREATE TABLE group_member (
    id         BIGINT      PRIMARY KEY,
    tenant_id  BIGINT      NOT NULL,
    group_id   BIGINT      NOT NULL,
    student_id BIGINT      NOT NULL,
    role       VARCHAR(64) NOT NULL
);
CREATE UNIQUE INDEX uk_group_member_student ON group_member (tenant_id, group_id, student_id);
CREATE INDEX idx_group_member_student ON group_member (tenant_id, student_id);

CREATE TABLE checkpoint_result (
    id              BIGINT PRIMARY KEY,
    tenant_id       BIGINT       NOT NULL,
    instance_id     BIGINT       NOT NULL,
    checkpoint_id   VARCHAR(64)  NOT NULL,
    judge_task_ref  VARCHAR(64)  NULL,
    passed          BOOLEAN      NOT NULL DEFAULT false,
    score           NUMERIC(5,2) NOT NULL DEFAULT 0,
    detail_ref      VARCHAR(128) NULL,
    judged_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_checkpoint_result_instance_cp ON checkpoint_result (tenant_id, instance_id, checkpoint_id);
CREATE INDEX idx_checkpoint_result_instance ON checkpoint_result (tenant_id, instance_id);
CREATE INDEX idx_checkpoint_result_judge_task ON checkpoint_result (tenant_id, judge_task_ref);

CREATE TABLE experiment_report (
    id           BIGINT PRIMARY KEY,
    tenant_id    BIGINT       NOT NULL,
    instance_id  BIGINT       NOT NULL,
    student_id   BIGINT       NOT NULL,
    content_ref  VARCHAR(255) NOT NULL,
    manual_score NUMERIC(5,2) NULL,
    comment      TEXT         NULL,
    status       SMALLINT     NOT NULL,
    submitted_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_experiment_report_student ON experiment_report (tenant_id, instance_id, student_id);
CREATE INDEX idx_experiment_report_instance ON experiment_report (tenant_id, instance_id);

CREATE TRIGGER trg_experiment_updated BEFORE UPDATE ON experiment FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DO $$
DECLARE
    t TEXT;
    tenant_tables TEXT[] := ARRAY[
        'experiment','experiment_instance','experiment_group','group_member','checkpoint_result','experiment_report'
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
