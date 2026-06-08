-- 迁移 0010:M9 管理后台 —— 配置、告警、统计快照与备份记录。
-- 依据 docs/09-管理后台/02-数据模型.md。M9 只写自有运维元数据,审计查询走 M1 audit_log。

CREATE TABLE system_config (
    id         BIGINT PRIMARY KEY,
    scope      SMALLINT    NOT NULL,
    tenant_id  BIGINT      NULL,
    key        VARCHAR(128) NOT NULL,
    value      JSONB       NOT NULL DEFAULT '{}'::jsonb,
    version    INT         NOT NULL DEFAULT 1,
    updated_by BIGINT      NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_system_config_scope CHECK (
        (scope = 1 AND tenant_id IS NULL) OR (scope = 2 AND tenant_id IS NOT NULL)
    )
);
CREATE UNIQUE INDEX uk_system_config_scope_key ON system_config (scope, COALESCE(tenant_id, 0), key);
CREATE INDEX idx_system_config_tenant ON system_config (tenant_id);

CREATE TABLE config_change_log (
    id          BIGINT PRIMARY KEY,
    config_id   BIGINT      NOT NULL,
    tenant_id   BIGINT      NULL,
    old_value   JSONB       NOT NULL DEFAULT '{}'::jsonb,
    new_value   JSONB       NOT NULL DEFAULT '{}'::jsonb,
    operator_id BIGINT      NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_config_change_log_config ON config_change_log (config_id, created_at DESC);
CREATE INDEX idx_config_change_log_tenant ON config_change_log (tenant_id, created_at DESC);

CREATE TABLE alert_rule (
    id         BIGINT PRIMARY KEY,
    scope      SMALLINT     NOT NULL,
    tenant_id  BIGINT       NULL,
    name       VARCHAR(128) NOT NULL,
    metric     VARCHAR(64)  NOT NULL,
    condition  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    level      SMALLINT     NOT NULL,
    enabled    BOOLEAN      NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT ck_alert_rule_scope CHECK (
        (scope = 1 AND tenant_id IS NULL) OR (scope = 2 AND tenant_id IS NOT NULL)
    )
);
CREATE INDEX idx_alert_rule_scope ON alert_rule (scope, tenant_id, enabled);

CREATE TABLE alert_event (
    id           BIGINT PRIMARY KEY,
    rule_id      BIGINT       NOT NULL,
    tenant_id    BIGINT       NULL,
    level        SMALLINT     NOT NULL,
    message      VARCHAR(500) NOT NULL,
    status       SMALLINT     NOT NULL,
    handler_id   BIGINT       NULL,
    triggered_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    handled_at   TIMESTAMPTZ  NULL
);
CREATE INDEX idx_alert_event_status ON alert_event (tenant_id, status, triggered_at DESC);

CREATE TABLE platform_statistics (
    id         BIGINT PRIMARY KEY,
    scope      SMALLINT    NOT NULL,
    tenant_id  BIGINT      NULL,
    stat_date  DATE        NOT NULL,
    metrics    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_platform_statistics_scope CHECK (
        (scope = 1 AND tenant_id IS NULL) OR (scope = 2 AND tenant_id IS NOT NULL)
    )
);
CREATE UNIQUE INDEX uk_platform_statistics_scope_date ON platform_statistics (scope, COALESCE(tenant_id, 0), stat_date);
CREATE INDEX idx_platform_statistics_tenant_date ON platform_statistics (tenant_id, stat_date DESC);

CREATE TABLE backup_record (
    id          BIGINT PRIMARY KEY,
    type        SMALLINT     NOT NULL,
    storage_ref VARCHAR(255) NOT NULL,
    size_bytes  BIGINT       NOT NULL DEFAULT 0,
    status      SMALLINT     NOT NULL,
    started_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ  NULL
);
CREATE INDEX idx_backup_record_started ON backup_record (started_at DESC);

CREATE TRIGGER trg_alert_rule_updated BEFORE UPDATE ON alert_rule FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DO $$
DECLARE
    t TEXT;
    tenant_tables TEXT[] := ARRAY['system_config','config_change_log','alert_rule','alert_event','platform_statistics'];
BEGIN
    FOREACH t IN ARRAY tenant_tables LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format($f$
            CREATE POLICY tenant_isolation ON %I
                USING (tenant_id IS NULL OR tenant_id = COALESCE(NULLIF(current_setting('app.tenant_id', true), '')::BIGINT, -1))
                WITH CHECK (tenant_id IS NULL OR tenant_id = COALESCE(NULLIF(current_setting('app.tenant_id', true), '')::BIGINT, -1))
        $f$, t);
    END LOOP;
END $$;
