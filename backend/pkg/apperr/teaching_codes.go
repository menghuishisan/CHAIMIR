// apperr teaching_codes 文件定义 M6 教学 61xxx/62xxx/63xxx/64xxx 错误码。
package apperr

const (
	// CodeTeachingCourseNotFound 表示课程不存在。
	CodeTeachingCourseNotFound = "61001"
	// CodeTeachingCourseInvalid 表示课程参数非法。
	CodeTeachingCourseInvalid = "61002"
	// CodeTeachingCourseForbidden 表示无权访问或管理课程。
	CodeTeachingCourseForbidden = "61003"
	// CodeTeachingCourseStateInvalid 表示课程状态不支持当前操作。
	CodeTeachingCourseStateInvalid = "61004"
	// CodeTeachingChapterInvalid 表示章节参数或归属非法。
	CodeTeachingChapterInvalid = "61005"
	// CodeTeachingLessonInvalid 表示课时参数或归属非法。
	CodeTeachingLessonInvalid = "61006"
	// CodeTeachingMemberInvalid 表示课程成员操作非法。
	CodeTeachingMemberInvalid = "61007"
	// CodeTeachingInviteInvalid 表示邀请码不可用。
	CodeTeachingInviteInvalid = "61008"
	// CodeTeachingDiscussionInvalid 表示讨论、公告或评价操作非法。
	CodeTeachingDiscussionInvalid = "61009"
)

const (
	// CodeTeachingAssignmentNotFound 表示作业不存在。
	CodeTeachingAssignmentNotFound = "62001"
	// CodeTeachingAssignmentInvalid 表示作业参数非法。
	CodeTeachingAssignmentInvalid = "62002"
	// CodeTeachingAssignmentStateInvalid 表示作业状态不支持当前操作。
	CodeTeachingAssignmentStateInvalid = "62003"
	// CodeTeachingSubmissionInvalid 表示提交内容不符合要求。
	CodeTeachingSubmissionInvalid = "62004"
	// CodeTeachingSubmissionLimitExceeded 表示提交次数已达上限。
	CodeTeachingSubmissionLimitExceeded = "62005"
	// CodeTeachingLateSubmissionRejected 表示当前作业不允许迟交。
	CodeTeachingLateSubmissionRejected = "62006"
	// CodeTeachingJudgeOutboxInvalid 表示自动判题派发状态非法。
	CodeTeachingJudgeOutboxInvalid = "62007"
	// CodeTeachingDraftInvalid 表示作答草稿非法。
	CodeTeachingDraftInvalid = "62008"
	// CodeTeachingJudgeServiceUnavailable 表示自动判题服务未装配或暂不可用。
	CodeTeachingJudgeServiceUnavailable = "62009"
)

const (
	// CodeTeachingProgressInvalid 表示学习进度上报非法。
	CodeTeachingProgressInvalid = "63001"
)

const (
	// CodeTeachingGradeInvalid 表示成绩参数非法。
	CodeTeachingGradeInvalid = "64001"
	// CodeTeachingGradeWeightInvalid 表示成绩权重配置非法。
	CodeTeachingGradeWeightInvalid = "64002"
	// CodeTeachingGradeLocked 表示成绩处于写保护态。
	CodeTeachingGradeLocked = "64003"
	// CodeTeachingGradeExportFailed 表示成绩导出失败。
	CodeTeachingGradeExportFailed = "64004"
	// CodeTeachingGradeEventPublishFailed 表示成绩变更事件发布失败。
	CodeTeachingGradeEventPublishFailed = "64005"
)

var (
	// ErrTeachingCourseNotFound 表示课程不存在或已移除。
	ErrTeachingCourseNotFound = New(CodeTeachingCourseNotFound, "课程不存在或已移除")
	// ErrTeachingCourseInvalid 表示课程信息不完整。
	ErrTeachingCourseInvalid = New(CodeTeachingCourseInvalid, "课程信息不完整,请检查后重试")
	// ErrTeachingCourseForbidden 表示无法访问该课程。
	ErrTeachingCourseForbidden = New(CodeTeachingCourseForbidden, "无法访问该课程")
	// ErrTeachingCourseStateInvalid 表示课程状态不支持当前操作。
	ErrTeachingCourseStateInvalid = New(CodeTeachingCourseStateInvalid, "当前课程状态不支持该操作")
	// ErrTeachingChapterInvalid 表示章节信息不正确。
	ErrTeachingChapterInvalid = New(CodeTeachingChapterInvalid, "章节信息不正确")
	// ErrTeachingLessonInvalid 表示课时信息不正确。
	ErrTeachingLessonInvalid = New(CodeTeachingLessonInvalid, "课时信息不正确")
	// ErrTeachingMemberInvalid 表示课程成员信息不正确。
	ErrTeachingMemberInvalid = New(CodeTeachingMemberInvalid, "课程成员信息不正确")
	// ErrTeachingInviteInvalid 表示邀请码不可用。
	ErrTeachingInviteInvalid = New(CodeTeachingInviteInvalid, "邀请码无效或课程暂不可加入")
	// ErrTeachingDiscussionInvalid 表示讨论、公告或评价信息不正确。
	ErrTeachingDiscussionInvalid = New(CodeTeachingDiscussionInvalid, "课程互动信息不正确")
)

var (
	// ErrTeachingAssignmentNotFound 表示作业不存在或已移除。
	ErrTeachingAssignmentNotFound = New(CodeTeachingAssignmentNotFound, "作业不存在或已移除")
	// ErrTeachingAssignmentInvalid 表示作业信息不完整。
	ErrTeachingAssignmentInvalid = New(CodeTeachingAssignmentInvalid, "作业信息不完整,请检查后重试")
	// ErrTeachingAssignmentStateInvalid 表示作业状态不支持当前操作。
	ErrTeachingAssignmentStateInvalid = New(CodeTeachingAssignmentStateInvalid, "当前作业状态不支持该操作")
	// ErrTeachingSubmissionInvalid 表示提交内容不正确。
	ErrTeachingSubmissionInvalid = New(CodeTeachingSubmissionInvalid, "提交内容不正确")
	// ErrTeachingSubmissionLimitExceeded 表示提交次数已达上限。
	ErrTeachingSubmissionLimitExceeded = New(CodeTeachingSubmissionLimitExceeded, "提交次数已达上限")
	// ErrTeachingLateSubmissionRejected 表示不允许迟交。
	ErrTeachingLateSubmissionRejected = New(CodeTeachingLateSubmissionRejected, "该作业已截止,不能再提交")
	// ErrTeachingJudgeOutboxInvalid 表示自动判题派发异常。
	ErrTeachingJudgeOutboxInvalid = New(CodeTeachingJudgeOutboxInvalid, "自动判题暂时无法派发,请稍后重试")
	// ErrTeachingDraftInvalid 表示草稿内容不正确。
	ErrTeachingDraftInvalid = New(CodeTeachingDraftInvalid, "草稿内容不正确")
	// ErrTeachingJudgeServiceUnavailable 表示自动判题服务未装配或暂不可用。
	ErrTeachingJudgeServiceUnavailable = New(CodeTeachingJudgeServiceUnavailable, "自动判题服务暂不可用")
)

var (
	// ErrTeachingProgressInvalid 表示学习进度不正确。
	ErrTeachingProgressInvalid = New(CodeTeachingProgressInvalid, "学习进度信息不正确")
)

var (
	// ErrTeachingGradeInvalid 表示成绩信息不正确。
	ErrTeachingGradeInvalid = New(CodeTeachingGradeInvalid, "成绩信息不正确")
	// ErrTeachingGradeWeightInvalid 表示成绩权重不正确。
	ErrTeachingGradeWeightInvalid = New(CodeTeachingGradeWeightInvalid, "成绩权重需要合计为 100%")
	// ErrTeachingGradeLocked 表示成绩已锁定。
	ErrTeachingGradeLocked = New(CodeTeachingGradeLocked, "成绩已锁定,暂不能修改")
	// ErrTeachingGradeExportFailed 表示成绩导出失败。
	ErrTeachingGradeExportFailed = New(CodeTeachingGradeExportFailed, "成绩导出失败,请稍后重试")
	// ErrTeachingGradeEventPublishFailed 表示成绩变更事件暂时无法同步。
	ErrTeachingGradeEventPublishFailed = New(CodeTeachingGradeEventPublishFailed, "成绩变更暂时无法同步,请稍后重试")
)
