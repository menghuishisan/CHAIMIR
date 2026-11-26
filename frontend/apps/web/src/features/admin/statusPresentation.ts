import type { StatusTone } from '@chaimir/ui'
import { BackupStatus } from '@chaimir/api-client'

const BACKUP_STATUS_TONES: Record<BackupStatus, StatusTone> = {
  [BackupStatus.SUCCEEDED]: 'success',
  [BackupStatus.FAILED]: 'danger',
}

/** backupStatusTone 返回备份状态语义色。 */
export function backupStatusTone(status: BackupStatus): StatusTone {
  return BACKUP_STATUS_TONES[status]
}
