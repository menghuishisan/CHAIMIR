// M6 枚举定义:课程、成员、作业、提交、进度与成绩来源状态。
package teaching

const (
	CourseStatusDraft     int16 = 1
	CourseStatusPublished int16 = 2
	CourseStatusRunning   int16 = 3
	CourseStatusEnded     int16 = 4
	CourseStatusArchived  int16 = 5

	CourseVisibilityPrivate int16 = 1
	CourseVisibilityShared  int16 = 2

	JoinModeInvite  int16 = 1
	JoinModeTeacher int16 = 2

	AssignmentStatusDraft     int16 = 1
	AssignmentStatusPublished int16 = 2

	GradingModeAuto   int16 = 1
	GradingModeManual int16 = 2

	LatePolicyReject    int16 = 1
	LatePolicyDeduct    int16 = 2
	LatePolicyNoPenalty int16 = 3

	SubmissionStatusSubmitted int16 = 1
	SubmissionStatusPending   int16 = 2
	SubmissionStatusGraded    int16 = 3

	SubmissionJudgeOutboxPending int16 = 1
	SubmissionJudgeOutboxRunning int16 = 2
	SubmissionJudgeOutboxDone    int16 = 3

	ProgressNotStarted int16 = 1
	ProgressInProgress int16 = 2
	ProgressCompleted  int16 = 3

	LessonContentVideo      int16 = 1
	LessonContentMarkdown   int16 = 2
	LessonContentAttachment int16 = 3
	LessonContentExperiment int16 = 4
	LessonContentSimulation int16 = 5

	GradeSourceAssignment int16 = 1
	GradeSourceExperiment int16 = 2
	GradeSourceExam       int16 = 3
)
