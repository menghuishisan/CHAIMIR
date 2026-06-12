// apperr admin_codes 文件定义 M9 管理后台 91xxx/92xxx/93xxx/94xxx 错误码。
package apperr

const (
	// CodeAdminDashboardInvalid 表示看板聚合暂不可用或请求范围非法。
	CodeAdminDashboardInvalid = "91001"
	// CodeAdminStatisticsInvalid 表示统计查询条件不正确。
	CodeAdminStatisticsInvalid = "91002"
)

const (
	// CodeAdminAuditQueryInvalid 表示审计查询条件不正确。
	CodeAdminAuditQueryInvalid = "92001"
	// CodeAdminAuditExportFailed 表示审计导出失败。
	CodeAdminAuditExportFailed = "92002"
	// CodeAdminAuditWriteFailed 表示管理操作审计写入失败。
	CodeAdminAuditWriteFailed = "92003"
)

const (
	// CodeAdminConfigInvalid 表示配置请求不正确。
	CodeAdminConfigInvalid = "93001"
	// CodeAdminConfigNotFound 表示配置不存在或不可访问。
	CodeAdminConfigNotFound = "93002"
	// CodeAdminConfigConflict 表示配置乐观锁冲突。
	CodeAdminConfigConflict = "93003"
	// CodeAdminMonitoringInvalid 表示监控面板配置不符合安全要求。
	CodeAdminMonitoringInvalid = "93004"
	// CodeAdminBackupInvalid 表示备份记录请求不正确。
	CodeAdminBackupInvalid = "93005"
)

const (
	// CodeAdminAlertInvalid 表示告警规则或事件请求不正确。
	CodeAdminAlertInvalid = "94001"
	// CodeAdminAlertNotFound 表示告警不存在或不可访问。
	CodeAdminAlertNotFound = "94002"
	// CodeAdminAlertStateInvalid 表示告警状态不允许当前操作。
	CodeAdminAlertStateInvalid = "94003"
)

var (
	// ErrAdminDashboardInvalid 表示管理看板暂不可用。
	ErrAdminDashboardInvalid = New(CodeAdminDashboardInvalid, "管理看板暂时无法加载,请稍后重试")
	// ErrAdminStatisticsInvalid 表示统计查询条件不正确。
	ErrAdminStatisticsInvalid = New(CodeAdminStatisticsInvalid, "统计查询条件不正确,请检查后重试")
	// ErrAdminAuditQueryInvalid 表示审计查询条件不正确。
	ErrAdminAuditQueryInvalid = New(CodeAdminAuditQueryInvalid, "审计查询条件不正确,请检查后重试")
	// ErrAdminAuditExportFailed 表示审计导出失败。
	ErrAdminAuditExportFailed = New(CodeAdminAuditExportFailed, "审计记录暂时无法导出,请稍后重试")
	// ErrAdminAuditWriteFailed 表示管理操作审计写入失败。
	ErrAdminAuditWriteFailed = New(CodeAdminAuditWriteFailed, "操作记录暂时无法保存,请稍后重试")
	// ErrAdminConfigInvalid 表示配置内容不正确。
	ErrAdminConfigInvalid = New(CodeAdminConfigInvalid, "配置内容不正确,请检查后重试")
	// ErrAdminConfigNotFound 表示配置不存在。
	ErrAdminConfigNotFound = New(CodeAdminConfigNotFound, "配置不存在或已被移除")
	// ErrAdminConfigConflict 表示配置版本冲突。
	ErrAdminConfigConflict = New(CodeAdminConfigConflict, "配置已被更新,请刷新后重试")
	// ErrAdminMonitoringInvalid 表示监控面板配置非法。
	ErrAdminMonitoringInvalid = New(CodeAdminMonitoringInvalid, "监控面板配置不正确,请联系管理员处理")
	// ErrAdminBackupInvalid 表示备份记录请求不正确。
	ErrAdminBackupInvalid = New(CodeAdminBackupInvalid, "备份记录信息不正确,请检查后重试")
	// ErrAdminAlertInvalid 表示告警信息不正确。
	ErrAdminAlertInvalid = New(CodeAdminAlertInvalid, "告警信息不正确,请检查后重试")
	// ErrAdminAlertNotFound 表示告警不存在。
	ErrAdminAlertNotFound = New(CodeAdminAlertNotFound, "告警不存在或已被处理")
	// ErrAdminAlertStateInvalid 表示告警状态不支持当前操作。
	ErrAdminAlertStateInvalid = New(CodeAdminAlertStateInvalid, "当前告警状态不支持该操作")
)
