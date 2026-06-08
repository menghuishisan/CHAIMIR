// M6 DTO 定义:教学模块 HTTP 输入输出与服务层参数。
package teaching

import "time"

// CourseRequest 是课程创建和编辑输入。
type CourseRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Type        int16          `json:"type"`
	Difficulty  int16          `json:"difficulty"`
	CoverURL    string         `json:"cover_url"`
	Semester    string         `json:"semester"`
	Credits     float64        `json:"credits"`
	Schedule    map[string]any `json:"schedule"`
}

// CourseDTO 是课程输出。
type CourseDTO struct {
	ID          string         `json:"id"`
	TeacherID   string         `json:"teacher_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Type        int16          `json:"type"`
	Difficulty  int16          `json:"difficulty"`
	CoverURL    string         `json:"cover_url,omitempty"`
	Semester    string         `json:"semester"`
	Credits     float64        `json:"credits"`
	Schedule    map[string]any `json:"schedule"`
	InviteCode  string         `json:"invite_code,omitempty"`
	Status      int16          `json:"status"`
	Visibility  int16          `json:"visibility"`
	CreatedAt   time.Time      `json:"created_at,omitempty"`
	UpdatedAt   time.Time      `json:"updated_at,omitempty"`
}

// JoinCourseRequest 是邀请码加入课程输入。
type JoinCourseRequest struct {
	InviteCode string `json:"invite_code"`
}

// MemberBatchRequest 是教师批量添加成员输入。
type MemberBatchRequest struct {
	StudentIDs []string `json:"student_ids"`
}

// MemberDTO 是课程成员输出。
type MemberDTO struct {
	ID        string    `json:"id"`
	CourseID  string    `json:"course_id"`
	StudentID string    `json:"student_id"`
	JoinMode  int16     `json:"join_mode"`
	JoinedAt  time.Time `json:"joined_at,omitempty"`
}

// ChapterRequest 是章节输入。
type ChapterRequest struct {
	Title string `json:"title"`
	Sort  int32  `json:"sort"`
}

// ChapterDTO 是章节输出。
type ChapterDTO struct {
	ID       string      `json:"id"`
	CourseID string      `json:"course_id"`
	Title    string      `json:"title"`
	Sort     int32       `json:"sort"`
	Lessons  []LessonDTO `json:"lessons,omitempty"`
}

// LessonRequest 是课时输入。
type LessonRequest struct {
	Title       string         `json:"title"`
	ContentType int16          `json:"content_type"`
	ContentRef  map[string]any `json:"content_ref"`
	Sort        int32          `json:"sort"`
}

// LessonContentRequest 是课时内容更新输入。
type LessonContentRequest struct {
	ContentType int16          `json:"content_type"`
	ContentRef  map[string]any `json:"content_ref"`
}

// LessonDTO 是课时输出。
type LessonDTO struct {
	ID          string         `json:"id"`
	ChapterID   string         `json:"chapter_id"`
	Title       string         `json:"title"`
	ContentType int16          `json:"content_type"`
	ContentRef  map[string]any `json:"content_ref"`
	Sort        int32          `json:"sort"`
}

// AssignmentRequest 是作业创建和编辑输入。
type AssignmentRequest struct {
	Title       string                `json:"title"`
	ChapterID   string                `json:"chapter_id"`
	DueAt       time.Time             `json:"due_at"`
	MaxAttempts int32                 `json:"max_attempts"`
	LatePolicy  int16                 `json:"late_policy"`
	LatePenalty map[string]any        `json:"late_penalty"`
	Items       []AssignmentItemInput `json:"items"`
}

// AssignmentItemInput 是作业题目引用输入。
type AssignmentItemInput struct {
	ItemCode    string `json:"item_code"`
	ItemVersion string `json:"item_version"`
	Score       int32  `json:"score"`
	Seq         int32  `json:"seq"`
	GradingMode int16  `json:"grading_mode"`
	JudgerCode  string `json:"judger_code"`
}

// AssignmentDTO 是作业输出。
type AssignmentDTO struct {
	ID          string              `json:"id"`
	CourseID    string              `json:"course_id"`
	Title       string              `json:"title"`
	ChapterID   string              `json:"chapter_id,omitempty"`
	DueAt       time.Time           `json:"due_at,omitempty"`
	MaxAttempts int32               `json:"max_attempts"`
	LatePolicy  int16               `json:"late_policy"`
	LatePenalty map[string]any      `json:"late_penalty"`
	Status      int16               `json:"status"`
	Items       []AssignmentItemDTO `json:"items,omitempty"`
}

// AssignmentItemDTO 是作业题目输出。
type AssignmentItemDTO struct {
	ID          string         `json:"id"`
	ItemCode    string         `json:"item_code"`
	ItemVersion string         `json:"item_version"`
	Score       int32          `json:"score"`
	Seq         int32          `json:"seq"`
	GradingMode int16          `json:"grading_mode"`
	JudgerCode  string         `json:"judger_code,omitempty"`
	Face        map[string]any `json:"face,omitempty"`
}

// DraftRequest 是作答草稿输入。
type DraftRequest struct {
	Content map[string]any `json:"content"`
}

// SubmitRequest 是学生提交输入。
type SubmitRequest struct {
	ContentRef     map[string]any `json:"content_ref"`
	CodeStorageKey string         `json:"code_storage_key"`
	CodeHash       string         `json:"code_hash"`
	ExtraInput     map[string]any `json:"extra_input"`
}

// GradeSubmissionRequest 是教师批改提交输入。
type GradeSubmissionRequest struct {
	Score   int32  `json:"score"`
	Comment string `json:"comment"`
}

// SubmissionDTO 是提交输出。
type SubmissionDTO struct {
	ID           string         `json:"id"`
	AssignmentID string         `json:"assignment_id"`
	StudentID    string         `json:"student_id"`
	AttemptNo    int32          `json:"attempt_no"`
	ContentRef   map[string]any `json:"content_ref"`
	JudgeTaskRef string         `json:"judge_task_ref,omitempty"`
	AutoScore    *int32         `json:"auto_score,omitempty"`
	ManualScore  *int32         `json:"manual_score,omitempty"`
	FinalScore   *int32         `json:"final_score,omitempty"`
	Comment      string         `json:"comment,omitempty"`
	IsLate       bool           `json:"is_late"`
	Status       int16          `json:"status"`
	SubmittedAt  time.Time      `json:"submitted_at,omitempty"`
}

// ProgressRequest 是课时进度上报输入。
type ProgressRequest struct {
	Status      int16 `json:"status"`
	VideoPos    int32 `json:"video_pos"`
	DurationSec int32 `json:"duration_sec"`
}

// ProgressStatsDTO 是课程学习进度统计输出。
type ProgressStatsDTO struct {
	CourseID            string `json:"course_id"`
	CompletedCount      int64  `json:"completed_count"`
	InProgressCount     int64  `json:"in_progress_count"`
	LearningDurationSec int64  `json:"learning_duration_sec"`
}

// PostRequest 是讨论帖输入。
type PostRequest struct {
	ParentID string `json:"parent_id"`
	Content  string `json:"content"`
}

// PostDTO 是讨论帖输出。
type PostDTO struct {
	ID        string    `json:"id"`
	CourseID  string    `json:"course_id"`
	ParentID  string    `json:"parent_id,omitempty"`
	AuthorID  string    `json:"author_id"`
	Content   string    `json:"content"`
	IsPinned  bool      `json:"is_pinned"`
	LikeCount int32     `json:"like_count"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// AnnouncementRequest 是公告输入。
type AnnouncementRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// AnnouncementDTO 是公告输出。
type AnnouncementDTO struct {
	ID        string    `json:"id"`
	CourseID  string    `json:"course_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	IsPinned  bool      `json:"is_pinned"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// ReviewRequest 是课程评价输入。
type ReviewRequest struct {
	Rating  int16  `json:"rating"`
	Comment string `json:"comment"`
}

// CourseGradeDTO 是单课程成绩输出。
type CourseGradeDTO struct {
	ID            string   `json:"id,omitempty"`
	CourseID      string   `json:"course_id"`
	StudentID     string   `json:"student_id"`
	AutoTotal     float64  `json:"auto_total"`
	OverrideTotal *float64 `json:"override_total,omitempty"`
	FinalTotal    float64  `json:"final_total"`
	IsOverridden  bool     `json:"is_overridden"`
}

// GradeOverrideRequest 是成绩覆盖输入。
type GradeOverrideRequest struct {
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

// StatsDTO 是 M6 内部统计输出。
type StatsDTO struct {
	TenantID            string `json:"tenant_id"`
	CourseCount         int64  `json:"course_count"`
	ActiveCourseCount   int64  `json:"active_course_count"`
	LearningDurationSec int64  `json:"learning_duration_sec"`
}
