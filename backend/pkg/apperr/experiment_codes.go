// M7 实验模块错误码(71xxx 定义 / 72xxx 实例 / 73xxx 协作 / 74xxx 结果)。
// 文案面向用户,内部技术原因通过 WithCause 进入日志。
package apperr

var (
	ErrExperimentNotFound          = New("71001", "实验不存在或已被移除")
	ErrExperimentInvalid           = New("71002", "实验配置不完整,请检查后重试")
	ErrExperimentState             = New("71003", "实验当前状态不支持该操作")
	ErrExperimentForbidden         = New("71004", "你不能访问或管理该实验")
	ErrExperimentEngineFailed      = New("71005", "实验环境准备失败,请稍后重试")
	ErrExperimentStatsQueryInvalid = New("71006", "实验统计查询条件不正确,请检查后重试")
	ErrExperimentQueryFailed       = New("71007", "实验数据暂时无法读取,请稍后重试")
	ErrExperimentAuditFailed       = New("71008", "实验操作暂时无法记录,请稍后重试")

	ErrExperimentInstanceNotFound = New("72001", "实验实例不存在或已被移除")
	ErrExperimentInstanceInvalid  = New("72002", "实验实例信息不正确")
	ErrExperimentInstanceState    = New("72003", "实验实例当前状态不支持该操作")

	ErrExperimentGroupNotFound = New("73001", "协作小组不存在或已被移除")
	ErrExperimentGroupInvalid  = New("73002", "协作小组信息不正确")

	ErrCheckpointResultNotFound = New("74001", "检查点结果不存在")
	ErrCheckpointResultInvalid  = New("74002", "检查点结果信息不正确")
	ErrCheckpointJudgeFailed    = New("74003", "检查点判分失败,请稍后重试")
	ErrExperimentReportNotFound = New("74004", "实验报告不存在或已被移除")
	ErrExperimentReportInvalid  = New("74005", "实验报告信息不正确")
	ErrExperimentScoreInvalid   = New("74006", "实验得分不正确,请检查评分配置")
	ErrExperimentEventFailed    = New("74007", "实验结果暂时无法同步,请稍后重试")
	ErrExperimentEventInvalid   = New("74008", "实验事件数据不正确,请检查后重试")
)
