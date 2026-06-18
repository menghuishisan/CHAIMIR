// teaching dto 文件定义 M6 HTTP 请求与响应结构。
package teaching

type CourseDTO struct {
	ID          int64          `json:"id"`
	TenantID    int64          `json:"tenant_id"`
	TeacherID   int64          `json:"teacher_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Type        int16          `json:"type"`
	Difficulty  int16          `json:"difficulty"`
	CoverURL    string         `json:"cover_url,omitempty"`
	Semester    string         `json:"semester"`
	Credits     float64        `json:"credits"`
	Schedule    map[string]any `json:"schedule"`
	StartAt     string         `json:"start_at"`
	EndAt       string         `json:"end_at"`
	InviteCode  string         `json:"invite_code,omitempty"`
	Status      int16          `json:"status"`
	Visibility  int16          `json:"visibility"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

type CourseRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Type        int16          `json:"type"`
	Difficulty  int16          `json:"difficulty"`
	CoverURL    string         `json:"cover_url"`
	Semester    string         `json:"semester"`
	Credits     float64        `json:"credits"`
	Schedule    map[string]any `json:"schedule"`
	StartAt     string         `json:"start_at"`
	EndAt       string         `json:"end_at"`
}

type CloneCourseRequest struct {
	Name string `json:"name"`
}

type ChapterRequest struct {
	Title string `json:"title"`
	Sort  int32  `json:"sort"`
}

type ChapterDTO struct {
	ID        int64  `json:"id"`
	CourseID  int64  `json:"course_id"`
	Title     string `json:"title"`
	Sort      int32  `json:"sort"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type LessonRequest struct {
	Title       string         `json:"title"`
	ContentType int16          `json:"content_type"`
	ContentRef  map[string]any `json:"content_ref"`
	Sort        int32          `json:"sort"`
}

type LessonDTO struct {
	ID          int64          `json:"id"`
	ChapterID   int64          `json:"chapter_id"`
	Title       string         `json:"title"`
	ContentType int16          `json:"content_type"`
	ContentRef  map[string]any `json:"content_ref"`
	Sort        int32          `json:"sort"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

type OutlineDTO struct {
	Course   CourseDTO     `json:"course"`
	Chapters []ChapterDTO  `json:"chapters"`
	Lessons  []LessonDTO   `json:"lessons"`
	Progress []ProgressDTO `json:"progress"`
}

type JoinCourseRequest struct {
	InviteCode string `json:"invite_code"`
}

type BatchMembersRequest struct {
	StudentIDs []int64 `json:"student_ids"`
}

type MemberDTO struct {
	ID        int64  `json:"id"`
	CourseID  int64  `json:"course_id"`
	StudentID int64  `json:"student_id"`
	JoinMode  int16  `json:"join_mode"`
	JoinedAt  string `json:"joined_at"`
}

type AssignmentRequest struct {
	Title       string                `json:"title"`
	ChapterID   int64                 `json:"chapter_id"`
	DueAt       string                `json:"due_at"`
	MaxAttempts int32                 `json:"max_attempts"`
	LatePolicy  int16                 `json:"late_policy"`
	LatePenalty map[string]any        `json:"late_penalty"`
	Items       []AssignmentItemInput `json:"items"`
}

type AssignmentItemInput struct {
	ItemCode    string `json:"item_code"`
	ItemVersion string `json:"item_version"`
	Score       int32  `json:"score"`
	Seq         int32  `json:"seq"`
	GradingMode int16  `json:"grading_mode"`
	JudgerCode  string `json:"judger_code"`
}

type AssignmentDTO struct {
	ID          int64          `json:"id"`
	CourseID    int64          `json:"course_id"`
	Title       string         `json:"title"`
	ChapterID   int64          `json:"chapter_id,omitempty"`
	DueAt       string         `json:"due_at"`
	MaxAttempts int32          `json:"max_attempts"`
	LatePolicy  int16          `json:"late_policy"`
	LatePenalty map[string]any `json:"late_penalty"`
	Status      int16          `json:"status"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

type AssignmentItemDTO struct {
	ID          int64          `json:"id"`
	ItemCode    string         `json:"item_code"`
	ItemVersion string         `json:"item_version"`
	Score       int32          `json:"score"`
	Seq         int32          `json:"seq"`
	GradingMode int16          `json:"grading_mode"`
	JudgerCode  string         `json:"judger_code,omitempty"`
	Title       string         `json:"title,omitempty"`
	Type        int16          `json:"type,omitempty"`
	Difficulty  int16          `json:"difficulty,omitempty"`
	Body        map[string]any `json:"body,omitempty"`
}

type AssignmentDetailDTO struct {
	Assignment AssignmentDTO       `json:"assignment"`
	Items      []AssignmentItemDTO `json:"items"`
}

type DraftRequest struct {
	Content map[string]any `json:"content"`
}

type DraftDTO struct {
	AssignmentID int64          `json:"assignment_id"`
	StudentID    int64          `json:"student_id"`
	Content      map[string]any `json:"content"`
	UpdatedAt    string         `json:"updated_at"`
	Exists       bool           `json:"exists"`
}

type SubmitAssignmentRequest struct {
	ContentRef map[string]any `json:"content_ref"`
}

type GradeSubmissionRequest struct {
	Score   int32  `json:"score"`
	Comment string `json:"comment"`
}

type SubmissionDTO struct {
	ID           int64          `json:"id"`
	AssignmentID int64          `json:"assignment_id"`
	StudentID    int64          `json:"student_id"`
	AttemptNo    int32          `json:"attempt_no"`
	Content      map[string]any `json:"content"`
	JudgeTaskRef string         `json:"judge_task_ref,omitempty"`
	AutoScore    int32          `json:"auto_score,omitempty"`
	ManualScore  int32          `json:"manual_score,omitempty"`
	FinalScore   int32          `json:"final_score,omitempty"`
	Comment      string         `json:"comment,omitempty"`
	IsLate       bool           `json:"is_late"`
	Status       int16          `json:"status"`
	SubmittedAt  string         `json:"submitted_at"`
}

type ProgressRequest struct {
	Status      int16 `json:"status"`
	VideoPos    int32 `json:"video_pos"`
	DurationSec int32 `json:"duration_sec"`
}

type ProgressDTO struct {
	LessonID    int64  `json:"lesson_id"`
	StudentID   int64  `json:"student_id"`
	Status      int16  `json:"status"`
	VideoPos    int32  `json:"video_pos"`
	DurationSec int32  `json:"duration_sec"`
	UpdatedAt   string `json:"updated_at"`
}

type ProgressStatsDTO struct {
	CourseID            int64 `json:"course_id"`
	MemberCount         int64 `json:"member_count"`
	LessonCount         int64 `json:"lesson_count"`
	CompletedCount      int64 `json:"completed_count"`
	LearningDurationSec int64 `json:"learning_duration_sec"`
}

type PostRequest struct {
	ParentID int64  `json:"parent_id"`
	Content  string `json:"content"`
}

type PostDTO struct {
	ID        int64  `json:"id"`
	CourseID  int64  `json:"course_id"`
	ParentID  int64  `json:"parent_id,omitempty"`
	AuthorID  int64  `json:"author_id"`
	Content   string `json:"content"`
	IsPinned  bool   `json:"is_pinned"`
	LikeCount int32  `json:"like_count"`
	CreatedAt string `json:"created_at"`
}

type AnnouncementRequest struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	IsPinned bool   `json:"is_pinned"`
}

type AnnouncementDTO struct {
	ID        int64  `json:"id"`
	CourseID  int64  `json:"course_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	IsPinned  bool   `json:"is_pinned"`
	CreatedAt string `json:"created_at"`
}

type ReviewRequest struct {
	Rating  int16  `json:"rating"`
	Comment string `json:"comment"`
}

type ReviewDTO struct {
	ID        int64  `json:"id"`
	CourseID  int64  `json:"course_id"`
	StudentID int64  `json:"student_id"`
	Rating    int16  `json:"rating"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"created_at"`
}

type GradeWeightRequest struct {
	Items []GradeWeightInput `json:"items"`
}

type GradeWeightInput struct {
	SourceType int16   `json:"source_type"`
	SourceRef  string  `json:"source_ref"`
	Weight     float64 `json:"weight"`
}

type GradeWeightDTO struct {
	ID         int64   `json:"id"`
	SourceType int16   `json:"source_type"`
	SourceRef  string  `json:"source_ref"`
	Weight     float64 `json:"weight"`
}

type OverrideGradeRequest struct {
	Total float64 `json:"total"`
}

type GradeDTO struct {
	CourseID      int64   `json:"course_id"`
	StudentID     int64   `json:"student_id"`
	AutoTotal     float64 `json:"auto_total"`
	OverrideTotal float64 `json:"override_total,omitempty"`
	FinalTotal    float64 `json:"final_total"`
	IsOverridden  bool    `json:"is_overridden"`
	IsLocked      bool    `json:"is_locked"`
	Credits       float64 `json:"credits"`
	UpdatedAt     string  `json:"updated_at"`
}
