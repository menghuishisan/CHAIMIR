// apperr admin_codes 文件定义 M9 管理后台 91xxx/92xxx/93xxx/94xxx 错误码。
package apperr

const (
	// CodeAdminStatisticsInvalid 表示统计查询条件不正确。
	CodeAdminStatisticsInvalid = "91002"
	// CodeAdminDashboardIdentityFailed 表示看板身份统计依赖暂不可用。
	CodeAdminDashboardIdentityFailed = "91003"
	// CodeAdminDashboardTeachingFailed 表示看板教学统计依赖暂不可用。
	CodeAdminDashboardTeachingFailed = "91004"
	// CodeAdminDashboardExperimentFailed 表示看板实验统计依赖暂不可用。
	CodeAdminDashboardExperimentFailed = "91005"
	// CodeAdminDashboardContestFailed 表示看板竞赛统计依赖暂不可用。
	CodeAdminDashboardContestFailed = "91006"
	// CodeAdminDashboardSandboxFailed 表示看板沙箱统计依赖暂不可用。
	CodeAdminDashboardSandboxFailed = "91007"
)

const (
	// CodeAdminAuditQueryInvalid 表示审计查询条件不正确。
	CodeAdminAuditQueryInvalid = "92001"
	// CodeAdminAuditWriteFailed 表示管理操作审计写入失败。
	CodeAdminAuditWriteFailed = "92003"
	// CodeAdminAuditExportTaskCreateFailed 表示审计导出任务创建失败。
	CodeAdminAuditExportTaskCreateFailed = "92004"
	// CodeAdminAuditExportCSVFailed 表示审计导出 CSV 生成失败。
	CodeAdminAuditExportCSVFailed = "92005"
	// CodeAdminAuditExportUploadPlanFailed 表示审计导出上传路径规划失败。
	CodeAdminAuditExportUploadPlanFailed = "92006"
	// CodeAdminAuditExportObjectWriteFailed 表示审计导出对象写入失败。
	CodeAdminAuditExportObjectWriteFailed = "92007"
	// CodeAdminAuditExportTaskCompleteFailed 表示审计导出任务完成状态保存失败。
	CodeAdminAuditExportTaskCompleteFailed = "92008"
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
	// ErrAdminStatisticsInvalid 表示统计查询条件不正确。
	ErrAdminStatisticsInvalid = New(CodeAdminStatisticsInvalid, "统计查询条件不正确,请检查后重试")
	// ErrAdminDashboardIdentityFailed 表示身份统计暂不可用。
	ErrAdminDashboardIdentityFailed = New(CodeAdminDashboardIdentityFailed, "管理看板身份统计暂时无法加载,请稍后重试")
	// ErrAdminDashboardTeachingFailed 表示教学统计暂不可用。
	ErrAdminDashboardTeachingFailed = New(CodeAdminDashboardTeachingFailed, "管理看板教学统计暂时无法加载,请稍后重试")
	// ErrAdminDashboardExperimentFailed 表示实验统计暂不可用。
	ErrAdminDashboardExperimentFailed = New(CodeAdminDashboardExperimentFailed, "管理看板实验统计暂时无法加载,请稍后重试")
	// ErrAdminDashboardContestFailed 表示竞赛统计暂不可用。
	ErrAdminDashboardContestFailed = New(CodeAdminDashboardContestFailed, "管理看板竞赛统计暂时无法加载,请稍后重试")
	// ErrAdminDashboardSandboxFailed 表示沙箱统计暂不可用。
	ErrAdminDashboardSandboxFailed = New(CodeAdminDashboardSandboxFailed, "管理看板实验环境统计暂时无法加载,请稍后重试")
	// ErrAdminAuditQueryInvalid 表示审计查询条件不正确。
	ErrAdminAuditQueryInvalid = New(CodeAdminAuditQueryInvalid, "审计查询条件不正确,请检查后重试")
	// ErrAdminAuditWriteFailed 表示管理操作审计写入失败。
	ErrAdminAuditWriteFailed = New(CodeAdminAuditWriteFailed, "操作记录暂时无法保存,请稍后重试")
	// ErrAdminAuditExportTaskCreateFailed 表示审计导出任务创建失败。
	ErrAdminAuditExportTaskCreateFailed = New(CodeAdminAuditExportTaskCreateFailed, "审计导出任务暂时无法创建,请稍后重试")
	// ErrAdminAuditExportCSVFailed 表示审计导出文件生成失败。
	ErrAdminAuditExportCSVFailed = New(CodeAdminAuditExportCSVFailed, "审计导出文件暂时无法生成,请稍后重试")
	// ErrAdminAuditExportUploadPlanFailed 表示审计导出文件路径规划失败。
	ErrAdminAuditExportUploadPlanFailed = New(CodeAdminAuditExportUploadPlanFailed, "审计导出文件暂时无法准备,请稍后重试")
	// ErrAdminAuditExportObjectWriteFailed 表示审计导出对象写入失败。
	ErrAdminAuditExportObjectWriteFailed = New(CodeAdminAuditExportObjectWriteFailed, "审计导出文件暂时无法保存,请稍后重试")
	// ErrAdminAuditExportTaskCompleteFailed 表示审计导出任务完成状态保存失败。
	ErrAdminAuditExportTaskCompleteFailed = New(CodeAdminAuditExportTaskCompleteFailed, "审计导出任务暂时无法完成,请稍后重试")
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
