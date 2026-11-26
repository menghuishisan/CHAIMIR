CREATE TABLE IF NOT EXISTS alert_rule (
    id BIGINT PRIMARY KEY,
    scope SMALLINT NOT NULL,
    tenant_id BIGINT NULL,
    name VARCHAR(128) NOT NULL,
    metric VARCHAR(64) NOT NULL,
    condition JSONB NOT NULL DEFAULT '{}'::jsonb,
    level SMALLINT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_alert_rule_scope CHECK (scope IN (1,2)),
    CONSTRAINT chk_alert_rule_level CHECK (level BETWEEN 1 AND 4),
    CONSTRAINT chk_alert_rule_tenant CHECK ((scope = 1 AND tenant_id IS NULL) OR (scope = 2 AND tenant_id IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_alert_rule_scope ON alert_rule(scope, tenant_id, enabled);

CREATE TABLE IF NOT EXISTS alert_evaluation (
    rule_id BIGINT NOT NULL REFERENCES alert_rule(id),
    tenant_id BIGINT NOT NULL CHECK (tenant_id > 0),
    revision BIGINT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    breach_since TIMESTAMPTZ NULL,
    fired BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (rule_id, tenant_id)
);

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
    notify_trace_id VARCHAR(128) NOT NULL DEFAULT '',
    notify_attempts INTEGER NOT NULL DEFAULT 0 CHECK (notify_attempts >= 0),
    notify_lease_token BIGINT NOT NULL DEFAULT 0,
    notify_next_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    notify_published_at TIMESTAMPTZ NULL,
    notify_last_error VARCHAR(255) NOT NULL DEFAULT '',
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
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_backup_record_type CHECK (type = 1),
    CONSTRAINT chk_backup_record_status CHECK (status IN (1,2)),
    CONSTRAINT chk_backup_record_interval CHECK (finished_at >= started_at),
    CONSTRAINT chk_backup_record_size CHECK (size_bytes >= 0),
    CONSTRAINT chk_backup_record_ref CHECK ((status = 1 AND btrim(storage_ref) <> '') OR (status = 2 AND storage_ref = ''))
);

CREATE TABLE IF NOT EXISTS audit_export_request (
    transfer_task_id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    query_snapshot JSONB NOT NULL,
    next_check_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT audit_export_request_tenant_check CHECK (tenant_id >= 0)
);

CREATE INDEX IF NOT EXISTS idx_audit_export_request_due ON audit_export_request(next_check_at, created_at);

CREATE INDEX IF NOT EXISTS idx_backup_record_started ON backup_record(started_at DESC);

CREATE INDEX IF NOT EXISTS idx_alert_event_notify_due ON alert_event(notify_next_at, id) WHERE notify_published_at IS NULL;

ALTER TABLE alert_evaluation ENABLE ROW LEVEL SECURITY;
CREATE POLICY alert_evaluation_tenant_rls ON alert_evaluation
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT);

ALTER TABLE alert_rule ENABLE ROW LEVEL SECURITY;
ALTER TABLE alert_event ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_statistics ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_export_request ENABLE ROW LEVEL SECURITY;

CREATE POLICY alert_rule_tenant_rls ON alert_rule
    USING (NULLIF(current_setting('app.admin_platform', true), '')::boolean IS TRUE OR tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    WITH CHECK (
        (scope = 1 AND tenant_id IS NULL)
        OR (scope = 2 AND tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    );
CREATE POLICY alert_event_tenant_rls ON alert_event
    USING (NULLIF(current_setting('app.admin_platform', true), '')::boolean IS TRUE OR tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    WITH CHECK (
        NULLIF(current_setting('app.admin_platform', true), '')::boolean IS TRUE OR tenant_id IS NULL
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT
    );
CREATE POLICY platform_statistics_tenant_rls ON platform_statistics
    USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    WITH CHECK (
        (scope = 1 AND tenant_id IS NULL)
        OR (scope = 2 AND tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    );
CREATE POLICY audit_export_request_tenant_rls ON audit_export_request
    USING (tenant_id = COALESCE(NULLIF(current_setting('app.tenant_id', true), '')::BIGINT, 0));
