CREATE TABLE IF NOT EXISTS content_category (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    parent_id BIGINT,
    name VARCHAR(128) NOT NULL,
    sort INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, parent_id) REFERENCES content_category(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS content_item (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    code VARCHAR(96) NOT NULL,
    version VARCHAR(32) NOT NULL,
    type SMALLINT NOT NULL CHECK (type IN (1, 2, 3)),
    title VARCHAR(255) NOT NULL,
    category_id BIGINT,
    difficulty SMALLINT NOT NULL CHECK (difficulty IN (1, 2, 3, 4)),
    tags TEXT[] NOT NULL DEFAULT '{}',
    knowledge_points TEXT[] NOT NULL DEFAULT '{}',
    author_id BIGINT NOT NULL,
    author_type SMALLINT NOT NULL CHECK (author_type IN (1, 2, 3)),
    visibility SMALLINT NOT NULL CHECK (visibility IN (1, 2, 3)),
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3)),
    usage_count INT NOT NULL DEFAULT 0 CHECK (usage_count >= 0),
    version_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, author_id) REFERENCES account(tenant_id, id),
    FOREIGN KEY (tenant_id, category_id) REFERENCES content_category(tenant_id, id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_content_item_code_version_active ON content_item(tenant_id, code, version) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS content_body (
    item_id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    body JSONB NOT NULL DEFAULT '{}'::jsonb,
    sensitive_fields TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, item_id) REFERENCES content_item(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS content_usage_ref (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    item_id BIGINT NOT NULL,
    item_code VARCHAR(96) NOT NULL,
    item_version VARCHAR(32) NOT NULL,
    source_scope VARCHAR(32) NOT NULL,
    source_ref VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, source_scope, source_ref, item_code, item_version),
    FOREIGN KEY (tenant_id, item_id) REFERENCES content_item(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS paper (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    name VARCHAR(255) NOT NULL,
    author_id BIGINT NOT NULL,
    gen_mode SMALLINT NOT NULL CHECK (gen_mode IN (1, 2)),
    gen_criteria JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, author_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS paper_item (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    paper_id BIGINT NOT NULL,
    item_code VARCHAR(96) NOT NULL,
    item_version VARCHAR(32) NOT NULL,
    score INT NOT NULL CHECK (score > 0),
    seq INT NOT NULL CHECK (seq > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, paper_id, seq),
    FOREIGN KEY (tenant_id, paper_id) REFERENCES paper(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_content_category_parent ON content_category(tenant_id, parent_id, sort);
CREATE INDEX IF NOT EXISTS idx_content_item_type_status ON content_item(tenant_id, type, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_content_item_author ON content_item(tenant_id, author_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_content_item_visibility ON content_item(visibility) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_content_item_tags ON content_item USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_content_item_kps ON content_item USING GIN(knowledge_points);
CREATE INDEX IF NOT EXISTS idx_content_usage_ref_item ON content_usage_ref(tenant_id, item_id);
CREATE INDEX IF NOT EXISTS idx_content_usage_ref_source ON content_usage_ref(tenant_id, source_scope, source_ref);
CREATE INDEX IF NOT EXISTS idx_paper_author ON paper(tenant_id, author_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_paper_item_paper_seq ON paper_item(tenant_id, paper_id, seq);

ALTER TABLE content_category ENABLE ROW LEVEL SECURITY;
ALTER TABLE content_item ENABLE ROW LEVEL SECURITY;
ALTER TABLE content_body ENABLE ROW LEVEL SECURITY;
ALTER TABLE content_usage_ref ENABLE ROW LEVEL SECURITY;
ALTER TABLE paper ENABLE ROW LEVEL SECURITY;
ALTER TABLE paper_item ENABLE ROW LEVEL SECURITY;

CREATE POLICY content_category_tenant_rls ON content_category USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY content_item_select_tenant_or_shared_rls ON content_item FOR SELECT USING (
    tenant_id = current_setting('app.tenant_id')::BIGINT OR (visibility = 3 AND status = 2 AND deleted_at IS NULL)
);
CREATE POLICY content_item_insert_tenant_rls ON content_item FOR INSERT WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY content_item_update_tenant_rls ON content_item FOR UPDATE USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY content_item_delete_tenant_rls ON content_item FOR DELETE USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY content_body_select_tenant_or_shared_rls ON content_body FOR SELECT USING (
    tenant_id = current_setting('app.tenant_id')::BIGINT OR EXISTS (
        SELECT 1 FROM content_item ci WHERE ci.tenant_id = content_body.tenant_id AND ci.id = content_body.item_id AND ci.visibility = 3 AND ci.status = 2 AND ci.deleted_at IS NULL
    )
);
CREATE POLICY content_body_insert_tenant_rls ON content_body FOR INSERT WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY content_body_update_tenant_rls ON content_body FOR UPDATE USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY content_body_delete_tenant_rls ON content_body FOR DELETE USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY content_usage_ref_tenant_rls ON content_usage_ref USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY paper_tenant_rls ON paper USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY paper_item_tenant_rls ON paper_item USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
