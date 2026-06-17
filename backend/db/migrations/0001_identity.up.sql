CREATE TABLE IF NOT EXISTS platform_admin (
    id BIGINT PRIMARY KEY,
    username VARCHAR(64) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(64) NOT NULL,
    status SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS platform_auth_session (
    id BIGINT PRIMARY KEY,
    platform_admin_id BIGINT NOT NULL REFERENCES platform_admin(id),
    refresh_token_hash VARCHAR(64) NOT NULL UNIQUE,
    device_info VARCHAR(255),
    ip VARCHAR(64),
    status SMALLINT NOT NULL,
    expire_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tenant (
    id BIGINT PRIMARY KEY,
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    type SMALLINT NOT NULL,
    status SMALLINT NOT NULL,
    deploy_mode SMALLINT NOT NULL,
    expire_at TIMESTAMPTZ,
    logo_url VARCHAR(255),
    display_name VARCHAR(128),
    feature_flags JSONB NOT NULL DEFAULT '{}'::jsonb,
    auth_mode SMALLINT NOT NULL DEFAULT 1,
    enable_activation_code BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tenant_application (
    id BIGINT PRIMARY KEY,
    school_name VARCHAR(128) NOT NULL,
    school_type SMALLINT NOT NULL,
    contact_name VARCHAR(64) NOT NULL,
    contact_phone VARCHAR(32) NOT NULL,
    contact_email VARCHAR(128) NOT NULL,
    status SMALLINT NOT NULL,
    reject_reason VARCHAR(255),
    reviewed_by BIGINT REFERENCES platform_admin(id),
    tenant_id BIGINT REFERENCES tenant(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS department (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    name VARCHAR(128) NOT NULL,
    code VARCHAR(32),
    deleted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS major (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    department_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    deleted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, department_id) REFERENCES department(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS class (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    major_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    enrollment_year SMALLINT NOT NULL,
    status SMALLINT NOT NULL,
    deleted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, major_id) REFERENCES major(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS account (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    phone_enc BYTEA NOT NULL,
    phone_hash VARCHAR(64) NOT NULL,
    password_hash VARCHAR(255),
    name VARCHAR(64) NOT NULL,
    base_identity SMALLINT NOT NULL,
    status SMALLINT NOT NULL,
    must_change_pwd BOOLEAN NOT NULL DEFAULT false,
    pwd_failed_count SMALLINT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    activated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS account_role (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    account_id BIGINT NOT NULL,
    role SMALLINT NOT NULL,
    FOREIGN KEY (tenant_id, account_id) REFERENCES account(tenant_id, id),
    UNIQUE (tenant_id, account_id, role)
);

CREATE TABLE IF NOT EXISTS account_profile (
    account_id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    no VARCHAR(64) NOT NULL,
    org_id BIGINT NOT NULL,
    enrollment_year SMALLINT,
    title VARCHAR(64),
    FOREIGN KEY (tenant_id, account_id) REFERENCES account(tenant_id, id),
    UNIQUE (tenant_id, no)
);

CREATE TABLE IF NOT EXISTS auth_session (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    account_id BIGINT NOT NULL,
    refresh_token_hash VARCHAR(64) NOT NULL UNIQUE,
    device_info VARCHAR(255),
    ip VARCHAR(64),
    status SMALLINT NOT NULL,
    expire_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, account_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS sms_code (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    phone_hash VARCHAR(64) NOT NULL,
    code_hash VARCHAR(64) NOT NULL,
    scene SMALLINT NOT NULL,
    expire_at TIMESTAMPTZ NOT NULL,
    verify_attempts SMALLINT NOT NULL DEFAULT 0,
    used BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS activation_code (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    account_id BIGINT NOT NULL,
    code_hash VARCHAR(64) NOT NULL UNIQUE,
    status SMALLINT NOT NULL,
    expire_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, account_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS sso_config (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    type SMALLINT NOT NULL,
    config JSONB NOT NULL,
    match_field SMALLINT NOT NULL,
    enabled BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, type)
);

CREATE TABLE IF NOT EXISTS import_preview (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    operator_id BIGINT NOT NULL,
    target_type SMALLINT NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    rows JSONB NOT NULL,
    preview_result JSONB NOT NULL,
    status SMALLINT NOT NULL,
    expire_at TIMESTAMPTZ NOT NULL,
    submitted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, operator_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS import_batch (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    operator_id BIGINT NOT NULL,
    target_type SMALLINT NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    total INT NOT NULL,
    success INT NOT NULL,
    failed INT NOT NULL,
    error_detail JSONB NOT NULL,
    status SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, operator_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS audit_log (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT,
    actor_id BIGINT NOT NULL,
    actor_role SMALLINT NOT NULL,
    action VARCHAR(64) NOT NULL,
    target_type VARCHAR(64) NOT NULL,
    target_id BIGINT,
    detail JSONB NOT NULL,
    ip VARCHAR(64),
    trace_id VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_auth_session_account_status ON auth_session(account_id, status);
CREATE INDEX IF NOT EXISTS idx_auth_session_tenant_account_status ON auth_session(tenant_id, account_id, status);
CREATE INDEX IF NOT EXISTS idx_auth_session_refresh ON auth_session(refresh_token_hash);
CREATE UNIQUE INDEX IF NOT EXISTS uk_department_tenant_name_active ON department(tenant_id, name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_major_tenant_department ON major(tenant_id, department_id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_major_tenant_department_name_active ON major(tenant_id, department_id, name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_class_tenant_major ON class(tenant_id, major_id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_class_tenant_major_name_active ON class(tenant_id, major_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_account_tenant_phone_active ON account(tenant_id, phone_hash) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_account_role_tenant_account ON account_role(tenant_id, account_id);
CREATE INDEX IF NOT EXISTS idx_account_profile_tenant_account ON account_profile(tenant_id, account_id);
CREATE INDEX IF NOT EXISTS idx_activation_code_tenant_account ON activation_code(tenant_id, account_id);
CREATE INDEX IF NOT EXISTS idx_sms_code_lookup ON sms_code(tenant_id, phone_hash, scene, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_tenant_created ON audit_log(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor_created ON audit_log(actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_import_preview_operator ON import_preview(tenant_id, operator_id, status);
CREATE INDEX IF NOT EXISTS idx_import_batch_tenant_operator ON import_batch(tenant_id, operator_id);

CREATE OR REPLACE FUNCTION validate_account_profile_org()
RETURNS TRIGGER AS $$
DECLARE
    account_identity SMALLINT;
BEGIN
    SELECT base_identity
      INTO account_identity
      FROM account
     WHERE id = NEW.account_id
       AND tenant_id = NEW.tenant_id;

    IF account_identity IS NULL THEN
        RAISE EXCEPTION 'account_profile account does not belong to tenant';
    END IF;

    IF account_identity = 1 THEN
        IF NEW.enrollment_year IS NULL THEN
            RAISE EXCEPTION 'student account_profile requires enrollment_year';
        END IF;
        IF NOT EXISTS (
            SELECT 1
              FROM class c
             WHERE c.id = NEW.org_id
               AND c.tenant_id = NEW.tenant_id
               AND c.deleted_at IS NULL
        ) THEN
            RAISE EXCEPTION 'student account_profile org_id must reference active class in same tenant';
        END IF;
    ELSIF account_identity = 2 THEN
        IF NOT EXISTS (
            SELECT 1
              FROM department d
             WHERE d.id = NEW.org_id
               AND d.tenant_id = NEW.tenant_id
               AND d.deleted_at IS NULL
        ) THEN
            RAISE EXCEPTION 'teacher account_profile org_id must reference active department in same tenant';
        END IF;
    ELSE
        RAISE EXCEPTION 'account_profile base_identity is invalid';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_account_profile_org_check
BEFORE INSERT OR UPDATE OF tenant_id, account_id, org_id, enrollment_year ON account_profile
FOR EACH ROW EXECUTE FUNCTION validate_account_profile_org();

ALTER TABLE department ENABLE ROW LEVEL SECURITY;
ALTER TABLE major ENABLE ROW LEVEL SECURITY;
ALTER TABLE class ENABLE ROW LEVEL SECURITY;
ALTER TABLE account ENABLE ROW LEVEL SECURITY;
ALTER TABLE account_role ENABLE ROW LEVEL SECURITY;
ALTER TABLE account_profile ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth_session ENABLE ROW LEVEL SECURITY;
ALTER TABLE sms_code ENABLE ROW LEVEL SECURITY;
ALTER TABLE activation_code ENABLE ROW LEVEL SECURITY;
ALTER TABLE sso_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE import_preview ENABLE ROW LEVEL SECURITY;
ALTER TABLE import_batch ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;

CREATE POLICY department_tenant_rls ON department USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY major_tenant_rls ON major USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY class_tenant_rls ON class USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY account_tenant_rls ON account USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY account_role_tenant_rls ON account_role USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY account_profile_tenant_rls ON account_profile USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY auth_session_tenant_rls ON auth_session USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY sms_code_tenant_rls ON sms_code USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY activation_code_tenant_rls ON activation_code USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY sso_config_tenant_rls ON sso_config USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY import_preview_tenant_rls ON import_preview USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY import_batch_tenant_rls ON import_batch USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY audit_log_tenant_rls ON audit_log USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
