CREATE TABLE IF NOT EXISTS sim_package (
    id BIGINT PRIMARY KEY,
    code VARCHAR(96) NOT NULL,
    version VARCHAR(32) NOT NULL,
    name VARCHAR(128) NOT NULL,
    category VARCHAR(32) NOT NULL,
    compute SMALLINT NOT NULL CHECK (compute IN (1, 2)),
    scale_limit JSONB NOT NULL DEFAULT '{}'::jsonb,
    bundle_key VARCHAR(255) NOT NULL,
    bundle_hash VARCHAR(64) NOT NULL,
    backend_adapter VARCHAR(96),
    backend_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    author_type SMALLINT NOT NULL CHECK (author_type IN (1, 2, 3)),
    author_id BIGINT,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3, 4, 5)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (code, version)
);

CREATE TABLE IF NOT EXISTS sim_package_review (
    id BIGINT PRIMARY KEY,
    package_id BIGINT NOT NULL REFERENCES sim_package(id) ON DELETE CASCADE,
    submitter_id BIGINT NOT NULL,
    preview_report JSONB NOT NULL DEFAULT '{}'::jsonb,
    reviewer_id BIGINT,
    result SMALLINT NOT NULL CHECK (result IN (1, 2, 3)),
    comment VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sim_session (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    package_id BIGINT NOT NULL REFERENCES sim_package(id),
    source_ref VARCHAR(128) NOT NULL,
    owner_account_id BIGINT NOT NULL,
    seed BIGINT NOT NULL,
    init_params JSONB NOT NULL DEFAULT '{}'::jsonb,
    compute SMALLINT NOT NULL CHECK (compute IN (1, 2)),
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3, 4, 5, 6)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, owner_account_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS sim_action_log (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    session_id BIGINT NOT NULL,
    seq INT NOT NULL CHECK (seq > 0),
    at_tick INT NOT NULL CHECK (at_tick >= 0),
    event_type VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, session_id, seq),
    FOREIGN KEY (tenant_id, session_id) REFERENCES sim_session(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sim_checkpoint (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    session_id BIGINT NOT NULL,
    checkpoint_id VARCHAR(96) NOT NULL,
    answer JSONB NOT NULL DEFAULT '{}'::jsonb,
    achieved BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, session_id, checkpoint_id),
    FOREIGN KEY (tenant_id, session_id) REFERENCES sim_session(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sim_share (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    session_id BIGINT NOT NULL,
    code VARCHAR(48) NOT NULL UNIQUE,
    created_by BIGINT NOT NULL,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3)),
    expire_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, session_id) REFERENCES sim_session(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, created_by) REFERENCES account(tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_sim_package_status_category ON sim_package(status, category);
CREATE INDEX IF NOT EXISTS idx_sim_package_code ON sim_package(code, version);
CREATE INDEX IF NOT EXISTS idx_sim_package_review_result ON sim_package_review(result, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_sim_session_owner ON sim_session(tenant_id, owner_account_id);
CREATE INDEX IF NOT EXISTS idx_sim_session_source_ref ON sim_session(tenant_id, source_ref);
CREATE INDEX IF NOT EXISTS idx_sim_action_session_seq ON sim_action_log(tenant_id, session_id, seq);
CREATE INDEX IF NOT EXISTS idx_sim_checkpoint_session ON sim_checkpoint(tenant_id, session_id);
CREATE INDEX IF NOT EXISTS idx_sim_share_session ON sim_share(tenant_id, session_id);

ALTER TABLE sim_session ENABLE ROW LEVEL SECURITY;
ALTER TABLE sim_action_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE sim_checkpoint ENABLE ROW LEVEL SECURITY;

CREATE POLICY sim_session_tenant_rls ON sim_session USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY sim_action_log_tenant_rls ON sim_action_log USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY sim_checkpoint_tenant_rls ON sim_checkpoint USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
