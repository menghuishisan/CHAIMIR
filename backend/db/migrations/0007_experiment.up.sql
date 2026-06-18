-- 0007_experiment.up.sql 创建 M7 实验模块自有表、索引和租户级 RLS 策略。
CREATE TABLE IF NOT EXISTS experiment (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    course_id BIGINT,
    author_id BIGINT NOT NULL,
    template_ref VARCHAR(96),
    template_version VARCHAR(32),
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    components JSONB NOT NULL DEFAULT '{"envs":[],"sims":[],"checkpoints":[],"stages":[]}'::jsonb,
    collab_mode SMALLINT NOT NULL CHECK (collab_mode IN (1, 2)),
    group_config JSONB,
    require_report BOOLEAN NOT NULL DEFAULT false,
    wizard_step SMALLINT NOT NULL DEFAULT 1 CHECK (wizard_step BETWEEN 1 AND 6),
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, author_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS experiment_group (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    experiment_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, experiment_id) REFERENCES experiment(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS group_member (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    group_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    role VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, group_id, student_id),
    FOREIGN KEY (tenant_id, group_id) REFERENCES experiment_group(tenant_id, id),
    FOREIGN KEY (tenant_id, student_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS experiment_instance (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    experiment_id BIGINT NOT NULL,
    owner_account_id BIGINT NOT NULL,
    group_id BIGINT,
    source_ref VARCHAR(128) NOT NULL,
    sandbox_refs JSONB NOT NULL DEFAULT '[]'::jsonb,
    sim_session_refs JSONB NOT NULL DEFAULT '[]'::jsonb,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3, 4, 5, 6, 7)),
    score NUMERIC(5,2),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, source_ref),
    FOREIGN KEY (tenant_id, experiment_id) REFERENCES experiment(tenant_id, id),
    FOREIGN KEY (tenant_id, owner_account_id) REFERENCES account(tenant_id, id),
    FOREIGN KEY (tenant_id, group_id) REFERENCES experiment_group(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS checkpoint_result (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    instance_id BIGINT NOT NULL,
    checkpoint_id VARCHAR(64) NOT NULL,
    judge_task_ref VARCHAR(64),
    passed BOOLEAN NOT NULL DEFAULT false,
    score NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (score >= 0),
    detail_ref VARCHAR(128),
    binding_output JSONB NOT NULL DEFAULT '{}'::jsonb,
    judged_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, instance_id, checkpoint_id),
    FOREIGN KEY (tenant_id, instance_id) REFERENCES experiment_instance(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS experiment_report (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    instance_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    content_ref VARCHAR(255) NOT NULL,
    manual_score NUMERIC(5,2),
    comment TEXT,
    status SMALLINT NOT NULL CHECK (status IN (1, 2)),
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, instance_id, student_id),
    FOREIGN KEY (tenant_id, instance_id) REFERENCES experiment_instance(tenant_id, id),
    FOREIGN KEY (tenant_id, student_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS experiment_score_outbox (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    experiment_id BIGINT NOT NULL,
    instance_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    score NUMERIC(5,2) NOT NULL,
    trace_id VARCHAR(128) NOT NULL,
    scored_at TIMESTAMPTZ NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1,
    retry_count INT NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    last_error VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_experiment_score_outbox_status CHECK (status IN (1,2,3,4)),
    FOREIGN KEY (tenant_id, instance_id) REFERENCES experiment_instance(tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_experiment_course_status ON experiment(tenant_id, course_id, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_experiment_author ON experiment(tenant_id, author_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_experiment_instance_owner ON experiment_instance(tenant_id, experiment_id, owner_account_id);
CREATE INDEX IF NOT EXISTS idx_experiment_instance_status_active ON experiment_instance(tenant_id, status, last_active_at);
CREATE INDEX IF NOT EXISTS idx_experiment_instance_group ON experiment_instance(tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_group_member_student ON group_member(tenant_id, student_id);
CREATE INDEX IF NOT EXISTS idx_checkpoint_result_instance ON checkpoint_result(tenant_id, instance_id);
CREATE INDEX IF NOT EXISTS idx_checkpoint_result_judge ON checkpoint_result(tenant_id, judge_task_ref);
CREATE INDEX IF NOT EXISTS idx_experiment_report_instance_student ON experiment_report(tenant_id, instance_id, student_id);
CREATE INDEX IF NOT EXISTS idx_experiment_score_outbox_status ON experiment_score_outbox(status, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_experiment_score_outbox_instance_student ON experiment_score_outbox(tenant_id, instance_id, student_id);

ALTER TABLE experiment ENABLE ROW LEVEL SECURITY;
ALTER TABLE experiment_instance ENABLE ROW LEVEL SECURITY;
ALTER TABLE experiment_group ENABLE ROW LEVEL SECURITY;
ALTER TABLE group_member ENABLE ROW LEVEL SECURITY;
ALTER TABLE checkpoint_result ENABLE ROW LEVEL SECURITY;
ALTER TABLE experiment_report ENABLE ROW LEVEL SECURITY;
ALTER TABLE experiment_score_outbox ENABLE ROW LEVEL SECURITY;

CREATE POLICY experiment_tenant_rls ON experiment USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY experiment_instance_tenant_rls ON experiment_instance USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY experiment_group_tenant_rls ON experiment_group USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY group_member_tenant_rls ON group_member USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY checkpoint_result_tenant_rls ON checkpoint_result USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY experiment_report_tenant_rls ON experiment_report USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY experiment_score_outbox_tenant_rls ON experiment_score_outbox USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
