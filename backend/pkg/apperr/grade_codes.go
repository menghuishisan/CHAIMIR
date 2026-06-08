// M11 成绩中心错误码(B0xxx 审核 / GPA / 申诉 / 预警 / 成绩单)。
// 文案面向终端用户,内部技术原因通过 WithCause 进入日志。
package apperr

var (
	ErrGradeConfigInvalid     = New("B0001", "成绩等级配置不完整,请检查后重试")
	ErrGradeConfigNotFound    = New("B0002", "成绩等级配置不可用,请先完成配置")
	ErrGradeSemesterInvalid   = New("B0003", "学期信息不完整,请检查后重试")
	ErrGradeReviewState       = New("B0004", "成绩审核状态不允许当前操作")
	ErrGradeReviewNotFound    = New("B0005", "成绩审核记录不存在或已被移除")
	ErrGradeAggregateFailed   = New("B0006", "成绩暂时无法聚合,请稍后重试")
	ErrGradeAppealInvalid     = New("B0007", "申诉内容不完整,请检查后重试")
	ErrGradeAppealState       = New("B0008", "当前申诉状态不允许重复处理")
	ErrGradeAppealNotFound    = New("B0009", "申诉记录不存在或已被移除")
	ErrGradeWarningFailed     = New("B0010", "学业预警暂时无法生成,请稍后重试")
	ErrGradeTranscriptFailed  = New("B0011", "成绩单暂时无法生成,请稍后重试")
	ErrGradeAppealExpired     = New("B0012", "申诉期限已过,如需帮助请联系任课教师或管理员")
	ErrGradeReviewInvalid     = New("B0013", "成绩审核信息不完整,请检查后重试")
	ErrGradeAggregateInvalid  = New("B0014", "成绩聚合参数不完整,请检查后重试")
	ErrGradeWarningInvalid    = New("B0015", "学业预警扫描条件不完整,请检查后重试")
	ErrGradeTranscriptInvalid = New("B0016", "成绩单生成信息不完整,请检查后重试")
	ErrGradeAuditWriteFailed  = New("B0017", "成绩操作审计暂时无法记录,请稍后重试")
)
