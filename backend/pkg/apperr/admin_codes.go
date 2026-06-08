// M9 管理后台错误码(91xxx 看板 / 92xxx 审计 / 93xxx 配置 / 94xxx 告警与运维)。
// 文案面向管理员用户,内部技术原因通过 WithCause 进入日志。
package apperr

var (
	ErrAdminDashboard              = New("91001", "看板数据暂时无法获取,请稍后重试")
	ErrAdminApplicationInvalid     = New("91002", "入驻审核信息不完整,请检查后重试")
	ErrAdminStatisticsQueryInvalid = New("91003", "统计查询条件不正确,请检查后重试")
	ErrAdminIdentityUnavailable    = New("91004", "身份管理服务暂时不可用,请稍后重试")

	ErrAdminAuditQuery        = New("92001", "审计记录暂时无法查询,请稍后重试")
	ErrAdminAuditExport       = New("92002", "审计记录暂时无法导出,请稍后重试")
	ErrAdminAuditQueryInvalid = New("92003", "审计查询条件不正确,请检查后重试")
	ErrAdminAuditWriteFailed  = New("92004", "操作审计暂时无法记录,请稍后重试")

	ErrAdminConfigNotFound = New("93001", "配置项不存在")
	ErrAdminConfigInvalid  = New("93002", "配置内容不正确")
	ErrAdminConfigConflict = New("93003", "配置已被他人更新,请刷新后重试")

	ErrAdminAlertInvalid      = New("94001", "告警规则不正确")
	ErrAdminAlertNotFound     = New("94002", "告警不存在或已被移除")
	ErrAdminAlertState        = New("94003", "告警当前状态不支持该操作")
	ErrAdminMonitoringInvalid = New("94004", "监控面板配置不正确")
	ErrAdminBackupInvalid     = New("94005", "备份请求不正确")
	ErrAdminAlertNotifyFailed = New("94006", "告警通知暂时无法发送,请稍后重试")
)
