// apperr sim_codes 文件定义 M4 仿真可视化引擎 41xxx/42xxx/43xxx 错误码。
package apperr

const (
	// CodeSimPackageNotFound 表示仿真包不存在。
	CodeSimPackageNotFound = "41001"
	// CodeSimPackageInvalid 表示仿真包元数据或参数非法。
	CodeSimPackageInvalid = "41002"
	// CodeSimPackageVersionConflict 表示同 code/version 已存在。
	CodeSimPackageVersionConflict = "41003"
	// CodeSimPackageUnavailable 表示仿真包未上架或已下架。
	CodeSimPackageUnavailable = "41004"
	// CodeSimPackageValidationFailed 表示安全或运行校验未通过。
	CodeSimPackageValidationFailed = "41005"
	// CodeSimBundleUnreadable 表示 bundle 读取、上传或对象存储访问失败。
	CodeSimBundleUnreadable = "41006"
	// CodeSimPackageDataCorrupt 表示仿真包持久化数据与平台枚举约束不一致。
	CodeSimPackageDataCorrupt = "41007"
	// CodeSimPackageQueryFailed 表示仿真包数据读取失败。
	CodeSimPackageQueryFailed = "41008"
)

const (
	// CodeSimSessionNotFound 表示仿真会话不存在。
	CodeSimSessionNotFound = "42001"
	// CodeSimSessionInvalid 表示仿真会话参数非法。
	CodeSimSessionInvalid = "42002"
	// CodeSimSessionStateInvalid 表示当前会话状态不允许操作。
	CodeSimSessionStateInvalid = "42003"
	// CodeSimActionSeqInvalid 表示操作序列不连续或同 seq 内容冲突。
	CodeSimActionSeqInvalid = "42004"
	// CodeSimBackendComputeUnavailable 表示后端计算适配器不可用。
	CodeSimBackendComputeUnavailable = "42005"
	// CodeSimCheckpointInvalid 表示检查点上报内容非法。
	CodeSimCheckpointInvalid = "42006"
	// CodeSimShareCodeInvalid 表示分享码不存在、撤销或过期。
	CodeSimShareCodeInvalid = "42007"
	// CodeSimSessionDataCorrupt 表示仿真会话或操作持久化数据异常。
	CodeSimSessionDataCorrupt = "42008"
	// CodeSimSessionQueryFailed 表示仿真会话数据读取失败。
	CodeSimSessionQueryFailed = "42009"
	// CodeSimShareQueryFailed 表示分享数据读取失败。
	CodeSimShareQueryFailed = "42010"
)

const (
	// CodeSimReviewNotFound 表示审核记录不存在。
	CodeSimReviewNotFound = "43001"
	// CodeSimReviewStateInvalid 表示审核记录状态不允许当前操作。
	CodeSimReviewStateInvalid = "43002"
	// CodeSimReviewDataCorrupt 表示审核记录持久化数据与平台枚举约束不一致。
	CodeSimReviewDataCorrupt = "43003"
	// CodeSimReviewQueryFailed 表示审核记录数据读取失败。
	CodeSimReviewQueryFailed = "43004"
)

var (
	// ErrSimPackageNotFound 表示仿真场景不存在或暂不可用。
	ErrSimPackageNotFound = New(CodeSimPackageNotFound, "仿真场景不存在或暂不可用")
	// ErrSimPackageInvalid 表示仿真场景信息不完整。
	ErrSimPackageInvalid = New(CodeSimPackageInvalid, "仿真场景信息不完整,请检查后重试")
	// ErrSimPackageVersionConflict 表示同版本已经存在。
	ErrSimPackageVersionConflict = New(CodeSimPackageVersionConflict, "该版本已存在,请提交新的版本号")
	// ErrSimPackageUnavailable 表示仿真场景暂未上架。
	ErrSimPackageUnavailable = New(CodeSimPackageUnavailable, "仿真场景暂未上架")
	// ErrSimPackageValidationFailed 表示仿真场景未通过安全校验。
	ErrSimPackageValidationFailed = New(CodeSimPackageValidationFailed, "仿真场景未通过安全校验")
	// ErrSimBundleUnreadable 表示仿真场景资源暂时无法加载。
	ErrSimBundleUnreadable = New(CodeSimBundleUnreadable, "仿真场景资源暂时无法加载")
	// ErrSimPackageDataCorrupt 表示仿真场景数据异常。
	ErrSimPackageDataCorrupt = New(CodeSimPackageDataCorrupt, "仿真场景数据异常,请联系管理员处理")
	// ErrSimPackageQueryFailed 表示仿真场景数据读取失败。
	ErrSimPackageQueryFailed = New(CodeSimPackageQueryFailed, "仿真场景暂时无法加载,请稍后重试")
)

var (
	// ErrSimSessionNotFound 表示仿真会话不存在或已结束。
	ErrSimSessionNotFound = New(CodeSimSessionNotFound, "仿真会话不存在或已结束")
	// ErrSimSessionInvalid 表示仿真启动或操作参数不完整。
	ErrSimSessionInvalid = New(CodeSimSessionInvalid, "仿真启动参数不完整,请检查后重试")
	// ErrSimSessionStateInvalid 表示当前仿真状态不支持该操作。
	ErrSimSessionStateInvalid = New(CodeSimSessionStateInvalid, "当前仿真状态不支持该操作")
	// ErrSimActionSeqInvalid 表示操作记录顺序异常。
	ErrSimActionSeqInvalid = New(CodeSimActionSeqInvalid, "操作记录顺序异常,请刷新后重试")
	// ErrSimBackendComputeUnavailable 表示仿真计算服务暂不可用。
	ErrSimBackendComputeUnavailable = New(CodeSimBackendComputeUnavailable, "仿真计算服务暂不可用")
	// ErrSimCheckpointInvalid 表示检查点结果不完整。
	ErrSimCheckpointInvalid = New(CodeSimCheckpointInvalid, "检查点结果不完整,请重新提交")
	// ErrSimShareCodeInvalid 表示分享内容不存在或已失效。
	ErrSimShareCodeInvalid = New(CodeSimShareCodeInvalid, "分享内容不存在或已失效")
	// ErrSimSessionDataCorrupt 表示仿真会话数据异常。
	ErrSimSessionDataCorrupt = New(CodeSimSessionDataCorrupt, "仿真会话数据异常,请联系管理员处理")
	// ErrSimSessionQueryFailed 表示仿真会话数据读取失败。
	ErrSimSessionQueryFailed = New(CodeSimSessionQueryFailed, "仿真会话暂时无法加载,请稍后重试")
	// ErrSimShareQueryFailed 表示分享数据读取失败。
	ErrSimShareQueryFailed = New(CodeSimShareQueryFailed, "分享内容暂时无法加载,请稍后重试")
)

var (
	// ErrSimReviewNotFound 表示审核记录不存在或已处理。
	ErrSimReviewNotFound = New(CodeSimReviewNotFound, "审核记录不存在或已处理")
	// ErrSimReviewStateInvalid 表示当前审核状态不支持该操作。
	ErrSimReviewStateInvalid = New(CodeSimReviewStateInvalid, "当前审核状态不支持该操作")
	// ErrSimReviewDataCorrupt 表示审核数据异常。
	ErrSimReviewDataCorrupt = New(CodeSimReviewDataCorrupt, "审核数据异常,请联系管理员处理")
	// ErrSimReviewQueryFailed 表示审核数据读取失败。
	ErrSimReviewQueryFailed = New(CodeSimReviewQueryFailed, "审核记录暂时无法加载,请稍后重试")
)
