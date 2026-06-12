CREATE TABLE IF NOT EXISTS course (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    teacher_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    type SMALLINT NOT NULL CHECK (type IN (1, 2, 3, 4)),
    difficulty SMALLINT NOT NULL CHECK (difficulty IN (1, 2, 3, 4)),
    cover_url VARCHAR(255),
    semester VARCHAR(32) NOT NULL,
    credits NUMERIC(3,1) NOT NULL CHECK (credits >= 0),
    schedule JSONB NOT NULL DEFAULT '[]'::jsonb,
    invite_code VARCHAR(16) NOT NULL,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3, 4, 5)),
    visibility SMALLINT NOT NULL CHECK (visibility IN (1, 2)),
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, teacher_id) REFERENCES account(tenant_id, id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_course_invite_code_active ON course(invite_code) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS chapter (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    course_id BIGINT NOT NULL,
    title VARCHAR(255) NOT NULL,
    sort INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, course_id) REFERENCES course(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS lesson (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    chapter_id BIGINT NOT NULL,
    title VARCHAR(255) NOT NULL,
    content_type SMALLINT NOT NULL CHECK (content_type IN (1, 2, 3, 4, 5)),
    content_ref JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, chapter_id) REFERENCES chapter(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS course_member (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    course_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    join_mode SMALLINT NOT NULL CHECK (join_mode IN (1, 2)),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, course_id, student_id),
    FOREIGN KEY (tenant_id, course_id) REFERENCES course(tenant_id, id),
    FOREIGN KEY (tenant_id, student_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS assignment (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    course_id BIGINT NOT NULL,
    title VARCHAR(255) NOT NULL,
    chapter_id BIGINT,
    due_at TIMESTAMPTZ NOT NULL,
    max_attempts INT NOT NULL CHECK (max_attempts > 0),
    late_policy SMALLINT NOT NULL CHECK (late_policy IN (1, 2, 3)),
    late_penalty JSONB NOT NULL DEFAULT '{}'::jsonb,
    status SMALLINT NOT NULL CHECK (status IN (1, 2)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, course_id) REFERENCES course(tenant_id, id),
    FOREIGN KEY (tenant_id, chapter_id) REFERENCES chapter(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS assignment_item (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    assignment_id BIGINT NOT NULL,
    item_code VARCHAR(96) NOT NULL,
    item_version VARCHAR(32) NOT NULL,
    score INT NOT NULL CHECK (score > 0),
    seq INT NOT NULL CHECK (seq > 0),
    grading_mode SMALLINT NOT NULL CHECK (grading_mode IN (1, 2)),
    judger_code VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, assignment_id, seq),
    FOREIGN KEY (tenant_id, assignment_id) REFERENCES assignment(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS submission (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    assignment_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    attempt_no INT NOT NULL CHECK (attempt_no > 0),
    content_ref JSONB NOT NULL DEFAULT '{}'::jsonb,
    judge_task_ref VARCHAR(64),
    auto_score INT,
    manual_score INT,
    final_score INT,
    comment TEXT,
    is_late BOOLEAN NOT NULL DEFAULT false,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3)),
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, assignment_id, student_id, attempt_no),
    FOREIGN KEY (tenant_id, assignment_id) REFERENCES assignment(tenant_id, id),
    FOREIGN KEY (tenant_id, student_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS submission_judge_outbox (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    submission_id BIGINT NOT NULL,
    assignment_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    item_code VARCHAR(96) NOT NULL,
    item_version VARCHAR(32) NOT NULL,
    judger_code VARCHAR(64) NOT NULL,
    code_storage_key VARCHAR(255) NOT NULL,
    code_hash VARCHAR(128) NOT NULL,
    extra_input JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_ref VARCHAR(96) NOT NULL,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3)),
    retry_count INT NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, submission_id),
    FOREIGN KEY (tenant_id, submission_id) REFERENCES submission(tenant_id, id),
    FOREIGN KEY (tenant_id, assignment_id) REFERENCES assignment(tenant_id, id),
    FOREIGN KEY (tenant_id, student_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS submission_draft (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    assignment_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    content JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, assignment_id, student_id),
    FOREIGN KEY (tenant_id, assignment_id) REFERENCES assignment(tenant_id, id),
    FOREIGN KEY (tenant_id, student_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS lesson_progress (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    lesson_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    status SMALLINT NOT NULL CHECK (status IN (1, 2, 3)),
    video_pos INT,
    duration_sec INT NOT NULL DEFAULT 0 CHECK (duration_sec >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, lesson_id, student_id),
    FOREIGN KEY (tenant_id, lesson_id) REFERENCES lesson(tenant_id, id),
    FOREIGN KEY (tenant_id, student_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS discussion_post (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    course_id BIGINT NOT NULL,
    parent_id BIGINT,
    author_id BIGINT NOT NULL,
    content TEXT NOT NULL,
    is_pinned BOOLEAN NOT NULL DEFAULT false,
    like_count INT NOT NULL DEFAULT 0 CHECK (like_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, course_id) REFERENCES course(tenant_id, id),
    FOREIGN KEY (tenant_id, parent_id) REFERENCES discussion_post(tenant_id, id),
    FOREIGN KEY (tenant_id, author_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS announcement (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    course_id BIGINT NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    is_pinned BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, course_id) REFERENCES course(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS course_review (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    course_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, course_id, student_id),
    FOREIGN KEY (tenant_id, course_id) REFERENCES course(tenant_id, id),
    FOREIGN KEY (tenant_id, student_id) REFERENCES account(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS grade_weight (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    course_id BIGINT NOT NULL,
    source_type SMALLINT NOT NULL CHECK (source_type IN (1, 2, 3)),
    source_ref VARCHAR(96) NOT NULL,
    weight NUMERIC(5,2) NOT NULL CHECK (weight > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, course_id, source_type, source_ref),
    FOREIGN KEY (tenant_id, course_id) REFERENCES course(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS course_grade (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenant(id),
    course_id BIGINT NOT NULL,
    student_id BIGINT NOT NULL,
    auto_total NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (auto_total >= 0),
    override_total NUMERIC(5,2),
    is_overridden BOOLEAN NOT NULL DEFAULT false,
    is_locked BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, course_id, student_id),
    FOREIGN KEY (tenant_id, course_id) REFERENCES course(tenant_id, id),
    FOREIGN KEY (tenant_id, student_id) REFERENCES account(tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_course_teacher_status ON course(tenant_id, teacher_id, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_course_status ON course(tenant_id, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_chapter_course_sort ON chapter(tenant_id, course_id, sort) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_lesson_chapter_sort ON lesson(tenant_id, chapter_id, sort) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_course_member_student ON course_member(tenant_id, student_id);
CREATE INDEX IF NOT EXISTS idx_assignment_course ON assignment(tenant_id, course_id, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_assignment_item_assignment ON assignment_item(tenant_id, assignment_id, seq);
CREATE INDEX IF NOT EXISTS idx_submission_assignment_student ON submission(tenant_id, assignment_id, student_id);
CREATE INDEX IF NOT EXISTS idx_submission_judge_ref ON submission(tenant_id, judge_task_ref);
CREATE INDEX IF NOT EXISTS idx_submission_outbox_status ON submission_judge_outbox(tenant_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_lesson_progress_student ON lesson_progress(tenant_id, student_id);
CREATE INDEX IF NOT EXISTS idx_discussion_course_parent ON discussion_post(tenant_id, course_id, parent_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_announcement_course ON announcement(tenant_id, course_id, is_pinned, created_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_course_grade_student ON course_grade(tenant_id, student_id);

ALTER TABLE course ENABLE ROW LEVEL SECURITY;
ALTER TABLE chapter ENABLE ROW LEVEL SECURITY;
ALTER TABLE lesson ENABLE ROW LEVEL SECURITY;
ALTER TABLE course_member ENABLE ROW LEVEL SECURITY;
ALTER TABLE assignment ENABLE ROW LEVEL SECURITY;
ALTER TABLE assignment_item ENABLE ROW LEVEL SECURITY;
ALTER TABLE submission ENABLE ROW LEVEL SECURITY;
ALTER TABLE submission_judge_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE submission_draft ENABLE ROW LEVEL SECURITY;
ALTER TABLE lesson_progress ENABLE ROW LEVEL SECURITY;
ALTER TABLE discussion_post ENABLE ROW LEVEL SECURITY;
ALTER TABLE announcement ENABLE ROW LEVEL SECURITY;
ALTER TABLE course_review ENABLE ROW LEVEL SECURITY;
ALTER TABLE grade_weight ENABLE ROW LEVEL SECURITY;
ALTER TABLE course_grade ENABLE ROW LEVEL SECURITY;

CREATE POLICY course_select_tenant_or_shared_rls ON course FOR SELECT USING (tenant_id = current_setting('app.tenant_id')::BIGINT OR visibility = 2);
CREATE POLICY course_insert_tenant_rls ON course FOR INSERT WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY course_update_tenant_rls ON course FOR UPDATE USING (tenant_id = current_setting('app.tenant_id')::BIGINT) WITH CHECK (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY course_delete_tenant_rls ON course FOR DELETE USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY chapter_tenant_rls ON chapter USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY lesson_tenant_rls ON lesson USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY course_member_tenant_rls ON course_member USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY assignment_tenant_rls ON assignment USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY assignment_item_tenant_rls ON assignment_item USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY submission_tenant_rls ON submission USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY submission_judge_outbox_tenant_rls ON submission_judge_outbox USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY submission_draft_tenant_rls ON submission_draft USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY lesson_progress_tenant_rls ON lesson_progress USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY discussion_post_tenant_rls ON discussion_post USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY announcement_tenant_rls ON announcement USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY course_review_tenant_rls ON course_review USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY grade_weight_tenant_rls ON grade_weight USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
CREATE POLICY course_grade_tenant_rls ON course_grade USING (tenant_id = current_setting('app.tenant_id')::BIGINT);
