-- 迁移 0006:M5 题库与模板中心 —— 内容外壳/内容体/分类/组卷租户表。
-- 依据 docs/05-题库与模板中心/03-数据模型.md。
-- M5 是答案与判题配置正本;题面接口由服务层过滤敏感字段。

CREATE TABLE content_item (
    id                 BIGINT PRIMARY KEY,
    tenant_id          BIGINT       NOT NULL,
    code               VARCHAR(96)  NOT NULL,
    version            VARCHAR(32)  NOT NULL,
    type               SMALLINT     NOT NULL,
    title              VARCHAR(255) NOT NULL,
    category_id        BIGINT       NULL,
    difficulty         SMALLINT     NOT NULL,
    tags               TEXT[]       NOT NULL DEFAULT ARRAY[]::TEXT[],
    knowledge_points   TEXT[]       NOT NULL DEFAULT ARRAY[]::TEXT[],
    author_id          BIGINT       NOT NULL,
    author_type        SMALLINT     NOT NULL,
    visibility         SMALLINT     NOT NULL,
    status             SMALLINT     NOT NULL,
    usage_count        INT          NOT NULL DEFAULT 0,
    body_hash          VARCHAR(64)  NOT NULL,
    deleted_at         TIMESTAMPTZ  NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_content_item_tenant_code_version ON content_item (tenant_id, code, version) WHERE deleted_at IS NULL;
CREATE INDEX idx_content_item_tenant_type_status ON content_item (tenant_id, type, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_content_item_visibility ON content_item (visibility, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_content_item_tags ON content_item USING GIN (tags);
CREATE INDEX idx_content_item_kp ON content_item USING GIN (knowledge_points);

CREATE TABLE content_body (
    item_id          BIGINT PRIMARY KEY,
    tenant_id        BIGINT      NOT NULL,
    body             JSONB       NOT NULL DEFAULT '{}'::jsonb,
    sensitive_fields TEXT[]      NOT NULL DEFAULT ARRAY[]::TEXT[],
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_content_body_tenant ON content_body (tenant_id);

CREATE TABLE content_category (
    id         BIGINT       PRIMARY KEY,
    tenant_id  BIGINT       NOT NULL,
    parent_id  BIGINT       NULL,
    name       VARCHAR(128) NOT NULL,
    sort       INT          NOT NULL DEFAULT 0,
    deleted_at TIMESTAMPTZ  NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_content_category_parent ON content_category (tenant_id, parent_id, sort) WHERE deleted_at IS NULL;

CREATE TABLE paper (
    id           BIGINT       PRIMARY KEY,
    tenant_id    BIGINT       NOT NULL,
    name         VARCHAR(255) NOT NULL,
    author_id    BIGINT       NOT NULL,
    gen_mode     SMALLINT     NOT NULL,
    gen_criteria JSONB        NOT NULL DEFAULT '{}'::jsonb,
    deleted_at   TIMESTAMPTZ  NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_paper_tenant_author ON paper (tenant_id, author_id) WHERE deleted_at IS NULL;

CREATE TABLE paper_item (
    id           BIGINT      PRIMARY KEY,
    tenant_id    BIGINT      NOT NULL,
    paper_id     BIGINT      NOT NULL,
    item_code    VARCHAR(96) NOT NULL,
    item_version VARCHAR(32) NOT NULL,
    score        INT         NOT NULL,
    seq          INT         NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_paper_item_seq ON paper_item (tenant_id, paper_id, seq);
CREATE INDEX idx_paper_item_paper ON paper_item (tenant_id, paper_id, seq);

CREATE TRIGGER trg_content_item_updated BEFORE UPDATE ON content_item
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_content_body_updated BEFORE UPDATE ON content_body
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_content_category_updated BEFORE UPDATE ON content_category
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_paper_updated BEFORE UPDATE ON paper
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DO $$
DECLARE
    t TEXT;
    tenant_tables TEXT[] := ARRAY[
        'content_item','content_body','content_category','paper','paper_item'
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
