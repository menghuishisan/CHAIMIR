// M9 枚举常量:集中定义配置、告警、统计与备份状态。
package admin

const (
	ScopeGlobal int16 = 1
	ScopeTenant int16 = 2

	AlertLevelInfo     int16 = 1
	AlertLevelWarning  int16 = 2
	AlertLevelCritical int16 = 3
	AlertLevelUrgent   int16 = 4

	AlertEventPending int16 = 1
	AlertEventHandled int16 = 2
	AlertEventIgnored int16 = 3

	BackupTypeFull        int16 = 1
	BackupTypeIncremental int16 = 2

	BackupStatusRunning int16 = 1
	BackupStatusSuccess int16 = 2
	BackupStatusFailed  int16 = 3
)
