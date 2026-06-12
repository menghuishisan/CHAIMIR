// apperr grade_codes 文件定义 M11 成绩中心 B0xxx 错误码。
package apperr

const (
	// CodeGradeConfigInvalid 表示等级映射或学期配置不正确。
	CodeGradeConfigInvalid = "B0001"
	// CodeGradeReviewInvalid 表示成绩审核请求不正确。
	CodeGradeReviewInvalid = "B0002"
	// CodeGradeReviewStateInvalid 表示成绩审核状态不允许当前操作。
	CodeGradeReviewStateInvalid = "B0003"
	// CodeGradeAggregationFailed 表示 GPA 聚合失败。
	CodeGradeAggregationFailed = "B0004"
	// CodeGradeAppealInvalid 表示成绩申诉请求不正确。
	CodeGradeAppealInvalid = "B0005"
	// CodeGradeAppealExpired 表示申诉已超过受理期限。
	CodeGradeAppealExpired = "B0006"
	// CodeGradeWarningInvalid 表示学业预警请求不正确。
	CodeGradeWarningInvalid = "B0007"
	// CodeGradeTranscriptFailed 表示成绩单生成或下载失败。
	CodeGradeTranscriptFailed = "B0008"
	// CodeGradeForbidden 表示无权访问成绩中心资源。
	CodeGradeForbidden = "B0009"
)

var (
	// ErrGradeConfigInvalid 表示成绩配置不正确。
	ErrGradeConfigInvalid = New(CodeGradeConfigInvalid, "成绩配置不正确,请检查后重试")
	// ErrGradeReviewInvalid 表示成绩审核信息不正确。
	ErrGradeReviewInvalid = New(CodeGradeReviewInvalid, "成绩审核信息不正确,请检查后重试")
	// ErrGradeReviewStateInvalid 表示成绩审核状态不允许当前操作。
	ErrGradeReviewStateInvalid = New(CodeGradeReviewStateInvalid, "当前成绩审核状态不支持该操作")
	// ErrGradeAggregationFailed 表示 GPA 暂时无法重算。
	ErrGradeAggregationFailed = New(CodeGradeAggregationFailed, "成绩暂时无法汇总,请稍后重试")
	// ErrGradeAppealInvalid 表示申诉信息不正确。
	ErrGradeAppealInvalid = New(CodeGradeAppealInvalid, "申诉信息不正确,请检查后重试")
	// ErrGradeAppealExpired 表示申诉超时。
	ErrGradeAppealExpired = New(CodeGradeAppealExpired, "已超过成绩申诉期限")
	// ErrGradeWarningInvalid 表示学业预警信息不正确。
	ErrGradeWarningInvalid = New(CodeGradeWarningInvalid, "学业预警信息不正确,请检查后重试")
	// ErrGradeTranscriptFailed 表示成绩单不可用。
	ErrGradeTranscriptFailed = New(CodeGradeTranscriptFailed, "成绩单暂时无法生成或下载,请稍后重试")
	// ErrGradeForbidden 表示无法访问成绩资源。
	ErrGradeForbidden = New(CodeGradeForbidden, "无法访问该成绩信息")
)
