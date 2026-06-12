// grade enum 文件定义 M11 审核、申诉和学业预警枚举。
package grade

const (
	// ReviewStatusPending 表示成绩审核待审。
	ReviewStatusPending int16 = 1
	// ReviewStatusApproved 表示成绩审核已通过。
	ReviewStatusApproved int16 = 2
	// ReviewStatusRejected 表示成绩审核已驳回。
	ReviewStatusRejected int16 = 3
)

const (
	// AppealStatusPending 表示申诉待处理。
	AppealStatusPending int16 = 1
	// AppealStatusAccepted 表示申诉已受理并等待 M6 改分。
	AppealStatusAccepted int16 = 2
	// AppealStatusCompleted 表示申诉处理完成。
	AppealStatusCompleted int16 = 3
	// AppealStatusRejected 表示申诉被驳回。
	AppealStatusRejected int16 = 4
)

const (
	// WarningTypeFailedCourse 表示挂科预警。
	WarningTypeFailedCourse int16 = 1
	// WarningTypeLowGPA 表示低 GPA 预警。
	WarningTypeLowGPA int16 = 2
)

const (
	// TranscriptScopeSemester 表示学期成绩单。
	TranscriptScopeSemester int16 = 1
	// TranscriptScopeFull 表示完整成绩单。
	TranscriptScopeFull int16 = 2
)
