-- 迁移 0012:M11 成绩中心 —— 等级配置、学期、审核、GPA 聚合、申诉、预警与成绩单元数据。
-- 依据 docs/11-成绩中心/02-数据模型.md。单课程成绩仍归 M6 course_grade,M11 只读聚合。

CREATE TABLE grade_level_config (
    id            BIGINT      PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL,
    name          VARCHAR(64) NOT NULL,
    mapping       JSONB       NOT NULL,
    warning_rules JSONB       NOT NULL DEFAULT '{}'::JSONB,
    is_default    BOOLEAN     NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ NULL
);
CREATE INDEX idx_grade_level_config_tenant ON grade_level_config (tenant_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uk_grade_level_config_default ON grade_level_config (tenant_id) WHERE is_default = true AND deleted_at IS NULL;

CREATE TABLE semester (
    id         BIGINT      PRIMARY KEY,
    tenant_id  BIGINT      NOT NULL,
    name       VARCHAR(64) NOT NULL,
    start_date DATE        NOT NULL,
    end_date   DATE        NOT NULL,
    is_current BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT ck_semester_date_range CHECK (start_date <= end_date)
);
CREATE UNIQUE INDEX uk_semester_name ON semester (tenant_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uk_semester_current ON semester (tenant_id) WHERE is_current = true AND deleted_at IS NULL;

CREATE TABLE grade_review (
    id           BIGINT       PRIMARY KEY,
    tenant_id    BIGINT       NOT NULL,
    course_id    BIGINT       NOT NULL,
    semester_id  BIGINT       NULL,
    submitter_id BIGINT       NOT NULL,
    reviewer_id  BIGINT       NULL,
    status       SMALLINT     NOT NULL,
    is_locked    BOOLEAN      NOT NULL DEFAULT false,
    comment      VARCHAR(500) NULL,
    submitted_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    reviewed_at  TIMESTAMPTZ  NULL,
    deleted_at   TIMESTAMPTZ  NULL,
    CONSTRAINT ck_grade_review_status CHECK (status IN (1, 2, 3))
);
CREATE INDEX idx_grade_review_course_status ON grade_review (tenant_id, course_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_grade_review_semester ON grade_review (tenant_id, semester_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uk_grade_review_course_active ON grade_review (tenant_id, course_id) WHERE deleted_at IS NULL;

CREATE TABLE student_semester_grade (
    id             BIGINT      PRIMARY KEY,
    tenant_id      BIGINT      NOT NULL,
    student_id     BIGINT      NOT NULL,
    semester_id    BIGINT      NOT NULL,
    total_credits  NUMERIC(5,1) NOT NULL,
    gpa            NUMERIC(4,3) NOT NULL,
    cumulative_gpa NUMERIC(4,3) NOT NULL,
    computed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_student_semester_grade_non_negative CHECK (total_credits >= 0 AND gpa >= 0 AND cumulative_gpa >= 0)
);
CREATE UNIQUE INDEX uk_student_semester_grade_student ON student_semester_grade (tenant_id, student_id, semester_id);
CREATE INDEX idx_student_semester_grade_student ON student_semester_grade (tenant_id, student_id);

CREATE TABLE grade_appeal (
    id             BIGINT      PRIMARY KEY,
    tenant_id      BIGINT      NOT NULL,
    student_id     BIGINT      NOT NULL,
    course_id      BIGINT      NOT NULL,
    reason         TEXT        NOT NULL,
    status         SMALLINT    NOT NULL,
    handler_id     BIGINT      NULL,
    result_comment TEXT        NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    handled_at     TIMESTAMPTZ NULL,
    CONSTRAINT ck_grade_appeal_status CHECK (status IN (1, 2, 3, 4))
);
CREATE INDEX idx_grade_appeal_student_status ON grade_appeal (tenant_id, student_id, status);
CREATE INDEX idx_grade_appeal_course_status ON grade_appeal (tenant_id, course_id, status);
CREATE UNIQUE INDEX uk_grade_appeal_open ON grade_appeal (tenant_id, student_id, course_id) WHERE status IN (1, 2);

CREATE TABLE academic_warning (
    id          BIGINT      PRIMARY KEY,
    tenant_id   BIGINT      NOT NULL,
    student_id  BIGINT      NOT NULL,
    semester_id BIGINT      NOT NULL,
    type        SMALLINT    NOT NULL,
    detail      JSONB       NOT NULL,
    status      SMALLINT    NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_academic_warning_type CHECK (type IN (1, 2)),
    CONSTRAINT ck_academic_warning_status CHECK (status IN (1, 2))
);
CREATE INDEX idx_academic_warning_student_semester ON academic_warning (tenant_id, student_id, semester_id);
CREATE INDEX idx_academic_warning_status ON academic_warning (tenant_id, status);

CREATE TABLE transcript_record (
    id           BIGINT       PRIMARY KEY,
    tenant_id    BIGINT       NOT NULL,
    student_id   BIGINT       NOT NULL,
    scope        SMALLINT     NOT NULL,
    semester_id  BIGINT       NULL,
    pdf_ref      VARCHAR(255) NOT NULL,
    generated_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT ck_transcript_record_scope CHECK (scope IN (1, 2))
);
CREATE INDEX idx_transcript_record_student ON transcript_record (tenant_id, student_id, generated_at DESC);

DO $$
DECLARE
    t TEXT;
    tenant_tables TEXT[] := ARRAY[
        'grade_level_config','semester','grade_review','student_semester_grade',
        'grade_appeal','academic_warning','transcript_record'
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
