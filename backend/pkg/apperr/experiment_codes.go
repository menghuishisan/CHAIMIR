// apperr experiment_codes 文件定义 M7 实验模块的稳定错误码和用户向文案。
package apperr

const (
	// CodeExperimentNotFound 表示实验定义不存在或已移除。
	CodeExperimentNotFound = "71001"
	// CodeExperimentInvalid 表示实验定义请求或组件编排非法。
	CodeExperimentInvalid = "71002"
	// CodeExperimentStateInvalid 表示实验定义状态不允许当前操作。
	CodeExperimentStateInvalid = "71003"
	// CodeExperimentDependencyInvalid 表示发布前依赖校验存在阻断问题。
	CodeExperimentDependencyInvalid = "71004"
	// CodeExperimentTemplateUnavailable 表示模板锁定版本不可读取。
	CodeExperimentTemplateUnavailable = "71005"
	// CodeExperimentContentUsageFailed 表示模板或检查点引用计数更新失败。
	CodeExperimentContentUsageFailed = "71006"
	// CodeExperimentEventFailed 表示实验事件发布失败。
	CodeExperimentEventFailed = "71007"
)

const (
	// CodeExperimentInstanceNotFound 表示实验实例不存在。
	CodeExperimentInstanceNotFound = "72001"
	// CodeExperimentInstanceInvalid 表示实验实例请求参数不合法。
	CodeExperimentInstanceInvalid = "72002"
	// CodeExperimentInstanceStateInvalid 表示实例状态不允许当前操作。
	CodeExperimentInstanceStateInvalid = "72003"
	// CodeExperimentInstanceAccessDenied 表示当前账号无法访问该实例。
	CodeExperimentInstanceAccessDenied = "72004"
	// CodeExperimentSandboxUnavailable 表示沙箱契约缺失或创建失败。
	CodeExperimentSandboxUnavailable = "72005"
	// CodeExperimentSimUnavailable 表示仿真契约缺失或创建失败。
	CodeExperimentSimUnavailable = "72006"
	// CodeExperimentRecycleFailed 表示实例资源回收失败。
	CodeExperimentRecycleFailed = "72007"
	// CodeExperimentResumeFailed 表示环境释放后恢复重建失败。
	CodeExperimentResumeFailed = "72008"
	// CodeExperimentSourceRefInvalid 表示实例来源引用非法或不匹配。
	CodeExperimentSourceRefInvalid = "72009"
)

const (
	// CodeExperimentGroupNotFound 表示协作小组不存在。
	CodeExperimentGroupNotFound = "73001"
	// CodeExperimentGroupInvalid 表示协作小组或成员请求非法。
	CodeExperimentGroupInvalid = "73002"
	// CodeExperimentGroupMemberDenied 表示当前账号不是该协作小组成员。
	CodeExperimentGroupMemberDenied = "73003"
	// CodeExperimentRoleInvalid 表示协作角色不在实验角色定义中。
	CodeExperimentRoleInvalid = "73004"
)

const (
	// CodeExperimentCheckpointInvalid 表示检查点定义或触发请求非法。
	CodeExperimentCheckpointInvalid = "74001"
	// CodeExperimentJudgeUnavailable 表示评测契约缺失或判题任务提交失败。
	CodeExperimentJudgeUnavailable = "74002"
	// CodeExperimentReportInvalid 表示实验报告提交或对象引用非法。
	CodeExperimentReportInvalid = "74003"
	// CodeExperimentReportNotFound 表示实验报告不存在。
	CodeExperimentReportNotFound = "74004"
	// CodeExperimentScoreInvalid 表示得分汇总或批改分非法。
	CodeExperimentScoreInvalid = "74005"
	// CodeExperimentProgressUnavailable 表示实例进度订阅信息无法生成。
	CodeExperimentProgressUnavailable = "74006"
)

var (
	// ErrExperimentNotFound 表示实验不存在或已移除。
	ErrExperimentNotFound = New(CodeExperimentNotFound, "实验不存在或已移除")
	// ErrExperimentInvalid 表示实验信息不完整。
	ErrExperimentInvalid = New(CodeExperimentInvalid, "实验信息不完整,请检查后重试")
	// ErrExperimentStateInvalid 表示当前实验状态不支持操作。
	ErrExperimentStateInvalid = New(CodeExperimentStateInvalid, "当前实验状态不支持该操作")
	// ErrExperimentDependencyInvalid 表示发布前校验未通过。
	ErrExperimentDependencyInvalid = New(CodeExperimentDependencyInvalid, "实验依赖未通过校验,请修正后再发布")
	// ErrExperimentTemplateUnavailable 表示模板暂不可用。
	ErrExperimentTemplateUnavailable = New(CodeExperimentTemplateUnavailable, "实验模板暂时无法读取")
	// ErrExperimentContentUsageFailed 表示内容引用登记失败。
	ErrExperimentContentUsageFailed = New(CodeExperimentContentUsageFailed, "实验内容引用暂时无法确认")
	// ErrExperimentEventFailed 表示事件发布失败。
	ErrExperimentEventFailed = New(CodeExperimentEventFailed, "实验结果暂时无法同步,请稍后重试")
	// ErrExperimentInstanceNotFound 表示实例不存在。
	ErrExperimentInstanceNotFound = New(CodeExperimentInstanceNotFound, "实验实例不存在或已结束")
	// ErrExperimentInstanceInvalid 表示实例请求非法。
	ErrExperimentInstanceInvalid = New(CodeExperimentInstanceInvalid, "实验实例请求不完整,请检查后重试")
	// ErrExperimentInstanceStateInvalid 表示实例状态不支持操作。
	ErrExperimentInstanceStateInvalid = New(CodeExperimentInstanceStateInvalid, "当前实验实例状态不支持该操作")
	// ErrExperimentInstanceAccessDenied 表示实例访问被拒绝。
	ErrExperimentInstanceAccessDenied = New(CodeExperimentInstanceAccessDenied, "无法访问该实验实例")
	// ErrExperimentSandboxUnavailable 表示实验环境无法准备。
	ErrExperimentSandboxUnavailable = New(CodeExperimentSandboxUnavailable, "实验环境暂时无法准备,请稍后重试")
	// ErrExperimentSimUnavailable 表示仿真会话无法准备。
	ErrExperimentSimUnavailable = New(CodeExperimentSimUnavailable, "仿真场景暂时无法准备,请稍后重试")
	// ErrExperimentRecycleFailed 表示资源回收失败。
	ErrExperimentRecycleFailed = New(CodeExperimentRecycleFailed, "实验环境暂时无法释放,请稍后重试")
	// ErrExperimentResumeFailed 表示恢复重建失败。
	ErrExperimentResumeFailed = New(CodeExperimentResumeFailed, "实验环境恢复失败,请稍后重试")
	// ErrExperimentSourceRefInvalid 表示来源引用非法。
	ErrExperimentSourceRefInvalid = New(CodeExperimentSourceRefInvalid, "实验资源归属信息不正确")
	// ErrExperimentGroupNotFound 表示小组不存在。
	ErrExperimentGroupNotFound = New(CodeExperimentGroupNotFound, "实验小组不存在")
	// ErrExperimentGroupInvalid 表示小组信息非法。
	ErrExperimentGroupInvalid = New(CodeExperimentGroupInvalid, "实验小组信息不完整,请检查后重试")
	// ErrExperimentGroupMemberDenied 表示当前账号不是小组成员。
	ErrExperimentGroupMemberDenied = New(CodeExperimentGroupMemberDenied, "你不是该实验小组成员")
	// ErrExperimentRoleInvalid 表示角色不符合实验设置。
	ErrExperimentRoleInvalid = New(CodeExperimentRoleInvalid, "实验角色设置不正确")
	// ErrExperimentCheckpointInvalid 表示检查点请求非法。
	ErrExperimentCheckpointInvalid = New(CodeExperimentCheckpointInvalid, "检查点信息不正确")
	// ErrExperimentJudgeUnavailable 表示判分暂不可用。
	ErrExperimentJudgeUnavailable = New(CodeExperimentJudgeUnavailable, "检查点暂时无法判分,请稍后重试")
	// ErrExperimentReportInvalid 表示报告信息非法。
	ErrExperimentReportInvalid = New(CodeExperimentReportInvalid, "实验报告信息不正确")
	// ErrExperimentReportNotFound 表示报告不存在。
	ErrExperimentReportNotFound = New(CodeExperimentReportNotFound, "实验报告不存在")
	// ErrExperimentScoreInvalid 表示得分非法。
	ErrExperimentScoreInvalid = New(CodeExperimentScoreInvalid, "实验得分信息不正确")
	// ErrExperimentProgressUnavailable 表示进度订阅信息不可用。
	ErrExperimentProgressUnavailable = New(CodeExperimentProgressUnavailable, "实验进度暂时无法订阅")
)
