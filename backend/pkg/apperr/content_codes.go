// apperr content_codes 文件定义 M5 题库与模板中心 51xxx/52xxx/53xxx/54xxx 错误码。
package apperr

const (
	// CodeContentNotFound 表示内容不存在。
	CodeContentNotFound = "51001"
	// CodeContentInvalid 表示内容请求或内容体非法。
	CodeContentInvalid = "51002"
	// CodeContentForbidden 表示无权访问该内容。
	CodeContentForbidden = "51003"
	// CodeContentStateInvalid 表示内容状态不允许当前操作。
	CodeContentStateInvalid = "51004"
	// CodeContentAnswerForbidden 表示当前身份不能读取答案或判题配置。
	CodeContentAnswerForbidden = "51005"
	// CodeContentBodyInvalid 表示类型化内容体不符合结构约束。
	CodeContentBodyInvalid = "51006"
	// CodeContentDeleteBlocked 表示内容已被引用,不可删除。
	CodeContentDeleteBlocked = "51007"
	// CodeContentCategoryInvalid 表示分类参数非法。
	CodeContentCategoryInvalid = "51008"
	// CodeContentSystemImportInvalid 表示系统导入来源或内容非法。
	CodeContentSystemImportInvalid = "51009"
	// CodeContentQueryInvalid 表示内容查询条件非法。
	CodeContentQueryInvalid = "51010"
	// CodeContentFullAccessDenied 表示 full 接口身份不满足教师作者或内部服务要求。
	CodeContentFullAccessDenied = "51011"
)

const (
	// CodeContentVersionConflict 表示同 code/version 已存在。
	CodeContentVersionConflict = "52001"
	// CodeContentVersionInvalid 表示版本号非法。
	CodeContentVersionInvalid = "52002"
	// CodeContentVersionImmutable 表示已发布版本不可修改。
	CodeContentVersionImmutable = "52003"
	// CodeContentVersionNotPublished 表示锁定版本不可被新引用。
	CodeContentVersionNotPublished = "52004"
)

const (
	// CodeContentShareInvalid 表示共享状态非法。
	CodeContentShareInvalid = "53001"
	// CodeContentCloneInvalid 表示克隆来源或目标非法。
	CodeContentCloneInvalid = "53002"
	// CodeContentSharedNotFound 表示共享库内容不存在。
	CodeContentSharedNotFound = "53003"
)

const (
	// CodePaperNotFound 表示试卷不存在。
	CodePaperNotFound = "54001"
	// CodePaperInvalid 表示试卷参数非法。
	CodePaperInvalid = "54002"
	// CodePaperItemInvalid 表示试卷题目引用或分值非法。
	CodePaperItemInvalid = "54003"
	// CodePaperGenerateFailed 表示随机组卷失败。
	CodePaperGenerateFailed = "54004"
	// CodePaperRegenerateFailed 表示重新组卷失败。
	CodePaperRegenerateFailed = "54005"
	// CodePaperPickNotEnough 表示满足条件的题目数量不足。
	CodePaperPickNotEnough = "54006"
)

var (
	// ErrContentNotFound 表示内容不存在或已移除。
	ErrContentNotFound = New(CodeContentNotFound, "内容不存在或已移除")
	// ErrContentInvalid 表示内容信息不完整。
	ErrContentInvalid = New(CodeContentInvalid, "内容信息不完整,请检查后重试")
	// ErrContentForbidden 表示无法访问该内容。
	ErrContentForbidden = New(CodeContentForbidden, "无法访问该内容")
	// ErrContentStateInvalid 表示当前内容状态不支持该操作。
	ErrContentStateInvalid = New(CodeContentStateInvalid, "当前内容状态不支持该操作")
	// ErrContentAnswerForbidden 表示当前身份不能查看答案。
	ErrContentAnswerForbidden = New(CodeContentAnswerForbidden, "当前身份不能查看答案或判题配置")
	// ErrContentBodyInvalid 表示内容正文不符合要求。
	ErrContentBodyInvalid = New(CodeContentBodyInvalid, "内容正文不符合要求")
	// ErrContentDeleteBlocked 表示内容已被引用。
	ErrContentDeleteBlocked = New(CodeContentDeleteBlocked, "内容已被引用,不能删除")
	// ErrContentCategoryInvalid 表示分类信息不正确。
	ErrContentCategoryInvalid = New(CodeContentCategoryInvalid, "分类信息不正确")
	// ErrContentSystemImportInvalid 表示系统导入内容不正确。
	ErrContentSystemImportInvalid = New(CodeContentSystemImportInvalid, "系统导入内容不正确")
	// ErrContentQueryInvalid 表示查询条件不正确。
	ErrContentQueryInvalid = New(CodeContentQueryInvalid, "查询条件不正确")
	// ErrContentFullAccessDenied 表示 full 接口无权访问。
	ErrContentFullAccessDenied = New(CodeContentFullAccessDenied, "无法查看该内容的完整信息")
)

var (
	// ErrContentVersionConflict 表示版本已存在。
	ErrContentVersionConflict = New(CodeContentVersionConflict, "该版本已存在,请使用新的版本号")
	// ErrContentVersionInvalid 表示版本号不正确。
	ErrContentVersionInvalid = New(CodeContentVersionInvalid, "版本号不正确")
	// ErrContentVersionImmutable 表示已发布版本不可修改。
	ErrContentVersionImmutable = New(CodeContentVersionImmutable, "已发布版本不能直接修改,请创建新版本")
	// ErrContentVersionNotPublished 表示内容版本暂不可被引用。
	ErrContentVersionNotPublished = New(CodeContentVersionNotPublished, "该内容版本暂不可被引用")
)

var (
	// ErrContentShareInvalid 表示共享操作不正确。
	ErrContentShareInvalid = New(CodeContentShareInvalid, "共享操作不正确")
	// ErrContentCloneInvalid 表示克隆操作不正确。
	ErrContentCloneInvalid = New(CodeContentCloneInvalid, "克隆操作不正确")
	// ErrContentSharedNotFound 表示共享内容不存在。
	ErrContentSharedNotFound = New(CodeContentSharedNotFound, "共享内容不存在或已取消共享")
)

var (
	// ErrPaperNotFound 表示试卷不存在。
	ErrPaperNotFound = New(CodePaperNotFound, "试卷不存在或已移除")
	// ErrPaperInvalid 表示试卷信息不完整。
	ErrPaperInvalid = New(CodePaperInvalid, "试卷信息不完整,请检查后重试")
	// ErrPaperItemInvalid 表示试卷题目信息不正确。
	ErrPaperItemInvalid = New(CodePaperItemInvalid, "试卷题目信息不正确")
	// ErrPaperGenerateFailed 表示组卷失败。
	ErrPaperGenerateFailed = New(CodePaperGenerateFailed, "组卷失败,请稍后重试")
	// ErrPaperRegenerateFailed 表示重新组卷失败。
	ErrPaperRegenerateFailed = New(CodePaperRegenerateFailed, "重新组卷失败,请稍后重试")
	// ErrPaperPickNotEnough 表示题目数量不足。
	ErrPaperPickNotEnough = New(CodePaperPickNotEnough, "符合条件的题目数量不足")
)
