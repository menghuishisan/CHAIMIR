-- 迁移 0007:M6 教学 —— 课程、课时、作业、提交、互动与单课程成绩租户表。
-- 依据 docs/06-教学/02-数据模型.md。
-- M6 不保存题目正文/答案,只保存 M5 锁定版本引用;判题结果来自 M3。

CREATE TABLE course (
    id          BIGINT PRIMARY KEY,
    tenant_id   BIGINT       NOT NULL,
    teacher_id  BIGINT       NOT NULL,
    name        VARCHAR(255) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    type        SMALLINT     NOT NULL,
    difficulty  SMALLINT     NOT NULL,
    cover_url   VARCHAR(255) NULL,
    semester    VARCHAR(32)  NOT NULL,
    credits     NUMERIC(3,1) NOT NULL,
    schedule    JSONB        NOT NULL DEFAULT '{}'::jsonb,
    invite_code VARCHAR(16)  NOT NULL,
    status      SMALLINT     NOT NULL,
    visibility  SMALLINT     NOT NULL,
    deleted_at  TIMESTAMPTZ  NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_course_teacher_status ON course (tenant_id, teacher_id, status) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uk_course_invite_code ON course (tenant_id, invite_code) WHERE deleted_at IS NULL;
CREATE INDEX idx_course_visibility ON course (visibility, status) WHERE deleted_at IS NULL;

CREATE TABLE chapter (
    id         BIGINT PRIMARY KEY,
    tenant_id  BIGINT       NOT NULL,
    course_id  BIGINT       NOT NULL,
    title      VARCHAR(255) NOT NULL,
    sort       INT          NOT NULL DEFAULT 0,
    deleted_at TIMESTAMPTZ  NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_chapter_course_sort ON chapter (tenant_id, course_id, sort) WHERE deleted_at IS NULL;

CREATE TABLE lesson (
    id           BIGINT PRIMARY KEY,
    tenant_id    BIGINT       NOT NULL,
    chapter_id   BIGINT       NOT NULL,
    title        VARCHAR(255) NOT NULL,
    content_type SMALLINT     NOT NULL,
    content_ref  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    sort         INT          NOT NULL DEFAULT 0,
    deleted_at   TIMESTAMPTZ  NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_lesson_chapter_sort ON lesson (tenant_id, chapter_id, sort) WHERE deleted_at IS NULL;

CREATE TABLE course_member (
    id         BIGINT PRIMARY KEY,
    tenant_id  BIGINT      NOT NULL,
    course_id  BIGINT      NOT NULL,
    student_id BIGINT      NOT NULL,
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    join_mode  SMALLINT    NOT NULL
);
CREATE UNIQUE INDEX uk_course_member_student ON course_member (tenant_id, course_id, student_id);
CREATE INDEX idx_course_member_student ON course_member (tenant_id, student_id);

CREATE TABLE assignment (
    id           BIGINT PRIMARY KEY,
    tenant_id    BIGINT       NOT NULL,
    course_id    BIGINT       NOT NULL,
    title        VARCHAR(255) NOT NULL,
    chapter_id   BIGINT       NULL,
    due_at       TIMESTAMPTZ  NOT NULL,
    max_attempts INT          NOT NULL,
    late_policy  SMALLINT     NOT NULL,
    late_penalty JSONB        NOT NULL DEFAULT '{}'::jsonb,
    status       SMALLINT     NOT NULL,
    deleted_at   TIMESTAMPTZ  NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_assignment_course_status ON assignment (tenant_id, course_id, status) WHERE deleted_at IS NULL;

CREATE TABLE assignment_item (
    id            BIGINT      PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL,
    assignment_id BIGINT      NOT NULL,
    item_code     VARCHAR(96) NOT NULL,
    item_version  VARCHAR(32) NOT NULL,
    score         INT         NOT NULL,
    seq           INT         NOT NULL,
    grading_mode  SMALLINT    NOT NULL,
    judger_code   VARCHAR(64) NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_assignment_item_seq ON assignment_item (tenant_id, assignment_id, seq);
CREATE INDEX idx_assignment_item_assignment ON assignment_item (tenant_id, assignment_id, seq);

CREATE TABLE submission (
    id              BIGINT PRIMARY KEY,
    tenant_id       BIGINT       NOT NULL,
    assignment_id   BIGINT       NOT NULL,
    student_id      BIGINT       NOT NULL,
    attempt_no      INT          NOT NULL,
    content_ref     JSONB        NOT NULL DEFAULT '{}'::jsonb,
    judge_task_ref  VARCHAR(64)  NULL,
    auto_score      INT          NULL,
    manual_score    INT          NULL,
    final_score     INT          NULL,
    comment         TEXT         NULL,
    is_late         BOOLEAN      NOT NULL DEFAULT false,
    status          SMALLINT     NOT NULL,
    submitted_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_submission_assignment_student ON submission (tenant_id, assignment_id, student_id, attempt_no);
CREATE INDEX idx_submission_judge_task ON submission (tenant_id, judge_task_ref);

CREATE TABLE submission_judge_outbox (
    id              BIGINT PRIMARY KEY,
    tenant_id       BIGINT       NOT NULL,
    submission_id   BIGINT       NOT NULL,
    assignment_id   BIGINT       NOT NULL,
    student_id      BIGINT       NOT NULL,
    item_code       VARCHAR(96)  NOT NULL,
    item_version    VARCHAR(32)  NOT NULL,
    judger_code     VARCHAR(64)  NOT NULL,
    code_storage_key VARCHAR(255) NOT NULL,
    code_hash       VARCHAR(128) NOT NULL,
    extra_input     JSONB        NOT NULL DEFAULT '{}'::jsonb,
    source_ref      VARCHAR(96)  NOT NULL,
    status          SMALLINT     NOT NULL,
    retry_count     INT          NOT NULL DEFAULT 0,
    last_error      TEXT         NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_submission_judge_outbox_submission ON submission_judge_outbox (tenant_id, submission_id);
CREATE INDEX idx_submission_judge_outbox_status ON submission_judge_outbox (tenant_id, status, created_at);

CREATE TABLE submission_draft (
    id            BIGINT PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL,
    assignment_id BIGINT      NOT NULL,
    student_id    BIGINT      NOT NULL,
    content       JSONB       NOT NULL DEFAULT '{}'::jsonb,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_submission_draft_student ON submission_draft (tenant_id, assignment_id, student_id);

CREATE TABLE lesson_progress (
    id           BIGINT PRIMARY KEY,
    tenant_id    BIGINT      NOT NULL,
    lesson_id    BIGINT      NOT NULL,
    student_id   BIGINT      NOT NULL,
    status       SMALLINT    NOT NULL,
    video_pos    INT         NULL,
    duration_sec INT         NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_lesson_progress_student ON lesson_progress (tenant_id, lesson_id, student_id);

CREATE TABLE discussion_post (
    id         BIGINT PRIMARY KEY,
    tenant_id  BIGINT      NOT NULL,
    course_id  BIGINT      NOT NULL,
    parent_id  BIGINT      NULL,
    author_id  BIGINT      NOT NULL,
    content    TEXT        NOT NULL,
    is_pinned  BOOLEAN     NOT NULL DEFAULT false,
    like_count INT         NOT NULL DEFAULT 0,
    deleted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_discussion_post_course_parent ON discussion_post (tenant_id, course_id, parent_id) WHERE deleted_at IS NULL;

CREATE TABLE announcement (
    id         BIGINT PRIMARY KEY,
    tenant_id  BIGINT       NOT NULL,
    course_id  BIGINT       NOT NULL,
    title      VARCHAR(255) NOT NULL,
    content    TEXT         NOT NULL,
    is_pinned  BOOLEAN      NOT NULL DEFAULT false,
    deleted_at TIMESTAMPTZ  NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_announcement_course ON announcement (tenant_id, course_id, is_pinned, created_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE course_review (
    id         BIGINT PRIMARY KEY,
    tenant_id  BIGINT      NOT NULL,
    course_id  BIGINT      NOT NULL,
    student_id BIGINT      NOT NULL,
    rating     SMALLINT    NOT NULL,
    comment    TEXT        NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_course_review_student ON course_review (tenant_id, course_id, student_id);

CREATE TABLE grade_weight (
    id          BIGINT PRIMARY KEY,
    tenant_id   BIGINT       NOT NULL,
    course_id   BIGINT       NOT NULL,
    source_type SMALLINT     NOT NULL,
    source_ref  VARCHAR(96)  NOT NULL,
    weight      NUMERIC(5,2) NOT NULL
);
CREATE UNIQUE INDEX uk_grade_weight_source ON grade_weight (tenant_id, course_id, source_type, source_ref);

CREATE TABLE course_grade (
    id             BIGINT PRIMARY KEY,
    tenant_id      BIGINT       NOT NULL,
    course_id      BIGINT       NOT NULL,
    student_id     BIGINT       NOT NULL,
    auto_total     NUMERIC(5,2) NOT NULL,
    override_total NUMERIC(5,2) NULL,
    is_overridden  BOOLEAN      NOT NULL DEFAULT false,
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_course_grade_student ON course_grade (tenant_id, course_id, student_id);

CREATE TRIGGER trg_course_updated BEFORE UPDATE ON course FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_chapter_updated BEFORE UPDATE ON chapter FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_lesson_updated BEFORE UPDATE ON lesson FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_assignment_updated BEFORE UPDATE ON assignment FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_submission_judge_outbox_updated BEFORE UPDATE ON submission_judge_outbox FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_discussion_post_updated BEFORE UPDATE ON discussion_post FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_announcement_updated BEFORE UPDATE ON announcement FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DO $$
DECLARE
    t TEXT;
    tenant_tables TEXT[] := ARRAY[
        'course','chapter','lesson','course_member','assignment','assignment_item',
        'submission','submission_judge_outbox','submission_draft','lesson_progress','discussion_post','announcement',
        'course_review','grade_weight','course_grade'
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
