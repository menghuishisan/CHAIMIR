CREATE TABLE IF NOT EXISTS notification (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    receiver_id BIGINT NOT NULL,
    type VARCHAR(64) NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    link VARCHAR(255) NULL,
    is_read BOOLEAN NOT NULL DEFAULT false,
    read_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_notification_inbox ON notification(tenant_id, receiver_id, is_read, created_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS notification_template (
    id BIGINT PRIMARY KEY,
    type VARCHAR(64) NOT NULL UNIQUE,
    title_tpl VARCHAR(255) NOT NULL,
    content_tpl TEXT NOT NULL,
    channels TEXT[] NOT NULL DEFAULT ARRAY['inbox'],
    force BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS notification_preference (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    type VARCHAR(64) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_notification_preference_account_type ON notification_preference(tenant_id, account_id, type);

CREATE TABLE IF NOT EXISTS system_announcement (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    scope SMALLINT NOT NULL,
    target_roles SMALLINT[] NULL,
    publisher_id BIGINT NOT NULL,
    published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expire_at TIMESTAMPTZ NULL,
    CONSTRAINT chk_system_announcement_scope CHECK (scope IN (1,2,3))
);

CREATE INDEX IF NOT EXISTS idx_system_announcement_visible ON system_announcement(tenant_id, scope, published_at DESC);

CREATE TABLE IF NOT EXISTS announcement_read (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    announcement_id BIGINT NOT NULL REFERENCES system_announcement(id),
    account_id BIGINT NOT NULL,
    read_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_announcement_read_account ON announcement_read(tenant_id, announcement_id, account_id);

ALTER TABLE notification ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_preference ENABLE ROW LEVEL SECURITY;
ALTER TABLE system_announcement ENABLE ROW LEVEL SECURITY;
ALTER TABLE announcement_read ENABLE ROW LEVEL SECURITY;

CREATE POLICY notification_tenant_rls ON notification USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY notification_preference_tenant_rls ON notification_preference USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY system_announcement_tenant_rls ON system_announcement USING (tenant_id IS NULL OR tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY announcement_read_tenant_rls ON announcement_read USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
