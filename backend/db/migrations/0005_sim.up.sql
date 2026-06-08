-- 迁移 0005:M4 仿真可视化引擎 —— 仿真包/审核全局表 + 会话/操作/检查点/分享租户表。
-- 依据 docs/04-仿真可视化引擎/04-数据模型.md。
-- M4 不保存题目答案与判分规则;检查点只保存本次会话结果快照。

CREATE TABLE sim_package (
    id              BIGINT PRIMARY KEY,
    code            VARCHAR(96)  NOT NULL,
    version         VARCHAR(32)  NOT NULL,
    name            VARCHAR(128) NOT NULL,
    category        VARCHAR(32)  NOT NULL,
    compute         SMALLINT     NOT NULL,
    scale_limit     JSONB        NOT NULL DEFAULT '{}'::jsonb,
    bundle_key      VARCHAR(255) NOT NULL,
    bundle_hash     VARCHAR(64)  NOT NULL,
    backend_adapter VARCHAR(96)  NULL,
    backend_config  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    author_type     SMALLINT     NOT NULL,
    author_id       BIGINT       NULL,
    status          SMALLINT     NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_sim_package_code_version ON sim_package (code, version);
CREATE INDEX idx_sim_package_status_category ON sim_package (status, category);

CREATE TABLE sim_package_review (
    id             BIGINT PRIMARY KEY,
    package_id     BIGINT      NOT NULL,
    submitter_id   BIGINT      NOT NULL,
    preview_report JSONB       NOT NULL DEFAULT '{}'::jsonb,
    reviewer_id    BIGINT      NULL,
    result         SMALLINT    NOT NULL DEFAULT 1,
    comment        VARCHAR(500) NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sim_package_review_result ON sim_package_review (result, created_at);
CREATE INDEX idx_sim_package_review_package ON sim_package_review (package_id);

CREATE TABLE sim_session (
    id               BIGINT PRIMARY KEY,
    tenant_id        BIGINT       NOT NULL,
    package_id       BIGINT       NOT NULL,
    source_ref       VARCHAR(128) NOT NULL,
    owner_account_id BIGINT       NOT NULL,
    seed             BIGINT       NOT NULL,
    init_params      JSONB        NOT NULL DEFAULT '{}'::jsonb,
    compute          SMALLINT     NOT NULL,
    status           SMALLINT     NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_sim_session_tenant_owner ON sim_session (tenant_id, owner_account_id);
CREATE INDEX idx_sim_session_tenant_source ON sim_session (tenant_id, source_ref);
CREATE INDEX idx_sim_session_tenant_status ON sim_session (tenant_id, status);

CREATE TABLE sim_action_log (
    id         BIGINT      PRIMARY KEY,
    tenant_id  BIGINT      NOT NULL,
    session_id BIGINT      NOT NULL,
    seq        INT         NOT NULL,
    at_tick    INT         NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    payload    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_sim_action_log_seq ON sim_action_log (tenant_id, session_id, seq);
CREATE INDEX idx_sim_action_log_session ON sim_action_log (tenant_id, session_id, seq);

CREATE TABLE sim_checkpoint (
    id            BIGINT      PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL,
    session_id    BIGINT      NOT NULL,
    checkpoint_id VARCHAR(96) NOT NULL,
    answer        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    achieved      BOOLEAN     NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_sim_checkpoint_session ON sim_checkpoint (tenant_id, session_id, checkpoint_id);

CREATE TABLE sim_share (
    id         BIGINT      PRIMARY KEY,
    tenant_id  BIGINT      NOT NULL,
    session_id BIGINT      NOT NULL,
    code       VARCHAR(48) NOT NULL,
    created_by BIGINT      NOT NULL,
    status     SMALLINT    NOT NULL DEFAULT 1,
    expire_at  TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_sim_share_code ON sim_share (code);
CREATE INDEX idx_sim_share_session ON sim_share (tenant_id, session_id);

CREATE TRIGGER trg_sim_package_updated BEFORE UPDATE ON sim_package
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_sim_package_review_updated BEFORE UPDATE ON sim_package_review
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_sim_session_updated BEFORE UPDATE ON sim_session
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_sim_share_updated BEFORE UPDATE ON sim_share
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DO $$
DECLARE
    t TEXT;
    tenant_tables TEXT[] := ARRAY[
        'sim_session','sim_action_log','sim_checkpoint','sim_share'
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
