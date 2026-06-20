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
    prepulled BOOLEAN NOT NULL DEFAULT false,
    prepull_status SMALLINT NOT NULL CHECK (prepull_status IN (1, 2, 3, 4)),
    prepull_detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    prepulled_at TIMESTAMPTZ,
    genesis_baked BOOLEAN NOT NULL DEFAULT false,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (runtime_id, version),
    UNIQUE (runtime_id, image_url),
    UNIQUE (id, runtime_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_runtime_image_default
ON runtime_image(runtime_id)
WHERE is_default = true;

CREATE TABLE IF NOT EXISTS tool (
    id BIGINT PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    kind SMALLINT NOT NULL CHECK (kind IN (1, 2, 3)),
    eco_tags VARCHAR(255) NOT NULL,
    resource_spec JSONB NOT NULL DEFAULT '{}'::jsonb,
    status SMALLINT NOT NULL CHECK (status IN (1, 2)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sandbox (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    runtime_id BIGINT NOT NULL REFERENCES runtime(id),
    image_id BIGINT NOT NULL,
    namespace VARCHAR(128) NOT NULL UNIQUE,
    source_ref VARCHAR(128) NOT NULL,
    owner_account_id BIGINT NOT NULL,
    phase SMALLINT NOT NULL CHECK (phase IN (1, 2, 3, 4)),
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3, 4, 5, 6, 7, 8)),
    keep_alive BOOLEAN NOT NULL DEFAULT false,
    snapshot_enabled BOOLEAN NOT NULL DEFAULT false,
    code_storage_key VARCHAR(255) NOT NULL,
    code_hash VARCHAR(128),
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
    FOREIGN KEY (image_id, runtime_id) REFERENCES runtime_image(id, runtime_id),
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
    owner_account_id BIGINT NOT NULL,
    reason VARCHAR(64) NOT NULL,
    trace_id VARCHAR(128) NOT NULL,
    recycled_at TIMESTAMPTZ NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1,
    retry_count INT NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    last_error VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_sandbox_recycle_outbox_status CHECK (status IN (1,2,3,4)),
    FOREIGN KEY (tenant_id, sandbox_id) REFERENCES sandbox(tenant_id, id) ON DELETE CASCADE
);

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
CREATE INDEX IF NOT EXISTS idx_runtime_image_runtime_status ON runtime_image(runtime_id, status, prepull_status, prepulled);
CREATE INDEX IF NOT EXISTS idx_tool_status ON tool(status);
CREATE INDEX IF NOT EXISTS idx_sandbox_tenant_status ON sandbox(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_sandbox_tenant_owner ON sandbox(tenant_id, owner_account_id);
CREATE INDEX IF NOT EXISTS idx_sandbox_last_active ON sandbox(last_active_at);
CREATE INDEX IF NOT EXISTS idx_sandbox_source_ref ON sandbox(tenant_id, source_ref);
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
