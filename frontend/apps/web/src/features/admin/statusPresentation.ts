// admin statusPresentation 文件维护 M9 状态到设计系统语义色的映射。

import type { StatusTone } from '@chaimir/ui'
import { AlertLevel, AlertStatus, BackupStatus } from '@chaimir/api-client'

const ALERT_STATUS_TONES: Record<AlertStatus, StatusTone> = {
  [AlertStatus.PENDING]: 'warning',
  [AlertStatus.HANDLED]: 'success',
  [AlertStatus.IGNORED]: 'neutral',
}

/** alertStatusTone 返回告警处理状态语义色。 */
export function alertStatusTone(status: AlertStatus): StatusTone {
  return ALERT_STATUS_TONES[status]
}

const ALERT_LEVEL_TONES: Record<AlertLevel, StatusTone> = {
  [AlertLevel.NOTICE]: 'info',
  [AlertLevel.WARNING]: 'warning',
  [AlertLevel.SEVERE]: 'danger',
  [AlertLevel.CRITICAL]: 'danger',
}

/** alertLevelTone 返回告警级别语义色。 */
export function alertLevelTone(level: AlertLevel): StatusTone {
  return ALERT_LEVEL_TONES[level]
}

const BACKUP_STATUS_TONES: Record<BackupStatus, StatusTone> = {
  [BackupStatus.RUNNING]: 'info',
  [BackupStatus.SUCCEEDED]: 'success',
  [BackupStatus.FAILED]: 'danger',
}

/** backupStatusTone 返回备份状态语义色。 */
export function backupStatusTone(status: BackupStatus): StatusTone {
  return BACKUP_STATUS_TONES[status]
}
