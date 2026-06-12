// teaching enum 文件定义 M6 教学模块状态、类型和审计目标常量。
package teaching

const (
	CourseTypeTheory  int16 = 1
	CourseTypeLab     int16 = 2
	CourseTypeMixed   int16 = 3
	CourseTypeProject int16 = 4
)

const (
	DifficultyIntro    int16 = 1
	DifficultyAdvanced int16 = 2
	DifficultyExpert   int16 = 3
	DifficultyResearch int16 = 4
)

const (
	CourseStatusDraft     int16 = 1
	CourseStatusPublished int16 = 2
	CourseStatusRunning   int16 = 3
	CourseStatusEnded     int16 = 4
	CourseStatusArchived  int16 = 5
)

const (
	CourseVisibilityPrivate int16 = 1
	CourseVisibilityShared  int16 = 2
)

const (
	LessonContentVideo      int16 = 1
	LessonContentMarkdown   int16 = 2
	LessonContentAttachment int16 = 3
	LessonContentExperiment int16 = 4
	LessonContentSimulation int16 = 5
)

const (
	JoinModeInvite  int16 = 1
	JoinModeTeacher int16 = 2
)

const (
	AssignmentStatusDraft     int16 = 1
	AssignmentStatusPublished int16 = 2
)

const (
	LatePolicyReject    int16 = 1
	LatePolicyPenalize  int16 = 2
	LatePolicyNoPenalty int16 = 3
)

const (
	GradingModeAuto   int16 = 1
	GradingModeManual int16 = 2
)

const (
	SubmissionStatusSubmitted int16 = 1
	SubmissionStatusPending   int16 = 2
	SubmissionStatusGraded    int16 = 3
)

const (
	OutboxStatusPending int16 = 1
	OutboxStatusRunning int16 = 2
	OutboxStatusDone    int16 = 3
)

const (
	ProgressNotStarted int16 = 1
	ProgressInProgress int16 = 2
	ProgressDone       int16 = 3
)

const (
	GradeSourceAssignment int16 = 1
	GradeSourceExperiment int16 = 2
	GradeSourceExam       int16 = 3
)

const (
	auditTargetCourse       = "teaching.course"
	auditTargetChapter      = "teaching.chapter"
	auditTargetLesson       = "teaching.lesson"
	auditTargetAssignment   = "teaching.assignment"
	auditTargetSubmission   = "teaching.submission"
	auditTargetProgress     = "teaching.progress"
	auditTargetDiscussion   = "teaching.discussion"
	auditTargetAnnouncement = "teaching.announcement"
	auditTargetReview       = "teaching.review"
	auditTargetGrade        = "teaching.grade"
)
