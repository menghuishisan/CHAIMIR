// M10 通知与实时推送错误码(A0xxx 站内信 / 公告 / 实时)。
// 文案面向终端用户,内部技术原因通过 WithCause 进入日志。
package apperr

var (
	ErrNotifyInvalid              = New("A0001", "通知内容不完整,请检查后重试")
	ErrNotifyTemplateMissing      = New("A0002", "通知模板不可用,请联系管理员处理")
	ErrNotifySendFailed           = New("A0003", "通知暂时无法发送,请稍后重试")
	ErrNotifyNotFound             = New("A0004", "站内信不存在或已被移除")
	ErrNotifyPreferenceLocked     = New("A0005", "该类通知为必要通知,不能关闭")
	ErrNotifyAnnouncementInvalid  = New("A0006", "公告内容不完整,请检查后重试")
	ErrNotifyAnnouncementNotFound = New("A0007", "公告不存在或已过期")
	ErrNotifyTopicForbidden       = New("A0008", "你不能订阅该实时消息")
	ErrNotifyPushFailed           = New("A0009", "实时消息暂时无法发送,请稍后重试")
	ErrNotifyRateLimited          = New("A0010", "通知发送过于频繁,请稍后再试")
	ErrNotifySubscriptionInvalid  = New("A0011", "实时消息订阅请求不正确,请刷新后重试")
	ErrNotifyInboxQueryInvalid    = New("A0012", "站内信查询条件不正确,请检查后重试")
	ErrNotifyRealtimeUnavailable  = New("A0013", "实时消息通道暂时不可用,请稍后重试")
)
