-- 迁移 0011:M10 通知与实时推送 —— 站内信、通知模板、偏好、系统公告与公告已读状态。
-- 依据 docs/10-通知与实时推送/02-数据模型.md。实时推送无持久表,系统公告不写放大。

CREATE TABLE notification (
    id          BIGINT PRIMARY KEY,
    tenant_id   BIGINT       NOT NULL,
    receiver_id BIGINT       NOT NULL,
    type        VARCHAR(64)  NOT NULL,
    title       VARCHAR(255) NOT NULL,
    content     TEXT         NOT NULL,
    link        VARCHAR(255) NULL,
    is_read     BOOLEAN      NOT NULL DEFAULT false,
    read_at     TIMESTAMPTZ  NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ  NULL
);
CREATE INDEX idx_notification_inbox ON notification (tenant_id, receiver_id, is_read, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_notification_type ON notification (tenant_id, receiver_id, type, created_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE notification_template (
    id          BIGINT       PRIMARY KEY,
    type        VARCHAR(64)  NOT NULL,
    title_tpl   VARCHAR(255) NOT NULL,
    content_tpl TEXT         NOT NULL,
    channels    TEXT[]       NOT NULL DEFAULT ARRAY['inbox']::TEXT[],
    force       BOOLEAN      NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX uk_notification_template_type ON notification_template (type);

INSERT INTO notification_template (id, type, title_tpl, content_tpl, channels, force)
VALUES
    (1000000000010, 'admin.alert.handled', '告警已处理', '{{message}}', ARRAY['inbox']::TEXT[], true);

CREATE TABLE notification_preference (
    id         BIGINT      PRIMARY KEY,
    tenant_id  BIGINT      NOT NULL,
    account_id BIGINT      NOT NULL,
    type       VARCHAR(64) NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT true
);
CREATE UNIQUE INDEX uk_notification_preference_account_type ON notification_preference (tenant_id, account_id, type);
CREATE INDEX idx_notification_preference_account ON notification_preference (tenant_id, account_id);

CREATE TABLE system_announcement (
    id           BIGINT       PRIMARY KEY,
    tenant_id    BIGINT       NULL,
    title        VARCHAR(255) NOT NULL,
    content      TEXT         NOT NULL,
    scope        SMALLINT     NOT NULL,
    target_roles SMALLINT[]   NULL,
    publisher_id BIGINT       NOT NULL,
    published_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    expire_at    TIMESTAMPTZ  NULL,
    CONSTRAINT ck_system_announcement_scope CHECK (
        (scope = 1 AND tenant_id IS NULL) OR (scope IN (2, 3) AND tenant_id IS NOT NULL)
    )
);
CREATE INDEX idx_system_announcement_visible ON system_announcement (tenant_id, scope, published_at DESC);

CREATE TABLE announcement_read (
    id              BIGINT      PRIMARY KEY,
    tenant_id       BIGINT      NOT NULL,
    announcement_id BIGINT      NOT NULL,
    account_id      BIGINT      NOT NULL,
    read_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_announcement_read_account ON announcement_read (tenant_id, announcement_id, account_id);
CREATE INDEX idx_announcement_read_account ON announcement_read (tenant_id, account_id);

DO $$
DECLARE
    t TEXT;
    tenant_tables TEXT[] := ARRAY['notification','notification_preference','system_announcement','announcement_read'];
BEGIN
    FOREACH t IN ARRAY tenant_tables LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        IF t = 'system_announcement' THEN
            EXECUTE format($f$
                CREATE POLICY tenant_isolation ON %I
                    USING (tenant_id IS NULL OR tenant_id = COALESCE(NULLIF(current_setting('app.tenant_id', true), '')::BIGINT, -1))
                    WITH CHECK (tenant_id IS NULL OR tenant_id = COALESCE(NULLIF(current_setting('app.tenant_id', true), '')::BIGINT, -1))
            $f$, t);
        ELSE
            EXECUTE format($f$
                CREATE POLICY tenant_isolation ON %I
                    USING (tenant_id = current_setting('app.tenant_id')::BIGINT)
                    WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT)
            $f$, t);
        END IF;
    END LOOP;
END $$;
