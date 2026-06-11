// apperr judge_codes 文件定义 M3 评测引擎 31xxx/32xxx/33xxx 错误码。
package apperr

const (
	// CodeJudgerNotFound 表示判题器不存在。
	CodeJudgerNotFound = "31001"
	// CodeJudgerUnavailable 表示判题器停用或未通过自检。
	CodeJudgerUnavailable = "31002"
	// CodeJudgerConfigInvalid 表示判题器配置非法。
	CodeJudgerConfigInvalid = "31003"
	// CodeJudgerSelftestFailed 表示判题器自检失败。
	CodeJudgerSelftestFailed = "31004"
	// CodeJudgerPersistFailed 表示判题器配置持久化失败。
	CodeJudgerPersistFailed = "31005"
)

const (
	// CodeJudgeTaskNotFound 表示判题任务不存在。
	CodeJudgeTaskNotFound = "32001"
	// CodeJudgeSubmitInvalid 表示判题提交参数非法。
	CodeJudgeSubmitInvalid = "32002"
	// CodeJudgeTaskEnqueueFailed 表示任务落库或入队失败。
	CodeJudgeTaskEnqueueFailed = "32003"
	// CodeJudgeWorkerFailed 表示 worker 执行失败。
	CodeJudgeWorkerFailed = "32004"
	// CodeJudgeTimeout 表示判题超时。
	CodeJudgeTimeout = "32005"
	// CodeJudgeSubmitRateLimited 表示同账号同题提交限频。
	CodeJudgeSubmitRateLimited = "32006"
	// CodeJudgeTaskStateInvalid 表示当前任务状态不允许操作。
	CodeJudgeTaskStateInvalid = "32007"
	// CodeJudgeManualScoreInvalid 表示人工评分分值非法。
	CodeJudgeManualScoreInvalid = "32008"
	// CodeJudgeSpecUnavailable 表示 M5 判题配置或对象存储不可用。
	CodeJudgeSpecUnavailable = "32009"
	// CodeJudgeTaskPersistFailed 表示判题任务状态持久化失败。
	CodeJudgeTaskPersistFailed = "32010"
	// CodeJudgeEventPublishFailed 表示判题完成或失败事件发布失败。
	CodeJudgeEventPublishFailed = "32011"
	// CodeJudgeAuditFailed 表示判题关键操作审计写入失败。
	CodeJudgeAuditFailed = "32012"
	// CodeJudgeInputArchiveInvalid 表示判题输入归档路径、类型或展开规模非法。
	CodeJudgeInputArchiveInvalid = "32013"
)

const (
	// CodeFingerprintNotFound 表示未找到匹配提交。
	CodeFingerprintNotFound = "33001"
	// CodeFingerprintRequestInvalid 表示查重请求或特征数据非法。
	CodeFingerprintRequestInvalid = "33002"
	// CodeFingerprintSimilarityFailed 表示相似度计算失败。
	CodeFingerprintSimilarityFailed = "33003"
)

var (
	// ErrJudgerNotFound 表示判题器不存在。
	ErrJudgerNotFound = New(CodeJudgerNotFound, "判题器不存在")
	// ErrJudgerUnavailable 表示判题器暂不可用。
	ErrJudgerUnavailable = New(CodeJudgerUnavailable, "判题器暂不可用")
	// ErrJudgerConfigInvalid 表示判题器配置不正确。
	ErrJudgerConfigInvalid = New(CodeJudgerConfigInvalid, "判题器配置不正确")
	// ErrJudgerSelftestFailed 表示判题器自检未通过。
	ErrJudgerSelftestFailed = New(CodeJudgerSelftestFailed, "判题器自检未通过")
	// ErrJudgerPersistFailed 表示判题器配置保存失败。
	ErrJudgerPersistFailed = New(CodeJudgerPersistFailed, "判题器配置保存失败,请稍后重试")
)

var (
	// ErrJudgeTaskNotFound 表示判题任务不存在。
	ErrJudgeTaskNotFound = New(CodeJudgeTaskNotFound, "判题任务不存在")
	// ErrJudgeSubmitInvalid 表示判题请求信息有误。
	ErrJudgeSubmitInvalid = New(CodeJudgeSubmitInvalid, "判题请求信息有误")
	// ErrJudgeTaskEnqueueFailed 表示判题任务提交失败。
	ErrJudgeTaskEnqueueFailed = New(CodeJudgeTaskEnqueueFailed, "判题任务提交失败,请稍后重试")
	// ErrJudgeWorkerFailed 表示判题执行失败。
	ErrJudgeWorkerFailed = New(CodeJudgeWorkerFailed, "判题执行失败,请稍后重试")
	// ErrJudgeTimeout 表示判题超时。
	ErrJudgeTimeout = New(CodeJudgeTimeout, "判题超时,请稍后重试")
	// ErrJudgeSubmitRateLimited 表示提交过于频繁。
	ErrJudgeSubmitRateLimited = New(CodeJudgeSubmitRateLimited, "提交过于频繁,请稍后再试")
	// ErrJudgeTaskStateInvalid 表示任务状态不支持该操作。
	ErrJudgeTaskStateInvalid = New(CodeJudgeTaskStateInvalid, "判题任务当前状态不支持该操作")
	// ErrJudgeManualScoreInvalid 表示人工评分信息不正确。
	ErrJudgeManualScoreInvalid = New(CodeJudgeManualScoreInvalid, "人工评分信息不正确")
	// ErrJudgeSpecUnavailable 表示题目判题配置暂不可用。
	ErrJudgeSpecUnavailable = New(CodeJudgeSpecUnavailable, "题目判题配置暂不可用")
	// ErrJudgeTaskPersistFailed 表示判题任务状态保存失败。
	ErrJudgeTaskPersistFailed = New(CodeJudgeTaskPersistFailed, "判题任务状态保存失败,请稍后重试")
	// ErrJudgeEventPublishFailed 表示判题结果通知失败。
	ErrJudgeEventPublishFailed = New(CodeJudgeEventPublishFailed, "判题结果通知失败,请稍后重试")
	// ErrJudgeAuditFailed 表示操作记录保存失败。
	ErrJudgeAuditFailed = New(CodeJudgeAuditFailed, "操作记录保存失败,请稍后重试")
	// ErrJudgeInputArchiveInvalid 表示判题输入文件不正确。
	ErrJudgeInputArchiveInvalid = New(CodeJudgeInputArchiveInvalid, "判题输入文件不正确")
)

var (
	// ErrFingerprintNotFound 表示未找到匹配提交记录。
	ErrFingerprintNotFound = New(CodeFingerprintNotFound, "未找到匹配提交记录")
	// ErrFingerprintRequestInvalid 表示查重请求信息有误。
	ErrFingerprintRequestInvalid = New(CodeFingerprintRequestInvalid, "查重请求信息有误")
	// ErrFingerprintSimilarityFailed 表示相似度计算失败。
	ErrFingerprintSimilarityFailed = New(CodeFingerprintSimilarityFailed, "相似度计算失败,请稍后重试")
)
