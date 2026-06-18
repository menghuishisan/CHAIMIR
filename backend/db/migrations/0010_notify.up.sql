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

INSERT INTO notification_template (id, type, title_tpl, content_tpl, channels, force)
VALUES
    (1001001, 'account.opened', '账号已开通', '你的平台账号已开通,请按学校通知完成登录与安全设置。', ARRAY['inbox'], true),
    (1001002, 'account.security', '账号安全提醒', '{{action}} 已完成。如非本人操作,请立即联系管理员。', ARRAY['inbox'], true),
    (1002001, 'assignment.published', '新作业已发布', '{{course}} 发布了新作业 {{assignment}},请及时查看。', ARRAY['inbox'], false),
    (1002002, 'assignment.due', '作业即将截止', '{{course}} 的 {{assignment}} 将于 {{due}} 截止,请合理安排提交。', ARRAY['inbox'], false),
    (1003001, 'experiment.timeout', '实验即将超时', '{{experiment}} 的实验环境将于 {{deadline}} 到期,请及时保存进度。', ARRAY['inbox'], false),
    (1003002, 'experiment.completed', '实验结果已更新', '{{experiment}} 的实验结果已更新,请前往课程查看。', ARRAY['inbox'], false),
    (1004001, 'contest.registration', '竞赛报名状态更新', '{{contest}} 的报名状态已更新,请查看竞赛页面。', ARRAY['inbox'], false),
    (1004002, 'contest.started', '竞赛已开始', '{{contest}} 已开始,请在规定时间内参赛。', ARRAY['inbox'], true),
    (1005001, 'grade.review', '成绩审核状态更新', '{{course}} 的成绩审核状态已更新: {{status}}。', ARRAY['inbox'], true),
    (1005002, 'grade.appeal', '成绩申诉状态更新', '{{course}} 的成绩申诉状态已更新: {{status}}。', ARRAY['inbox'], true),
    (1005003, 'grade.warning', '学业预警提醒', '你的当前 GPA 为 {{gpa}},已触发学业预警,请及时查看并处理。', ARRAY['inbox'], true),
    (1006001, 'system.maintenance', '系统维护通知', '{{title}}: {{time}}。', ARRAY['inbox'], true),
    (1006002, 'system.alert', '系统告警通知', '{{message}}', ARRAY['inbox'], true)
ON CONFLICT (type) DO UPDATE
SET title_tpl = EXCLUDED.title_tpl,
    content_tpl = EXCLUDED.content_tpl,
    channels = EXCLUDED.channels,
    force = EXCLUDED.force,
    updated_at = now();

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

CREATE POLICY notification_tenant_rls ON notification
    USING (tenant_id = current_setting('app.tenant_id')::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY notification_preference_tenant_rls ON notification_preference
    USING (tenant_id = current_setting('app.tenant_id')::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY system_announcement_tenant_rls ON system_announcement
    USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT)
    WITH CHECK (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::BIGINT);
CREATE POLICY announcement_read_tenant_rls ON announcement_read
    USING (tenant_id = current_setting('app.tenant_id')::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
