// M11 枚举常量:审核、申诉、预警与成绩单状态值。
package grade

const (
	// ReviewStatusPending 表示成绩审核待处理。
	ReviewStatusPending int16 = 1
	// ReviewStatusApproved 表示成绩审核已通过并锁定。
	ReviewStatusApproved int16 = 2
	// ReviewStatusRejected 表示成绩审核已驳回。
	ReviewStatusRejected int16 = 3

	// AppealStatusPending 表示申诉待处理。
	AppealStatusPending int16 = 1
	// AppealStatusAccepted 表示申诉已受理,等待 M6 改分事件。
	AppealStatusAccepted int16 = 2
	// AppealStatusCompleted 表示申诉已完成。
	AppealStatusCompleted int16 = 3
	// AppealStatusRejected 表示申诉已驳回。
	AppealStatusRejected int16 = 4

	// WarningTypeFailedCourse 表示挂科预警。
	WarningTypeFailedCourse int16 = 1
	// WarningTypeLowGPA 表示低 GPA 预警。
	WarningTypeLowGPA int16 = 2

	// WarningStatusPending 表示预警待处理。
	WarningStatusPending int16 = 1
	// WarningStatusAcknowledged 表示预警已知悉。
	WarningStatusAcknowledged int16 = 2

	// TranscriptScopeSemester 表示学期成绩单。
	TranscriptScopeSemester int16 = 1
	// TranscriptScopeAll 表示全部成绩单。
	TranscriptScopeAll int16 = 2
)
