// 通知模板与待写入记录的内部模型。
package notify

type notificationTemplate struct {
	Type       string
	TitleTpl   string
	ContentTpl string
	Force      bool
}

type notificationRecord struct {
	ID         int64
	TenantID   int64
	ReceiverID int64
	Type       string
	Title      string
	Content    string
	Link       string
	SourceRef  string
}
