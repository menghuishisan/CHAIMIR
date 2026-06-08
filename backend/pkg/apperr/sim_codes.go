// M4 仿真可视化引擎错误码(41xxx 仿真包 / 42xxx 会话 / 43xxx 审核)。
// 文案面向用户,内部技术原因通过 WithCause 进入日志。
package apperr

var (
	ErrSimPackageNotFound        = New("41001", "仿真场景不存在或暂不可用")
	ErrSimPackageInvalid         = New("41002", "仿真场景信息不完整,请检查后重试")
	ErrSimPackageVersionConflict = New("41003", "该版本已存在,请提交新的版本号")
	ErrSimPackageUnavailable     = New("41004", "仿真场景暂未上架")
	ErrSimPackageValidationFail  = New("41005", "仿真场景未通过安全校验")
	ErrSimBundleReadFail         = New("41006", "仿真场景资源暂时无法加载")
	ErrSimBundleTooLarge         = New("41007", "仿真场景资源过大,请精简后重新上传")
	ErrSimPackageQueryFailed     = New("41008", "仿真场景列表暂时无法加载,请稍后重试")

	ErrSimSessionNotFound     = New("42001", "仿真会话不存在或已结束")
	ErrSimSessionInvalid      = New("42002", "仿真启动参数不完整,请检查后重试")
	ErrSimSessionInvalidState = New("42003", "当前仿真状态不支持该操作")
	ErrSimActionInvalid       = New("42004", "操作记录顺序异常,请刷新后重试")
	ErrSimBackendUnavailable  = New("42005", "仿真计算服务暂不可用")
	ErrSimCheckpointInvalid   = New("42006", "检查点结果不完整,请重新提交")
	ErrSimShareInvalid        = New("42007", "分享内容不存在或已失效")
	ErrSimAccessDenied        = New("42008", "无法访问该仿真会话")
	ErrSimEventPublish        = New("42009", "仿真状态暂时无法同步,请稍后重试")
	ErrSimShareCodeGenerate   = New("42010", "分享码暂时无法生成,请稍后重试")
	ErrSimReplayReadFailed    = New("42011", "回放数据暂时无法加载,请稍后重试")
	ErrSimAuditFailed         = New("42012", "操作记录暂时无法保存,请稍后重试")

	ErrSimReviewNotFound     = New("43001", "审核记录不存在或已处理")
	ErrSimReviewInvalidState = New("43002", "当前审核状态不支持该操作")
	ErrSimReviewQueryFailed  = New("43003", "审核列表暂时无法加载,请稍后重试")
)
