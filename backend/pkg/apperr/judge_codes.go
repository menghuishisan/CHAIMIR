// M3 评测引擎错误码(31xxx 判题器 / 32xxx 任务 / 33xxx 查重)。
// 文案面向用户,内部技术原因通过 WithCause 进入日志。
package apperr

var (
	ErrJudgerNotFound       = New("31001", "判题器不存在")
	ErrJudgerUnavailable    = New("31002", "判题器暂不可用")
	ErrJudgerInvalid        = New("31003", "判题器配置不正确")
	ErrJudgerSelftestFailed = New("31004", "判题器自检未通过")
	ErrJudgerPersistence    = New("31005", "判题器配置保存失败,请稍后重试")

	ErrJudgeTaskNotFound        = New("32001", "判题任务不存在")
	ErrJudgeTaskInvalid         = New("32002", "判题请求信息有误")
	ErrJudgeTaskQueuedFail      = New("32003", "判题任务提交失败,请稍后重试")
	ErrJudgeTaskRunFail         = New("32004", "判题执行失败,请稍后重试")
	ErrJudgeTaskTimeout         = New("32005", "判题超时,请稍后重试")
	ErrJudgeTaskRateLimited     = New("32006", "提交过于频繁,请稍后再试")
	ErrJudgeTaskInvalidState    = New("32007", "判题任务当前状态不支持该操作")
	ErrJudgeManualScoreInvalid  = New("32008", "人工评分信息不正确")
	ErrJudgeConfigUnavailable   = New("32009", "题目判题配置暂不可用")
	ErrJudgeTaskPersistence     = New("32010", "判题任务状态保存失败,请稍后重试")
	ErrJudgeEventPublish        = New("32011", "判题结果通知失败,请稍后重试")
	ErrJudgeAuditFail           = New("32012", "操作记录保存失败,请稍后重试")
	ErrJudgeInputArchiveInvalid = New("32013", "判题输入文件不正确")

	ErrFingerprintNotFound = New("33001", "未找到匹配提交记录")
	ErrFingerprintInvalid  = New("33002", "查重请求信息有误")
	ErrSimilarityFailed    = New("33003", "相似度计算失败,请稍后重试")
)
