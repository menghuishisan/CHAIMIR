// admin enum 文件定义 M9 管理后台范围、告警和备份枚举。
package admin

const (
	// ScopeGlobal 表示平台级配置或统计。
	ScopeGlobal int16 = 1
	// ScopeTenant 表示租户级配置或统计。
	ScopeTenant int16 = 2
)

const (
	// AlertLevelNotice 表示提示级告警。
	AlertLevelNotice int16 = 1
	// AlertLevelWarning 表示警告级告警。
	AlertLevelWarning int16 = 2
	// AlertLevelSevere 表示严重级告警。
	AlertLevelSevere int16 = 3
	// AlertLevelCritical 表示紧急级告警。
	AlertLevelCritical int16 = 4
)

const (
	// AlertStatusPending 表示告警待处理。
	AlertStatusPending int16 = 1
	// AlertStatusHandled 表示告警已处理。
	AlertStatusHandled int16 = 2
	// AlertStatusIgnored 表示告警已忽略。
	AlertStatusIgnored int16 = 3
)

const (
	// BackupTypeFull 表示全量备份。
	BackupTypeFull int16 = 1
)

const (
	// BackupStatusRunning 表示备份任务已记录并等待运维/CronJob 执行。
	BackupStatusRunning int16 = 1
	// BackupStatusSucceeded 表示备份任务成功。
	BackupStatusSucceeded int16 = 2
	// BackupStatusFailed 表示备份任务失败。
	BackupStatusFailed int16 = 3
)
