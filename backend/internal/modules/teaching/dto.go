// teaching dto 文件定义 M6 HTTP 请求与响应结构。
package teaching

import "chaimir/internal/platform/ids"

// CourseCoverUploadRequest 描述 API 层已读取并校验大小后的封面文件。
type CourseCoverUploadRequest struct {
	FileName    string
	ContentType string
	Content     []byte
}

// CourseCoverUploadDTO 是封面上传后的受控对象引用。
type CourseCoverUploadDTO struct {
	ObjectRef string `json:"object_ref"`
	FileName  string `json:"file_name"`
	Size      int64  `json:"size"`
}

// CourseCoverAccessDTO 是封面的短时投放授权响应。
type CourseCoverAccessDTO struct {
	Token     string `json:"token"`
	Mode      string `json:"mode"`
	ExpiresAt string `json:"expires_at"`
}

// LessonMaterialAccessDTO 是课时材料的短时投放授权响应。
type LessonMaterialAccessDTO struct {
	Token       string `json:"token"`
	Mode        string `json:"mode"`
	FileName    string `json:"file_name"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	ExpiresAt   string `json:"expires_at"`
}

// LessonMaterialUploadRequest 描述 API 层已读取并校验大小后的课时材料。
type LessonMaterialUploadRequest struct {
	FileName    string
	ContentType string
	Content     []byte
}

// DraftSaveDTO 是服务端草稿保存成功后的固定响应。
type DraftSaveDTO struct {
	UpdatedAt string `json:"updated_at"`
}

type CourseDTO struct {
	ID          ids.ID         `json:"id"`
	TenantID    ids.ID         `json:"tenant_id"`
	TeacherID   ids.ID         `json:"teacher_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Type        int16          `json:"type"`
	Difficulty  int16          `json:"difficulty"`
	CoverRef    string         `json:"cover_ref,omitempty"`
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
	CoverRef    string         `json:"cover_ref"`
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
	ID        ids.ID `json:"id"`
	CourseID  ids.ID `json:"course_id"`
	Title     string `json:"title"`
	Sort      int32  `json:"sort"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type LessonRequest struct {
	Title       string                  `json:"title"`
	ContentType int16                   `json:"content_type"`
	ContentRef  LessonContentRefRequest `json:"content_ref"`
	Sort        int32                   `json:"sort"`
}

// LessonContentRefRequest 是教师按课时形态提交的公开内容引用。
// 视频和附件的对象引用只能由独立上传接口写入,客户端请求中没有对象存储字段。
type LessonContentRefRequest struct {
	Markdown     string `json:"markdown,omitempty"`
	ExperimentID ids.ID `json:"experiment_id,omitempty"`
	PackageCode  string `json:"package_code,omitempty"`
	Version      string `json:"version,omitempty"`
}

type LessonDTO struct {
	ID          ids.ID              `json:"id"`
	ChapterID   ids.ID              `json:"chapter_id"`
	Title       string              `json:"title"`
	ContentType int16               `json:"content_type"`
	ContentRef  LessonContentRefDTO `json:"content_ref"`
	Sort        int32               `json:"sort"`
	CreatedAt   string              `json:"created_at"`
	UpdatedAt   string              `json:"updated_at"`
}

// LessonContentRefDTO 是课时读取时的固定内容引用结构,不暴露 object_ref。
type LessonContentRefDTO struct {
	FileName     string `json:"file_name,omitempty"`
	Size         int64  `json:"size,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
	Markdown     string `json:"markdown,omitempty"`
	ExperimentID ids.ID `json:"experiment_id,omitempty"`
	PackageCode  string `json:"package_code,omitempty"`
	Version      string `json:"version,omitempty"`
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

// BatchMembersRequest 是按班级批量添加课程成员的请求。
// 与 M6 需求 C1「教师按班级批量添加」一致:教师手上有的是班级,学生编号是内部标识,
// 由服务端经 M1 契约解析该班在校学生,不让客户端先拉一份账号目录再回传编号数组。
type BatchMembersRequest struct {
	ClassID ids.ID `json:"class_id"`
}

type MemberDTO struct {
	ID          ids.ID `json:"id"`
	CourseID    ids.ID `json:"course_id"`
	StudentID   ids.ID `json:"student_id"`
	StudentName string `json:"student_name"`
	StudentNo   string `json:"student_no,omitempty"`
	JoinMode    int16  `json:"join_mode"`
	JoinedAt    string `json:"joined_at"`
}

type AssignmentRequest struct {
	Title       string                `json:"title"`
	ChapterID   ids.ID                `json:"chapter_id"`
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
	ID          ids.ID         `json:"id"`
	CourseID    ids.ID         `json:"course_id"`
	Title       string         `json:"title"`
	ChapterID   ids.ID         `json:"chapter_id,omitempty"`
	DueAt       string         `json:"due_at"`
	MaxAttempts int32          `json:"max_attempts"`
	LatePolicy  int16          `json:"late_policy"`
	LatePenalty map[string]any `json:"late_penalty"`
	Status      int16          `json:"status"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

type AssignmentItemDTO struct {
	ID          ids.ID         `json:"id"`
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
	AssignmentID ids.ID         `json:"assignment_id"`
	StudentID    ids.ID         `json:"student_id"`
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
	ID           ids.ID         `json:"id"`
	AssignmentID ids.ID         `json:"assignment_id"`
	StudentID    ids.ID         `json:"student_id"`
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
	LessonID    ids.ID `json:"lesson_id"`
	StudentID   ids.ID `json:"student_id"`
	Status      int16  `json:"status"`
	VideoPos    int32  `json:"video_pos"`
	DurationSec int32  `json:"duration_sec"`
	UpdatedAt   string `json:"updated_at"`
}

type ProgressStatsDTO struct {
	CourseID            ids.ID `json:"course_id"`
	MemberCount         int64  `json:"member_count"`
	LessonCount         int64  `json:"lesson_count"`
	CompletedCount      int64  `json:"completed_count"`
	LearningDurationSec int64  `json:"learning_duration_sec"`
}

type PostRequest struct {
	ParentID ids.ID `json:"parent_id"`
	Content  string `json:"content"`
}

type PostDTO struct {
	ID        ids.ID `json:"id"`
	CourseID  ids.ID `json:"course_id"`
	ParentID  ids.ID `json:"parent_id,omitempty"`
	AuthorID  ids.ID `json:"author_id"`
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
	ID        ids.ID `json:"id"`
	CourseID  ids.ID `json:"course_id"`
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
	ID        ids.ID `json:"id"`
	CourseID  ids.ID `json:"course_id"`
	StudentID ids.ID `json:"student_id"`
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
	ID         ids.ID  `json:"id"`
	SourceType int16   `json:"source_type"`
	SourceRef  string  `json:"source_ref"`
	Weight     float64 `json:"weight"`
}

type OverrideGradeRequest struct {
	Total float64 `json:"total"`
}

type GradeDTO struct {
	CourseID      ids.ID  `json:"course_id"`
	StudentID     ids.ID  `json:"student_id"`
	AutoTotal     float64 `json:"auto_total"`
	OverrideTotal float64 `json:"override_total,omitempty"`
	FinalTotal    float64 `json:"final_total"`
	IsOverridden  bool    `json:"is_overridden"`
	IsLocked      bool    `json:"is_locked"`
	Credits       float64 `json:"credits"`
	UpdatedAt     string  `json:"updated_at"`
}
