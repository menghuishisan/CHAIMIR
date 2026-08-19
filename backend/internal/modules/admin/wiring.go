// admin wiring 文件为组合根装配 M9 窄服务入口,不承载业务流程。
package admin

import (
	"fmt"

	"chaimir/internal/platform/db"
	"chaimir/pkg/snowflake"
)

// NewBackupRecorderFromDatabase 由 cron 组合根装配受控备份服务,隐藏 M9 repo 细节。
func NewBackupRecorderFromDatabase(database *db.DB, ids snowflake.Generator) (*BackupRecorder, error) {
	if database == nil {
		return nil, fmt.Errorf("备份记录服务缺少数据库")
	}
	return NewBackupRecorder(NewStore(database), ids)
}
