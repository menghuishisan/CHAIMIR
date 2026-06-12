CREATE TABLE IF NOT EXISTS grade_level_config (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    name VARCHAR(64) NOT NULL,
    mapping JSONB NOT NULL DEFAULT '[]'::jsonb,
    warning_rules JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_grade_level_default ON grade_level_config(tenant_id) WHERE is_default;

CREATE TABLE IF NOT EXISTS semester (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    name VARCHAR(64) NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    is_current BOOLEAN NOT NULL DEFAULT false
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_semester_name ON semester(tenant_id, name);
CREATE UNIQUE INDEX IF NOT EXISTS uk_semester_current ON semester(tenant_id) WHERE is_current;

CREATE TABLE IF NOT EXISTS grade_review (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    course_id BIGINT NOT NULL,
    semester_id BIGINT NULL REFERENCES semester(id),
    submitter_id BIGINT NOT NULL,
    reviewer_id BIGINT NULL,
    status SMALLINT NOT NULL,
    is_locked BOOLEAN NOT NULL DEFAULT false,
    comment VARCHAR(500) NULL,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at TIMESTAMPTZ NULL,
    CONSTRAINT chk_grade_review_status CHECK (status IN (1,2,3))
);

CREATE INDEX IF NOT EXISTS idx_grade_review_course_status ON grade_review(tenant_id, course_id, status);

CREATE TABLE IF NOT EXISTS student_semester_grade (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    semester_id BIGINT NOT NULL REFERENCES semester(id),
    total_credits NUMERIC(5,1) NOT NULL,
    gpa NUMERIC(4,3) NOT NULL,
    cumulative_gpa NUMERIC(4,3) NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_student_semester_grade ON student_semester_grade(tenant_id, student_id, semester_id);

CREATE TABLE IF NOT EXISTS grade_appeal (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    course_id BIGINT NOT NULL,
    reason TEXT NOT NULL,
    status SMALLINT NOT NULL,
    handler_id BIGINT NULL,
    result_comment TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    handled_at TIMESTAMPTZ NULL,
    CONSTRAINT chk_grade_appeal_status CHECK (status IN (1,2,3,4))
);

CREATE INDEX IF NOT EXISTS idx_grade_appeal_student_status ON grade_appeal(tenant_id, student_id, status);

CREATE TABLE IF NOT EXISTS academic_warning (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    semester_id BIGINT NOT NULL REFERENCES semester(id),
    type SMALLINT NOT NULL,
    detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    status SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_academic_warning_type CHECK (type IN (1,2)),
    CONSTRAINT chk_academic_warning_status CHECK (status IN (1,2,3))
);

CREATE INDEX IF NOT EXISTS idx_academic_warning_student_semester ON academic_warning(tenant_id, student_id, semester_id);

CREATE TABLE IF NOT EXISTS transcript_record (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    scope SMALLINT NOT NULL,
    semester_id BIGINT NULL REFERENCES semester(id),
    pdf_ref VARCHAR(255) NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_transcript_record_scope CHECK (scope IN (1,2))
);

CREATE INDEX IF NOT EXISTS idx_transcript_record_student ON transcript_record(tenant_id, student_id, generated_at DESC);

ALTER TABLE grade_level_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE semester ENABLE ROW LEVEL SECURITY;
ALTER TABLE grade_review ENABLE ROW LEVEL SECURITY;
ALTER TABLE student_semester_grade ENABLE ROW LEVEL SECURITY;
ALTER TABLE grade_appeal ENABLE ROW LEVEL SECURITY;
ALTER TABLE academic_warning ENABLE ROW LEVEL SECURITY;
ALTER TABLE transcript_record ENABLE ROW LEVEL SECURITY;

CREATE POLICY grade_level_config_tenant_rls ON grade_level_config USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY semester_tenant_rls ON semester USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY grade_review_tenant_rls ON grade_review USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY student_semester_grade_tenant_rls ON student_semester_grade USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY grade_appeal_tenant_rls ON grade_appeal USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY academic_warning_tenant_rls ON academic_warning USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY transcript_record_tenant_rls ON transcript_record USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
