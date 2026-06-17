// teaching model 文件定义 M6 service 与 repo 之间传递的领域模型。
package teaching

import "time"

type Course struct {
	ID          int64
	TenantID    int64
	TeacherID   int64
	Name        string
	Description string
	Type        int16
	Difficulty  int16
	CoverURL    string
	Semester    string
	Credits     float64
	Schedule    map[string]any
	StartAt     time.Time
	EndAt       time.Time
	InviteCode  string
	Status      int16
	Visibility  int16
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Chapter struct {
	ID        int64
	TenantID  int64
	CourseID  int64
	Title     string
	Sort      int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Lesson struct {
	ID          int64
	TenantID    int64
	ChapterID   int64
	Title       string
	ContentType int16
	ContentRef  map[string]any
	Sort        int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CourseMember struct {
	ID        int64
	TenantID  int64
	CourseID  int64
	StudentID int64
	JoinedAt  time.Time
	JoinMode  int16
}

type Assignment struct {
	ID          int64
	TenantID    int64
	CourseID    int64
	Title       string
	ChapterID   int64
	DueAt       time.Time
	MaxAttempts int32
	LatePolicy  int16
	LatePenalty map[string]any
	Status      int16
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type AssignmentItem struct {
	ID           int64
	TenantID     int64
	AssignmentID int64
	ItemCode     string
	ItemVersion  string
	Score        int32
	Seq          int32
	GradingMode  int16
	JudgerCode   string
	CreatedAt    time.Time
}

type AssignmentDetail struct {
	Assignment Assignment
	Items      []AssignmentItemFace
}

type AssignmentItemFace struct {
	AssignmentItem
	Title      string
	Type       int16
	Difficulty int16
	Body       map[string]any
}

type Submission struct {
	ID           int64
	TenantID     int64
	AssignmentID int64
	StudentID    int64
	AttemptNo    int32
	ContentRef   map[string]any
	JudgeTaskRef string
	AutoScore    int32
	ManualScore  int32
	FinalScore   int32
	Comment      string
	IsLate       bool
	Status       int16
	SubmittedAt  time.Time
}

type JudgeOutbox struct {
	ID               int64
	TenantID         int64
	SubmissionID     int64
	AssignmentItemID int64
	AssignmentID     int64
	SourceOwnerID    int64
	SourceCourseID   int64
	SourceScope      string
	StudentID        int64
	ItemCode         string
	ItemVersion      string
	JudgerCode       string
	CodeStorageKey   string
	CodeHash         string
	ExtraInput       map[string]any
	SourceRef        string
	Status           int16
	RetryCount       int32
	LastError        string
	Score            int32
	CompletedAt      time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type SubmissionDraft struct {
	ID           int64
	TenantID     int64
	AssignmentID int64
	StudentID    int64
	Content      map[string]any
	UpdatedAt    time.Time
}

type LessonProgress struct {
	ID          int64
	TenantID    int64
	LessonID    int64
	StudentID   int64
	Status      int16
	VideoPos    int32
	DurationSec int32
	UpdatedAt   time.Time
}

type DiscussionPost struct {
	ID        int64
	TenantID  int64
	CourseID  int64
	ParentID  int64
	AuthorID  int64
	Content   string
	IsPinned  bool
	LikeCount int32
	CreatedAt time.Time
}

type Announcement struct {
	ID        int64
	TenantID  int64
	CourseID  int64
	Title     string
	Content   string
	IsPinned  bool
	CreatedAt time.Time
}

type CourseReview struct {
	ID        int64
	TenantID  int64
	CourseID  int64
	StudentID int64
	Rating    int16
	Comment   string
	CreatedAt time.Time
}

type GradeWeight struct {
	ID         int64
	TenantID   int64
	CourseID   int64
	SourceType int16
	SourceRef  string
	Weight     float64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type CourseGrade struct {
	ID            int64
	TenantID      int64
	CourseID      int64
	Semester      string
	StudentID     int64
	AutoTotal     float64
	OverrideTotal float64
	IsOverridden  bool
	IsLocked      bool
	Credits       float64
	UpdatedAt     time.Time
}

// TeachingGradeEventOutbox 是成绩变更事件的生产者 outbox 记录。
type TeachingGradeEventOutbox struct {
	ID             int64
	TenantID       int64
	CourseID       int64
	StudentID      int64
	TraceID        string
	EventUpdatedAt time.Time
	Status         int16
	RetryCount     int32
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CourseListFilter struct {
	Role   string
	Status int16
	Page   int
	Size   int
}

type ProgressStats struct {
	CourseID            int64 `json:"course_id"`
	LessonCount         int64 `json:"lesson_count"`
	CompletedCount      int64 `json:"completed_count"`
	ProgressRecordCount int64 `json:"progress_record_count"`
	DurationSec         int64 `json:"duration_sec"`
}
