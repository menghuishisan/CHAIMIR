-- 迁移 0003:M2 沙箱引擎 —— 运行时/工具全局配置 + 沙箱实例租户表 + RLS。
-- 依据 docs/02-沙箱引擎/03-数据模型.md。
-- 全局表(runtime/runtime_image/tool):平台级配置,无 tenant_id。
-- 租户表(sandbox/sandbox_tool/sandbox_event/tenant_quota):含 tenant_id NOT NULL,启用 RLS 强制隔离。

-- ============================================================
-- 一、平台级配置表
-- ============================================================

CREATE TABLE runtime (
    id               BIGINT PRIMARY KEY,
    code             VARCHAR(64)  NOT NULL UNIQUE,
    name             VARCHAR(128) NOT NULL,
    eco              VARCHAR(32)  NOT NULL,
    adapter_level    SMALLINT     NOT NULL,
    adapter_spec     JSONB        NOT NULL DEFAULT '{}'::jsonb,
    capability_impl  VARCHAR(128) NULL,
    plugin_ref       VARCHAR(128) NULL,
    selftest_status  SMALLINT     NOT NULL DEFAULT 1, -- 1待测/2通过/3失败
    selftest_detail  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    status           SMALLINT     NOT NULL DEFAULT 2, -- 1可用/2接入中/3停用
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_runtime_code ON runtime (code);
CREATE INDEX idx_runtime_status ON runtime (status);

CREATE TABLE runtime_image (
    id            BIGINT PRIMARY KEY,
    runtime_id    BIGINT       NOT NULL,
    image_url     VARCHAR(255) NOT NULL,
    version       VARCHAR(64)  NOT NULL,
    prepulled     BOOLEAN      NOT NULL DEFAULT false,
    prepull_status SMALLINT    NOT NULL DEFAULT 1, -- 1未预拉取/2已完成/3失败/4进行中
    prepull_detail JSONB       NOT NULL DEFAULT '{}'::jsonb,
    prepulled_at   TIMESTAMPTZ NULL,
    genesis_baked BOOLEAN      NOT NULL DEFAULT false,
    is_default    BOOLEAN      NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_runtime_image_runtime ON runtime_image (runtime_id);
CREATE UNIQUE INDEX uk_runtime_image_version ON runtime_image (runtime_id, version);
CREATE UNIQUE INDEX uk_runtime_image_default ON runtime_image (runtime_id) WHERE is_default;
ALTER TABLE runtime_image
    ADD CONSTRAINT runtime_image_runtime_fk
    FOREIGN KEY (runtime_id) REFERENCES runtime(id);

CREATE TABLE tool (
    id            BIGINT PRIMARY KEY,
    code          VARCHAR(64)  NOT NULL UNIQUE,
    name          VARCHAR(128) NOT NULL,
    kind          SMALLINT     NOT NULL, -- 1=terminal/2=web-embed/3=platform-builtin
    image_url     VARCHAR(255) NULL,
    port          INT          NULL,
    eco_tags      VARCHAR(255) NOT NULL,
    resource_spec JSONB        NOT NULL DEFAULT '{}'::jsonb,
    status        SMALLINT     NOT NULL DEFAULT 1, -- 1可用/2停用
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_tool_code ON tool (code);
CREATE INDEX idx_tool_status ON tool (status);

CREATE TRIGGER trg_runtime_updated BEFORE UPDATE ON runtime
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_tool_updated BEFORE UPDATE ON tool
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 二、租户级实例表
-- ============================================================

CREATE TABLE sandbox (
    id               BIGINT PRIMARY KEY,
    tenant_id        BIGINT       NOT NULL,
    runtime_id       BIGINT       NOT NULL,
    image_id         BIGINT       NOT NULL,
    namespace        VARCHAR(128) NOT NULL,
    source_ref       VARCHAR(128) NOT NULL,
    owner_account_id BIGINT       NOT NULL,
    phase            SMALLINT     NOT NULL DEFAULT 1, -- 1分配中/2环境就绪/3初始化中/4完全就绪
    status           SMALLINT     NOT NULL DEFAULT 1, -- 1creating/2ready/3running/4idle/5recycling/6destroyed/7error
    keep_alive       BOOLEAN      NOT NULL DEFAULT false,
    snapshot_enabled BOOLEAN      NOT NULL DEFAULT false,
    code_storage_key VARCHAR(255) NOT NULL,
    code_hash        VARCHAR(128) NULL,
    init_script_ref  VARCHAR(255) NULL,
    snapshot_ref     VARCHAR(255) NULL,
    snapshot_created_at TIMESTAMPTZ NULL,
    snapshot_expire_at TIMESTAMPTZ NULL,
    keep_alive_until TIMESTAMPTZ NULL,
    last_active_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    expire_at        TIMESTAMPTZ  NOT NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_sandbox_namespace ON sandbox (namespace);
CREATE INDEX idx_sandbox_tenant_status ON sandbox (tenant_id, status);
CREATE INDEX idx_sandbox_tenant_owner ON sandbox (tenant_id, owner_account_id);
CREATE INDEX idx_sandbox_last_active ON sandbox (last_active_at);
CREATE INDEX idx_sandbox_source_ref ON sandbox (source_ref);
CREATE INDEX idx_sandbox_snapshot_expire ON sandbox (snapshot_expire_at);
ALTER TABLE sandbox
    ADD CONSTRAINT sandbox_runtime_fk
    FOREIGN KEY (runtime_id) REFERENCES runtime(id);
ALTER TABLE sandbox
    ADD CONSTRAINT sandbox_image_fk
    FOREIGN KEY (image_id) REFERENCES runtime_image(id);

CREATE TABLE sandbox_tool (
    id              BIGINT PRIMARY KEY,
    tenant_id       BIGINT       NOT NULL,
    sandbox_id      BIGINT       NOT NULL,
    tool_id         BIGINT       NOT NULL,
    access_endpoint VARCHAR(255) NOT NULL,
    status          SMALLINT     NOT NULL DEFAULT 2 -- 1就绪/2启动中/3失败
);
CREATE INDEX idx_sandbox_tool_sandbox ON sandbox_tool (tenant_id, sandbox_id);
CREATE UNIQUE INDEX uk_sandbox_tool ON sandbox_tool (tenant_id, sandbox_id, tool_id);
ALTER TABLE sandbox_tool
    ADD CONSTRAINT sandbox_tool_sandbox_fk
    FOREIGN KEY (sandbox_id) REFERENCES sandbox(id);
ALTER TABLE sandbox_tool
    ADD CONSTRAINT sandbox_tool_tool_fk
    FOREIGN KEY (tool_id) REFERENCES tool(id);

CREATE TABLE sandbox_event (
    id         BIGINT PRIMARY KEY,
    tenant_id  BIGINT      NOT NULL,
    sandbox_id BIGINT      NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    detail     JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sandbox_event_sandbox_time ON sandbox_event (tenant_id, sandbox_id, created_at);
ALTER TABLE sandbox_event
    ADD CONSTRAINT sandbox_event_sandbox_fk
    FOREIGN KEY (sandbox_id) REFERENCES sandbox(id);

CREATE TABLE tenant_quota (
    tenant_id              BIGINT      PRIMARY KEY,
    max_concurrent_sandbox INT         NOT NULL,
    max_cpu                INT         NOT NULL,
    max_memory_mb          INT         NOT NULL,
    idle_timeout_min       INT         NOT NULL DEFAULT 30,
    max_lifetime_min       INT         NOT NULL,
    max_keepalive_min      INT         NOT NULL,
    max_snapshot_retention_min INT     NOT NULL,
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_sandbox_updated BEFORE UPDATE ON sandbox
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 三、行级安全(RLS)
-- ============================================================

DO $$
DECLARE
    t TEXT;
    tenant_tables TEXT[] := ARRAY[
        'sandbox','sandbox_tool','sandbox_event','tenant_quota'
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
