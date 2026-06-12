// apperr notify_codes 文件定义 M10 通知与实时推送 A0xxx 错误码。
package apperr

const (
	// CodeNotifyRequestInvalid 表示通知请求不正确。
	CodeNotifyRequestInvalid = "A0001"
	// CodeNotifyTemplateUnavailable 表示通知模板不存在或不可用。
	CodeNotifyTemplateUnavailable = "A0002"
	// CodeNotifySendFailed 表示通知发送失败。
	CodeNotifySendFailed = "A0003"
	// CodeNotifyInboxNotFound 表示站内信不存在或不可访问。
	CodeNotifyInboxNotFound = "A0004"
	// CodeNotifyPreferenceLocked 表示强制通知偏好不可关闭。
	CodeNotifyPreferenceLocked = "A0005"
	// CodeNotifyAnnouncementInvalid 表示公告请求不正确。
	CodeNotifyAnnouncementInvalid = "A0006"
	// CodeNotifyAnnouncementNotFound 表示公告不存在或不可访问。
	CodeNotifyAnnouncementNotFound = "A0007"
	// CodeNotifyTopicForbidden 表示实时主题不允许订阅。
	CodeNotifyTopicForbidden = "A0008"
	// CodeNotifyPushFailed 表示实时推送失败。
	CodeNotifyPushFailed = "A0009"
	// CodeNotifyRateLimited 表示通知发送过于频繁。
	CodeNotifyRateLimited = "A0010"
	// CodeNotifySubscribeInvalid 表示实时订阅请求不正确。
	CodeNotifySubscribeInvalid = "A0011"
	// CodeNotifyInboxQueryInvalid 表示站内信查询条件不正确。
	CodeNotifyInboxQueryInvalid = "A0012"
	// CodeNotifyChannelUnavailable 表示实时通道不可用。
	CodeNotifyChannelUnavailable = "A0013"
)

var (
	// ErrNotifyRequestInvalid 表示通知内容不完整。
	ErrNotifyRequestInvalid = New(CodeNotifyRequestInvalid, "通知内容不完整,请检查后重试")
	// ErrNotifyTemplateUnavailable 表示模板不可用。
	ErrNotifyTemplateUnavailable = New(CodeNotifyTemplateUnavailable, "通知模板不可用,请联系管理员处理")
	// ErrNotifySendFailed 表示通知暂时无法发送。
	ErrNotifySendFailed = New(CodeNotifySendFailed, "通知暂时无法发送,请稍后重试")
	// ErrNotifyInboxNotFound 表示站内信不可访问。
	ErrNotifyInboxNotFound = New(CodeNotifyInboxNotFound, "站内信不存在或已被移除")
	// ErrNotifyPreferenceLocked 表示必要通知不可关闭。
	ErrNotifyPreferenceLocked = New(CodeNotifyPreferenceLocked, "该类通知为必要通知,不能关闭")
	// ErrNotifyAnnouncementInvalid 表示公告内容不完整。
	ErrNotifyAnnouncementInvalid = New(CodeNotifyAnnouncementInvalid, "公告内容不完整,请检查后重试")
	// ErrNotifyAnnouncementNotFound 表示公告不可访问。
	ErrNotifyAnnouncementNotFound = New(CodeNotifyAnnouncementNotFound, "公告不存在或已过期")
	// ErrNotifyTopicForbidden 表示不能订阅该主题。
	ErrNotifyTopicForbidden = New(CodeNotifyTopicForbidden, "你不能订阅该实时消息")
	// ErrNotifyPushFailed 表示实时推送失败。
	ErrNotifyPushFailed = New(CodeNotifyPushFailed, "实时消息暂时无法发送,请稍后重试")
	// ErrNotifyRateLimited 表示通知发送过于频繁。
	ErrNotifyRateLimited = New(CodeNotifyRateLimited, "通知发送过于频繁,请稍后再试")
	// ErrNotifySubscribeInvalid 表示订阅请求不正确。
	ErrNotifySubscribeInvalid = New(CodeNotifySubscribeInvalid, "实时消息订阅请求不正确,请刷新后重试")
	// ErrNotifyInboxQueryInvalid 表示收件箱查询条件不正确。
	ErrNotifyInboxQueryInvalid = New(CodeNotifyInboxQueryInvalid, "站内信查询条件不正确,请检查后重试")
	// ErrNotifyChannelUnavailable 表示实时通道不可用。
	ErrNotifyChannelUnavailable = New(CodeNotifyChannelUnavailable, "实时消息通道暂时不可用,请稍后重试")
)
