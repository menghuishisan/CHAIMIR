CREATE TABLE IF NOT EXISTS judger (
    id BIGINT PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    type SMALLINT NOT NULL CHECK (type IN (1, 2, 3, 4, 5, 6)),
    executor_ref VARCHAR(128) NOT NULL,
    runtime_required BOOLEAN NOT NULL DEFAULT false,
    default_timeout_sec INT NOT NULL CHECK (default_timeout_sec > 0),
    resource_spec JSONB NOT NULL DEFAULT '{}'::jsonb,
    selftest_status SMALLINT NOT NULL CHECK (selftest_status IN (1, 2, 3)),
    status SMALLINT NOT NULL CHECK (status IN (1, 2)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS judge_task (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    judger_id BIGINT NOT NULL REFERENCES judger(id),
    source_ref VARCHAR(128) NOT NULL,
    source_owner_id BIGINT NOT NULL CHECK (source_owner_id > 0),
    source_course_id BIGINT NOT NULL CHECK (source_course_id >= 0),
    source_scope VARCHAR(32) NOT NULL CHECK (source_scope IN ('teaching', 'experiment', 'contest')),
    submitter_id BIGINT NOT NULL,
    problem_ref VARCHAR(128) NOT NULL,
    code_storage_key VARCHAR(255) NOT NULL DEFAULT '',
    code_hash VARCHAR(64) NOT NULL DEFAULT '',
    input_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    sandbox_mode SMALLINT NOT NULL CHECK (sandbox_mode IN (1, 2)),
    target_sandbox_ref VARCHAR(64),
    priority SMALLINT NOT NULL DEFAULT 1,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3, 4, 5, 6, 7)),
    retry_count INT NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    max_retries INT NOT NULL DEFAULT 0 CHECK (max_retries >= 0),
    last_error VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, source_ref),
    FOREIGN KEY (tenant_id, submitter_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS judge_result (
    id BIGINT PRIMARY KEY,
    task_id BIGINT NOT NULL,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    version INT NOT NULL CHECK (version > 0),
    passed BOOLEAN NOT NULL,
    score INT NOT NULL CHECK (score >= 0),
    max_score INT NOT NULL CHECK (max_score >= 0),
    details JSONB NOT NULL DEFAULT '[]'::jsonb,
    judge_sandbox_ref VARCHAR(128) NOT NULL,
    judged_at TIMESTAMPTZ NOT NULL,
    is_rejudge BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (tenant_id, task_id, version),
    FOREIGN KEY (tenant_id, task_id) REFERENCES judge_task(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS judge_event_outbox (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    task_id BIGINT NOT NULL,
    subject VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3)),
    retry_count INT NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, task_id) REFERENCES judge_task(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS submission_fingerprint (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    source_ref VARCHAR(128) NOT NULL,
    problem_ref VARCHAR(128) NOT NULL,
    submitter_id BIGINT NOT NULL,
    code_hash VARCHAR(64) NOT NULL,
    sim_vector JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, submitter_id) REFERENCES account(tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_judger_status ON judger(status, selftest_status);
CREATE INDEX IF NOT EXISTS idx_judge_task_queue ON judge_task(tenant_id, status, priority DESC, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_judge_task_submitter ON judge_task(tenant_id, submitter_id);
CREATE INDEX IF NOT EXISTS idx_judge_task_owner ON judge_task(tenant_id, source_owner_id, source_ref);
CREATE INDEX IF NOT EXISTS idx_judge_result_latest ON judge_result(tenant_id, task_id, version DESC);
CREATE INDEX IF NOT EXISTS idx_judge_event_outbox_status ON judge_event_outbox(status, next_attempt_at ASC, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_submission_fingerprint_exact ON submission_fingerprint(tenant_id, problem_ref, code_hash);
CREATE INDEX IF NOT EXISTS idx_submission_fingerprint_source ON submission_fingerprint(tenant_id, source_ref);

ALTER TABLE judge_task ENABLE ROW LEVEL SECURITY;
ALTER TABLE judge_result ENABLE ROW LEVEL SECURITY;
ALTER TABLE judge_event_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE submission_fingerprint ENABLE ROW LEVEL SECURITY;

CREATE POLICY judge_task_tenant_rls ON judge_task USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY judge_result_tenant_rls ON judge_result USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY judge_event_outbox_tenant_rls ON judge_event_outbox USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY submission_fingerprint_tenant_rls ON submission_fingerprint USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
