CREATE TABLE IF NOT EXISTS sim_package (
    id BIGINT PRIMARY KEY,
    code VARCHAR(96) NOT NULL,
    version VARCHAR(32) NOT NULL,
    name VARCHAR(128) NOT NULL,
    category VARCHAR(32) NOT NULL,
    -- compute 是执行位置:1=浏览器 Worker(平台内置包)、2=后端隔离容器(教师/第三方扩展包与重计算仿真)。
    -- 它按 author_type 派生,不是作者可选项(见 docs/04-仿真可视化引擎/02-架构设计.md §8)。
    compute SMALLINT NOT NULL CHECK (compute IN (1, 2)),
    scale_limit JSONB NOT NULL DEFAULT '{}'::jsonb,
    bundle_key VARCHAR(255) NOT NULL,
    bundle_hash VARCHAR(64) NOT NULL,
    -- entry 是归档内入口模块相对路径,供隔离容器装配;内置包由 sim-sdk registry 按 code 装配故为 NULL。
    entry VARCHAR(255),
    backend_adapter VARCHAR(96),
    backend_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    interaction_schema JSONB NOT NULL DEFAULT '{"events":{}}'::jsonb,
    code_trace JSONB NOT NULL DEFAULT '{}'::jsonb,
    author_type SMALLINT NOT NULL CHECK (author_type IN (1, 2, 3)),
    author_id BIGINT,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3, 4, 5)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- 作者类型决定命名空间、执行位置与装配方式三者,合成一条约束表达它们的绑定关系:
    -- 平台内置 → builtin__ 前缀、浏览器执行、无入口模块、无运行能力;
    -- 教师/第三方 → 各自前缀、隔离容器执行、必须有入口模块与已注册运行能力。
    -- 三者拆成独立约束会让"内置包带 entry"这类矛盾组合通过校验。
    CHECK ((author_type = 1
            AND substring(code FROM 1 FOR 9) = 'builtin__'
            AND author_id IS NULL
            AND compute = 1
            AND entry IS NULL
            AND backend_adapter IS NULL
            AND backend_config = '{}'::jsonb)
        OR (author_type = 2
            AND author_id IS NOT NULL
            AND substring(code FROM 1 FOR length('teacher_' || author_id::TEXT || '__')) = ('teacher_' || author_id::TEXT || '__')
            AND compute = 2
            AND entry IS NOT NULL
            AND backend_adapter IS NOT NULL)
        OR (author_type = 3
            AND code ~ '^org_[a-z0-9_]+__'
            AND compute = 2
            AND entry IS NOT NULL
            AND backend_adapter IS NOT NULL)),
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
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    preview_attempt_count INT NOT NULL DEFAULT 0 CHECK (preview_attempt_count >= 0),
    preview_lease_token VARCHAR(64) NOT NULL DEFAULT '',
    preview_lease_until TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_sim_package_review_preview_lease ON sim_package_review(result, preview_lease_until) WHERE result = 1;

CREATE TABLE IF NOT EXISTS sim_session (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    package_id BIGINT NOT NULL REFERENCES sim_package(id),
    source_ref VARCHAR(128) NOT NULL,
    scope_ref VARCHAR(128) NOT NULL,
    owner_account_id BIGINT NOT NULL,
    shared_account_ids BIGINT[] NOT NULL DEFAULT '{}'::BIGINT[],
    seed BIGINT NOT NULL,
    init_params JSONB NOT NULL DEFAULT '{}'::jsonb,
    compute SMALLINT NOT NULL CHECK (compute IN (1, 2)),
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3, 4, 5, 6)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, scope_ref),
    CHECK (source_ref ~ '^[a-z]+:[0-9]{4}:[a-z][a-z0-9_-]*:[0-9A-Za-z_-]+$'),
    CONSTRAINT chk_sim_session_shared_accounts_positive CHECK (0 < ALL(shared_account_ids)),
    CONSTRAINT chk_sim_session_shared_accounts_no_null CHECK (array_position(shared_account_ids, NULL) IS NULL),
    CONSTRAINT chk_sim_session_shared_accounts_exclude_owner CHECK (NOT (shared_account_ids @> ARRAY[owner_account_id])),
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
-- 隔离执行会话按租户计数(并发闸门 SIM_BACKEND_MAX_CONCURRENT_SESSIONS_PER_TENANT):
-- 只索引 compute=2 的活跃态,因为浏览器执行的会话不占集群资源、不参与该闸门。
CREATE INDEX IF NOT EXISTS idx_sim_session_isolated_active ON sim_session(tenant_id)
    WHERE compute = 2 AND status IN (1, 2, 3);
CREATE INDEX IF NOT EXISTS idx_sim_action_session_seq ON sim_action_log(tenant_id, session_id, seq);
CREATE INDEX IF NOT EXISTS idx_sim_checkpoint_session ON sim_checkpoint(tenant_id, session_id);
CREATE INDEX IF NOT EXISTS idx_sim_share_session ON sim_share(tenant_id, session_id);

ALTER TABLE sim_session ENABLE ROW LEVEL SECURITY;
ALTER TABLE sim_action_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE sim_checkpoint ENABLE ROW LEVEL SECURITY;
ALTER TABLE sim_share ENABLE ROW LEVEL SECURITY;

CREATE POLICY sim_session_tenant_rls ON sim_session USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY sim_action_log_tenant_rls ON sim_action_log USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY sim_checkpoint_tenant_rls ON sim_checkpoint USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY sim_share_tenant_rls ON sim_share USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
