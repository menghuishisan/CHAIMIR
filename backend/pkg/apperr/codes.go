// apperr 通用错误码:定义跨模块共享的 1xxxx/115xx 平台错误。
package apperr

const (
	// CodeOK 表示请求成功。
	CodeOK = "0"
	// CodeInternal 表示未分类内部错误。
	CodeInternal = "11000"
	// CodeUnauthorized 表示用户未登录或登录态无效。
	CodeUnauthorized = "11001"
	// CodeForbidden 表示当前身份没有执行该操作的权限。
	CodeForbidden = "11002"
	// CodeCrossTenant 表示请求试图访问不属于当前租户的数据。
	CodeCrossTenant = "11003"
	// CodeBadRequest 表示请求参数不符合接口约定。
	CodeBadRequest = "11004"
	// CodeNotFound 表示请求资源不存在。
	CodeNotFound = "11005"
	// CodeConflict 表示请求与当前状态冲突。
	CodeConflict = "11006"
	// CodeRateLimited 表示请求过于频繁。
	CodeRateLimited = "11007"
	// CodeServiceUnauthorized 表示内部服务鉴权失败。
	CodeServiceUnauthorized = "11008"
	// CodeAuditActorResolveFailed 表示审计主体定位失败。
	CodeAuditActorResolveFailed = "11009"
	// CodePathIDInvalid 表示路径参数不正确。
	CodePathIDInvalid = "11010"
	// CodeRequestBodyInvalid 表示请求体格式不正确。
	CodeRequestBodyInvalid = "11011"
	// CodeQueryParamInvalid 表示查询参数格式不正确。
	CodeQueryParamInvalid = "11012"
	// CodeUnhandledFailure 表示未分类失败折叠码。
	CodeUnhandledFailure = "11501"
	// CodePanicRecovered 表示 panic 恢复后的统一错误码。
	CodePanicRecovered = "11502"
	// CodeHTTPRouterMissing 表示 HTTP 路由装配缺少 router。
	CodeHTTPRouterMissing = "11503"
	// CodeHTTPServiceMissing 表示 HTTP 路由装配缺少 service。
	CodeHTTPServiceMissing = "11504"
	// CodeHTTPAuthMissing 表示 HTTP 路由装配缺少鉴权管理器。
	CodeHTTPAuthMissing = "11505"
	// CodeEventBusMissing 表示事件订阅装配缺少事件总线。
	CodeEventBusMissing = "11506"
	// CodeEventServiceMissing 表示事件订阅装配缺少业务服务。
	CodeEventServiceMissing = "11507"
	// CodeAuditWriterMissing 表示审计装配缺少统一审计写入器。
	CodeAuditWriterMissing = "11508"
	// CodeTransferTaskInvalid 表示统一导入导出任务参数或状态非法。
	CodeTransferTaskInvalid = "11601"
	// CodeTransferTaskNotFound 表示统一导入导出任务不存在。
	CodeTransferTaskNotFound = "11602"
	// CodeTransferTaskForbidden 表示当前账号不可访问该导入导出任务。
	CodeTransferTaskForbidden = "11603"
	// CodeTransferTaskNotDownloadable 表示任务尚未产生可下载产物。
	CodeTransferTaskNotDownloadable = "11604"
)

const (
	// MessageOK 是成功响应的固定用户向文案。
	MessageOK = "ok"
	// MessageInternal 是内部错误的兜底用户向文案。
	MessageInternal = "服务暂时不可用,请稍后重试"
)

var (
	// ErrInternal 表示未知内部错误,详细原因只进入日志。
	ErrInternal = New(CodeInternal, MessageInternal)
	// ErrUnauthorized 表示用户未登录或登录态无效。
	ErrUnauthorized = New(CodeUnauthorized, "登录已失效,请重新登录")
	// ErrForbidden 表示当前账号没有执行该操作的权限。
	ErrForbidden = New(CodeForbidden, "你没有权限执行此操作")
	// ErrCrossTenant 表示租户隔离校验失败。
	ErrCrossTenant = New(CodeCrossTenant, "无法访问该资源")
	// ErrBadRequest 表示请求参数不符合接口约定。
	ErrBadRequest = New(CodeBadRequest, "请求信息有误,请检查后重试")
	// ErrNotFound 表示资源不存在。
	ErrNotFound = New(CodeNotFound, "请求的内容不存在或已被移除")
	// ErrConflict 表示状态冲突。
	ErrConflict = New(CodeConflict, "操作冲突,请刷新后重试")
	// ErrRateLimited 表示触发服务端限频保护。
	ErrRateLimited = New(CodeRateLimited, "操作过于频繁,请稍后再试")
	// ErrServiceUnauthorized 表示内部服务鉴权失败。
	ErrServiceUnauthorized = New(CodeServiceUnauthorized, "内部服务鉴权未通过")
	// ErrAuditActorResolveFailed 表示审计主体定位失败。
	ErrAuditActorResolveFailed = New(CodeAuditActorResolveFailed, "操作身份暂时无法确认,请稍后重试")
	// ErrPathIDInvalid 表示路径参数不正确。
	ErrPathIDInvalid = New(CodePathIDInvalid, "请求路径不正确,请检查后重试")
	// ErrRequestBodyInvalid 表示请求体格式不正确。
	ErrRequestBodyInvalid = New(CodeRequestBodyInvalid, "请求内容格式不正确,请检查后重试")
	// ErrQueryParamInvalid 表示查询参数格式不正确。
	ErrQueryParamInvalid = New(CodeQueryParamInvalid, "查询条件格式不正确,请检查后重试")
	// ErrUnhandledFailure 表示未分类失败折叠码。
	ErrUnhandledFailure = New(CodeUnhandledFailure, "服务暂时无法处理请求,请稍后重试")
	// ErrPanicRecovered 表示 panic 恢复后的统一错误码。
	ErrPanicRecovered = New(CodePanicRecovered, "服务暂时无法处理请求,请稍后重试")
	// ErrHTTPRouterMissing 表示 HTTP 路由装配缺少 router。
	ErrHTTPRouterMissing = New(CodeHTTPRouterMissing, "服务暂时不可用,请稍后重试")
	// ErrHTTPServiceMissing 表示 HTTP 路由装配缺少 service。
	ErrHTTPServiceMissing = New(CodeHTTPServiceMissing, "服务暂时不可用,请稍后重试")
	// ErrHTTPAuthMissing 表示 HTTP 路由装配缺少鉴权管理器。
	ErrHTTPAuthMissing = New(CodeHTTPAuthMissing, "服务暂时不可用,请稍后重试")
	// ErrEventBusMissing 表示事件订阅装配缺少事件总线。
	ErrEventBusMissing = New(CodeEventBusMissing, "服务暂时不可用,请稍后重试")
	// ErrEventServiceMissing 表示事件订阅装配缺少业务服务。
	ErrEventServiceMissing = New(CodeEventServiceMissing, "服务暂时不可用,请稍后重试")
	// ErrAuditWriterMissing 表示审计装配缺少统一审计写入器。
	ErrAuditWriterMissing = New(CodeAuditWriterMissing, "服务暂时不可用,请稍后重试")
	// ErrTransferTaskInvalid 表示统一导入导出任务信息不正确。
	ErrTransferTaskInvalid = New(CodeTransferTaskInvalid, "导入导出任务信息不正确")
	// ErrTransferTaskNotFound 表示统一导入导出任务不存在。
	ErrTransferTaskNotFound = New(CodeTransferTaskNotFound, "导入导出任务不存在或已被清理")
	// ErrTransferTaskForbidden 表示当前账号不可访问该导入导出任务。
	ErrTransferTaskForbidden = New(CodeTransferTaskForbidden, "无法访问该导入导出任务")
	// ErrTransferTaskNotDownloadable 表示任务尚未产生可下载产物。
	ErrTransferTaskNotDownloadable = New(CodeTransferTaskNotDownloadable, "文件尚未准备完成,请稍后再试")
)
