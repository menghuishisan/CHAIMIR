CREATE TABLE IF NOT EXISTS runtime (
    id BIGINT PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    eco VARCHAR(32) NOT NULL,
    adapter_level SMALLINT NOT NULL CHECK (adapter_level IN (1, 2, 3)),
    adapter_spec JSONB NOT NULL,
    capability_impl VARCHAR(128),
    plugin_ref VARCHAR(128),
    selftest_status SMALLINT NOT NULL CHECK (selftest_status IN (1, 2, 3)),
    selftest_detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS runtime_image (
    id BIGINT PRIMARY KEY,
    runtime_id BIGINT NOT NULL REFERENCES runtime(id),
    image_url VARCHAR(255) NOT NULL,
    version VARCHAR(64) NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1 CHECK (status IN (1, 2)),
    genesis_baked BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (runtime_id, version),
    UNIQUE (runtime_id, image_url),
    UNIQUE (id, runtime_id)
);

CREATE TABLE IF NOT EXISTS sandbox_composition (
    composition_digest VARCHAR(128) PRIMARY KEY,
    snapshot JSONB NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1 CHECK (status IN (1, 2)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sandbox_composition_prepull (
    id BIGINT PRIMARY KEY,
    runtime_image_id BIGINT NOT NULL REFERENCES runtime_image(id) ON DELETE CASCADE,
    composition_digest VARCHAR(128) NOT NULL,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3, 4)),
    attempt_id VARCHAR(64) NOT NULL,
    daemonset_name VARCHAR(253) NOT NULL DEFAULT '',
    image_closure JSONB NOT NULL DEFAULT '[]'::jsonb,
    desired_nodes INT NOT NULL DEFAULT 0 CHECK (desired_nodes >= 0),
    ready_nodes INT NOT NULL DEFAULT 0 CHECK (ready_nodes >= 0),
    detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (runtime_image_id, composition_digest),
    UNIQUE (runtime_image_id, attempt_id)
);

CREATE TABLE IF NOT EXISTS tool (
    id BIGINT PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    category VARCHAR(16) NOT NULL CHECK (category IN ('infra', 'tool')),
    kind SMALLINT NOT NULL CHECK (kind IN (1, 2, 3, 4, 5)),
    eco_tags VARCHAR(255) NOT NULL,
    resource_spec JSONB NOT NULL DEFAULT '{}'::jsonb,
    status SMALLINT NOT NULL CHECK (status IN (1, 2)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sandbox (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    namespace VARCHAR(128) NOT NULL UNIQUE,
    source_ref VARCHAR(128) NOT NULL,
    scope_ref VARCHAR(128) NOT NULL,
    composition_digest VARCHAR(128) NOT NULL,
    composition_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    access_profile VARCHAR(32) NOT NULL CHECK (access_profile IN ('experiment', 'contest-solve', 'contest-battle', 'vulnerability-prevalidate', 'judge-private')),
    owner_account_id BIGINT NOT NULL,
    shared_account_ids BIGINT[] NOT NULL DEFAULT '{}'::BIGINT[],
    phase SMALLINT NOT NULL CHECK (phase IN (1, 2, 3, 4)),
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3, 4, 5, 6, 7, 8)),
    keep_alive BOOLEAN NOT NULL DEFAULT false,
    snapshot_enabled BOOLEAN NOT NULL DEFAULT false,
    code_storage_key VARCHAR(255) NOT NULL,
    code_hash VARCHAR(128),
    workspace_revision BIGINT NOT NULL DEFAULT 1 CHECK (workspace_revision > 0),
    init_code_ref VARCHAR(255),
    init_script_ref VARCHAR(255),
    snapshot_ref VARCHAR(255),
    snapshot_domains JSONB NOT NULL DEFAULT '[]'::jsonb,
    snapshot_created_at TIMESTAMPTZ,
    snapshot_expire_at TIMESTAMPTZ,
    keep_alive_until TIMESTAMPTZ,
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expire_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    CONSTRAINT chk_sandbox_shared_accounts_positive CHECK (0 < ALL(shared_account_ids)),
    CONSTRAINT chk_sandbox_shared_accounts_no_null CHECK (array_position(shared_account_ids, NULL) IS NULL),
    CONSTRAINT chk_sandbox_shared_accounts_exclude_owner CHECK (NOT (shared_account_ids @> ARRAY[owner_account_id])),
    FOREIGN KEY (tenant_id, owner_account_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS sandbox_tool (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    sandbox_id BIGINT NOT NULL,
    tool_id BIGINT NOT NULL REFERENCES tool(id),
    access_endpoint VARCHAR(255) NOT NULL,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3)),
    FOREIGN KEY (tenant_id, sandbox_id) REFERENCES sandbox(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, sandbox_id, tool_id)
);

CREATE TABLE IF NOT EXISTS sandbox_event (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    sandbox_id BIGINT NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    detail JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, sandbox_id) REFERENCES sandbox(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sandbox_recycle_outbox (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    sandbox_id BIGINT NOT NULL,
    source_ref VARCHAR(128) NOT NULL,
    scope_ref VARCHAR(128) NOT NULL,
    owner_account_id BIGINT NOT NULL,
    reason VARCHAR(64) NOT NULL,
    trace_id VARCHAR(128) NOT NULL,
    recycled_at TIMESTAMPTZ NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1,
    retry_count INT NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    last_error VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_token VARCHAR(64) NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ,
    CONSTRAINT chk_sandbox_recycle_outbox_status CHECK (status IN (1,2,3,4)),
    FOREIGN KEY (tenant_id, sandbox_id) REFERENCES sandbox(tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_sandbox_recycle_outbox_lease ON sandbox_recycle_outbox(status, lease_until) WHERE status = 2;

CREATE TABLE IF NOT EXISTS tenant_quota (
    tenant_id BIGINT PRIMARY KEY REFERENCES tenant(id),
    max_concurrent_sandbox INT NOT NULL CHECK (max_concurrent_sandbox > 0),
    max_cpu INT NOT NULL CHECK (max_cpu > 0),
    max_memory_mb INT NOT NULL CHECK (max_memory_mb > 0),
    idle_timeout_min INT NOT NULL CHECK (idle_timeout_min > 0),
    max_lifetime_min INT NOT NULL CHECK (max_lifetime_min > 0),
    max_keepalive_min INT NOT NULL CHECK (max_keepalive_min >= 0),
    max_snapshot_retention_min INT NOT NULL CHECK (max_snapshot_retention_min >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_runtime_status ON runtime(status, selftest_status);
CREATE INDEX IF NOT EXISTS idx_runtime_image_runtime_status ON runtime_image(runtime_id, status);
CREATE INDEX IF NOT EXISTS idx_sandbox_composition_prepull_status ON sandbox_composition_prepull(status, updated_at);
CREATE INDEX IF NOT EXISTS idx_tool_status ON tool(status);
CREATE INDEX IF NOT EXISTS idx_sandbox_tenant_status ON sandbox(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_sandbox_tenant_owner ON sandbox(tenant_id, owner_account_id);
CREATE INDEX IF NOT EXISTS idx_sandbox_last_active ON sandbox(last_active_at);
CREATE INDEX IF NOT EXISTS idx_sandbox_source_ref ON sandbox(tenant_id, source_ref);
CREATE INDEX IF NOT EXISTS idx_sandbox_scope_ref ON sandbox(tenant_id, scope_ref);
-- M8 对局来源是单场对局的稳定生命周期键;同一场未销毁对局不得重复创建沙箱。
-- 其他业务来源(例如多组件实验)允许拥有多个沙箱,故只对 battle 来源施加局部唯一约束。
CREATE UNIQUE INDEX IF NOT EXISTS uk_sandbox_battle_source_active
ON sandbox(tenant_id, source_ref)
WHERE source_ref LIKE 'contest:____:battle:%' AND status <> 5;
-- 稳定单例来源(解题环境和漏洞预验证)在同一作用域内只能保留一个未销毁沙箱。
CREATE UNIQUE INDEX IF NOT EXISTS uk_sandbox_same_scope_source_active
ON sandbox(tenant_id, source_ref, scope_ref)
WHERE source_ref = scope_ref AND status <> 5;
CREATE INDEX IF NOT EXISTS idx_sandbox_snapshot_expire ON sandbox(snapshot_expire_at);
CREATE INDEX IF NOT EXISTS idx_sandbox_event_tenant_sandbox_created ON sandbox_event(tenant_id, sandbox_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sandbox_recycle_outbox_status ON sandbox_recycle_outbox(status, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_sandbox_recycle_outbox_tenant_status ON sandbox_recycle_outbox(tenant_id, status, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_sandbox_recycle_outbox_tenant_sandbox ON sandbox_recycle_outbox(tenant_id, sandbox_id);

ALTER TABLE sandbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE sandbox_tool ENABLE ROW LEVEL SECURITY;
ALTER TABLE sandbox_event ENABLE ROW LEVEL SECURITY;
ALTER TABLE sandbox_recycle_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_quota ENABLE ROW LEVEL SECURITY;

CREATE POLICY sandbox_tenant_rls ON sandbox
USING (tenant_id = current_setting('app.tenant_id')::BIGINT)
WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);

CREATE POLICY sandbox_tool_tenant_rls ON sandbox_tool
USING (tenant_id = current_setting('app.tenant_id')::BIGINT)
WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);

CREATE POLICY sandbox_event_tenant_rls ON sandbox_event
USING (tenant_id = current_setting('app.tenant_id')::BIGINT)
WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);

CREATE POLICY sandbox_recycle_outbox_tenant_rls ON sandbox_recycle_outbox
USING (tenant_id = current_setting('app.tenant_id')::BIGINT)
WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);

CREATE POLICY tenant_quota_tenant_rls ON tenant_quota
USING (tenant_id = current_setting('app.tenant_id')::BIGINT)
WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
