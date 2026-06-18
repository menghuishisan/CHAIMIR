CREATE TABLE IF NOT EXISTS system_config (
    id BIGINT PRIMARY KEY,
    scope SMALLINT NOT NULL,
    tenant_id BIGINT NULL,
    key VARCHAR(128) NOT NULL,
    value JSONB NOT NULL DEFAULT '{}'::jsonb,
    version INT NOT NULL DEFAULT 1,
    updated_by BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_system_config_scope CHECK (scope IN (1,2)),
    CONSTRAINT chk_system_config_tenant CHECK ((scope = 1 AND tenant_id IS NULL) OR (scope = 2 AND tenant_id IS NOT NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_system_config_global_key ON system_config(scope, key) WHERE tenant_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_system_config_tenant_key ON system_config(scope, tenant_id, key) WHERE tenant_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS config_change_log (
    id BIGINT PRIMARY KEY,
    config_id BIGINT NOT NULL REFERENCES system_config(id),
    tenant_id BIGINT NULL,
    old_value JSONB NOT NULL DEFAULT '{}'::jsonb,
    new_value JSONB NOT NULL DEFAULT '{}'::jsonb,
    operator_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_config_change_log_config ON config_change_log(config_id, created_at DESC);

CREATE TABLE IF NOT EXISTS alert_rule (
    id BIGINT PRIMARY KEY,
    scope SMALLINT NOT NULL,
    tenant_id BIGINT NULL,
    name VARCHAR(128) NOT NULL,
    metric VARCHAR(64) NOT NULL,
    condition JSONB NOT NULL DEFAULT '{}'::jsonb,
    level SMALLINT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_alert_rule_scope CHECK (scope IN (1,2)),
    CONSTRAINT chk_alert_rule_level CHECK (level BETWEEN 1 AND 4),
    CONSTRAINT chk_alert_rule_tenant CHECK ((scope = 1 AND tenant_id IS NULL) OR (scope = 2 AND tenant_id IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_alert_rule_scope ON alert_rule(scope, tenant_id, enabled);

CREATE TABLE IF NOT EXISTS alert_event (
    id BIGINT PRIMARY KEY,
    rule_id BIGINT NOT NULL REFERENCES alert_rule(id),
    tenant_id BIGINT NULL,
    level SMALLINT NOT NULL,
    message VARCHAR(500) NOT NULL,
    status SMALLINT NOT NULL,
    handler_id BIGINT NULL,
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    handled_at TIMESTAMPTZ NULL,
    CONSTRAINT chk_alert_event_level CHECK (level BETWEEN 1 AND 4),
    CONSTRAINT chk_alert_event_status CHECK (status IN (1,2,3))
);

CREATE INDEX IF NOT EXISTS idx_alert_event_tenant_status ON alert_event(tenant_id, status, triggered_at DESC);

CREATE TABLE IF NOT EXISTS platform_statistics (
    id BIGINT PRIMARY KEY,
    scope SMALLINT NOT NULL,
    tenant_id BIGINT NULL,
    stat_date DATE NOT NULL,
    metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_platform_statistics_scope CHECK (scope IN (1,2)),
    CONSTRAINT chk_platform_statistics_tenant CHECK ((scope = 1 AND tenant_id IS NULL) OR (scope = 2 AND tenant_id IS NOT NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_statistics_global_date ON platform_statistics(scope, stat_date) WHERE tenant_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_statistics_tenant_date ON platform_statistics(scope, tenant_id, stat_date) WHERE tenant_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS backup_record (
    id BIGINT PRIMARY KEY,
    type SMALLINT NOT NULL,
    storage_ref VARCHAR(255) NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    status SMALLINT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ NULL,
    CONSTRAINT chk_backup_record_type CHECK (type = 1),
    CONSTRAINT chk_backup_record_status CHECK (status IN (1,2,3))
);

CREATE INDEX IF NOT EXISTS idx_backup_record_started ON backup_record(started_at DESC);

ALTER TABLE system_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE config_change_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE alert_rule ENABLE ROW LEVEL SECURITY;
ALTER TABLE alert_event ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_statistics ENABLE ROW LEVEL SECURITY;

CREATE POLICY system_config_tenant_rls ON system_config
    USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    WITH CHECK (
        (scope = 1 AND tenant_id IS NULL)
        OR (scope = 2 AND tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    );
CREATE POLICY config_change_log_tenant_rls ON config_change_log
    USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    WITH CHECK (
        tenant_id IS NULL
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT
    );
CREATE POLICY alert_rule_tenant_rls ON alert_rule
    USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    WITH CHECK (
        (scope = 1 AND tenant_id IS NULL)
        OR (scope = 2 AND tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    );
CREATE POLICY alert_event_tenant_rls ON alert_event
    USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    WITH CHECK (
        tenant_id IS NULL
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT
    );
CREATE POLICY platform_statistics_tenant_rls ON platform_statistics
    USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    WITH CHECK (
        (scope = 1 AND tenant_id IS NULL)
        OR (scope = 2 AND tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    );
