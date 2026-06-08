-- 迁移 0002:M1 身份与租户 —— 建表 + 索引 + RLS 策略(纯 schema)。
-- 依据 docs/01-身份与租户/02-数据模型.md。
-- 全局表(platform_admin/tenant/tenant_application):无 tenant_id,不启 RLS。
-- 租户表(13 张):含 tenant_id NOT NULL,启用 RLS 强制隔离。
-- 雪花 ID 由应用生成(BIGINT);时间 TIMESTAMPTZ;软删 deleted_at;枚举 SMALLINT。
-- 注:角色/授权(chaimir_app)不在此 —— 见 scripts/db/00_role.sql。

-- ============================================================
-- 一、全局表(不属于任何租户)
-- ============================================================

CREATE TABLE platform_admin (
    id            BIGINT PRIMARY KEY,
    username      VARCHAR(64)  NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,            -- argon2id
    name          VARCHAR(64)  NOT NULL,
    status        SMALLINT     NOT NULL DEFAULT 1,  -- 1正常/2停用
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE platform_auth_session (
    id                 BIGINT PRIMARY KEY,
    platform_admin_id  BIGINT       NOT NULL,
    refresh_token_hash VARCHAR(64)  NOT NULL,
    device_info        VARCHAR(255) NULL,
    ip                 VARCHAR(64)  NULL,
    status             SMALLINT     NOT NULL DEFAULT 1,
    expire_at          TIMESTAMPTZ  NOT NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_platform_auth_session_admin ON platform_auth_session (platform_admin_id, status);
CREATE INDEX idx_platform_auth_session_token ON platform_auth_session (refresh_token_hash);

CREATE TABLE tenant (
    id                     BIGINT PRIMARY KEY,
    code                   VARCHAR(32)  NOT NULL UNIQUE,     -- 学校短码 pku
    name                   VARCHAR(128) NOT NULL,
    type                   SMALLINT     NOT NULL,            -- 1博士/2硕士/3本科/4专科
    status                 SMALLINT     NOT NULL DEFAULT 1,  -- 1正常/2停用/3到期
    deploy_mode            SMALLINT     NOT NULL DEFAULT 1,  -- 1=SaaS/2=私有化
    expire_at              TIMESTAMPTZ  NULL,
    logo_url               VARCHAR(255) NULL,
    display_name           VARCHAR(128) NULL,
    feature_flags          JSONB        NOT NULL DEFAULT '{}'::jsonb,
    auth_mode              SMALLINT     NOT NULL DEFAULT 1,  -- 1本地/2CAS/3LDAP
    enable_activation_code BOOLEAN      NOT NULL DEFAULT false,
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE tenant_application (
    id            BIGINT PRIMARY KEY,
    school_name   VARCHAR(128) NOT NULL,
    school_type   SMALLINT     NOT NULL,
    contact_name  VARCHAR(64)  NOT NULL,
    contact_phone VARCHAR(32)  NOT NULL,
    contact_email VARCHAR(128) NOT NULL,
    status        SMALLINT     NOT NULL DEFAULT 1,  -- 1申请中/2通过/3驳回
    reject_reason VARCHAR(255) NULL,
    reviewed_by   BIGINT       NULL,                -- platform_admin.id
    tenant_id     BIGINT       NULL,                -- 通过后创建的租户
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_platform_admin_updated BEFORE UPDATE ON platform_admin
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_tenant_updated BEFORE UPDATE ON tenant
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_tenant_application_updated BEFORE UPDATE ON tenant_application
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 二、组织架构(租户表)
-- ============================================================

CREATE TABLE department (
    id         BIGINT PRIMARY KEY,
    tenant_id  BIGINT       NOT NULL,
    name       VARCHAR(128) NOT NULL,
    code       VARCHAR(32)  NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ  NULL
);
CREATE UNIQUE INDEX uk_department_tenant_name ON department (tenant_id, name) WHERE deleted_at IS NULL;
CREATE INDEX idx_department_tenant ON department (tenant_id);

CREATE TABLE major (
    id            BIGINT PRIMARY KEY,
    tenant_id     BIGINT       NOT NULL,
    department_id BIGINT       NOT NULL,
    name          VARCHAR(128) NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ  NULL
);
CREATE INDEX idx_major_tenant ON major (tenant_id);
CREATE INDEX idx_major_department ON major (tenant_id, department_id);

CREATE TABLE class (
    id              BIGINT PRIMARY KEY,
    tenant_id       BIGINT       NOT NULL,
    major_id        BIGINT       NOT NULL,
    name            VARCHAR(128) NOT NULL,
    enrollment_year SMALLINT     NOT NULL,
    status          SMALLINT     NOT NULL DEFAULT 1,  -- 1在读/2已归档
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ  NULL
);
CREATE INDEX idx_class_tenant ON class (tenant_id);
CREATE INDEX idx_class_major ON class (tenant_id, major_id);

CREATE TRIGGER trg_department_updated BEFORE UPDATE ON department
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_major_updated BEFORE UPDATE ON major
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_class_updated BEFORE UPDATE ON class
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 三、账号与角色(租户表)
-- ============================================================

CREATE TABLE account (
    id               BIGINT PRIMARY KEY,
    tenant_id        BIGINT       NOT NULL,
    phone_enc        BYTEA        NOT NULL,            -- AES-GCM 密文
    phone_hash       VARCHAR(64)  NOT NULL,            -- HMAC,唯一查询索引
    password_hash    VARCHAR(255) NULL,                -- argon2id;SSO 账号可空
    name             VARCHAR(64)  NOT NULL,
    base_identity    SMALLINT     NOT NULL,            -- 1学生/2教师(不可变)
    status           SMALLINT     NOT NULL DEFAULT 1,  -- 1待激活/2正常/3停用/4归档/5注销
    must_change_pwd  BOOLEAN      NOT NULL DEFAULT false,
    pwd_failed_count SMALLINT     NOT NULL DEFAULT 0,
    locked_until     TIMESTAMPTZ  NULL,
    activated_at     TIMESTAMPTZ  NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);
-- 一号多校:(tenant_id, phone_hash) 联合唯一,而非全局唯一。
CREATE UNIQUE INDEX uk_account_tenant_phone ON account (tenant_id, phone_hash);

CREATE TABLE account_role (
    id         BIGINT PRIMARY KEY,
    tenant_id  BIGINT   NOT NULL,
    account_id BIGINT   NOT NULL,
    role       SMALLINT NOT NULL  -- 1平台管理员/2学校管理员/3教师/4学生
);
CREATE UNIQUE INDEX uk_account_role ON account_role (tenant_id, account_id, role);
CREATE INDEX idx_account_role_account ON account_role (tenant_id, account_id);

CREATE TABLE account_profile (
    account_id      BIGINT PRIMARY KEY,
    tenant_id       BIGINT       NOT NULL,
    no              VARCHAR(64)  NOT NULL,  -- 学号/工号(不可变)
    org_id          BIGINT       NOT NULL,  -- 学生=class.id,教师=department.id
    enrollment_year SMALLINT     NULL,      -- 学生入学年份
    title           VARCHAR(64)  NULL       -- 教师职称
);
CREATE UNIQUE INDEX uk_account_profile_no ON account_profile (tenant_id, no);

CREATE TRIGGER trg_account_updated BEFORE UPDATE ON account
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 四、认证与会话(租户表)
-- ============================================================

CREATE TABLE auth_session (
    id                 BIGINT PRIMARY KEY,
    tenant_id          BIGINT       NOT NULL,
    account_id         BIGINT       NOT NULL,
    refresh_token_hash VARCHAR(64)  NOT NULL,            -- 不存明文
    device_info        VARCHAR(255) NULL,
    ip                 VARCHAR(64)  NULL,
    status             SMALLINT     NOT NULL DEFAULT 1,  -- 1有效/2已吊销
    expire_at          TIMESTAMPTZ  NOT NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_auth_session_account ON auth_session (account_id, status);
CREATE INDEX idx_auth_session_token ON auth_session (refresh_token_hash);

CREATE TABLE sms_code (
    id         BIGINT PRIMARY KEY,
    tenant_id  BIGINT       NULL,           -- 找回场景可空
    phone_hash VARCHAR(64)  NOT NULL,
    code_hash  VARCHAR(64)  NOT NULL,
    scene      SMALLINT     NOT NULL,       -- 1登录/2找回/3换绑
    expire_at  TIMESTAMPTZ  NOT NULL,
    used       BOOLEAN      NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_sms_code_phone ON sms_code (phone_hash, scene);

CREATE TABLE activation_code (
    id          BIGINT PRIMARY KEY,
    tenant_id   BIGINT      NOT NULL,
    account_id  BIGINT      NOT NULL,
    code_hash   VARCHAR(64) NOT NULL,
    status      SMALLINT    NOT NULL DEFAULT 1, -- 1有效/2已使用/3已吊销
    expire_at   TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ NULL,
    created_by  BIGINT      NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_activation_code_hash ON activation_code (code_hash);
CREATE INDEX idx_activation_code_account ON activation_code (tenant_id, account_id, status);

CREATE TABLE sso_config (
    id          BIGINT PRIMARY KEY,
    tenant_id   BIGINT      NOT NULL,
    type        SMALLINT    NOT NULL,       -- 1CAS/2LDAP
    config      JSONB       NOT NULL,       -- 协议参数(密码字段加密)
    match_field SMALLINT    NOT NULL,       -- 1学号工号/2手机号
    enabled     BOOLEAN     NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sso_config_tenant ON sso_config (tenant_id);
CREATE UNIQUE INDEX uk_sso_config_tenant ON sso_config (tenant_id);

CREATE TRIGGER trg_sso_config_updated BEFORE UPDATE ON sso_config
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 五、导入与审计(租户表)
-- ============================================================

CREATE TABLE import_preview (
    id             BIGINT PRIMARY KEY,
    tenant_id      BIGINT       NOT NULL,
    operator_id    BIGINT       NOT NULL,
    target_type    SMALLINT     NOT NULL,     -- 1教师/2学生
    file_name      VARCHAR(255) NOT NULL,
    rows           JSONB        NOT NULL DEFAULT '[]'::jsonb,
    preview_result JSONB        NOT NULL DEFAULT '{}'::jsonb,
    status         SMALLINT     NOT NULL DEFAULT 1, -- 1待提交/2已提交/3已过期
    expire_at      TIMESTAMPTZ  NOT NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    submitted_at   TIMESTAMPTZ  NULL
);
CREATE INDEX idx_import_preview_tenant_operator ON import_preview (tenant_id, operator_id, created_at DESC);
CREATE INDEX idx_import_preview_expire ON import_preview (expire_at);

CREATE TABLE import_batch (
    id           BIGINT PRIMARY KEY,
    tenant_id    BIGINT       NOT NULL,
    operator_id  BIGINT       NOT NULL,
    target_type  SMALLINT     NOT NULL,     -- 1教师/2学生/3组织
    file_name    VARCHAR(255) NOT NULL,
    total        INT          NOT NULL DEFAULT 0,
    success      INT          NOT NULL DEFAULT 0,
    failed       INT          NOT NULL DEFAULT 0,
    error_detail JSONB        NOT NULL DEFAULT '[]'::jsonb,
    status       SMALLINT     NOT NULL DEFAULT 1, -- 1处理中/2完成/3失败
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_import_batch_tenant ON import_batch (tenant_id, created_at);

-- 全平台唯一审计表(M1-M11 统一写入)。
CREATE TABLE audit_log (
    id          BIGINT PRIMARY KEY,
    tenant_id   BIGINT       NULL,          -- 平台级操作可空
    actor_id    BIGINT       NOT NULL,
    actor_role  SMALLINT     NOT NULL,
    action      VARCHAR(64)  NOT NULL,      -- 动作码 account.import/judge.run...
    target_type VARCHAR(64)  NOT NULL,      -- 对象类型(兼来源模块标识)
    target_id   BIGINT       NULL,
    detail      JSONB        NOT NULL DEFAULT '{}'::jsonb,
    ip          VARCHAR(64)  NULL,
    trace_id    VARCHAR(64)  NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_log_tenant_time ON audit_log (tenant_id, created_at);
CREATE INDEX idx_audit_log_actor_time ON audit_log (actor_id, created_at);
CREATE INDEX idx_audit_log_trace_id ON audit_log (trace_id);

-- ============================================================
-- 六、行级安全(RLS)—— 所有租户表统一启用
-- ============================================================
-- 统一模式(见 0001 注释):ENABLE(不 FORCE)+ tenant_isolation 策略。
--   不 FORCE:应用以非属主 chaimir_app 连接受约束;属主用于迁移与登录前受控跨租户查询。
-- USING 控制可见行;WITH CHECK 防写入他租户 tenant_id。
-- 用 DO 块对 13 张租户表批量套用,避免逐表重复 SQL。
DO $$
DECLARE
    t TEXT;
    tenant_tables TEXT[] := ARRAY[
        'department','major','class',
        'account','account_role','account_profile',
        'auth_session','sms_code','activation_code','sso_config',
        'import_preview','import_batch','audit_log'
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
